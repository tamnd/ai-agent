// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tamnd/ai-agent/audit"
	"github.com/tamnd/ai-agent/cite"
	"github.com/tamnd/ai-agent/coverage"
	"github.com/tamnd/ai-agent/quality"
	"github.com/tamnd/ai-agent/site"
)

// reportQuality computes the metrics of spec 15.9 from the content tree, the writer state, the last audit report and,
// when given, the findings.
func reportQuality(args []string) error {
	f := newFlags("report quality", "[flags]")
	lf := addLadderFlags(f)
	state := f.String("state", "work/state", "the state files of the writer (14.7), for the audit first-pass rate; a missing directory skips the metric")
	auditFile := f.String("audit", audit.File, "the report of the weekly aik audit -links, for the dead links; a missing file skips the metric")
	issues := f.String("issues", "", "the findings as gh issue list --label finding --state all --json number,state,labels,createdAt,closedAt writes them, for the S2 metric")
	date := f.String("date", "", "the day of the report, YYYY-MM-DD; the default is today")
	out := f.String("o", "", "also write the JSON report to this file, for example build/quality.json")
	asJSON := f.Bool("json", false, "print the report as JSON")
	if err := parse(f, args); err != nil {
		return err
	}
	if f.NArg() > 0 {
		return usageError{"aik report quality takes no arguments"}
	}
	now := time.Now().UTC()
	if *date != "" {
		t, err := time.Parse("2006-01-02", *date)
		if err != nil {
			return usageError{fmt.Sprintf("-date %q is not YYYY-MM-DD", *date)}
		}
		now = t
	}
	c, st, err := lf.ladders()
	if err != nil {
		return err
	}
	shown, _, err := site.ForAgents(c)
	if err != nil {
		return err
	}
	ix, err := cite.Load(c.Root)
	if err != nil {
		return err
	}
	rows, err := coverage.LoadUniverse(c.Root)
	if err != nil {
		return err
	}
	in := quality.Input{Corpus: shown, Status: st, Cite: ix, Core: coverage.CoreIDs(rows), Now: now}
	if in.Audits, err = quality.LoadAudits(under(c.Root, *state)); err != nil {
		return err
	}
	if in.Links, err = quality.LoadLinks(under(c.Root, *auditFile)); err != nil {
		return err
	}
	if *issues != "" {
		b, err := os.ReadFile(*issues)
		if err != nil {
			return err
		}
		in.Issues = []quality.Issue{}
		if err := json.Unmarshal(b, &in.Issues); err != nil {
			return fmt.Errorf("%s: %w", *issues, err)
		}
	}
	if in.Waivers, err = quality.LoadWaivers(c.Root); err != nil {
		return err
	}
	r := quality.Compute(in)
	if *out != "" {
		b, err := json.MarshalIndent(r, "", "  ")
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(*out, append(b, '\n'), 0o644); err != nil {
			return err
		}
	}
	if *asJSON {
		return printJSON(r)
	}
	printQuality(r)
	return nil
}

func printQuality(r quality.Report) {
	fmt.Printf("quality on %s: %d nodes, %d core items\n", r.Date, r.Nodes, r.Core)
	fmt.Printf("%-66s %8s %13s %10s %10s\n", "metric", "value", "count", "M3", "M5")
	mark := func(m quality.Metric, t *quality.Target, met *bool) string {
		s := m.TargetText(t)
		if met != nil && !*met {
			s += "!"
		}
		return s
	}
	for _, m := range r.Metrics {
		count := ""
		if m.Value != nil {
			count = fmt.Sprintf("%d/%d", m.Num, m.Den)
		}
		fmt.Printf("%-66s %8s %13s %10s %10s\n", m.Name, m.Text(), count, mark(m, m.M3, m.MetM3), mark(m, m.M5, m.MetM5))
	}
	var missing []string
	for _, m := range r.Metrics {
		if m.Value == nil {
			missing = append(missing, fmt.Sprintf("  %s: %s", m.ID, m.NotComputed))
		}
	}
	if len(missing) > 0 {
		fmt.Println("not computed")
		fmt.Println(strings.Join(missing, "\n"))
	}
	if off := r.Off(false); len(off) > 0 {
		ids := make([]string, 0, len(off))
		for _, m := range off {
			ids = append(ids, m.ID)
		}
		fmt.Printf("off the M3 target (marked !): %s\n", strings.Join(ids, ", "))
	}
}
