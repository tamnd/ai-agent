package site

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/url"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/tamnd/ai-agent/check"
	"github.com/tamnd/ai-agent/facts"
	"github.com/tamnd/ai-agent/markup"
	"github.com/tamnd/ai-agent/node"
	"github.com/tamnd/ai-agent/status"
)

// NodePage is the data of a node page.
type NodePage struct {
	Base
	N        *node.Node
	KindName string
	Body     template.HTML
	TOC      []markup.Heading
	Status   status.Status
	Rungs    []RungView
	Card     *Card
	Prereqs  []Link
	Leads    []Link
	Related  []Link
	Solvers  []SolverRow
	Impls    []ImplView
	Proofs   []ProofView
	Lean     *LeanView
	Player   *PlayerView
	Sources  []SourceView
	Facts    []FactView
	Claims   []ClaimView
	Exps     []ExpView
	Shapes   *ShapesView
	Share    string // the claim share line of "About this page"
	Edit     string
	Copies   bool // the page has the Markdown and JSON copies; a stub has none
	History  string
	Report   string
	Tests    string // the summary line of the native tests
}

// RungView is one row of the ladder table of "About this page".
type RungView struct {
	Ladder string
	Lang   string
	status.Rung
}

// Card is the summary card of a method, an architecture, a metric or a task (spec 16.6).
type Card struct {
	Solves     []Link
	Techniques []Link
	Implements []Link
	Time       []Claim
	Space      []Claim
	Other      []Claim
	Model      *Link
	Vars       []string
	Props      []Prop
	Evidence   []Badge
	Origin     string
}

// Claim is one bound with the cases that share it.
type Claim struct {
	Bound string
	Cases string
	Group string
}

// Prop is a property chip.
type Prop struct {
	Text string
	On   bool
}

// Badge is one evidence badge (spec 13.1).
type Badge struct {
	Text string
	Tick bool
	Href string
	Hint string
}

// SolverRow is one row of the table of the methods that solve a task.
type SolverRow struct {
	Link
	Time  string
	Space string
	Props []Prop
	Langs int
}

// ImplView is one language tab.
type ImplView struct {
	Lang     string
	Name     string
	Tier     int
	File     string
	Lines    int
	HTML     template.HTML
	Tests    []FileView
	Rung     status.Rung
	Test     *status.Test
	Raw      string
	GitHub   string
	Markers  template.JS
	Selected bool
}

// FileView is one more file of an implementation.
type FileView struct {
	Name  string
	Lines int
	HTML  template.HTML
}

// ProofView is one human proof.
type ProofView struct {
	ID     string
	Title  string
	Status string
	Proves []string
	HTML   template.HTML
}

// PlayerView is the data of the player.
type PlayerView struct {
	Slug    string
	Script  string
	Presets template.JS
	Input   template.JS
	Pseudo  template.HTML
	Full    string
}

// FactView is one row of the facts table of a node page (spec 04.9).
type FactView struct {
	ID      string
	Label   string
	Value   string
	Status  string
	AsOf    string
	Note    string
	Sources []FactSourceView
}

// FactSourceView is one source of a fact on the page. A disputed fact shows the value of each source.
type FactSourceView struct {
	Src   string
	Kind  string
	Loc   string
	Value string
	AsOf  string
}

// SourceView is one entry of Further reading.
type SourceView struct {
	Text    string
	URL     string
	Locator string
	Note    string
}

var h2RE = regexp.MustCompile(`(?s)<h2 id="([^"]+)">(.*?)</h2>`)
var idAttrRE = regexp.MustCompile(` id="line-[^"]*"`)
var tagRE = regexp.MustCompile(`<[^>]+>`)

// insertBefore puts extra before the h2 with one of the ids, or at the end.
func insertBefore(html, extra string, ids ...string) string {
	for _, id := range ids {
		if i := strings.Index(html, `<h2 id="`+id+`"`); i >= 0 {
			return html[:i] + extra + html[i:]
		}
	}
	return html + extra
}

// insertAfterSection puts extra at the end of the section with the id, or before the h2 with one of the ids, or at the end.
func insertAfterSection(html, extra, id string, fallback ...string) string {
	i := strings.Index(html, `<h2 id="`+id+`"`)
	if i < 0 {
		return insertBefore(html, extra, fallback...)
	}
	j := strings.Index(html[i+4:], "<h2 ")
	if j < 0 {
		return html + extra
	}
	j += i + 4
	return html[:j] + extra + html[j:]
}

