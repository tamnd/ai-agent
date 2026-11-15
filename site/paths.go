// SPDX-License-Identifier: Apache-2.0

package site

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/tamnd/ai-agent/bridge"
	"github.com/tamnd/ai-agent/course"
	"github.com/tamnd/ai-agent/graph"
	"github.com/tamnd/ai-agent/node"
)

// PathStep is one step of a reading path (16.12): the item, its ladder digits and the time it takes to read.
type PathStep struct {
	Link
	// Key is the key of the step in the reader's progress: the slug of a node, the key of a neighbour, or the name of
	// an item that has no page yet.
	Key     string
	Summary string
	Digits  string
	Minutes int
	Target  bool
}

// PathSection is a heading of a path and its steps. A computed path has one section with no heading.
type PathSection struct {
	Heading string
	Steps   []PathStep
}

// PathPage is a page of /paths/.
type PathPage struct {
	Base
	Heading  string
	Lead     string
	ID       string // the id of the path in the reader's browser, such as to/transformer
	Basis    string
	BasisURL string // the source of the outside system of a curriculum path
	Sections []PathSection
	Steps    int
	Written  int
	Minutes  int
}

// minPathSteps is the smallest path to a target that gets a page: the target and two steps before it. A shorter
// path is the "Read first" strip of the page.
const minPathSteps = 3

// reservedPaths are the path ids that a course cannot take, since the other kinds of path use them.
var reservedPaths = []string{"to", "area", "from-scratch", "cs2023-ai", "acm-ccs-ml"}

// levelPace is how many minutes a page of a level takes for each 200 words, for the time estimate of a step.
var levelPace = map[string]float64{"intro": 1, "core": 1.5, "advanced": 2, "research": 2.5}

// pathTargets finds the nodes whose reading path is long enough for a page, and sets b.pathTo.
func (b *builder) pathTargets() {
	b.graph = graph.Build(b.c)
	b.pathTo = map[string]string{}
	for _, n := range b.c.Nodes {
		if isStub(n) || n.Kind == "person" || n.Kind == "exercise" {
			continue
		}
		if len(b.graph.Path(n.Slug)) >= minPathSteps {
			b.pathTo[n.Slug] = "/paths/to/" + n.Slug + "/"
		}
	}
}

// paths writes the reading paths of 16.12: the path to each target, the courses, the area paths, and the index.
func (b *builder) paths() error {
	courses, err := course.Load(b.c.Root)
	if err != nil {
		return err
	}
	var index []Group
	find := b.byName()
	written := map[string]bool{} // the ids of the courses with a page
	cg := Group{ID: "courses", Title: "Courses", Intro: "Curated courses with section headings, each cross-checked against the public syllabi it names."}
	for _, c := range courses {
		if slices.Contains(reservedPaths, c.ID) {
			b.opt.Log("course %s: the id is reserved for another kind of path", c.ID)
			continue
		}
		p := b.coursePage(c, find)
		written[c.ID] = true
		if err := b.page("paths", p.Path, p); err != nil {
			return err
		}
		cg.Items = append(cg.Items, Item{Link: Link{Name: c.Title, URL: p.Path, Hue: 235}, Meta: pathMeta(p), Summary: p.Lead})
	}
	if len(cg.Items) > 0 {
		index = append(index, cg)
	}
	curricula, err := b.curriculumPaths()
	if err != nil {
		return err
	}
	ug := Group{ID: "curricula", Title: "Curricula", Intro: "The units of outside curricula and classifications in their own order, with the pages that the crosswalks map to each."}
	for _, p := range curricula {
		if written[p.ID] {
			b.opt.Log("crosswalk path %s: a course has the id", p.ID)
			continue
		}
		if err := b.page("paths", p.Path, p); err != nil {
			return err
		}
		ug.Items = append(ug.Items, Item{Link: Link{Name: p.Heading, URL: p.Path, Hue: 200}, Meta: pathMeta(p), Summary: p.Lead})
	}
	if len(ug.Items) > 0 {
		index = append(index, ug)
	}
	ag := Group{ID: "areas", Title: "Areas", Intro: "The first and the core pages of each area, each after its prerequisites."}
	for _, a := range b.c.Areas {
		p, ok := b.areaPath(a)
		if !ok {
			continue
		}
		if err := b.page("paths", p.Path, p); err != nil {
			return err
		}
		ag.Items = append(ag.Items, Item{Link: Link{Name: a.Name, URL: p.Path, Hue: b.hueOf(a.ID)}, Meta: pathMeta(p)})
	}
	if len(ag.Items) > 0 {
		index = append(index, ag)
	}
	tg := Group{ID: "to", Title: "From zero to a target", Intro: "The prerequisites of one page in an order where each comes after its own. Mark what you know and the path skips it."}
	var targets []*node.Node
	for slug := range b.pathTo {
		targets = append(targets, b.c.BySlug[slug])
	}
	sort.Slice(targets, func(i, j int) bool { return targets[i].Name < targets[j].Name })
	for _, n := range targets {
		p := b.targetPage(n)
		if err := b.page("paths", p.Path, p); err != nil {
			return err
		}
		tg.Items = append(tg.Items, Item{Link: b.link(n.Slug), Meta: pathMeta(p), Level: n.Level})
		tg.Items[len(tg.Items)-1].URL = p.Path
	}
	if len(tg.Items) > 0 {
		index = append(index, tg)
	}
	code := b.codeTargets()
	fg := Group{ID: "from-scratch", Title: "From scratch in code", Intro: "The NumPy references that build up to one, each after the code it builds on."}
	for _, n := range b.sortedTargets(code) {
		p := b.codePage(n, code[n.Slug])
		if err := b.page("paths", p.Path, p); err != nil {
			return err
		}
		it := Item{Link: b.link(n.Slug), Meta: pathMeta(p), Level: n.Level}
		it.URL = p.Path
		fg.Items = append(fg.Items, it)
	}
	if len(fg.Items) > 0 {
		index = append(index, fg)
	}
	desc := "Reading paths through the site: courses, curricula, areas in reading order, the way from zero to any page, and code built from scratch."
	lp := ListPage{Base: b.base("Paths", desc, "/paths/")}
	lp.Section = "paths"
	lp.Heading, lp.Lead, lp.Groups = "Paths", desc, index
	if len(index) == 0 {
		lp.Empty = "No path yet: a path needs a page with prerequisites, a course or an area with first pages."
	}
	return b.page("list", "/paths/", lp)
}

