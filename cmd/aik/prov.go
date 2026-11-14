// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/tamnd/ai-agent/node"
	"github.com/tamnd/ai-agent/prov"
)

// cmdProv reads, checks and appends the provenance logs of the nodes (spec 04.8).
func cmdProv(args []string) error {
	if len(args) == 0 {
		return usageError{"give show <slug>, check [<slug>...] or record -author <login> <slug>..."}
	}
	switch args[0] {
	case "show":
		return provShow(args[1:])
	case "check":
		return provCheck(args[1:])
	case "record":
		return provRecord(args[1:])
	case "-h", "--help", "help":
		fmt.Println("Usage: aik prov show <slug|tag> | check [-strict] [<slug>...] | record -author <login> [flags] <slug>...")
		return nil
	}
	return usageError{fmt.Sprintf("%q is not show, check or record", args[0])}
}

// provNode finds a node by its slug or its tag.
func provNode(c *node.Corpus, s string) (*node.Node, error) {
	if n := c.BySlug[s]; n != nil {
		return n, nil
	}
	if n := c.ByTag[s]; n != nil {
		return n, nil
	}
	return nil, fmt.Errorf("no node has the slug or the tag %q", s)
}

func provShow(args []string) error {
	f := newFlags("prov show", "<slug|tag>")
	root := f.String("root", ".", "the root of the content repository")
	asJSON := f.Bool("json", false, "print the lines and the states of the files as JSON")
	if err := parse(f, args); err != nil {
		return err
	}
	if f.NArg() != 1 {
		return usageError{"give one slug or tag"}
	}
	c, err := load(*root)
	if err != nil {
		return err
	}
	n, err := provNode(c, f.Arg(0))
	if err != nil {
		return err
	}
	ls, err := prov.Read(*root, n.Tag)
	if err != nil {
		return err
	}
	files, err := prov.Files(n.Dir)
	if err != nil {
		return err
	}
	states := prov.States(files, ls)
	if *asJSON {
		return printJSON(map[string]any{"slug": n.Slug, "tag": n.Tag, "lines": ls, "files": states})
	}
	if ls == nil {
		fmt.Printf("%s (%s) has no provenance log: %s/%s.jsonl\n", n.Slug, n.Tag, prov.Dir, n.Tag)
	} else {
		fmt.Printf("%s (%s): %d lines, the newest first\n", n.Slug, n.Tag, len(ls))
	}
	for i := len(ls) - 1; i >= 0; i-- {
		l := ls[i]
		fmt.Printf("  %s %-10s %-24s %s", l.Date, l.Op, l.File, l.Author)
		if l.Audit != "" {
			fmt.Printf("; audit %s", l.Audit)
		}
		if l.Reviewer != nil && *l.Reviewer != "" {
			fmt.Printf("; reviewer %s", *l.Reviewer)
		}
		if l.Note != "" {
			fmt.Printf("; %s", l.Note)
		}
		fmt.Println()
	}
	count := map[string]int{}
	for _, s := range states {
		count[s.State]++
	}
	fmt.Printf("files: %s\n", stateCounts(count))
	for _, s := range states {
		if s.State != prov.Recorded {
			fmt.Printf("  %-10s %s\n", s.State, s.File)
		}
	}
	return nil
}

func stateCounts(m map[string]int) string {
	var parts []string
	for _, k := range []string{prov.Recorded, prov.Changed, prov.Created, prov.Unrecorded, prov.Removed} {
		parts = append(parts, fmt.Sprintf("%d %s", m[k], k))
	}
	return strings.Join(parts, ", ")
}

// provReport is the check of the log of one node.
type provReport struct {
	Slug     string         `json:"slug"`
	Tag      string         `json:"tag"`
	NoLog    bool           `json:"no_log,omitempty"`
	Problems []prov.Problem `json:"problems,omitempty"`
	Files    map[string]int `json:"files"` // the number of files in each state
	Pending  []string       `json:"pending,omitempty"`
}