// dropEmptyH2 removes the heading of a section that the build fills, when the build had nothing to put in it.
func dropEmptyH2(html, id string) string {
	i := strings.Index(html, `<h2 id="`+id+`"`)
	if i < 0 {
		return html
	}
	end := strings.Index(html[i:], "</h2>")
	if end < 0 {
		return html
	}
	end += i + len("</h2>")
	rest := strings.TrimLeft(html[end:], " \n")
	if rest == "" || strings.HasPrefix(rest, "<h2 ") {
		return html[:i] + rest
	}
	return html
}

func hasH2(html, id string) bool { return strings.Contains(html, `<h2 id="`+id+`"`) }

func toc(html string) []markup.Heading {
	var out []markup.Heading
	for _, m := range h2RE.FindAllStringSubmatch(html, -1) {
		out = append(out, markup.Heading{Level: 2, ID: m[1], Text: strings.TrimSpace(tagRE.ReplaceAllString(m[2], ""))})
	}
	return out
}

func (b *builder) nodes() error {
	for _, n := range b.c.Nodes {
		p, err := b.nodePage(n)
		if err != nil {
			return err
		}
		if err := b.page("node", "/"+n.Slug+"/", p); err != nil {
			return err
		}
		// The code of each language and every proof are on the node page. _redirects sends the old language and proof pages there (spec 14.2).
		if p.Lean != nil {
			lp := *p
			lp.Base = b.base("Lean: "+n.Name, "The Lean model, specification and proofs of "+n.Name+".", p.Lean.URL)
			lp.Hue, lp.Crumbs = p.Hue, append(slices.Clone(p.Crumbs), Link{Name: n.Name, URL: "/" + n.Slug + "/"})
			if err := b.page("lean", lp.Path, lp); err != nil {
				return err
			}
		}
		if p.Player != nil {
			for _, kind := range []string{"visualize", "embed"} {
				vp := *p
				path := "/" + n.Slug + "/visualize/"
				if kind == "embed" {
					path = "/embed/" + n.Slug + "/"
				}
				vp.Base = b.base("See "+n.Name+" run", "An interactive, step-by-step picture of "+n.Name+".", path)
				vp.Hue, vp.Wide, vp.BodyKind = p.Hue, true, kind
				vp.Crumbs = append(slices.Clone(p.Crumbs), Link{Name: n.Name, URL: "/" + n.Slug + "/"})
				vp.Scripts = []string{p.Player.Script, "/assets/player.js"}
				if kind == "embed" {
					vp.NoIndex = true
				}
				if err := b.page(kind, path, vp); err != nil {
					return err
				}
			}
		}
		b.addSearch(n)
	}
	return nil
}

