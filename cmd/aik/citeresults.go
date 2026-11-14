// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/tamnd/ai-agent/card"
	"github.com/tamnd/ai-agent/cite"
	"github.com/tamnd/ai-agent/results"
)

// citeResults is the citation check of the result cards whose source is a work (spec 10.6.2, 07.5): aik cite results.
// A leaderboard row is checked against its snapshot when it is ingested; a number from a paper is checked here,
// against the text of the cited version of the paper at the card's locator. The card gets checks.cite found when a
// number of that passage rounds to the score, and not-found when none does, so that rule M08 keeps it out of the
// tables.
func citeResults(args []string) error {
	f := newFlags("cite results", "[flags]")
	root := f.String("root", ".", "the root of the content repository")
	offline := f.Bool("offline", false, "do not fetch; use only the snapshots in the store")
	store := f.String("store", "", "the raw store; the default is $AIK_RAW or the user cache")
	redo := f.Bool("redo", false, "check again the cards that have checks.cite")
	dry := f.Bool("n", false, "report and write nothing")
	if err := parse(f, args); err != nil {
		return err
	}
	if f.NArg() > 0 {
		return usageError{"cite results takes no arguments"}
	}
	c, err := load(*root)
	if err != nil {
		return err
	}
	ix, err := cite.Load(*root)
	if err != nil {
		return err
	}
	ck := &citer{c: c, ix: ix, store: cite.DefaultStore(), offline: *offline, docs: map[string]*cite.Doc{}, prev: map[string]*cite.Doc{}, failed: map[string]string{}}
	if *store != "" {
		ck.store = cite.Store{Dir: *store}
	}
	works := map[string]string{} // tag or slug of a work card -> its slug
	for _, cd := range c.Cards {
		if cd.Kind == "work" && cd.Slug != "" {
			works[cd.Slug] = cd.Slug
			if cd.Tag != "" {
				works[cd.Tag] = cd.Slug
			}
		}
	}
	dir := filepath.Join(c.Root, "cards", "result")
	lines, err := card.ReadKind(dir)
	if err != nil {
		return err
	}
	ctx := context.Background()
	counts := map[string]int{}
	changed := false
	for _, l := range lines {
		var r struct {
			Score  results.Score  `json:"score"`
			Source results.Source `json:"source"`
			Checks struct {
				Cite string `json:"cite"`
			} `json:"checks"`
		}
		if err := json.Unmarshal(mustRaw(l.Card.Fields), &r); err != nil {
			return fmt.Errorf("the result %s: %w", l.Card.Tag, err)
		}
		slug, ok := works[r.Source.Src]
		if !ok || r.Checks.Cite != "" && !*redo {
			continue
		}
		state, why := ck.result(ctx, slug, r.Source.Loc, r.Score)
		counts[state]++
		fmt.Printf("%-9s %s  %s, %s", state, l.Card.Tag, slug, r.Source.Loc)
		if why != "" {
			fmt.Printf(": %s", why)
		}
		fmt.Println()
		if state != results.CiteFound && state != results.CiteNotFound || state == r.Checks.Cite {
			continue
		}
		checks := map[string]json.RawMessage{}
		if raw, ok := l.Card.Fields["checks"]; ok {
			if err := json.Unmarshal(raw, &checks); err != nil {
				return fmt.Errorf("the result %s: checks: %w", l.Card.Tag, err)
			}
		}
		checks["cite"] = mustMarshal(state)
		if err := l.Card.Set("checks", checks); err != nil {
			return err
		}
		l.Changed, changed = true, true
	}
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s %d", k, counts[k]))
	}
	if len(parts) == 0 {
		parts = []string{"no result card cites a work"}
	}
	fmt.Println("citation check of the results: " + strings.Join(parts, ", "))
	if *dry {
		return nil
	}
	if changed {
		if err := card.WriteKind(dir, lines, card.MaxShard); err != nil {
			return err
		}
	}
	return cite.WriteManifest(*root, ix.Snapshots)
}

// result checks one score against the passage that the locator names in the cited version of a work. The state is
// found, not-found, or unchecked when the text could not be read; why says what was missing.
func (ck *citer) result(ctx context.Context, slug, loc string, s results.Score) (state, why string) {
	if s.V == nil {
		return results.CiteNotFound, "the card has no score"
	}
	w, why := ck.work(slug)
	if why != "" {
		return "unchecked", why
	}
	_, d, why := ck.doc(ctx, w)
	if why != "" {
		return "unchecked", why
	}
	refs, err := cite.ParseLocator(loc)
	if err != nil || len(refs) == 0 {
		return results.CiteNotFound, fmt.Sprintf("the locator %q is not in the grammar of 13.6.1", loc)
	}
	spans, fails := cite.Resolve(d, refs)
	if len(spans) == 0 {
		return results.CiteNotFound, "the locator names no place of the text: " + strings.Join(fails, "; ")
	}
	passage, _ := cite.Passage(d, spans)
	in := cite.Numbers(passage)
	for _, n := range scoreNumbers(*s.V, s.Unit) {
		if cite.Found(n, in) {
			return results.CiteFound, ""
		}
	}
	return results.CiteNotFound, fmt.Sprintf("no number of the passage rounds to %g", *s.V)
}

// scoreNumbers gives the forms in which a source may print a score, at the precision of the card: a fraction 0.873
// is 0.873 or 87.3 (percent).
func scoreNumbers(v float64, unit string) []cite.Number {
	text := strconv.FormatFloat(v, 'f', -1, 64)
	out := cite.Numbers(text)
	if unit == "fraction" {
		dec := 0
		if i := strings.IndexByte(text, '.'); i >= 0 {
			dec = len(text) - i - 1
		}
		p := max(0, dec-2)
		pct := strconv.FormatFloat(math.Round(v*100*math.Pow(10, float64(p)))/math.Pow(10, float64(p)), 'f', p, 64)
		out = append(out, cite.Numbers(pct)...)
	}
	return out
}

func mustRaw(fields map[string]json.RawMessage) []byte {
	b, _ := json.Marshal(fields)
	return b
}

func mustMarshal(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
