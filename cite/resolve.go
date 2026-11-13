// SPDX-License-Identifier: Apache-2.0

package cite

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// around is how much text an equation, a theorem or a footnote holds on each side of its mark, in bytes, within its
// page. A displayed equation is read with the sentences that introduce and explain it.
const around = 700

// Resolve turns each place of a locator into a span of the text. It returns the spans of the places that it found
// and a message for each place that it did not.
func Resolve(d *Doc, refs []Ref) (spans []Span, fails []string) {
	for _, r := range refs {
		s, err := d.resolve(r)
		if err != "" {
			fails = append(fails, r.String()+": "+err)
			continue
		}
		low := Norm(d.Text[s.Start:s.End])
		for _, q := range []struct{ what, v string }{{"the heading", r.Title}, {"the row", r.Row}, {"the column", r.Column}} {
			if q.v != "" && !strings.Contains(low, Norm(q.v)) {
				fails = append(fails, fmt.Sprintf("%s: %s %q is not in it", r.String(), q.what, q.v))
			}
		}
		spans = append(spans, s)
	}
	return spans, fails
}

func (d *Doc) resolve(r Ref) (Span, string) {
	switch r.Kind {
	case "section", "chapter", "appendix":
		return d.heads(r)
	case "table", "figure", "algorithm":
		return d.captions(r)
	case "equation", "footnote":
		return d.marks(r)
	case "page":
		return d.pages(r)
	case "abstract":
		if d.Abstract != nil {
			return *d.Abstract, ""
		}
		if len(d.Heads) > 0 {
			return Span{0, d.Heads[0].Start}, ""
		}
		return Span{}, "the text has no abstract"
	case "title-page":
		return d.PageSpan(0), ""
	case "theorem", "lemma", "definition", "proposition", "corollary":
		return d.labelled(r)
	case "article", "recital", "annex":
		return d.provision(r)
	}
	return Span{}, "the kind " + r.Kind + " is not known"
}

func (d *Doc) head(num string) (Head, bool) {
	for _, h := range d.Heads {
		if h.Num == num {
			return h, true
		}
	}
	return Head{}, false
}

func (d *Doc) heads(r Ref) (Span, string) {
	from, ok := d.head(r.From)
	if !ok {
		return Span{}, fmt.Sprintf("the text has no heading %s (it has %s)", r.From, d.headList())
	}
	s := Span{from.Start, from.End}
	if r.To != "" {
		to, ok := d.head(r.To)
		if !ok || to.Start < from.Start {
			return Span{}, fmt.Sprintf("the text has no heading %s after %s", r.To, r.From)
		}
		s.End = to.End
	}
	return s, ""
}

func (d *Doc) headList() string {
	if len(d.Heads) == 0 {
		return "no numbered headings"
	}
	var ns []string
	for _, h := range d.Heads {
		if strings.Count(h.Num, ".") == 0 {
			ns = append(ns, h.Num)
		}
	}
	return "the headings " + strings.Join(ns, ", ") + " at the top level"
}

// numbers gives the numbers of a range: "2" to "9" is 2, 3, …, 9. A range of numbers with a dot or a letter is its
// two ends.
func numbers(r Ref) []string {
	if r.To == "" {
		return []string{r.From}
	}
	a, err1 := strconv.Atoi(r.From)
	b, err2 := strconv.Atoi(r.To)
	if err1 != nil || err2 != nil || b < a || b-a > 50 {
		return []string{r.From, r.To}
	}
	var out []string
	for i := a; i <= b; i++ {
		out = append(out, strconv.Itoa(i))
	}
	return out
}

// captions resolves a table, a figure or an algorithm to the pages that hold its caption, because the extraction
// does not keep the cells of a table next to its caption.
func (d *Doc) captions(r Ref) (Span, string) {
	var s Span
	found := false
	for _, num := range numbers(r) {
		ok := false
		for _, m := range d.Marks {
			if m.Kind == r.Kind && m.Num == num {
				p := d.PageSpan(d.Page(m.Start))
				s = union(s, p, found)
				found, ok = true, true
			}
		}
		if !ok {
			return Span{}, fmt.Sprintf("the text has no caption %s %s", r.Kind, num)
		}
	}
	return s, ""
}

func (d *Doc) marks(r Ref) (Span, string) {
	var s Span
	found := false
	for _, num := range numbers(r) {
		ok := false
		for _, m := range d.Marks {
			if m.Kind == r.Kind && m.Num == num {
				s = union(s, d.near(m.Start), found)
				found, ok = true, true
				break
			}
		}
		if !ok {
			return Span{}, fmt.Sprintf("the text has no %s %s", r.Kind, num)
		}
	}
	return s, ""
}

// near is the text around an offset, within its page.
func (d *Doc) near(off int) Span {
	p := d.PageSpan(d.Page(off))
	return Span{max(p.Start, off-around), min(p.End, off+around)}
}

