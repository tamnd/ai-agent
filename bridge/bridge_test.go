// SPDX-License-Identifier: Apache-2.0

package bridge

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const alIndex = `{"commit":"abc","nodes":[
{"tag":"K8YWXT","kind":"problem","slug":"2-sat","name":"2-SAT","area":"logic-and-constraints/sat-solving","summary":"SAT with two literals in each clause.","url":"https://algorithm.tamnd.com/2-sat/"},
{"tag":"5KMEAN","kind":"algorithm","slug":"k-means","name":"k-means","area":"machine-learning/clustering","summary":"Lloyd's algorithm.","url":"https://algorithm.tamnd.com/k-means/"},
{"tag":"QSORT1","kind":"algorithm","slug":"quicksort","name":"Quicksort","area":"sorting/comparison","summary":"Sort by partition.","url":"https://algorithm.tamnd.com/quicksort/"}
]}`

func TestParseIndex(t *testing.T) {
	idx, err := ParseIndex([]byte(`{"nodes":[{"tag":"000AYPA9AJ","kind":"concept","name":"Vector space","msc":["15A03"],"url":"https://math2.tamnd.com/wiki/vector-space/"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := idx.Nodes[0].Slug; got != "vector-space" {
		t.Errorf("the slug from the URL is %q", got)
	}
	if !Sites["m2"].Scope.Has(idx.Nodes[0]) {
		t.Error("the default scope of math2 does not have a linear algebra concept")
	}
	if _, err := ParseIndex([]byte(`{"nodes":[]}`)); err == nil {
		t.Error("an empty index is not an error")
	}
}

func TestSplit(t *testing.T) {
	for _, tc := range []struct {
		ref, site, slug string
		ok              bool
	}{
		{"k-means@al", "al", "k-means", true},
		{"al:k-means", "al", "k-means", true},
		{"m2:kl-divergence", "m2", "kl-divergence", true},
		{"k-means", "", "", false},
		{"k-means@xx", "", "", false},
		{"@al", "", "", false},
	} {
		site, slug, ok := Split(tc.ref)
		if site != tc.site || slug != tc.slug || ok != tc.ok {
			t.Errorf("Split(%q) = %q, %q, %v", tc.ref, site, slug, ok)
		}
	}
}

func TestSync(t *testing.T) {
	root := t.TempDir()
	al := Sites["al"]
	idx, err := ParseIndex([]byte(alIndex))
	if err != nil {
		t.Fatal(err)
	}
	rep, err := Sync(root, al, idx, al.Scope, "2026-11-01")
	if err != nil {
		t.Fatal(err)
	}
	if rep.Cards != 2 || !slices.Equal(rep.Added, []string{"2-sat", "k-means"}) {
		t.Fatalf("the first sync gives %+v", rep)
	}
	if !slices.Equal(rep.Files, []string{"cards/bridge/al/_.jsonl"}) {
		t.Errorf("the files are %v", rep.Files)
	}
	b, _ := os.ReadFile(filepath.Join(root, "cards", "bridge", "al", "_.jsonl"))
	if lines := strings.Split(strings.TrimSpace(string(b)), "\n"); len(lines) != 2 || !strings.Contains(lines[0], `"tag":"5KMEAN"`) {
		t.Errorf("the shard is not sorted by tag:\n%s", b)
	}

	// The neighbour renames k-means and removes 2-SAT. The card follows the tag, and 2-SAT stays as a retired card.
	idx.Nodes = []Item{{Tag: "5KMEAN", Kind: "algorithm", Slug: "k-means-clustering", Name: "k-means clustering", Area: "machine-learning/clustering", URL: "https://algorithm.tamnd.com/k-means-clustering/"}}
	rep, err = Sync(root, al, idx, al.Scope, "2026-11-08")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(rep.Renamed, []string{"k-means -> k-means-clustering"}) || !slices.Equal(rep.Retired, []string{"2-sat"}) {
		t.Fatalf("the second sync gives %+v", rep)
	}
	cs, bad := Read(root)
	if len(bad) > 0 {
		t.Fatal(bad)
	}
	set := NewSet(cs)
	if c := set.Find("k-means@al"); c == nil || c.Slug != "k-means-clustering" {
		t.Errorf("the old slug does not find the renamed card: %+v", c)
	}
	if c := set.Find("al:2-sat"); c == nil || c.Retired != "2026-11-08" {
		t.Errorf("2-SAT is not retired: %+v", c)
	}

	// 2-SAT comes back, and the date of the retirement goes.
	idx, _ = ParseIndex([]byte(alIndex))
	rep, err = Sync(root, al, idx, al.Scope, "2026-11-15")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(rep.Back, []string{"2-sat"}) || !slices.Equal(rep.Renamed, []string{"k-means-clustering -> k-means"}) {
		t.Fatalf("the third sync gives %+v", rep)
	}
	cs, _ = Read(root)
	set = NewSet(cs)
	if c := set.Find("al:k-means"); c == nil || c.Retired != "" || !slices.Equal(c.OldSlugs, []string{"k-means-clustering"}) {
		t.Errorf("k-means is %+v", c)
	}
}

func TestShards(t *testing.T) {
	old := MaxShard
	MaxShard = 300
	defer func() { MaxShard = old }()
	root := t.TempDir()
	idx, _ := ParseIndex([]byte(alIndex))
	pg := Sites["pg"]
	rep, err := Sync(root, pg, idx, Scope{All: true}, "2026-11-01")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(rep.Files, []string{"cards/bridge/pg/5.jsonl", "cards/bridge/pg/K.jsonl", "cards/bridge/pg/Q.jsonl"}) {
		t.Fatalf("the shards are %v", rep.Files)
	}
	MaxShard = old
	if rep, err = Sync(root, pg, idx, Scope{All: true}, "2026-11-01"); err != nil || len(rep.Files) != 1 {
		t.Fatalf("the shards after a merge are %v (%v)", rep.Files, err)
	}
	ents, _ := os.ReadDir(filepath.Join(root, "cards", "bridge", "pg"))
	if len(ents) != 1 {
		t.Errorf("the old shards are still there: %v", ents)
	}
}

func TestLoadScope(t *testing.T) {
	root := t.TempDir()
	al := Sites["al"]
	sc, err := LoadScope(root, al)
	if err != nil || !slices.Equal(sc.Areas, al.Scope.Areas) {
		t.Fatalf("the default scope is %+v (%v)", sc, err)
	}
	if err := os.MkdirAll(filepath.Join(root, "crosswalk"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "crosswalk", "algorithm.yaml"), []byte("bridge:\n  areas: [sorting]\n  tags: [K8YWXT]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sc, err = LoadScope(root, al)
	if err != nil {
		t.Fatal(err)
	}
	if !sc.Has(Item{Tag: "QSORT1", Area: "sorting/comparison"}) || !sc.Has(Item{Tag: "K8YWXT", Area: "logic-and-constraints"}) || sc.Has(Item{Tag: "5KMEAN", Area: "machine-learning"}) {
		t.Errorf("the scope of crosswalk/algorithm.yaml is wrong: %+v", sc)
	}
}
