// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/tamnd/ai-agent/contract"
	"github.com/tamnd/ai-agent/lang"
	"github.com/tamnd/ai-agent/node"
	"github.com/tamnd/ai-agent/status"
)

type diffFlags struct {
	classes, seeds string
	write          bool
	root, out      string
	box            *lang.Sandbox
}

// problemInputs are the generated inputs of one problem, made once for all the implementations, or the reason why the problem has none.
type problemInputs struct {
	in   *lang.Inputs
	skip string
	err  error
}

// inputCache makes the generated inputs of each problem once, for all the implementations.
type inputCache struct {
	d       diffFlags
	classes []string
	seeds   []int
	inputs  map[string]*problemInputs
	closers []func()
}

func newInputCache(d diffFlags) (*inputCache, error) {
	c := &inputCache{d: d, classes: strings.Split(d.classes, ","), inputs: map[string]*problemInputs{}}
	for _, cl := range c.classes {
		if !contains(contract.ClassOrder, cl) {
			return nil, usageError{fmt.Sprintf("the class %q is not one of %s", cl, strings.Join(contract.ClassOrder, ", "))}
		}
	}
	for _, s := range strings.Split(d.seeds, ",") {
		if s = strings.TrimSpace(s); s == "" {
			continue
		}
		n, err := strconv.Atoi(s)
		if err != nil {
			return nil, usageError{"-seeds is a comma list of integers"}
		}
		c.seeds = append(c.seeds, n)
	}
	return c, nil
}

// Close stops the oracles and the generators.
func (c *inputCache) Close() {
	for _, f := range c.closers {
		f()
	}
}

// get gives the generated inputs of the problem, with the oracle output as expect.
func (c *inputCache) get(k *contract.Contract) *problemInputs {
	if pi, ok := c.inputs[k.Problem]; ok {
		return pi
	}
	pi := &problemInputs{}
	c.inputs[k.Problem] = pi
	g, err := k.LoadGenerated()
	if err != nil {
		pi.err = err
		return pi
	}
	if g == nil {
		pi.skip = "the problem has no cases/generated.yaml"
		return pi
	}
	if c.d.box != nil {
		if err := c.d.box.Ready("python"); err != nil {
			pi.err = fmt.Errorf("the generators and the oracle run in the Python image: %w", err)
			return pi
		}
	}
	o, err := k.StartOracle(lang.CheckerWrap(c.d.box, k))
	if errors.Is(err, contract.ErrNoOracle) {
		pi.skip = err.Error()
		return pi
	}
	if err != nil {
		pi.err = err
		return pi
	}
	c.closers = append(c.closers, o.Close)
	gen, err := k.StartGenerator(lang.CheckerWrap(c.d.box, k))
	if err != nil {
		pi.err = err
		return pi
	}
	c.closers = append(c.closers, gen.Close)
	ss := c.seeds
	if len(ss) == 0 {
		ss = g.Seeds.PR
	}
	start := time.Now()
	pi.in, pi.err = lang.MakeInputs(k, g, gen, o, g.Plan(k, c.classes, ss))
	if pi.err == nil {
		fmt.Printf("%s: %d generated inputs with the oracle %s (%d ms)\n", k.Problem, len(pi.in.Cases), o.File, time.Since(start).Milliseconds())
	}
	return pi
}

