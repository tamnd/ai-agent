// SPDX-License-Identifier: Apache-2.0

package cite

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html"
	"math"
	"os/exec"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

// ExtractorVersion is the version of the code that turns the text of a snapshot into a Doc. A change of it changes
// the key of every check (spec 7.5), so every claim is checked again.
const ExtractorVersion = "aik-doc-4"

// Doc is the text of a snapshot with the places that a locator can name (spec 13.6). Offsets are in bytes of Text.
type Doc struct {
	Extractor string   `json:"extractor"`
	Text      string   `json:"text"`
	Pages     []int    `json:"pages"` // the start of each page
	Heads     []Head   `json:"heads"`
	Marks     []Mark   `json:"marks"`
	Abstract  *Span    `json:"abstract,omitempty"`
	Warnings  []string `json:"warnings,omitempty"`
}

// Head is a numbered heading: a section, a subsection or an appendix.
type Head struct {
	Num   string `json:"num"` // "3.2", "A", "A.4"
	Title string `json:"title"`
	Start int    `json:"start"`
	End   int    `json:"end"` // the start of the next heading at the same depth or above, or the end of the text
}

// Mark is a numbered place that is not a heading: a table or figure caption, an equation number, a footnote.
type Mark struct {
	Kind  string `json:"kind"`
	Num   string `json:"num"`
	Start int    `json:"start"`
}

// Span is a part of the text.
type Span struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// ErrNoExtractor means that the tool that reads PDFs is not installed.
var ErrNoExtractor = errors.New("pdftotext is not installed; install poppler (brew install poppler, apt install poppler-utils)")

// ExtractPDF reads the text of a PDF with pdftotext in reading order, and finds its places.
func ExtractPDF(ctx context.Context, pdf []byte) (*Doc, error) {
	tool, err := exec.LookPath("pdftotext")
	if err != nil {
		return nil, ErrNoExtractor
	}
	cmd := exec.CommandContext(ctx, tool, "-enc", "UTF-8", "-", "-")
	cmd.Stdin = bytes.NewReader(pdf)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("pdftotext: %w: %s", err, strings.TrimSpace(errb.String()))
	}
	ver := "pdftotext"
	if v, err := exec.CommandContext(ctx, tool, "-v").CombinedOutput(); err == nil {
		if f := strings.Fields(string(v)); len(f) >= 3 {
			ver += "-" + f[2]
		}
	}
	d := Parse(out.String())
	d.Extractor = ver + "+" + ExtractorVersion
	return d, nil
}

var (
	tagRE   = regexp.MustCompile(`(?is)<(script|style|nav|header|footer)\b.*?</(script|style|nav|header|footer)>|<[^>]+>`)
	hOpenRE = regexp.MustCompile(`(?i)<h[1-6]\b[^>]*>`)
	blockRE = regexp.MustCompile(`(?i)</(p|div|h[1-6]|li|tr|table|section|pre|blockquote)>|<br\s*/?>`)
)

// ExtractHTML reads the text of a web page, with one line for each heading and block. Its headings are those of its
// markup: a line of a table of contents, as "Bibliography" in the contents of an entry of the Stanford Encyclopedia,
// looks like a heading in the text, and a page has no later page to tell the contents from the body.
func ExtractHTML(page []byte) *Doc {
	d := Parse(htmlText(string(page)))
	d.Extractor = "html+" + ExtractorVersion
	var cands []cand
	var stops []int
	done, at := 0, 0 // the text before the byte done of the page is at bytes of the text
	for _, m := range hTagRE.FindAllStringSubmatchIndex(string(page), -1) {
		// The heading starts where the text of the page before it ends, give or take the blank lines. The conversion is
		// local, so the text of the page before it is the text before the last heading and the text in between.
		at += len(clean(htmlText(string(page[done:m[0]]))))
		done = m[0]
		title := strings.Join(strings.Fields(html.UnescapeString(tagRE.ReplaceAllString(string(page[m[4]:m[5]]), " "))), " ")
		if title == "" {
			continue
		}
		start := min(at, len(d.Text))
		if i := strings.Index(d.Text[start:], title); i >= 0 && i < 200 {
			start += i
		}
		h := headRE.FindStringSubmatch(title)
		if refsRE.MatchString(title) || (h == nil && page[m[2]] <= '2') {
			// The references end the body, and so does a heading of the top level with no number, as "Conclusion".
			stops = append(stops, start)
			continue
		}
		if h == nil || h[2] == "" {
			continue
		}
		if n := tuple(h[1]); n != nil {
			cands = append(cands, cand{Head{Num: h[1], Title: h[2], Start: start}, n, 3})
		}
	}
	if heads := chain(cands); len(heads) > 0 {
		d.Heads = heads
		d.ends(stops)
	}
	return d
}

