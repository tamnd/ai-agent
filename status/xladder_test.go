// SPDX-License-Identifier: Apache-2.0

package status

import (
	"strings"
	"testing"

	"github.com/tamnd/ai-agent/node"
)

func TestXLadder(t *testing.T) {
	ref := node.Impl{Lang: "numpy", Files: []node.File{{Name: "dropout.py", Content: "def forward(x):\n    pass\n\n\ndef backward(dy):\n    pass\n"}, {Name: "test_dropout.py", Test: true}}}
	torch := node.Impl{Lang: "pytorch", Files: []node.File{{Name: "dropout.py"}, {Name: "test_dropout.py", Test: true}}}
	n := &node.Node{Impls: []node.Impl{ref, torch}}
	n.Slug, n.Kind = "dropout", "method"
	clean := []Lint{{Tool: "ruff format", Pass: true}, {Tool: "ruff check", Pass: true}, {Tool: "pyright", Pass: true}}
	tests := []struct {
		name  string
		im    node.Impl
		test  Test
		level int
		next  string
	}{
		{"fails", ref, Test{Failed: []string{"test_mask"}}, 0, "the native tests fail (X03): test_mask"},
		{"no gradient", ref, Test{Pass: true, Passed: []string{"test_mask"}}, 0, "a passing test_gradient"},
		{"no fixture", ref, Test{Pass: true, Passed: []string{"test_gradient"}}, 1, "X2 needs a passing test_shared_fixture"},
		{"suffix", ref, Test{Pass: true, Passed: []string{"test_gradient_input", "test_shared_fixture"}}, 2, "X3 needs ruff format did not run"},
		{"a prefix is not a suffix", ref, Test{Pass: true, Passed: []string{"test_gradients", "test_shared_fixture"}}, 0, "a passing test_gradient"},
		{"one run fails", torch, Test{Pass: true, Passed: []string{"test_gradient", "test_shared_fixture"}, Failed: []string{"test_gradient"}}, 0, "test_gradient"},
		{"torch gradient", torch, Test{Pass: true, Passed: []string{"test_shared_fixture"}}, 1, "X2 needs a passing test_gradient"},
		{"lint", torch, Test{Pass: true, Passed: []string{"test_gradient", "test_shared_fixture"}, Lint: []Lint{{Tool: "ruff format", Pass: true}, {Tool: "ruff check", Output: "dropout.py:9:21: F401 unused"}, {Tool: "pyright", Missing: true}}, Mutation: &Mutation{Mutants: 10, Killed: 10}}, 2, "X3 needs ruff check is not clean (X13): dropout.py:9:21"},
		{"score", torch, Test{Pass: true, Passed: []string{"test_gradient", "test_shared_fixture"}, Lint: clean, Mutation: &Mutation{Mutants: 22, Killed: 17, Survivors: []string{"dropout.py:27: relational >= -> >"}}}, 2, "kill 17 of 22 mutants (77%), and X3 needs 80% (X06); the first survivor is dropout.py:27"},
		{"x3", torch, Test{Pass: true, Passed: []string{"test_gradient", "test_shared_fixture"}, Lint: clean, Mutation: &Mutation{Mutants: 10, Killed: 8}}, 3, "X4 needs a review"},
	}
	for _, tc := range tests {
		env := Env{Results: map[string]map[string]Test{"dropout": {tc.im.Lang: tc.test}}}
		r := xImpl(n, tc.im, env)
		if r.Level != tc.level || r.Label != "X"+string(rune('0'+tc.level)) {
			t.Errorf("%s: %s, want X%d (next %v)", tc.name, r.Label, tc.level, r.Next)
		}
		if len(r.Next) == 0 || !strings.Contains(r.Next[0], tc.next) {
			t.Errorf("%s: next %q, want %q", tc.name, r.Next, tc.next)
		}
	}

	// A framework version with no NumPy reference does not start the ladder.
	alone := &node.Node{Impls: []node.Impl{torch}}
	alone.Slug, alone.Kind = "dropout", "method"
	if r := xImpl(alone, torch, Env{}); r.Level != 0 || !strings.Contains(r.Next[0], "(X01)") {
		t.Errorf("no reference: %+v", r)
	}
}

func TestXLadderKinds(t *testing.T) {
	arch := &node.Node{}
	arch.Kind = "architecture"
	_, x2 := xTests(arch, node.Impl{Lang: "jax"})
	var names []string
	for _, x := range x2 {
		names = append(names, x.name)
	}
	if got := strings.Join(names, " "); got != "test_shared_fixture test_gradient test_shapes test_param_count test_learns" {
		t.Errorf("architecture: %s", got)
	}
	names = nil
	_, x2 = xTests(arch, node.Impl{Lang: "numpy", Files: []node.File{{Name: "transformer.py", Content: "def forward(x):\n    pass\n"}}})
	for _, x := range x2 {
		names = append(names, x.name)
	}
	if got := strings.Join(names, " "); got != "test_shared_fixture test_shapes test_param_count" {
		t.Errorf("an architecture with no backward in NumPy: %s", got)
	}

	m := &node.Node{}
	m.Slug, m.Kind = "bleu", "metric"
	im := node.Impl{Lang: "numpy", Files: []node.File{{Name: "bleu.py"}, {Name: "test_bleu.py", Test: true}}}
	m.Impls = []node.Impl{im}
	for _, tc := range []struct {
		v     *Vectors
		level int
		next  string
	}{
		{nil, 1, "the test vectors did not run in this build (X05)"},
		{&Vectors{Cases: 4, Passed: 3, Failures: []CaseFailure{{Case: "official.jsonl:exp", Reason: "score: got 1, want 2"}}}, 1, "1 of 4 test vectors fail (X05), the first is official.jsonl:exp"},
		{&Vectors{Cases: 4, Passed: 4}, 2, "X3 needs"},
	} {
		r := xImpl(m, im, Env{Results: map[string]map[string]Test{"bleu": {"numpy": {Pass: true, Vectors: tc.v}}}})
		if r.Level != tc.level || !strings.Contains(r.Next[0], tc.next) {
			t.Errorf("metric %+v: X%d %q", tc.v, r.Level, r.Next)
		}
	}
}
