// SPDX-License-Identifier: Apache-2.0

package contract

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

const boundedSort = sortContract + `bounds:
  len(xs): [0, 1000]
`

const genYAML = `size: len(xs)
classes:
  tiny: {range: [0, 2], count: 0, exhaustive: true}
  small: {range: [3, 40], count: 10}
  medium: {range: [41, 500], count: 4}
generators:
  - {name: uniform, file: generators/uniform.py, params: {k: [3, 1000]}}
  - {name: killer, file: generators/killer.py, adversarial: true}
seeds:
  pr: [1, 2]
`

func loadGen(t *testing.T, yml string) (*Contract, *Generated, error) {
	t.Helper()
	dir := write(t, map[string]string{
		"contract.yaml":         boundedSort,
		"cases/generated.yaml":  yml,
		"generators/uniform.py": "def generate(rng, n, T='int', k=10):\n    return {'xs': [rng.randint(0, k) for _ in range(n)]}\n",
		"generators/killer.py":  "def generate(rng, n, T='int'):\n    return {'xs': list(range(n, 0, -1))}\n",
	})
	k, _, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	g, err := k.LoadGenerated()
	return k, g, err
}

func TestPlan(t *testing.T) {
	k, g, err := loadGen(t, genYAML)
	if err != nil {
		t.Fatal(err)
	}
	classes := []string{"tiny", "small", "medium"}
	a := g.Plan(k, classes, g.Seeds.PR)
	if !reflect.DeepEqual(a, g.Plan(k, classes, g.Seeds.PR)) {
		t.Fatal("two plans with the same seeds differ")
	}
	count := map[string]int{}
	for _, q := range a {
		count[q.Class]++
		c := g.Classes[q.Class]
		if q.N < c.Range[0] || q.N > c.Range[1] {
			t.Errorf("%s: the size %d is not in %v", q.CaseName(0), q.N, c.Range)
		}
		if q.Gen == "killer" && q.Class != "small" {
			t.Errorf("the adversarial generator runs at %s", q.Class)
		}
		if q.All != (q.Class == "tiny") {
			t.Errorf("%s: all = %v", q.CaseName(0), q.All)
		}
	}
	// tiny: 3 sizes and 2 parameter combinations for each seed; small: 10 for each seed; medium: 4 for each seed.
	if want := map[string]int{"tiny": 12, "small": 20, "medium": 8}; !reflect.DeepEqual(count, want) {
		t.Errorf("the plan has %v requests, want %v", count, want)
	}
	if b := g.Plan(k, classes, []int{3}); reflect.DeepEqual(a[12:], b) {
		t.Error("another seed gives the same plan")
	}
	if got := (GenRequest{Gen: "uniform", Params: map[string]any{"k": 3}, Class: "small", N: 7, Seed: 1, Index: 2, T: "string"}).CaseName(0); got != "small-uniform-k=3-string-n7-s1-2" {
		t.Errorf("CaseName = %s", got)
	}
}

func TestLoadGeneratedErrors(t *testing.T) {
	cases := map[string]string{
		"size":      strings.Replace(genYAML, "size: len(xs)", "size: len(ys)", 1),
		"class":     strings.Replace(genYAML, "medium:", "huge:", 1),
		"range":     strings.Replace(genYAML, "[41, 500]", "[500, 41]", 1),
		"file":      strings.Replace(genYAML, "killer.py", "missing.py", 1),
		"seeds":     strings.Replace(genYAML, "pr: [1, 2]", "pr: []", 1),
		"field":     genYAML + "extra: 1\n",
		"duplicate": strings.Replace(genYAML, "name: killer", "name: uniform", 1),
	}
	for name, yml := range cases {
		if _, _, err := loadGen(t, yml); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

func TestShrink(t *testing.T) {
	// The failure: the list has an item larger than 9. The smallest failing list is [10].
	fails := func(c Case) (bool, error) {
		var xs []json.Number
		if err := json.Unmarshal(c.Args["xs"], &xs); err != nil {
			return false, err
		}
		for _, x := range xs {
			if n, _ := x.Int64(); n > 9 {
				return true, nil
			}
		}
		return false, nil
	}
	c := Case{Name: "big", Args: map[string]json.RawMessage{"xs": json.RawMessage(`[3, -7, 512, 4, 0, 99, 1]`)}, Expect: json.RawMessage(`[]`)}
	got, calls, err := Shrink(c, fails, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if string(got.Args["xs"]) != "[10]" || got.Expect != nil || calls == 0 {
		t.Fatalf("Shrink = %s, expect %s, %d calls", got.Args["xs"], got.Expect, calls)
	}
	if _, calls, _ := Shrink(c, fails, 5); calls != 5 {
		t.Errorf("the budget of 5 gives %d calls", calls)
	}

	// A string shrinks to the shortest string that fails.
	str := Case{Args: map[string]json.RawMessage{"s": json.RawMessage(`"hello world"`)}}
	got, _, err = Shrink(str, func(c Case) (bool, error) { return strings.Contains(string(c.Args["s"]), "o"), nil }, 1000)
	if err != nil || string(got.Args["s"]) != `"o"` {
		t.Fatalf("Shrink of a string = %s, %v", got.Args["s"], err)
	}
}

func TestSafeCase(t *testing.T) {
	dir := write(t, map[string]string{"contract.yaml": sortContract})
	k, _, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	c := Case{Name: "extremes", Args: map[string]json.RawMessage{"xs": json.RawMessage(`[9223372036854775807, 1, -9007199254740993]`)}, Expect: json.RawMessage(`[-9007199254740993, 1, 9223372036854775807]`)}
	got, err := k.SafeCase(c)
	if err != nil {
		t.Fatal(err)
	}
	if string(got.Args["xs"]) != `["9223372036854775807",1,"-9007199254740993"]` || string(got.Expect) != `["-9007199254740993",1,"9223372036854775807"]` {
		t.Fatalf("SafeCase = %s, %s", got.Args["xs"], got.Expect)
	}
	if err := k.CheckCase(got); err != nil {
		t.Fatal(err)
	}
}
