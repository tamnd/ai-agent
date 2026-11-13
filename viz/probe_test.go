// SPDX-License-Identifier: Apache-2.0

package viz

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestProbeLine(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"  let lo = 0, hi = a.length; ", `  __probe("L1"); let lo = 0, hi = a.length; `},
		{"  while (lo < hi) { ", `  while (__probe("L1") && (lo < hi)) { `},
		{"  } else if (a[mid] > f(x)) { ", `  } else if (__probe("L1") && (a[mid] > f(x))) { `},
		{"  if (hi - lo <= 1) return; ", `  if (__probe("L1") && (hi - lo <= 1)) return; `},
		{"  if (s === \")\") { ", `  if (__probe("L1") && (s === ")")) { `},
		{"  for (let i = 1; i < n; i++) { ", `  for (let i = 1; __probe("L1") && (i < n); i++) { `},
		{"  for (;;) { ", `  for (; __probe("L1") && (true);) { `},
		{"  for (const x of xs) { ", `  for (const x of xs) { __probe("L1"); `},
		{"  } else { ", `  } else { __probe("L1"); `},
	} {
		got, err := probeLine(c.in, "L1")
		if err != nil || got != c.want {
			t.Errorf("probeLine(%q) = %q, %v; want %q", c.in, got, err, c.want)
		}
	}
	for _, in := range []string{"   ", "  while (lo <", "  for (const x of xs)"} {
		if _, err := probeLine(in, "L1"); err == nil {
			t.Errorf("probeLine(%q) gives no error", in)
		}
	}
}

func TestLineTrace(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is not installed")
	}
	b, err := Build("testdata/ok/scene.ts", false)
	if err != nil {
		t.Fatal(err)
	}
	presets := []Preset{{"one", []int{5}}, {"three", []int{3, 1, 2}}, {"two", []int{4, 4}}}
	ids := []string{"L1", "L2"}
	ref := func(name string) ([]byte, error) {
		src, err := os.ReadFile("testdata/ok/" + name)
		if err != nil {
			t.Fatal(err)
		}
		return Reference(string(src), name)
	}
	good, err := ref("ref.ts")
	if err != nil {
		t.Fatal(err)
	}
	if r := Check(Scene{Slug: "demo", Bundle: b, Presets: presets, IDs: ids, Ref: good}); len(r.Fails) > 0 {
		t.Fatalf("fails: %v", r.Fails)
	}
	bad, err := ref("wrong.ts")
	if err != nil {
		t.Fatal(err)
	}
	r := Check(Scene{Slug: "demo", Bundle: b, Presets: presets, IDs: ids, Ref: bad})
	if len(r.Fails) != 3 || !strings.Contains(r.Fails[0], "the lines differ from the TypeScript reference at line 1 of 2: the scene runs L1 L2, the reference runs L2 L1 (V06)") {
		t.Fatalf("want 3 V06 fails, got %v", r.Fails)
	}
	_, err = Reference("let x = 1;\nfoo( // @line L1\n", "broken.ts")
	r = Check(Scene{Slug: "demo", Bundle: b, Presets: presets, IDs: ids, RefErr: err})
	if err == nil || len(r.Fails) == 0 || !strings.Contains(r.Fails[0], "broken.ts:3: Unexpected end of file (V06)") {
		t.Fatalf("want a fail for a reference that does not compile, got %v, %v", err, r.Fails)
	}
}
