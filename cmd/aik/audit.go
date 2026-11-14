// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/tamnd/ai-agent/audit"
	"github.com/tamnd/ai-agent/check"
	"github.com/tamnd/ai-agent/cite"
	"github.com/tamnd/ai-agent/harvest"
	"github.com/tamnd/ai-agent/status"
)

// cmdAudit is the daily audit of audit.yml (spec 20.5): every rule on every node, the citation records, the size
// alarm (20.3) and, with -links, the external links (14.9). It writes reports/audit.json and exits with 1 when a rule
// blocks.
func cmdAudit(args []string) error {
	f := newFlags("audit", "[flags]")
	root := f.String("root", ".", "the root of the content repository")
	out := f.String("out", audit.File, "the report")
	decls := f.String("decls", "build/lean/decls.jsonl", "the declarations that aik lean build wrote, for the L rules; a missing file skips them")
	links := f.Bool("links", false, "also check every external link of the nodes, as the weekly run does")
	workers := f.Int("workers", 8, "with -links, the requests at a time; one host gets one at a time with -gap between")
	gap := f.Duration("gap", 2*time.Second, "with -links, the least time between two requests to one host")
	if err := parse(f, args); err != nil {
		return err
	}
	c, err := load(*root)
	if err != nil {
		return err
	}
	ds, err := status.LoadDecls(under(*root, *decls))
	if err != nil {
		return err
	}
	r := &audit.Report{Check: audit.Checks(check.Run(c, check.Options{Main: true, Decls: ds}))}
	ix, err := cite.Load(c.Root)
	if err != nil {
		return err
	}
	r.Cite = audit.Cites(ix)
	if r.Size, err = audit.Sizes(c.Root, audit.MaxRepo, audit.MaxDir); err != nil {
		return err
	}
	if *links {
		urls, err := audit.URLs(c)
		if err != nil {
			return err
		}
		cl := harvest.NewClient(ver(), *gap)
		cl.HTTP.Timeout = 30 * time.Second
		r.Links = audit.CheckLinks(context.Background(), cl, urls, *workers)
	}
	if err := audit.Write(under(*root, *out), r); err != nil {
		return err
	}
	fmt.Printf("check: %d findings, %d block, %d level, %d warn\n", r.Check.Findings, r.Check.Block, r.Check.Level, r.Check.Warn)
	fmt.Printf("cite: %d records, %d pass the checks of 13.6, %d failing\n", r.Cite.Records, r.Cite.Pass, len(r.Cite.Failing))
	fmt.Printf("size: %.1f MB", float64(r.Size.Bytes)/(1<<20))
	if r.Size.Alarm {
		fmt.Printf(", over the alarm of 20.3: the repository passes %d GB or %d directories pass %d MB", audit.MaxRepo>>30, len(r.Size.Big), audit.MaxDir>>20)
	}
	fmt.Println()
	if r.Links != nil {
		fmt.Printf("links: %d checked, %d dead, %d unsure\n", r.Links.Checked, len(r.Links.Dead), len(r.Links.Unsure))
	}
	fmt.Println("wrote", under(*root, *out))
	if r.Check.Block > 0 {
		return errFindings
	}
	return nil
}
