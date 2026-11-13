// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tamnd/ai-agent/contract"
	"github.com/tamnd/ai-agent/lang"
	"github.com/tamnd/ai-agent/mutate"
	"github.com/tamnd/ai-agent/node"
	"github.com/tamnd/ai-agent/status"
)

// withMutant gives a copy of the implementation with the mutant in its file.
func withMutant(im node.Impl, m mutate.Mutant) node.Impl {
	out := im
	out.Files = append([]node.File{}, im.Files...)
	for i, f := range out.Files {
		if f.Name == m.File {
			out.Files[i].Content = m.Apply(f.Content)
		}
	}
	return out
}

// killer runs the tests of one implementation on a version of its code, in the order of their cost: the shared cases, the generated inputs of the differential run, then the native tests (spec 09.6). It stops at the first test that fails.
type killer struct {
	r        *lang.Runner
	n        *node.Node
	problems []*sharedCases
	inputs   func(*contract.Contract) *problemInputs
}

// kill tells which test fails on the code, or gives "" when each test passes. A build that fails and a timeout kill the mutant.
func (k killer) kill(im node.Impl) (string, error) {
	for _, p := range k.problems {
		ss, err := k.r.Open(k.n, im, p.k)
		if err != nil {
			return "the build fails: " + firstLine(err.Error()), nil
		}
		why, err := firstFailure(ss, p.cases, "the shared case ")
		if err == nil && why == "" {
			if pi := k.inputs(p.k); pi.in != nil {
				why, err = firstFailure(ss, pi.in.Cases, "the generated case ")
			}
		}
		ss.Close()
		if err != nil || why != "" {
			return why, err
		}
	}
	if im.HasTests() {
		if t := k.r.Test(k.n, im); !t.Pass {
			return "the native tests fail", nil
		}
	}
	return "", nil
}

// firstFailure runs the cases until one fails, and gives the reason.
func firstFailure(ss *lang.Session, cases []contract.Case, what string) (string, error) {
	for _, c := range cases {
		if !ss.Supports(c) {
			continue
		}
		_, reason, err := ss.Run(c)
		if err != nil {
			return "", err
		}
		if reason != "" {
			return what + c.ID() + " fails: " + firstLine(reason), nil
		}
	}
	return "", nil
}

// mutantCaseTimeout is the longest time of one case in the mutation run.
const mutantCaseTimeout = 2 * time.Second

