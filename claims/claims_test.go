// SPDX-License-Identifier: Apache-2.0

package claims

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// env is a fake Env. Each source is grounded unless it is in ungrounded. Two works are independent unless they share the text before the first dot.
type env struct {
	standing   map[string]string
	ungrounded map[string]bool
	proved     bool
	passed     bool
	failed     bool
}

func (e env) Grounded(_ *Claim, ev Evidence) bool { return !e.ungrounded[ev.Work] }
func (e env) Standing(w string) string {
	if s, ok := e.standing[w]; ok {
		return s
	}
	return PeerReviewed
}
func (e env) Independent(a, b string) bool {
	return strings.Split(a, ".")[0] != strings.Split(b, ".")[0]
}
func (e env) Proved(*Claim) bool           { return e.proved }
func (e env) Measured(*Claim) (bool, bool) { return e.passed, e.failed }

func ev(work, role, stance string) Evidence {
	return Evidence{Work: work, Role: role, Stance: stance, Locator: "p. 1"}
}

func TestCompute(t *testing.T) {
	for _, c := range []struct {
		name     string
		claim    Claim
		env      env
		labels   []string
		headline string
	}{
		{"one source", Claim{Type: Empirical, Evidence: []Evidence{ev("a", "original", "for")}}, env{}, []string{Reported}, Reported},
		{"two independent", Claim{Type: Empirical, Evidence: []Evidence{ev("a", "original", "for"), ev("b", "replication", "for")}}, env{}, []string{Replicated}, Replicated},
		{"two related", Claim{Type: Empirical, Evidence: []Evidence{ev("a.1", "original", "for"), ev("a.2", "replication", "for")}}, env{}, []string{Reported}, Reported},
		{"against", Claim{Type: Empirical, Evidence: []Evidence{ev("a", "original", "for"), ev("b", "replication", "against")}}, env{}, []string{Contested, Reported}, Contested},
		{"a correction is not a dispute", Claim{Type: Empirical, Evidence: []Evidence{ev("a.1", "original", "for"), ev("a.2", "original", "against")}}, env{}, []string{Reported}, Reported},
		{"measured here", Claim{Type: Empirical, Evidence: []Evidence{ev("a", "original", "for")}}, env{passed: true}, []string{MeasuredHere}, MeasuredHere},
		{"failed here", Claim{Type: Empirical, Evidence: []Evidence{ev("a", "original", "for"), ev("b", "replication", "for")}}, env{failed: true}, []string{Contested, Replicated}, Contested},
		{"self-reported", Claim{Type: Empirical, SelfReported: true, Evidence: []Evidence{ev("a", "original", "for"), ev("b", "original", "for")}}, env{standing: map[string]string{"a": Developer, "b": Developer}, passed: true}, []string{Reported}, Reported},
		{"retracted", Claim{Type: Empirical, Evidence: []Evidence{ev("a", "original", "for"), ev("b", "replication", "for")}}, env{standing: map[string]string{"b": Retracted}}, []string{Reported}, Reported},
		{"community", Claim{Type: Empirical, Evidence: []Evidence{ev("a", "original", "for")}}, env{standing: map[string]string{"a": Community}}, []string{}, ""},
		{"community for an attributive claim", Claim{Type: Attributive, Evidence: []Evidence{ev("a", "statement", "for")}}, env{standing: map[string]string{"a": Community}}, []string{Reported}, Reported},
		{"a review counts once", Claim{Type: Empirical, Evidence: []Evidence{ev("a", "original", "for"), ev("b", "review", "for")}}, env{standing: map[string]string{"b": SecondaryReference}}, []string{Reported}, Reported},
		{"not grounded", Claim{Type: Empirical, Evidence: []Evidence{ev("a", "original", "for")}}, env{ungrounded: map[string]bool{"a": true}}, []string{}, ""},
		{"proved", Claim{Type: Mathematical, Evidence: []Evidence{ev("a", "original", "for")}}, env{proved: true}, []string{Proved}, Proved},
		{"proved overrides contested", Claim{Type: Mathematical, Evidence: []Evidence{ev("a", "original", "for"), ev("b", "original", "against")}}, env{proved: true}, []string{Proved}, Proved},
		{"history with a primary document", Claim{Type: Historical, Evidence: []Evidence{ev("a.1", "primary-document", "for"), ev("a.2", "review", "for")}}, env{}, []string{Replicated}, Replicated},
		{"law", Claim{Type: NormativeReported, Evidence: []Evidence{ev("eu.1", "official-text", "for"), ev("eu.2", "official-guidance", "for")}}, env{standing: map[string]string{"eu.1": Official, "eu.2": Official}}, []string{Replicated}, Replicated},
		{"definition", Claim{Type: Definitional, Evidence: []Evidence{ev("a", "original", "for")}}, env{}, []string{}, ""},
	} {
		cl := c.claim
		Compute(&cl, c.env)
		if !slices.Equal(cl.Labels, c.labels) || cl.Headline != c.headline {
			t.Errorf("%s: labels %q and headline %q, want %q and %q", c.name, cl.Labels, cl.Headline, c.labels, c.headline)
		}
	}
}

