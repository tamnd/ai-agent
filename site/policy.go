// SPDX-License-Identifier: Apache-2.0

package site

import (
	"slices"
	"strconv"
	"strings"

	"github.com/tamnd/ai-agent/card"
	"github.com/tamnd/ai-agent/facts"
	"github.com/tamnd/ai-agent/node"
)

// ProvisionRow is one provision card in the generated list of the main provisions of a policy (spec 06, policy 3).
type ProvisionRow struct {
	Name    string
	URL     string
	Tag     string
	Summary string
	// Applies are the dates from which the provision applies, from its applies facts, labelled with the part of the
	// provision or the class of systems that the date is for, such as "existing generative systems".
	Applies []CardFact
}

// LegalNotice is the notice of a policy page: the page is a summary, not legal advice (spec 06, policy).
type LegalNotice struct {
	Official string // the URL of the official text, from official_text; empty when the node does not give it
}

// legalNotice gives the notice of a policy page, or nil for a node of another kind.
func legalNotice(n *node.Node) *LegalNotice {
	if n.Kind != "policy" {
		return nil
	}
	u, _ := n.Fields["official_text"].(string)
	if !strings.HasPrefix(u, "https://") && !strings.HasPrefix(u, "http://") {
		u = ""
	}
	return &LegalNotice{Official: u}
}

// provisionRows gives the provision cards that the policy owns, in the order of their articles: Article 5 before
// Article 50.
func (b *builder) provisionRows(n *node.Node) []ProvisionRow {
	var cs []*card.Card
	for _, c := range b.cardKinds()["provision"] {
		if c.Owner == n.Slug {
			cs = append(cs, c)
		}
	}
	slices.SortStableFunc(cs, func(x, y *card.Card) int { return naturalCmp(x.Slug, y.Slug) })
	rows := make([]ProvisionRow, 0, len(cs))
	for _, c := range cs {
		r := ProvisionRow{Name: c.Name, URL: b.cardLink(c), Tag: c.Tag, Summary: c.Summary}
		for _, f := range c.Facts {
			if f.ID == "applies" || strings.HasPrefix(f.ID, "applies.") {
				part := strings.ReplaceAll(strings.TrimPrefix(strings.TrimPrefix(f.ID, "applies"), "."), "_", " ")
				r.Applies = append(r.Applies, CardFact{Label: part, Value: facts.Format(f.V, ""), Src: f.Src, Loc: f.Loc})
			}
		}
		// In the order of time: an ISO date orders as text.
		slices.SortStableFunc(r.Applies, func(x, y CardFact) int { return strings.Compare(x.Value, y.Value) })
		rows = append(rows, r)
	}
	return rows
}

// naturalCmp compares two slugs with their runs of digits as numbers, so that article-5 comes before article-50 and
// article-50 before article-113.
func naturalCmp(a, b string) int {
	for a != "" && b != "" {
		da, db := digits(a), digits(b)
		if da > 0 && db > 0 {
			x, _ := strconv.Atoi(a[:da])
			y, _ := strconv.Atoi(b[:db])
			if x != y {
				if x < y {
					return -1
				}
				return 1
			}
			a, b = a[da:], b[db:]
			continue
		}
		if a[0] != b[0] {
			if a[0] < b[0] {
				return -1
			}
			return 1
		}
		a, b = a[1:], b[1:]
	}
	return len(a) - len(b)
}

func digits(s string) int {
	i := 0
	for i < len(s) && i < 9 && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	return i
}
