// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/tamnd/ai-agent/card"
	"github.com/tamnd/ai-agent/coverage"
	"github.com/tamnd/ai-agent/node"
	"github.com/tamnd/ai-agent/sample"
)

// emptyTree is the hash of the empty git tree, the base when the repository has no commit before the week.
const emptyTree = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"

// cmdSample draws the weekly sample of spec 15.6 from the pages that changed in an ISO week.
func cmdSample(args []string) error {
	f := newFlags("sample", "[flags]")
	root := f.String("root", ".", "the root of the content repository, a git work tree")
	week := f.String("week", "", "the ISO week, for example 2027-W14; the last whole week when not given")
	asJSON := f.Bool("json", false, "print the sample as JSON")
	if err := parse(f, args); err != nil {
		return err
	}
	if f.NArg() > 0 {
		return usageError{"aik sample takes no arguments"}
	}
	if *week == "" {
		*week = sample.WeekOf(time.Now().UTC().AddDate(0, 0, -7))
	}
	from, to, err := sample.Week(*week)
	if err != nil {
		return usageError{err.Error()}
	}
	c, err := load(*root)
	if err != nil {
		return err
	}
	pages, err := samplePages(*root, c, from, to)
	if err != nil {
		return err
	}
	r, err := sample.Draw(*week, pages)
	if err != nil {
		return err
	}
	if *asJSON {
		return printJSON(r)
	}
	fmt.Printf("sample of %s (%s to %s): %d pages, %d changed, %d new cards; %d taken\n", r.Week, r.From, r.To, r.Pages, r.Changed, r.New, len(r.Picks))
	for _, n := range r.Counts {
		fmt.Printf("  %-13s %5.0f%%  %d of %d\n", n.Stratum, n.Rate*100, n.Took, n.Pages)
	}
	for _, p := range r.Picks {
		also := ""
		if len(p.Also) > 0 {
			also = " (also " + strings.Join(p.Also, ", ") + ")"
		}
		src := ""
		if p.Source != "" {
			src = " from " + p.Source
		}
		fmt.Printf("  %-13s %s %s %s%s%s  %s\n", p.Stratum, p.Form, p.Kind, p.ID, src, also, p.Path)
	}
	fmt.Printf("not computed: %s\n", strings.Join(r.NotComputed, "; "))
	return nil
}

// samplePages gives every node and card of the corpus, marked with what the strata read: core, gold, contested,
// people, the release date of a model, and whether it changed or is new in the week [from, to).
func samplePages(root string, c *node.Corpus, from, to time.Time) ([]sample.Page, error) {
	base, err := revBefore(root, from)
	if err != nil {
		return nil, err
	}
	head, err := revBefore(root, to)
	if err != nil {
		return nil, err
	}
	var files []string
	cardDiff := ""
	if head != "" {
		if base == "" {
			base = emptyTree
		}
		out, err := git(root, "diff", "--name-only", base, head)
		if err != nil {
			return nil, err
		}
		for _, p := range strings.Split(out, "\n") {
			if p = strings.TrimSpace(p); p != "" {
				files = append(files, p)
			}
		}
		if cardDiff, err = git(root, "diff", "-U0", "--no-color", base, head, "--", card.Dir); err != nil {
			return nil, err
		}
	}
	changedCards, newCards := sample.CardChanges(cardDiff)

	rows, err := coverage.LoadUniverse(root)
	if err != nil {
		return nil, err
	}
	core := coverage.CoreIDs(rows)
	gold, err := loadGold(root)
	if err != nil {
		return nil, err
	}

	var pages []sample.Page
	for _, n := range c.Nodes {
		p := sample.Page{ID: n.Slug, Form: "node", Kind: n.Kind, Path: n.Rel, Core: core[n.Slug] || core[n.Tag],
			Gold: gold[n.Slug], People: n.Kind == "person" || len(n.People) > 0,
			Contested: n.Kind == "open-problem" || n.Kind == "argument"}
		if _, err := os.Stat(filepath.Join(c.Root, n.Rel, "positions.yaml")); err == nil {
			p.Contested = true
		}
		for _, f := range files {
			if strings.HasPrefix(f, n.Rel+"/") {
				p.Changed = true
				break
			}
		}
		pages = append(pages, p)
	}
	for _, cd := range c.Cards {
		if cd.Kind == "bridge" {
			continue
		}
		p := sample.Page{ID: cd.Key(), Form: "card", Kind: cd.Kind, Path: fmt.Sprintf("%s:%d", cd.File, cd.Line),
			Core: core[cd.Key()] || core[cd.Tag], Gold: gold[cd.Key()], People: cd.Kind == "person",
			Changed: changedCards[cd.Tag], New: newCards[cd.Tag]}
		if p.New && len(cd.Harvest) > 0 {
			p.Source = cd.Harvest[0].Source
		}
		for _, fc := range cd.Facts {
			if fc.ID == "released" {
				if s, ok := fc.V.(string); ok {
					p.Released = releaseDate(s)
				}
			}
		}
		pages = append(pages, p)
	}
	return pages, nil
}

// releaseDate reads a date of the form 2025-03-28, 2025-03 or 2025; a month or a year is taken at its first day.
func releaseDate(s string) time.Time {
	for _, l := range []string{"2006-01-02", "2006-01", "2006"} {
		if t, err := time.Parse(l, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// loadGold reads the slugs of registers/gold.yaml (06.12): a map from a kind to a slug or a list of slugs. It gives an
// empty set when the register does not exist.
func loadGold(root string) (map[string]bool, error) {
	b, err := os.ReadFile(filepath.Join(root, "registers", "gold.yaml"))
	if errors.Is(err, os.ErrNotExist) {
		return map[string]bool{}, nil
	}
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := yaml.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("registers/gold.yaml: %w", err)
	}
	gold := map[string]bool{}
	for kind, v := range m {
		switch v := v.(type) {
		case string:
			gold[v] = true
		case []any:
			for _, s := range v {
				str, ok := s.(string)
				if !ok {
					return nil, fmt.Errorf("registers/gold.yaml: the kind %s has an entry that is not a slug", kind)
				}
				gold[str] = true
			}
		default:
			return nil, fmt.Errorf("registers/gold.yaml: the kind %s is not a slug or a list of slugs", kind)
		}
	}
	return gold, nil
}

// revBefore is the last commit of HEAD before a time, or "" when there is none.
func revBefore(root string, t time.Time) (string, error) {
	out, err := git(root, "rev-list", "-1", "--before="+t.UTC().Format(time.RFC3339), "HEAD")
	return strings.TrimSpace(out), err
}

func git(root string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(ee.Stderr)))
		}
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return string(out), nil
}
