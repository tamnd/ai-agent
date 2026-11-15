// SPDX-License-Identifier: Apache-2.0

package reports

import (
	"testing"

	"github.com/tamnd/ai-agent/exp"
	"github.com/tamnd/ai-agent/node"
	"github.com/tamnd/ai-agent/status"
)

func corpus(t *testing.T) (*node.Corpus, map[string]status.Status) {
	t.Helper()
	c, err := node.Load("../testdata/corpus")
	if err != nil {
		t.Fatal(err)
	}
	res := map[string]map[string]status.Test{
		"binary-search": {"go": {Slug: "binary-search", Lang: "go", Pass: true, Ms: 12,
			Shared:   []status.Shared{{Problem: "search", Cases: 10, Passed: 9, Skipped: 1}},
			Mutation: &status.Mutation{Mutants: 10, Killed: 9}}},
	}
	st := map[string]status.Status{}
	for _, n := range c.Nodes {
		if c.BySlug[n.Slug] == n {
			st[n.Slug] = status.Of(n, status.Env{Corpus: c, Results: res})
		}
	}
	return c, st
}

func TestLevels(t *testing.T) {
	c, st := corpus(t)
	r := MakeLevels(c, st)
	if r.Nodes != len(st) || r.Nodes == 0 {
		t.Fatalf("nodes = %d, want %d", r.Nodes, len(st))
	}
	byID := map[string]Ladder{}
	for _, l := range r.Ladders {
		byID[l.ID] = l
		sum := 0
		for _, n := range l.Total {
			sum += n
		}
		if len(l.Total) != len(l.Levels) {
			t.Errorf("%s: %d counts for %d levels", l.ID, len(l.Total), len(l.Levels))
		}
		if l.ID != "I" && l.ID != "X" && sum != r.Nodes && l.ID != "D" && l.ID != "R" {
			t.Errorf("%s counts %d nodes, want %d", l.ID, sum, r.Nodes)
		}
	}
	w, ok := byID["W"]
	if !ok || len(w.ByKind) == 0 || len(w.ByArea) == 0 {
		t.Fatalf("W = %+v", w)
	}
	if _, ok := w.ByArea["searching"]; !ok {
		t.Errorf("W by area = %v, want searching", w.ByArea)
	}
	if i, ok := byID["I"]; !ok || len(i.ByLang) != 2 {
		t.Errorf("I = %+v, want two languages", byID["I"])
	}
}

func TestTests(t *testing.T) {
	c, st := corpus(t)
	r := MakeTests(c, st)
	if r.Total.Impls != 2 || r.Total.Pass != 1 || r.Total.NotRun != 1 || len(r.Langs) != 2 {
		t.Fatalf("tests = %+v", r)
	}
	var row TestRow
	for _, x := range r.Rows {
		if x.Lang == "go" {
			row = x
		}
	}
	if row.State != "pass" || row.Shared != [2]int{9, 9} || row.Mutation != 0.9 || row.Ms != 12 {
		t.Errorf("go row = %+v", row)
	}
}

func TestExperiments(t *testing.T) {
	c, _ := corpus(t)
	n := c.BySlug["binary-search"]
	n.Exps = []*exp.Experiment{
		{Slug: "e1", Design: &exp.Design{Claims: []string{"c1"}, Budget: "cpu-small", Arms: []exp.Arm{{ID: "a"}, {ID: "b"}}, Seeds: exp.Seeds{N: 4}}},
		{Slug: "e2"},
	}
	r := MakeExperiments(c)
	if r.Total != 2 || r.ByVerdict["not run"] != 2 || r.ByLevel["R0"] != 2 || r.ByBudget["cpu-small"] != 1 || r.ByBudget["none"] != 1 {
		t.Fatalf("experiments = %+v", r)
	}
	if x := r.Rows[0]; x.Exp != "e1" || x.Arms != 2 || x.Seeds != 4 || len(x.Next) == 0 {
		t.Errorf("row = %+v", x)
	}
	if TopArea("") != "none" || TopArea("a/b") != "a" {
		t.Error("TopArea")
	}
}
