// SPDX-License-Identifier: Apache-2.0

// Package graph holds the typed edges of spec 04.7 between the nodes and the cards of a corpus (03.4): the edges as
// the front matter and the cards write them, the evaluated_on edges that the result cards give, the reverse edges that
// the build derives, the closure of a set of edge types, the reading paths of the prerequisite DAG (05.7) and the
// cycles of the edge types that may have none (15.3).
package graph

import (
	"encoding/json"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/tamnd/ai-agent/bridge"
	"github.com/tamnd/ai-agent/node"
	"github.com/tamnd/ai-agent/results"
)

// Edge is one edge: From names To in a field of type Type.
type Edge struct {
	Type string `json:"type"`
	From string `json:"from"`
	To   string `json:"to"`
	// Known is true when To is a node, a card, a bridge card that the last sync found, or a version of a benchmark.
	// A lineage view shows an edge with an unknown end as unknown, never guessed (16.9).
	Known bool `json:"known"`
	// Site is the neighbour, al, m2 or pg, of a target written as slug@al (5.9).
	Site string `json:"site,omitempty"`
	// Note, Claim, Role, Source and Locator are the keys of a target written as a map, such as
	// {slug: x, claim: c-y, role: replication} for evidence_for (07.3).
	Note    string `json:"note,omitempty"`
	Claim   string `json:"claim,omitempty"`
	Role    string `json:"role,omitempty"`
	Source  string `json:"source,omitempty"`
	Locator string `json:"locator,omitempty"`
	// Derived is true for an edge that the build makes, not a person: evaluated_on from a result card (rule E06).
	Derived bool `json:"derived,omitempty"`
}

// Reverse names the reverse of each edge type that has one. The build derives the reverse edges; a field with one of
// these names is written by hand, which rule E11 blocks. related_to, see_also and same_as have no reverse: the first two
// are shown from both ends as they are, the last names things outside the corpus.
var Reverse = map[string]string{
	"prereqs":           "needed_by",
	"uses":              "used_by",
	"components":        "component_of",
	"variant_of":        "variants",
	"generalizes":       "generalized_by",
	"applies_to":        "applied_by",
	"solves":            "solved_by",
	"evaluated_on":      "evaluates",
	"measured_by":       "measures",
	"trained_on":        "trains",
	"architecture":      "architecture_of",
	"fine_tuned_from":   "fine_tunes",
	"distilled_from":    "distilled_into",
	"merged_from":       "merged_into",
	"member_of":         "members",
	"developed_by":      "developed",
	"introduced_by":     "introduces",
	"cites":             "cited_by",
	"proves":            "proved_by",
	"explains":          "explained_by",
	"evidence_for":      "supported_by",
	"evidence_against":  "challenged_by",
	"instance_of":       "instances",
	"mitigates":         "mitigated_by",
	"regulates":         "regulated_by",
	"implements_policy": "implemented_by",
	"amends":            "amended_by",
	"responds_to":       "responses",
	"succeeded_by":      "succeeds",
	"exercises":         "exercise_for",
	"implements":        "implemented_in",
	"techniques":        "technique_of",
}

// Lineage are the edge types of the lineage of weights (rule E07): each joins two models.
var Lineage = []string{"fine_tuned_from", "distilled_from", "merged_from"}

// cardRefs are the fields of a card that name other items besides the edge types of 04.7.
var cardRefs = []string{"developer", "family", "task", "benchmark", "model", "policy", "organization", "authors"}

// noEdge are the edge types whose values are not items of the corpus.
var noEdge = map[string]bool{"same_as": true}

// Types gives the fields that the graph reads, in order: the edge keys of the front matter (04.6), then the other edge
// types of 04.7 in the order of its table, then the other fields of a card that name items.
func Types() []string {
	out := slices.Clone(node.EdgeKeys)
	for _, e := range node.Edges {
		if !noEdge[e.Type] && !slices.Contains(out, e.Type) {
			out = append(out, e.Type)
		}
	}
	for _, k := range cardRefs {
		if !slices.Contains(out, k) {
			out = append(out, k)
		}
	}
	return out
}

// Graph holds the edges of a corpus both ways.
type Graph struct {
	c     *node.Corpus
	reg   *results.Register
	types []string
	out   map[string][]Edge // by the key of the item
	in    map[string][]Edge
	n     int
}

