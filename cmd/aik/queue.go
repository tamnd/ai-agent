// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/tamnd/ai-agent/check"
	"github.com/tamnd/ai-agent/queue"
	"github.com/tamnd/ai-agent/status"
)

// cmdQueue builds the work queue of spec 14.5 and writes it to work/state/queue.jsonl, keeping the leases that run.
func cmdQueue(args []string) error {
	f := newFlags("queue", "[flags]")
	lf := addLadderFlags(f)
	milestone := f.String("milestone", "M1", "the milestone whose targets rank the items: "+strings.Join(queue.Milestones, ", "))
	area := f.String("area", "", "only the items of this top area, for example deep-learning-training")
	dry := f.Bool("n", false, "print the queue and do not write it")
	asJSON := f.Bool("json", false, "print the items as JSON")
	limit := f.Int("limit", 20, "print at most this many items; 0 prints all")
	if err := parse(f, args); err != nil {
		return err
	}
	if f.NArg() > 0 {
		return usageError{"aik queue takes no arguments"}
	}
	if !slices.Contains(queue.Milestones, *milestone) {
		return usageError{fmt.Sprintf("the milestone %q is not one of %s", *milestone, strings.Join(queue.Milestones, ", "))}
	}
	if *area != "" && !*dry {
		// A queue of one area would drop the items and the leases of the others.
		return usageError{"-area filters what is printed; give -n with it, since the queue file holds every area"}
	}
	c, st, err := lf.ladders()
	if err != nil {
		return err
	}
	now := time.Now()
	r, err := coverageReport(*lf.root, c, st, now)
	if err != nil {
		return err
	}
	ds, err := status.LoadDecls(under(*lf.root, *lf.decls))
	if err != nil {
		return err
	}
	old, err := queue.Read(*lf.root)
	if err != nil {
		return err
	}
	items := queue.Build(queue.Input{Report: r, Findings: check.Run(c, check.Options{Decls: ds}),
		Milestone: *milestone, Area: *area, Old: old, Now: now})
	if !*dry {
		if err := queue.Write(*lf.root, items); err != nil {
			return err
		}
	}
	if *asJSON {
		return printJSON(items)
	}
	n := queue.Counts(items)
	fmt.Printf("%d items for %s: write %d, card %d, raise %d, decide %d, fix %d\n", len(items), *milestone,
		n[queue.KindWrite], n[queue.KindCard], n[queue.KindRaise], n[queue.KindDecide], n[queue.KindFix])
	for i, it := range items {
		if *limit > 0 && i == *limit {
			fmt.Printf("  ... %d more\n", len(items)-i)
			break
		}
		lease := ""
		if it.Lease != nil {
			lease = " (held by " + it.Lease.By + " until " + it.Lease.Until + ")"
		}
		fmt.Printf("  %d %-6s %s%s\n      %s\n", it.Rank, it.Kind, it.Target, lease, it.Why)
		for _, x := range it.Next {
			fmt.Printf("      - %s\n", x)
		}
	}
	fmt.Printf("not read yet: %s\n", strings.Join(queue.NotComputed, "; "))
	if !*dry {
		fmt.Printf("wrote %s\n", queue.File)
	}
	return nil
}
