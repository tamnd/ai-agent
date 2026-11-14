// SPDX-License-Identifier: Apache-2.0

package cite

import (
	"regexp"
	"strconv"
	"strings"
)

// Provision is a place of a legal text (spec 13.6.1): a recital, an article or an annex, with the text up to the next
// heading of its level or above. A division (a part, a title, a chapter or a section of the enacting terms), the
// enacting formula ("HAVE ADOPTED THIS REGULATION:") and the final formula ("Done at…" and the signatures) are kept as
// well, because they end the provision before them, but a locator does not name them. The binding formula ("This
// Regulation shall be binding in its entirety…") stays with the last article, where readers cite it.
type Provision struct {
	Kind  string `json:"kind"` // recital, article, annex, division, enacting, final
	Num   string `json:"num,omitempty"`
	Start int    `json:"start"`
	End   int    `json:"end"`
}

var (
	// A heading of an article is "Article N" on a line of its own; the title, if any, is on the next line. "Article
	// 6(1) and the corresponding obligations" and "Article 4 of Regulation (EU) 2024/1689" are sentences.
	artHeadRE = regexp.MustCompile(`^Article\s+([0-9]{1,4})([a-z]{0,2})$`)
	// "ANNEX III", or "Annex III", on a line of its own.
	annexHeadRE = regexp.MustCompile(`^(?:ANNEX|Annex)\s+([IVXLC]+|[0-9]{1,3})$`)
	// "CHAPTER III", "SECTION 2", "TITLE IV", "PART 1", in capitals on a line of their own.
	divisionRE = regexp.MustCompile(`^(?:PART|TITLE|CHAPTER|SECTION)\s+(?:[IVXLC]+|[0-9]{1,3})$`)
	// A recital: "(12)" alone on its line, as in the Official Journal on the web, or at the start of its text, as in
	// its PDF.
	recitalRE = regexp.MustCompile(`^\(([0-9]{1,3})\)(?:\s|$)`)
	whereasRE = regexp.MustCompile(`^Whereas\s*:$`)
	enactRE   = regexp.MustCompile(`^HA(?:VE|S) ADOPTED TH(?:IS|ESE)\b`)
	finalRE   = regexp.MustCompile(`^Done at\s`)
	bindingRE = regexp.MustCompile(`^This (?:Regulation|Directive|Decision) (?:shall be binding|is addressed)`)
)

// divisionReach is how many lines that are not empty may come between a division and the first article under it: its
// title, a section with its title, and the running head and the page number of a PDF.
const divisionReach = 8

