// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/tamnd/ai-agent/card"
	"github.com/tamnd/ai-agent/check"
	"github.com/tamnd/ai-agent/facts"
	"github.com/tamnd/ai-agent/node"
	"github.com/tamnd/ai-agent/promote"
	"github.com/tamnd/ai-agent/universe"
)

// cmdPromote makes the node directory of a card, moves its facts to facts.yaml, removes its line from the shard and
// points its universe row at the node (spec 04.1). The tag does not change.
func cmdPromote(args []string) error {
	f := newFlags("promote", "[flags] <tag|slug|owner/slug>")
	root := f.String("root", ".", "the root of the content repository")
	area := f.String("area", "", "the area of the node, top/sub; the default is the area of the universe row of the card")
	level := f.String("level", "core", "the level: intro, core, advanced or research")
	dry := f.Bool("n", false, "print the node and the facts that the promotion would write, and change nothing")
	commit := f.Bool("commit", false, "commit the promotion: the node, the shard and the register in one commit")
	author := f.String("author", "", "the author of the provenance log; the default is $GITHUB_ACTOR or the git user name")
	if err := parse(f, args); err != nil {
		return err
	}
	if f.NArg() != 1 {
		return usageError{"give the tag, the slug or the owner/slug of one card"}
	}
	if !levelOK(*level) {
		return usageError{fmt.Sprintf("the level %q is not intro, core, advanced or research", *level)}
	}
	c, err := load(*root)
	if err != nil {
		return err
	}
	cd := findCard(c, f.Arg(0))
	if cd == nil {
		return fmt.Errorf("no card has the tag or the key %q", f.Arg(0))
	}
	if strings.HasPrefix(cd.File, "cards/bridge/") {
		return fmt.Errorf("%s is a bridge card: its item lives on a neighbour site (5.9)", cd.Tag)
	}
	if n := c.BySlug[cd.Slug]; n != nil {
		return fmt.Errorf("the slug %q is used by the node %s", cd.Slug, n.Rel)
	}
	if n := c.ByTag[cd.Tag]; n != nil {
		return fmt.Errorf("the tag %s is the tag of the node %s", cd.Tag, n.Rel)
	}
	items, err := universe.Read(*root)
	if err != nil {
		return err
	}
	var row *universe.Item
	for _, it := range items {
		if universe.TagOf(it.Target) == cd.Tag {
			row = it
			break
		}
	}
	if *area == "" && row != nil {
		*area = row.Area
	}
	if *area == "" {
		return usageError{"the card has no universe row with an area; give the area with -area"}
	}
	if _, ok := c.AreaByID[*area]; !ok {
		return usageError{fmt.Sprintf("the area %q is not in topics/areas.yaml", *area)}
	}
	p, err := promote.Make(cd, *area, *level, check.Order[cd.Kind])
	if err != nil {
		return err
	}
	ff, err := promote.FactFile(p.Facts)
	if err != nil {
		return err
	}
	dir := filepath.Join(*root, filepath.FromSlash(p.Rel))
	if *dry {
		fmt.Printf("%s/node.md:\n%s", p.Rel, p.Front)
		if len(ff) > 0 {
			fmt.Printf("\n%s/%s:\n%s", p.Rel, facts.File, ff)
		}
		printDropped(p)
		return nil
	}
	if _, err := os.Stat(dir); err == nil {
		return fmt.Errorf("%s exists", dir)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "node.md"), []byte(p.Front), 0o644); err != nil {
		return err
	}
	if len(ff) > 0 {
		if err := os.WriteFile(filepath.Join(dir, facts.File), ff, 0o644); err != nil {
			return err
		}
	}
	// The line leaves its shard.
	kindDir := filepath.Join(*root, filepath.Dir(filepath.FromSlash(cd.File)))
	lines, err := card.ReadKind(kindDir)
	if err != nil {
		return err
	}
	kept := lines[:0]
	for _, l := range lines {
		if l.Card.Tag != cd.Tag {
			kept = append(kept, l)
		}
	}
	if err := card.WriteKind(kindDir, kept, card.MaxShard); err != nil {
		return err
	}
	if row != nil && row.Outcome != "node" {
		row.Outcome = "node"
		if err := universe.Write(*root, items); err != nil {
			return err
		}
	}
	if *author == "" {
		*author = defaultAuthor(*root)
	}
	rec, err := json.Marshal(map[string]any{"date": time.Now().UTC().Format("2006-01-02"), "file": "*", "op": "create", "author": *author, "sources": nonNil(p.Sources), "note": "promoted from the card"})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(*root, "prov"), 0o755); err != nil {
		return err
	}
	if err := appendLines(filepath.Join(*root, "prov", cd.Tag+".jsonl"), string(rec)+"\n"); err != nil {
		return err
	}
	fmt.Printf("promoted %s %s to %s\n", cd.Tag, cd.Key(), p.Rel)
	printDropped(p)
	if *commit {
		return gitCommit(*root, "promote "+cd.Slug+" ("+cd.Tag+") from a card to a node (04.1)",
			p.Rel, filepath.ToSlash(filepath.Dir(cd.File)), universe.Dir, "prov/"+cd.Tag+".jsonl")
	}
	return nil
}

func levelOK(l string) bool {
	switch l {
	case "intro", "core", "advanced", "research":
		return true
	}
	return false
}

// findCard finds a card by its tag or its key.
func findCard(c *node.Corpus, id string) *card.Card {
	for _, cd := range c.Cards {
		if cd.Tag == id {
			return cd
		}
	}
	if cd := c.CardByKey[id]; cd != nil {
		return cd
	}
	for _, cd := range c.Cards {
		if strings.EqualFold(cd.Tag, id) {
			return cd
		}
	}
	return nil
}

func printDropped(p *promote.Plan) {
	for _, d := range p.Dropped {
		fmt.Printf("  not moved: %s\n", d)
	}
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// gitCommit stages paths and commits them in one commit.
func gitCommit(root, msg string, paths ...string) error {
	for _, argv := range [][]string{append([]string{"add", "-A", "--"}, paths...), {"commit", "-m", msg}} {
		cmd := exec.Command("git", argv...)
		cmd.Dir = root
		cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("git %s: %w", argv[0], err)
		}
	}
	return nil
}
