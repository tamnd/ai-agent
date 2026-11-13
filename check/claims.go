// SPDX-License-Identifier: Apache-2.0

package check

import (
	"slices"
	"strings"
	"time"

	"github.com/tamnd/ai-agent/claims"
	"github.com/tamnd/ai-agent/facts"
	"github.com/tamnd/ai-agent/node"
)

// claims checks the claim records of a node and the claim anchors of its prose (spec 07.3 and 07.11).
func (r *run) claims(n *node.Node) {
	p := n.Rel + "/" + claims.File
	byID := map[string]*claims.Claim{}
	for i := range n.Claims {
		c := &n.Claims[i]
		switch {
		case c.ID == "":
			r.add("T01", Block, p, 0, "a claim has no id")
			continue
		case !claims.ValidID(c.ID):
			r.add("T01", Block, p, 0, "the claim id %q is not c- and a slug, such as c-reduces-overfitting", c.ID)
		case byID[c.ID] != nil:
			r.add("T01", Block, p, 0, "the claim %q is in the file more than once", c.ID)
		}
		byID[c.ID] = c
		r.claim(p, c)
	}
	for _, c := range n.Claims {
		if c.SupersededBy != "" && byID[c.SupersededBy] == nil {
			r.add("T01", Block, p, 0, "the claim %q is superseded by %q, which is not a claim of the node", c.ID, c.SupersededBy)
		}
	}
	r.anchors(n, byID)
}

func (r *run) claim(p string, c *claims.Claim) {
	for _, k := range c.Written {
		r.add("T05", Block, p, 0, "the claim %q has the field %q; the build computes it (07.3)", c.ID, k)
	}
	if !slices.Contains(claims.Types, c.Type) {
		r.add("T01", Block, p, 0, "the claim %q has the type %q; the types are %s", c.ID, c.Type, strings.Join(claims.Types, ", "))
	}
	if strings.TrimSpace(c.Text) == "" {
		r.add("T01", Block, p, 0, "the claim %q has no text", c.ID)
	}
	if c.Type == claims.Empirical && c.Scope.Empty() {
		r.add("T01", Block, p, 0, "the empirical claim %q has no scope; write models, data, scale and setting, and unknown for what the source does not say (07.8)", c.ID)
	}
	if c.Type == claims.Mathematical && len(c.Assumes) == 0 {
		r.add("T01", Block, p, 0, "the mathematical claim %q has no assumes (07.2)", c.ID)
	}
	if c.Type != claims.Empirical && !c.Scope.Empty() {
		r.add("T06", Block, p, 0, "the %s claim %q has a scope; only an empirical claim has one", c.Type, c.ID)
	}
	for _, d := range []struct{ name, v string }{{"as_of", c.AsOf}, {"review_after", c.ReviewAfter}} {
		if d.v != "" && !facts.IsDate(d.v) {
			r.add("T01", Block, p, 0, "the %s of the claim %q is %q; write a date such as 2026-11-13", d.name, c.ID, d.v)
		}
	}
	if len(c.Evidence) == 0 && (c.Type != claims.Mathematical || (c.Derivation == "" && c.Lean == "")) {
		r.add("G02", Level, p, 0, "the claim %q has no source", c.ID)
	}
	for i, e := range c.Evidence {
		r.evidence(p, c, i+1, e)
	}
	if c.Type == claims.Forecast && c.AsOf == "" {
		r.add("K07", Level, p, 0, "the forecast %q needs the date of the forecast in as_of", c.ID)
	}
	if c.ReviewAfter != "" && facts.IsDate(c.ReviewAfter) && c.ReviewAfter < time.Now().UTC().Format(time.DateOnly) {
		r.add("K13", Warn, p, 0, "the claim %q was due for review on %s", c.ID, c.ReviewAfter)
	}
}

func (r *run) evidence(p string, c *claims.Claim, i int, e claims.Evidence) {
	if e.Work == "" {
		r.add("G01", Block, p, 0, "the source %d of the claim %q names no work", i, c.ID)
	}
	switch {
	case e.Role == "":
		r.add("T01", Block, p, 0, "the source %d of the claim %q has no role", i, c.ID)
	case !slices.Contains(claims.Roles, e.Role):
		r.add("T01", Block, p, 0, "the source %d of the claim %q has the role %q; the roles are %s", i, c.ID, e.Role, strings.Join(claims.Roles, ", "))
	case slices.Contains(claims.Types, c.Type) && !claims.Allows(c.Type, e.Role):
		r.add("K03", Block, p, 0, "a %s claim cannot cite a source with the role %s (07.2); the claim is %q", c.Type, e.Role, c.ID)
	}
	if !slices.Contains(claims.Stances, e.Stance) {
		r.add("T01", Block, p, 0, "the source %d of the claim %q has the stance %q; write for, against or mixed", i, c.ID, e.Stance)
	}
	switch {
	case e.Reuses != "" && e.Role != "replication":
		r.add("T06", Block, p, 0, "the source %d of the claim %q has reuses, but only a replication has it", i, c.ID)
	case e.Reuses != "" && !slices.Contains(claims.Reuses, e.Reuses):
		r.add("T01", Block, p, 0, "the source %d of the claim %q has reuses %q; write code, data, weights or none", i, c.ID, e.Reuses)
	}
	if e.Locator == "" {
		r.add("G03", Level, p, 0, "the source %d of the claim %q has no locator", i, c.ID)
	}
}

// anchors checks the {#c-...} anchors and the ::claim references of the prose (rule K01).
func (r *run) anchors(n *node.Node, byID map[string]*claims.Claim) {
	p := n.Rel + "/node.md"
	anchors, refs, runs := claims.Scan(n.Body)
	seen := map[string]bool{}
	for _, a := range anchors {
		line := n.BodyLine + a.Line - 1
		c := byID[a.ID]
		switch {
		case c == nil:
			r.add("K01", Block, p, line, "the anchor {#%s} names no claim of %s", a.ID, claims.File)
		case seen[a.ID]:
			r.add("K01", Block, p, line, "the claim %s is anchored twice; a page anchors a claim once", a.ID)
		case c.SupersededBy != "":
			r.add("K09", Level, p, line, "the claim %s is superseded by %s, so it is not in the main text", a.ID, c.SupersededBy)
		}
		seen[a.ID] = true
	}
	for _, l := range runs {
		r.add("K01", Block, p, n.BodyLine+l-1, "a sentence holds more than two claims; split it (Q-27)")
	}
	for _, ref := range refs {
		line := n.BodyLine + ref.Line - 1
		o := r.c.BySlug[ref.Slug]
		if o == nil {
			r.add("K01", Block, p, line, "the claim reference names the node %q, which does not exist", ref.Slug)
			continue
		}
		if !slices.ContainsFunc(o.Claims, func(c claims.Claim) bool { return c.ID == ref.ID }) {
			r.add("K01", Block, p, line, "the node %s has no claim %s", ref.Slug, ref.ID)
		}
	}
}
