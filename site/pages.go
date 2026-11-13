package site

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/tamnd/ai-agent/markup"
	"github.com/tamnd/ai-agent/node"
	"github.com/tamnd/ai-agent/spec"
	"github.com/tamnd/ai-agent/status"
)

//go:embed pages/*.md
var pageFS embed.FS

// searchEntry is one entry of search.json. The keys are short, because the palette loads the whole file.
type searchEntry struct {
	Name    string   `json:"n"`
	URL     string   `json:"u"`
	Kind    string   `json:"k"`
	Summary string   `json:"s,omitempty"`
	Aliases []string `json:"a,omitempty"`
	Area    string   `json:"r,omitempty"`
	Tag     string   `json:"t,omitempty"`
	Rank    int      `json:"-"`
}

var kindRank = func() map[string]int {
	out := map[string]int{}
	for _, k := range node.KindTable {
		out[k.ID] = k.Rank
	}
	return out
}()

var levelRank = map[string]int{"intro": 6, "core": 4, "advanced": 2}

func (b *builder) addSearch(n *node.Node) {
	area := ""
	if a := b.area(strings.SplitN(n.Area, "/", 2)[0]); a != nil {
		area = a.Name
	}
	b.search = append(b.search, searchEntry{
		Name: n.Name, URL: "/" + n.Slug + "/", Kind: kindName(n.Kind), Summary: n.Summary,
		Aliases: n.Aliases, Area: area, Tag: n.Tag, Rank: kindRank[n.Kind] + levelRank[n.Level],
	})
}

// Item is one row of a list.
type Item struct {
	Link
	Summary string
	Level   string
	Meta    string
	Chips   []string
}

// Tile is one tile of a grid.
type Tile struct {
	Name    string
	URL     string
	Summary string
	Hue     int
	Count   int
	Unit    string
	Missing bool
	Extra   string
}

// Group is one titled part of a list page.
type Group struct {
	ID    string
	Title string
	Intro string
	Items []Item
}

// Fact is one row of a fact table.
type Fact struct {
	Key, Value string
	Code       bool
}

// ListPage is the data of an index page.
type ListPage struct {
	Base
	Heading string
	Lead    string
	Facts   []Fact
	Tiles   []Tile
	Groups  []Group
	Empty   string
	Body    template.HTML
}

// HomePage is the data of the home page.
type HomePage struct {
	Base
	Tiles  []Tile
	Start  []Item
	Scenes []Tile
	Langs  []Tile
}

// DocPage is a page of prose.
type DocPage struct {
	Base
	Heading string
	Lead    string
	Body    template.HTML
	TOC     []markup.Heading
	Source  string
}

func (b *builder) item(n *node.Node) Item {
	it := Item{Link: b.link(n.Slug), Summary: n.Summary, Level: n.Level}
	if n.Complexity != nil {
		if t := claimsOf("time", n.Complexity.Time); len(t) > 0 {
			it.Meta = t[0].Bound + " " + t[0].Cases
		}
	}
	if len(n.Impls) > 0 {
		it.Chips = append(it.Chips, fmt.Sprintf("%d languages", len(n.Impls)))
	}
	if n.Scene != "" {
		it.Chips = append(it.Chips, "visual")
	}
	return it
}

func (b *builder) items(ns []*node.Node) []Item {
	out := make([]Item, 0, len(ns))
	for _, n := range ns {
		out = append(out, b.item(n))
	}
	return out
}

// kindOrder is the order of the kinds in lists: the order of spec 04.2, with the areas left out.
var kindOrder = func() []string {
	var out []string
	for _, k := range node.KindTable {
		if k.ID != "area" {
			out = append(out, k.ID)
		}
	}
	return out
}()

// byKind groups nodes by kind, in the order of kindOrder.
func (b *builder) byKind(ns []*node.Node) []Group {
	var out []Group
	for _, k := range kindOrder {
		var in []*node.Node
		for _, n := range ns {
			if n.Kind == k {
				in = append(in, n)
			}
		}
		if len(in) == 0 {
			continue
		}
		sortNodes(in)
		out = append(out, Group{ID: k, Title: title(kindName(k, true)), Items: b.items(in)})
	}
	return out
}

