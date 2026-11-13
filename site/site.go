// Package site renders the content tree to a static site (spec 14).
//
// Build writes plain files: HTML pages, the machine copies (.md, .json), the search index, the scene bundles and the assets. A static host serves them. The site works with no script: the script adds the search palette, the code tabs, the theme switch and the player.
package site

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/tamnd/ai-agent/bridge"
	"github.com/tamnd/ai-agent/check"
	"github.com/tamnd/ai-agent/formal"
	"github.com/tamnd/ai-agent/katex"
	"github.com/tamnd/ai-agent/markup"
	"github.com/tamnd/ai-agent/node"
	"github.com/tamnd/ai-agent/status"
	"github.com/tamnd/ai-agent/viz"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed assets
var assetFS embed.FS

// Options control a build.
type Options struct {
	Out      string // the output directory; Build empties it first
	BaseURL  string // for example https://ai.tamnd.com
	Results  string // the directory of the test results, build/results
	Decls    string // the declarations of the Lean package, build/lean/decls.jsonl
	SpecDir  string // a directory of the specification to publish at /spec/, or "" for the copy in the binary
	Version  string // the version of aik
	Commit   string // the commit of the content
	Repo     string // for example https://github.com/tamnd/ai
	AgentURL string // for example https://github.com/tamnd/ai-agent
	Minify   bool
	// Now is the build time. A zero time means time.Now.
	Now time.Time
	// Log gets one line for each problem that does not stop the build.
	Log func(format string, args ...any)
}

// Report is what a build made.
type Report struct {
	Pages      int
	Files      int
	Bytes      int64
	MathErrors []string
	Scenes     map[string]viz.Report
}

// Meta is the data that every page shares.
type Meta struct {
	BaseURL  string
	Version  string
	Commit   string
	Short    string
	Date     string
	Repo     string
	AgentURL string
	Counts   Counts
	Year     int
}

// Counts are the numbers of the home page.
type Counts struct {
	Nodes, Methods, Architectures, Tasks, Impls, Langs, Proofs, Scenes, Areas int
}

// Link is a link with a name.
type Link struct {
	Name    string
	URL     string
	Kind    string
	Missing bool
	Note    string
	Hue     int
	// Elsewhere is the mark of a link to a neighbour, such as "on algorithm.tamnd.com" (spec 16.4).
	Elsewhere string
}

// Base is the frame of a page.
type Base struct {
	Title    string
	Desc     string
	Path     string // the canonical path, for example /merge-sort/
	Hue      int
	Section  string // the top bar item that is current
	Crumbs   []Link
	NoIndex  bool
	Scripts  []string
	Wide     bool
	BodyKind string
	Meta     *Meta
	JSONLD   template.JS
}

type builder struct {
	opt     Options
	c       *node.Corpus
	meta    *Meta
	tpl     map[string]*template.Template
	math    *katex.Renderer
	results map[string]map[string]status.Test
	decls   formal.Decls
	scenes  map[string]viz.Report
	status  map[string]status.Status
	hue     map[string]int // by top area id
	report  Report
	sitemap []string
	search  []searchEntry
}

// Build renders the corpus into opt.Out. It writes a new tree next to opt.Out and then puts it in place, so a server that reads opt.Out never sees a half-written site.
func Build(c *node.Corpus, opt Options) (*Report, error) {
	final := filepath.Clean(opt.Out)
	opt.Out = final + ".next"
	r, err := build(c, opt)
	if err != nil {
		_ = os.RemoveAll(opt.Out)
		return nil, err
	}
	old := final + ".old"
	if err := os.RemoveAll(old); err != nil {
		return nil, err
	}
	if err := os.Rename(final, old); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if err := os.Rename(opt.Out, final); err != nil {
		return nil, err
	}
	return r, os.RemoveAll(old)
}

