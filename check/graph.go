// SPDX-License-Identifier: Apache-2.0

package check

import (
	"slices"
	"strings"

	"github.com/tamnd/ai-agent/bridge"
	"github.com/tamnd/ai-agent/graph"
	"github.com/tamnd/ai-agent/node"
)

// Foundations are the top areas that a core node reaches by its prerequisites (rule E09, 5.7).
var Foundations = []string{"ai-foundations", "math-for-ai", "deep-learning-foundations"}

// foundationSites are the neighbours whose items count as foundations for E09: math-for-ai holds bridge cards to math2
// for the mathematics (5.9), and algorithm has the shared foundations of learning, such as backpropagation.
var foundationSites = []string{"m2", "al"}

// where gives the file and the line of the item with the key: the node.md of a node, or the shard line of a card.
func (r *run) where(key string) (string, int) {
	if n := r.c.BySlug[key]; n != nil {
		return n.Rel + "/node.md", 0
	}
	if cd := r.c.CardByKey[key]; cd != nil {
		return cd.File, cd.Line
	}
	return "", 0
}

// wantKey tells if the run checks the item: every card and every node, or the nodes of Only.
func (r *run) wantKey(key string) bool {
	if len(r.opt.Only) == 0 {
		return true
	}
	return r.c.BySlug[key] != nil && r.opt.Only[key]
}

// graphRules runs the rules of 15.3 that need the whole graph: the cycles (E03, E04), the derived and typed edges
// (E06, E07), the path to the foundations (E09), the notes of related_to (E10), the reverse edges (E11) and the keys
// that evidence and mitigation edges carry (E13, E14).
func (r *run) graphRules() {
	g := graph.Build(r.c)
	for _, cy := range []struct {
		rule, what string
		types      []string
	}{
		{"E03", "prereqs", []string{"prereqs"}},
		{"E04", "variant_of and generalizes", []string{"variant_of", "generalizes"}},
		{"E04", "fine_tuned_from", []string{"fine_tuned_from"}},
		{"E04", "succeeded_by", []string{"succeeded_by"}},
	} {
		for _, c := range g.Cycles(cy.types...) {
			if p, l := r.where(c[0]); p != "" && r.wantKey(c[0]) {
				r.add(cy.rule, Block, p, l, "the %s edges have a cycle: %s", cy.what, strings.Join(c, " -> "))
			}
		}
	}
	for _, e := range g.Edges() {
		if !r.wantKey(e.From) {
			continue
		}
		p, l := r.where(e.From)
		if p == "" {
			continue // a derived edge from a model that has no card; M01 reports the result
		}
		switch e.Type {
		case "evaluated_on":
			if !e.Derived {
				r.add("E06", Block, p, l, "evaluated_on: %s is written by hand; the build derives evaluated_on from the result cards (10.6)", e.To)
			}
		case "architecture":
			if k, ok := g.Ref(e.To); ok && k != "architecture" {
				r.add("E07", Block, p, l, "architecture: %q is %s, not an architecture", e.To, article(k))
			}
		case "fine_tuned_from", "distilled_from", "merged_from":
			if k, _ := g.Ref(e.From); k != "model" {
				r.add("E07", Block, p, l, "%s joins models, and %s is %s", e.Type, e.From, article(k))
			} else if k, ok := g.Ref(e.To); ok && k != "model" {
				r.add("E07", Block, p, l, "%s: %q is %s, not a model", e.Type, e.To, article(k))
			}
		case "evidence_for", "evidence_against":
			if e.Claim == "" || e.Role == "" {
				r.add("E13", Block, p, l, "%s %s has no %s; write {slug: %s, claim: <claim id>, role: <role>} (07.3)", e.Type, e.To, missing(map[string]string{"claim": e.Claim, "role": e.Role}), e.To)
			}
		case "mitigates", "regulates":
			if e.Source == "" || e.Locator == "" {
				r.add("E14", Level, p, l, "%s %s has no %s; write {slug: %s, source: <work>, locator: <locator>}", e.Type, e.To, missing(map[string]string{"source": e.Source, "locator": e.Locator}), e.To)
			}
		}
	}
	r.reverseFields()
	for _, n := range r.c.Nodes {
		if !r.want(n) {
			continue
		}
		r.foundationPath(g, n)
		r.relatedNotes(g, n)
	}
}