var hTagRE = regexp.MustCompile(`(?is)<h([1-6])\b[^>]*>(.*?)</h[1-6]>`)

func htmlText(s string) string {
	s = hOpenRE.ReplaceAllString(s, "\n")
	s = blockRE.ReplaceAllString(s, "\n")
	s = tagRE.ReplaceAllString(s, " ")
	return html.UnescapeString(s)
}

var (
	// A heading line: a number, then a title that starts with a capital letter. "3.2 Attention", "6. Results", "A.4 Dropout Rate".
	headRE = regexp.MustCompile(`^((?:[0-9]+|[A-Z])(?:\.[0-9]+)*)\.?(?:\s+([A-Z][^\n]{0,100}))?$`)
	// "Appendix A. A Practical Guide", "Appendix B: Details".
	appRE = regexp.MustCompile(`^Appendix\s+([A-Z])\b[.:]?\s*([^\n]{0,100})$`)
	// "Table 1: Results" and "Figure 2. The model"; also "Table 1 Parameter counts", with no colon, where a word with
	// a capital follows the number. "Table 1 shows" is a mention, not a caption.
	captionRE = regexp.MustCompile(`^(Table|Figure|Fig\.|Algorithm)\s+([0-9]+|[A-Z]\.?[0-9]+)(?:\s*[:.|]|\s+[A-Z][a-z]+\s)`)
	// A caption that two-column text put in the middle of a line has a colon after its number: "error Figure 11:
	// Monte-Carlo". "Figure 11 shows" is a mention, not a caption.
	midCaptionRE = regexp.MustCompile(`\s(Table|Figure|Fig\.|Algorithm)\s+([0-9]+|[A-Z]\.?[0-9]+):\s`)
	eqRE         = regexp.MustCompile(`\(((?:[0-9]+|[A-Z]\.[0-9]+)(?:\.[0-9]+)?[a-z]?)\)\s*$`)
	footRE       = regexp.MustCompile(`^([0-9]{1,2})\s?[A-Za-z]`)
	bareRE       = regexp.MustCompile(`^[0-9]{1,2}$`)
	bareHeadRE   = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+)*\.?$`)
	urlRE        = regexp.MustCompile(`https?://|www\.`)
	listRE       = regexp.MustCompile(`^[0-9]+\.\s`)
	abstRE       = regexp.MustCompile(`(?i)^abstract\b`)
	refsRE       = regexp.MustCompile(`(?i)^(references( and notes)?|bibliography|literature cited)$`)
	// citedRE is a line of a list of references: initials first, or a year in brackets or at the end.
	citedRE = regexp.MustCompile(`^[A-Z]\.\s|\((1[89]|20)[0-9]{2}\)|\b(1[89]|20)[0-9]{2}\.$`)
)

