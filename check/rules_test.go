// SPDX-License-Identifier: Apache-2.0

package check

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/tamnd/ai-agent/contract"
	"github.com/tamnd/ai-agent/formal"
	"github.com/tamnd/ai-agent/node"
)

// fixture copies the test corpus, writes the files, and loads it.
func fixture(t *testing.T, files map[string]string) *node.Corpus {
	t.Helper()
	root := filepath.Join(t.TempDir(), "corpus")
	if err := os.CopyFS(root, os.DirFS("../testdata/corpus")); err != nil {
		t.Fatal(err)
	}
	for p, s := range files {
		full := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	c, err := node.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// fired returns the findings of the rule, as path: message.
func fired(fs []Finding, rule string) []string {
	var out []string
	for _, f := range fs {
		if f.Rule == rule {
			out = append(out, f.Path+": "+string(f.Severity)+": "+f.Msg)
		}
	}
	return out
}

const problemDir = "nodes/searching/sequential-and-binary-search/search-in-sorted-array/"

const searchContract = `problem: search-in-sorted-array
entry: search
params:
  - {name: a, type: "list<int>"}
  - {name: x, type: int}
returns: {type: int}
bounds:
  len(a): [0, 1000]
checker: custom
edge_classes: [empty, missing]
`

func TestSRules(t *testing.T) {
	c := fixture(t, map[string]string{
		problemDir + "contract.yaml":         searchContract,
		problemDir + "checker.py":            "def check(args, output, expected):\n    return None\n",
		problemDir + "cases/edge.jsonl":      `{"name":"e1","class":["empty","odd"],"args":{"a":[],"x":1},"flaky":true}` + "\n",
		problemDir + "cases/generated.yaml":  "generators:\n  - {name: uniform, file: generators/uniform.py}\n  - {name: uniform, file: ../x.py}\n",
		problemDir + "generators/uniform.py": "import random\n\ndef generate(rng, n):\n    return {\"a\": sorted(random.sample(range(10**6), n)), \"x\": 1}\n",
	})
	p := c.BySlug["search-in-sorted-array"]
	p.Level, p.Tag = "core", "ABCDEF"
	bs := c.BySlug["binary-search"]
	bs.Complexity.Time["worst"] = "Θ(log m)"
	fs := Run(c, Options{})
	for rule, want := range map[string][]string{
		"S01": {`the claim "Θ(log m)" uses "m"`},
		"S03": {"needs checker_cases.jsonl"},
		"S04": {`no case has the edge class "missing"`, `the class "odd" is not in edge_classes`},
		"S05": {`the generator name "uniform" is empty or used twice`, `the file "../x.py"`, `uses "import random"`},
		"S06": {"needs a review by a person: no review record reviews/ABCDEF/contract.yaml"},
		"S09": {"the flaky case e1 needs an issue"},
	} {
		got := fired(fs, rule)
		for _, w := range want {
			if !slices.ContainsFunc(got, func(s string) bool { return strings.Contains(s, w) }) {
				t.Errorf("%s does not report %q; it reports %q", rule, w, got)
			}
		}
	}
	if got := fired(fs, "S04"); len(got) > 0 && !strings.Contains(got[0], ": level: ") {
		t.Errorf("S04 is not a level rule: %q", got)
	}
}

func TestS02AndS03Cases(t *testing.T) {
	c := fixture(t, map[string]string{
		problemDir + "contract.yaml":       searchContract,
		problemDir + "checker.py":          "",
		problemDir + "cases/basic.jsonl":   `{"name":"b1","args":{"a":"no","x":1}}` + "\n",
		problemDir + "checker_cases.jsonl": `{"name":"ok","args":{"a":[1],"x":1},"output":0,"valid":true}` + "\n" + `{"name":"bad","args":{"a":[1],"x":1},"output":5,"valid":false}` + "\n",
	})
	fs := Run(c, Options{})
	if got := fired(fs, "S02"); len(got) != 1 || !strings.Contains(got[0], "cases/basic.jsonl:1") {
		t.Errorf("S02 = %q", got)
	}
	c = fixture(t, map[string]string{
		problemDir + "contract.yaml":       searchContract,
		problemDir + "checker.py":          "",
		problemDir + "checker_cases.jsonl": `{"name":"ok","args":{"a":[1],"x":1},"output":0,"valid":true}` + "\n" + `{"name":"bad","args":{"a":[1],"x":1},"output":5,"valid":false}` + "\n",
	})
	if got := fired(Run(c, Options{}), "S03"); len(got) != 1 || !strings.Contains(got[0], "the invalid output bad needs a note") {
		t.Errorf("S03 = %q", got)
	}
	c = fixture(t, map[string]string{problemDir + "contract.yaml": strings.Replace(searchContract, "checker: custom", "checker: fuzzy", 1)})
	if got := fired(Run(c, Options{}), "S01"); len(got) != 1 || !strings.Contains(got[0], `"fuzzy" is not a checker kind`) {
		t.Errorf("S01 = %q", got)
	}
}

func TestS03Run(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("no python3")
	}
	// The checker accepts an index of x, and rejects all else. The case "wrong" says that 1 is valid, but a[1] is not x.
	checker := "def check(args, output, expected=None):\n" +
		"    a, x = args['a'], args['x']\n" +
		"    if isinstance(output, int) and not isinstance(output, bool) and 0 <= output < len(a) and a[output] == x:\n" +
		"        return None\n" +
		"    return 'not an index of x'\n"
	c := fixture(t, map[string]string{
		problemDir + "contract.yaml": searchContract,
		problemDir + "checker.py":    checker,
		problemDir + "checker_cases.jsonl": `{"name":"ok","args":{"a":[1,2],"x":1},"output":0,"valid":true}` + "\n" +
			`{"name":"wrong","args":{"a":[1,2],"x":1},"output":1,"valid":true}` + "\n" +
			`{"name":"past","args":{"a":[1,2],"x":1},"output":2,"valid":false,"note":"past the end"}` + "\n" +
			`{"name":"lax","args":{"a":[1,2],"x":1},"output":0,"valid":false,"note":"the checker must reject it, but it is right"}` + "\n",
	})
	run := func(k *contract.Contract) (*contract.Custom, error) { return k.StartCustom(nil) }
	got := fired(Run(c, Options{Rule: "S03", Checker: run}), "S03")
	want := []string{"checker_cases.jsonl: block: the checker rejects the valid output wrong: not an index of x", "checker_cases.jsonl: block: the checker accepts the invalid output lax"}
	if len(got) != len(want) {
		t.Fatalf("S03 = %q", got)
	}
	for i, w := range want {
		if !strings.Contains(got[i], w) {
			t.Errorf("S03[%d] = %q, want %q", i, got[i], w)
		}
	}
	if got := fired(Run(c, Options{Rule: "S03"}), "S03"); len(got) != 0 {
		t.Errorf("without Checker, S03 = %q", got)
	}
}

func TestLRules(t *testing.T) {
	c := fixture(t, map[string]string{"lean/Algo/Search/BinarySearch.lean": "module\n"})
	n := c.BySlug["binary-search"]
	n.Level, n.Tag = "core", "ABCDEF"
	n.Lean = map[string]string{"module": "Algo.Search.BinarySearch", "model": "Algo.Search.BinarySearch.search", "spec": "Algo.Search.Spec", "correct": "Algo.Search.BinarySearch.ok", "cost": "Algo.Search.BinarySearch.cost"}
	ds := formal.Decls{
		"Algo.Search.BinarySearch.search": {Decl: "Algo.Search.BinarySearch.search", Kind: "def", Module: "Algo.Search.BinarySearch"},
		"Algo.Search.Spec":                {Decl: "Algo.Search.Spec", Kind: "def", Module: "Algo.Search"},
		"Algo.Search.BinarySearch.ok":     {Decl: "Algo.Search.BinarySearch.ok", Kind: "theorem", Module: "Algo.Search.BinarySearch", Type: "True", Axioms: []string{"sorryAx"}, Sorry: true, Pos: &formal.Pos{Line: 3, EndLine: 4}},
	}
	fs := Run(c, Options{Decls: ds})
	for rule, want := range map[string]string{
		"L01": "lean.cost: Algo.Search.BinarySearch.cost is not a declaration",
		"L02": "does not mention the spec Algo.Search.Spec",
		"L03": "lean/Algo/Search/BinarySearch.lean: block: Algo.Search.BinarySearch.ok uses the axiom sorryAx (a sorry)",
		"L04": "needs a fidelity review by a person: no review record reviews/ABCDEF/lean-spec.yaml",
		"L05": "time.worst: Θ(log n) has no class lemma in Algo.Search.BinarySearch",
		"L07": "Algo.Search.BinarySearch.search is not computable",
		"L09": "Algo.Search.BinarySearch does not mirror the node path",
	} {
		got := fired(fs, rule)
		if !slices.ContainsFunc(got, func(s string) bool { return strings.Contains(s, want) }) {
			t.Errorf("%s does not report %q; it reports %q", rule, want, got)
		}
	}
	n.Lean["module"] = "Algo.Search.Missing"
	if got := fired(Run(c, Options{}), "L01"); len(got) != 1 || !strings.Contains(got[0], "lean/Algo/Search/Missing.lean does not exist") {
		t.Errorf("L01 for a missing module = %q", got)
	}
}

func TestS08(t *testing.T) {
	const ignore = "nodes/searching/sequential-and-binary-search/binary-search/impl/python/mutants.ignore"
	tests := []struct {
		name, text, want string
	}{
		{"none", "", ""},
		{"no reason", "binary_search.py:9: relational < -> <=\n", "the reason why it cannot change the output"},
		{"stale", "binary_search.py:99: relational < -> <= # never equal\n", "names no mutant of the sample"},
		{"one", "binary_search.py:19: constant 2 -> 3 # a third of the range also halves the search\n", ""},
		{"too many", "binary_search.py:19: constant 2 -> 3 # a third also works\nbinary_search.py:19: constant 2 -> 1 # x\n", "the list has 2 of 21 mutants"},
	}
	for _, tc := range tests {
		files := map[string]string{}
		if tc.text != "" {
			files[ignore] = "# The equivalent mutants.\n" + tc.text
		}
		got := fired(Run(fixture(t, files), Options{}), "S08")
		switch {
		case tc.want == "" && len(got) > 0:
			t.Errorf("%s: %v, want no finding", tc.name, got)
		case tc.want != "" && (len(got) == 0 || !strings.Contains(got[0], tc.want)):
			t.Errorf("%s: %v, want a finding with %q", tc.name, got, tc.want)
		}
	}
}
