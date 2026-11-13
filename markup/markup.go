// Package markup turns the Markdown of nodes and proofs into HTML (spec 06.2).
//
// It adds four things to CommonMark and GFM:
//   - [[slug]] and [[slug|text]] link to a node;
//   - $...$ is inline math, and $$...$$ or a ```math block is display math, which KaTeX renders at build time;
//   - a ```pseudo block is pseudocode with stable line ids (spec 06.4);
//   - other fenced blocks are highlighted with Chroma at build time.
package markup

import (
	"bytes"
	"fmt"
	"html"
	"regexp"
	"strings"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// Resolver finds the name and the URL of a node by its slug.
type Resolver func(slug string) (name, url string, ok bool)

// MathFunc renders TeX to HTML. display selects display math.
type MathFunc func(tex string, display bool) (string, error)

// Options are the inputs of RenderWith.
type Options struct {
	Resolve Resolver
	// Math renders formulas at build time. When it is nil, a formula is left as TeX between \( \) or \[ \], for a renderer in the browser.
	Math MathFunc
}

// Heading is one entry of the table of contents.
type Heading struct {
	Level int
	ID    string
	Text  string
}

// Result is the output of Render.
type Result struct {
	HTML       string
	TOC        []Heading
	Missing    []string // slugs of [[links]] that do not resolve
	LineIDs    []string // the line ids of the pseudocode blocks, in order
	Sections   []string // the texts of the level 2 headings, in order
	MathErrors []string // the formulas that do not render (rule W07)
}

// wikiLink finds [[slug]], [[slug|label]], and a link to a neighbour such as [[al:k-means]] (spec 6.2).
var wikiLink = regexp.MustCompile(`\[\[((?:(?:al|m2|pg):)?[a-z0-9][a-z0-9-]*)(?:\|([^\]]+))?\]\]`)

// Render renders Markdown to HTML.
func Render(src string, resolve Resolver) (Result, error) {
	return RenderWith(src, Options{Resolve: resolve})
}

// RenderWith renders Markdown to HTML with the options.
func RenderWith(src string, opt Options) (Result, error) {
	var res Result
	src, bad := displayMath(src)
	res.MathErrors = append(res.MathErrors, bad...)
	src = linkWiki(src, opt.Resolve, &res)
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM, extension.Footnote, mathExt{}),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		goldmark.WithRendererOptions(renderer.WithNodeRenderers(util.Prioritized(&codeRenderer{res: &res, math: opt.Math}, 100))),
	)
	source := []byte(src)
	doc := md.Parser().Parse(text.NewReader(source))
	err := ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		h, ok := n.(*ast.Heading)
		if !ok || !entering {
			return ast.WalkContinue, nil
		}
		id, _ := h.AttributeString("id")
		idb, _ := id.([]byte)
		t := plainText(h, source)
		if h.Level == 2 {
			res.Sections = append(res.Sections, t)
		}
		if h.Level <= 3 {
			res.TOC = append(res.TOC, Heading{Level: h.Level, ID: string(idb), Text: t})
		}
		return ast.WalkContinue, nil
	})
	if err != nil {
		return res, err
	}
	var buf bytes.Buffer
	if err := md.Renderer().Render(&buf, source, doc); err != nil {
		return res, err
	}
	res.HTML = buf.String()
	return res, nil
}

func plainText(n ast.Node, src []byte) string {
	var b strings.Builder
	_ = ast.Walk(n, func(c ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			if t, ok := c.(*ast.Text); ok {
				b.Write(t.Segment.Value(src))
			}
			if t, ok := c.(*ast.String); ok {
				b.Write(t.Value)
			}
			if m, ok := c.(*mathInline); ok {
				b.WriteString(m.value)
			}
		}
		return ast.WalkContinue, nil
	})
	return b.String()
}