func build(c *node.Corpus, opt Options) (*Report, error) {
	if opt.Now.IsZero() {
		opt.Now = time.Now().UTC()
	}
	if opt.Log == nil {
		opt.Log = func(string, ...any) {}
	}
	if opt.BaseURL == "" {
		opt.BaseURL = "https://ai.tamnd.com"
	}
	opt.BaseURL = strings.TrimRight(opt.BaseURL, "/")
	if opt.Repo == "" {
		opt.Repo = "https://github.com/tamnd/ai"
	}
	if opt.AgentURL == "" {
		opt.AgentURL = "https://github.com/tamnd/ai-agent"
	}
	b := &builder{opt: opt, c: c, scenes: map[string]viz.Report{}, status: map[string]status.Status{}, hue: map[string]int{}}
	b.report.Scenes = b.scenes
	short := opt.Commit
	if len(short) > 7 {
		short = short[:7]
	}
	b.meta = &Meta{BaseURL: opt.BaseURL, Version: opt.Version, Commit: opt.Commit, Short: short, Date: opt.Now.Format("2006-01-02"), Repo: opt.Repo, AgentURL: opt.AgentURL, Year: opt.Now.Year()}
	var err error
	if b.results, err = status.LoadResults(opt.Results); err != nil {
		return nil, err
	}
	if b.decls, err = status.LoadDecls(opt.Decls); err != nil {
		return nil, err
	}
	if b.math, err = katex.New(nil); err != nil {
		return nil, err
	}
	if err := b.parseTemplates(); err != nil {
		return nil, err
	}
	if err := os.RemoveAll(opt.Out); err != nil {
		return nil, err
	}
	for i, a := range c.Areas {
		b.hue[a.ID] = int(float64(i)*137.508+18) % 360
	}
	b.counts()
	if err := b.assets(); err != nil {
		return nil, err
	}
	if err := b.buildScenes(); err != nil {
		return nil, err
	}
	computeClaims(c)
	for _, n := range c.Nodes {
		b.status[n.Slug] = status.Of(n, status.Env{Corpus: c, Results: b.results, VizChecked: b.vizChecked(), Decls: b.decls})
	}
	steps := []func() error{b.nodes, b.home, b.areas, b.tasks, b.kinds, b.collections, b.resultTables, b.gallery, b.languages, b.matrix, b.pages, b.spec, b.machine}
	for _, f := range steps {
		if err := f(); err != nil {
			return nil, err
		}
	}
	if err := b.finish(); err != nil {
		return nil, err
	}
	err = filepath.WalkDir(opt.Out, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		b.report.Files++
		b.report.Bytes += info.Size()
		return nil
	})
	return &b.report, err
}

func (b *builder) vizChecked() map[string][]string {
	out := map[string][]string{}
	for slug, r := range b.scenes {
		if r.Skipped != "" {
			continue
		}
		out[slug] = r.Fails
	}
	return out
}

func (b *builder) counts() {
	k := &b.meta.Counts
	langs := map[string]bool{}
	for _, n := range b.c.Nodes {
		k.Nodes++
		switch n.Kind {
		case "method":
			k.Methods++
		case "architecture":
			k.Architectures++
		case "task":
			k.Tasks++
		}
		k.Impls += len(n.Impls)
		for _, im := range n.Impls {
			langs[im.Lang] = true
		}
		k.Proofs += len(n.Proofs)
		if n.Scene != "" {
			k.Scenes++
		}
	}
	k.Langs = len(langs)
	k.Areas = len(b.c.Areas)
}

var funcs = template.FuncMap{
	"json": func(v any) (template.JS, error) {
		bs, err := json.Marshal(v)
		return template.JS(bs), err
	},
	"plural": func(n int, one, many string) string {
		if n == 1 {
			return fmt.Sprintf("%d %s", n, one)
		}
		return fmt.Sprintf("%s %s", comma(n), many)
	},
	"comma": comma,
	"join":  strings.Join,
	"title": func(s string) string {
		if s == "" {
			return s
		}
		return strings.ToUpper(s[:1]) + s[1:]
	},
	"kindName": kindName,
	"add":      func(a, b int) int { return a + b },
	"pct": func(a, b int) int {
		if b == 0 {
			return 0
		}
		return a * 100 / b
	},
}

