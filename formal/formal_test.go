// SPDX-License-Identifier: Apache-2.0

package formal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tamnd/ai-agent/node"
)

func TestLoadAndCheck(t *testing.T) {
	path := filepath.Join(t.TempDir(), "decls.jsonl")
	lines := []string{
		`{"decl":"Algo.M.model","kind":"def","module":"Algo.M","axioms":[],"computable":true}`,
		`{"decl":"Algo.M.good","kind":"theorem","module":"Algo.M","axioms":["Classical.choice","Quot.sound","propext"]}`,
		`{"decl":"Algo.M.bad","kind":"theorem","module":"Algo.M","axioms":["propext","sorryAx"],"sorry":true}`,
		`{"decl":"Algo.M.fast","kind":"theorem","module":"Algo.M","axioms":["Lean.ofReduceBool"]}`,
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ds, err := Load(path)
	if err != nil || len(ds) != 4 {
		t.Fatalf("Load = %d decls, %v", len(ds), err)
	}
	c := &node.Corpus{Nodes: []*node.Node{{Slug: "m", Rel: "nodes/sorting/m", Lean: map[string]string{
		"module":  "Algo.M",
		"model":   "Algo.M.good",
		"correct": "Algo.M.missing",
	}}}}
	var got []string
	for _, f := range Check(c, ds) {
		got = append(got, f.String())
	}
	want := []string{
		"L03 Algo.M.bad: uses the axiom sorryAx (a sorry)",
		"L03 Algo.M.fast: uses the axiom Lean.ofReduceBool",
		"L01 m lean.correct: Algo.M.missing is not a declaration of the Algo package",
		"L01 m lean.model: Algo.M.good is of the kind theorem, want def",
		"L09 m lean.module: Algo.M does not mirror the node path; use a module such as Algo.Sorting.M",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("Check =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestClasses(t *testing.T) {
	for typ, want := range map[string]string{
		"(fun n => ↑n * ↑(Nat.clog 2 n)) =O[Filter.atTop] fun n => ↑n * Real.log ↑n": "nlogn",
		"f =Θ[Filter.atTop] fun (n : ℕ) => (n : ℝ) ^ 2":                              "n^2",
		"∀ n, f n ≤ n": "",
	} {
		if got := ClassOf(typ); got != want {
			t.Errorf("ClassOf(%q) = %q, want %q", typ, got, want)
		}
	}
	for claim, want := range map[string]string{"Θ(n log n)": "nlogn", "O(n lg n)": "nlogn", "Θ(n^2)": "n^2", "Θ(n²)": "n²", "amortized": ""} {
		if got := ClaimClass(claim); got != want {
			t.Errorf("ClaimClass(%q) = %q, want %q", claim, got, want)
		}
	}
}

func TestCheckClaims(t *testing.T) {
	ds := Decls{
		"Algo.M.cost":  {Decl: "Algo.M.cost", Kind: "theorem", Module: "Algo.M"},
		"Algo.M.class": {Decl: "Algo.M.class", Kind: "theorem", Module: "Algo.M", Type: "f =O[Filter.atTop] fun n => ↑n * Real.log ↑n"},
	}
	n := &node.Node{Slug: "m", Lean: map[string]string{"module": "Algo.M", "cost": "Algo.M.cost"}, Complexity: &node.Complexity{
		Time:  map[string]string{"worst": "Θ(n log n)", "best": "Θ(n)"},
		Space: map[string]string{"extra": "Θ(n)"},
	}}
	var got []string
	for _, f := range CheckClaims(n, ds) {
		got = append(got, f.String())
	}
	want := "L05 m time.best: Θ(n) has no class lemma in Algo.M"
	if strings.Join(got, "\n") != want {
		t.Errorf("CheckClaims = %q, want %q", got, want)
	}
	delete(n.Lean, "cost")
	if fs := CheckClaims(n, ds); len(fs) != 0 {
		t.Errorf("a node without lean.cost gets findings: %v", fs)
	}
}

func TestMirrors(t *testing.T) {
	rel := "nodes/sorting/comparison-sorts/merge-sort"
	for mod, want := range map[string]bool{
		"Algo.Sorting.ComparisonSorts.MergeSort": true,
		"Algo.Sorting.MergeSort":                 true,
		"Algo.Sorting.Merge":                     false,
		"Algo.Graphs.MergeSort":                  false,
		"Algo.Sorting.Other.MergeSort":           false,
		"Algo.MergeSort":                         false,
	} {
		if got := Mirrors(mod, rel); got != want {
			t.Errorf("Mirrors(%q) = %v, want %v", mod, got, want)
		}
	}
	if got := MirrorOf("nodes/graphs/shortest-paths/dijkstra"); got != "Algo.Graphs.ShortestPaths.Dijkstra" {
		t.Errorf("MirrorOf = %q", got)
	}
}

func TestStatementAndModel(t *testing.T) {
	ds := Decls{
		"Algo.S.M.run":     {Decl: "Algo.S.M.run", Kind: "def", Module: "Algo.S.M"},
		"Algo.S.M.run'":    {Decl: "Algo.S.M.run'", Kind: "opaque", Module: "Algo.S.M"},
		"Algo.S.P.Spec":    {Decl: "Algo.S.P.Spec", Kind: "def", Module: "Algo.S.P"},
		"Algo.S.M.correct": {Decl: "Algo.S.M.correct", Kind: "theorem", Module: "Algo.S.M", Type: "∀ xs, Algo.S.P.Spec xs (Algo.S.M.run' xs)"},
	}
	n := &node.Node{Slug: "m", Rel: "nodes/s/m", Lean: map[string]string{"module": "Algo.S.M", "model": "Algo.S.M.run", "spec": "Algo.S.P.Spec", "correct": "Algo.S.M.correct"}}
	var got []string
	for _, f := range CheckNode(n, ds) {
		got = append(got, f.String())
	}
	want := []string{
		"L02 m lean.correct: the statement of Algo.S.M.correct does not mention the model Algo.S.M.run",
		"L07 m lean.model: Algo.S.M.run is not computable, so the oracle cannot run it",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("CheckNode =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	n.Lean["model"] = "Algo.S.M.run'"
	got = got[:0]
	for _, f := range CheckNode(n, ds) {
		got = append(got, f.String())
	}
	if len(got) != 2 || !strings.Contains(got[0], "is of the kind opaque, want def") || !strings.Contains(got[1], "L07 m lean.model: Algo.S.M.run' is opaque") {
		t.Errorf("CheckNode with an opaque model = %q", got)
	}
}

func TestCheckTimes(t *testing.T) {
	ts := []ModuleTime{
		{Module: "Algo.A", File: "Algo/A.lean", Took: 5 * time.Second},
		{Module: "Algo.B", File: "Algo/B.lean", Took: 90 * time.Second},
		{Module: "Algo.C", File: "Algo/C.lean", Took: 400 * time.Second},
	}
	fs := CheckTimes(ts)
	if len(fs) != 2 {
		t.Fatalf("CheckTimes = %v", fs)
	}
	if fs[0].Where != "Algo/B.lean" || fs[0].Sev() != "warn" || !strings.Contains(fs[0].Msg, "takes 1m30s") {
		t.Errorf("the slow module: %+v", fs[0])
	}
	if fs[1].Where != "Algo/C.lean" || fs[1].Sev() != "block" {
		t.Errorf("the very slow module: %+v", fs[1])
	}
	if (Finding{Rule: "L03"}).Sev() != "block" {
		t.Error("L03 is not a block rule")
	}
}

func TestOwnTime(t *testing.T) {
	log := "Algo/Cost.lean:1:0: elaboration took 105ms\ncumulative profiling times:\n\telaboration 63ms\n\timport 8.46s\n\tinitialization 277ms\n\tinterpretation 1.48s\n\ttype checking 1.04ms\n\tfix level params 0.5μs\n"
	got, ok := ownTime(log)
	want := 63*time.Millisecond + 1480*time.Millisecond + 1040*time.Microsecond + 500*time.Nanosecond
	if !ok || got != want {
		t.Errorf("ownTime = %v %v, want %v", got, ok, want)
	}
	if _, ok := ownTime("no profile"); ok {
		t.Error("ownTime reads a log with no profile")
	}
}