func sortNodes(ns []*node.Node) {
	sort.SliceStable(ns, func(i, j int) bool {
		a, c := slices.Index(node.Levels, ns[i].Level), slices.Index(node.Levels, ns[j].Level)
		if a != c {
			return a < c
		}
		return ns[i].Name < ns[j].Name
	})
}

func (b *builder) home() error {
	p := HomePage{Base: b.base("Algorithm", "Algorithms and data structures, explained in plain words, proved by hand and in Lean, implemented and tested in many languages, and shown step by step.", "/")}
	p.Section = "home"
	p.Tiles = b.areaTiles()
	var start []*node.Node
	for _, n := range b.c.Nodes {
		if n.Level == "intro" && (n.Kind == "method" || n.Kind == "architecture" || n.Kind == "concept" || n.Kind == "task") {
			start = append(start, n)
		}
	}
	sortNodes(start)
	if len(start) > 6 {
		start = start[:6]
	}
	p.Start = b.items(start)
	p.Scenes = b.sceneTiles(6)
	for _, l := range b.langTiles() {
		if l.Count > 0 {
			p.Langs = append(p.Langs, l)
		}
	}
	return b.page("home", "/", p)
}

func (b *builder) areaTiles() []Tile {
	var out []Tile
	for _, a := range b.c.Areas {
		n := len(b.c.InArea(a.ID))
		out = append(out, Tile{Name: a.Name, URL: "/" + a.ID + "/", Summary: a.Summary, Hue: b.hue[a.ID], Count: n, Unit: "page", Missing: n == 0})
	}
	return out
}

