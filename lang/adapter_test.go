// SPDX-License-Identifier: Apache-2.0

package lang

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/ai-agent/contract"
	"github.com/tamnd/ai-agent/node"
)

const testContract = `problem: sorting-problem
entry: sort
params:
  - {name: xs, type: "list<T>", mode: inout}
returns: {type: unit}
generic: {T: ordered}
checker: exact
`

const testCases = `{"name":"example","args":{"xs":[5,2,4,6,1,3]},"expect":[1,2,3,4,5,6]}
{"name":"duplicates","args":{"xs":[3,1,3,2]},"expect":[1,2,3,3]}
{"name":"words","T":"string","args":{"xs":["pear","fig","apple"]},"expect":["apple","fig","pear"]}
{"name":"extremes","args":{"xs":["9223372036854775807",0,"-9223372036854775808"]},"expect":["-9223372036854775808",0,"9223372036854775807"]}
`

func loadTestContract(t *testing.T) (*contract.Contract, []contract.Case) {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "cases"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "contract.yaml"), []byte(testContract), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cases", "basic.jsonl"), []byte(testCases), 0o644); err != nil {
		t.Fatal(err)
	}
	k, cases, err := contract.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	return k, cases
}

func pyImpl(src string) node.Impl {
	return node.Impl{Lang: "python", Files: []node.File{{Name: "insertion_sort.py", Content: src}}}
}

const goodSort = `def insertion_sort(a):
    for i in range(1, len(a)):
        x, j = a[i], i
        while j > 0 and x < a[j - 1]:
            a[j] = a[j - 1]
            j -= 1
        a[j] = x
`

func TestSharedPython(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("no python3")
	}
	k, cases := loadTestContract(t)
	n := &node.Node{}
	n.Slug = "insertion-sort"
	r := &Runner{}

	s := r.Shared(n, pyImpl(goodSort), k, cases)
	if !s.OK() || s.Passed != 4 || s.Skipped != 0 {
		t.Fatalf("a right sort: %+v", s)
	}

	// The loop stops one item early, so the last item stays in its place. In each case the last item is not the largest, so each case fails.
	wrong := strings.Replace(goodSort, "range(1, len(a))", "range(1, len(a) - 1)", 1)
	s = r.Shared(n, pyImpl(wrong), k, cases)
	if s.OK() || len(s.Failures) != 4 || s.Passed != 0 {
		t.Fatalf("a wrong sort: %+v", s)
	}
	f := s.Failures[0]
	if f.Case != "basic/example" || f.Got != "[1,2,4,5,6,3]" || f.Want != "[1,2,3,4,5,6]" {
		t.Errorf("the first failure: %+v", f)
	}

	// A crash fails one case, and the next case gets a new adapter.
	crash := goodSort + "    if len(a) == 4:\n        import os; os._exit(3)\n"
	s = r.Shared(n, pyImpl(crash), k, cases)
	if len(s.Failures) != 1 || s.Failures[0].Case != "basic/duplicates" || !strings.Contains(s.Failures[0].Reason, "crashed") || s.Passed != 3 {
		t.Fatalf("a crash: %+v", s)
	}

	// An exception is an error of the call.
	raise := goodSort + "    if len(a) == 4:\n        raise IndexError('boom')\n"
	s = r.Shared(n, pyImpl(raise), k, cases)
	if len(s.Failures) != 1 || !strings.Contains(s.Failures[0].Reason, "IndexError: boom") {
		t.Fatalf("an exception: %+v", s)
	}

	// A flaky case counts as a failure, also when it passes (S09).
	flaky := append([]contract.Case(nil), cases...)
	flaky[1].Flaky, flaky[1].Issue = true, "#7"
	s = r.Shared(n, pyImpl(goodSort), k, flaky)
	if s.OK() || s.Passed != 3 || len(s.Failures) != 1 || s.Failures[0].Case != "basic/duplicates" || !strings.Contains(s.Failures[0].Reason, "marked flaky (#7)") {
		t.Fatalf("a flaky case: %+v", s)
	}
}

func TestSharedNoBuild(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go")
	}
	k, cases := loadTestContract(t)
	n := &node.Node{}
	n.Slug = "insertion-sort"
	// The entry takes []int64 only, so the string variant does not build. The int cases still run.
	im := node.Impl{Lang: "go", Files: []node.File{{Name: "insertion_sort.go", Content: "package insertionsort\n\nimport \"slices\"\n\nfunc Sort(a []int64) { slices.Sort(a) }\n"}}}
	s := (&Runner{}).Shared(n, im, k, cases)
	if !s.OK() || s.Passed != 3 || s.Skipped != 1 || !strings.Contains(s.SkipReason, "T = string") {
		t.Fatalf("%+v", s)
	}
}

func TestNames(t *testing.T) {
	if snake("merge-sort") != "merge_sort" || camel("merge-sort") != "mergeSort" || pascal("lower_bound") != "LowerBound" || camel("sort") != "sort" {
		t.Error("the case rule is wrong")
	}
}
