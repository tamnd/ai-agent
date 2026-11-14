// SPDX-License-Identifier: Apache-2.0

package queue

import (
	"reflect"
	"testing"
	"time"

	"github.com/tamnd/ai-agent/check"
	"github.com/tamnd/ai-agent/coverage"
)

func TestGoal(t *testing.T) {
	for _, c := range []struct {
		m    string
		it   coverage.Item
		want int
	}{
		{"M1", coverage.Item{Form: "node", Kind: "method"}, 2},
		{"M1", coverage.Item{Form: "card", Kind: "model"}, 1},
		{"M1", coverage.Item{Form: "card", Kind: "model", Core: true}, 2},
		{"M3", coverage.Item{Form: "node", Kind: "method", Core: true}, 4},
		{"M3", coverage.Item{Form: "node", Kind: "finding", Core: true}, 3},
		{"M5", coverage.Item{Form: "node", Kind: "finding", Core: true}, 5},
		{"M5", coverage.Item{Form: "node", Kind: "finding"}, 2},
	} {
		if got := Goal(c.m, c.it); got != c.want {
			t.Errorf("Goal(%s, %+v) = %d, want %d", c.m, c.it, got, c.want)
		}
	}
}

func TestBuild(t *testing.T) {
	now := time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC)
	r := &coverage.Report{
		Promises: []coverage.Promise{
			{Row: "U:dropout", Outcome: "node", Target: "ai://NEW-dropout", Gap: "missing-node", Area: "deep-learning-training"},
			{Row: "U:mmlu", Outcome: "card", Target: "ai://NEW-mmlu", Gap: "missing-card", Core: true, Area: "evaluation"},
			{Row: "U:bert", Outcome: "node", Target: "ai://NEW-bert", Gap: "below-target", Level: "C2"},
		},
		Items: []coverage.Item{
			{ID: "bleu", Form: "node", Kind: "metric", Area: "evaluation", Level: 1, Label: "C1", Next: []string{"W2"}},
			{ID: "logit", Form: "card", Kind: "definition", Level: 1, Label: "C1"},
			{ID: "llama-3-1-405b", Form: "card", Kind: "model", Level: 0, Label: "C0", Next: []string{"the required fact licence"}},
		},
		Universe: coverage.Universe{Pending: []coverage.Pending{{ID: "U:new", Days: 3, Area: "evaluation"}, {ID: "U:old", Days: 40}}},
	}
	fs := []check.Finding{
		{Rule: "M01", Severity: check.Block, Path: "cards/result/_.jsonl", Line: 4, Msg: "no setting"},
		{Rule: "M01", Severity: check.Block, Path: "cards/result/_.jsonl", Line: 4, Msg: "no date"},
		{Rule: "W02", Severity: check.Block, Path: "nodes/x/node.md", Msg: "not a result rule"},
	}
	old := []Item{
		{ID: "raise:bleu", Lease: &Lease{By: "github:someone", Until: "2026-11-20T00:00:00Z"}},
		{ID: "write:U:dropout", Lease: &Lease{By: "writer", Until: "2026-11-01T00:00:00Z"}}, // ran out
	}
	items := Build(Input{Report: r, Findings: fs, Milestone: "M1", Old: old, Now: now})
	var ids []string
	for _, it := range items {
		ids = append(ids, it.ID)
	}
	want := []string{
		"card:U:mmlu",                    // core first
		"fix:cards/result/_.jsonl:4:M01", // rank 1, no area
		"raise:llama-3-1-405b",           //
		"write:U:dropout",                // deep-learning-training
		"raise:bleu",                     // evaluation
		"decide:U:old", "decide:U:new",   // rank 2, the older first
	}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("queue %v\nwant  %v", ids, want)
	}
	byID := map[string]Item{}
	for _, it := range items {
		byID[it.ID] = it
	}
	if l := byID["raise:bleu"].Lease; l == nil || l.By != "github:someone" {
		t.Errorf("the running lease is lost: %+v", byID["raise:bleu"])
	}
	if byID["write:U:dropout"].Lease != nil {
		t.Error("a lease that ran out is kept")
	}
	if f := byID["fix:cards/result/_.jsonl:4:M01"]; len(f.Next) != 2 || f.Rank != 1 {
		t.Errorf("the two findings of one line are not one item: %+v", f)
	}
	if b := byID["raise:bleu"]; b.Level != "C1" || b.Want != "C2" {
		t.Errorf("raise %+v", b)
	}
	if _, ok := byID["raise:logit"]; ok {
		t.Error("a card at C1 is raised in M1")
	}

	ev := Build(Input{Report: r, Milestone: "M1", Area: "evaluation", Now: now})
	ids = ids[:0]
	for _, it := range ev {
		ids = append(ids, it.ID)
	}
	if !reflect.DeepEqual(ids, []string{"card:U:mmlu", "raise:bleu", "decide:U:new"}) {
		t.Errorf("the area evaluation has %v", ids)
	}
}

func TestReadWrite(t *testing.T) {
	root := t.TempDir()
	if got, err := Read(root); got != nil || err != nil {
		t.Fatalf("no queue gives %v %v", got, err)
	}
	items := []Item{{ID: "raise:bleu", Kind: KindRaise, Target: "bleu", Why: "w", Lease: &Lease{By: "b", Until: "2026-11-20T00:00:00Z"}}}
	if err := Write(root, items); err != nil {
		t.Fatal(err)
	}
	got, err := Read(root)
	if err != nil || !reflect.DeepEqual(got, items) {
		t.Errorf("read %+v %v", got, err)
	}
}