// Parse finds the places of a text: pages at form feeds, numbered headings in order, captions, equation numbers and
// footnotes. It keeps a heading only when its number follows the one before (2 after 1.3, 3.2 after 3.1), so that a
// number at the start of a line of a table or a list is not taken for a heading.
func Parse(text string) *Doc {
	text = clean(text)
	d := &Doc{Text: text, Pages: []int{0}}
	for i := 0; i < len(text); i++ {
		if text[i] == '\f' {
			d.Pages = append(d.Pages, i+1)
		}
	}
	if n := len(d.Pages); n > 1 && strings.TrimSpace(text[d.Pages[n-1]:]) == "" {
		d.Pages = d.Pages[:n-1]
	}
	var lines []line
	page := 0
	for off := 0; off < len(text); {
		end := strings.IndexByte(text[off:], '\n')
		if end < 0 {
			end = len(text)
		} else {
			end += off
		}
		raw := text[off:end]
		page += strings.Count(raw, "\f")
		// A line that starts a page starts after its form feeds, which end the page before: a caption or a heading at
		// the top of a page is on that page.
		lead := len(raw) - len(strings.TrimLeft(raw, "\f"))
		lines = append(lines, line{off + lead, strings.TrimSpace(strings.Trim(raw, "\f")), lead > 0, page})
		off = end + 1
	}
	inApp, inRefs := false, false
	var cands []cand
	var stops []int // the starts of the lists of references, which end the heading before them
	for i, l := range lines {
		s := l.s
		if s == "" {
			continue
		}
		if m := captionRE.FindStringSubmatch(s); m != nil {
			k := strings.ToLower(strings.TrimSuffix(m[1], "."))
			if k == "fig" {
				k = "figure"
			}
			d.Marks = append(d.Marks, Mark{Kind: k, Num: strings.ReplaceAll(m[2], ".", ""), Start: l.off})
			continue
		}
		if m := midCaptionRE.FindStringSubmatchIndex(s); m != nil {
			k := strings.ToLower(strings.TrimSuffix(s[m[2]:m[3]], "."))
			if k == "fig" {
				k = "figure"
			}
			d.Marks = append(d.Marks, Mark{Kind: k, Num: strings.ReplaceAll(s[m[4]:m[5]], ".", ""), Start: l.off})
		}
		if m := eqRE.FindStringSubmatch(s); m != nil && (len(s) < 8 || mathy(s)) {
			d.Marks = append(d.Marks, Mark{Kind: "equation", Num: m[1], Start: l.off})
		}
		if d.Abstract == nil && abstRE.MatchString(s) && len(s) < 120 {
			d.Abstract = &Span{Start: l.off}
			// A heading comes after the abstract: a number before it on the first pages is in the address of an
			// author, as "1 Microsoft Way".
			if l.pno < 2 {
				cands = cands[:0]
			}
		}
		if refsRE.MatchString(s) {
			inRefs = true
			stops = append(stops, l.off)
			continue
		}
		title, num := "", ""
		at := i // the line of the title
		if m := appRE.FindStringSubmatch(s); m != nil {
			num, title, inApp, inRefs = m[1], strings.TrimSpace(m[2]), true, false
		} else if m := headRE.FindStringSubmatch(s); m != nil && !inRefs {
			num, title = m[1], m[2]
			if title == "" {
				// The number is alone on its line and the title is on the next line that is not empty.
				// A cell of a table, a formula or a web address can come between them, so up to three lines that are
				// not words are skipped; more is a table, whose cells are not the title.
				skipped := 0
				for j := i + 1; j < len(lines) && j <= i+8 && skipped <= 3; j++ {
					t := lines[j].s
					if t == "" {
						continue
					}
					if lines[j].page || captionRE.MatchString(t) {
						break // a page number above a caption or the first line of the next page is not a heading
					}
					if bareHeadRE.MatchString(t) {
						break // another number alone, as the 2.2 after a footnote mark 4: the title is its own
					}
					if !wordy(t) || len([]rune(t)) < 3 {
						skipped++
						continue // as the K above "Experiments"
					}
					if len(t) <= 100 && unicode.IsUpper(firstRune(t)) && !strings.HasSuffix(t, ".") {
						title, at = t, j
					}
					break
				}
			}
			// A bare letter is an appendix only after the word Appendix; "M. Chen" in the references is not one.
			if num[0] >= 'A' && num[0] <= 'Z' && (!inApp || !strings.Contains(num, ".")) {
				continue
			}
			// A number with a dot alone on its line, as 3.5 above a table, is a heading whose title the extraction
			// moved; a whole number alone is more often a page number, so it needs its title.
			if title == "" && !strings.Contains(num, ".") {
				continue
			}
			if listRE.MatchString(s) && (i > 0 && listRE.MatchString(lines[i-1].s) || i+1 < len(lines) && listRE.MatchString(lines[i+1].s)) {
				continue // an item of a numbered list, next to another: "1. Smoothing helps", "2. No single method"
			}
			if title != "" && sentence(title, lines, at) {
				continue // a footnote or a numbered sentence: "3 As a guide to the eye, we have underlined the"
			}
		} else {
			continue
		}
		title = joinCaps(title)
		n := tuple(num)
		if n == nil || !headTitle(num, title) {
			continue
		}
		// A numbered reference, "1 D. E. Rumelhart, ... (1986)", is not a heading.
		if title != "" && (len(strings.Fields(title)) > 14 || citedRE.MatchString(title)) {
			continue
		}
		// The first heading again, after a table of contents: the candidates so far were its lines.
		// The page may be the same, as when the contents end above the introduction, so it is the title that tells.
		if len(n) == 1 && n[0] == 1 && title != "" && len(cands) > 0 && slices.Equal(cands[0].n, n) && strings.EqualFold(cands[0].h.Title, title) {
			cands = nil
		}
		if bareHeadRE.MatchString(s) && axis(lines, i, at, num) {
			continue // a tick of the axis of a plot and its label: "3.5", "Human Score", then "3.5", "3", "2.5"
		}
		// A number with its title weighs more than a number alone, which may be a cell of a table, and a title that
		// goes on in lower case, which may be a sentence, weighs less than one that does not.
		w := 3
		switch {
		case title == "":
			w = 1
		case runOn(lines, at):
			w = 2
		}
		cands = append(cands, cand{Head{Num: num, Title: title, Start: l.off}, n, w})
	}
	d.Heads = chain(cands)
	d.ends(stops)
	d.footnotes(lines)
	return d
}