// linkWiki changes [[slug]] to a Markdown link, outside of fenced code.
func linkWiki(src string, resolve Resolver, res *Result) string {
	lines := strings.Split(src, "\n")
	fence := ""
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
			if fence == "" {
				fence = t[:3]
			} else if strings.HasPrefix(t, fence) {
				fence = ""
			}
			continue
		}
		if fence != "" {
			continue
		}
		lines[i] = wikiLink.ReplaceAllStringFunc(l, func(m string) string {
			sm := wikiLink.FindStringSubmatch(m)
			slug, label := sm[1], sm[2]
			name, url, ok := "", "", false
			if resolve != nil {
				name, url, ok = resolve(slug)
			}
			if !ok {
				res.Missing = append(res.Missing, slug)
				if label == "" {
					label = slug
				}
				return label
			}
			if label == "" {
				label = name
			}
			return "[" + label + "](" + url + ")"
		})
	}
	return strings.Join(lines, "\n")
}

// codeRenderer renders fenced code blocks.
type codeRenderer struct {
	res  *Result
	math MathFunc
}

func (r *codeRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindFencedCodeBlock, r.render)
	reg.Register(kindMathInline, r.renderMathInline)
}

// formula writes one formula, rendered at build time when it can be.
func (r *codeRenderer) formula(w util.BufWriter, tex string, display bool) {
	cls, open, closeTag := "math", `\(`, `\)`
	if display {
		cls, open, closeTag = "math display", `\[`, `\]`
	}
	if r.math != nil {
		out, err := r.math(tex, display)
		if err == nil {
			tag := "span"
			if display {
				tag = "div"
			}
			fmt.Fprintf(w, "<%s class=\"%s\">%s</%s>", tag, cls, out, tag)
			return
		}
		r.res.MathErrors = append(r.res.MathErrors, tex+": "+err.Error())
		fmt.Fprintf(w, "<code class=\"math-error\" title=\"%s\">%s</code>", html.EscapeString(err.Error()), html.EscapeString(tex))
		return
	}
	if display {
		fmt.Fprintf(w, "<div class=\"%s\">%s%s%s</div>\n", cls, open, html.EscapeString(tex), closeTag)
		return
	}
	fmt.Fprintf(w, "<span class=\"%s\">%s%s%s</span>", cls, open, html.EscapeString(tex), closeTag)
}

func (r *codeRenderer) render(w util.BufWriter, src []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	b := n.(*ast.FencedCodeBlock)
	lang := string(b.Language(src))
	var code strings.Builder
	for i := 0; i < b.Lines().Len(); i++ {
		seg := b.Lines().At(i)
		code.Write(seg.Value(src))
	}
	switch lang {
	case "pseudo", "pseudocode":
		ids := Pseudo(w, code.String())
		r.res.LineIDs = append(r.res.LineIDs, ids...)
	case "math":
		r.formula(w, strings.TrimSpace(code.String()), true)
	default:
		_, _ = w.WriteString(Highlight(code.String(), lang))
	}
	return ast.WalkSkipChildren, nil
}

var pseudoLine = regexp.MustCompile(`^(L\d+)( +)(.*)$`)
var pseudoKeywords = regexp.MustCompile(`\b(if|then|else|while|do|for|to|downto|each|in|repeat|until|return|and|or|not|nil|break|continue|true|false)\b`)
var pseudoCalls = regexp.MustCompile(`\b([A-Z][A-Z0-9]*(?:-[A-Z0-9]+)+|[A-Z]{3,}[A-Z0-9]*)\b`)

