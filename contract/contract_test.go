// SPDX-License-Identifier: Apache-2.0

package contract

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	for _, s := range []string{"int", "list<int>", "option<index>", "map<string, list<T>>", "tuple<int,string,bool>", "matrix<float>"} {
		if _, err := Parse(s); err != nil {
			t.Errorf("Parse(%q): %v", s, err)
		}
	}
	for _, s := range []string{"", "list", "list<int", "integer", "map<int>", "T<int>", "int>"} {
		if _, err := Parse(s); err == nil {
			t.Errorf("Parse(%q) has no error", s)
		}
	}
	ty, _ := Parse("list<T>")
	if got := ty.Subst("T", "string").String(); got != "list<string>" {
		t.Errorf("Subst = %s", got)
	}
}

func TestCheckType(t *testing.T) {
	cases := []struct {
		ty, v string
		ok    bool
	}{
		{"int", "3", true},
		{"int", `"9223372036854775807"`, true},
		{"int", `"9223372036854775808"`, false},
		{"int", "3.5", false},
		{"index", "-1", false},
		{"option<index>", "null", true},
		{"list<int>", "[1,2,3]", true},
		{"list<int>", `[1,"a"]`, false},
		{"list<string>", `["a",""]`, true},
		{"unit", "null", true},
		{"unit", "[]", false},
		{"tuple<int,bool>", "[1,true]", true},
		{"tuple<int,bool>", "[1]", false},
	}
	for _, c := range cases {
		ty, err := Parse(c.ty)
		if err != nil {
			t.Fatal(err)
		}
		v, err := decode([]byte(c.v))
		if err != nil {
			t.Fatal(err)
		}
		if err := ty.Check(v); (err == nil) != c.ok {
			t.Errorf("%s.Check(%s) = %v, want ok %v", c.ty, c.v, err, c.ok)
		}
	}
}

// write makes a problem directory with the files.
func write(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, text := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

const sortContract = `problem: sorting-problem
entry: sort
params:
  - {name: xs, type: "list<T>", mode: inout}
returns: {type: unit}
generic: {T: ordered}
checker: exact
`

func TestLoad(t *testing.T) {
	dir := write(t, map[string]string{
		"contract.yaml":     sortContract,
		"cases/basic.jsonl": `{"name":"two","args":{"xs":[2,1]},"expect":[1,2]}` + "\n\n" + `{"name":"words","T":"string","args":{"xs":["b","a"]},"expect":["a","b"]}` + "\n",
		"cases/edge.jsonl":  `{"name":"empty","class":["empty"],"args":{"xs":[]},"expect":[]}` + "\n",
	})
	k, cases, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) != 3 || cases[2].ID() != "edge/empty" || cases[1].Type() != "string" || cases[0].Line != 1 || cases[1].Line != 3 {
		t.Fatalf("cases = %+v", cases)
	}
	if k.Output() != 0 || k.OutputType("int").String() != "list<int>" {
		t.Errorf("Output = %d, OutputType = %s", k.Output(), k.OutputType("int"))
	}
}

func TestLoadErrors(t *testing.T) {
	cases := map[string]map[string]string{
		"has no entry":             {"contract.yaml": "problem: p\nparams: [{name: a, type: int}]\nreturns: {type: int}\nchecker: exact\n"},
		"field extra not found":    {"contract.yaml": sortContract + "extra: 1\n"},
		"not a checker kind":       {"contract.yaml": strings.Replace(sortContract, "exact", "close", 1)},
		"must have the mode":       {"contract.yaml": strings.Replace(sortContract, ", mode: inout", "", 1)},
		"no checker.py":            {"contract.yaml": strings.Replace(sortContract, "exact", "custom", 1)},
		"also in cases/basic":      {"contract.yaml": sortContract, "cases/basic.jsonl": `{"name":"a","args":{"xs":[]},"expect":[]}` + "\n", "cases/edge.jsonl": `{"name":"a","args":{"xs":[]},"expect":[]}` + "\n"},
		"is not an integer":        {"contract.yaml": sortContract, "cases/basic.jsonl": `{"name":"a","args":{"xs":[1.5]},"expect":[1.5]}` + "\n"},
		"needs expect":             {"contract.yaml": sortContract, "cases/basic.jsonl": `{"name":"a","args":{"xs":[1]}}` + "\n"},
		"no parameter":             {"contract.yaml": sortContract, "cases/basic.jsonl": `{"name":"a","args":{"xs":[1],"ys":[]},"expect":[1]}` + "\n"},
		"tested only with":         {"contract.yaml": sortContract, "cases/basic.jsonl": `{"name":"a","T":"float","args":{"xs":[1]},"expect":[1]}` + "\n"},
		`unknown field "expected"`: {"contract.yaml": sortContract, "cases/basic.jsonl": `{"name":"a","args":{"xs":[1]},"expected":[1]}` + "\n"},
	}
	for want, files := range cases {
		_, _, err := Load(write(t, files))
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: err = %v", want, err)
		}
	}
}

