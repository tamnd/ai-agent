// SPDX-License-Identifier: Apache-2.0

package check

import (
	"errors"
	"path"
	"slices"

	"github.com/tamnd/ai-agent/card"
	"github.com/tamnd/ai-agent/facts"
	"github.com/tamnd/ai-agent/node"
)

// cards checks the cards of cards/ (spec 04.6 and 20.3).
func (r *run) cards() {
	for _, err := range r.c.CardErrors {
		var e *card.Error
		if errors.As(err, &e) {
			r.add("T01", Block, e.File, e.Line, "the line is not a card: %v", e.Err)
			continue
		}
		r.add("T01", Block, card.Dir, 0, "%v", err)
	}
	reg := map[string]string{}
	for _, t := range r.c.Tags {
		reg[t.Tag] = t.Slug
	}
	nodeTags := map[string]string{}
	for _, n := range r.c.Nodes {
		nodeTags[n.Tag] = n.Rel
	}
	seenTag := map[string]*card.Card{}
	seenKey := map[string]*card.Card{}
	lastTag := map[string]string{}
	unsorted := map[string]bool{}
	for _, c := range r.c.Cards {
		r.card(c, reg)
		if c.Tag != "" {
			if o := seenTag[c.Tag]; o != nil {
				r.add("T02", Block, c.File, c.Line, "the tag %s is also the tag of the card on %s:%d", c.Tag, o.File, o.Line)
			} else if rel, ok := nodeTags[c.Tag]; ok {
				r.add("T12", Block, c.File, c.Line, "the tag %s is also the tag of the node %s; a promoted card leaves cards/ (4.1)", c.Tag, rel)
			}
			seenTag[c.Tag] = c
			if c.Tag < lastTag[c.File] && !unsorted[c.File] {
				unsorted[c.File] = true
				r.add("T11", Block, c.File, c.Line, "the lines are not sorted by tag; %s comes after %s (20.3)", c.Tag, lastTag[c.File])
			}
			lastTag[c.File] = c.Tag
		}
		if c.Slug == "" {
			continue
		}
		if o := seenKey[c.Key()]; o != nil {
			r.add("T03", Block, c.File, c.Line, "the slug %q is also the slug of the card on %s:%d", c.Key(), o.File, o.Line)
		} else if n := r.c.BySlug[c.Slug]; n != nil && c.Owner == "" {
			r.add("T03", Block, c.File, c.Line, "the slug %q is also the slug of the node %s", c.Slug, n.Rel)
		}
		seenKey[c.Key()] = c
	}
}

func (r *run) card(c *card.Card, reg map[string]string) {
	p, l := c.File, c.Line
	for _, f := range []struct{ name, v string }{{"tag", c.Tag}, {"kind", c.Kind}, {"slug", c.Slug}, {"name", c.Name}} {
		if f.v == "" {
			r.add("T01", Block, p, l, "the card %s has no %s", c.Slug, f.name)
		}
	}
	if dir := path.Base(path.Dir(c.File)); c.Kind != "" {
		switch {
		case !node.IsKind(c.Kind):
			r.add("T01", Block, p, l, "the card %s has the kind %q, which is not a kind of 04.2", c.Slug, c.Kind)
		case dir != c.Kind:
			r.add("T01", Block, p, l, "the card %s is a %s, but it is in cards/%s/", c.Slug, c.Kind, dir)
		}
	}
	switch {
	case c.Tag == "":
	case provRE.MatchString(c.Tag):
		if r.opt.Main {
			r.add("T02", Block, p, l, "the provisional id %s is on main; run aik mint", c.Tag)
		}
	case !tagRE.MatchString(c.Tag):
		r.add("T02", Block, p, l, "the tag %q is not 6 characters of Crockford base32", c.Tag)
	default:
		if s, ok := reg[c.Tag]; !ok {
			r.add("T02", Block, p, l, "the tag %s is not in registers/tags.reg", c.Tag)
		} else if s != c.Key() {
			r.add("T02", Block, p, l, "registers/tags.reg gives the tag %s to %q, not %q", c.Tag, s, c.Key())
		}
		if sh := card.Shard(c.File); !card.InShard(c.Tag, sh) {
			r.add("T11", Block, p, l, "the tag %s is not in the shard %s; it starts with another prefix (20.3)", c.Tag, sh)
		}
	}
	if c.Slug != "" && (!slugRE.MatchString(c.Slug) || len(c.Slug) > 60) {
		r.add("T03", Block, p, l, "the slug %q is not lowercase kebab-case of at most 60 characters", c.Slug)
	}
	if c.Owner != "" && r.c.BySlug[c.Owner] == nil && r.c.CardByKey[c.Owner] == nil {
		r.add("E01", Block, p, l, "owner: no node or card has the slug %q", c.Owner)
	}
	if w := card.Words(c.Summary); w > card.MaxSummary {
		r.add("S10", Block, p, l, "the summary of the card %s has %d words; a card summary has at most %d (4.6)", c.Slug, w, card.MaxSummary)
	}
	for _, b := range c.Bad {
		r.add("T01", Block, p, l, "the card %s: %s", c.Slug, b)
	}
	for _, f := range c.Facts {
		if f.V == nil {
			r.add("D01", Block, p, l, "the fact %s of the card %s has no value", f.ID, c.Slug)
		}
		if f.Src == "" {
			r.add("D01", Block, p, l, "the fact %s of the card %s has no src", f.ID, c.Slug)
		}
	}
	for i, h := range c.Harvest {
		if h.Source == "" {
			r.add("T01", Block, p, l, "the harvest record %d of the card %s names no source", i+1, c.Slug)
		}
		if h.Date != "" && !facts.IsDate(h.Date) {
			r.add("T01", Block, p, l, "the harvest record %d of the card %s has the date %q; write a date such as 2026-11-13", i+1, c.Slug, h.Date)
		}
	}
	if c.Kind == "model" && c.Role != "" && !slices.Contains([]string{"family", "release", "checkpoint"}, c.Role) {
		r.add("T01", Block, p, l, "the model card %s has the role %q; write family, release or checkpoint", c.Slug, c.Role)
	}
}
