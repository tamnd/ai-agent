// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/tamnd/ai-agent/cite"
	"github.com/tamnd/ai-agent/coverage"
	"github.com/tamnd/ai-agent/node"
	"github.com/tamnd/ai-agent/release"
	"github.com/tamnd/ai-agent/status"
)

// ladderFlags are the flags of the commands that compute the ladders of every node.
type ladderFlags struct {
	root, results, decls *string
	viz                  *bool
}

func addLadderFlags(f *flag.FlagSet) ladderFlags {
	return ladderFlags{
		root:    f.String("root", ".", "the root of the content repository"),
		results: f.String("results", "build/results", "the directory of the test results"),
		decls:   f.String("decls", "build/lean/decls.jsonl", "the declarations that aik lean build wrote"),
		viz:     f.Bool("viz", false, "build and check the scenes, so the V ladder counts; without it V stays at what the files give"),
	}
}

// ladders loads the corpus and computes the ladders of each node (04.10), as aik status does for one node.
func (l ladderFlags) ladders() (*node.Corpus, map[string]status.Status, error) {
	c, err := load(*l.root)
	if err != nil {
		return nil, nil, err
	}
	res, err := status.LoadResults(under(*l.root, *l.results))
	if err != nil {
		return nil, nil, err
	}
	ds, err := status.LoadDecls(under(*l.root, *l.decls))
	if err != nil {
		return nil, nil, err
	}
	ix, err := cite.Load(*l.root)
	if err != nil {
		return nil, nil, err
	}
	env := status.Env{Corpus: c, Results: res, Decls: ds, Cite: ix}
	if *l.viz {
		env.VizChecked = map[string][]string{}
		for _, n := range c.Nodes {
			if n.Scene == "" && n.ArchScene == "" {
				continue
			}
			if r := checkScene(c, n); r.Skipped == "" {
				env.VizChecked[n.Slug] = r.Fails
			}
		}
	}
	st := map[string]status.Status{}
	for _, n := range c.Nodes {
		st[n.Slug] = status.Of(n, env)
	}
	return c, st, nil
}

// coverageReport computes the report of 18.7 against the last release in manifests/releases.
func coverageReport(root string, c *node.Corpus, st map[string]status.Status, now time.Time) (*coverage.Report, error) {
	rows, err := coverage.LoadUniverse(root)
	if err != nil {
		return nil, err
	}
	works, err := coverage.LoadCanon(root)
	if err != nil {
		return nil, err
	}
	courses, err := coverage.LoadCourses(root)
	if err != nil {
		return nil, err
	}
	in := coverage.Input{Corpus: c, Status: st, Rows: rows, Canon: works, Courses: courses, Now: now}
	last, err := release.Latest(filepath.Join(root, filepath.FromSlash(release.Dir)), "")
	if err != nil {
		return nil, err
	}
	if last != nil {
		in.Last, in.LastVersion = &last.Coverage, last.Version
	}
	return coverage.Compute(in), nil
}

// cmdCoverage computes the coverage levels and the coverage report of spec 18.7. It fails only on a broken promise:
// a core node row below its milestone target after the milestone date.
func cmdCoverage(args []string) error {
	f := newFlags("coverage", "[flags]")
	lf := addLadderFlags(f)
	asJSON := f.Bool("json", false, "print the whole report as JSON")
	out := f.String("o", "", "also write the JSON report to this file, for example build/coverage.json")
	ladders := f.String("ladders", "", "also write the ladders of every node to this file, for example build/status.json")
	items := f.Bool("items", false, "list each item with its level and what the next level needs")
	if err := parse(f, args); err != nil {
		return err
	}
	if f.NArg() > 0 {
		return usageError{"aik coverage takes no arguments"}
	}
	c, st, err := lf.ladders()
	if err != nil {
		return err
	}
	r, err := coverageReport(*lf.root, c, st, time.Now())
	if err != nil {
		return err
	}
	for p, v := range map[string]any{*out: r, *ladders: st} {
		if p == "" {
			continue
		}
		if err := writeFileJSON(under(*lf.root, p), v); err != nil {
			return err
		}
	}
	if *asJSON {
		if err := printJSON(r); err != nil {
			return err
		}
	} else {
		printCoverage(r, *items)
	}
	if len(r.Fails) > 0 {
		if !*asJSON {
			fmt.Fprintf(os.Stderr, "%d %s broken\n", len(r.Fails), plural(len(r.Fails), "promise", "promises"))
		}
		return errFindings
	}
	return nil
}

func writeFileJSON(p string, v any) error {
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	fh, err := os.Create(p)
	if err != nil {
		return err
	}
	if err := writeJSON(fh, v); err != nil {
		_ = fh.Close()
		return err
	}
	return fh.Close()
}

func printCoverage(r *coverage.Report, items bool) {
	fmt.Printf("Coverage on %s (targets: %s)\n\n", r.Date, r.Targets)
	row := func(name string, n coverage.Counts) {
		fmt.Printf("  %-32s", name)
		for _, x := range n {
			fmt.Printf(" %6d", x)
		}
		fmt.Println()
	}
	fmt.Printf("  %-32s", "")
	for i := range coverage.Names {
		fmt.Printf(" %6s", fmt.Sprintf("C%d", i))
	}
	fmt.Println()
	row("all items", r.Levels.Total)
	if r.Levels.Change != nil {
		row("change since "+r.Levels.Since, coverage.Counts(*r.Levels.Change))
	}
	for _, group := range []struct {
		title string
		m     map[string]coverage.Counts
	}{{"by area", r.Levels.ByArea}, {"by kind", r.Levels.ByKind}, {"by core flag", r.Levels.ByCore}} {
		fmt.Printf("\n  %s\n", group.title)
		var keys []string
		for k := range group.m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			row("  "+k, group.m[k])
		}
	}
	u := r.Universe
	fmt.Printf("\nUniverse: %d rows; %d pending; %d items with no row; %d of %d sources unharvested, %d closed\n", u.Rows, len(u.Pending), u.Unregistered, len(u.Unharvested), len(coverage.Sources), len(u.Closed))
	fmt.Printf("Promises: %d (missing-node %d, missing-card %d, below-target %d)\n", len(r.Promises),
		len(r.Gaps["missing-node"]), len(r.Gaps["missing-card"]), len(r.Gaps["below-target"]))
	fmt.Printf("Core items: %d\n", len(r.Core))
	var gaps []string
	for _, g := range []string{"unharvested", "pending", "missing-node", "missing-card", "below-target", "stale", "broken-bridge", "demand"} {
		gaps = append(gaps, fmt.Sprintf("%s %d", g, len(r.Gaps[g])))
	}
	fmt.Printf("Gaps: %s\n", strings.Join(gaps, ", "))
	named := 0
	for _, w := range r.Canon {
		named += w.Named
	}
	fmt.Printf("Canon: %d works, %d named items in the universe\n", len(r.Canon), named)
	cnamed := 0
	for _, w := range r.Courses {
		cnamed += w.Named
	}
	fmt.Printf("Courses: %d courses, %d named items in the universe\n", len(r.Courses), cnamed)
	for _, s := range r.NotComputed {
		fmt.Printf("Not computed: %s\n", s)
	}
	for _, s := range r.Fails {
		fmt.Printf("FAIL %s\n", s)
	}
	if items {
		fmt.Println()
		for _, it := range r.Items {
			fmt.Printf("  %s  %-5s %-12s %s\n", it.Label, it.Form, it.Kind, it.ID)
			for _, s := range it.Next {
				fmt.Printf("            next: %s\n", s)
			}
		}
	}
}
