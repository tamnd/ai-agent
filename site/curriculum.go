// SPDX-License-Identifier: Apache-2.0

package site

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/tamnd/ai-agent/crosswalk"
	"github.com/tamnd/ai-agent/node"
)

// codeEdges are the edges that a code path walks: a node's code builds on the code of its prerequisites, its components
// and the methods it uses.
var codeEdges = []string{"prereqs", "components", "uses"}

// codeLang is the language of the code paths: the NumPy references, which each build from the arrays up (08.1).
const codeLang = "numpy"

// curriculumPaths makes the curriculum paths of 16.12 from the paths of the crosswalks (5.8): a section for each unit of
// the outside system, in its order, with the items mapped to it. A unit with no item is a step that is not written.
func (b *builder) curriculumPaths() ([]PathPage, error) {
	cws, err := crosswalk.Load(b.c.Root)
	if err != nil {
		return nil, err
	}
	for _, p := range crosswalk.Check(b.c, cws) {
		b.opt.Log("%s", p)
	}
	var out []PathPage
	for _, cw := range cws {
		for _, p := range cw.Paths {
			if p.ID == "" || strings.Contains(p.ID, "/") || p.ID == "to" || p.ID == "area" || p.ID == "from-scratch" {
				continue // Check reports it
			}
			secs, err := cw.Sections(b.c, p)
			if err != nil {
				b.opt.Log("%v", err)
				continue
			}
			out = append(out, b.curriculumPage(cw, p, secs))
		}
	}
	return out, nil
}

func (b *builder) curriculumPage(cw crosswalk.Crosswalk, p crosswalk.Path, secs []crosswalk.Section) PathPage {
	seen := map[string]bool{}
	var ps []PathSection
	units, covered := 0, 0
	for _, s := range secs {
		sec := PathSection{Heading: s.Row.Name + " (" + s.Row.ID + ")"}
		for _, it := range s.Items {
			for _, st := range b.itemSteps(it) {
				if seen[st.Key] {
					continue
				}
				seen[st.Key] = true
				sec.Steps = append(sec.Steps, st)
			}
		}
		units++
		if slices.ContainsFunc(s.Items, func(it crosswalk.Item) bool { return crosswalk.Covered(b.c, it) }) {
			covered++
		}
		if len(s.Items) == 0 {
			note := "No page covers this unit yet."
			if s.Row.Note != "" {
				note = upperFirst(strings.TrimSuffix(s.Row.Note, ".")) + "."
			}
			sec.Steps = []PathStep{{Link: Link{Name: s.Row.Name, Missing: true, Note: note}, Key: "unit:" + cw.Name + ":" + s.Row.ID}}
		}
		ps = append(ps, sec)
	}
	desc := fmt.Sprintf("The %s units of %s, in their order, with the pages that the crosswalk maps to each; %d of the %s %s a page.",
		comma(units), cw.System, covered, comma(units), plural(covered, "has", "have"))
	page := b.pathPage(p.ID, p.Title, desc, ps)
	var basis []string
	if cw.Version != "" {
		basis = append(basis, "version "+cw.Version)
	}
	if cw.Checked != "" {
		basis = append(basis, "checked "+cw.Checked)
	}
	page.Basis = "It follows " + cw.System
	if len(basis) > 0 {
		page.Basis += " (" + strings.Join(basis, ", ") + ")"
	}
	page.Basis += ", by the crosswalk " + crosswalk.Dir + "/" + cw.Name + ".yaml (5.8). Only the ids and names of the units are kept."
	page.BasisURL = cw.Source
	return page
}

// itemSteps are the steps of a crosswalk item: a node, a card, or the first and the core pages of an area in reading
// order.
func (b *builder) itemSteps(it crosswalk.Item) []PathStep {
	switch {
	case it.Node != nil:
		return []PathStep{b.step(it.Node.Slug)}
	case it.Key != "":
		c := b.c.CardByKey[it.Key]
		return []PathStep{{Link: Link{Name: c.Name, URL: b.cardLink(c), Kind: c.Kind, Hue: b.hueOf("")}, Key: it.Key, Summary: c.Summary, Minutes: 1}}
	}
	var ns []*node.Node
	for _, n := range b.c.InArea(it.Area) {
		if (n.Level == "intro" || n.Level == "core") && !isStub(n) && n.Kind != "person" && n.Kind != "exercise" {
			ns = append(ns, n)
		}
	}
	if len(ns) == 0 {
		l := b.link(it.Area[strings.LastIndex(it.Area, "/")+1:])
		l.URL, l.Missing, l.Note = "", true, "No page of this area is written yet."
		return []PathStep{{Link: l, Key: "area:" + it.Area}}
	}
	var out []PathStep
	for _, n := range readingOrder(ns) {
		out = append(out, b.step(n.Slug))
	}
	return out
}

// codeTargets are the nodes with a code path: a NumPy reference, and at least two nodes with one that it builds on.
func (b *builder) codeTargets() map[string][]string {
	out := map[string][]string{}
	for _, n := range b.c.Nodes {
		if isStub(n) || n.Lang(codeLang) == nil || b.c.BySlug[n.Slug] != n {
			continue
		}
		var steps []string
		for _, k := range b.graph.Walk(n.Slug, codeEdges...) {
			if m := b.c.BySlug[k]; m != nil && m.Lang(codeLang) != nil && !isStub(m) {
				steps = append(steps, k)
			}
		}
		if len(steps) >= minPathSteps {
			out[n.Slug] = steps
		}
	}
	return out
}

// codePage is the code path to a node (16.12): the nodes whose NumPy references it builds on, each after the ones its
// code builds on, and then the node.
func (b *builder) codePage(n *node.Node, slugs []string) PathPage {
	var steps []PathStep
	for _, k := range slugs {
		s := b.step(k)
		s.URL += "#code"
		s.Target = k == n.Slug
		if m := b.c.BySlug[k]; m != nil {
			s.Minutes = codeMinutes(m.Lang(codeLang))
		}
		steps = append(steps, s)
	}
	desc := fmt.Sprintf("The NumPy references to read and run before the one of %s, each after the code it builds on: its prerequisites, its components and the methods it uses.", n.Name)
	p := b.pathPage("from-scratch/"+n.Slug, n.Name+" from scratch", desc, []PathSection{{Steps: steps}})
	p.Basis = "The time of a step here is the time to read its code and its tests, about 20 lines a minute."
	return p
}

// codeMinutes estimates the time to read an implementation and its tests: 20 lines a minute, at least 2 minutes.
func codeMinutes(im *node.Impl) int {
	if im == nil {
		return 0
	}
	lines := 0
	for _, f := range im.Files {
		lines += strings.Count(f.Content, "\n")
	}
	return max((lines+19)/20, 2)
}

// sortedTargets are the targets of a map of paths, by the name of their node.
func (b *builder) sortedTargets(m map[string][]string) []*node.Node {
	var ns []*node.Node
	for slug := range m {
		ns = append(ns, b.c.BySlug[slug])
	}
	sort.Slice(ns, func(i, j int) bool { return ns[i].Name < ns[j].Name })
	return ns
}
