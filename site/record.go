// SPDX-License-Identifier: Apache-2.0

package site

import (
	"encoding/json"
	"slices"
	"sort"
	"strings"

	"github.com/tamnd/ai-agent/card"
	"github.com/tamnd/ai-agent/claims"
	"github.com/tamnd/ai-agent/facts"
	"github.com/tamnd/ai-agent/graph"
	"github.com/tamnd/ai-agent/node"
	"github.com/tamnd/ai-agent/results"
	"github.com/tamnd/ai-agent/status"
)

// LineageTypes are the edge types of the lineage views (16.9), in the order the views list them.
var LineageTypes = []string{
	"fine_tuned_from", "distilled_from", "merged_from", "member_of",
	"introduced_by", "variant_of", "generalizes", "uses", "responds_to", "components",
}

// Thing is a node or a card.
type Thing struct {
	Node *node.Node
	Card *card.Card
}

// Key is the slug of a node, or the key of a card.
func (it Thing) Key() string {
	if it.Node != nil {
		return it.Node.Slug
	}
	return it.Card.Key()
}

// Edge is one edge of the graph (04.7): From names To with a field of type Type. Known is false when To is not an
// item of the corpus; the lineage views show such an edge as unknown, never guessed (16.9).
type Edge struct {
	Type  string `json:"type"`
	From  string `json:"from"`
	To    string `json:"to"`
	Known bool   `json:"known"`
}

// Graph holds the edges of a corpus both ways and the result cards that name each item, for the JSON records (16.10)
// and the MCP server.
type Graph struct {
	c     *node.Corpus
	base  string
	reg   *results.Register
	out   map[string][]Edge // by the key of the item
	in    map[string][]Edge
	cites map[string][]string // the tags of the result cards that name the item
}

// NewGraph takes the edges of each node and card of a corpus from package graph. base is the URL of the site. The
// records and the MCP server show the edges as they are written, between items of the corpus; only the lineage keeps
// an end that names nothing, which its views mark as unknown (16.9). Neighbours have their own links.
func NewGraph(c *node.Corpus, base string) *Graph {
	g := &Graph{c: c, base: strings.TrimRight(base, "/"), reg: results.Load(c),
		out: map[string][]Edge{}, in: map[string][]Edge{}, cites: map[string][]string{}}
	for _, r := range g.reg.Results {
		for _, k := range []string{r.Model, r.Benchmark} {
			if k != "" && r.Tag != "" {
				g.cites[k] = append(g.cites[k], r.Tag)
			}
		}
	}
	gr := graph.Build(c)
	add := func(key string) {
		for _, e := range gr.Out(key) {
			if e.Derived || e.Site != "" || !e.Known && !slices.Contains(LineageTypes, e.Type) {
				continue
			}
			if _, ok := g.Ref(e.To); ok != e.Known {
				continue // a benchmark version, which has no page of its own
			}
			se := Edge{Type: e.Type, From: e.From, To: e.To, Known: e.Known}
			g.out[key] = append(g.out[key], se)
			if se.Known {
				g.in[se.To] = append(g.in[se.To], se)
			}
		}
	}
	for _, n := range c.Nodes {
		add(n.Slug)
	}
	for _, cd := range c.Cards {
		add(cd.Key())
	}
	for k := range g.in {
		sort.SliceStable(g.in[k], func(i, j int) bool { return g.in[k][i].From < g.in[k][j].From })
	}
	return g
}

// Results is the results register of the corpus.
func (g *Graph) Results() *results.Register { return g.reg }

// Out gives the edges from an item, In the edges to it.
func (g *Graph) Out(key string) []Edge { return g.out[key] }

// In gives the edges to an item.
func (g *Graph) In(key string) []Edge { return g.in[key] }

// Ref finds the item that a slug, an old slug or a card key names.
func (g *Graph) Ref(s string) (Thing, bool) {
	if n := g.c.BySlug[s]; n != nil {
		return Thing{Node: n}, true
	}
	if c := g.c.CardByKey[s]; c != nil {
		return Thing{Card: c}, true
	}
	if to, ok := g.c.OldSlugs[s]; ok && g.c.BySlug[to] != nil {
		return Thing{Node: g.c.BySlug[to]}, true
	}
	return Thing{}, false
}

// URL is the URL of the page of an item. A result card has none; the table of its benchmark version has it.
func (g *Graph) URL(it Thing) string {
	if it.Node != nil {
		return g.base + "/" + it.Node.Slug + "/"
	}
	if u := cardURL(it.Card); u != "" {
		return g.base + u
	}
	if r := results.Parse(it.Card); r != nil && r.Benchmark != "" {
		return g.base + "/benchmarks/" + r.Benchmark + "/" + r.Version + "/"
	}
	return ""
}