func TestNoChecks(t *testing.T) {
	c := Claim{Type: Empirical, Evidence: []Evidence{ev("a", "original", "for"), ev("b", "replication", "for")}}
	Compute(&c, NoChecks{})
	if len(c.Labels) != 0 || c.Headline != "" || c.IndependentFor != 0 {
		t.Errorf("with no checks, the claim has the labels %q", c.Labels)
	}
}

func TestCounts(t *testing.T) {
	c := Claim{Type: Empirical, Evidence: []Evidence{ev("a", "original", "for"), ev("b", "replication", "for"), ev("b.2", "replication", "for"), ev("c", "replication", "against")}}
	Compute(&c, env{})
	if c.IndependentFor != 2 || c.Against != 1 {
		t.Errorf("for %d and against %d, want 2 and 1", c.IndependentFor, c.Against)
	}
}

func TestRead(t *testing.T) {
	dir := t.TempDir()
	write := func(s string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, File), []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("- id: c-one\n  type: empirical\n  text: A claim.\n  as_of: 2026-11-13\n  headline: replicated\n  evidence:\n    - {work: a, role: original, stance: for, locator: \"p. 1\"}\n")
	cs, err := Read(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 1 || cs[0].AsOf != "2026-11-13" || !slices.Equal(cs[0].Written, []string{"headline"}) || cs[0].Evidence[0].Locator != "p. 1" {
		t.Errorf("Read gives %+v", cs)
	}
	write("- id: c-one\n  type: empirical\n  text: A claim.\n  colour: red\n")
	if _, err := Read(dir); err == nil || !strings.Contains(err.Error(), `"colour"`) {
		t.Errorf("an unknown field gives the error %v", err)
	}
	if cs, err := Read(t.TempDir()); cs != nil || err != nil {
		t.Errorf("a node with no claim file gives %v and %v", cs, err)
	}
}

func TestScan(t *testing.T) {
	body := "Dropout reduces overfitting {#c-reduces}.\n\n```\nx {#c-in-code}\n```\n\nSee `{#c-inline}` here.\n\n::claim{ref=dropout#c-reduces}\n\nA {#c-a} B {#c-b} C {#c-c}\n"
	anchors, refs, runs := Scan(body)
	var ids []string
	for _, a := range anchors {
		ids = append(ids, a.ID)
	}
	if !slices.Equal(ids, []string{"c-reduces", "c-a", "c-b", "c-c"}) {
		t.Errorf("anchors %q", ids)
	}
	if anchors[0].Line != 1 {
		t.Errorf("the first anchor is on line %d, want 1", anchors[0].Line)
	}
	if len(refs) != 1 || refs[0].Slug != "dropout" || refs[0].ID != "c-reduces" {
		t.Errorf("refs %+v", refs)
	}
	if len(runs) != 0 {
		t.Errorf("runs %v; the anchors of the last line are apart", runs)
	}
	if _, _, runs := Scan("One {#c-a} {#c-b} {#c-c}.\n"); !slices.Equal(runs, []int{1}) {
		t.Errorf("runs %v, want [1]", runs)
	}
}

func TestValidID(t *testing.T) {
	for id, want := range map[string]bool{"c-reduces-overfitting": true, "c-1": true, "reduces": false, "c-": false, "c-Bad": false, "c--a": false} {
		if ValidID(id) != want {
			t.Errorf("ValidID(%q) = %v", id, !want)
		}
	}
}
