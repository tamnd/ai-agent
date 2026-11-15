// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/tamnd/ai-agent/reports"
)

// reportLevels counts the ladders of 04.10 over the nodes, by kind and by top area.
func reportLevels(args []string) error {
	f := newFlags("report levels", "[flags]")
	lf := addLadderFlags(f)
	out := f.String("o", "", "also write the JSON report to this file, for example reports/levels.json")
	asJSON := f.Bool("json", false, "print the report as JSON")
	by := f.String("by", "", "also print the counts of each ladder by kind, area or lang")
	if err := parse(f, args); err != nil {
		return err
	}
	if f.NArg() > 0 {
		return usageError{"aik report levels takes no arguments"}
	}
	if *by != "" && *by != "kind" && *by != "area" && *by != "lang" {
		return usageError{fmt.Sprintf("-by %q is not kind, area or lang", *by)}
	}
	c, st, err := lf.ladders()
	if err != nil {
		return err
	}
	r := reports.MakeLevels(c, st)
	if err := writeOut(*lf.root, *out, r); err != nil {
		return err
	}
	if *asJSON {
		return printJSON(r)
	}
	fmt.Printf("The ladders of %d nodes\n", r.Nodes)
	for _, l := range r.Ladders {
		fmt.Printf("\n%s, %s\n", l.ID, l.Name)
		for i, name := range l.Levels {
			n := 0
			if i < len(l.Total) {
				n = l.Total[i]
			}
			fmt.Printf("  %s%d %-28s %6d\n", l.ID, i, name, n)
		}
		var m map[string]reports.Counts
		switch *by {
		case "kind":
			m = l.ByKind
		case "area":
			m = l.ByArea
		case "lang":
			m = l.ByLang
		}
		for _, k := range sortedKeys(m) {
			fmt.Printf("  %-31s", k)
			for i := range l.Levels {
				n := 0
				if i < len(m[k]) {
					n = m[k][i]
				}
				fmt.Printf(" %s%d:%d", l.ID, i, n)
			}
			fmt.Println()
		}
	}
	return nil
}

// reportTests prints the test matrix: the implementations of each language that pass, fail or have not run.
func reportTests(args []string) error {
	f := newFlags("report tests", "[flags]")
	lf := addLadderFlags(f)
	out := f.String("o", "", "also write the JSON report to this file, for example reports/tests.json")
	asJSON := f.Bool("json", false, "print the report as JSON")
	rows := f.Bool("rows", false, "also print each implementation")
	if err := parse(f, args); err != nil {
		return err
	}
	if f.NArg() > 0 {
		return usageError{"aik report tests takes no arguments"}
	}
	c, st, err := lf.ladders()
	if err != nil {
		return err
	}
	r := reports.MakeTests(c, st)
	if err := writeOut(*lf.root, *out, r); err != nil {
		return err
	}
	if *asJSON {
		return printJSON(r)
	}
	fmt.Printf("%-12s %6s %6s %6s %8s\n", "lang", "impls", "pass", "fail", "not run")
	for _, l := range append(r.Langs, r.Total) {
		fmt.Printf("%-12s %6d %6d %6d %8d\n", l.Lang, l.Impls, l.Pass, l.Fail, l.NotRun)
	}
	if *rows {
		fmt.Println()
		for _, x := range r.Rows {
			extra := ""
			if x.Shared[1] > 0 {
				extra += fmt.Sprintf(" shared %d/%d", x.Shared[0], x.Shared[1])
			}
			if x.Diff[1] > 0 {
				extra += fmt.Sprintf(" diff %d/%d", x.Diff[0], x.Diff[1])
			}
			if x.Mutation > 0 {
				extra += fmt.Sprintf(" mutation %.2f", x.Mutation)
			}
			fmt.Printf("  %-40s %-8s %-4s %-8s%s\n", x.Slug, x.Lang, x.Level, x.State, extra)
		}
	}
	return nil
}

// reportExperiments prints the experiment matrix: each experiment with its claims, budget, verdict and R level.
func reportExperiments(args []string) error {
	f := newFlags("report experiments", "[flags]")
	root := f.String("root", ".", "the root of the content repository")
	out := f.String("o", "", "also write the JSON report to this file, for example reports/experiments.json")
	asJSON := f.Bool("json", false, "print the report as JSON")
	if err := parse(f, args); err != nil {
		return err
	}
	if f.NArg() > 0 {
		return usageError{"aik report experiments takes no arguments"}
	}
	c, err := load(*root)
	if err != nil {
		return err
	}
	r := reports.MakeExperiments(c)
	if err := writeOut(*root, *out, r); err != nil {
		return err
	}
	if *asJSON {
		return printJSON(r)
	}
	fmt.Printf("%d experiments, %.1f hours, $%.2f\n", r.Total, r.Hours, r.CostUSD)
	fmt.Printf("  levels:   %s\n", countLine(r.ByLevel, []string{"R0", "R1", "R2"}))
	fmt.Printf("  verdicts: %s\n", countLine(r.ByVerdict, reports.Verdicts))
	fmt.Printf("  budgets:  %s\n\n", countLine(r.ByBudget, nil))
	for _, x := range r.Rows {
		v := x.Verdict
		if v == "" {
			v = "not run"
		}
		fmt.Printf("  %-32s %-28s %-3s %-13s %-10s %s\n", x.Node, x.Exp, x.Level, v, x.Budget, strings.Join(x.Claims, ","))
		for _, s := range x.Next {
			fmt.Printf("  %-32s   next: %s\n", "", s)
		}
	}
	return nil
}

func writeOut(root, p string, v any) error {
	if p == "" {
		return nil
	}
	return writeFileJSON(under(root, p), v)
}

func sortedKeys[V any](m map[string]V) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

// countLine prints the counts in the order of first, then the other keys in order.
func countLine(m map[string]int, first []string) string {
	var parts []string
	seen := map[string]bool{}
	for _, k := range first {
		seen[k] = true
		if m[k] > 0 {
			parts = append(parts, fmt.Sprintf("%s %d", k, m[k]))
		}
	}
	for _, k := range sortedKeys(m) {
		if !seen[k] {
			parts = append(parts, fmt.Sprintf("%s %d", k, m[k]))
		}
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, ", ")
}