func (b *builder) nodePage(n *node.Node) (*NodePage, error) {
	p := &NodePage{N: n, KindName: kindName(n.Kind)}
	p.Base = b.base(n.Name, n.Summary, "/"+n.Slug+"/")
	p.Hue = b.hueOf(n.Area)
	p.Crumbs = b.crumbs(n)
	p.Status = b.status[n.Slug]
	p.Edit = fmt.Sprintf("%s/edit/main/%s/node.md", b.opt.Repo, filepath.ToSlash(n.Rel))
	p.Copies = !isStub(n)
	p.History = fmt.Sprintf("%s/commits/main/%s", b.opt.Repo, filepath.ToSlash(n.Rel))
	p.Report = b.opt.Repo + "/issues/new?" + url.Values{"template": {"error.yml"}, "title": {"Error on " + n.Name}, "url": {b.opt.BaseURL + "/" + n.Slug + "/"}}.Encode()
	p.Prereqs = b.links(n.Prereqs)
	for _, l := range b.c.Leads(n.Slug) {
		p.Leads = append(p.Leads, b.link(l.Slug))
	}
	rel := slices.Clone(n.SeeAlso)
	for _, s := range n.RelatedTo {
		if !slices.Contains(rel, s) {
			rel = append(rel, s)
		}
	}
	if n.VariantOf != "" {
		rel = append(rel, n.VariantOf)
	}
	for _, s := range n.Solves {
		for _, o := range b.c.Solvers(s.Problem) {
			if o.Slug != n.Slug && !slices.Contains(rel, o.Slug) {
				rel = append(rel, o.Slug)
			}
		}
	}
	if len(rel) > 8 {
		rel = rel[:8]
	}
	p.Related = b.links(rel)

	r := b.render(factRefs(b.c, shapeRefs(n.Shapes, n.Body)))
	html := b.claimMarks(n, r.HTML)
	p.Impls = b.implViews(n)
	p.Proofs = b.proofViews(n)
	if n.Scene != "" {
		p.Player = b.playerView(n, r)
		p.Scripts = []string{p.Player.Script, "/assets/player.js"}
		html = insertAfterSection(html, b.snippet("player-section", p), "in-plain-words", "the-method", "overview", "definition")
	}
	if len(p.Proofs) > 0 {
		html = insertAfterSection(html, b.snippet("proofs-inline", p), "correctness", "complexity", "history", "references")
	}
	if n.Kind == "task" {
		p.Solvers = b.solverRows(n)
		if len(p.Solvers) > 0 {
			if hasH2(html, "approaches") {
				html = insertAfterSection(html, b.snippet("solver-table", p), "approaches")
			} else {
				html = insertBefore(html, b.snippet("approaches", p), "results", "difficulties", "applications", "history", "references")
			}
		}
	}
	p.Lean = b.leanView(n)
	if p.Lean != nil {
		if hasH2(html, "formal-proof") {
			html = insertAfterSection(html, b.snippet("formal-body", p), "formal-proof")
		} else {
			html = insertBefore(html, b.snippet("formal-section", p), "history", "exercises", "references")
		}
	}
	p.Shapes = b.shapesView(n)
	if p.Shapes != nil && p.Shapes.Arch != nil {
		p.Scripts = append(p.Scripts, "/assets/arch.js")
	}
	if p.Shapes != nil {
		if hasH2(html, "shapes-and-parameters") {
			html = insertAfterSection(html, b.snippet("shapes-body", p), "shapes-and-parameters")
		} else {
			html = insertBefore(html, b.snippet("shapes-section", p), "computation-and-cost", "training", "inductive-biases", "evidence", "limitations", "history", "references")
		}
	}
	if len(p.Impls) > 0 {
		if hasH2(html, "implementations") {
			html = insertAfterSection(html, b.snippet("code-body", p), "implementations")
		} else {
			html = insertBefore(html, b.snippet("code-section", p), "formal-proof", "history", "exercises", "references")
		}
	}
	p.Facts = factViews(n.Facts)
	p.Claims = claimViews(n)
	p.Share = claimShare(n)
	if len(p.Claims) > 0 {
		if hasH2(html, "evidence") {
			html = insertAfterSection(html, b.snippet("claim-table", p), "evidence")
		} else {
			html = insertBefore(html, b.snippet("evidence-section", p), "history", "exercises", "references")
		}
	}
	p.Exps = b.expViews(n)
	if len(p.Exps) > 0 {
		if hasH2(html, "experiments") {
			html = insertAfterSection(html, b.snippet("exp-list", p), "experiments")
		} else {
			html = insertBefore(html, b.snippet("exp-section", p), "history", "exercises", "references")
		}
	}
	p.Sources = b.sourceViews(n)
	if len(p.Sources) > 0 {
		if hasH2(html, "references") {
			html = insertAfterSection(html, b.snippet("refs-list", p), "references")
		} else {
			html += b.snippet("refs-section", p)
		}
	}
	for _, id := range []string{"implementations", "approaches", "references"} {
		html = dropEmptyH2(html, id)
	}
	p.Body = template.HTML(html)
	p.TOC = toc(html)
	p.Card = b.card(n, p)
	p.Rungs = rungs(p.Status)
	p.Tests = testLine(p)
	p.JSONLD = jsonLD(b, n)
	return p, nil
}

func testLine(p *NodePage) string {
	pass, ran := 0, 0
	for _, im := range p.Impls {
		if im.Test != nil {
			ran++
			if im.Test.Pass {
				pass++
			}
		}
	}
	if ran == 0 {
		return ""
	}
	return fmt.Sprintf("The native tests pass in %d of %d languages in this build.", pass, len(p.Impls))
}

