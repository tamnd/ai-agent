// SPDX-License-Identifier: Apache-2.0

package quality

import (
	"math"
	"testing"
	"time"

	"github.com/tamnd/ai-agent/audit"
	"github.com/tamnd/ai-agent/cite"
	"github.com/tamnd/ai-agent/claims"
	"github.com/tamnd/ai-agent/node"
	"github.com/tamnd/ai-agent/status"
)

func ev(work, loc string) claims.Evidence {
	return claims.Evidence{Work: work, Role: "primary", Locator: loc, Stance: "supports"}
}

func TestCompute(t *testing.T) {
	a := &node.Node{Slug: "a", Tag: "A", Claims: []claims.Claim{
		{ID: "a1", Text: "one", Evidence: []claims.Evidence{ev("w1", "p. 1")}, Headline: claims.Proved},
		{ID: "a2", Text: "two", Headline: claims.Reported, ReviewAfter: "2026-01-01"},
	}}
	b := &node.Node{Slug: "b", Tag: "B", Claims: []claims.Claim{
		{ID: "b1", Text: "three", Evidence: []claims.Evidence{ev("w1", "p. 2")}},
		{ID: "b2", Text: "four", Evidence: []claims.Evidence{ev("w1", "p. 3"), ev("w2", "p. 1")}, ReviewAfter: "2027-01-01"},
	}}
	c := &node.Node{Slug: "c", Tag: "C", Claims: []claims.Claim{
		{ID: "c1", Text: "five", Evidence: []claims.Evidence{ev("w1", "p. 4")}},
	}}
	ix := &cite.Index{Snapshots: map[string]cite.Snapshot{"w1": {Work: "w1", Hash: "h1", Extractor: "x1"}},
		Records: map[string]map[string]cite.Record{}}
	rec := func(n *node.Node, i int, pass bool, verdict string) {
		cl := &n.Claims[i]
		e := cl.Evidence[0]
		if ix.Records[n.Tag] == nil {
			ix.Records[n.Tag] = map[string]cite.Record{}
		}
		ix.Records[n.Tag][cite.RecordID(cl.ID, e.Work, e.Locator)] = cite.Record{Claim: cl.ID, Work: e.Work, Locator: e.Locator,
			Key: cite.Key(cite.Judged(cl), "h1", e.Locator, "x1"), Pass: pass, Verdict: verdict}
	}
	rec(a, 0, true, cite.Supported)
	rec(b, 0, true, cite.Unsupported)
	rec(b, 1, true, cite.Supported) // its second work has no record
	rec(c, 0, true, cite.Supported)

	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	day := 24 * time.Hour
	s2 := []Label{{Name: "finding"}, {Name: "S2"}}
	in := Input{
		Corpus: &node.Corpus{Nodes: []*node.Node{a, b, c}},
		Status: map[string]status.Status{"a": {W: status.Rung{Level: 5}}, "b": {W: status.Rung{Level: 3}}, "c": {W: status.Rung{Level: 1}}},
		Cite:   ix,
		Core:   map[string]bool{"A": true, "not-written": true},
		Now:    now,
		Audits: map[string][]string{"a": {"pass"}, "b": {"fail", "pass"}},
		Links:  &audit.Links{Checked: 200, Dead: []audit.Link{{URL: "https://example.org/gone"}}},
		Issues: []Issue{
			{State: "CLOSED", Labels: s2, CreatedAt: now.Add(-30 * day), ClosedAt: now.Add(-27 * day)},
			{State: "CLOSED", Labels: s2, CreatedAt: now.Add(-30 * day), ClosedAt: now.Add(-20 * day)},
			{State: "OPEN", Labels: s2, CreatedAt: now.Add(-20 * day)},
			{State: "OPEN", Labels: s2, CreatedAt: now.Add(-2 * day)},
			{State: "OPEN", Labels: []Label{{Name: "finding"}, {Name: "severity:S1"}}, CreatedAt: now.Add(-20 * day)},
			{State: "OPEN", Labels: []Label{{Name: "S2"}}, CreatedAt: now.Add(-20 * day)},
		},
		Waivers: []Waiver{{Rule: "W01", End: "2027-06-01"}, {Rule: "W02", End: "2026-01-01"}},
	}
	r := Compute(in)
	if r.Date != "2026-11-15" || r.Nodes != 3 || r.Core != 2 {
		t.Fatalf("date %s, %d nodes, %d core", r.Date, r.Nodes, r.Core)
	}
	byID := map[string]Metric{}
	for _, m := range r.Metrics {
		byID[m.ID] = m
	}
	if len(byID) != 14 {
		t.Fatalf("%d metrics, want the 14 of 15.9", len(byID))
	}
	for _, w := range []struct {
		id       string
		num, den int
		m3, m5   *bool
	}{
		{"g2", 3, 4, no, no},
		{"g3", 1, 3, no, no},
		{"headline", 1, 2, yes, yes},
		{"audit-first-pass", 1, 2, nil, nil},
		{"s2-fixed", 1, 3, no, no},
		{"w5", 1, 2, yes, no},
		{"waivers", 1, 3, no, no},
		{"stale", 1, 5, no, no},
		{"dead-links", 1, 200, yes, no},
	} {
		m := byID[w.id]
		if m.Value == nil {
			t.Errorf("%s: not computed: %s", w.id, m.NotComputed)
			continue
		}
		if m.Num != w.num || m.Den != w.den {
			t.Errorf("%s: %d/%d, want %d/%d", w.id, m.Num, m.Den, w.num, w.den)
		}
		if !same(m.MetM3, w.m3) || !same(m.MetM5, w.m5) {
			t.Errorf("%s: met %v %v, want %v %v", w.id, show(m.MetM3), show(m.MetM5), show(w.m3), show(w.m5))
		}
	}
	if v := *byID["waivers"].Value; math.Abs(v-1000.0/3) > 1e-9 {
		t.Errorf("waivers per 1,000 nodes %v", v)
	}
	for _, id := range []string{"critical-defects", "major-defects", "disagreement", "canary", "s1-hidden"} {
		if m := byID[id]; m.Value != nil || m.NotComputed == "" {
			t.Errorf("%s: want no value and a reason, have %+v", id, m)
		}
	}
	off := r.Off(false)
	ids := []string{}
	for _, m := range off {
		ids = append(ids, m.ID)
	}
	want := []string{"g2", "g3", "s2-fixed", "stale", "waivers"}
	if len(ids) != len(want) {
		t.Fatalf("off M3 %v, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("off M3 %v, want %v", ids, want)
		}
	}
}