// ends sets the end of each heading, at the next heading at its depth or above or at a list of references, and the
// end of the abstract, at the first heading after it.
func (d *Doc) ends(stops []int) {
	for i := range d.Heads {
		h := &d.Heads[i]
		h.End = len(d.Text)
		depth := strings.Count(h.Num, ".")
		for _, o := range d.Heads[i+1:] {
			if strings.Count(o.Num, ".") <= depth {
				h.End = o.Start
				break
			}
		}
		for _, s := range stops {
			if s > h.Start && s < h.End {
				h.End = s
				break
			}
		}
	}
	if d.Abstract != nil {
		d.Abstract.End = len(d.Text)
		for _, h := range d.Heads {
			if h.Start > d.Abstract.Start {
				d.Abstract.End = h.Start
				break
			}
		}
	}
}

// cand is a line that can be a heading, with its number as a tuple and its weight.
type cand struct {
	h Head
	n []int
	w int
}

// chain keeps the candidates that make the heaviest run in which each number follows the one before. A footnote or a
// list item that looks like a heading ("3 As a guide to the eye") is left out when keeping it would cut the run of the
// headings after it, which a greedy reading, keeping each number that follows the last one kept, does not see.
func chain(cs []cand) []Head {
	best := make([]int, len(cs))
	prev := make([]int, len(cs))
	top := -1
	for i, c := range cs {
		best[i], prev[i] = -1, -1
		if follows(nil, c.n) {
			best[i] = c.w
		}
		for j := range i {
			if best[j] >= 0 && best[j]+c.w > best[i] && follows(cs[j].n, c.n) {
				best[i], prev[i] = best[j]+c.w, j
			}
		}
		if best[i] >= 0 && (top < 0 || best[i] > best[top]) {
			top = i
		}
	}
	var out []Head
	for i := top; i >= 0; i = prev[i] {
		out = append(out, cs[i].h)
	}
	slices.Reverse(out)
	return out
}

// sentence tells if the title of a heading candidate, on the line at, starts a sentence, as a footnote or a numbered
// item does: it holds a web address, it ends with a comma, or it is long and the next line goes on in lower case with
// no blank line between them. A title of up to seven words can wrap: "5.1 Which metric is best suited for pairwise",
// "comparison?"; a footnote fills the width of its column, eight words or more. An item "3. Publish your system
// outputs" that goes on in lower case is a sentence from four words.
func sentence(title string, lines []line, at int) bool {
	if urlRE.MatchString(title) || strings.HasSuffix(title, ",") || strings.HasSuffix(title, ";") {
		return true
	}
	long := 8
	if listRE.MatchString(lines[at].s) {
		long = 4
	}
	return len(strings.Fields(title)) >= long && runOn(lines, at)
}

