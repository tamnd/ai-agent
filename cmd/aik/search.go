// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/tamnd/ai-agent/search"
	"github.com/tamnd/ai-agent/site"
)

// cmdSearch runs the evaluation of the search (17.10).
func cmdSearch(args []string) error {
	if len(args) == 0 || args[0] != "eval" {
		return usageError{"use aik search eval"}
	}
	f := newFlags("search eval", "[flags]")
	root := f.String("root", ".", "the root of the content repository")
	index := f.String("index", "build/site/search.json", "the index that aik site build wrote")
	baseline := f.String("baseline", "", "the MRR at 10 of the last release, or the JSON report of its evaluation")
	asJSON := f.Bool("json", false, "print the report as JSON")
	all := f.Int("show", 20, "print at most this many failed queries; -1 prints all")
	if err := parse(f, args[1:]); err != nil {
		return err
	}
	if f.NArg() > 0 {
		return usageError{"aik search eval takes no arguments"}
	}
	set, err := search.Read(*root)
	if err != nil {
		return err
	}
	if set == nil {
		return fmt.Errorf("no evaluation set: %s is missing", search.File)
	}
	var base *float64
	if *baseline != "" {
		v, err := readBaseline(*baseline)
		if err != nil {
			return usageError{err.Error()}
		}
		base = &v
	}
	rep, err := evalSearch(set, under(*root, *index), base)
	if err != nil {
		return err
	}
	if *asJSON {
		if err := printJSON(rep); err != nil {
			return err
		}
	} else {
		printSearchReport(rep, *all)
	}
	if !rep.OK() {
		return errFindings
	}
	return nil
}

// evalSearch runs a set on an index file with the script of the site.
func evalSearch(set []search.Query, index string, base *float64) (*search.Report, error) {
	b, err := os.ReadFile(index)
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("no index at %s: run aik site build first", index)
	}
	if err != nil {
		return nil, err
	}
	return search.Eval(set, site.FindJS(), b, base)
}

// readBaseline reads the MRR of the last release: a number, or the JSON report of aik search eval -json.
func readBaseline(s string) (float64, error) {
	if v, err := strconv.ParseFloat(s, 64); err == nil {
		return v, nil
	}
	b, err := os.ReadFile(s)
	if err != nil {
		return 0, fmt.Errorf("-baseline: %w", err)
	}
	var r struct {
		MRR *float64 `json:"mrr"`
	}
	if err := json.Unmarshal(b, &r); err != nil || r.MRR == nil {
		return 0, fmt.Errorf("-baseline: %s is not a report of aik search eval -json", s)
	}
	return *r.MRR, nil
}

func printSearchReport(rep *search.Report, show int) {
	fmt.Printf("search eval: %d queries on %d entries, MRR at 10 %.4f\n", rep.Queries, rep.Entries, rep.MRR)
	if n := len(rep.Pending); n > 0 {
		fmt.Printf("  %d queries pending: none of their items is in the index, so no gate counts them\n", n)
	}
	for _, g := range rep.Gates {
		mark := "ok  "
		if !g.OK {
			mark = "FAIL"
		}
		var got string
		switch {
		case g.AtMost:
			got = fmt.Sprintf("%.0f KB, at most %.0f KB", g.Got/1000, g.Want/1000)
		case g.Note == "no queries":
			got = "no queries"
		default:
			got = fmt.Sprintf("%.4f of %d, at least %.2f", g.Got, g.N, g.Want)
		}
		note := ""
		if g.Note != "" && g.Note != "no queries" {
			note = " (" + g.Note + ")"
		}
		fmt.Printf("  %s %s: %s%s\n", mark, g.Name, got, note)
	}
	for _, s := range rep.NotComputed {
		fmt.Printf("  not measured: %s\n", s)
	}
	for i, q := range rep.Pending {
		if show >= 0 && i >= show {
			fmt.Printf("... and %d more pending queries\n", len(rep.Pending)-i)
			break
		}
		fmt.Printf("%s:%d: %q (%s) is pending: %s not in the index\n", search.File, q.Line, q.Q, q.Class, strings.Join(q.Want, ", "))
	}
	for i, o := range rep.Failed {
		if show >= 0 && i >= show {
			fmt.Printf("... and %d more failed queries; -show -1 prints all\n", len(rep.Failed)-i)
			break
		}
		printOutcome(o)
	}
}

func printOutcome(o search.Outcome) {
	var want []string
	for i, w := range o.Want {
		r := "not in the results"
		if o.Ranks[i] > 0 {
			r = fmt.Sprintf("rank %d", o.Ranks[i])
		}
		want = append(want, w+" ("+r+")")
	}
	fmt.Printf("%s:%d: %q (%s) expects %s %s\n", search.File, o.Line, o.Q, o.Class, strings.Join(want, ", "), search.Expect(o.Query))
	for _, m := range o.Missing {
		fmt.Printf("    %s is not in the index, so the query skips it\n", m)
	}
	if len(o.Top) == 0 {
		fmt.Println("    no results")
	}
	for i, e := range o.Top {
		fmt.Printf("    %d. %s %s [%s] %.4f\n", i+1, e.Name, e.URL, e.Kind, e.Sc)
	}
}

// searchGate runs the evaluation set after a build of the site, when the content repository has one, and fails the
// build when a gate fails (17.10).
func searchGate(root, out string) error {
	set, err := search.Read(root)
	if err != nil || set == nil {
		return err
	}
	rep, err := evalSearch(set, filepath.Join(out, "search.json"), nil)
	if err != nil {
		return err
	}
	if rep.OK() {
		fmt.Fprintf(os.Stderr, "search eval: %d queries, MRR at 10 %.4f, every gate passed\n", rep.Queries, rep.MRR)
		return nil
	}
	printSearchReport(rep, 20)
	return fmt.Errorf("a gate of the search evaluation failed (17.10); see aik search eval")
}