// Head is what every record and every answer of the MCP server says of its item.
type Head struct {
	URL  string `json:"url"`
	Tag  string `json:"tag,omitempty"`
	Kind string `json:"kind"`
	Key  string `json:"key"`
	Name string `json:"name"`
	AsOf string `json:"as_of"`
}

// Head gives the head of an item, with the date of the data.
func (g *Graph) Head(it Thing, asOf string) Head {
	h := Head{URL: g.URL(it), AsOf: asOf, Key: it.Key()}
	if it.Node != nil {
		h.Tag, h.Kind, h.Name = it.Node.Tag, it.Node.Kind, it.Node.Name
	} else {
		h.Tag, h.Kind, h.Name = it.Card.Tag, it.Card.Kind, it.Card.Name
	}
	return h
}

// Record is the JSON record of an item (16.10): its fields, facts, claims, ladders, edges both ways and the result
// cards that name it. It never holds the prose, which has another licence (19.2); the Markdown twin has it.
type Record struct {
	Head
	Summary  string         `json:"summary,omitempty"`
	Markdown string         `json:"markdown_url,omitempty"`
	Fields   map[string]any `json:"fields"`
	Facts    []RecordFact   `json:"facts,omitempty"`
	Claims   []claims.Claim `json:"claims,omitempty"`
	// Status holds the ladders that the build computed. It is nil outside a build.
	Status  *status.Status `json:"status,omitempty"`
	Out     []Edge         `json:"edges_out,omitempty"`
	In      []Edge         `json:"edges_in,omitempty"`
	Results []string       `json:"results,omitempty"`
}

// Record gives the record of an item.
func (g *Graph) Record(it Thing, asOf string) Record {
	k := it.Key()
	r := Record{Head: g.Head(it, asOf), Out: g.out[k], In: g.in[k], Results: g.cites[k], Facts: FactsOf(it)}
	if it.Node != nil {
		r.Summary, r.Claims, r.Fields = it.Node.Summary, it.Node.Claims, frontMap(it.Node)
		if HasTwin(it.Node) {
			r.Markdown = g.base + "/" + it.Node.Slug + ".md"
		}
		return r
	}
	r.Summary = it.Card.Summary
	r.Fields = map[string]any{}
	for k, raw := range it.Card.Fields {
		var v any
		if json.Unmarshal(raw, &v) == nil {
			r.Fields[k] = v
		}
	}
	return r
}

// HasTwin tells whether a node has a page body, which the site writes as its Markdown twin. A stub and a node that the
// quarantine withdrew have none.
func HasTwin(n *node.Node) bool { return !isStub(n) }

// RecordFact is one dated fact of an item with its sources (04.9 for a node, 04.6 for a card).
type RecordFact struct {
	ID      string         `json:"id"`
	Label   string         `json:"label"`
	Value   any            `json:"value"`
	Unit    string         `json:"unit,omitempty"`
	Text    string         `json:"text"`
	Status  string         `json:"status,omitempty"`
	AsOf    string         `json:"as_of,omitempty"`
	Note    string         `json:"note,omitempty"`
	Sources []RecordSource `json:"sources,omitempty"`
}

// RecordSource is a source of a fact.
type RecordSource struct {
	Src   string `json:"src"`
	Kind  string `json:"kind,omitempty"`
	Loc   string `json:"loc,omitempty"`
	Value any    `json:"value,omitempty"`
	AsOf  string `json:"as_of,omitempty"`
}

// FactsOf gives the facts of an item.
func FactsOf(it Thing) []RecordFact {
	var out []RecordFact
	if it.Node != nil {
		for _, f := range it.Node.Facts {
			v := RecordFact{ID: f.ID, Label: facts.Label(f.ID), Value: f.Value, Unit: f.Unit, Text: facts.Format(f.Value, f.Unit), Status: f.Status, AsOf: facts.AsOf(f), Note: f.Note}
			for _, s := range f.Sources {
				v.Sources = append(v.Sources, RecordSource{Src: s.Src, Kind: s.Kind, Loc: s.Loc, Value: s.Value, AsOf: s.AsOf})
			}
			out = append(out, v)
		}
		return out
	}
	for _, f := range it.Card.Facts {
		v := RecordFact{ID: f.ID, Label: facts.Label(f.ID), Value: f.V, Text: facts.Format(f.V, "")}
		if f.Src != "" {
			v.Sources = []RecordSource{{Src: f.Src, Loc: f.Loc}}
		}
		out = append(out, v)
	}
	return out
}

// frontMap gives the front matter of a node as a map, the fields of its record.
func frontMap(n *node.Node) map[string]any {
	var m map[string]any
	b, _ := json.Marshal(n.Front)
	_ = json.Unmarshal(b, &m)
	return m
}
