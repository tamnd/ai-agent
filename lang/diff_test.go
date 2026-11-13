// SPDX-License-Identifier: Apache-2.0

package lang

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/tamnd/ai-agent/contract"
	"github.com/tamnd/ai-agent/node"
)

const diffContract = `problem: sorting-problem
entry: sort
params:
  - {name: xs, type: "list<T>", mode: inout}
returns: {type: unit}
generic: {T: ordered}
bounds:
  len(xs): [0, 1000]
  int: [-2^63, 2^63-1]
  string: {len: [0, 5], alphabet: "a-z"}
checker: exact
oracle: {reference: reference.py}
`

const diffGenerated = `size: len(xs)
classes:
  tiny: {range: [0, 3], count: 0, exhaustive: true}
  small: {range: [4, 40], count: 30}
generators:
  - {name: uniform, file: generators/uniform.py, params: {k: [3, 1000]}}
pre: generators/pre.py
seeds:
  pr: [1, 2]
`

const diffGen = `def generate(rng, n, T="int", k=1000):
    if T == "string":
        return {"xs": ["".join(rng.choice("abc") for _ in range(rng.randint(0, 3))) for _ in range(n)]}
    return {"xs": [rng.randint(-k, k) for _ in range(n)]}

def exhaustive(n, T="int", k=1000):
    if T == "string" or n > 2:
        return []
    import itertools
    return [{"xs": list(p)} for p in itertools.product([0, 1], repeat=n)]
`

const diffPre = `def pre(args):
    if len(args["xs"]) > 1000:
        return "too long"
    return None
`

const diffRef = `def sort(xs):
    xs.sort()
`

func loadDiffContract(t *testing.T) (*contract.Contract, *contract.Generated) {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"contract.yaml":         diffContract,
		"reference.py":          diffRef,
		"cases/basic.jsonl":     testCases,
		"cases/generated.yaml":  diffGenerated,
		"generators/uniform.py": diffGen,
		"generators/pre.py":     diffPre,
	}
	for name, body := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	k, _, err := contract.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	g, err := k.LoadGenerated()
	if err != nil {
		t.Fatal(err)
	}
	return k, g
}

func diffInputs(t *testing.T, k *contract.Contract, g *contract.Generated) *Inputs {
	t.Helper()
	gen, err := k.StartGenerator(nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(gen.Close)
	oracle, err := k.StartOracle(nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(oracle.Close)
	in, err := MakeInputs(k, g, gen, oracle, g.Plan(k, []string{"tiny", "small"}, g.Seeds.PR))
	if err != nil {
		t.Fatal(err)
	}
	return in
}

func TestDiffPlantedBug(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("no python3")
	}
	k, g := loadDiffContract(t)
	in := diffInputs(t, k, g)
	// The small class has 30 inputs for each seed, and the tiny class adds each list of 0 and 1 of size 0 to 2.
	if len(in.Cases) <= 2*30 {
		t.Fatalf("got %d generated cases, want more than %d", len(in.Cases), 2*30)
	}
	n := &node.Node{}
	n.Slug = "insertion-sort"
	r := &Runner{}

	ss, err := r.Open(n, pyImpl(goodSort), k)
	if err != nil {
		t.Fatal(err)
	}
	res, err := Diff(ss, in)
	ss.Close()
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Failures) != 0 || res.Passed != len(in.Cases) {
		t.Fatalf("a right sort: %+v", res)
	}

	// The planted bug: the sort drops each repeated item. The small inputs with k = 3 have repeated items, so the run finds the bug, and the shrinker gives an input of two equal items.
	bad := "def insertion_sort(a):\n    a[:] = sorted(set(a))\n"
	ss, err = r.Open(n, pyImpl(bad), k)
	if err != nil {
		t.Fatal(err)
	}
	res, err = Diff(ss, in)
	ss.Close()
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Failures) == 0 {
		t.Fatalf("the planted bug is not found: %+v", res)
	}
	for _, f := range res.Failures {
		var xs []any
		if err := json.Unmarshal(f.Shrunk.Args["xs"], &xs); err != nil {
			t.Fatal(err)
		}
		if len(xs) != 2 || xs[0] != xs[1] {
			t.Errorf("case %s shrinks to %s, want two equal items", f.Case.Name, f.Shrunk.Args["xs"])
		}
		if f.ShrunkReason == "" {
			t.Errorf("the shrunk case of %s passes", f.Case.Name)
		}
		if f.Calls == 0 || f.Calls > ShrinkBudget {
			t.Errorf("the shrinker made %d calls", f.Calls)
		}
	}
	reg := Regression(k, res.Failures[0], "insertion-sort/python", time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC))
	if reg.Found != "insertion-sort/python" || len(reg.Expect) == 0 || len(reg.Name) != len("regress-2026-11-03-")+7 {
		t.Fatalf("regression case: %+v", reg)
	}
	if err := k.CheckCase(reg); err != nil {
		t.Fatalf("the regression case does not meet the contract: %v", err)
	}
}