// runDiff runs the differential test of each implementation against the oracle of each problem that it solves, on the generated inputs (spec 09.5). It shrinks each failure, writes the regression cases to build/regress/<problem>.jsonl, and with -write-regress also to cases/regress.jsonl of the problem.
func runDiff(r *lang.Runner, nodes []*node.Node, want map[string]bool, problems func(*node.Node) []*sharedCases, d diffFlags) error {
	cache, err := newInputCache(d)
	if err != nil {
		return err
	}
	defer cache.Close()
	get := cache.get
	failed := 0
	byLang := map[string]*status.Results{}
	record := func(n *node.Node, l string, dr status.Diff) {
		res := byLang[l]
		if res == nil {
			res = &status.Results{Lang: l, Kind: "diff"}
			byLang[l] = res
		}
		if k := len(res.Tests); k == 0 || res.Tests[k-1].Slug != n.Slug {
			res.Tests = append(res.Tests, status.Test{Slug: n.Slug, Lang: l})
		}
		t := &res.Tests[len(res.Tests)-1]
		t.Diff = append(t.Diff, dr)
	}
	regress := map[string][]contract.Case{}
	dirs := map[string]string{}
	for _, n := range nodes {
		for _, im := range n.Impls {
			if len(want) > 0 && !want[im.Lang] {
				continue
			}
			rc, ok := lang.Recipes[im.Lang]
			if !ok {
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
			for _, p := range problems(n) {
				if p.err != nil {
					return fmt.Errorf("the contract of a problem that %s solves: %w", n.Slug, p.err)
				}
				pi := get(p.k)
				if pi.err != nil {
					return fmt.Errorf("%s: %w", p.k.Problem, pi.err)
				}
				if pi.skip != "" {
					fmt.Printf("skip %s/%s differential run of %s: %s\n", n.Slug, im.Lang, p.k.Problem, pi.skip)
					continue
				}
				ss, err := r.Open(n, im, p.k)
				if err != nil {
					fmt.Printf("FAIL %s/%s differential run of %s: %v\n", n.Slug, im.Lang, p.k.Problem, err)
					failed++
					record(n, im.Lang, status.Diff{Problem: p.k.Problem, Error: err.Error()})
					continue
				}
				res, err := lang.Diff(ss, pi.in)
				ss.Close()
				if err != nil {
					return fmt.Errorf("%s/%s: %w", n.Slug, im.Lang, err)
				}
				state := "pass"
				if len(res.Failures) > 0 {
					state = "FAIL"
					failed++
				}
				line := fmt.Sprintf("%-4s %s/%s differential run of %s: %d of %d pass", state, n.Slug, im.Lang, p.k.Problem, res.Passed, res.Cases)
				if len(res.Failures) > 0 && res.Passed+len(res.Failures) < res.Cases {
					line = fmt.Sprintf("%-4s %s/%s differential run of %s: %d failures, the run stopped after %d of %d cases", state, n.Slug, im.Lang, p.k.Problem, len(res.Failures), res.Passed+len(res.Failures), res.Cases)
				}
				if res.Skipped > 0 {
					line += fmt.Sprintf(", %d skipped", res.Skipped)
				}
				fmt.Printf("%s (%d ms)\n", line, res.Took.Milliseconds())
				dr := status.Diff{Problem: p.k.Problem, Cases: res.Cases, Passed: res.Passed, Skipped: res.Skipped}
				for _, f := range res.Failures {
					dr.Failures = append(dr.Failures, status.CaseFailure{Case: f.Case.Name, Reason: f.Reason + "; it shrinks to the arguments " + short(argsJSON(f.Shrunk.Args)), Got: short(f.Got), Want: short(f.Case.Expect)})
				}
				record(n, im.Lang, dr)
				for _, f := range res.Failures {
					reg := lang.Regression(p.k, f, n.Slug+"/"+im.Lang, time.Now().UTC())
					fmt.Println(indent(fmt.Sprintf("case %s: %s\n  got:  %s\n  want: %s\nshrunk in %d runs to %s: %s\n  args: %s\n  got:  %s\n  want: %s", f.Case.Name, f.Reason, short(f.Got), short(f.Case.Expect), f.Calls, reg.Name, f.ShrunkReason, short(argsJSON(reg.Args)), short(f.ShrunkGot), short(reg.Expect))))
					regress[p.k.Problem] = append(regress[p.k.Problem], reg)
					dirs[p.k.Problem] = p.k.Dir
				}
			}
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
			if err := os.WriteFile(filepath.Join(d.out, l+".diff.json"), buf.Bytes(), 0o644); err != nil {
				return err
			}
		}
	}
	for prob, cs := range regress {
		out := filepath.Join(d.root, "build", "regress", prob+".jsonl")
		if err := appendCases(out, cs); err != nil {
			return err
		}
		fmt.Printf("wrote %d regression cases to %s\n", len(cs), out)
		if d.write {
			path := filepath.Join(dirs[prob], "cases", "regress.jsonl")
			if err := appendCases(path, cs); err != nil {
				return err
			}
			fmt.Printf("added %d regression cases to %s\n", len(cs), path)
		}
	}
	if failed > 0 {
		return errFindings
	}
	return nil
}

func contains(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}

func short(b []byte) string {
	if len(b) == 0 {
		return "(none)"
	}
	var buf bytes.Buffer
	if json.Compact(&buf, b) == nil {
		b = buf.Bytes()
	}
	return contract.Short(string(b))
}

func argsJSON(a map[string]json.RawMessage) []byte {
	b, _ := json.Marshal(a)
	return b
}

// appendCases adds the cases to a case file, and skips a case whose name is in the file already.
func appendCases(path string, cs []contract.Case) error {
	old, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var b bytes.Buffer
	b.Write(old)
	if len(old) > 0 && !bytes.HasSuffix(old, []byte("\n")) {
		b.WriteByte('\n')
	}
	for _, c := range cs {
		if bytes.Contains(old, []byte(`"name":"`+c.Name+`"`)) {
			continue
		}
		line, err := json.Marshal(c)
		if err != nil {
			return err
		}
		b.Write(line)
		b.WriteByte('\n')
	}
	return os.WriteFile(path, b.Bytes(), 0o644)
}
