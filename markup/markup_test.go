package markup

import (
	"strings"
	"testing"
)

func resolve(slug string) (string, string, bool) {
	if slug == "merge-sort" {
		return "Merge sort", "/merge-sort/", true
	}
	return "", "", false
}

func TestWikiLinks(t *testing.T) {
	r, err := Render("See [[merge-sort]] and [[merge-sort|the merge]] and [[nope]].\n\n```\n[[merge-sort]]\n```\n", resolve)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.HTML, `<a href="/merge-sort/">Merge sort</a>`) {
		t.Errorf("no link to the node: %s", r.HTML)
	}
	if !strings.Contains(r.HTML, `<a href="/merge-sort/">the merge</a>`) {
		t.Errorf("no link with a label: %s", r.HTML)
	}
	if len(r.Missing) != 1 || r.Missing[0] != "nope" {
		t.Errorf("missing = %v, want [nope]", r.Missing)
	}
	if !strings.Contains(r.HTML, "[[merge-sort]]") {
		t.Errorf("a link inside code was changed: %s", r.HTML)
	}
}

func TestMath(t *testing.T) {
	r, err := Render("The cost is $O(n \\log n)$, and it costs $5 and $6.\n", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.HTML, `<span class="math">\(O(n \log n)\)</span>`) {
		t.Errorf("no inline math: %s", r.HTML)
	}
	if strings.Count(r.HTML, `class="math"`) != 1 {
		t.Errorf("prices became math: %s", r.HTML)
	}
}

func TestPseudo(t *testing.T) {
	src := "```pseudo id=main\nFIND(A, x)\nL1  lo ← 0\nL2  while lo < hi do    ▷ a comment\nL3      return BINARY-SEARCH(A)\n```\n"
	r, err := Render(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(r.LineIDs, ",") != "L1,L2,L3" {
		t.Errorf("line ids = %v", r.LineIDs)
	}
	if !strings.Contains(r.HTML, `data-line="L2"`) || !strings.Contains(r.HTML, `<b>while</b>`) {
		t.Errorf("bad pseudocode: %s", r.HTML)
	}
	if !strings.Contains(r.HTML, `<span class="c">▷ a comment</span>`) {
		t.Errorf("no comment: %s", r.HTML)
	}
	if !strings.Contains(r.HTML, `<span class="sc">BINARY-SEARCH</span>`) || !strings.Contains(r.HTML, `class="head"`) {
		t.Errorf("no small caps or header: %s", r.HTML)
	}
}

func TestTOC(t *testing.T) {
	r, err := Render("## Idea\n\ntext\n\n### Detail\n\n## Complexity\n", nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(r.Sections, ",") != "Idea,Complexity" {
		t.Errorf("sections = %v", r.Sections)
	}
	if len(r.TOC) != 3 || r.TOC[0].ID != "idea" {
		t.Errorf("toc = %+v", r.TOC)
	}
}

func TestHighlight(t *testing.T) {
	h := Highlight("def f():\n    return 1\n", "python")
	if !strings.Contains(h, "chroma") {
		t.Errorf("no highlight: %s", h)
	}
	if !strings.Contains(HighlightCSS(), "[data-theme=dark]") {
		t.Error("no dark theme in the CSS")
	}
}

func TestPseudoIndent(t *testing.T) {
	src := "```pseudo id=main\nF(A)\nL1  for i ← 1 to n do\nL2      x ← i\nL10 return x\n```\n"
	r, err := Render(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`data-line="L1"><span class="lid" title="L1">1</span><code style="--indent:0">`, `title="L2">2</span><code style="--indent:1">`, `title="L10">3</span><code style="--indent:0">`} {
		if !strings.Contains(r.HTML, want) {
			t.Errorf("no %s in %s", want, r.HTML)
		}
	}
}

func TestDisplayMathDollars(t *testing.T) {
	cases := map[string]string{
		"one line, spaces":   "Then\n$$ a < b, \\qquad c $$\nand so on.",
		"one line, no space": "$$T(n) = 2T(n/2) + n.$$",
		"several lines":      "Text\n$$\nx = 1\n$$\n",
		"inline in a line":   "We have $$x^2$$ here.",
		"indented in a list": "- item\n\n  $$ y = 2 $$\n",
	}
	for name, src := range cases {
		res, err := Render(src, nil)
		if err != nil {
			t.Fatal(name, err)
		}
		if strings.Contains(res.HTML, "$$") || strings.Contains(res.HTML, "$<span") {
			t.Errorf("%s: raw $$ in %q", name, res.HTML)
		}
		if !strings.Contains(res.HTML, "math") {
			t.Errorf("%s: no math in %q", name, res.HTML)
		}
		if len(res.MathErrors) != 0 {
			t.Errorf("%s: errors %v", name, res.MathErrors)
		}
	}
	if !strings.Contains(mustRender(t, "Then\n$$ a $$\nand").HTML, `class="math display"`) {
		t.Error("a $$ line is not display math")
	}
	res, _ := Render("$$\nx = 1\n\nmore", nil)
	if len(res.MathErrors) != 1 {
		t.Errorf("unclosed $$ not reported: %v", res.MathErrors)
	}
}

func mustRender(t *testing.T, src string) Result {
	t.Helper()
	res, err := Render(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	return res
}
