// SPDX-License-Identifier: Apache-2.0

package formal

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/tamnd/ai-agent/node"
)

const stepsMod = "AI.T.Thm"

const stepsSrc = `theorem lemma_a : True := trivial

-- step: one
/-- A doc comment. -/
theorem lemma_b : 1 = 1 := rfl

theorem main_thm : 2 = 2 := by
  -- step: step-two
  have h : 1 + 1 = 2 := by
    rfl
  exact rfl
`

func stepsDecls(sorry bool) Decls {
	std := []string{"propext"}
	ds := Decls{
		stepsMod + ".lemma_a":  {Decl: stepsMod + ".lemma_a", Kind: "theorem", Module: stepsMod, Type: "True", Axioms: std, Pos: &Pos{Line: 1, EndLine: 1}},
		stepsMod + ".lemma_b":  {Decl: stepsMod + ".lemma_b", Kind: "theorem", Module: stepsMod, Type: "1 = 1", Axioms: std, Pos: &Pos{Line: 4, EndLine: 5}},
		stepsMod + ".main_thm": {Decl: stepsMod + ".main_thm", Kind: "theorem", Module: stepsMod, Type: "2 = 2", Axioms: std, Pos: &Pos{Line: 7, EndLine: 11}},
	}
	if sorry {
		d := ds[stepsMod+".main_thm"]
		d.Sorry, d.Axioms = true, append(d.Axioms, "sorryAx")
		ds[stepsMod+".main_thm"] = d
	}
	return ds
}

func TestLeanSteps(t *testing.T) {
	steps := LeanSteps([]byte(stepsSrc), stepsMod, stepsDecls(false))
	if len(steps) != 2 {
		t.Fatalf("got %d steps, want 2: %+v", len(steps), steps)
	}
	a, b := steps[0], steps[1]
	if a.ID != "one" || a.Line != 3 || a.Decl != stepsMod+".lemma_b" || a.Text != "1 = 1" || !a.Builds {
		t.Errorf("a step that heads a theorem: %+v", a)
	}
	if b.ID != "two" || b.Line != 8 || b.Decl != stepsMod+".main_thm" || b.Text != "have h : 1 + 1 = 2" || !b.Builds {
		t.Errorf("a step inside a proof: %+v", b)
	}
	if s := LeanSteps([]byte(stepsSrc), stepsMod, stepsDecls(true)); s[1].Builds {
		t.Error("a step in a proof with sorry builds")
	}
	if s := LeanSteps([]byte(stepsSrc), stepsMod, nil); len(s) != 2 || s[0].Decl != "" || s[0].Text != "theorem lemma_b : 1 = 1" {
		t.Errorf("without the build: %+v", s)
	}
}

func TestCheckSteps(t *testing.T) {
	n := &node.Node{Front: node.Front{Slug: "thm", Lean: map[string]string{"steps": "derivations/proof.md"}}}
	steps := LeanSteps([]byte(stepsSrc), stepsMod, stepsDecls(false))
	tests := []struct {
		name string
		page []string
		lean []Step
		want []string // a part of the message of each finding, in order
	}{
		{"linked", []string{"one", "two"}, steps, nil},
		{"no ids", nil, steps, []string{"gives no step an id"}},
		{"missing", []string{"one", "two", "three"}, steps, []string{"step-three of the page has no -- step: three"}},
		{"not on the page", []string{"one"}, steps, []string{"two (AI.T.Thm line 8) is not a step of derivations/proof.md"}},
		{"order", []string{"two", "one"}, steps, []string{"in the order one, two, but the page has two, one"}},
		{"twice", []string{"one", "two"}, append(slices.Clone(steps), steps[1]), []string{"two is marked twice, at lines 8 and 8"}},
		{"sorry", []string{"one", "two"}, LeanSteps([]byte(stepsSrc), stepsMod, stepsDecls(true)), []string{"has a sorry"}},
		{"not built", []string{"one", "two"}, LeanSteps([]byte(stepsSrc), stepsMod, nil), []string{"one (AI.T.Thm line 3) is in no declaration", "two (AI.T.Thm line 8) is in no declaration"}},
	}
	for _, tc := range tests {
		fs := CheckSteps(n, tc.page, tc.lean)
		if len(fs) != len(tc.want) {
			t.Errorf("%s: %d findings, want %d: %v", tc.name, len(fs), len(tc.want), fs)
			continue
		}
		for i, f := range fs {
			if f.Rule != "F08" || f.Sev() != "level" || !strings.Contains(f.Msg, tc.want[i]) {
				t.Errorf("%s: %s, want F08 with %q", tc.name, f, tc.want[i])
			}
		}
	}
}

func TestPageSteps(t *testing.T) {
	dir := t.TempDir()
	p := dir + "/proof.md"
	if err := writeFile(p, "1. First. {#step-one}\n2. Then. {#step-two}\n\nSee eq-x {#eq-x}.\n"); err != nil {
		t.Fatal(err)
	}
	got, err := PageSteps(p)
	if err != nil || !slices.Equal(got, []string{"one", "two"}) {
		t.Errorf("PageSteps = %v, %v", got, err)
	}
}

func writeFile(path, s string) error { return os.WriteFile(path, []byte(s), 0o644) }
