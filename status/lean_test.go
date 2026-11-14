// SPDX-License-Identifier: Apache-2.0

package status

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/ai-agent/formal"
	"github.com/tamnd/ai-agent/internal/reviewtest"
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

// A theorem is on the F ladder: its statement, then its proof with the allowed axioms. The keys expected and steps are
// not declarations (11.4). Its human proof is the derivation of kind proof.
func TestTheoremLadders(t *testing.T) {
	root := t.TempDir()
	const m = "AI.LearningTheory.PacBoundFiniteHypothesisClass"
	mod := filepath.Join(root, "lean", "AI", "LearningTheory", "PacBoundFiniteHypothesisClass.lean")
	if err := os.MkdirAll(filepath.Dir(mod), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mod, []byte("module\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	std := []string{"Classical.choice", "Quot.sound", "propext"}
	stmt := m + ".pac_bound_realizable"
	proved := formal.Decls{stmt: {Decl: stmt, Kind: "theorem", Module: m, Axioms: std}}
	sorry := formal.Decls{stmt: {Decl: stmt, Kind: "theorem", Module: m, Axioms: append(std, "sorryAx"), Sorry: true}}
	notThm := formal.Decls{stmt: {Decl: stmt, Kind: "def", Module: m, Computable: true}}
	field := map[string]string{"module": m, "statement": stmt, "expected": "true", "steps": "derivations/proof.md"}
	tests := []struct {
		name  string
		lean  map[string]string
		decls formal.Decls
		level int
	}{
		{"no lean part", nil, proved, 0},
		{"not built", field, nil, 0},
		{"proved", field, proved, 2},
		{"sorry", field, sorry, 1},
		{"not a theorem", field, notThm, 0},
		{"no statement", map[string]string{"module": m, "expected": "true"}, proved, 0},
	}
	for _, tc := range tests {
		n := &node.Node{Front: node.Front{Kind: "theorem", Slug: "pac-bound-finite-hypothesis-class", Level: "intro", Lean: tc.lean}, Rel: "nodes/learning-theory/pac-learning/pac-bound-finite-hypothesis-class"}
		r := lean(n, Env{Corpus: &node.Corpus{Root: root}, Decls: tc.decls})
		if r.Level != tc.level || r.Label != "F"+string(rune('0'+tc.level)) {
			t.Errorf("%s: %s, want F%d (next %v)", tc.name, r.Label, tc.level, r.Next)
		}
	}

	n := &node.Node{Front: node.Front{Kind: "theorem", Tag: "XSZ95D"}}
	env := Env{Corpus: &node.Corpus{Root: root}}
	if r := human(n, env); r.Level != 0 || r.Label != "H0" {
		t.Errorf("no derivation: %s", r.Label)
	}
	path := "nodes/t/derivations/proof.md"
	if err := os.MkdirAll(filepath.Join(root, "nodes/t/derivations"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, path), []byte("---\nkind: proof\n---\nthe proof\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	n.Derivations = []node.Derivation{{ID: "bound", Path: "nodes/t/derivations/bound.md", Front: node.DerivationFront{Kind: "bound", Status: "full"}}}
	if r := human(n, env); r.Level != 0 {
		t.Errorf("a bound is not a proof: %s", r.Label)
	}
	n.Derivations = append(n.Derivations, node.Derivation{ID: "proof", Path: path, Front: node.DerivationFront{Kind: "proof", Status: "sketch"}})
	if r := human(n, env); r.Level != 1 {
		t.Errorf("a sketch: %s, want H1", r.Label)
	}
	n.Derivations[1].Front.Status = "full"
	if r := human(n, env); r.Level != 2 {
		t.Errorf("a full proof: %s, want H2 (next %v)", r.Label, r.Next)
	}
	reviewtest.Pass(t, root, n, "proof-proof", filepath.Join(root, path))
	if r := human(n, env); r.Level != 3 {
		t.Errorf("a reviewed proof: %s, want H3 (next %v)", r.Label, r.Next)
	}
}

// F3 links the steps of the page proof to the Lean proof (11.6, F08); on a core node the lean-statement review must
// also hash the page proof.
func TestF3(t *testing.T) {
	root := t.TempDir()
	const m = "AI.T.Thm"
	rel := "nodes/t/thm"
	mod := filepath.Join(root, "lean", "AI", "T", "Thm.lean")
	proof := filepath.Join(root, rel, "derivations", "proof.md")
	write := func(path, s string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(mod, "-- step: one\ntheorem a : True := trivial\n\ntheorem thm : True := by\n  -- step: two\n  have h : True := trivial\n  exact h\n")
	write(proof, "1. A. {#step-one}\n2. B. {#step-two}\n")
	std := []string{"propext"}
	stmt := m + ".thm"
	ds := formal.Decls{
		m + ".a": {Decl: m + ".a", Kind: "theorem", Module: m, Type: "True", Axioms: std, Pos: &formal.Pos{Line: 2, EndLine: 2}},
		stmt:     {Decl: stmt, Kind: "theorem", Module: m, Type: "True", Axioms: std, Pos: &formal.Pos{Line: 4, EndLine: 7}},
	}
	n := &node.Node{Front: node.Front{Kind: "theorem", Slug: "thm", Tag: "AAAAAA", Level: "intro",
		Lean: map[string]string{"module": m, "statement": stmt, "steps": "derivations/proof.md"}},
		Rel: rel, Dir: filepath.Join(root, rel)}
	env := Env{Corpus: &node.Corpus{Root: root}, Decls: ds}
	if r := lean(n, env); r.Level != 3 || r.Label != "F3" {
		t.Errorf("linked steps: %s, want F3 (next %v)", r.Label, r.Next)
	}
	write(proof, "1. B. {#step-two}\n2. A. {#step-one}\n")
	if r := lean(n, env); r.Level != 2 || len(r.Next) != 1 || !strings.Contains(r.Next[0], "(F08)") {
		t.Errorf("steps out of order: %s, want F2 with F08 (next %v)", r.Label, r.Next)
	}
	write(proof, "1. A. {#step-one}\n2. B. {#step-two}\n")
	n.Level = "core"
	reviewtest.Pass(t, root, n, "lean-statement", mod)
	if r := lean(n, env); r.Level != 2 || !strings.Contains(strings.Join(r.Next, " "), "does not hash derivations/proof.md") {
		t.Errorf("a core node whose review does not hash the page proof: %s, want F2 (next %v)", r.Label, r.Next)
	}
	reviewtest.Pass(t, root, n, "lean-statement", mod, proof)
	if r := lean(n, env); r.Level != 3 {
		t.Errorf("a core node with the steps reviewed: %s, want F3 (next %v)", r.Label, r.Next)
	}
	n.Lean["steps"] = ""
	if r := lean(n, env); r.Level != 2 || !strings.Contains(r.Next[0], "lean.steps") {
		t.Errorf("no lean.steps: %s, want F2 (next %v)", r.Label, r.Next)
	}
}
