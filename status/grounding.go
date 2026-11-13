// SPDX-License-Identifier: Apache-2.0

package status

import (
	"fmt"

	"github.com/tamnd/ai-agent/cite"
	cl "github.com/tamnd/ai-agent/claims"
	"github.com/tamnd/ai-agent/node"
)

var gNames = []string{"no sources", "sourced", "checked", "verified"}

// Grounding is the ladder G0 to G3 of spec 04.10 over the claims of claims.yaml:
//
//   - G1: every claim has a source that names a work card, with a locator in the grammar of 13.6 (G01 to G03);
//   - G2: the citation check found every claim in the stored snapshot of at least one of its sources (G05);
//   - G3: the verifier judged every claim supported by a grounded source, and none contradicted (15.4).
//
// A mathematical claim with a derivation or a Lean proof and no source is grounded by its proof, as rule G02 allows.
func Grounding(n *node.Node, env Env) (r Rung) {
	r = Rung{Max: 3}
	defer func() { r.Label, r.Name = fmt.Sprintf("G%d", r.Level), gNames[r.Level] }()
	if len(n.Claims) == 0 {
		r.Next = []string{"write the claims of the page in claims.yaml (07.6)"}
		return r
	}
	var g1, g2, g3 []string
	for i := range n.Claims {
		c := &n.Claims[i]
		if len(c.Evidence) == 0 {
			if c.Type != cl.Mathematical || (c.Derivation == "" && c.Lean == "") {
				g1 = append(g1, c.ID+" has no source (G02)")
			}
			continue
		}
		var located, grounded, supported, contradicted bool
		for _, e := range c.Evidence {
			if e.Work == "" || env.Corpus.CardByKey[e.Work] == nil {
				continue
			}
			if _, err := cite.ParseLocator(e.Locator); err != nil {
				continue
			}
			located = true
			rec, ok := env.Cite.Current(n.Tag, c, e)
			if !ok || !rec.Pass {
				continue
			}
			grounded = true
			switch rec.Verdict {
			case cite.Supported:
				supported = true
			case cite.Contradicted:
				contradicted = true
			}
		}
		switch {
		case !located:
			g1 = append(g1, c.ID+": no source names a work card with a locator in the grammar of 13.6 (G01, G03)")
		case !grounded:
			g2 = append(g2, c.ID+": the citation check has not found it in a source (aik cite, G05)")
		case contradicted:
			g3 = append(g3, c.ID+": the verifier found a source that contradicts it (15.4)")
		case !supported:
			g3 = append(g3, c.ID+": the verifier has not judged it supported (15.4)")
		}
	}
	switch {
	case len(g1) > 0:
		r.Next = g1
	case len(g2) > 0:
		r.Level, r.Next = 1, g2
	case len(g3) > 0:
		r.Level, r.Next = 2, g3
	default:
		r.Level = 3
	}
	return r
}