func comma(n int) string {
	s := fmt.Sprint(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}

func kindName(k string, many ...bool) string {
	n, ok := node.Kind(k)
	if !ok {
		return k
	}
	if len(many) > 0 && many[0] {
		return n.Plural
	}
	return n.Name
}

func (b *builder) parseTemplates() error {
	b.tpl = map[string]*template.Template{}
	layout, err := template.New("layout.html").Funcs(funcs).ParseFS(templateFS, "templates/layout.html", "templates/parts.html")
	if err != nil {
		return err
	}
	files, err := fs.Glob(templateFS, "templates/*.html")
	if err != nil {
		return err
	}
	for _, f := range files {
		name := filepath.Base(f)
		if name == "layout.html" || name == "parts.html" {
			continue
		}
		t, err := layout.Clone()
		if err != nil {
			return err
		}
		if _, err := t.ParseFS(templateFS, f); err != nil {
			return err
		}
		b.tpl[strings.TrimSuffix(name, ".html")] = t
	}
	return nil
}

// page renders the template name with data into the path p, which ends in / for an index page.
func (b *builder) page(name, p string, data any) error {
	t, ok := b.tpl[name]
	if !ok {
		return fmt.Errorf("no template %s", name)
	}
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "layout.html", data); err != nil {
		return fmt.Errorf("%s: %w", p, err)
	}
	out := p
	if strings.HasSuffix(p, "/") {
		out += "index.html"
		b.sitemap = append(b.sitemap, p)
	}
	b.report.Pages++
	return b.write(out, buf.Bytes())
}

func (b *builder) write(p string, data []byte) error {
	full := filepath.Join(b.opt.Out, filepath.FromSlash(strings.TrimPrefix(p, "/")))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	return os.WriteFile(full, data, 0o644)
}

func (b *builder) base(title, desc, p string) Base {
	return Base{Title: title, Desc: desc, Path: p, Hue: 235, Meta: b.meta}
}

// resolve finds a node or an area for a [[slug]] link.
func (b *builder) resolve(slug string) (string, string, bool) {
	if _, _, ok := bridge.Split(slug); ok {
		if c := b.c.Bridges.Find(slug); c != nil && c.Retired == "" {
			return c.Name, c.URL, true
		}
		return "", "", false
	}
	if n, ok := b.c.BySlug[slug]; ok {
		return n.Name, "/" + n.Slug + "/", true
	}
	if a := b.area(slug); a != nil {
		return a.Name, "/" + a.ID + "/", true
	}
	return "", "", false
}

// area finds a top area or a sub-area by its id.
func (b *builder) area(id string) *node.Area {
	if a, ok := b.c.AreaByID[id]; ok {
		return a
	}
	for _, a := range b.c.AreaByID {
		if a.ID == id {
			return a
		}
	}
	return nil
}

func (b *builder) render(src string) markup.Result {
	r, err := markup.RenderWith(src, markup.Options{Resolve: b.resolve, Math: b.math.Render})
	if err != nil {
		b.opt.Log("markup: %v", err)
	}
	b.report.MathErrors = append(b.report.MathErrors, r.MathErrors...)
	return r
}

func (b *builder) link(slug string) Link {
	if site, s, ok := bridge.Split(slug); ok {
		c := b.c.Bridges.Find(slug)
		if c == nil || c.Retired != "" {
			return Link{Name: humanize(s), Missing: true, Note: "This item is not on " + bridge.Sites[site].Host + "."}
		}
		return Link{Name: c.Name, URL: c.URL, Kind: c.Kind, Elsewhere: c.Mark(), Hue: b.hueOf("")}
	}
	if n, ok := b.c.BySlug[slug]; ok {
		return Link{Name: n.Name, URL: "/" + n.Slug + "/", Kind: n.Kind, Hue: b.hueOf(n.Area)}
	}
	if a := b.area(slug); a != nil {
		return Link{Name: a.Name, URL: "/" + a.ID + "/", Kind: "area", Hue: b.hueOf(a.Path())}
	}
	return Link{Name: humanize(slug), URL: "/" + slug + "/", Missing: true, Note: "This page does not exist yet."}
}

func (b *builder) links(slugs []string) []Link {
	out := make([]Link, 0, len(slugs))
	for _, s := range slugs {
		out = append(out, b.link(s))
	}
	return out
}

