// SPDX-License-Identifier: Apache-2.0

package status

import (
	"strings"
	"testing"

	"github.com/tamnd/ai-agent/card"
	"github.com/tamnd/ai-agent/cite"
	cl "github.com/tamnd/ai-agent/claims"
	"github.com/tamnd/ai-agent/node"
)

func TestGroundingLadder(t *testing.T) {
	snap := cite.Snapshot{Work: "w-2014", Hash: "h", Extractor: "x+" + cite.ExtractorVersion}
	c := &node.Corpus{CardByKey: map[string]*card.Card{"w-2014": {Slug: "w-2014"}}}
	claim := func(loc string) cl.Claim {
		return cl.Claim{ID: "c-a", Type: cl.Empirical, Text: "Dropout lowers the error.",
			Evidence: []cl.Evidence{{Work: "w-2014", Role: "original", Locator: loc, Stance: "for"}}}
	}
	rec := func(x cl.Claim, pass bool, verdict string) *cite.Index {
		e := x.Evidence[0]
		r := cite.Record{Claim: x.ID, Work: e.Work, Locator: e.Locator, Key: cite.Key(x.Text, snap.Hash, e.Locator, snap.Extractor), Pass: pass, Verdict: verdict}
		return &cite.Index{Snapshots: map[string]cite.Snapshot{"w-2014": snap},
			Records: map[string]map[string]cite.Record{"T": {cite.RecordID(r.Claim, r.Work, r.Locator): r}}}
	}
	good := claim("Section 6")
	stale := good
	stale.Text = "Dropout lowers the error a lot."
	proof := cl.Claim{ID: "c-m", Type: cl.Mathematical, Text: "x", Derivation: "derivations/x.md"}
	other := claim("Section 6")
	other.Evidence[0].Work = "no-card"
	for _, tc := range []struct {
		name   string
		claims []cl.Claim
		ix     *cite.Index
		want   int
		next   string
	}{
		{"no claims", nil, nil, 0, "claims.yaml"},
		{"no card", []cl.Claim{other}, nil, 0, "work card"},
		{"free-form locator", []cl.Claim{claim("NeurIPS 26, the sections on GLMs")}, nil, 0, "grammar"},
		{"not checked", []cl.Claim{good}, nil, 1, "aik cite"},
		{"check fails", []cl.Claim{good}, rec(good, false, ""), 1, "aik cite"},
		{"stale record", []cl.Claim{stale}, rec(good, true, cite.Supported), 1, "aik cite"},
		{"checked", []cl.Claim{good}, rec(good, true, ""), 2, "not judged"},
		{"contradicted", []cl.Claim{good}, rec(good, true, cite.Contradicted), 2, "contradicts"},
		{"verified", []cl.Claim{good}, rec(good, true, cite.Supported), 3, ""},
		{"a proof is its source", []cl.Claim{good, proof}, rec(good, true, cite.Supported), 3, ""},
	} {
		n := &node.Node{Tag: "T", Claims: tc.claims}
		r := Grounding(n, Env{Corpus: c, Cite: tc.ix})
		if r.Level != tc.want || !strings.Contains(strings.Join(r.Next, "; "), tc.next) {
			t.Errorf("%s: %s, next %q; want G%d with %q", tc.name, r.Label, r.Next, tc.want, tc.next)
		}
	}
}
