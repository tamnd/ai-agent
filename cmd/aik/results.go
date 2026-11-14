// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/tamnd/ai-agent/results"
)

// cmdResults reads the results register of spec 10.
func cmdResults(args []string) error {
	if len(args) == 0 || args[0] != "table" {
		return usageError{"use aik results table benchmark@version"}
	}
	f := newFlags("results table", "[flags] benchmark@version")
	root := f.String("root", ".", "the root of the content repository")
	asJSON := f.Bool("json", false, "print the table as JSON")
	if err := parse(f, args[1:]); err != nil {
		return err
	}
	if f.NArg() != 1 {
		return usageError{"name one benchmark version: benchmark@version"}
	}
	bench, version, ok := strings.Cut(f.Arg(0), "@")
	if !ok || bench == "" || version == "" {
		return usageError{fmt.Sprintf("%q is not benchmark@version", f.Arg(0))}
	}
	c, err := load(*root)
	if err != nil {
		return err
	}
	reg := results.Load(c)
	v := reg.Versions[bench][version]
	if v == nil {
		return usageError{fmt.Sprintf("the benchmark %s has no version %q", bench, version)}
	}
	t := reg.TableOf(bench, version, nil)
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(t)
	}
	name := v.Name
	if name == "" {
		name = bench + " " + version
	}
	fmt.Printf("%s · results as of %s · %d results in %d comparable groups\n", name, time.Now().UTC().Format(time.DateOnly), t.Count, len(t.Groups)+len(t.Other))
	fmt.Println("Population: all result cards of this version that passed the citation check.")
	if t.Hidden > 0 {
		fmt.Printf("Not shown: %d results whose number the citation check did not find (M08).\n", t.Hidden)
	}
	print := func(gs []results.Group) {
		for _, g := range gs {
			fmt.Printf("\nMetric %s · setting: %s\n", g.Metric, g.Setting)
			for i, r := range g.Rows {
				mark := ""
				if r.SelfReported {
					mark = " (self-reported)"
				}
				model := r.Model
				if r.ModelVariant != "" {
					model += " (" + r.ModelVariant + ")"
				}
				fmt.Printf("  %2d  %-32s %10s  [%s, %s]  %s%s\n", i+1, model, scoreText(r.Score), r.Source.Src, r.Source.Loc, r.Evidence, mark)
			}
		}
	}
	print(t.Groups)
	if len(t.Other) > 0 {
		fmt.Println("\nOther settings: these results have a hard key that the source does not state.")
		print(t.Other)
	}
	return nil
}

func scoreText(s results.Score) string {
	if s.V == nil {
		return "?"
	}
	if s.Unit == "fraction" {
		return fmt.Sprintf("%.1f%%", 100**s.V)
	}
	if s.Unit == "rating" {
		return ratingText(s)
	}
	return fmt.Sprintf("%g", *s.V)
}

// ratingText prints a rating to the unit with its interval, as the arenas publish it: 1496 [1490, 1503]. The card keeps
// the value of the source.
func ratingText(s results.Score) string {
	if len(s.Interval) == 2 {
		return fmt.Sprintf("%.0f [%.0f, %.0f]", *s.V, s.Interval[0], s.Interval[1])
	}
	return fmt.Sprintf("%.0f", *s.V)
}