func humanize(slug string) string {
	s := strings.ReplaceAll(slug, "-", " ")
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func (b *builder) hueOf(area string) int {
	top, _, _ := strings.Cut(area, "/")
	if h, ok := b.hue[top]; ok {
		return h
	}
	return 235
}

func (b *builder) crumbs(n *node.Node) []Link {
	var out []Link
	top, sub, hasSub := strings.Cut(n.Area, "/")
	if a := b.c.AreaByID[top]; a != nil {
		out = append(out, Link{Name: a.Name, URL: "/" + a.ID + "/"})
	}
	if hasSub {
		if a := b.c.AreaByID[top+"/"+sub]; a != nil {
			out = append(out, Link{Name: a.Name, URL: "/" + a.ID + "/"})
		}
	}
	out = append(out, Link{Name: title(kindName(n.Kind)), URL: "/kind/" + n.Kind + "/"})
	return out
}

func title(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func (b *builder) assets() error {
	err := fs.WalkDir(assetFS, "assets", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := assetFS.ReadFile(p)
		if err != nil {
			return err
		}
		return b.write(p, data)
	})
	if err != nil {
		return err
	}
	if err := b.write("assets/code.css", []byte(markup.HighlightCSS())); err != nil {
		return err
	}
	return fs.WalkDir(katex.Assets(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := fs.ReadFile(katex.Assets(), p)
		if err != nil {
			return err
		}
		return b.write("assets/katex/"+p, data)
	})
}

func (b *builder) buildScenes() error {
	for _, n := range b.c.Nodes {
		if n.Scene == "" {
			continue
		}
		js, err := viz.Build(filepath.Join(b.c.Root, n.Scene), b.opt.Minify)
		if err != nil {
			b.opt.Log("%s: the scene does not build: %v", n.Scene, err)
			b.scenes[n.Slug] = viz.Report{Slug: n.Slug, Fails: []string{"the scene does not build: " + err.Error()}}
			n.Scene = ""
			continue
		}
		var presets []viz.Preset
		for _, p := range n.Presets {
			presets = append(presets, viz.Preset{Name: p.Name, Input: p.Input})
		}
		ids, _ := check.PseudoIDs(n.Body)
		var ref []byte
		var refErr error
		if im := n.Lang("typescript"); im != nil && im.Main() != nil {
			ref, refErr = viz.Reference(im.Main().Content, im.Main().Path)
		}
		r := viz.Check(viz.Scene{Slug: n.Slug, Bundle: js, Presets: presets, IDs: ids["main"], Ref: ref, RefErr: refErr})
		for _, f := range r.Fails {
			b.opt.Log("%s: %s", n.Scene, f)
		}
		b.scenes[n.Slug] = r
		if err := b.write("assets/scenes/"+n.Slug+".js", js); err != nil {
			return err
		}
	}
	return nil
}

func (b *builder) finish() error {
	var sm strings.Builder
	sm.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n" + `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
	slices.Sort(b.sitemap)
	for _, p := range b.sitemap {
		if p == "/404/" {
			continue
		}
		fmt.Fprintf(&sm, "  <url><loc>%s%s</loc><lastmod>%s</lastmod></url>\n", b.opt.BaseURL, p, b.meta.Date)
	}
	sm.WriteString("</urlset>\n")
	rules, moved := b.redirects()
	movedJSON, err := json.Marshal(moved)
	if err != nil {
		return err
	}
	files := map[string]string{
		"_redirects":     rules,
		"api/moved.json": string(movedJSON),
		"sitemap.xml":    sm.String(),
		"robots.txt":     "User-agent: *\nAllow: /\n\nSitemap: " + b.opt.BaseURL + "/sitemap.xml\n",
		"_headers":       headers,
		"opensearch.xml": `<?xml version="1.0" encoding="UTF-8"?>
<OpenSearchDescription xmlns="http://a9.com/-/spec/opensearch/1.1/">
  <ShortName>AI</ShortName>
  <Description>Search the methods, models, tasks and ideas of artificial intelligence.</Description>
  <InputEncoding>UTF-8</InputEncoding>
  <Image width="16" height="16" type="image/svg+xml">` + b.opt.BaseURL + `/assets/icon.svg</Image>
  <Url type="text/html" template="` + b.opt.BaseURL + `/search/?q={searchTerms}"/>
</OpenSearchDescription>
`,
	}
	for name, data := range files {
		if err := b.write(name, []byte(data)); err != nil {
			return err
		}
	}
	sort.Slice(b.search, func(i, j int) bool {
		return b.search[i].Rank > b.search[j].Rank || (b.search[i].Rank == b.search[j].Rank && b.search[i].Name < b.search[j].Name)
	})
	data, err := json.Marshal(b.search)
	if err != nil {
		return err
	}
	return b.write("search.json", data)
}

const headers = `/*
  X-Content-Type-Options: nosniff
  Referrer-Policy: strict-origin-when-cross-origin
  Permissions-Policy: camera=(), microphone=(), geolocation=(), interest-cohort=()
  Cross-Origin-Opener-Policy: same-origin
  Content-Security-Policy: default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'; frame-ancestors 'self'; base-uri 'self'; form-action 'self'
  Strict-Transport-Security: max-age=31536000; includeSubDomains

/embed/*
  Content-Security-Policy: default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; frame-ancestors *; base-uri 'self'; form-action 'self'

/assets/*
  Cache-Control: public, max-age=86400, stale-while-revalidate=604800

/assets/fonts/*
  Cache-Control: public, max-age=31536000, immutable

/assets/katex/*
  Cache-Control: public, max-age=31536000, immutable

/*.json
  Access-Control-Allow-Origin: *

/*.md
  Content-Type: text/markdown; charset=utf-8
  Access-Control-Allow-Origin: *

/*.txt
  Content-Type: text/plain; charset=utf-8
`

// redirects returns the rules of _redirects and the map of the other moved paths. The static assets of a Worker read at most 2,000 rules without a placeholder and 100 with one. The rules hold the old sub-pages and the old slugs of registers/aliases.reg. The tags and the aliases of the nodes are more than 2,000, so they go to /api/moved.json, and the 404 page sends the reader on with the map.
func (b *builder) redirects() (string, map[string]string) {
	var s strings.Builder
	moved := map[string]string{}
	// The code of each language and every proof are on the node page, so the old sub-pages go to their part of it. These rules come first, so that no other rule hides them.
	for _, l := range b.c.Langs {
		fmt.Fprintf(&s, "/:slug/%s/ /:slug/#code-%s 301\n", l.ID, l.ID)
		fmt.Fprintf(&s, "/:slug/%s /:slug/#code-%s 301\n", l.ID, l.ID)
		fmt.Fprintf(&s, "/:slug/%s.txt /:slug/#code-%s 301\n", l.ID, l.ID)
	}
	s.WriteString("/:slug/proof/ /:slug/#proofs 301\n/:slug/proof /:slug/#proofs 301\n")
	for _, n := range b.c.Nodes {
		if n.Tag != "" {
			moved["/t/"+n.Tag] = "/" + n.Slug + "/"
		}
	}
	seen := map[string]bool{}
	for _, n := range b.c.Nodes {
		seen[n.Slug] = true
	}
	// The old slugs of registers/aliases.reg come first: an old URL keeps its target (spec 14.2).
	olds := make([]string, 0, len(b.c.OldSlugs))
	for o := range b.c.OldSlugs {
		olds = append(olds, o)
	}
	sort.Strings(olds)
	for _, o := range olds {
		if to := b.c.OldSlugs[o]; !seen[o] && b.c.BySlug[to] != nil {
			seen[o] = true
			// The splat keeps the pages under the node, such as /old/visualize/.
			fmt.Fprintf(&s, "/%s /%s/ 301\n", o, to)
			fmt.Fprintf(&s, "/%s/* /%s/:splat 301\n", o, to)
			fmt.Fprintf(&s, "/%s.md /%s.md 301\n", o, to)
			fmt.Fprintf(&s, "/embed/%s/ /embed/%s/ 301\n", o, to)
		}
	}
	for _, n := range b.c.Nodes {
		for _, a := range n.Aliases {
			s2 := slugify(a)
			if s2 == "" || seen[s2] || b.c.Reserved[s2] {
				continue
			}
			seen[s2] = true
			moved["/"+s2] = "/" + n.Slug + "/"
		}
	}
	b.cardMoves(moved)
	return s.String(), moved
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	return strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(s), "-"), "-")
}
