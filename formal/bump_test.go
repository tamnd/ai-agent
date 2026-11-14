// SPDX-License-Identifier: Apache-2.0

package formal

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

const lakefile = `name = "ai"
defaultTargets = ["AI"]

[[require]]
name = "batteries"
git = "https://github.com/leanprover-community/batteries"
rev = "v4.35.0"

[[require]]
name = "mathlib"
git = "https://github.com/leanprover-community/mathlib4"
rev = "v4.35.0" # the tag of the toolchain

[[lean_lib]]
name = "AI"
`

func writePackage(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"lakefile.toml":        lakefile,
		"lean-toolchain":       "leanprover/lean4:v4.35.0\n",
		"AI/Basic.lean":        "",
		"AI/Graph/Search.lean": "",
		"AI/Graph/notes.md":    "",
		"AITest/Basic.lean":    "",
	}
	for name, body := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestPins(t *testing.T) {
	dir := writePackage(t)
	p, err := ReadPins(dir)
	if err != nil {
		t.Fatal(err)
	}
	if want := (Pins{"leanprover/lean4:v4.35.0", "v4.35.0"}); p != want || p.Tag() != "v4.35.0" {
		t.Fatalf("ReadPins = %+v, tag %s", p, p.Tag())
	}
	if err := WritePins(dir, "v4.36.1"); err != nil {
		t.Fatal(err)
	}
	p, err = ReadPins(dir)
	if err != nil {
		t.Fatal(err)
	}
	if want := (Pins{"leanprover/lean4:v4.36.1", "v4.36.1"}); p != want {
		t.Fatalf("after WritePins, pins = %+v", p)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "lakefile.toml"))
	// Only the rev of mathlib moves: batteries and the comment stay.
	want := strings.Replace(lakefile, `rev = "v4.35.0" # the tag`, `rev = "v4.36.1" # the tag`, 1)
	if string(b) != want {
		t.Fatalf("lakefile.toml:\n%s", b)
	}
	for _, bad := range []string{"v4.36.0-rc1", "4.36.0", "master", ""} {
		if err := WritePins(dir, bad); err == nil {
			t.Errorf("WritePins(%q) took a tag that is not a release", bad)
		}
	}
}

func TestMathlibRevMissing(t *testing.T) {
	for _, toml := range []string{
		"name = \"ai\"\n",
		"[[require]]\nname = \"mathlib\"\ngit = \"x\"\n\n[[lean_lib]]\nname = \"AI\"\nrev = \"v1.0.0\"\n",
	} {
		if _, _, err := mathlibRev(toml); err == nil {
			t.Errorf("mathlibRev found a rev in\n%s", toml)
		}
	}
}

func TestLatestTag(t *testing.T) {
	ls := strings.Join([]string{
		"a1\trefs/tags/v4.9.0",
		"a2\trefs/tags/v4.35.0",
		"a3\trefs/tags/v4.35.0^{}",
		"a4\trefs/tags/v4.36.0-rc1",
		"a5\trefs/tags/v4.36.0-rc2^{}",
		"a6\trefs/tags/nightly-testing-2026-11-01",
		"a7\trefs/tags/v4.10.2",
		"garbage",
	}, "\n")
	if got := LatestTag(ls); got != "v4.35.0" {
		t.Fatalf("LatestTag = %q", got)
	}
	if got := LatestTag(""); got != "" {
		t.Fatalf("LatestTag of nothing = %q", got)
	}
}

func TestModules(t *testing.T) {
	dir := writePackage(t)
	got, err := Modules(dir)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"AI.Basic", "AI.Graph.Search"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Modules = %v", got)
	}
	got, err = Modules(t.TempDir())
	if err != nil || got != nil {
		t.Fatalf("Modules of an empty package = %v, %v", got, err)
	}
}

