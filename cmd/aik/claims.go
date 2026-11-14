// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/tamnd/ai-agent/check"
	"github.com/tamnd/ai-agent/claims"
	"github.com/tamnd/ai-agent/node"
)

// extractReport is what aik claims extract finds in one node.
type extractReport struct {
	Slug       string            `json:"slug"`
	Sentences  int               `json:"sentences"`
	Classes    map[string]int    `json:"classes"`
	Anchored   int               `json:"anchored"`
	Candidates []claims.Sentence `json:"candidates"`
	Unanchored []string          `json:"unanchored_claims"`
	Findings   []check.Finding   `json:"findings"`
	Out        string            `json:"out"`
}

// cmdClaims runs the cheap part of the extraction pass of 07.6: it splits the prose of a node into sentences, classes
// them, matches the anchors, and finds the phrases of K04 and K05. The sentences go to build/claims/<slug>.jsonl for
// the model part, which matches each claim sentence to a claim by meaning before the audit.
func cmdClaims(args []string) error {
	if len(args) == 0 || args[0] != "extract" {
		return usageError{"use aik claims extract <slug>..."}
	}
	f := newFlags("claims extract", "[flags] <slug>... | -all")
	root := f.String("root", ".", "the root of the content repository")
	out := f.String("out", "build/claims", "the directory of the sentence files")
	asJSON := f.Bool("json", false, "print the reports as JSON")
	strict := f.Bool("strict", false, "fail on a K02 candidate too: an unanchored claim sentence with a year, a citation, a number or a reporting verb")
	all := f.Bool("all", false, "extract every node")
	show := f.Int("show", 20, "print at most this many K02 candidates of a node; -1 prints all")
	if err := parse(f, args[1:]); err != nil {
		return err
	}
	if f.NArg() == 0 && !*all || f.NArg() > 0 && *all {
		return usageError{"give the slug of at least one node, or -all"}
	}
	c, err := load(*root)
	if err != nil {
		return err
	}
	slugs := f.Args()
	if *all {
		for _, n := range c.Nodes {
			slugs = append(slugs, n.Slug)
		}
		slices.Sort(slugs)
	}
	dir := under(*root, *out)
	var reps []extractReport
	failed := false
	for _, slug := range slugs {
		n := c.BySlug[slug]
		if n == nil {
			return usageError{fmt.Sprintf("no node has the slug %q", slug)}
		}
		rep, err := extract(c, n, dir)
		if err != nil {
			return err
		}
		if len(rep.Findings) > 0 || *strict && len(rep.Candidates) > 0 {
			failed = true
		}
		reps = append(reps, rep)
	}
	if *asJSON {
		if err := printJSON(reps); err != nil {
			return err
		}
	} else {
		for _, r := range reps {
			printExtract(r, *show)
		}
	}
	if failed {
		return errFindings
	}
	return nil
}

// extract writes the sentence file of a node and reports on it.
func extract(c *node.Corpus, n *node.Node, dir string) (extractReport, error) {
	ss := claims.Extract(n.Body, n.BodyLine)
	rep := extractReport{Slug: n.Slug, Sentences: len(ss), Classes: map[string]int{}, Candidates: []claims.Sentence{}, Unanchored: []string{}, Findings: []check.Finding{}}
	anchored := map[string]bool{}
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	for _, s := range ss {
		rep.Classes[s.Class]++
		if len(s.Anchors) > 0 {
			rep.Anchored++
		}
		for _, a := range s.Anchors {
			anchored[a] = true
		}
		if s.Candidate() {
			rep.Candidates = append(rep.Candidates, s)
		}
		if err := enc.Encode(s); err != nil {
			return rep, err
		}
	}
	for _, cl := range n.Claims {
		if !anchored[cl.ID] && cl.SupersededBy == "" {
			rep.Unanchored = append(rep.Unanchored, cl.ID)
		}
	}
	slices.Sort(rep.Unanchored)
	rep.Out = filepath.Join(dir, n.Slug+".jsonl")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return rep, err
	}
	if err := os.WriteFile(rep.Out, b.Bytes(), 0o644); err != nil {
		return rep, err
	}
	// The anchor match is rule K01, and the forbidden phrases are K04 and K05.
	for _, x := range check.Run(c, check.Options{Only: map[string]bool{n.Slug: true}}) {
		if (x.Rule == "K01" || x.Rule == "K04" || x.Rule == "K05") && strings.HasPrefix(x.Path, n.Rel+"/") {
			rep.Findings = append(rep.Findings, x)
		}
	}
	return rep, nil
}

func printExtract(r extractReport, show int) {
	var cls []string
	for _, k := range []string{claims.ClassClaim, claims.ClassDefinition, claims.ClassStructure, claims.ClassQuote} {
		if r.Classes[k] > 0 {
			cls = append(cls, fmt.Sprintf("%d %s", r.Classes[k], k))
		}
	}
	fmt.Printf("%s: %d sentences (%s), %d anchored; wrote %s\n", r.Slug, r.Sentences, strings.Join(cls, ", "), r.Anchored, r.Out)
	for _, x := range r.Findings {
		fmt.Printf("  %s %s:%d %s\n", x.Rule, x.Path, x.Line, x.Msg)
	}
	if len(r.Unanchored) > 0 {
		fmt.Printf("  claims with no anchor in the prose: %s\n", strings.Join(r.Unanchored, ", "))
	}
	if len(r.Candidates) > 0 {
		fmt.Printf("  %d K02 candidates, unanchored claim sentences with a mark of a claim about the world; anchor each, or the model pass decides it:\n", len(r.Candidates))
		for i, s := range r.Candidates {
			if show >= 0 && i >= show {
				fmt.Printf("    ... %d more; -show -1 prints all\n", len(r.Candidates)-i)
				break
			}
			t := claims.Plain(s.Text)
			if len(t) > 110 {
				t = t[:107] + "..."
			}
			fmt.Printf("    %d [%s] %s\n", s.Line, strings.Join(s.Signals, ","), t)
		}
	}
}