func rungs(s status.Status) []RungView {
	out := []RungView{{Ladder: "Human proof", Rung: s.H}, {Ladder: "Lean", Rung: s.L}}
	langs := make([]string, 0, len(s.I))
	for l := range s.I {
		langs = append(langs, l)
	}
	slices.Sort(langs)
	for _, l := range langs {
		out = append(out, RungView{Ladder: "Code", Lang: l, Rung: s.I[l]})
	}
	out = append(out, RungView{Ladder: "Visual", Rung: s.V}, RungView{Ladder: "Writing", Rung: s.W}, RungView{Ladder: "Grounding", Rung: s.G})
	if s.D != nil {
		out = append(out, RungView{Ladder: "Data facts", Rung: *s.D})
	}
	if s.R != nil {
		out = append(out, RungView{Ladder: "Reproduction", Rung: *s.R})
	}
	return out
}

func (b *builder) snippet(name string, data any) string {
	var sb strings.Builder
	if err := b.tpl["node"].ExecuteTemplate(&sb, name, data); err != nil {
		b.opt.Log("template %s: %v", name, err)
	}
	return sb.String()
}

// order of the cases of a claim (spec 14.7.4).
var caseOrder = []string{"worst", "average", "best", "amortized", "expected", "extra", "total"}

func claimsOf(group string, m map[string]string) []Claim {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := slices.Index(caseOrder, keys[i]), slices.Index(caseOrder, keys[j])
		if a < 0 {
			a = 99
		}
		if b < 0 {
			b = 99
		}
		if a != b {
			return a < b
		}
		return keys[i] < keys[j]
	})
	var out []Claim
	for _, k := range keys {
		v := m[k]
		if i := slices.IndexFunc(out, func(c Claim) bool { return c.Bound == v }); i >= 0 {
			out[i].Cases += " · " + k
			continue
		}
		out = append(out, Claim{Bound: v, Cases: k, Group: group})
	}
	return out
}

var propNames = map[string][2]string{
	"stable":        {"stable", "not stable"},
	"in_place":      {"in place", "not in place"},
	"adaptive":      {"adaptive", "not adaptive"},
	"online":        {"online", "offline"},
	"deterministic": {"deterministic", "randomized"},
	"comparison":    {"comparison based", "not comparison based"},
	"recursive":     {"recursive", "iterative"},
}

func props(m map[string]any) []Prop {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	var out []Prop
	for _, k := range keys {
		v, ok := m[k].(bool)
		if !ok {
			continue
		}
		names, known := propNames[k]
		if !known {
			names = [2]string{strings.ReplaceAll(k, "_", " "), "not " + strings.ReplaceAll(k, "_", " ")}
		}
		if v {
			out = append(out, Prop{Text: names[0], On: true})
		} else {
			out = append(out, Prop{Text: names[1]})
		}
	}
	return out
}

func (b *builder) card(n *node.Node, p *NodePage) *Card {
	if n.Kind != "method" && n.Kind != "architecture" && n.Kind != "metric" && n.Kind != "task" {
		return nil
	}
	c := &Card{}
	for _, s := range n.Solves {
		c.Solves = append(c.Solves, b.link(s.Problem))
	}
	c.Techniques = b.links(n.Techniques)
	c.Implements = b.links(n.Implements)
	if x := n.Complexity; x != nil {
		c.Time = claimsOf("time", x.Time)
		c.Space = claimsOf("space", x.Space)
		groups := make([]string, 0, len(x.Other))
		for g := range x.Other {
			groups = append(groups, g)
		}
		slices.Sort(groups)
		for _, g := range groups {
			c.Other = append(c.Other, claimsOf(strings.ReplaceAll(g, "_", " "), x.Other[g])...)
		}
		for _, k := range sortedKeys(x.Vars) {
			c.Vars = append(c.Vars, k+" = "+x.Vars[k])
		}
	}
	if n.Model != "" {
		l := b.link(n.Model)
		c.Model = &l
	}
	c.Props = props(n.Properties)
	s := p.Status
	switch {
	case s.L.Level >= 3:
		c.Evidence = append(c.Evidence, Badge{Text: "Proved in Lean: correctness and cost", Tick: true, Href: "#formal-proof"})
	case s.L.Level == 2:
		c.Evidence = append(c.Evidence, Badge{Text: "Proved in Lean: correctness", Tick: true, Href: "#formal-proof"})
	}
	switch {
	case s.H.Level >= 3:
		c.Evidence = append(c.Evidence, Badge{Text: "Proof reviewed", Tick: true, Href: "#correctness"})
	case s.H.Level == 2:
		c.Evidence = append(c.Evidence, Badge{Text: "Full proof", Href: "#correctness"})
	case s.H.Level == 1:
		c.Evidence = append(c.Evidence, Badge{Text: "Proof sketch", Href: "#correctness"})
	}
	tested := 0
	var names []string
	for _, im := range p.Impls {
		if im.Rung.Level >= 2 {
			tested++
		}
		names = append(names, im.Name)
	}
	if tested > 0 {
		c.Evidence = append(c.Evidence, Badge{Text: fmt.Sprintf("Tested in %d languages", tested), Tick: true, Href: "#code-tabs"})
	} else if len(p.Impls) > 0 {
		c.Evidence = append(c.Evidence, Badge{Text: fmt.Sprintf("Code in %d languages", len(p.Impls)), Href: "#code-tabs", Hint: strings.Join(names, ", ")})
	}
	switch {
	case s.V.Level >= 2:
		c.Evidence = append(c.Evidence, Badge{Text: "Visual", Tick: true, Href: "#see-it-run"})
	case p.Player != nil:
		c.Evidence = append(c.Evidence, Badge{Text: "Interactive scene", Href: "#see-it-run"})
	}
	if h := n.History; h != nil && h.Year > 0 {
		var who []string
		for _, l := range b.links(h.People) {
			who = append(who, l.Name)
		}
		c.Origin = strings.TrimSpace(strings.Join(who, ", ") + fmt.Sprintf(", %d", h.Year))
		c.Origin = strings.TrimPrefix(c.Origin, ", ")
	}
	return c
}

