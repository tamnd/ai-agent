// SPDX-License-Identifier: Apache-2.0

package site

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/tamnd/ai-agent/card"
	"github.com/tamnd/ai-agent/facts"
	"github.com/tamnd/ai-agent/node"
)

// collections are the catalogue routes of spec 16.2. A kind with no route has its cards on /kind/<kind>/.
var collections = map[string]string{
	"model": "/models/", "benchmark": "/benchmarks/", "dataset": "/datasets/", "work": "/works/",
	"incident": "/incidents/", "result": "/results/", "system": "/systems/", "software": "/software/",
	"hardware": "/hardware/", "standard": "/standards/", "policy": "/policy/", "risk": "/risks/",
	"person": "/people/", "organization": "/orgs/", "event": "/events/", "definition": "/glossary/",
	"exercise": "/exercises/",
}

// ownRoute are the kinds whose cards have a URL under the catalogue route, such as /works/<slug>/. The cards of the other kinds have a global slug, such as /<slug>/ (spec 16.2).
var ownRoute = map[string]bool{"model": true, "benchmark": true, "dataset": true, "work": true, "incident": true, "exercise": true}

// PageSize is the number of rows of one page of a catalogue (spec 16.6).
const PageSize = 200

// CardsPage is one page of a catalogue: the node pages of the kind, and a table of its cards.
type CardsPage struct {
	Base
	Heading string
	Lead    string
	Count   string
	AsOf    string
	Nodes   []Group
	Rows    []CardRow
	Pages   []Link // the links to the pages of the table, when there is more than one
	JSON    string // the path of the JSON file of the cards of the kind
}

// CardRow is one card in the table of its catalogue.
type CardRow struct {
	Anchor  string
	Name    string
	Tag     string
	Summary string
	Owner   string
	Facts   []CardFact
	Sources []string
}

// CardFact is one fact of a card row.
type CardFact struct {
	Label string
	Value string
	Src   string
	Loc   string
}

// collection gives the path of the catalogue of a kind.
func collection(kind string) string {
	if p, ok := collections[kind]; ok {
		return p
	}
	return "/kind/" + kind + "/"
}

// cardAnchor is the anchor of the row of a card on its catalogue.
func cardAnchor(c *card.Card) string {
	if c.Owner != "" {
		return c.Owner + "--" + c.Slug
	}
	return c.Slug
}

// cardURL is the path that a card owns (spec 16.2). It redirects to the row of the card. A result card has no path of its own.
func cardURL(c *card.Card) string {
	switch {
	case c.Kind == "result":
		return ""
	case c.Owner != "" && ownRoute[c.Kind]:
		// A release under its family, a benchmark version under its benchmark, an exercise under its site:
		// /models/llama/llama-3-1-405b/, /benchmarks/mmlu/pro/, /exercises/gymnasium/frozen-lake-v1/.
		return collection(c.Kind) + c.Owner + "/" + c.Slug + "/"
	case c.Owner != "":
		// A provision under its policy: /eu-ai-act/article-50/.
		return "/" + c.Owner + "/" + c.Slug + "/"
	case ownRoute[c.Kind]:
		return collection(c.Kind) + c.Slug + "/"
	}
	return "/" + c.Slug + "/"
}

// cardKinds gives the cards of each kind, sorted by name. The map is shared: callers must not change it.
func (b *builder) cardKinds() map[string][]*card.Card {
	if b.cards != nil {
		return b.cards
	}
	out := map[string][]*card.Card{}
	for _, c := range b.c.Cards {
		if node.IsKind(c.Kind) && c.Slug != "" {
			out[c.Kind] = append(out[c.Kind], c)
		}
	}
	for _, cs := range out {
		sort.SliceStable(cs, func(i, j int) bool {
			if a, b := strings.ToLower(cs[i].Name), strings.ToLower(cs[j].Name); a != b {
				return a < b
			}
			return cs[i].Tag < cs[j].Tag
		})
	}
	b.cards = out
	return out
}

// cardLink gives the link of a card: the row of the card on its catalogue, the only page that a card has (spec 16.2).
func (b *builder) cardLink(c *card.Card) string {
	return b.cardPage(b.cardKinds(), c)
}

