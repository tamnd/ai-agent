// SPDX-License-Identifier: Apache-2.0

package cite

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html"
	"os/exec"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

// ExtractorVersion is the version of the code that turns the text of a snapshot into a Doc. A change of it changes
// the key of every check (spec 7.5), so every claim is checked again.
const ExtractorVersion = "aik-doc-1"

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

// ExtractHTML reads the text of a web page, with one line for each heading and block.
func ExtractHTML(page []byte) *Doc {
	s := hOpenRE.ReplaceAllString(string(page), "\n")
	s = blockRE.ReplaceAllString(s, "\n")
	s = tagRE.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	d := Parse(s)
	d.Extractor = "html+" + ExtractorVersion
	return d
}

var (
	// A heading line: a number, then a title that starts with a capital letter. "3.2 Attention", "6. Results", "A.4 Dropout Rate".
	headRE = regexp.MustCompile(`^((?:[0-9]+|[A-Z])(?:\.[0-9]+)*)\.?(?:\s+([A-Z][^\n]{0,100}))?$`)
	// "Appendix A. A Practical Guide", "Appendix B: Details".
	appRE     = regexp.MustCompile(`^Appendix\s+([A-Z])\b[.:]?\s*([^\n]{0,100})$`)
	captionRE = regexp.MustCompile(`^(Table|Figure|Fig\.|Algorithm)\s+([0-9]+|[A-Z]\.?[0-9]+)\s*[:.|]`)
	// A caption that two-column text put in the middle of a line has a colon after its number: "error Figure 11:
	// Monte-Carlo". "Figure 11 shows" is a mention, not a caption.
	midCaptionRE = regexp.MustCompile(`\s(Table|Figure|Fig\.|Algorithm)\s+([0-9]+|[A-Z]\.?[0-9]+):\s`)
	eqRE         = regexp.MustCompile(`\(((?:[0-9]+|[A-Z]\.[0-9]+)(?:\.[0-9]+)?[a-z]?)\)\s*$`)
	footRE       = regexp.MustCompile(`^([0-9]{1,2})\s?[A-Za-z]`)
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
	var last []int // the number of the last heading, with letters as 1000+index
	var lines []line
	for off := 0; off < len(text); {
		end := strings.IndexByte(text[off:], '\n')
		if end < 0 {
			end = len(text)
		} else {
			end += off
		}
		raw := text[off:end]
		lines = append(lines, line{off, strings.TrimSpace(strings.Trim(raw, "\f")), strings.HasPrefix(raw, "\f")})
		off = end + 1
	}
	inApp, inRefs := false, false
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
		}
		if refsRE.MatchString(s) {
			inRefs = true
			stops = append(stops, l.off)
			continue
		}
		title, num := "", ""
		if m := appRE.FindStringSubmatch(s); m != nil {
			num, title, inApp, inRefs = m[1], strings.TrimSpace(m[2]), true, false
		} else if m := headRE.FindStringSubmatch(s); m != nil && !inRefs {
			num, title = m[1], m[2]
			if title == "" {
				// The number is alone on its line and the title is on the next line that is not empty.
				// A cell of a table can come between them, so a line that is not words is skipped.
				for j := i + 1; j < len(lines) && j <= i+5; j++ {
					t := lines[j].s
					if t == "" {
						continue
					}
					if lines[j].page || captionRE.MatchString(t) {
						break // a page number above a caption or the first line of the next page is not a heading
					}
					if !wordy(t) {
						continue
					}
					if len(t) <= 100 && unicode.IsUpper(firstRune(t)) && !strings.HasSuffix(t, ".") {
						title = t
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
		} else {
			continue
		}
		n := tuple(num)
		if n == nil || !follows(last, n) {
			continue
		}
		// A numbered reference, "1 D. E. Rumelhart, ... (1986)", is not a heading.
		if title != "" && (len(strings.Fields(title)) > 14 || citedRE.MatchString(title)) {
			continue
		}
		last = n
		d.Heads = append(d.Heads, Head{Num: num, Title: title, Start: l.off})
	}
	for i := range d.Heads {
		h := &d.Heads[i]
		h.End = len(text)
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
		d.Abstract.End = len(text)
		for _, h := range d.Heads {
			if h.Start > d.Abstract.Start {
				d.Abstract.End = h.Start
				break
			}
		}
	}
	d.footnotes(lines)
	return d
}

type line struct {
	off  int
	s    string
	page bool // the line starts a page
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
		return slices.Equal(n[:len(last)], last) && len(n) == len(last)+1 && n[len(n)-1] <= 2
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