// runOn tells if the line after the line at goes on in lower case, with no blank line between them.
func runOn(lines []line, at int) bool {
	return at+1 < len(lines) && lines[at+1].s != "" && !lines[at+1].page && unicode.IsLower(firstRune(lines[at+1].s))
}

// axis tells if a number alone on the line i, with its title on the line at, is a tick of the axis of a plot: the
// numbers alone on the four lines after the title that are not empty go on from it in equal steps, or repeat it, as
// "3.5", "Human Score", "3.5", "3", "2.5", or the line before it is a tick at most 1 from it with a fraction. A
// heading above a plot, "4.1", its title, then the ticks "21.0", "20.5", is not one: its number is not in their steps.
func axis(lines []line, i, at int, num string) bool {
	x, err := strconv.ParseFloat(num, 64)
	if err != nil {
		return false
	}
	// The tick before it, as "4.5" above "4", "Human Score".
	for j := i - 1; j >= 0 && j >= i-3; j-- {
		if lines[j].s == "" {
			continue
		}
		if v, err := strconv.ParseFloat(lines[j].s, 64); err == nil && v != x && math.Abs(v-x) <= 1 && (v != math.Trunc(v) || x != math.Trunc(x)) {
			return true
		}
		break
	}
	var ticks []float64
	seen := 0
	for j := at + 1; j < len(lines) && seen < 4; j++ {
		if lines[j].s == "" {
			continue
		}
		seen++
		if v, err := strconv.ParseFloat(lines[j].s, 64); err == nil {
			ticks = append(ticks, v)
		}
	}
	if len(ticks) > 0 && ticks[0] == x {
		return len(ticks) >= 2
	}
	return len(ticks) >= 2 && ticks[0]-ticks[1] != 0 && math.Abs((x-ticks[0])-(ticks[0]-ticks[1])) < 1e-9
}

type line struct {
	off  int
	s    string
	page bool // the line starts a page
	pno  int  // the index of the page of the line
}

// footnotes finds the lines that start with a small number in the last third of a page.
func (d *Doc) footnotes(lines []line) {
	for _, l := range lines {
		p := d.Page(l.off)
		start, end := d.Pages[p], len(d.Text)
		if p+1 < len(d.Pages) {
			end = d.Pages[p+1]
		}
		if l.off < start+(end-start)*2/3 {
			continue
		}
		if m := footRE.FindStringSubmatch(l.s); m != nil && !headRE.MatchString(l.s) {
			d.Marks = append(d.Marks, Mark{Kind: "footnote", Num: m[1], Start: l.off})
		}
	}
	// A footnote whose number is alone on its line, in the lower half of a page, with its text on the next line that
	// is not empty: a sentence of eight words or more ("4", then "To illustrate why the dot products get large").
	for i, l := range lines {
		if !bareRE.MatchString(l.s) || l.s == "0" {
			continue
		}
		span := d.PageSpan(d.Page(l.off))
		if l.off < span.Start+(span.End-span.Start)/2 {
			continue
		}
		for j := i + 1; j < len(lines) && j <= i+2; j++ {
			t := lines[j].s
			if t == "" {
				continue
			}
			if !lines[j].page && unicode.IsUpper(firstRune(t)) && len(strings.Fields(t)) >= 8 {
				d.Marks = append(d.Marks, Mark{Kind: "footnote", Num: l.s, Start: l.off})
			}
			break
		}
	}
}

// Page gives the index of the page that holds the offset.
func (d *Doc) Page(off int) int {
	i, _ := slices.BinarySearch(d.Pages, off+1)
	return max(i-1, 0)
}

// PageSpan gives the text of the page with the index i, from 0.
func (d *Doc) PageSpan(i int) Span {
	end := len(d.Text)
	if i+1 < len(d.Pages) {
		end = d.Pages[i+1]
	}
	return Span{d.Pages[i], end}
}

// wordy tells if a line is mostly letters, as a title is, and not a cell of numbers or symbols.
func wordy(s string) bool {
	letters := 0
	for _, r := range s {
		if unicode.IsLetter(r) || r == ' ' {
			letters++
		}
	}
	return letters*10 >= len([]rune(s))*8 && wordRE.MatchString(s)
}