func TestComputeEmpty(t *testing.T) {
	r := Compute(Input{Corpus: &node.Corpus{}, Now: time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC)})
	for _, m := range r.Metrics {
		if m.ID == "waivers" {
			if m.Value == nil || *m.Value != 0 {
				t.Errorf("waivers with no register: %+v", m)
			}
			continue
		}
		if m.Value != nil || m.NotComputed == "" || m.MetM3 != nil {
			t.Errorf("%s on an empty tree: %+v", m.ID, m)
		}
	}
}

func TestText(t *testing.T) {
	m := Metric{Unit: Share, Value: ptr(0.955)}
	if s := m.Text(); s != "95.5%" {
		t.Errorf("text %q", s)
	}
	if s := m.TargetText(&Target{Value: 0.9}); s != "90%" {
		t.Errorf("target %q", s)
	}
	if s := m.TargetText(&Target{Value: 0.005, Under: true}); s != "under 0.5%" {
		t.Errorf("target %q", s)
	}
	if s := m.TargetText(nil); s != "tracked" {
		t.Errorf("target %q", s)
	}
	if s := (Metric{Unit: PerK, Value: ptr(4)}).Text(); s != "4.0" {
		t.Errorf("per 1,000 %q", s)
	}
}

var (
	yes = func() *bool { b := true; return &b }()
	no  = func() *bool { b := false; return &b }()
)

func same(a, b *bool) bool { return (a == nil) == (b == nil) && (a == nil || *a == *b) }

func show(b *bool) string {
	if b == nil {
		return "nil"
	}
	if *b {
		return "true"
	}
	return "false"
}
