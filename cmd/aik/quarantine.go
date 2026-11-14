// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/tamnd/ai-agent/prov"
)

// cmdQuarantine hides a node, or some of its files, from the site at the next build, or clears the quarantine
// (15.7, 19.11). It writes lines of the provenance log of the node (04.8): only a person quarantines or clears.
func cmdQuarantine(args []string) error {
	f := newFlags("quarantine", "-by <login> -why <reason> [-file <path>]... <slug> | -clear -by <login> <slug>")
	root := f.String("root", ".", "the root of the content repository")
	by := f.String("by", os.Getenv("GITHUB_ACTOR"), "the GitHub login of the person; the default is $GITHUB_ACTOR")
	why := f.String("why", "", "the reason, for the log; the page never shows it")
	clear := f.Bool("clear", false, "clear the quarantine of the files, or of every file of the node when no -file is given")
	list := f.Bool("list", false, "list the nodes and files in quarantine")
	day := f.String("date", time.Now().UTC().Format("2006-01-02"), "the date of the line")
	var files listFlag
	f.Var(&files, "file", "a file of the node directory, for example impl/python/x.py; repeat or give a comma list; the default is the whole node")
	if err := parse(f, args); err != nil {
		return err
	}
	c, err := load(*root)
	if err != nil {
		return err
	}
	if *list {
		if f.NArg() > 0 {
			return usageError{"-list takes no slug"}
		}
		n := 0
		for _, nd := range c.Nodes {
			if nd.Tag == "" || strings.HasPrefix(nd.Tag, "NEW-") {
				continue
			}
			ls, err := prov.Read(*root, nd.Tag)
			if err != nil {
				return err
			}
			q := prov.Quarantined(ls)
			keys := make([]string, 0, len(q))
			for k := range q {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				l := q[k]
				fmt.Printf("%s %s %s by %s: %s\n", l.Date, nd.Slug, k, l.Author, l.Note)
				n++
			}
		}
		fmt.Printf("%d files in quarantine\n", n)
		return nil
	}
	if f.NArg() != 1 {
		return usageError{"give one slug"}
	}
	if *by == "" {
		return usageError{"give the GitHub login of the person with -by"}
	}
	if !*clear && strings.TrimSpace(*why) == "" {
		return usageError{"give the reason with -why"}
	}
	n, err := provNode(c, f.Arg(0))
	if err != nil {
		return err
	}
	if n.Tag == "" || strings.HasPrefix(n.Tag, "NEW-") {
		return fmt.Errorf("%s has no minted tag; a page that is not on main needs no quarantine", n.Slug)
	}
	ls, err := prov.Read(*root, n.Tag)
	if err != nil {
		return err
	}
	have, err := prov.Files(n.Dir)
	if err != nil {
		return err
	}
	q := prov.Quarantined(ls)
	targets := []string(files)
	if len(targets) == 0 {
		targets = []string{prov.Whole}
		if *clear {
			targets = nil
			for k := range q {
				targets = append(targets, k)
			}
			sort.Strings(targets)
		}
	}
	var add []prov.Line
	for _, t := range targets {
		t = path.Clean(filepath.ToSlash(t))
		if t != prov.Whole {
			if _, ok := have[t]; !ok {
				return fmt.Errorf("%s has no file %s", n.Slug, t)
			}
		}
		l := prov.Line{Date: *day, File: t, Op: prov.Quarantine, Hash: have[t], Author: prov.Author{Login: *by}, Note: strings.TrimSpace(*why)}
		if *clear {
			if _, ok := q[t]; !ok {
				return fmt.Errorf("%s of %s is not in quarantine", t, n.Slug)
			}
			l.Op = prov.Confirm
		}
		add = append(add, l)
	}
	if len(add) == 0 {
		fmt.Printf("%s has nothing in quarantine\n", n.Slug)
		return nil
	}
	if ps := prov.Check(add); len(ps) > 0 {
		return usageError{ps[0].Msg}
	}
	if err := prov.Append(*root, n.Tag, add); err != nil {
		return err
	}
	for _, l := range add {
		fmt.Printf("%s %s %s\n", l.Op, n.Slug, l.File)
	}
	if *clear {
		fmt.Println("the next site build shows it again")
	} else {
		fmt.Println("the next site build hides it; commit prov/" + n.Tag + ".jsonl and deploy")
	}
	return nil
}