var (
	capsRE = regexp.MustCompile(`\b([A-Z]) ([A-Z]{2,})\b`)
	abbrRE = regexp.MustCompile(`\p{L}\. \p{L}`)
	plurRE = regexp.MustCompile(`\b([A-Z]{2,}) S\b`)
	vsRE   = regexp.MustCompile(`(?i)\bvs\.`)
)

// joinCaps mends the small capitals of a title, which the extraction splits after the first letter of each word:
// "R EGULARIZING RNN S WITH LSTM CELLS" is "REGULARIZING RNNS WITH LSTM CELLS".
func joinCaps(t string) string {
	if t == "" || strings.ToUpper(t) != t {
		return t
	}
	t = plurRE.ReplaceAllString(t, "${1}S")
	return capsRE.ReplaceAllString(t, "$1$2")
}

// headTitle tells if a number and a title can be a heading. A title that is math ("X = apx") or a row of a table
// ("Avg. Test RMSE", "CNNs*") is not, though "vs." is a word of headings ("Model Averaging vs. Weight Scaling"); a number with no title and a part 0 or a leading zero (1.0, 5.02) is a number
// of a table.
func headTitle(num, title string) bool {
	if title == "" {
		for _, p := range strings.Split(num, ".")[1:] {
			if p == "0" || strings.HasPrefix(p, "0") {
				return false
			}
		}
		return true
	}
	if !wordy(title) || strings.ContainsAny(title, "=*|") || abbrRE.MatchString(vsRE.ReplaceAllString(title, "vs")) {
		return false
	}
	for _, r := range title {
		if r > 0xFFFF {
			return false // mathematical letters, as 𝑋
		}
	}
	return true
}

func firstRune(s string) rune {
	for _, r := range s {
		return r
	}
	return 0
}

// mathy tells if a line looks like a displayed equation: it holds an operator or a symbol of math.
func mathy(s string) bool {
	return strings.ContainsAny(s, "=≤≥<>∑∏∫√·×−+") || strings.Count(s, " ") < 4
}

// tuple turns "3.2" into [3 2] and "A.4" into [1001 4]. Letters come after every number, so the appendices follow
// the sections.
func tuple(num string) []int {
	var out []int
	for i, p := range strings.Split(num, ".") {
		if i == 0 && len(p) == 1 && p[0] >= 'A' && p[0] <= 'Z' {
			out = append(out, 1000+int(p[0]-'A')+1)
			continue
		}
		n, err := strconv.Atoi(p)
		if err != nil || n > 99 {
			return nil
		}
		out = append(out, n)
	}
	return out
}

// follows tells if the heading number n can come after the heading number last: the next one at a depth (3.2 after
// 3.1, 4 after 3.2.1), a first child (3.1 after 3), or the first appendix. It allows a gap of two, because the text
// of a heading is sometimes lost in the extraction.
func follows(last, n []int) bool {
	if len(last) == 0 {
		return (n[0] == 1 || n[0] == 1001) && allOnes(n[1:])
	}
	if n[0] > 1000 && last[0] < 1000 {
		return n[0] == 1001 && allOnes(n[1:])
	}
	if len(n) > len(last) {
		// A child: last followed by 1 or 2 (a 1 may be lost).
		return slices.Equal(n[:len(last)], last) && len(n) == len(last)+1 && n[len(n)-1] >= 1 && n[len(n)-1] <= 2
	}
	// The next one at the depth of n, which is the depth of last or above it.
	k := len(n) - 1
	if !slices.Equal(n[:k], last[:k]) {
		return false
	}
	step := n[k] - last[k]
	return step >= 1 && step <= 3
}

func allOnes(xs []int) bool {
	for _, x := range xs {
		if x != 1 {
			return false
		}
	}
	return true
}

var (
	hyphenRE = regexp.MustCompile(`([a-z])-\n([a-z])`)
	ligature = strings.NewReplacer("ﬁ", "fi", "ﬂ", "fl", "ﬀ", "ff", "ﬃ", "ffi", "ﬄ", "ffl", " ", " ", "\r\n", "\n", "\r", "\n")
)

// clean joins words that a line break cut, and writes ligatures as letters.
func clean(s string) string {
	s = ligature.Replace(s)
	return hyphenRE.ReplaceAllString(s, "$1$2\n")
}