func verdict(t *testing.T, k *Contract, c Case, out string, custom *Custom) string {
	t.Helper()
	r, err := k.Verdict(c, json.RawMessage(out), custom)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestVerdict(t *testing.T) {
	k := &Contract{Params: []Param{{Name: "xs", Type: "list<T>", Mode: "inout"}}, Returns: Return{Type: "unit"}, Generic: map[string]string{"T": "ordered"}, Checker: Checker{Kind: "exact"}}
	c := Case{Name: "a", Expect: json.RawMessage(`[1,2,"9223372036854775807"]`)}
	if r := verdict(t, k, c, `[1,2,9223372036854775807]`, nil); r != "" {
		t.Errorf("a big integer as a number: %s", r)
	}
	if r := verdict(t, k, c, `[1,2,3]`, nil); r == "" {
		t.Error("a wrong output passes")
	}
	if r := verdict(t, k, c, `[1,2.0,"9223372036854775807"]`, nil); !strings.Contains(r, "type") {
		t.Errorf("a float where the type is int: %q", r)
	}

	k = &Contract{Params: []Param{{Name: "a", Type: "list<int>"}}, Returns: Return{Type: "option<index>"}, Checker: Checker{Kind: "exact"}}
	c = Case{Name: "b", Expect: json.RawMessage(`null`)}
	if r := verdict(t, k, c, `-1`, nil); r != "" {
		t.Errorf("-1 is none: %s", r)
	}

	k.Returns.Type = "list<int>"
	k.Checker.Kind = "unordered"
	c = Case{Name: "c", Expect: json.RawMessage(`[3,1,2]`)}
	if r := verdict(t, k, c, `[1,2,3]`, nil); r != "" {
		t.Errorf("unordered: %s", r)
	}
	k.Checker = Checker{Kind: "float", Abs: 1e-9}
	k.Returns.Type = "float"
	c = Case{Name: "d", Expect: json.RawMessage(`0.3`)}
	if r := verdict(t, k, c, `0.30000000000000004`, nil); r != "" {
		t.Errorf("float: %s", r)
	}
	if r := verdict(t, k, c, `0.31`, nil); r == "" {
		t.Error("float: a far value passes")
	}
}

func TestCustom(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("no python3")
	}
	dir := write(t, map[string]string{
		"contract.yaml": "problem: p\nentry: search\nparams: [{name: a, type: \"list<int>\"}, {name: x, type: int}]\nreturns: {type: \"option<index>\"}\nchecker: custom\n",
		"checker.py":    "def check(args, output, expected=None):\n    if output is None:\n        return 'x is in a' if args['x'] in args['a'] else None\n    return None if args['a'][output] == args['x'] else 'wrong index'\n",
	})
	k, _, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	cu, err := k.StartCustom(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cu.Close()
	c := Case{Name: "a", Args: map[string]json.RawMessage{"a": json.RawMessage(`[1,3,3]`), "x": json.RawMessage(`3`)}}
	for out, want := range map[string]string{"1": "", "2": "", "0": "wrong index", "null": "x is in a", "-1": "x is in a"} {
		if r := verdict(t, k, c, out, cu); r != want {
			t.Errorf("output %s: reason %q, want %q", out, r, want)
		}
	}
}

func TestOracle(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("no python3")
	}
	dir := write(t, map[string]string{
		"contract.yaml": sortContract,
		"reference.py":  "def sort(xs):\n    xs.sort()\n",
	})
	k, _, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if k.OracleFile() != "reference.py" || k.OutName() != "xs" {
		t.Fatalf("OracleFile = %q, OutName = %q", k.OracleFile(), k.OutName())
	}
	o, err := k.StartOracle(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()
	good := Case{Name: "two", Args: map[string]json.RawMessage{"xs": json.RawMessage(`[3,1,2]`)}, Expect: json.RawMessage(`[1,2,3]`)}
	if r, out, err := k.Agree(o, good, nil); err != nil || r != "" || string(out) != "[1, 2, 3]" {
		t.Errorf("Agree = %q, %s, %v", r, out, err)
	}
	wrong := good
	wrong.Expect = json.RawMessage(`[3,2,1]`)
	if r, _, err := k.Agree(o, wrong, nil); err != nil || r == "" {
		t.Errorf("a wrong expect agrees: %q, %v", r, err)
	}
	if _, err := o.Run(map[string]json.RawMessage{"ys": json.RawMessage(`[]`)}, "xs"); err == nil || !strings.Contains(err.Error(), "TypeError") {
		t.Errorf("a bad argument gives %v", err)
	}
	big := Case{Name: "big", Args: map[string]json.RawMessage{"xs": json.RawMessage(`["9223372036854775807",0,"-9223372036854775808"]`)}, Expect: json.RawMessage(`["-9223372036854775808",0,"9223372036854775807"]`)}
	if r, _, err := k.Agree(o, big, nil); err != nil || r != "" {
		t.Errorf("the integers in strings: %q, %v", r, err)
	}
	words := Case{Name: "words", T: "string", Args: map[string]json.RawMessage{"xs": json.RawMessage(`["b","10","a"]`)}, Expect: json.RawMessage(`["10","a","b"]`)}
	if r, _, err := k.Agree(o, words, nil); err != nil || r != "" {
		t.Errorf("the strings of digits: %q, %v", r, err)
	}
	empty, _, _ := Load(write(t, map[string]string{"contract.yaml": sortContract}))
	if _, err := empty.StartOracle(nil); !errors.Is(err, ErrNoOracle) {
		t.Errorf("no oracle gives %v", err)
	}
}
