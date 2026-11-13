// SPDX-License-Identifier: Apache-2.0

package cite

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Ref is one place of a locator: a section, an equation, a table, a page, an article of a law.
type Ref struct {
	Kind string `json:"kind"`
	// From and To are the numbers of the place, as "3.2", "A.4", "50(4)" or "12". To is empty for one place.
	From string `json:"from,omitempty"`
	To   string `json:"to,omitempty"`
	// Row and Column name a cell of a table. Title is a heading or a paragraph name that the place must hold, as
	// "Residual Dropout" in Section 5.4.
	Row    string `json:"row,omitempty"`
	Column string `json:"column,omitempty"`
	Title  string `json:"title,omitempty"`
}

// The kinds of places of spec 13.6, with the words that name them. A plural form takes a range.
var kinds = []struct {
	kind  string
	words []string
	num   *regexp.Regexp // the form of a number of the kind; nil for a place without one
}{
	{"section", []string{"section", "sections", "§"}, secNum},
	{"appendix", []string{"appendix", "appendices"}, appNum},
	{"chapter", []string{"chapter", "chapters"}, secNum},
	{"equation", []string{"eq.", "eqs.", "equation", "equations"}, eqNum},
	{"table", []string{"table", "tables"}, plainNum},
	{"figure", []string{"figure", "figures", "fig."}, plainNum},
	{"algorithm", []string{"algorithm", "algorithms"}, plainNum},
	{"theorem", []string{"theorem", "theorems"}, secNum},
	{"lemma", []string{"lemma", "lemmas"}, secNum},
	{"definition", []string{"definition", "definitions"}, secNum},
	{"proposition", []string{"proposition", "propositions"}, secNum},
	{"corollary", []string{"corollary"}, secNum},
	{"footnote", []string{"footnote", "footnotes"}, plainNum},
	{"page", []string{"p.", "pp.", "page", "pages"}, plainNum},
	{"article", []string{"article", "articles"}, artNum},
	{"recital", []string{"recital", "recitals"}, plainNum},
	{"annex", []string{"annex", "annexes"}, annexNum},
	{"abstract", []string{"abstract", "the abstract"}, nil},
	{"title-page", []string{"title page", "the title page"}, nil},
}

var (
	secNum   = regexp.MustCompile(`^[0-9]+(\.[0-9]+)*$`)
	appNum   = regexp.MustCompile(`^[A-Z](\.[0-9]+)*$`)
	eqNum    = regexp.MustCompile(`^[0-9]+(\.[0-9]+)?[a-z]?$|^[A-Z]\.[0-9]+$`)
	plainNum = regexp.MustCompile(`^[0-9]+$|^[A-Z]\.?[0-9]+$`)
	artNum   = regexp.MustCompile(`^[0-9]+[a-z]?(\([0-9]+[a-z]?\))*(\([a-z]\))?$`)
	annexNum = regexp.MustCompile(`^[IVX]+$|^[0-9]+$`)
	quoted   = regexp.MustCompile(`^"([^"]+)"$`)
	cellRE   = regexp.MustCompile(`^(row|column) "([^"]+)"$`)
	// titledRE is a place with the heading it must hold: Section 5.4 "Residual Dropout".
	titledRE = regexp.MustCompile(`^(.*[^\s"])\s+"([^"]+)"$`)
)

// ParseLocator reads a locator in the grammar of spec 13.6.1: places separated by commas, semicolons or "and", each
// a kind and a number ("Section 3.2", "Eq. 4", "Article 50(4)"), a range ("Sections 2 to 7"), a bare number that
// continues the kind before it ("Sections 3 and 4"), or a qualifier of the place before it: row "x", column "y", or a
// quoted heading. "the abstract" and "the title page" are places without a number.
func ParseLocator(s string) ([]Ref, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, errors.New("the locator is empty")
	}
	parts, err := split(s)
	if err != nil {
		return nil, err
	}
	var out []Ref
	for _, p := range parts {
		if m := cellRE.FindStringSubmatch(p); m != nil {
			if len(out) == 0 || out[len(out)-1].Kind != "table" {
				return nil, fmt.Errorf("%s names a cell, but the place before it is not a table", p)
			}
			r := &out[len(out)-1]
			if m[1] == "row" {
				r.Row = m[2]
			} else {
				r.Column = m[2]
			}
			continue
		}
		if m := quoted.FindStringSubmatch(p); m != nil {
			if len(out) == 0 {
				return nil, fmt.Errorf("the heading %s comes before any place", p)
			}
			out[len(out)-1].Title = m[1]
			continue
		}
		title := ""
		if m := titledRE.FindStringSubmatch(p); m != nil {
			p, title = m[1], m[2]
		}
		r, err := place(p, out)
		if err != nil {
			return nil, err
		}
		r.Title = title
		out = append(out, r)
	}
	return out, nil
}