func pathMeta(p PathPage) string {
	s := comma(p.Steps) + " " + plural(p.Steps, "step", "steps")
	if p.Written < p.Steps {
		s += fmt.Sprintf(", %d written", p.Written)
	}
	if p.Minutes == 0 {
		return s
	}
	return s + fmt.Sprintf(", about %s", hours(p.Minutes))
}

// hours writes a time in minutes as minutes or hours.
func hours(m int) string {
	if m < 60 {
		return fmt.Sprintf("%d min", m)
	}
	h := float64(m) / 60
	if h < 10 {
		return strings.TrimSuffix(fmt.Sprintf("%.1f", h), ".0") + " h"
	}
	return fmt.Sprintf("%.0f h", h)
}

// step makes the step of a key: a node, a neighbour, or the name of an item that has no page yet.
func (b *builder) step(key string) PathStep {
	s := PathStep{Link: b.link(key), Key: key}
	if _, _, ok := bridge.Split(key); ok {
		s.Minutes = 10
		if c := b.c.Bridges.Find(key); c != nil {
			s.Key = c.Site + ":" + c.Slug
		}
		return s
	}
	n := b.c.BySlug[key]
	if n == nil {
		return s
	}
	s.Summary = n.Summary
	s.Minutes = readMinutes(n)
	if st, ok := b.status[n.Slug]; ok && !isStub(n) {
		s.Digits = fmt.Sprintf("H%d L%d V%d W%d G%d", st.H.Level, st.L.Level, st.V.Level, st.W.Level, st.G.Level)
	}
	return s
}

// readMinutes estimates the time to read a page from the length of its text and its level.
func readMinutes(n *node.Node) int {
	if isStub(n) {
		return 0
	}
	pace, ok := levelPace[n.Level]
	if !ok {
		pace = 1.5
	}
	m := int(float64(len(strings.Fields(n.Body)))/200*pace + 0.999)
	return max(m, 2)
}

func (b *builder) pathPage(id, title, desc string, secs []PathSection) PathPage {
	p := PathPage{Base: b.base(title, desc, "/paths/"+id+"/"), Heading: title, Lead: desc, ID: id, Sections: secs}
	p.Section = "paths"
	p.Crumbs = []Link{{Name: "Paths", URL: "/paths/"}}
	p.Scripts = []string{"/assets/paths.js"}
	for _, s := range secs {
		for _, st := range s.Steps {
			p.Steps++
			if !st.Missing {
				p.Written++
			}
			p.Minutes += st.Minutes
		}
	}
	return p
}