func provCheck(args []string) error {
	f := newFlags("prov check", "[-strict] [<slug>...]")
	root := f.String("root", ".", "the root of the content repository")
	strict := f.Bool("strict", false, "fail also on a node with no log, and on a file whose change is not recorded")
	asJSON := f.Bool("json", false, "print the report of each node as JSON")
	if err := parse(f, args); err != nil {
		return err
	}
	c, err := load(*root)
	if err != nil {
		return err
	}
	nodes := c.Nodes
	if f.NArg() > 0 {
		nodes = nil
		for _, s := range f.Args() {
			n, err := provNode(c, s)
			if err != nil {
				return err
			}
			nodes = append(nodes, n)
		}
	}
	var reps []provReport
	bad, noLog, pending := 0, 0, 0
	total := map[string]int{}
	for _, n := range nodes {
		if n.Tag == "" || strings.HasPrefix(n.Tag, "NEW-") {
			continue // aik mint makes the log with the tag
		}
		ls, err := prov.Read(*root, n.Tag)
		if err != nil {
			return err
		}
		files, err := prov.Files(n.Dir)
		if err != nil {
			return err
		}
		r := provReport{Slug: n.Slug, Tag: n.Tag, NoLog: ls == nil, Problems: prov.Check(ls), Files: map[string]int{}}
		for _, s := range prov.States(files, ls) {
			r.Files[s.State]++
			total[s.State]++
			if s.State == prov.Changed || s.State == prov.Unrecorded || s.State == prov.Removed {
				r.Pending = append(r.Pending, s.State+" "+s.File)
			}
		}
		bad += len(r.Problems)
		pending += len(r.Pending)
		if r.NoLog {
			noLog++
		}
		reps = append(reps, r)
	}
	failed := bad > 0 || *strict && (noLog > 0 || pending > 0)
	if *asJSON {
		if err := printJSON(reps); err != nil {
			return err
		}
	} else {
		for _, r := range reps {
			if show := len(r.Problems) > 0 || *strict && (r.NoLog || len(r.Pending) > 0); !show {
				continue
			}
			fmt.Printf("%s (%s/%s.jsonl)\n", r.Slug, prov.Dir, r.Tag)
			if r.NoLog {
				fmt.Println("  no log")
			}
			for _, p := range r.Problems {
				fmt.Printf("  line %d: %s\n", p.Line, p.Msg)
			}
			if *strict {
				for _, p := range r.Pending {
					fmt.Printf("  %s\n", p)
				}
			}
		}
		fmt.Printf("%d nodes, %d with no log, %d bad lines; files: %s\n", len(reps), noLog, bad, stateCounts(total))
	}
	if failed {
		return errFindings
	}
	return nil
}

// listFlag is a flag that can be given many times, or once with a comma list.
type listFlag []string

func (l *listFlag) String() string { return strings.Join(*l, ",") }

func (l *listFlag) Set(s string) error {
	for p := range strings.SplitSeq(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			*l = append(*l, p)
		}
	}
	return nil
}

func provRecord(args []string) error {
	f := newFlags("prov record", "-author <login> [flags] <slug>...")
	root := f.String("root", ".", "the root of the content repository")
	author := f.String("author", os.Getenv("GITHUB_ACTOR"), "the GitHub login of the person who made the changes; the default is $GITHUB_ACTOR")
	reviewer := f.String("reviewer", "", "the GitHub login of the reviewer, when there is one")
	note := f.String("note", "", "a short note on the change")
	day := f.String("date", time.Now().UTC().Format("2006-01-02"), "the date of the change")
	dry := f.Bool("n", false, "print the lines and do not write them")
	var sources listFlag
	f.Var(&sources, "source", "a source id that the change used, for example arxiv:1207.0580; repeat or give a comma list")
	if err := parse(f, args); err != nil {
		return err
	}
	if f.NArg() == 0 {
		return usageError{"give the slugs of the nodes"}
	}
	if *author == "" {
		return usageError{"give the GitHub login of the author with -author; the changes of agents are recorded by work/prov.py"}
	}
	tmpl := prov.Line{Date: *day, Author: prov.Author{Login: *author}, Note: *note, Sources: sources}
	if *reviewer != "" {
		tmpl.Reviewer = reviewer
	}
	if ps := prov.Check([]prov.Line{withHash(tmpl)}); len(ps) > 0 {
		return usageError{ps[0].Msg}
	}
	c, err := load(*root)
	if err != nil {
		return err
	}
	for _, s := range f.Args() {
		n, err := provNode(c, s)
		if err != nil {
			return err
		}
		if n.Tag == "" || strings.HasPrefix(n.Tag, "NEW-") {
			return fmt.Errorf("%s has no minted tag; run aik mint first", n.Slug)
		}
		ls, err := prov.Read(*root, n.Tag)
		if err != nil {
			return err
		}
		files, err := prov.Files(n.Dir)
		if err != nil {
			return err
		}
		add := prov.Changes(files, ls, tmpl)
		for _, l := range add {
			fmt.Printf("%s %s %s %s\n", n.Slug, l.Op, l.File, l.Hash)
		}
		if !*dry {
			if err := prov.Append(*root, n.Tag, add); err != nil {
				return err
			}
		}
		fmt.Printf("%s (%s): %d lines\n", n.Slug, n.Tag, len(add))
	}
	return nil
}

// withHash is a line with the fields that the check asks of an edit, to check the flags of record alone.
func withHash(l prov.Line) prov.Line {
	l.File, l.Op, l.Hash = "node.md", prov.Edit, prov.Hash(nil)
	return l
}