func (b *builder) sceneTiles(limit int) []Tile {
	var out []Tile
	for _, n := range b.c.Nodes {
		if n.Scene == "" {
			continue
		}
		out = append(out, Tile{Name: n.Name, URL: "/" + n.Slug + "/visualize/", Summary: n.Summary, Hue: b.hueOf(n.Area), Count: len(n.Presets), Unit: "preset", Extra: "/" + n.Slug + "/"})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func (b *builder) areas() error {
	p := ListPage{Base: b.base("Areas", "The areas of the field of algorithms, from foundations to research topics.", "/areas/")}
	p.Section, p.Heading = "areas", "Areas"
	p.Lead = fmt.Sprintf("The site puts each page in one of %d areas. An area has sub-areas. A grey tile has no pages yet.", len(b.c.Areas))
	p.Tiles = b.areaTiles()
	if err := b.page("list", "/areas/", p); err != nil {
		return err
	}
	for _, a := range b.c.Areas {
		if err := b.areaPage(a, nil); err != nil {
			return err
		}
		for _, s := range a.Children {
			if err := b.areaPage(s, &a); err != nil {
				return err
			}
		}
	}
	return nil
}

func (b *builder) areaPage(a node.Area, parent *node.Area) error {
	if b.c.BySlug[a.ID] != nil {
		b.opt.Log("the area %s has the same id as a node; the area page is not written", a.ID)
		return nil
	}
	path := a.Path()
	p := ListPage{Base: b.base(a.Name, a.Summary, "/"+a.ID+"/")}
	p.Section, p.Heading, p.Lead = "areas", a.Name, a.Summary
	top := a.ID
	if parent != nil {
		top = parent.ID
		p.Crumbs = []Link{{Name: "Areas", URL: "/areas/"}, {Name: parent.Name, URL: "/" + parent.ID + "/"}}
	} else {
		p.Crumbs = []Link{{Name: "Areas", URL: "/areas/"}}
	}
	p.Hue = b.hue[top]
	for _, s := range a.Children {
		n := len(b.c.InArea(s.Path()))
		p.Tiles = append(p.Tiles, Tile{Name: s.Name, URL: "/" + s.ID + "/", Summary: s.Summary, Hue: p.Hue, Count: n, Unit: "page", Missing: n == 0})
	}
	p.Groups = b.byKind(b.c.InArea(path))
	if len(p.Groups) == 0 {
		p.Empty = "This area has no pages yet. The coverage plan lists the pages that it needs. You can write one of them."
	}
	return b.page("list", "/"+a.ID+"/", p)
}

func (b *builder) tasks() error {
	p := ListPage{Base: b.base("Tasks", "The tasks that AI systems solve, with the methods that solve each one.", "/tasks/")}
	p.Section, p.Heading = "tasks", "Tasks"
	p.Lead = "A task says what a system must do and how success is measured. It does not say how. Each task page lists the methods that solve it."
	ps := append([]*node.Node(nil), b.c.Kind("task")...)
	sort.Slice(ps, func(i, j int) bool { return ps[i].Name < ps[j].Name })
	g := Group{ID: "all", Title: "All tasks"}
	for _, n := range ps {
		it := b.item(n)
		k := len(b.c.Solvers(n.Slug))
		it.Meta = fmt.Sprintf("%d known %s", k, map[bool]string{true: "method", false: "methods"}[k == 1])
		g.Items = append(g.Items, it)
	}
	if len(g.Items) > 0 {
		p.Groups = []Group{g}
	} else {
		p.Empty = "No task pages exist yet."
	}
	return b.page("list", "/tasks/", p)
}

var kindIntro = func() map[string]string {
	out := map[string]string{}
	for _, k := range node.KindTable {
		out[k.ID] = k.Intro
	}
	return out
}()

func (b *builder) kinds() error {
	for _, k := range node.Kinds {
		if k == "area" {
			continue
		}
		ns := b.c.Kind(k)
		sortNodes(ns)
		name := title(kindName(k, true))
		p := ListPage{Base: b.base(name, kindIntro[k], "/kind/"+k+"/")}
		p.Heading, p.Lead = name, kindIntro[k]
		if k == "method" {
			p.Section = "methods"
		}
		if len(ns) > 0 {
			p.Groups = []Group{{ID: "all", Title: fmt.Sprintf("All %s", kindName(k, true)), Items: b.items(ns)}}
		} else {
			p.Empty = fmt.Sprintf("No %s pages exist yet.", kindName(k))
		}
		if err := b.page("list", "/kind/"+k+"/", p); err != nil {
			return err
		}
	}
	return nil
}

func (b *builder) gallery() error {
	p := ListPage{Base: b.base("Visualize", "Every interactive scene of the site. Step through an algorithm, change the input and watch the state change.", "/visualize/")}
	p.Section, p.Heading = "visualize", "Visualize"
	p.Lead = "Each scene runs the algorithm on an input that you choose. You can step forward and back. The pseudocode shows the current line."
	p.Tiles = b.sceneTiles(0)
	if len(p.Tiles) == 0 {
		p.Empty = "No scenes exist yet."
	}
	return b.page("list", "/visualize/", p)
}

func (b *builder) langTiles() []Tile {
	count := map[string]int{}
	for _, n := range b.c.Nodes {
		for _, im := range n.Impls {
			count[im.Lang]++
		}
	}
	var out []Tile
	for _, l := range b.c.Langs {
		out = append(out, Tile{Name: l.Name, URL: "/languages/" + l.ID + "/", Count: count[l.ID], Unit: "implementation", Hue: 235, Missing: count[l.ID] == 0, Extra: fmt.Sprintf("Tier %d", l.Tier)})
	}
	return out
}

func (b *builder) languages() error {
	p := ListPage{Base: b.base("Languages", "The programming languages of the implementations, with the test framework and the toolchain of each.", "/languages/")}
	p.Section, p.Heading = "languages", "Languages"
	p.Lead = "Each implementation is one file with native tests. The build runs the tests in a pinned toolchain. A tier 1 language is in every build."
	p.Tiles = b.langTiles()
	if err := b.page("list", "/languages/", p); err != nil {
		return err
	}
	for _, l := range b.c.Langs {
		lp := ListPage{Base: b.base(l.Name, fmt.Sprintf("Algorithms and data structures in %s, with native tests.", l.Name), "/languages/"+l.ID+"/")}
		lp.Section, lp.Heading = "languages", l.Name
		lp.Crumbs = []Link{{Name: "Languages", URL: "/languages/"}}
		lp.Lead = fmt.Sprintf("Every implementation in %s on this site.", l.Name)
		lp.Facts = append(lp.Facts, Fact{Key: "Tier", Value: fmt.Sprint(l.Tier)})
		if l.Toolchain != "" {
			lp.Facts = append(lp.Facts, Fact{Key: "Toolchain", Value: l.Toolchain})
		}
		if l.Test.Framework != "" {
			lp.Facts = append(lp.Facts, Fact{Key: "Tests", Value: l.Test.Framework})
		}
		if l.Format.Tool != "" {
			lp.Facts = append(lp.Facts, Fact{Key: "Format", Value: l.Format.Tool})
		}
		if l.Lint.Tool != "" {
			lp.Facts = append(lp.Facts, Fact{Key: "Lint", Value: l.Lint.Tool})
		}
		if l.Run != "" {
			lp.Facts = append(lp.Facts, Fact{Key: "Run the tests", Value: l.Run, Code: true})
		}
		var g Group
		g.ID, g.Title = "all", "Implementations"
		var ns []*node.Node
		for _, n := range b.c.Nodes {
			if n.Lang(l.ID) != nil {
				ns = append(ns, n)
			}
		}
		sort.Slice(ns, func(i, j int) bool { return ns[i].Name < ns[j].Name })
		for _, n := range ns {
			it := b.item(n)
			it.URL = "/" + n.Slug + "/#code-" + l.ID
			it.Chips = nil
			r := b.status[n.Slug].I[l.ID]
			it.Chips = []string{r.Label + " " + r.Name}
			g.Items = append(g.Items, it)
		}
		if len(g.Items) > 0 {
			lp.Groups = []Group{g}
		} else {
			lp.Empty = fmt.Sprintf("No implementation in %s exists yet.", l.Name)
		}
		if err := b.page("list", lp.Path, lp); err != nil {
			return err
		}
	}
	return nil
}

// MatrixPage is the table of every node by every language.
type MatrixPage struct {
	Base
	Langs []node.Lang
	Rows  []MatrixRow
}

// MatrixRow is one node of the matrix.
type MatrixRow struct {
	Link
	H, L, V, W status.Rung
	Cells      []MatrixCell
}

// MatrixCell is one language of a node.
type MatrixCell struct {
	Level int
	Label string
	Title string
	URL   string
	Has   bool
}

func (b *builder) matrix() error {
	p := MatrixPage{Base: b.base("Matrix", "The state of every page: the proof, the Lean proof, the code in each language, the visual and the writing.", "/matrix/"), Langs: b.c.Langs}
	p.Wide = true
	ns := slices.Clone(b.c.Nodes)
	sort.Slice(ns, func(i, j int) bool { return ns[i].Name < ns[j].Name })
	for _, n := range ns {
		if n.Kind != "method" && n.Kind != "architecture" && n.Kind != "metric" {
			continue
		}
		s := b.status[n.Slug]
		row := MatrixRow{Link: b.link(n.Slug), H: s.H, L: s.L, V: s.V, W: s.W}
		for _, l := range b.c.Langs {
			c := MatrixCell{}
			if r, ok := s.I[l.ID]; ok {
				c = MatrixCell{Has: true, Level: r.Level, Label: r.Label, Title: l.Name + ": " + r.Name, URL: "/" + n.Slug + "/#code-" + l.ID}
			}
			row.Cells = append(row.Cells, c)
		}
		p.Rows = append(p.Rows, row)
	}
	return b.page("matrix", "/matrix/", p)
}

var docPages = []struct{ name, title, desc string }{
	{"about", "About", "What this site is, who writes it and how a page earns its badges."},
	{"contribute", "Contribute", "How to write a page, add an implementation, add a scene or review a proof."},
	{"badges", "Badges", "What each level of each ladder means and how the build computes it."},
	{"learn", "Learn", "A reading path through the site, from the first ideas to the core algorithms."},
}

func (b *builder) pages() error {
	for _, d := range docPages {
		src, err := pageFS.ReadFile("pages/" + d.name + ".md")
		if err != nil {
			return err
		}
		r := b.render(string(src))
		p := DocPage{Base: b.base(d.title, d.desc, "/"+d.name+"/"), Heading: d.title, Lead: d.desc, Body: template.HTML(r.HTML), TOC: r.TOC}
		p.Section = d.name
		if d.name == "learn" {
			p.Body += template.HTML(b.snippet2("doc", "learn-path", b.learnPath()))
		}
		if err := b.page("doc", p.Path, p); err != nil {
			return err
		}
	}
	all := slices.Clone(b.c.Nodes)
	sort.Slice(all, func(i, j int) bool { return all[i].Name < all[j].Name })
	sp := ListPage{Base: b.base("Search", "Search every page of the site by name, alias or summary.", "/search/")}
	sp.Heading = "Search"
	sp.Lead = "Type a name, an alias or a word of the summary. Press ⌘K on any page to open the search."
	sp.Groups = []Group{{ID: "all", Title: "All pages, A to Z", Items: b.items(all)}}
	sp.Scripts = []string{"/assets/search.js"}
	if err := b.page("search", "/search/", sp); err != nil {
		return err
	}
	nf := DocPage{Base: b.base("Page not found", "This page does not exist.", "/404/"), Heading: "Page not found", Lead: "This page does not exist yet. Maybe nobody wrote it, or the address has a mistake."}
	nf.NoIndex, nf.BodyKind = true, "notfound"
	nf.Body = template.HTML(`<p>Use the search, or go to the <a href="/areas/">areas</a>. If you think that the page must exist, <a href="` + b.opt.Repo + `/issues/new?template=node.yml">open an issue</a>.</p>`)
	if err := b.page("doc", "/404/", nf); err != nil {
		return err
	}
	data, err := os.ReadFile(filepath.Join(b.opt.Out, "404", "index.html"))
	if err != nil {
		return err
	}
	return b.write("404.html", data)
}

func (b *builder) snippet2(tpl, name string, data any) string {
	var sb strings.Builder
	if err := b.tpl[tpl].ExecuteTemplate(&sb, name, data); err != nil {
		b.opt.Log("template %s: %v", name, err)
	}
	return sb.String()
}

// learnPath orders the intro and core nodes so that each node comes after its prerequisites.
func (b *builder) learnPath() []Group {
	var out []Group
	for _, lv := range []string{"intro", "core"} {
		var ns []*node.Node
		for _, n := range b.c.Nodes {
			if n.Level == lv && n.Kind != "person" && n.Kind != "exercise" {
				ns = append(ns, n)
			}
		}
		sort.Slice(ns, func(i, j int) bool { return ns[i].Name < ns[j].Name })
		done := map[string]bool{}
		var order []*node.Node
		for len(order) < len(ns) {
			moved := false
			for _, n := range ns {
				if done[n.Slug] {
					continue
				}
				ready := true
				for _, q := range n.Prereqs {
					if m := b.c.BySlug[q]; m != nil && m.Level == lv && !done[q] {
						ready = false
					}
				}
				if ready {
					done[n.Slug] = true
					order = append(order, n)
					moved = true
				}
			}
			if !moved { // a cycle; rule G01 reports it
				for _, n := range ns {
					if !done[n.Slug] {
						done[n.Slug] = true
						order = append(order, n)
					}
				}
			}
		}
		if len(order) > 0 {
			out = append(out, Group{ID: lv, Title: map[string]string{"intro": "First steps", "core": "The core"}[lv], Items: b.items(order)})
		}
	}
	return out
}

func (b *builder) spec() error {
	var fsys fs.FS = spec.Files
	if b.opt.SpecDir != "" {
		fsys = os.DirFS(b.opt.SpecDir)
	}
	files, err := fs.Glob(fsys, "*.md")
	if err != nil || len(files) == 0 {
		return err
	}
	slices.Sort(files)
	idx := ListPage{Base: b.base("Specification", "The specification of the site, the content and the toolchain.", "/spec/")}
	idx.Heading = "Specification"
	idx.Lead = "This is the plan of the project. It says what a page contains, how the build checks it and what comes next. The specification changes with the project."
	g := Group{ID: "parts", Title: "Parts"}
	for _, f := range files {
		src, err := fs.ReadFile(fsys, f)
		if err != nil {
			return err
		}
		name := strings.TrimSuffix(filepath.Base(f), ".md")
		slug := strings.ToLower(name)
		heading, body := splitTitle(string(src))
		r := b.render(body)
		p := DocPage{Base: b.base(heading, "Part "+name[:2]+" of the specification.", "/spec/"+slug+"/"), Heading: heading, Body: template.HTML(r.HTML), TOC: r.TOC}
		p.Crumbs = []Link{{Name: "Specification", URL: "/spec/"}}
		p.Source = b.opt.AgentURL + "/blob/main/spec/" + filepath.Base(f)
		if err := b.page("doc", p.Path, p); err != nil {
			return err
		}
		g.Items = append(g.Items, Item{Link: Link{Name: heading, URL: p.Path}, Meta: name[:2]})
	}
	idx.Groups = []Group{g}
	return b.page("list", "/spec/", idx)
}

// splitTitle takes the first "# " heading out of a Markdown file.
func splitTitle(src string) (string, string) {
	lines := strings.Split(src, "\n")
	for i, l := range lines {
		if strings.HasPrefix(l, "# ") {
			return strings.TrimSpace(l[2:]), strings.Join(append(lines[:i:i], lines[i+1:]...), "\n")
		}
	}
	return "Specification", src
}

// machine writes the copies for programs: /<slug>.md, /<slug>.json, /api/nodes.json and /llms.txt (spec 14.2).
func (b *builder) machine() error {
	type apiNode struct {
		node.Front
		URL    string        `json:"url"`
		Path   string        `json:"path"`
		Status status.Status `json:"status"`
		Langs  []string      `json:"langs,omitempty"`
		Proofs []string      `json:"proofs,omitempty"`
		Scene  bool          `json:"scene"`
	}
	var all []apiNode
	var llms strings.Builder
	fmt.Fprintf(&llms, "# Algorithm\n\n> Algorithms and data structures, explained in plain words, proved by hand and in Lean, implemented and tested in many languages, and shown step by step.\n\nEach written page has a Markdown copy at /<slug>.md. The JSON record of every page is in /api/nodes.json. The text is CC BY-SA 4.0. The code is MIT.\n")
	groups := map[string][]*node.Node{}
	for _, n := range b.c.Nodes {
		a := apiNode{Front: n.Front, URL: b.opt.BaseURL + "/" + n.Slug + "/", Path: n.Rel, Status: b.status[n.Slug], Scene: n.Scene != ""}
		for _, im := range n.Impls {
			a.Langs = append(a.Langs, im.Lang)
		}
		for _, pr := range n.Proofs {
			a.Proofs = append(a.Proofs, pr.ID)
		}
		all = append(all, a)
		groups[n.Kind] = append(groups[n.Kind], n)
		if isStub(n) {
			continue
		}
		md, err := os.ReadFile(filepath.Join(n.Dir, "node.md"))
		if err != nil {
			return err
		}
		if err := b.write("/"+n.Slug+".md", md); err != nil {
			return err
		}
	}
	for _, k := range kindOrder {
		ns := groups[k]
		if len(ns) == 0 {
			continue
		}
		sort.Slice(ns, func(i, j int) bool { return ns[i].Name < ns[j].Name })
		fmt.Fprintf(&llms, "\n## %s\n\n", title(kindName(k, true)))
		for _, n := range ns {
			if isStub(n) {
				fmt.Fprintf(&llms, "- [%s](%s/%s/): %s\n", n.Name, b.opt.BaseURL, n.Slug, n.Summary)
				continue
			}
			fmt.Fprintf(&llms, "- [%s](%s/%s.md): %s\n", n.Name, b.opt.BaseURL, n.Slug, n.Summary)
		}
	}
	data, err := json.Marshal(map[string]any{"v": 1, "commit": b.opt.Commit, "nodes": all})
	if err != nil {
		return err
	}
	if err := b.write("/api/nodes.json", data); err != nil {
		return err
	}
	return b.write("/llms.txt", []byte(llms.String()))
}