// legal finds the provisions of a legal text. The headings of the articles are the heaviest run of "Article N" lines
// in which each number follows the one before (2 after 1, 4a after 4), starting from Article 1 when the text has one,
// so that an article that an amending act inserts ("Article 75b", inside the quotation of Article 1 of the act) is
// not a heading of the act. Recitals are the numbers in brackets between "Whereas:" and the enacting formula, in
// order; annexes are the "ANNEX" lines after the last article.
func (d *Doc) legal(lines []line) {
	type art struct {
		i      int // the line
		base   int
		suffix string
	}
	var arts []art
	for i, l := range lines {
		if m := artHeadRE.FindStringSubmatch(l.s); m != nil {
			n, _ := strconv.Atoi(m[1])
			arts = append(arts, art{i, n, m[2]})
		}
	}
	if len(arts) == 0 {
		return
	}
	hasOne := false
	for _, a := range arts {
		hasOne = hasOne || (a.base == 1 && a.suffix == "")
	}
	follows := func(a, b art) bool {
		if b.base == a.base {
			return nextSuffix(a.suffix, b.suffix)
		}
		return b.base > a.base && b.base-a.base <= 3 && (b.suffix == "" || b.suffix == "a")
	}
	best, prev := make([]int, len(arts)), make([]int, len(arts))
	top := -1
	for i, a := range arts {
		best[i], prev[i] = -1, -1
		if !hasOne || (a.base == 1 && a.suffix == "") {
			best[i] = 1
		}
		for j := range i {
			if best[j] >= 0 && best[j]+1 > best[i] && follows(arts[j], a) {
				best[i], prev[i] = best[j]+1, j
			}
		}
		if best[i] >= 0 && (top < 0 || best[i] > best[top]) {
			top = i
		}
	}
	var chain []art
	for i := top; i >= 0; i = prev[i] {
		chain = append([]art{arts[i]}, chain...)
	}
	first, last := chain[0].i, chain[len(chain)-1].i
	enact, whereas := -1, -1
	for i := 0; i < first; i++ {
		switch s := lines[i].s; {
		case whereas < 0 && whereasRE.MatchString(s):
			whereas = i
		case enact < 0 && enactRE.MatchString(s):
			enact = i
		}
	}
	if len(chain) < 2 && enact < 0 {
		return // one "Article N" line is not a legal text
	}
	var ps []Provision
	add := func(kind, num string, i int) { ps = append(ps, Provision{Kind: kind, Num: num, Start: lines[i].off}) }

	// The recitals, from (1), each the next number, between "Whereas:" and the enacting terms.
	end := first
	if enact >= 0 {
		end = enact
	}
	type rec struct{ i, n int }
	var recs []rec
	for i := whereas + 1; i < end; i++ {
		if m := recitalRE.FindStringSubmatch(lines[i].s); m != nil {
			n, _ := strconv.Atoi(m[1])
			recs = append(recs, rec{i, n})
		}
	}
	rb, rp := make([]int, len(recs)), make([]int, len(recs))
	rtop := -1
	for i, r := range recs {
		rb[i], rp[i] = -1, -1
		if r.n == 1 {
			rb[i] = 1
		}
		for j := range i {
			if rb[j] >= 0 && rb[j]+1 > rb[i] && r.n > recs[j].n && r.n-recs[j].n <= 2 {
				rb[i], rp[i] = rb[j]+1, j
			}
		}
		if rb[i] >= 0 && (rtop < 0 || rb[i] > rb[rtop]) {
			rtop = i
		}
	}
	var rchain []rec
	for i := rtop; i >= 0; i = rp[i] {
		rchain = append([]rec{recs[i]}, rchain...)
	}
	for _, r := range rchain {
		add("recital", strconv.Itoa(r.n), r.i)
	}
	if enact >= 0 {
		add("enacting", "", enact)
	}

	// The articles, with the divisions just above an article of the run.
	at := 0 // the next article of the chain
	for i := first - divisionReach*2; i <= last; i++ {
		if i < 0 || i <= enact {
			continue
		}
		for at < len(chain) && chain[at].i < i {
			at++
		}
		if at < len(chain) && chain[at].i == i {
			a := chain[at]
			add("article", strconv.Itoa(a.base)+a.suffix, i)
			continue
		}
		if at < len(chain) && divisionRE.MatchString(lines[i].s) {
			n := 0
			for j := i + 1; j < chain[at].i; j++ {
				if lines[j].s != "" {
					n++
				}
			}
			if n <= divisionReach {
				add("division", "", i)
			}
		}
	}

	// The final formula after the last article, and the annexes after it. The final formula starts at "Done at". In a
	// text with no such line, as a consolidated text or a page of the Official Journal on the web, the last article
	// keeps its binding formula and what follows it (the notes) is the final formula, unless an annex follows.
	after, binding := last+1, -1
	for i := last + 1; i < len(lines); i++ {
		if binding < 0 && bindingRE.MatchString(lines[i].s) {
			binding = i
		}
		if finalRE.MatchString(lines[i].s) {
			add("final", "", i)
			after, binding = i+1, -1
			break
		}
	}
	if binding >= 0 {
		after = binding + 1
		for j := binding + 1; j < len(lines); j++ {
			if lines[j].s == "" {
				continue
			}
			if !annexHeadRE.MatchString(lines[j].s) {
				add("final", "", j)
				after = j + 1
			}
			break
		}
	}
	for i := after; i < len(lines); i++ {
		if m := annexHeadRE.FindStringSubmatch(lines[i].s); m != nil {
			add("annex", m[1], i)
		}
	}

	// The end of each provision: the next heading of its level or above. An annex ends at the next annex only, as its
	// parts and sections are its own.
	for i := range ps {
		p := &ps[i]
		p.End = len(d.Text)
		for _, o := range ps[i+1:] {
			if p.Kind != "annex" || o.Kind == "annex" {
				p.End = o.Start
				break
			}
		}
	}
	d.Provisions = ps
}

// nextSuffix tells if the letter b of an article number follows a: 4a after 4, 4b after 4a, or one letter later, as a
// letter may be lost.
func nextSuffix(a, b string) bool {
	if len(a) > 1 || len(b) != 1 {
		return false
	}
	if a == "" {
		return b == "a" || b == "b"
	}
	return b[0] > a[0] && b[0]-a[0] <= 2
}

// provisionAt is the recital, the article or the annex that holds the offset.
func (d *Doc) provisionAt(off int) *Provision {
	for i := range d.Provisions {
		p := &d.Provisions[i]
		if (p.Kind == "recital" || p.Kind == "article" || p.Kind == "annex") && p.Start <= off && off < p.End {
			return p
		}
	}
	return nil
}

// dropInsideProvisions removes the numbered headings that are inside a recital, an article or an annex: a numbered
// paragraph ("1. This Regulation applies to:") or a point of an annex is not a section.
func (d *Doc) dropInsideProvisions() {
	if len(d.Provisions) == 0 {
		return
	}
	out := d.Heads[:0]
	for _, h := range d.Heads {
		if d.provisionAt(h.Start) == nil {
			out = append(out, h)
		}
	}
	d.Heads = out
}

// provisionTitle names a provision as a locator does: "Article 4a", "Recital 12", "Annex III".
func provisionTitle(p Provision) string {
	return strings.ToUpper(p.Kind[:1]) + p.Kind[1:] + " " + p.Num
}