// targetPage is the path from zero to a node: the topological order of its prerequisites (05.7).
func (b *builder) targetPage(n *node.Node) PathPage {
	var steps []PathStep
	for _, k := range b.graph.Path(n.Slug) {
		s := b.step(k)
		s.Target = k == n.Slug
		steps = append(steps, s)
	}
	desc := fmt.Sprintf("The pages to read before %s, each after its own prerequisites, and then %s.", n.Name, n.Name)
	return b.pathPage("to/"+n.Slug, "From zero to "+n.Name, desc, []PathSection{{Steps: steps}})
}

// byName finds the node of a course item that gives no slug: the one node whose name or alias is the item's name,
// in any case. A name that two nodes share finds none.
func (b *builder) byName() func(string) string {
	names := map[string]string{}
	for _, n := range b.c.Nodes {
		for _, s := range append([]string{n.Name}, n.Aliases...) {
			k := strings.ToLower(strings.TrimSpace(s))
			if old, ok := names[k]; ok && old != n.Slug {
				names[k] = ""
				continue
			}
			names[k] = n.Slug
		}
	}
	return func(name string) string { return names[strings.ToLower(strings.TrimSpace(name))] }
}

// coursePage is a curated course of topics/courses, with its section headings. An item with no node takes the node
// of its name when exactly one node has it.
func (b *builder) coursePage(c course.Course, find func(string) string) PathPage {
	var secs []PathSection
	for _, sec := range c.Sections {
		ps := PathSection{Heading: sec.Heading}
		for _, it := range sec.Items {
			var s PathStep
			if it.Node == "" {
				it.Node = find(it.Name)
			}
			if it.Node != "" {
				s = b.step(it.Node)
			} else {
				s = PathStep{Link: Link{Name: it.Name, Missing: true}, Key: "name:" + it.Name}
			}
			if s.Missing {
				s.Name, s.Note, s.Kind = it.Name, "This page is not written yet.", it.Kind
			}
			ps.Steps = append(ps.Steps, s)
		}
		secs = append(secs, ps)
	}
	desc := fmt.Sprintf("A course of %s steps in %d %s.", comma(countSteps(secs)), len(secs), plural(len(secs), "section", "sections"))
	p := b.pathPage(c.ID, c.Title, desc, secs)
	if c.Basis != "" {
		p.Basis = "It follows " + strings.TrimSuffix(c.Basis, ".") + "."
	}
	return p
}

func countSteps(secs []PathSection) int {
	n := 0
	for _, s := range secs {
		n += len(s.Steps)
	}
	return n
}

// areaPath is the path of the first and the core pages of a top area, in reading order. It is false when the area has
// none.
func (b *builder) areaPath(a node.Area) (PathPage, bool) {
	var ns []*node.Node
	for _, n := range b.c.InArea(a.Path()) {
		if (n.Level == "intro" || n.Level == "core") && !isStub(n) && n.Kind != "person" && n.Kind != "exercise" {
			ns = append(ns, n)
		}
	}
	if len(ns) == 0 {
		return PathPage{}, false
	}
	var steps []PathStep
	for _, n := range readingOrder(ns) {
		steps = append(steps, b.step(n.Slug))
	}
	desc := fmt.Sprintf("The first and the core pages of %s, each after its prerequisites in the area.", a.Name)
	p := b.pathPage("area/"+a.ID, a.Name, desc, []PathSection{{Steps: steps}})
	p.Crumbs = append(p.Crumbs, Link{Name: a.Name, URL: "/" + a.ID + "/"})
	return p, true
}

// readingOrder is the reading order of 05.7: each node after its prerequisites in the set, and among the nodes that
// are ready, the lower level first and then the name. A cycle, which rule E03 blocks, is cut at the first node.
func readingOrder(ns []*node.Node) []*node.Node {
	in := map[string]bool{}
	for _, n := range ns {
		in[n.Slug] = true
	}
	rank := func(n *node.Node) int { return slices.Index(node.Levels, n.Level) }
	rest := slices.Clone(ns)
	sort.SliceStable(rest, func(i, j int) bool {
		if a, c := rank(rest[i]), rank(rest[j]); a != c {
			return a < c
		}
		return rest[i].Name < rest[j].Name
	})
	done := map[string]bool{}
	var out []*node.Node
	for len(rest) > 0 {
		pick := 0
		for i, n := range rest {
			ready := true
			for _, q := range n.Prereqs {
				if in[q] && !done[q] && q != n.Slug {
					ready = false
					break
				}
			}
			if ready {
				pick = i
				break
			}
		}
		n := rest[pick]
		done[n.Slug] = true
		out = append(out, n)
		rest = slices.Delete(rest, pick, pick+1)
	}
	return out
}
