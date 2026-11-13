// SPDX-License-Identifier: Apache-2.0

package status

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/ai-agent/node"
)

func TestImplLadder(t *testing.T) {
	n := &node.Node{Slug: "merge-sort"}
	im := node.Impl{Lang: "c", Files: []node.File{{Name: "merge_sort.c"}, {Name: "test_merge_sort.c", Test: true}}}
	tests := []struct {
		name  string
		test  Test
		level int
	}{
		{"all pass", Test{Pass: true, Shared: []Shared{{Problem: "sorting-problem", Cases: 18, Passed: 18}}}, 1},
		{"expected skip", Test{Pass: true, Shared: []Shared{{Problem: "sorting-problem", Cases: 18, Passed: 13, Skipped: 5, SkipExpected: true}}}, 1},
		{"skip", Test{Pass: true, Shared: []Shared{{Problem: "sorting-problem", Cases: 18, Passed: 13, Skipped: 5, SkipReason: "no build"}}}, 0},
		{"failure", Test{Pass: true, Shared: []Shared{{Problem: "sorting-problem", Cases: 18, Passed: 17, Failures: []CaseFailure{{Case: "basic/example"}}}}}, 0},
		{"native fail", Test{Pass: false, Shared: []Shared{{Problem: "sorting-problem", Cases: 18, Passed: 18}}}, 0},
		{"no shared", Test{Pass: true}, 0},
		{"diff and mutation", Test{Pass: true, Shared: []Shared{{Problem: "sorting-problem", Cases: 18, Passed: 18}}, Diff: []Diff{{Problem: "sorting-problem", Cases: 9, Passed: 9}}, Mutation: &Mutation{Mutants: 20, Killed: 18, Ignored: 1}}, 2},
		{"diff fails", Test{Pass: true, Shared: []Shared{{Problem: "sorting-problem", Cases: 18, Passed: 18}}, Diff: []Diff{{Problem: "sorting-problem", Cases: 9, Passed: 8, Failures: []CaseFailure{{Case: "x"}}}}, Mutation: &Mutation{Mutants: 20, Killed: 20}}, 1},
	}
	for _, tc := range tests {
		env := Env{Results: map[string]map[string]Test{"merge-sort": {"c": tc.test}}}
		r := impl(n, im, env)
		if r.Level != tc.level {
			t.Errorf("%s: level %d, want %d (next %v)", tc.name, r.Level, tc.level, r.Next)
		}
	}
}

func TestDiffNext(t *testing.T) {
	tests := []struct {
		name string
		test Test
		want string
	}{
		{"no run", Test{}, "did not run in this build"},
		{"error", Test{Diff: []Diff{{Problem: "sorting-problem", Error: "the adapter does not build\nmore"}}}, "did not run: the adapter does not build"},
		{"failure", Test{Diff: []Diff{{Problem: "sorting-problem", Cases: 9, Passed: 8, Failures: []CaseFailure{{Case: "small-uniform-n4-s1-0"}}}}}, "fails on small-uniform-n4-s1-0 (I03)"},
		{"no mutation", Test{Diff: []Diff{{Problem: "sorting-problem", Cases: 9, Passed: 9}}}, "the mutation run did not run"},
		{"low score", Test{Diff: []Diff{{Problem: "sorting-problem", Cases: 9, Passed: 9}}, Mutation: &Mutation{Mutants: 22, Killed: 17, Ignored: 1, Survivors: []string{"a.py:3: relational < -> <="}}}, "kill 17 of 21 mutants (81%)"},
		{"lower score", Test{Diff: []Diff{{Problem: "sorting-problem", Cases: 9, Passed: 9}}, Mutation: &Mutation{Mutants: 20, Killed: 16, Survivors: []string{"a.py:3: relational < -> <="}}}, "add a case that kills a.py:3"},
		{"mutation error", Test{Diff: []Diff{{Problem: "sorting-problem", Cases: 9, Passed: 9}}, Mutation: &Mutation{Error: "the code fails"}}, "did not run: the code fails"},
	}
	for _, tc := range tests {
		got := diffNext(tc.test)
		if got == nil {
			got = mutationNext(tc.test)
		}
		if len(got) != 1 || !strings.Contains(got[0], tc.want) {
			t.Errorf("%s: %q, want a message with %q", tc.name, got, tc.want)
		}
	}
}

func TestLoadResultsDiff(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, r Results) {
		b, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.json", Results{Lang: "go", Tests: []Test{{Slug: "merge-sort", Lang: "go", Pass: true}}})
	write("go.diff.json", Results{Lang: "go", Kind: "diff", Tests: []Test{
		{Slug: "merge-sort", Lang: "go", Diff: []Diff{{Problem: "sorting-problem", Cases: 5, Passed: 5}}},
		{Slug: "heap-sort", Lang: "go", Diff: []Diff{{Problem: "sorting-problem", Cases: 5, Passed: 5}}},
	}})
	res, err := LoadResults(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := res["merge-sort"]["go"]; !got.Pass || len(got.Diff) != 1 || got.Diff[0].Passed != 5 {
		t.Errorf("merge-sort/go = %+v, want the native result with the diff", got)
	}
	if _, ok := res["heap-sort"]; ok {
		t.Errorf("heap-sort has a result, but it has only a differential run")
	}
}
