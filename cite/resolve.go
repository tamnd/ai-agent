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
		if d.Title != nil {
			return *d.Title, ""
		}
		return d.PageSpan(0), ""
	case "references":
		return d.references()
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
				sp := d.near(m.Start)
				if m.End > m.Start {
					sp = Span{m.Start, m.End}
				}
				s = union(s, sp, found)
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

// printed finds a page by the number printed on it, or else by its place in the file. A journal article that starts
// at page 1929 is cited by its printed numbers.
func (d *Doc) printed(n int) int {
	nums := d.printedNumbers()
	for i, p := range nums {
		if p == n {
			return i
		}
	}
	if n >= 1 && n <= len(d.Pages) {
		return n - 1
	}
	return -1
}

var pageNumRE = regexp.MustCompile(`^[0-9]{1,5}$`)

// edgeLines is how many lines at the top and at the bottom of a page can hold its printed number: a scan puts the
// number under its running head or under the noise of the margin, and a magazine puts it above its footer.
const edgeLines = 4

// printedNumbers gives the number printed on each page, or 0. The numbers that a page shows alone on a line near its
// top or its bottom are candidates; the pages whose candidates step by one with the pages vote for the difference
// between the printed number and the place, and the difference with the most votes, at least two, gives the number of
// a page that shows it, or that has a neighbour that shows its own. A scan that prints the numbers of the odd pages
// only in the margin, or loses them in its noise, still finds 418 between 417 and 419. With no such difference, the
// number is the one on the first or the last line of the page, as a single number cannot be checked.
func (d *Doc) printedNumbers() []int {
	cands := make([][]int, len(d.Pages))
	nums := make([]int, len(d.Pages))
	votes := map[int]int{}
	for i := range d.Pages {
		span := d.PageSpan(i)
		var lines []string
		for _, l := range strings.Split(d.Text[span.Start:span.End], "\n") {
			if l = strings.TrimSpace(strings.Trim(l, "\f")); l != "" {
				lines = append(lines, l)
			}
		}
		if len(lines) == 0 {
			continue
		}
		first, last := lines[0], lines[len(lines)-1]
		// The old rule: the number alone on the first or the last line, or after or before a running head.
		for _, f := range []string{first, last, lastField(first), firstField(first)} {
			if pageNumRE.MatchString(f) {
				nums[i], _ = strconv.Atoi(f)
				break
			}
		}
		for k, l := range lines {
			if k >= edgeLines && k < len(lines)-edgeLines {
				continue
			}
			if pageNumRE.MatchString(l) {
				v, _ := strconv.Atoi(l)
				if v > 0 && !slices.Contains(cands[i], v) {
					cands[i] = append(cands[i], v)
					votes[v-i]++
				}
			}
		}
	}
	off, top := 0, 1
	for o, v := range votes {
		if v > top || (v == top && top > 1 && o < off) {
			off, top = o, v
		}
	}
	if top < 2 {
		return nums
	}
	shows := func(i int) bool { return i >= 0 && i < len(cands) && slices.Contains(cands[i], i+off) }
	held := make([]bool, len(nums))
	lo, hi := -1, -1
	for i := range nums {
		if shows(i) || shows(i-1) || shows(i+1) {
			nums[i], held[i] = i+off, true
			if lo < 0 {
				lo = i
			}
			hi = i
		}
	}
	// A page between two that the difference holds is numbered by it too, and a number that the difference gives
	// to a page is no other page's: the contents ends with a lone 50, and page 50 is the one that follows 49.
	for i := lo + 1; i < hi; i++ {
		nums[i], held[i] = i+off, true
	}
	for i := range nums {
		if !held[i] && nums[i]-off >= lo && nums[i]-off <= hi && lo >= 0 {
			nums[i] = 0
		}
	}
	return nums
}

func firstField(s string) string {
	if f := strings.Fields(s); len(f) > 0 {
		return f[0]
	}
	return ""
}

func lastField(s string) string {
	if f := strings.Fields(s); len(f) > 0 {
		return f[len(f)-1]
	}
	return ""
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

var paraRE = regexp.MustCompile(`^([0-9]+[a-z]*)((?:\([0-9a-z]+\))*)$`)

// provision resolves an article, a recital or an annex of a legal text to the text from its heading to the next
// heading of its level or above (Provision), and the paragraph of an article to the text from its number to the next
// one.
func (d *Doc) provision(r Ref) (Span, string) {
	num, paras := r.From, ""
	if m := paraRE.FindStringSubmatch(r.From); m != nil && r.Kind == "article" {
		num, paras = m[1], m[2]
	}
	var s Span
	found := false
	for _, p := range d.Provisions {
		if p.Kind == r.Kind && p.Num == num {
			s, found = Span{p.Start, p.End}, true
			break
		}
	}
	if !found {
		return Span{}, fmt.Sprintf("the text has no heading %s %s (%s)", r.Kind, num, d.provisionList(r.Kind))
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

// provisionList says which provisions of a kind the text has, for the message of a place that it does not have.
func (d *Doc) provisionList(kind string) string {
	var ns []string
	for _, p := range d.Provisions {
		if p.Kind == kind {
			ns = append(ns, p.Num)
		}
	}
	switch {
	case len(d.Provisions) == 0:
		return "it has no headings of a legal text"
	case len(ns) == 0:
		return "it has no " + kind
	case len(ns) > 12:
		return fmt.Sprintf("it has %d, %s to %s", len(ns), ns[0], ns[len(ns)-1])
	}
	return "it has " + strings.Join(ns, ", ")
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

// refsLineRE is the heading of a list of references on a line of its own.
var refsLineRE = regexp.MustCompile(`(?im)^(references( and notes)?|bibliography|literature cited)[ \t]*$`)

// references is the list of references: from the last heading that names it, as a table of contents names it first,
// to the next numbered heading after it (an appendix) or the end of the text.
func (d *Doc) references() (Span, string) {
	all := refsLineRE.FindAllStringIndex(d.Text, -1)
	if len(all) == 0 {
		return Span{}, "the text has no list of references"
	}
	s := Span{Start: all[len(all)-1][0], End: len(d.Text)}
	for _, h := range d.Heads {
		if h.Start > s.Start {
			s.End = h.Start
			break
		}
	}
	return s, ""
}