// split cuts a locator at commas, semicolons and the word "and", outside quotes.
func split(s string) ([]string, error) {
	var parts []string
	var b strings.Builder
	in := false
	flush := func() error {
		p := strings.TrimSpace(b.String())
		b.Reset()
		if p == "" {
			return errors.New("the locator has an empty place")
		}
		parts = append(parts, p)
		return nil
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"':
			in = !in
			b.WriteByte(c)
		case !in && (c == ',' || c == ';'):
			if err := flush(); err != nil {
				return nil, err
			}
		case !in && strings.HasPrefix(s[i:], " and "):
			if err := flush(); err != nil {
				return nil, err
			}
			i += len(" and ") - 1
		default:
			b.WriteByte(c)
		}
	}
	if in {
		return nil, errors.New("the locator has an unclosed quote")
	}
	if err := flush(); err != nil {
		return nil, err
	}
	return parts, nil
}

func place(p string, before []Ref) (Ref, error) {
	low := strings.ToLower(p)
	for _, k := range kinds {
		for _, w := range k.words {
			if k.num == nil {
				if low == w {
					return Ref{Kind: k.kind}, nil
				}
				continue
			}
			if !strings.HasPrefix(low, w+" ") {
				continue
			}
			return numbered(k.kind, k.num, strings.TrimSpace(p[len(w):]))
		}
	}
	// A bare number continues the kind of the place before it: "Sections 3 and 4", "Tables 2, 3".
	if len(before) > 0 {
		prev := before[len(before)-1].Kind
		for _, k := range kinds {
			if k.kind == prev && k.num != nil && k.num.MatchString(strings.Fields(p)[0]) {
				return numbered(prev, k.num, p)
			}
		}
	}
	return Ref{}, fmt.Errorf("%q is not a place: write a kind and a number, as Section 3.2, Eq. 4, Table 3, Figure 2, p. 12 or Article 50(4) (13.6.1)", p)
}

func numbered(kind string, num *regexp.Regexp, rest string) (Ref, error) {
	from, to, _ := strings.Cut(rest, " to ")
	from, to = strings.TrimSpace(from), strings.TrimSpace(to)
	if kind == "appendix" {
		// "Appendix A." and "Appendix A" are the same place.
		from = strings.TrimSuffix(from, ".")
	}
	if !num.MatchString(from) {
		return Ref{}, fmt.Errorf("%q is not the number of a %s", from, kind)
	}
	if to != "" && !num.MatchString(to) {
		return Ref{}, fmt.Errorf("%q is not the number of a %s", to, kind)
	}
	return Ref{Kind: kind, From: from, To: to}, nil
}

// String writes the place as a locator.
func (r Ref) String() string {
	var b strings.Builder
	for _, k := range kinds {
		if k.kind == r.Kind {
			w := k.words[0]
			if k.num == nil {
				w = k.words[len(k.words)-1]
			}
			b.WriteString(strings.ToUpper(w[:1]) + w[1:])
			break
		}
	}
	if r.From != "" {
		b.WriteString(" " + r.From)
	}
	if r.To != "" {
		b.WriteString(" to " + r.To)
	}
	if r.Title != "" {
		b.WriteString(`, "` + r.Title + `"`)
	}
	if r.Row != "" {
		b.WriteString(`, row "` + r.Row + `"`)
	}
	if r.Column != "" {
		b.WriteString(`, column "` + r.Column + `"`)
	}
	return b.String()
}