// Pseudo writes a pseudocode block (spec 06.4). A line that starts with an id such as L3 is a step. A line with no id is a header, such as "MERGE-SORT(A, p, r)". The text after ▷ is a comment. Words in upper case are calls, and they render in small capitals.
// It returns the ids in order.
func Pseudo(w util.BufWriter, code string) []string {
	var ids []string
	_, _ = w.WriteString("<ol class=\"pseudo\" aria-label=\"Pseudocode\">\n")
	n := 0
	lines := strings.Split(strings.TrimRight(code, "\n"), "\n")
	// col is the column where the text of the steps starts. Each 4 spaces after it are one level of indent.
	col := -1
	for _, l := range lines {
		if m := pseudoLine.FindStringSubmatch(l); m != nil {
			if c := len(m[1]) + len(m[2]); col < 0 || c < col {
				col = c
			}
		}
	}
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			_, _ = w.WriteString("<li class=\"gap\" aria-hidden=\"true\"></li>\n")
			continue
		}
		m := pseudoLine.FindStringSubmatch(l)
		id, body := "", l
		indent := (len(body) - len(strings.TrimLeft(body, " "))) / 4
		if m != nil {
			id, body = m[1], m[3]
			ids = append(ids, id)
			indent = (len(m[1]) + len(m[2]) - col) / 4
		}
		body = strings.TrimLeft(body, " ")
		comment := ""
		if i := strings.Index(body, "▷"); i >= 0 {
			comment = `<span class="c">` + html.EscapeString(strings.TrimSpace(body[i:])) + `</span>`
			body = strings.TrimRight(body[:i], " ")
		}
		text := html.EscapeString(body)
		text = pseudoCalls.ReplaceAllString(text, `<span class="sc">$1</span>`)
		text = pseudoKeywords.ReplaceAllString(text, `<b>$1</b>`)
		if comment != "" {
			text += " " + comment
		}
		if id == "" {
			fmt.Fprintf(w, "<li class=\"head\"><span class=\"lid\"></span><code>%s</code></li>\n", text)
			continue
		}
		n++
		fmt.Fprintf(w, "<li id=\"line-%s\" data-line=\"%s\"><span class=\"lid\" title=\"%s\">%d</span><code style=\"--indent:%d\">%s</code></li>\n", id, id, id, n, indent, text)
	}
	_, _ = w.WriteString("</ol>\n")
	return ids
}

// Highlight returns highlighted HTML for code in a language. It uses CSS classes, so that the light and the dark themes can share the markup.
func Highlight(code, lang string) string {
	lexer := lexers.Get(lang)
	if lexer == nil {
		lexer = lexers.Fallback
	}
	it, err := lexer.Tokenise(nil, code)
	if err != nil {
		return "<pre><code>" + html.EscapeString(code) + "</code></pre>\n"
	}
	var buf bytes.Buffer
	f := chromahtml.New(chromahtml.WithClasses(true), chromahtml.WithLineNumbers(false), chromahtml.PreventSurroundingPre(false))
	if err := f.Format(&buf, styles.Get("github"), it); err != nil {
		return "<pre><code>" + html.EscapeString(code) + "</code></pre>\n"
	}
	return buf.String()
}

// HighlightCSS returns the CSS of the light and the dark code themes.
func HighlightCSS() string {
	var light, dark bytes.Buffer
	f := chromahtml.New(chromahtml.WithClasses(true))
	_ = f.WriteCSS(&light, styles.Get("github"))
	_ = f.WriteCSS(&dark, styles.Get("github-dark"))
	var b strings.Builder
	b.WriteString("/* Code colours from Chroma: github and github-dark. */\n")
	b.WriteString(stripBackground(light.String()))
	b.WriteString("@media (prefers-color-scheme: dark) {\n:root:not([data-theme=light]) {\n")
	b.WriteString(scope(stripBackground(dark.String()), ":root:not([data-theme=light]) "))
	b.WriteString("}\n}\n")
	b.WriteString(scope(stripBackground(dark.String()), ":root[data-theme=dark] "))
	return b.String()
}

// stripBackground drops the background of the code block, so that the site sets it.
func stripBackground(css string) string {
	var out []string
	for _, l := range strings.Split(css, "\n") {
		if strings.Contains(l, ".bg {") || strings.Contains(l, ".chroma {") {
			l = regexp.MustCompile(`background-color:\s*#[0-9a-fA-F]+;?`).ReplaceAllString(l, "")
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n") + "\n"
}

func scope(css, prefix string) string {
	var out []string
	for _, l := range strings.Split(css, "\n") {
		if strings.HasPrefix(l, ".") {
			l = prefix + l
		} else if strings.HasPrefix(l, "/*") && strings.Contains(l, "*/ .") {
			i := strings.Index(l, "*/ ") + 3
			l = l[:i] + prefix + l[i:]
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n") + "\n"
}