// Build finds the edges of each node and each card of a corpus, and the evaluated_on edges of its result cards.
func Build(c *node.Corpus) *Graph {
	reg := results.Load(c)
	g := &Graph{c: c, reg: reg, types: Types(), out: map[string][]Edge{}, in: map[string][]Edge{}}
	for _, n := range c.Nodes {
		fields := frontMap(n)
		for k, v := range n.Fields {
			if _, ok := fields[k]; !ok {
				fields[k] = v
			}
		}
		g.from(n.Slug, fields)
	}
	for _, cd := range c.Cards {
		fields := map[string]any{}
		for k, raw := range cd.Fields {
			var v any
			if json.Unmarshal(raw, &v) == nil {
				fields[k] = v
			}
		}
		g.from(cd.Key(), fields)
	}
	seen := map[[2]string]bool{}
	for _, r := range reg.Results {
		if r.Model == "" || r.Benchmark == "" || r.Version == "" {
			continue // rule M01 reports it
		}
		to := r.Benchmark + "@" + r.Version
		if seen[[2]string{r.Model, to}] {
			continue
		}
		seen[[2]string{r.Model, to}] = true
		_, known := g.Ref(to)
		g.add(Edge{Type: "evaluated_on", From: r.Model, To: to, Known: known, Derived: true})
	}
	for k := range g.in {
		sort.SliceStable(g.in[k], func(i, j int) bool { return g.in[k][i].From < g.in[k][j].From })
	}
	return g
}

func (g *Graph) from(key string, fields map[string]any) {
	for _, typ := range g.types {
		v, ok := fields[typ]
		if !ok {
			continue
		}
		for _, e := range targets(v) {
			e.Type, e.From = typ, key
			if site, _, ok := bridge.Split(e.To); ok {
				e.Site = site
				b := g.c.Bridges.Find(e.To)
				e.Known = b != nil && b.Retired == ""
			} else {
				_, e.Known = g.Ref(e.To)
			}
			g.add(e)
		}
	}
}

func (g *Graph) add(e Edge) {
	g.out[e.From] = append(g.out[e.From], e)
	g.in[e.To] = append(g.in[e.To], e)
	g.n++
}

func frontMap(n *node.Node) map[string]any {
	m := map[string]any{}
	b, _ := json.Marshal(n.Front)
	_ = json.Unmarshal(b, &m)
	return m
}

var refRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*(/[a-z0-9][a-z0-9.-]*)?(@[a-z0-9][a-z0-9.-]*)?$`)

// isRef tells if a string names an item: a slug, the key of an owned card, a version of a benchmark as
// benchmark@version, or a neighbour as slug@al or al:slug.
func isRef(s string) bool {
	if _, _, ok := bridge.Split(s); ok {
		return true
	}
	return refRE.MatchString(s)
}

// targets gives the edges that a field value names, with only To and the keys of a map set: a slug, a list of them,
// or a map whose slug, to, target, problem or v names the target.
func targets(v any) []Edge {
	switch t := v.(type) {
	case string:
		if s := strings.TrimSpace(t); isRef(s) {
			return []Edge{{To: s}}
		}
	case []any:
		var out []Edge
		for _, x := range t {
			out = append(out, targets(x)...)
		}
		return out
	case map[string]any:
		str := func(keys ...string) string {
			for _, k := range keys {
				if s, ok := t[k].(string); ok && s != "" {
					return strings.TrimSpace(s)
				}
			}
			return ""
		}
		for _, k := range []string{"slug", "to", "target", "problem", "v"} {
			x, ok := t[k]
			if !ok {
				continue
			}
			out := targets(x)
			for i := range out {
				out[i].Note, out[i].Claim, out[i].Role = str("note"), str("claim"), str("role")
				out[i].Source, out[i].Locator = str("source", "src", "work"), str("locator", "loc")
			}
			return out
		}
	}
	return nil
}

// Ref finds the item that a slug, an old slug, a card key or benchmark@version names, and gives its kind.
func (g *Graph) Ref(s string) (kind string, ok bool) {
	if b, v, found := strings.Cut(s, "@"); found {
		if _, ok := g.reg.Versions[b][v]; ok {
			return "benchmark version", true
		}
		return "", false
	}
	if n := g.c.BySlug[s]; n != nil {
		return n.Kind, true
	}
	if c := g.c.CardByKey[s]; c != nil {
		return c.Kind, true
	}
	if to, ok := g.c.OldSlugs[s]; ok && g.c.BySlug[to] != nil {
		return g.c.BySlug[to].Kind, true
	}
	return "", false
}

// Len is the number of edges.
func (g *Graph) Len() int { return g.n }

// Out gives the edges from an item, in the order of its fields.
func (g *Graph) Out(key string) []Edge { return g.out[key] }

// In gives the edges to an item, by the key of the item they come from.
func (g *Graph) In(key string) []Edge { return g.in[key] }

// Edges gives every edge, by the item it comes from, then by type and target.
func (g *Graph) Edges() []Edge {
	out := make([]Edge, 0, g.n)
	for _, es := range g.out {
		out = append(out, es...)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.From != b.From {
			return a.From < b.From
		}
		if a.Type != b.Type {
			return a.Type < b.Type
		}
		return a.To < b.To
	})
	return out
}

// Reversed gives the reverse edges that the build derives: for each edge of a type that has a reverse and a known end
// in the corpus, the edge back from its target. A neighbour and an unknown end have no page to show it.
func (g *Graph) Reversed() []Edge {
	var out []Edge
	for _, e := range g.Edges() {
		r, ok := Reverse[e.Type]
		if !ok || !e.Known || e.Site != "" {
			continue
		}
		e.Type, e.From, e.To = r, e.To, e.From
		out = append(out, e)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].From != out[j].From {
			return out[i].From < out[j].From
		}
		return out[i].Type < out[j].Type
	})
	return out
}

// next gives the targets of the edges of the given types from key, or the sources of the edges to key when up is false.
func (g *Graph) next(key string, down bool, types []string) []string {
	var out []string
	es := g.out[key]
	if !down {
		es = g.in[key]
	}
	for _, e := range es {
		if !slices.Contains(types, e.Type) {
			continue
		}
		if down {
			out = append(out, e.To)
		} else {
			out = append(out, e.From)
		}
	}
	return out
}

// Closure gives the items that key reaches by edges of the given types, in the order of a breadth-first walk. With up
// false it follows the edges forward, to what key names; with up true it follows them back, to what names key.
func (g *Graph) Closure(key string, up bool, types ...string) []string {
	seen := map[string]bool{key: true}
	var out []string
	frontier := []string{key}
	for len(frontier) > 0 {
		var next []string
		for _, k := range frontier {
			for _, t := range g.next(k, !up, types) {
				if !seen[t] {
					seen[t] = true
					out = append(out, t)
					next = append(next, t)
				}
			}
		}
		frontier = next
	}
	return out
}

// Path is the reading path to a target (05.7, 16.12): the topological order of its prerequisites, each after its own,
// in the order the front matter lists them, and then the target. A neighbour is a step with no prerequisites here; a
// slug that names nothing is left out. A cycle, which rule E03 blocks, is cut where the walk meets it again.
func (g *Graph) Path(target string) []string {
	seen := map[string]bool{}
	var out []string
	var visit func(k string)
	visit = func(k string) {
		if to, ok := g.c.OldSlugs[k]; ok && g.c.BySlug[k] == nil {
			k = to
		}
		if seen[k] {
			return
		}
		seen[k] = true
		for _, e := range g.out[k] {
			if e.Type == "prereqs" && e.Known {
				visit(e.To)
			}
		}
		out = append(out, k)
	}
	visit(target)
	return out
}

// Cycles gives each cycle of the edges of the given types once, as the keys along it with the first key again at the
// end, from the first key in sorted order.
func (g *Graph) Cycles(types ...string) [][]string {
	const (
		white = iota
		grey
		black
	)
	colour := map[string]int{}
	var stack []string
	var out [][]string
	var visit func(k string)
	visit = func(k string) {
		colour[k] = grey
		stack = append(stack, k)
		for _, t := range g.next(k, true, types) {
			switch colour[t] {
			case white:
				visit(t)
			case grey:
				i := slices.Index(stack, t)
				out = append(out, append(slices.Clone(stack[i:]), t))
			}
		}
		stack = stack[:len(stack)-1]
		colour[k] = black
	}
	keys := make([]string, 0, len(g.out))
	for k := range g.out {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if colour[k] == white {
			visit(k)
		}
	}
	return out
}

// Counts gives the number of edges of each type.
func (g *Graph) Counts() map[string]int {
	m := map[string]int{}
	for _, es := range g.out {
		for _, e := range es {
			m[e.Type]++
		}
	}
	return m
}
