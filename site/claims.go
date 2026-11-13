// SPDX-License-Identifier: Apache-2.0

package site

import (
	"fmt"
	"html/template"
	"regexp"
	"strings"

	"github.com/tamnd/ai-agent/claims"
	"github.com/tamnd/ai-agent/node"
	"github.com/tamnd/ai-agent/status"
)

// ClaimView is one row of the Evidence table and the data of one claim chip (spec 07.12).
type ClaimView struct {
	ID             string
	Type           string
	Text           string
	Headline       string
	Chip           string // the text of the chip
	Class          string // the label class of the chip
	Labels         []string
	IndependentFor int
	Against        int
	Scope          string
	SupersededBy   string
	Sources        []ClaimSourceView
}

// ClaimSourceView is one source of a claim.
type ClaimSourceView struct {
	Work    string
	Role    string
	Locator string
	Stance  string
}

// computeClaims sets the computed fields of each claim of the corpus. The citation check (13.6) is not in aik yet, so no source is grounded, and only the experiments of the node give a label (measured here).
func computeClaims(c *node.Corpus) {
	for _, n := range c.Nodes {
		env := measuredEnv{m: status.Measured(n, status.Env{Corpus: c})}
		for i := range n.Claims {
			claims.Compute(&n.Claims[i], env)
		}
	}
}

// measuredEnv is the claims.Env of the build: it knows the experiments of the node, and has none of the other checks yet.
type measuredEnv struct {
	claims.NoChecks
	m map[string][2]bool
}

func (e measuredEnv) Measured(c *claims.Claim) (bool, bool) {
	r := e.m[c.ID]
	return r[0], r[1]
}

func claimView(c *claims.Claim) ClaimView {
	v := ClaimView{ID: c.ID, Type: c.Type, Text: c.Text, Headline: c.Headline, Labels: c.Labels, IndependentFor: c.IndependentFor, Against: c.Against, SupersededBy: c.SupersededBy}
	switch {
	case c.Headline != "":
		v.Chip, v.Class = c.Headline, "label-"+c.Headline
	case !claims.Labelled(c.Type):
		v.Chip, v.Class = "source", "label-source"
	default:
		v.Chip, v.Class = "not checked yet", "label-unchecked"
	}
	if s := c.Scope; !s.Empty() {
		var parts []string
		for _, p := range []struct{ k, v string }{{"models", s.Models}, {"data", strings.Join(s.Data, ", ")}, {"scale", s.Scale}, {"setting", s.Setting}} {
			if p.v != "" {
				parts = append(parts, p.k+": "+p.v)
			}
		}
		v.Scope = strings.Join(parts, "; ")
	}
	for _, e := range c.Evidence {
		v.Sources = append(v.Sources, ClaimSourceView{Work: e.Work, Role: e.Role, Locator: e.Locator, Stance: e.Stance})
	}
	return v
}

func claimViews(n *node.Node) []ClaimView {
	out := make([]ClaimView, 0, len(n.Claims))
	for i := range n.Claims {
		out = append(out, claimView(&n.Claims[i]))
	}
	return out
}

// claimShare is the line of "About this page" that gives the share of claims above reported (spec 04.10).
func claimShare(n *node.Node) string {
	total, above, checked := 0, 0, 0
	for _, c := range n.Claims {
		if !claims.Labelled(c.Type) || c.SupersededBy != "" {
			continue
		}
		total++
		if c.Headline != "" {
			checked++
		}
		if c.Headline != "" && c.Headline != claims.Reported {
			above++
		}
	}
	switch {
	case total == 0:
		return ""
	case checked == 0:
		return fmt.Sprintf("Claims: %d, not checked yet. The citation check gives the labels.", total)
	}
	return fmt.Sprintf("Claims: %d%% replicated or better, of %d.", above*100/total, total)
}

var codeBlockRE = regexp.MustCompile(`(?s)<pre[ >].*?</pre>|<code[ >].*?</code>`)

// outsideCode applies f to the parts of html that are not in a pre or a code element.
func outsideCode(html string, f func(string) string) string {
	var sb strings.Builder
	last := 0
	for _, m := range codeBlockRE.FindAllStringIndex(html, -1) {
		sb.WriteString(f(html[last:m[0]]))
		sb.WriteString(html[m[0]:m[1]])
		last = m[1]
	}
	sb.WriteString(f(html[last:]))
	return sb.String()
}

// claimMarks replaces the claim anchors of the rendered prose with chips, and the claim references with the claim they name.
func (b *builder) claimMarks(n *node.Node, html string) string {
	byID := map[string]*claims.Claim{}
	for i := range n.Claims {
		byID[n.Claims[i].ID] = &n.Claims[i]
	}
	return outsideCode(html, func(s string) string {
		s = claims.RefRe.ReplaceAllStringFunc(s, func(m string) string {
			sm := claims.RefRe.FindStringSubmatch(m)
			o := b.c.BySlug[sm[1]]
			if o == nil {
				return m
			}
			for i := range o.Claims {
				if o.Claims[i].ID == sm[2] {
					v := claimView(&o.Claims[i])
					href := "/" + o.Slug + "/#" + v.ID
					return fmt.Sprintf(`<p class="claim-ref">%s <a class="chip claim-chip %s" href="%s" title="A claim of %s">%s</a></p>`, template.HTMLEscapeString(v.Text), v.Class, href, template.HTMLEscapeString(o.Name), template.HTMLEscapeString(v.Chip))
				}
			}
			return m
		})
		return claims.AnchorRe.ReplaceAllStringFunc(s, func(m string) string {
			id := claims.AnchorRe.FindStringSubmatch(m)[1]
			c := byID[id]
			if c == nil {
				return m
			}
			v := claimView(c)
			return fmt.Sprintf(` <a class="chip claim-chip %s" id="%s" href="#ev-%s" title="%s claim; see the evidence">%s</a>`, v.Class, id, id, v.Type, template.HTMLEscapeString(v.Chip))
		})
	})
}