func (d *Doc) pages(r Ref) (Span, string) {
	var s Span
	found := false
	for _, num := range numbers(r) {
		n, err := strconv.Atoi(num)
		if err != nil {
			return Span{}, "the page " + num + " is not a number"
		}
		i := d.printed(n)
		if i < 0 {
			return Span{}, fmt.Sprintf("the text has %d pages, and none is printed %d", len(d.Pages), n)
		}
		s = union(s, d.PageSpan(i), found)
		found = true
	}
	return s, ""
}

// printed finds a page by the number printed on it, in its first or last line, or else by its place in the file.
// A journal article that starts at page 1929 is cited by its printed numbers.
func (d *Doc) printed(n int) int {
	want := strconv.Itoa(n)
	for i := range d.Pages {
		lines := strings.Split(strings.TrimSpace(d.Text[d.PageSpan(i).Start:d.PageSpan(i).End]), "\n")
		if len(lines) == 0 {
			continue
		}
		first, last := strings.TrimSpace(lines[0]), strings.TrimSpace(lines[len(lines)-1])
		if first == want || last == want || strings.HasSuffix(first, " "+want) || strings.HasPrefix(first, want+" ") {
			return i
		}
	}
	if n >= 1 && n <= len(d.Pages) {
		return n - 1
	}
	return -1
}

func (d *Doc) labelled(r Ref) (Span, string) {
	word := strings.ToUpper(r.Kind[:1]) + r.Kind[1:]
	re := regexp.MustCompile(`(?m)^\s*` + word + `\s+` + regexp.QuoteMeta(r.From) + `\b`)
	loc := re.FindStringIndex(d.Text)
	if loc == nil {
		return Span{}, fmt.Sprintf("the text has no %s %s", r.Kind, r.From)
	}
	s := d.near(loc[0])
	s.Start = loc[0]
	return s, ""
}

var paraRE = regexp.MustCompile(`^([0-9]+[a-z]?)((?:\([0-9a-z]+\))*)$`)

// provision resolves an article, a recital or an annex of a legal text, and the paragraph of an article.
func (d *Doc) provision(r Ref) (Span, string) {
	word := strings.ToUpper(r.Kind[:1]) + r.Kind[1:]
	num, paras := r.From, ""
	if m := paraRE.FindStringSubmatch(r.From); m != nil {
		num, paras = m[1], m[2]
	}
	start := regexp.MustCompile(`(?m)^\s*` + word + `\s+` + regexp.QuoteMeta(num) + `\b`)
	loc := start.FindStringIndex(d.Text)
	if loc == nil && r.Kind == "recital" {
		loc = regexp.MustCompile(`(?m)^\s*\(` + regexp.QuoteMeta(num) + `\)`).FindStringIndex(d.Text)
	}
	if loc == nil {
		return Span{}, fmt.Sprintf("the text has no %s %s", r.Kind, num)
	}
	s := Span{loc[0], len(d.Text)}
	next := regexp.MustCompile(`(?m)^\s*` + word + `\s+[0-9IVX]+[a-z]?\b`)
	if m := next.FindStringIndex(d.Text[loc[1]:]); m != nil {
		s.End = loc[1] + m[0]
	}
	if paras == "" {
		return s, ""
	}
	// The first paragraph number: "50(4)" is the paragraph that starts "4." or "(4)" in the article.
	p := strings.TrimPrefix(strings.SplitN(paras, ")", 2)[0], "(")
	body := d.Text[s.Start:s.End]
	pre := regexp.MustCompile(`(?m)^\s*(?:` + regexp.QuoteMeta(p) + `\.|\(` + regexp.QuoteMeta(p) + `\))\s`)
	m := pre.FindStringIndex(body)
	if m == nil {
		return Span{}, fmt.Sprintf("%s %s has no paragraph %s", r.Kind, num, p)
	}
	ps := Span{s.Start + m[0], s.End}
	n, err := strconv.Atoi(strings.TrimRight(p, "abcdefghijklmnopqrstuvwxyz"))
	if err == nil {
		nx := regexp.MustCompile(`(?m)^\s*(?:` + strconv.Itoa(n+1) + `\.|\(` + strconv.Itoa(n+1) + `\))\s`)
		if m := nx.FindStringIndex(d.Text[ps.Start+1 : s.End]); m != nil {
			ps.End = ps.Start + 1 + m[0]
		}
	}
	return ps, ""
}

func union(a, b Span, ok bool) Span {
	if !ok {
		return b
	}
	return Span{min(a.Start, b.Start), max(a.End, b.End)}
}

// Merge sorts spans and joins the ones that overlap.
func Merge(spans []Span) []Span {
	s := slices.Clone(spans)
	slices.SortFunc(s, func(a, b Span) int { return a.Start - b.Start })
	var out []Span
	for _, x := range s {
		if n := len(out); n > 0 && x.Start <= out[n-1].End {
			out[n-1].End = max(out[n-1].End, x.End)
			continue
		}
		out = append(out, x)
	}
	return out
}