func TestCompareDecls(t *testing.T) {
	old := Decls{
		"AI.kept":    {Module: "AI.A", TypeHash: "1", Axioms: []string{"propext"}},
		"AI.lost":    {Module: "AI.A", TypeHash: "2"},
		"AI.sorry":   {Module: "AI.B", TypeHash: "3"},
		"AI.axiom":   {Module: "AI.B", TypeHash: "4"},
		"AI.type":    {Module: "AI.C", TypeHash: "5"},
		"AI.wassorr": {Module: "AI.C", TypeHash: "6", Sorry: true},
		"Mathlib.x":  {Module: "Mathlib.X", TypeHash: "7"},
	}
	cur := Decls{
		"AI.kept":    {Module: "AI.A", TypeHash: "1", Axioms: []string{"propext", "Classical.choice"}},
		"AI.sorry":   {Module: "AI.B", TypeHash: "3", Sorry: true},
		"AI.axiom":   {Module: "AI.B", TypeHash: "4", Axioms: []string{"sorryAx"}},
		"AI.type":    {Module: "AI.C", TypeHash: "50"},
		"AI.wassorr": {Module: "AI.C", TypeHash: "6", Sorry: true},
		"AI.new":     {Module: "AI.C", TypeHash: "8"},
	}
	got := CompareDecls(old, cur)
	want := []DeclChange{
		{"AI.axiom", "AI.B", "axioms"},
		{"AI.lost", "AI.A", "lost"},
		{"AI.sorry", "AI.B", "sorry"},
		{"AI.type", "AI.C", "type"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CompareDecls =\n%v\nwant\n%v", got, want)
	}
}

func TestLakeErrors(t *testing.T) {
	out := "✔ [1/3] Built AI.Basic\nerror: AI/Graph/Search.lean:4:2: unknown identifier 'foo'\n  error: second\nwarning: x\nerror: third\n"
	if got := lakeErrors(out, 2); !reflect.DeepEqual(got, []string{"error: AI/Graph/Search.lean:4:2: unknown identifier 'foo'", "error: second"}) {
		t.Fatalf("lakeErrors = %q", got)
	}
	if got := lakeErrors("killed\n", 5); len(got) != 1 || !strings.Contains(got[0], "no error line") {
		t.Fatalf("lakeErrors with no error line = %q", got)
	}
}

// TestFakeLake is not a test: BuildModules and Update run the test binary in its place, through Wrap. It fails the
// build of a module named in AIK_FAKE_LAKE_FAIL, and records each command in AIK_FAKE_LAKE_LOG.
func TestFakeLake(t *testing.T) {
	i := slices.Index(os.Args, "--")
	if os.Getenv("AIK_FAKE_LAKE") != "1" || i < 0 {
		t.Skip("run by BuildModules in TestBuildModules")
	}
	argv := os.Args[i+1:]
	f, err := os.OpenFile(os.Getenv("AIK_FAKE_LAKE_LOG"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err == nil {
		fmt.Fprintln(f, strings.Join(argv, " "))
		f.Close()
	}
	if len(argv) == 3 && argv[1] == "build" && slices.Contains(strings.Split(os.Getenv("AIK_FAKE_LAKE_FAIL"), ","), argv[2]) {
		fmt.Printf("✔ [1/2] Built Mathlib\nerror: %s.lean:1:0: unknown constant\nerror: build failed\n", argv[2])
		os.Exit(1)
	}
	os.Exit(0)
}

func fakeLake(t *testing.T, fail string) (Options, string) {
	t.Helper()
	log := filepath.Join(t.TempDir(), "lake.log")
	t.Setenv("AIK_FAKE_LAKE", "1")
	t.Setenv("AIK_FAKE_LAKE_LOG", log)
	t.Setenv("AIK_FAKE_LAKE_FAIL", fail)
	o := Options{Dir: writePackage(t), Wrap: func(argv []string) []string {
		return append([]string{os.Args[0], "-test.run=^TestFakeLake$", "--"}, argv...)
	}}
	return o, log
}

func TestBuildModules(t *testing.T) {
	o, log := fakeLake(t, "AI.Graph.Search")
	var all strings.Builder
	o.Log = &all
	ctx := context.Background()
	if err := Update(ctx, o); err != nil {
		t.Fatal(err)
	}
	mods, err := Modules(o.Dir)
	if err != nil {
		t.Fatal(err)
	}
	got, err := BuildModules(ctx, o, mods)
	if err != nil {
		t.Fatal(err)
	}
	want := []Failure{{Module: "AI.Graph.Search", Errors: []string{"error: AI.Graph.Search.lean:1:0: unknown constant", "error: build failed"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildModules = %+v", got)
	}
	b, _ := os.ReadFile(log)
	if want := "lake update mathlib\nlake exe cache get\nlake build AI.Basic\nlake build AI.Graph.Search\n"; string(b) != want {
		t.Fatalf("the commands were\n%s", b)
	}
	if !strings.Contains(all.String(), "unknown constant") {
		t.Fatalf("the log of lake does not have the output of the build:\n%s", all.String())
	}

	o, _ = fakeLake(t, "")
	if got, err := BuildModules(ctx, o, mods); err != nil || got != nil {
		t.Fatalf("BuildModules of a good package = %+v, %v", got, err)
	}
}