func sortedKeys[V any](m map[string]V) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	slices.Sort(ks)
	return ks
}

func (b *builder) solverRows(n *node.Node) []SolverRow {
	var out []SolverRow
	for _, s := range b.c.Solvers(n.Slug) {
		row := SolverRow{Link: b.link(s.Slug), Props: props(s.Properties), Langs: len(s.Impls)}
		if s.Complexity != nil {
			if t := claimsOf("time", s.Complexity.Time); len(t) > 0 {
				row.Time = t[0].Bound + " " + t[0].Cases
			}
			if sp := claimsOf("space", s.Complexity.Space); len(sp) > 0 {
				row.Space = sp[0].Bound + " " + sp[0].Cases
			}
		}
		out = append(out, row)
	}
	return out
}

func (b *builder) implViews(n *node.Node) []ImplView {
	var out []ImplView
	for _, im := range n.Impls {
		lang := b.c.LangByID[im.Lang]
		v := ImplView{Lang: im.Lang, Name: im.Lang, Tier: 9, Rung: b.status[n.Slug].I[im.Lang]}
		lexer := im.Lang
		if lang != nil {
			v.Name, v.Tier = lang.Name, lang.Tier
			if lang.Highlight != "" {
				lexer = lang.Highlight
			}
		}
		if t, ok := b.results[n.Slug][im.Lang]; ok {
			v.Test = &t
		}
		main := im.Main()
		if main == nil {
			continue
		}
		v.File = main.Name
		v.Lines = strings.Count(main.Content, "\n")
		v.HTML = template.HTML(markup.Highlight(main.Content, lexer))
		v.Raw = fmt.Sprintf("%s/raw/main/%s", b.opt.Repo, filepath.ToSlash(main.Path))
		v.GitHub = fmt.Sprintf("%s/blob/main/%s", b.opt.Repo, filepath.ToSlash(main.Path))
		v.Markers = markerMap(main.Content)
		for _, f := range im.Files {
			if f.Name == main.Name {
				continue
			}
			v.Tests = append(v.Tests, FileView{Name: f.Name, Lines: strings.Count(f.Content, "\n"), HTML: template.HTML(markup.Highlight(f.Content, lexer))})
		}
		out = append(out, v)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Tier != out[j].Tier {
			return out[i].Tier < out[j].Tier
		}
		return b.langIndex(out[i].Lang) < b.langIndex(out[j].Lang)
	})
	if len(out) > 0 {
		out[0].Selected = true
	}
	return out
}

func (b *builder) langIndex(id string) int {
	for i, l := range b.c.Langs {
		if l.ID == id {
			return i
		}
	}
	return 1 << 20
}

// markerMap gives, for each pseudocode line id, the 1-based source lines that carry its @line marker.
func markerMap(src string) template.JS {
	m := map[string][]int{}
	for i, l := range strings.Split(src, "\n") {
		for _, id := range check.Markers(l) {
			m[id] = append(m[id], i+1)
		}
	}
	bs, _ := json.Marshal(m)
	return template.JS(bs)
}

