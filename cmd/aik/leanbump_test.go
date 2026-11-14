// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/ai-agent/formal"
)

func TestLeanBumpDryRun(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "lean")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	lf := "name = \"ai\"\n\n[[require]]\nname = \"mathlib\"\ngit = \"https://github.com/leanprover-community/mathlib4\"\nrev = \"v4.35.0\"\n"
	for name, body := range map[string]string{"lakefile.toml": lf, "lean-toolchain": "leanprover/lean4:v4.35.0\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	tags := "a\trefs/tags/v4.35.0\nb\trefs/tags/v4.36.0\nc\trefs/tags/v4.37.0-rc1\n"
	old := lsRemote
	t.Cleanup(func() { lsRemote = old })
	lsRemote = func(context.Context, string) (string, error) { return tags, nil }

	if got := run([]string{"lean", "bump", "-root", root, "-n"}); got != 0 {
		t.Fatalf("aik lean bump -n exits with %d", got)
	}
	if got := run([]string{"lean", "bump", "-root", root, "-n", "-to", "v4.35.0"}); got != 0 {
		t.Fatalf("aik lean bump -to the current tag exits with %d", got)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "lakefile.toml"))
	if string(b) != lf {
		t.Fatalf("a dry run changed lakefile.toml:\n%s", b)
	}
	if got := run([]string{"lean", "bump", "-root", root, "extra"}); got != 2 {
		t.Fatalf("aik lean bump with an argument exits with %d", got)
	}
}

func TestBumpReport(t *testing.T) {
	from := formal.Pins{Toolchain: "leanprover/lean4:v4.35.0", Mathlib: "v4.35.0"}
	to := formal.Pins{Toolchain: "leanprover/lean4:v4.36.0", Mathlib: "v4.36.0"}

	// Every module builds, one proof now uses sorry, one statement changed its type.
	nodes := []bumpNode{
		{Slug: "binary-search", Module: "AI.Search", Before: 3, After: 3},
		{Slug: "gradient-descent", Module: "AI.Opt", Before: 2, After: 1, Why: "prove the theorem without sorry"},
		{Slug: "perceptron", Module: "AI.Perceptron", Before: 1, After: 1},
	}
	changes := []formal.DeclChange{{Decl: "AI.Opt.converges", Module: "AI.Opt", What: "sorry"}, {Decl: "AI.Search.correct", Module: "AI.Search", What: "type"}}
	rep, drops := bumpReport(from, to, 3, nil, changes, nodes)
	if drops != 1 {
		t.Fatalf("drops = %d\n%s", drops, rep)
	}
	for _, s := range []string{"# Lean bump to v4.36.0", "`AI.Opt.converges` | `AI.Opt` | now depends on sorry", "type changed", "| gradient-descent | F2 | F1 |", "**1 node at F2 or above breaks.**"} {
		if !strings.Contains(rep, s) {
			t.Errorf("the report lacks %q:\n%s", s, rep)
		}
	}
	if strings.Contains(rep, "| binary-search |") || strings.Contains(rep, "| perceptron |") {
		t.Errorf("the report lists a node whose level did not change:\n%s", rep)
	}

	// A module does not build, so the levels after are not known.
	nodes = []bumpNode{
		{Slug: "binary-search", Module: "AI.Search", Before: 3, After: -1, Broken: true},
		{Slug: "perceptron", Module: "AI.Perceptron", Before: 1, After: -1},
	}
	fails := []formal.Failure{{Module: "AI.Search", Errors: []string{"error: `x` unknown"}}}
	rep, drops = bumpReport(from, to, 2, fails, nil, nodes)
	if drops != 1 || !strings.Contains(rep, "- `AI.Search` (binary-search)") || !strings.Contains(rep, "| binary-search | F3 | does not build |") || !strings.Contains(rep, "| perceptron | F1 | not known |") || !strings.Contains(rep, "'x'") {
		t.Fatalf("drops = %d\n%s", drops, rep)
	}

	// Nothing changes.
	rep, drops = bumpReport(from, to, 1, nil, nil, []bumpNode{{Slug: "a", Module: "AI.A", Before: 2, After: 2}})
	if drops != 0 || !strings.Contains(rep, "so the bump can merge") || strings.Contains(rep, "## ") {
		t.Fatalf("drops = %d\n%s", drops, rep)
	}
}
