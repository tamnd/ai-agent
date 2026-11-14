// SPDX-License-Identifier: Apache-2.0

package coverage

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/tamnd/ai-agent/card"
	"github.com/tamnd/ai-agent/harvest"
	"github.com/tamnd/ai-agent/node"
	"github.com/tamnd/ai-agent/status"
)

func rung(l int) status.Rung { return status.Rung{Level: l} }

func TestNodeLevel(t *testing.T) {
	full := func() status.Status {
		return status.Status{W: rung(5), G: rung(3), H: rung(3), L: rung(4), V: rung(3),
			I: map[string]status.Rung{"numpy": rung(4), "pytorch": rung(4), "jax": rung(4)}}
	}
	with := func(f func(*status.Status)) status.Status {
		s := full()
		f(&s)
		return s
	}
	d := func(l int) *status.Rung { r := rung(l); return &r }
	for _, tc := range []struct {
		name string
		kind string
		st   status.Status
		want int
		next string
	}{
		{"stub", "concept", status.Status{}, 1, "the writing at W2"},
		{"no sources", "concept", with(func(s *status.Status) { s.G = rung(0) }), 1, "the grounding at G1"},
		{"paged", "concept", with(func(s *status.Status) { s.W, s.G = rung(2), rung(1) }), 2, "the writing at W3"},
		{"facts at D1", "model", with(func(s *status.Status) { s.D = d(1) }), 2, "the data facts at D2"},
		{"grounded, not verified", "concept", with(func(s *status.Status) { s.G = rung(2) }), 3, "the grounding at G3"},
		{"no jax code", "method", with(func(s *status.Status) { delete(s.I, "jax") }), 3, "the jax code at X2 (none yet)"},
		{"a concept has no code", "concept", with(func(s *status.Status) { s.I = nil }), 6, ""},
		{"no small run", "method", with(func(s *status.Status) { s.R = d(0) }), 3, "the experiments at R1"},
		{"jax at X2 is complete", "method", with(func(s *status.Status) { s.W = rung(4); s.I["jax"] = rung(2) }), 5, "the writing at W5"},
		{"pytorch at X2", "method", with(func(s *status.Status) { s.I["pytorch"] = rung(2) }), 4, "the pytorch code at X3"},
		{"reviewed", "method", full(), 6, ""},
	} {
		n := &node.Node{Front: node.Front{Kind: tc.kind}}
		got, next := NodeLevel(n, tc.st)
		if got != tc.want || !strings.Contains(strings.Join(next, "; "), tc.next) || (tc.next == "" && len(next) > 0) {
			t.Errorf("%s: C%d, next %q; want C%d with %q", tc.name, got, next, tc.want, tc.next)
		}
	}
}

func TestCardLevel(t *testing.T) {
	for _, tc := range []struct {
		name string
		c    card.Card
		want int
	}{
		{"valid", card.Card{Kind: "work"}, 1},
		{"bad field", card.Card{Kind: "work", Bad: []string{"no title"}}, 0},
	} {
		if got, _ := CardLevel(&tc.c); got != tc.want {
			t.Errorf("%s: C%d, want C%d", tc.name, got, tc.want)
		}
	}
}

