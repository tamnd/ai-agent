// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"

	"github.com/tamnd/ai-agent/cost"
)

// cmdReport prints the reports of 20.4. Only the cost report of 14.8 is built.
func cmdReport(args []string) error {
	if len(args) == 0 {
		return usageError{"use aik report cost"}
	}
	switch args[0] {
	case "cost":
		return reportCost(args[1:])
	case "levels", "tests", "experiments", "quality":
		return fmt.Errorf("aik report %s is not built yet; aik coverage and aik status give the levels", args[0])
	}
	return usageError{fmt.Sprintf("aik report has no report %q; use aik report cost", args[0])}
}

// reportCost sums the model calls of the agent runs by milestone, role, model and item (14.8).
func reportCost(args []string) error {
	f := newFlags("report cost", "[flags]")
	events := f.String("events", "work/logs/events.jsonl", "the events log of the agent runs (14.7)")
	prices := f.String("prices", "", "a YAML file of prices in USD per million tokens, models: {<id>: {in, out, total}}, to estimate the calls that report no cost")
	items := f.Int("items", 10, "print the most costly items, this many; -1 prints all")
	line := f.Float64("line", 0, "the model line of the milestone of the newest call in USD; exit 1 when its cost passes it")
	asJSON := f.Bool("json", false, "print the report as JSON")
	if err := parse(f, args); err != nil {
		return err
	}
	if f.NArg() > 0 {
		return usageError{"aik report cost takes no arguments"}
	}
	fh, err := os.Open(*events)
	if err != nil {
		return err
	}
	defer fh.Close()
	calls, err := cost.Read(fh)
	if err != nil {
		return fmt.Errorf("%s: %w", *events, err)
	}
	var p cost.Prices
	if *prices != "" {
		if p, err = cost.LoadPrices(*prices); err != nil {
			return err
		}
	}
	r := cost.Make(calls, p)
	if *asJSON {
		if err := printJSON(r); err != nil {
			return err
		}
	} else {
		printCost(r, *items)
	}
	if *line > 0 && len(r.Milestones) > 0 {
		cur := r.Milestones[len(r.Milestones)-1]
		if cur.USD() > *line {
			fmt.Fprintf(os.Stderr, "%s spent $%.2f, past its line of $%.2f; the queue stops until a person raises the line\n", cur.Key, cur.USD(), *line)
			return errFindings
		}
	}
	return nil
}

func printCost(r cost.Report, items int) {
	row := func(indent string, s cost.Sum) {
		extra := ""
		if s.Estimated > 0 {
			extra += fmt.Sprintf(", $%.2f of it estimated", s.Estimated)
		}
		if s.Unpriced > 0 {
			extra += fmt.Sprintf(", %d calls with no cost", s.Unpriced)
		}
		if s.Failed > 0 {
			extra += fmt.Sprintf(", %d failed", s.Failed)
		}
		if s.Limited > 0 {
			extra += fmt.Sprintf(", %d rate-limited", s.Limited)
		}
		fmt.Printf("%s%-28s $%9.2f  %5d calls  %10d tokens%s\n", indent, s.Key, s.USD(), s.Calls, s.Tokens, extra)
	}
	if r.Total.Calls == 0 {
		fmt.Println("no model calls")
		return
	}
	fmt.Printf("model calls from %s to %s\n", r.From, r.To)
	row("", r.Total)
	fmt.Println("by milestone and role")
	for _, m := range r.Milestones {
		row("  ", m.Sum)
		for _, s := range m.Roles {
			row("    ", s)
		}
	}
	fmt.Println("by model")
	for _, s := range r.Models {
		row("  ", s)
	}
	if r.Written > 0 {
		fmt.Printf("per written item: $%.2f over %d items (all their calls)\n", r.PerItem, r.Written)
	}
	if items != 0 && len(r.Items) > 0 {
		fmt.Println("by item")
		for i, s := range r.Items {
			if items > 0 && i == items {
				fmt.Printf("  and %d more\n", len(r.Items)-items)
				break
			}
			row("  ", s)
		}
	}
	if r.Total.Unpriced > 0 {
		fmt.Printf("%d calls report no cost (a backend that gives only tokens); give -prices to estimate them\n", r.Total.Unpriced)
	}
}
