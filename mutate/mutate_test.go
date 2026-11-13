// SPDX-License-Identifier: Apache-2.0

package mutate

import (
	"slices"
	"strings"
	"testing"
)

func ids(t *testing.T, lang, src string) []string {
	t.Helper()
	ms, err := Mutants(lang, "f", src)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, m := range ms {
		out = append(out, strings.TrimPrefix(m.ID(), "f:"))
		if got := m.Apply(src); got == src {
			t.Errorf("%s does not change the source", m.ID())
		}
	}
	return out
}

func TestOperators(t *testing.T) {
	tests := []struct {
		name, lang, src string
		want            []string
	}{
		{"relational", "go", "func f() {\n\tif a < b {\n\t}\n}\n", []string{"2: negate a < b -> !(a < b)", "2: relational < -> <="}},
		{"generic", "go", "var x List<int>\n", nil},
		{"tight arithmetic", "go", "x := (hi-lo)/2\n", []string{"1: arithmetic - -> +", "1: arithmetic / -> *", "1: constant 2 -> 3", "1: constant 2 -> 1"}},
		{"unary minus", "python", "x = -1\n", []string{"1: constant 1 -> 2", "1: constant 1 -> 0"}},
		{"comment and string", "python", "# a < b\ns = \"a < b\"\n", nil},
		{"floor division", "python", "m = a // 2\n", []string{"1: arithmetic // -> *", "1: constant 2 -> 3", "1: constant 2 -> 1"}},
		{"python condition", "python", "def f(a):\n    while lo < hi:\n        lo = 0\n", []string{"2: negate lo < hi -> not (lo < hi)", "2: relational < -> <=", "3: delete lo = 0 -> pass", "3: constant 0 -> 1"}},
		{"c for", "c", "void f(void) {\n    for (i = 0; i < n; i++) {\n        g(a, b);\n    }\n}\n", []string{"2: constant 0 -> 1", "2: negate i < n -> !(i < n)", "2: relational < -> <=", "2: arithmetic ++ -> --", "3: delete g(a, b); -> ;", "3: swap a, b -> b, a"}},
		{"definition", "java", "static int f(int a, int b) {\n}\n", nil},
		{"commutative", "java", "class A {\n    int x = Math.max(a, b);\n}\n", nil},
		{"preprocessor", "c", "#include <stdio.h>\n#define N 10\n", nil},
		{"float and hex", "c", "double x = 1.5 + 0x10;\n", []string{"1: arithmetic + -> -"}},
		{"suffix", "rust", "fn f() {\n    let x = 1usize;\n}\n", []string{"2: constant 1 -> 2", "2: constant 1 -> 0"}},
		{"lifetime", "rust", "fn f<'a>(x: &'a str) -> char {\n    'a'\n}\n", nil},
		{"go for range", "go", "func f() {\n\tfor i := range a {\n\t}\n}\n", nil},
	}
	for _, tc := range tests {
		got := ids(t, tc.lang, tc.src)
		if !slices.Equal(got, tc.want) {
			t.Errorf("%s:\n got  %q\n want %q", tc.name, got, tc.want)
		}
	}
}

func TestSample(t *testing.T) {
	var src strings.Builder
	for range 100 {
		src.WriteString("x = a + 1\n")
	}
	ms, err := Mutants("python", "f.py", src.String())
	if err != nil {
		t.Fatal(err)
	}
	a, b := Sample(ms, MaxPerFile), Sample(ms, MaxPerFile)
	if len(a) != MaxPerFile || !slices.Equal(a, b) {
		t.Fatalf("the sample has %d mutants, or it changes from run to run", len(a))
	}
	for i := 1; i < len(a); i++ {
		if a[i].Start < a[i-1].Start {
			t.Fatal("the sample is not in the order of the file")
		}
	}
	if got := Sample(ms, 0); len(got) != len(ms) {
		t.Errorf("Sample with 0 gives %d of %d", len(got), len(ms))
	}
}

func TestParseIgnore(t *testing.T) {
	list, err := ParseIgnore("# equivalent mutants\n\nf.py:3: relational < -> <= # the values are never equal\n")
	if err != nil || len(list) != 1 || list[0].ID != "f.py:3: relational < -> <=" || list[0].Line != 3 {
		t.Fatalf("%+v, %v", list, err)
	}
	if _, err := ParseIgnore("f.py:3: relational < -> <=\n"); err == nil {
		t.Error("an entry with no reason is accepted")
	}
}