func (b *builder) proofViews(n *node.Node) []ProofView {
	var out []ProofView
	for _, pr := range n.Proofs {
		r := b.render(pr.Body)
		t := pr.Front.Title
		if t == "" {
			t = humanize(pr.ID)
		}
		out = append(out, ProofView{ID: pr.ID, Title: t, Status: pr.Front.Status, Proves: pr.Front.Proves, HTML: template.HTML(r.HTML)})
	}
	return out
}

func (b *builder) playerView(n *node.Node, r markup.Result) *PlayerView {
	presets, _ := json.Marshal(n.Presets)
	pseudo := ""
	if i := strings.Index(r.HTML, `<ol class="pseudo"`); i >= 0 {
		if j := strings.Index(r.HTML[i:], "</ol>"); j >= 0 {
			pseudo = r.HTML[i : i+j+5]
		}
	}
	return &PlayerView{
		Slug:    n.Slug,
		Script:  "/assets/scenes/" + n.Slug + ".js",
		Presets: template.JS(presets),
		Pseudo:  template.HTML(idAttrRE.ReplaceAllString(strings.Replace(pseudo, `<ol class="pseudo"`, `<ol class="pseudo player-code"`, 1), "")),
		Full:    "/" + n.Slug + "/visualize/",
	}
}

func factViews(fs []facts.Fact) []FactView {
	out := make([]FactView, 0, len(fs))
	for _, f := range fs {
		v := FactView{ID: f.ID, Label: facts.Label(f.ID), Value: facts.Format(f.Value, f.Unit), Status: f.Status, AsOf: facts.AsOf(f), Note: f.Note}
		for _, s := range f.Sources {
			loc := s.Loc
			if loc == "" && s.Row != "" {
				loc = "row " + s.Row
			}
			v.Sources = append(v.Sources, FactSourceView{Src: s.Src, Kind: s.Kind, Loc: loc, Value: facts.Format(s.Value, f.Unit), AsOf: s.AsOf})
		}
		out = append(out, v)
	}
	return out
}

func (b *builder) sourceViews(n *node.Node) []SourceView {
	var out []SourceView
	for _, s := range n.Sources {
		v := SourceView{Locator: s.Locator, Note: s.Note, URL: s.URL}
		switch {
		case strings.HasPrefix(s.Book, "isbn:"):
			v.Text = "ISBN " + strings.TrimPrefix(s.Book, "isbn:")
			if v.URL == "" {
				v.URL = "https://openlibrary.org/isbn/" + strings.TrimPrefix(s.Book, "isbn:")
			}
		case strings.HasPrefix(s.Paper, "doi:"):
			v.Text = s.Paper
			if v.URL == "" {
				v.URL = "https://doi.org/" + strings.TrimPrefix(s.Paper, "doi:")
			}
		case s.Book != "":
			v.Text = s.Book
		case s.Paper != "":
			v.Text = s.Paper
		default:
			v.Text = s.URL
		}
		if s.Title != "" {
			v.Text = s.Title
		}
		out = append(out, v)
	}
	return out
}

func jsonLD(b *builder, n *node.Node) template.JS {
	t := "DefinedTerm"
	if n.Kind == "method" || n.Kind == "architecture" || n.Kind == "metric" {
		t = "TechArticle"
	}
	m := map[string]any{
		"@context":    "https://schema.org",
		"@type":       t,
		"name":        n.Name,
		"headline":    n.Name,
		"description": n.Summary,
		"url":         b.opt.BaseURL + "/" + n.Slug + "/",
		"identifier":  "ai://" + n.Tag,
		"license":     "https://creativecommons.org/licenses/by-sa/4.0/",
		"isPartOf":    map[string]string{"@type": "WebSite", "name": "AI", "url": b.opt.BaseURL + "/"},
	}
	if len(n.Aliases) > 0 {
		m["alternateName"] = n.Aliases
	}
	bs, _ := json.Marshal(m)
	return template.JS(bs)
}

// isStub reports whether a node has no section yet. A stub has only its front matter and a lead, so the site gives it no Markdown or JSON copy. This keeps the file count of a large skeleton under the limit of the host.
func isStub(n *node.Node) bool {
	return !strings.HasPrefix(n.Body, "## ") && !strings.Contains(n.Body, "\n## ")
}
