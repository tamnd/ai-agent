// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"

	"github.com/tamnd/ai-agent/crosswalk"
)

// cmdCrosswalk checks the crosswalks of 5.8 and lists their gaps (18.4).
func cmdCrosswalk(args []string) error {
	if len(args) == 0 {
		return usageError{"use aik crosswalk check or aik crosswalk gaps"}
	}
	switch args[0] {
	case "check", "gaps":
	default:
		return usageError{fmt.Sprintf("aik crosswalk has no subcommand %q; use check or gaps", args[0])}
	}
	f := newFlags("crosswalk "+args[0], "[flags]")
	root := f.String("root", ".", "the root of the content repository")
	asJSON := f.Bool("json", false, "print JSON")
	if err := parse(f, args[1:]); err != nil {
		return err
	}
	if f.NArg() > 0 {
		return usageError{"aik crosswalk " + args[0] + " takes no arguments"}
	}
	c, err := load(*root)
	if err != nil {
		return err
	}
	cws, err := crosswalk.Load(c.Root)
	if err != nil {
		return err
	}
	if args[0] == "check" {
		probs := crosswalk.Check(c, cws)
		if *asJSON {
			if probs == nil {
				probs = []string{}
			}
			if err := printJSON(map[string]any{"crosswalks": len(cws), "problems": probs}); err != nil {
				return err
			}
		} else {
			rows := 0
			for _, cw := range cws {
				rows += len(cw.Rows)
			}
			for _, p := range probs {
				fmt.Fprintln(os.Stderr, p)
			}
			fmt.Printf("%d crosswalks, %d rows, %d problems\n", len(cws), rows, len(probs))
		}
		if len(probs) > 0 {
			return errFindings
		}
		return nil
	}
	gaps := crosswalk.Gaps(c, cws)
	if *asJSON {
		if gaps == nil {
			gaps = []crosswalk.Gap{}
		}
		return printJSON(gaps)
	}
	by := map[string]int{}
	for _, g := range gaps {
		fmt.Printf("%s\t%s\t%s\n", g.Crosswalk, g.ID, g.Name)
		by[g.Crosswalk]++
	}
	for _, cw := range cws {
		fmt.Printf("%s: %d of %d rows map to no item\n", cw.Name, by[cw.Name], len(cw.Rows))
	}
	return nil
}
