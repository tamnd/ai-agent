// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"testing"

	"github.com/tamnd/ai-agent/card"
	"github.com/tamnd/ai-agent/cite"
	"github.com/tamnd/ai-agent/node"
	"github.com/tamnd/ai-agent/results"
)

// aik cite results finds the score of a result card in the passage of its locator, as a fraction or a percent, at
// the precision of the card.
func TestCiteResult(t *testing.T) {
	text := "1 Introduction\nWe evaluate on MMLU.\n\nTable 2: Results of the models.\nModel MMLU GSM8K\nOurs 405B 87.3 96.8\nBase 70B 79.3 83.7\n"
	store := cite.Store{Dir: t.TempDir()}
	h, err := store.Put([]byte(text), cite.Meta{URL: "https://example.org/p.txt"})
	if err != nil {
		t.Fatal(err)
	}
	wc, err := card.Parse([]byte(`{"tag":"9W0001","kind":"work","slug":"paper-a","name":"Paper A","cited":{"version":"v1","url":"https://example.org/p.txt","format":"text"}}`))
	if err != nil {
		t.Fatal(err)
	}
	c := &node.Corpus{CardByKey: map[string]*card.Card{"paper-a": wc}, BySlug: map[string]*node.Node{}}
	snap := cite.Snapshot{Work: "paper-a", Version: "v1", URL: "https://example.org/p.txt", Hash: h}
	ck := &citer{c: c, ix: &cite.Index{Snapshots: map[string]cite.Snapshot{"paper-a": snap}}, store: store, offline: true,
		docs: map[string]*cite.Doc{h: cite.Parse(text)}, prev: map[string]*cite.Doc{}, failed: map[string]string{}}
	f := func(v float64) *float64 { return &v }
	for _, tc := range []struct {
		loc  string
		s    results.Score
		want string
	}{
		{"Table 2", results.Score{V: f(0.873), Unit: "fraction"}, results.CiteFound},
		{"Table 2", results.Score{V: f(87.3), Unit: "percent"}, results.CiteFound},
		{"Table 2", results.Score{V: f(0.968), Unit: "fraction"}, results.CiteFound},
		{"Table 2", results.Score{V: f(0.874), Unit: "fraction"}, results.CiteNotFound},
		{"Table 2", results.Score{V: f(0.8731), Unit: "fraction"}, results.CiteNotFound}, // more exact than the source
		{"Table 7", results.Score{V: f(0.873), Unit: "fraction"}, results.CiteNotFound},
		{"Table 2", results.Score{Unit: "fraction"}, results.CiteNotFound},
	} {
		if got, why := ck.result(context.Background(), "paper-a", tc.loc, tc.s); got != tc.want {
			t.Errorf("%s, %v: %s (%s), want %s", tc.loc, *orZero(tc.s.V), got, why, tc.want)
		}
	}
	if got, _ := ck.result(context.Background(), "paper-b", "Table 2", results.Score{V: f(0.873), Unit: "fraction"}); got != "unchecked" {
		t.Errorf("a work with no card: %s, want unchecked", got)
	}
}

func orZero(v *float64) *float64 {
	if v == nil {
		return new(float64)
	}
	return v
}
