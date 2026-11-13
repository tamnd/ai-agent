// SPDX-License-Identifier: Apache-2.0

package status

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tamnd/ai-agent/formal"
	"github.com/tamnd/ai-agent/node"
)

func TestLeanLadder(t *testing.T) {
	root := t.TempDir()
	mod := filepath.Join(root, "lean", "AI", "Sorting", "MergeSort.lean")
	if err := os.MkdirAll(filepath.Dir(mod), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mod, []byte("module\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	std := []string{"Classical.choice", "Quot.sound", "propext"}
	const m = "AI.Sorting.MergeSort"
	full := formal.Decls{
		m + ".mergeSort":         {Decl: m + ".mergeSort", Kind: "def", Module: m, Computable: true, Axioms: std},
		"AI.Sorting.IsSort":      {Decl: "AI.Sorting.IsSort", Kind: "def", Module: "AI.Sorting", Computable: true},
		m + ".mergeSort_isSort":  {Decl: m + ".mergeSort_isSort", Kind: "theorem", Module: m, Type: "AI.Sorting.IsSort xs (AI.Sorting.MergeSort.mergeSort xs).ret", Axioms: std},
		m + ".mergeSort_time_le": {Decl: m + ".mergeSort_time_le", Kind: "theorem", Module: m, Axioms: std},
		m + ".bound_isBigO":      {Decl: m + ".bound_isBigO", Kind: "theorem", Module: m, Type: "(fun n => n) =O[Filter.atTop] fun n => n", Axioms: std},
	}
	field := map[string]string{"module": m, "model": m + ".mergeSort", "spec": "AI.Sorting.IsSort", "correct": m + ".mergeSort_isSort", "cost": m + ".mergeSort_time_le"}
	with := func(f func(formal.Decls)) formal.Decls {
		ds := formal.Decls{}
		for k, v := range full {
			ds[k] = v
		}
		f(ds)
		return ds
	}
	tests := []struct {
		name  string
		decls formal.Decls
		level int
	}{
		{"not built", nil, 0},
		{"all proved", full, 3},
		{"sorry in correctness", with(func(ds formal.Decls) {
			d := ds[m+".mergeSort_isSort"]
			d.Axioms, d.Sorry = append(std, "sorryAx"), true
			ds[d.Decl] = d
		}), 1},
		{"wrong statement", with(func(ds formal.Decls) {
			d := ds[m+".mergeSort_isSort"]
			d.Type = "True"
			ds[d.Decl] = d
		}), 1},
		{"no class lemma", with(func(ds formal.Decls) { delete(ds, m+".bound_isBigO") }), 2},
		{"missing model", with(func(ds formal.Decls) { delete(ds, m+".mergeSort") }), 0},
		{"model not computable", with(func(ds formal.Decls) {
			d := ds[m+".mergeSort"]
			d.Computable = false
			ds[d.Decl] = d
		}), 0},
	}
	for _, tc := range tests {
		n := &node.Node{Slug: "merge-sort", Rel: "nodes/sorting/comparison-sorts/merge-sort", Level: "intro", Lean: field}
		r := lean(n, Env{Corpus: &node.Corpus{Root: root}, Decls: tc.decls})
		if r.Level != tc.level {
			t.Errorf("%s: level %d, want %d (next %v)", tc.name, r.Level, tc.level, r.Next)
		}
		if want := "L" + string(rune('0'+tc.level)); r.Label != want {
			t.Errorf("%s: label %q, want %q", tc.name, r.Label, want)
		}
	}
}