func cardRow(c *card.Card) CardRow {
	r := CardRow{Anchor: cardAnchor(c), Name: c.Name, Tag: c.Tag, Summary: c.Summary, Owner: c.Owner}
	seen := map[string]bool{}
	for _, f := range c.Facts {
		r.Facts = append(r.Facts, CardFact{Label: facts.Label(f.ID), Value: facts.Format(f.V, facts.CardUnits[f.ID]), Src: f.Src, Loc: f.Loc})
		if f.Src != "" && !seen[f.Src] {
			seen[f.Src] = true
			r.Sources = append(r.Sources, f.Src)
		}
	}
	return r
}

// collections writes the catalogue of each kind that has cards, in pages of PageSize rows, and the JSON file of its cards.
// A kind with a catalogue route and node pages but no cards gets a catalogue too, so that the links to its route, such
// as the crumbs of a results table, point at a page.
func (b *builder) collections() error {
	kinds := map[string][]*card.Card{}
	for k, cs := range b.cardKinds() {
		kinds[k] = cs
	}
	for kind := range collections {
		if _, ok := kinds[kind]; !ok && len(b.c.Kind(kind)) > 0 {
			kinds[kind] = nil
		}
	}
	for kind, cs := range kinds {
		base := collection(kind)
		name := title(kindName(kind, true))
		ns := b.c.Kind(kind)
		sortNodes(ns)
		pages := max(1, (len(cs)+PageSize-1)/PageSize)
		jsonPath := "/api/cards/" + kind + ".json"
		for i := range pages {
			p := base
			if i > 0 {
				p = fmt.Sprintf("%spage/%d/", base, i+1)
			}
			cp := CardsPage{Base: b.base(name, kindIntro[kind], p), Heading: name, Lead: kindIntro[kind], AsOf: b.meta.Date, JSON: jsonPath}
			cp.Count = fmt.Sprintf("%d %s, %d %s", len(cs), plural(len(cs), "card", "cards"), len(ns), plural(len(ns), "page", "pages"))
			if i == 0 && len(ns) > 0 {
				cp.Nodes = []Group{{ID: "pages", Title: "Pages", Items: b.items(ns)}}
			}
			for _, c := range cs[i*PageSize : min(len(cs), (i+1)*PageSize)] {
				cp.Rows = append(cp.Rows, cardRow(c))
			}
			if pages > 1 {
				for j := range pages {
					u := base
					if j > 0 {
						u = fmt.Sprintf("%spage/%d/", base, j+1)
					}
					cp.Pages = append(cp.Pages, Link{Name: fmt.Sprint(j + 1), URL: u})
				}
			}
			if err := b.page("cards", p, cp); err != nil {
				return err
			}
		}
		recs := make([]map[string]json.RawMessage, 0, len(cs))
		for _, c := range cs {
			recs = append(recs, c.Fields)
		}
		data, err := json.Marshal(recs)
		if err != nil {
			return err
		}
		if err := b.write(jsonPath, data); err != nil {
			return err
		}
	}
	return nil
}

// cardPage gives the path of the catalogue page that holds the row of the card, with its anchor.
func (b *builder) cardPage(kinds map[string][]*card.Card, c *card.Card) string {
	i := 0
	for j, o := range kinds[c.Kind] {
		if o == c {
			i = j
			break
		}
	}
	p := collection(c.Kind)
	if n := i / PageSize; n > 0 {
		p = fmt.Sprintf("%spage/%d/", p, n+1)
	}
	return p + "#" + cardAnchor(c)
}

// cardMoves adds the paths of the cards and their tags to the map of moved paths, so that the 404 page sends the reader to the row of the card (spec 16.2).
func (b *builder) cardMoves(moved map[string]string) {
	kinds := b.cardKinds()
	for _, cs := range kinds {
		for _, c := range cs {
			to := b.cardPage(kinds, c)
			if c.Tag != "" {
				moved["/t/"+c.Tag] = to
			}
			u := cardURL(c)
			if u == "" {
				continue
			}
			if n := b.c.BySlug[c.Slug]; n != nil && u == "/"+c.Slug+"/" {
				continue
			}
			if k := strings.TrimSuffix(u, "/"); moved[k] == "" {
				moved[k] = to
			}
		}
	}
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