func TestCompute(t *testing.T) {
	drop := &node.Node{Front: node.Front{Kind: "method", Slug: "dropout", Area: "deep-learning-training/regularization"}}
	ov := &node.Node{Front: node.Front{Kind: "concept", Slug: "overfitting", Area: "supervised-learning/basics"}}
	w := &card.Card{Kind: "work", Slug: "srivastava-2014", Tag: "5TQH2W"}
	c := &node.Corpus{Nodes: []*node.Node{drop, ov}, BySlug: map[string]*node.Node{"dropout": drop, "overfitting": ov},
		Cards: []*card.Card{w}, CardByKey: map[string]*card.Card{"srivastava-2014": w}}
	st := map[string]status.Status{
		"dropout":     {W: rung(3), G: rung(2)},
		"overfitting": {W: rung(2), G: rung(1)},
	}
	rows := []Row{
		{ID: "u:1", Outcome: "node", Target: "ai://NEW-dropout", Core: true, Sources: []string{"arxiv", "canon:goodfellow-2016-deep-learning:7.12"}, File: "deep-learning-training.jsonl"},
		{ID: "u:2", Outcome: "node", Target: "ai://NEW-batch-norm", Core: true, Sources: []string{"canon:goodfellow-2016-deep-learning:8.7"}, File: "deep-learning-training.jsonl"},
		{ID: "u:3", Outcome: "card", Target: "srivastava-2014", Sources: []string{"openalex"}, File: "deep-learning-training.jsonl"},
		{ID: "u:4", Outcome: "card", Target: "hinton-2012", Sources: []string{"openalex"}, File: "deep-learning-training.jsonl"},
		{ID: "u:6", Outcome: "card", Target: "ai://5TQH2W", Sources: []string{"openalex"}, File: "deep-learning-training.jsonl"},
		{ID: "u:5", Outcome: "pending", Added: "2026-11-01", Sources: []string{"pwc-dump:methods"}, File: "deep-learning-training.jsonl"},
		{ID: "u:7", Outcome: "card", Target: "srivastava-2014", Sources: []string{"epoch-ai:GPT-4", "huggingface:org/m", "helm:lite:x"}, File: "deep-learning-training.jsonl"},
	}
	in := Input{Corpus: c, Status: st, Rows: rows, Canon: []Canon{{ID: "goodfellow-2016-deep-learning"}},
		Last: &Counts{0, 1, 1}, LastVersion: "2027.02", Now: time.Date(2027, 3, 1, 0, 0, 0, 0, time.UTC)}
	r := Compute(in)
	if r.Levels.Total != (Counts{0, 1, 1, 1}) {
		t.Errorf("total %v", r.Levels.Total)
	}
	if got := strings.Join(r.Gaps["missing-node"], ","); got != "u:2" {
		t.Errorf("missing-node %q", got)
	}
	if got := strings.Join(r.Gaps["missing-card"], ","); got != "u:4" {
		t.Errorf("missing-card %q", got)
	}
	if len(r.Universe.Pending) != 1 || r.Universe.Pending[0].Days != 120 {
		t.Errorf("pending %+v", r.Universe.Pending)
	}
	if r.Universe.Unregistered != 1 || len(r.Core) != 1 || r.Core[0].ID != "dropout" {
		t.Errorf("unregistered %d, core %+v", r.Universe.Unregistered, r.Core)
	}
	if r.Canon[0].Named != 2 || r.Canon[0].AtC2 != 1 {
		t.Errorf("canon %+v", r.Canon[0])
	}
	if r.Levels.Change == nil || r.Levels.Change[3] != 1 || r.Levels.Change[1] != 0 {
		t.Errorf("change %v", r.Levels.Change)
	}
	if len(r.Fails) != 0 {
		t.Errorf("fails before M3: %q", r.Fails)
	}
	// A row of the harvest source epoch-ai is a row of the source epoch of 18.4; a row of helm, of the leaderboards.
	for _, s := range []string{"epoch", "hf-hub", "leaderboards", "arxiv", "openalex", "pwc-dump"} {
		if slices.Contains(r.Universe.Unharvested, s) {
			t.Errorf("%s is unharvested, though a row has it: %q", s, r.Universe.Unharvested)
		}
	}
	if !slices.Contains(r.Universe.Unharvested, "aiid") {
		t.Errorf("aiid has no row but is not unharvested: %q", r.Universe.Unharvested)
	}
	// A closed source is named with its reason, and is not a gap.
	if slices.Contains(r.Universe.Unharvested, "openreview") || !strings.Contains(r.Universe.Closed["openreview"], "ChallengeRequiredError") {
		t.Errorf("openreview: unharvested %q, closed %q", r.Universe.Unharvested, r.Universe.Closed)
	}
	in.Now = M3End.Add(time.Hour)
	if r := Compute(in); len(r.Fails) != 1 || !strings.Contains(r.Fails[0], "u:2") {
		t.Errorf("fails after M3: %q", r.Fails)
	}
}

// TestClosedSources checks that a closed source is a source of 18.4 with a reason and with no adapter.
func TestClosedSources(t *testing.T) {
	for s, why := range ClosedSources {
		if !slices.Contains(Sources, s) || why == "" {
			t.Errorf("closed source %s: in 18.4 %v, reason %q", s, slices.Contains(Sources, s), why)
		}
		for _, id := range harvest.Names() {
			if SourceOf(id) == s {
				t.Errorf("closed source %s has the adapter %s", s, id)
			}
		}
	}
}

// TestSourceOf checks that every harvest adapter counts for a source of 18.4, so that a harvest that ran is not
// reported as unharvested.
func TestSourceOf(t *testing.T) {
	for _, id := range harvest.Names() {
		if !slices.Contains(Sources, SourceOf(id)) {
			t.Errorf("the harvest source %s is %s, which is not a source of 18.4", id, SourceOf(id))
		}
	}
}

func TestLoadUniverse(t *testing.T) {
	root := t.TempDir()
	if rows, err := LoadUniverse(root); err != nil || rows != nil {
		t.Fatalf("no register: %v, %v", rows, err)
	}
	dir := filepath.Join(root, "registers", "universe")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(s string) {
		if err := os.WriteFile(filepath.Join(dir, "x.jsonl"), []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(`{"id": "u:1", "outcome": "node", "target": "ai://NEW-x"}` + "\n\n")
	if rows, err := LoadUniverse(root); err != nil || len(rows) != 1 || rows[0].File != "x.jsonl" {
		t.Errorf("one row: %v, %v", rows, err)
	}
	write(`{"id": "u:1", "outcome": "maybe"}`)
	if _, err := LoadUniverse(root); err == nil || !strings.Contains(err.Error(), "x.jsonl:1") {
		t.Errorf("bad outcome: %v", err)
	}
}
