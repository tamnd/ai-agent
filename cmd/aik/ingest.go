// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/tamnd/ai-agent/cite"
	"github.com/tamnd/ai-agent/ingest"
)

// cmdIngest turns inventory rows into cards: aik ingest results.
func cmdIngest(args []string) error {
	if len(args) == 0 || args[0] != "results" {
		return usageError{"aik ingest results [flags] [source]..."}
	}
	f := newFlags("ingest results", "[flags] [source]...")
	root := f.String("root", ".", "the root of the content repository")
	store := f.String("store", "", "the raw store of the snapshots, for the citation check; the default is $AIK_RAW or the user cache")
	date := f.String("date", "", "the date of the run (YYYY-MM-DD); the default is today in UTC")
	dry := f.Bool("n", false, "report what would change and write nothing")
	asJSON := f.Bool("json", false, "print the report as JSON")
	if err := parse(f, args[1:]); err != nil {
		return err
	}
	day, err := runDate(*date)
	if err != nil {
		return err
	}
	c, err := load(*root)
	if err != nil {
		return err
	}
	st := cite.DefaultStore()
	if *store != "" {
		st = cite.Store{Dir: *store}
	}
	rep, err := ingest.Results(c, ingest.Options{Root: *root, Sources: f.Args(), Store: st, Date: day, Version: ver(), DryRun: *dry})
	if err != nil {
		return err
	}
	if *asJSON {
		b, _ := json.Marshal(rep)
		fmt.Println(string(b))
		return nil
	}
	fmt.Printf("%d result rows of %s: %d new cards, %d updated, %d unchanged, %d duplicates, %d not models, %d unresolved\n",
		rep.Rows, strings.Join(rep.Sources, ", "), rep.New, rep.Updated, rep.Unchanged, rep.Duplicates, rep.Skipped, rep.Unresolved)
	if len(rep.Cite) > 0 {
		fmt.Println("citation check:", counts(rep.Cite))
	}
	for i, m := range rep.Missing {
		if i == 20 {
			fmt.Printf("... and %d more names in %s/results-%s.json\n", len(rep.Missing)-20, ingest.ReportDir, day)
			break
		}
		fmt.Printf("  %4d  %s: %s\n", m.Rows, m.Name, m.Reason)
	}
	if *dry {
		fmt.Fprintln(os.Stderr, "-n: nothing written")
	}
	return nil
}
