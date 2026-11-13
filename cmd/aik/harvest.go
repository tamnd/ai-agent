// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/tamnd/ai-agent/cite"
	"github.com/tamnd/ai-agent/harvest"
)

// cmdHarvest is the harvester of spec 13: aik harvest <source>... | -all. Each source writes its inventory, the
// manifest of its snapshots, its run record and its row of registers/sources.yaml.
func cmdHarvest(args []string) error {
	f := newFlags("harvest", "[flags] <source>... | -all | sources")
	root := f.String("root", ".", "the root of the content repository")
	all := f.Bool("all", false, "harvest every source")
	limit := f.Int("limit", 0, "keep at most this many rows of each kind; a partial run removes no rows")
	since := f.String("since", "", "keep only rows from this date (YYYY-MM-DD); a partial run removes no rows")
	offline := f.Bool("offline", false, "do not fetch; read the newest snapshots of the manifest from the store")
	store := f.String("store", "", "the raw store; the default is $AIK_RAW or the user cache")
	date := f.String("date", "", "the date of the run (YYYY-MM-DD); the default is today in UTC")
	asJSON := f.Bool("json", false, "print the run records as JSON")
	if err := parse(f, args); err != nil {
		return err
	}
	if f.NArg() == 1 && f.Arg(0) == "sources" {
		for _, s := range harvest.Sources {
			fmt.Printf("%-10s %-14s %s  %s\n", s.ID, s.Licence, s.Class, s.URL)
		}
		return nil
	}
	names := f.Args()
	if *all {
		names = harvest.Names()
	}
	if len(names) == 0 {
		return usageError{"give a source (" + strings.Join(harvest.Names(), ", ") + ") or -all"}
	}
	if *since != "" {
		if _, err := time.Parse("2006-01-02", *since); err != nil {
			return usageError{"-since takes a date YYYY-MM-DD"}
		}
	}
	day, err := runDate(*date)
	if err != nil {
		return err
	}
	if _, err := os.Stat(*root); err != nil {
		return err
	}
	st := cite.DefaultStore()
	if *store != "" {
		st = cite.Store{Dir: *store}
	}
	o := harvest.Options{Limit: *limit, Since: *since}
	ctx := context.Background()
	failed := 0
	for _, name := range names {
		a, ok := harvest.Adapters[name]
		src := harvest.SourceByID(name)
		if !ok || src == nil {
			return usageError{fmt.Sprintf("no source %q; the sources are %s", name, strings.Join(harvest.Names(), ", "))}
		}
		r := &harvest.Run{Root: *root, Source: *src, Store: st, Date: day, Version: ver(), Offline: *offline}
		if !*offline {
			r.Client = harvest.NewClient(ver(), a.Gap)
		}
		rows, err := a.Run(ctx, r, o)
		if err != nil {
			fmt.Fprintf(os.Stderr, "aik harvest %s: %v\n", name, err)
			failed++
			continue
		}
		rep, err := r.Finish(rows, o)
		if err != nil {
			return err
		}
		if *asJSON {
			b, _ := json.Marshal(rep)
			fmt.Println(string(b))
			continue
		}
		part := ""
		if rep.Partial {
			part = " (partial)"
		}
		fmt.Printf("%s%s: %d rows, %d new, %d changed, %d removed, %d unchanged, %d skipped, %d requests, %d snapshots; %s (class %s)\n",
			name, part, rep.Rows, rep.New, rep.Changed, rep.Removed, rep.Unchanged, rep.Skipped, rep.Requests, len(rep.Snapshots), rep.Licence, rep.Class)
	}
	if failed > 0 {
		return errFindings
	}
	return nil
}

// runDate is the date of a run: the flag, or today in UTC.
func runDate(s string) (string, error) {
	if s == "" {
		return time.Now().UTC().Format("2006-01-02"), nil
	}
	if _, err := time.Parse("2006-01-02", s); err != nil {
		return "", usageError{"-date takes a date YYYY-MM-DD"}
	}
	return s, nil
}