// missing names the empty keys, in order.
func missing(keys map[string]string) string {
	var out []string
	for _, k := range sortedKeys(keys) {
		if keys[k] == "" {
			out = append(out, k)
		}
	}
	return strings.Join(out, " and ")
}

// reverseFields reports a field of a card that holds a reverse edge (E11). A node cannot have one: its front matter
// takes only the fields of its kind, and T01 reports any other. A field of the kind of 04.5, such as amended_by of a
// policy, is not a reverse edge written by hand.
func (r *run) reverseFields() {
	if len(r.opt.Only) > 0 {
		return
	}
	rev := map[string]string{}
	for k, v := range graph.Reverse {
		rev[v] = k
	}
	for _, cd := range r.c.Cards {
		for _, f := range sortedKeys(cd.Fields) {
			if of, ok := rev[f]; ok && !slices.Contains(node.KindFields[cd.Kind], f) {
				r.add("E11", Block, cd.File, cd.Line, "the card %s has %s, the reverse of %s; the build derives reverse edges, so write %s on the other end", cd.Key(), f, of, of)
			}
		}
	}
}

// foundationPath reports a core node with no path of prereqs back to a foundation (E09). The rule is about what a
// reader learns first, so it covers the kinds whose form is a node (04.2): the ideas, the tasks and the metrics. A
// record kind with a full page, such as a person, a model or a law, is read for its facts and has no prerequisites.
func (r *run) foundationPath(g *graph.Graph, n *node.Node) {
	if n.Level != "core" || isFoundation(r.c, n.Slug) {
		return
	}
	if k, ok := node.Kind(n.Kind); !ok || k.Form != "node" || k.ID == "area" {
		return
	}
	for _, k := range g.Closure(n.Slug, false, "prereqs") {
		if isFoundation(r.c, k) {
			return
		}
	}
	r.add("E09", Level, n.Rel+"/node.md", 0, "the core node has no path of prereqs back to %s, or to a bridge card of math2 or algorithm (5.7)", strings.Join(Foundations, ", "))
}

func isFoundation(c *node.Corpus, key string) bool {
	if site, _, ok := bridge.Split(key); ok {
		return slices.Contains(foundationSites, site)
	}
	n := c.BySlug[key]
	if n == nil {
		return false
	}
	top, _, _ := strings.Cut(n.Area, "/")
	return slices.Contains(Foundations, top)
}

// relatedNotes reports a related_to edge with no note on the difference (E10): the edge has no note, and no sentence
// of the text names the target. A kind whose related items are a generated section (06.3) needs the note on the edge.
func (r *run) relatedNotes(g *graph.Graph, n *node.Node) {
	text := prose(n.Body)
	for _, e := range g.Out(n.Slug) {
		if e.Type == "related_to" && e.Note == "" && !names(r.c, text, e.To) {
			r.add("E10", Level, n.Rel+"/node.md", 0, "related_to %s has no note: no sentence of the text names it; add one on the difference", e.To)
		}
	}
}

// prose is the body without its code fences.
func prose(body string) string {
	var b strings.Builder
	fence := false
	for _, l := range strings.Split(body, "\n") {
		if strings.HasPrefix(l, "```") || strings.HasPrefix(l, "~~~") {
			fence = !fence
			continue
		}
		if !fence {
			b.WriteString(l)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// names tells if a text names the target: a link to its page, [[al:slug]] for a neighbour, or its name or an alias.
func names(c *node.Corpus, text, target string) bool {
	low := strings.ToLower(text)
	has := func(names ...string) bool {
		return slices.ContainsFunc(names, func(s string) bool { return s != "" && strings.Contains(low, strings.ToLower(s)) })
	}
	if site, slug, ok := bridge.Split(target); ok {
		if strings.Contains(text, "[["+site+":"+slug+"]]") {
			return true
		}
		b := c.Bridges.Find(target)
		return b != nil && has(append([]string{b.Name}, b.Aliases...)...)
	}
	for _, l := range []string{"](/" + target + "/", "](/" + target + ")", "](/" + target + "#"} {
		if strings.Contains(text, l) {
			return true
		}
	}
	if n := c.BySlug[target]; n != nil {
		return has(append([]string{n.Name}, n.Aliases...)...)
	}
	if cd := c.CardByKey[target]; cd != nil {
		return has(cd.Name)
	}
	return false
}