// runMutate runs the mutation test of each implementation (spec 09.6). It writes the result of each language to <out>/<lang>.mutate.json. A mutant that the tests do not kill is a survivor; the report names it with its id, as mutants.ignore lists it.
func runMutate(r *lang.Runner, nodes []*node.Node, want map[string]bool, problems func(*node.Node) []*sharedCases, d diffFlags, max int, tc *testCache, all bool) error {
	cache, err := newInputCache(d)
	if err != nil {
		return err
	}
	defer cache.Close()
	// A mutant that loops forever is common, and a timeout kills it, so the time of a case is short. The code with no mutant must pass with the same time.
	r.CaseTimeout = mutantCaseTimeout
	r.NoLint = true // a mutant changes no format, and the lint of the code runs once in aik test
	byLang := map[string]*status.Results{}
	failed := 0
	for _, n := range nodes {
		for _, im := range n.Impls {
			if len(want) > 0 && !want[im.Lang] {
				continue
			}
			rc, ok := lang.Recipes[im.Lang]
			if !ok {
				continue
			}
			if _, ok := mutate.Languages[im.Lang]; !ok {
				fmt.Fprintf(os.Stderr, "skip %s/%s: the mutator does not read %s\n", n.Slug, im.Lang, im.Lang)
				continue
			}
			if err := r.Available(rc); err != nil {
				var skip lang.Skip
				if errors.As(err, &skip) {
					fmt.Fprintf(os.Stderr, "skip %s/%s: %v\n", n.Slug, im.Lang, err)
					continue
				}
				return err
			}
			ps := problems(n)
			for _, p := range ps {
				if p.err != nil {
					return fmt.Errorf("the contract of a problem that %s solves: %w", n.Slug, p.err)
				}
				if pi := cache.get(p.k); pi.err != nil {
					return fmt.Errorf("%s: %w", p.k.Problem, pi.err)
				}
			}
			res := byLang[im.Lang]
			if res == nil {
				res = &status.Results{Lang: im.Lang, Kind: "mutate"}
				byLang[im.Lang] = res
			}
			// A mutation run is the slowest check, so its result is cached as the tests are, under the inputs of the
			// tests and the sample: the same code, tests and seed give the same mutants and the same kills.
			var key string
			if tc != nil {
				var ks []*contract.Contract
				for _, p := range ps {
					ks = append(ks, p.k)
				}
				tool := r.ToolVersion(rc)
				if d.box != nil {
					ref, _ := d.box.Image(im.Lang)
					tool += " image " + ref
				}
				key = tc.key(fmt.Sprintf("%s mutate max %d classes %s seeds %s", tool, max, d.classes, d.seeds), n, im, ks)
				if t, ok := tc.Get(mutateCacheDir(im.Lang), key); ok && t.Mutation != nil {
					m := *t.Mutation
					fmt.Printf("pass %s/%s mutation run: the tests kill %d of %d mutants (%.1f%%) (cached)\n", n.Slug, im.Lang, m.Killed, m.Mutants-m.Ignored, 100*m.Score())
					res.Tests = append(res.Tests, t)
					continue
				}
			}
			m, err := mutateImpl(killer{r: r, n: n, problems: ps, inputs: cache.get}, im, max)
			if err != nil {
				return fmt.Errorf("%s/%s: %w", n.Slug, im.Lang, err)
			}
			pass := m.Error == "" && m.Score() >= status.MinScoreFor(im.Lang)
			if !pass {
				failed++
			}
			t := status.Test{Slug: n.Slug, Lang: im.Lang, Pass: pass, Mutation: &m}
			if tc != nil {
				// Put keeps only a result that passes, so a run under the threshold runs again next time.
				if err := tc.Put(mutateCacheDir(im.Lang), key, t); err != nil {
					return err
				}
			}
			res.Tests = append(res.Tests, t)
		}
	}
	if d.out != "" {
		if err := os.MkdirAll(d.out, 0o755); err != nil {
			return err
		}
		for l, res := range byLang {
			var buf bytes.Buffer
			if err := writeJSON(&buf, res); err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(d.out, l+".mutate.json"), buf.Bytes(), 0o644); err != nil {
				return err
			}
		}
	}
	if tc != nil && all {
		dirs := make([]string, 0, len(byLang))
		for l := range byLang {
			dirs = append(dirs, mutateCacheDir(l))
		}
		tc.Prune(dirs)
	}
	if failed > 0 {
		fmt.Fprintf(os.Stderr, "%d implementations have a mutation score under %.0f%% (%.0f%% for a framework version), or the run did not start\n", failed, 100*status.MinScore, 100*status.XMinScore)
		return errFindings
	}
	return nil
}

// mutateImpl runs the mutants of one implementation and prints the result.
func mutateImpl(k killer, im node.Impl, max int) (status.Mutation, error) {
	start := time.Now()
	var m status.Mutation
	name := k.n.Slug + "/" + im.Lang
	set, err := mutate.Of(im, max)
	if err != nil {
		m.Error = err.Error()
		fmt.Printf("FAIL %s mutation run: %s\n", name, m.Error)
		return m, nil
	}
	for _, e := range set.Stale {
		fmt.Printf("note %s: line %d of %s names no mutant of the sample: %s\n", name, e.Line, mutate.IgnoreFile, e.ID)
	}
	// The tests must pass on the code with no mutant, or no mutant can survive.
	why, err := k.kill(im)
	if err != nil {
		return m, err
	}
	if why != "" {
		m.Error = "the code fails its tests with no mutant: " + why
		fmt.Printf("FAIL %s mutation run: %s\n", name, m.Error)
		return m, nil
	}
	m.Mutants = len(set.Mutants)
	for _, mu := range set.Mutants {
		if _, ok := set.Ignore[mu.ID()]; ok {
			m.Ignored++
			continue
		}
		why, err := k.kill(withMutant(im, mu))
		if err != nil {
			return m, err
		}
		if why != "" {
			m.Killed++
			continue
		}
		m.Survivors = append(m.Survivors, mu.ID())
	}
	state := "pass"
	if m.Score() < status.MinScore {
		state = "FAIL"
	}
	fmt.Printf("%-4s %s mutation run: the tests kill %d of %d mutants (%.1f%%), %d ignored (%d ms)\n", state, name, m.Killed, m.Mutants-m.Ignored, 100*m.Score(), m.Ignored, time.Since(start).Milliseconds())
	for _, s := range m.Survivors {
		fmt.Println("    survives: " + s)
	}
	return m, nil
}

func firstLine(s string) string {
	l, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	return l
}

// mutateCacheDir is the directory of the cache that holds the mutation runs of a language, apart from its tests.
func mutateCacheDir(lang string) string { return lang + ".mutate" }
