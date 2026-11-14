package main

import (
	"os"
	"path/filepath"
	"testing"
)

const corpus = "../../testdata/corpus"

func TestExitCodes(t *testing.T) {
	cases := []struct {
		args []string
		want int
	}{
		{[]string{"version"}, 0},
		{[]string{"help"}, 0},
		{nil, 2},
		{[]string{"no-such-command"}, 2},
		{[]string{"lean"}, 2},
		{[]string{"bench"}, 3},
		{[]string{"check", "-root", corpus}, 0},
		{[]string{"check", "-root", "no-such-dir"}, 1},
		{[]string{"check", "-no-such-flag"}, 2},
		{[]string{"harvest", "-due", "weekly", "-list"}, 0},
		{[]string{"harvest", "-due", "yearly", "-list"}, 2},
		{[]string{"harvest", "-due", "weekly", "aiid"}, 2},
		{[]string{"harvest", "-list", "no-such-source"}, 2},
		{[]string{"card", "drift", "-root", corpus, "-base", corpus}, 0},
		{[]string{"card", "drift", "-root", corpus}, 2},
		{[]string{"card", "drift", "-base", corpus, "extra"}, 2},
	}
	for _, c := range cases {
		if got := run(c.args); got != c.want {
			t.Errorf("aik %v exits with %d, want %d", c.args, got, c.want)
		}
	}
}

func TestCardDrift(t *testing.T) {
	base, head := t.TempDir(), t.TempDir()
	write := func(root, line string) {
		t.Helper()
		dir := filepath.Join(root, "cards", "benchmark")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "_.jsonl"), []byte(line+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(base, `{"tag":"BBBBBB","kind":"benchmark","slug":"b","items":{"v":500,"src":"S:b"}}`)
	write(head, `{"tag":"BBBBBB","kind":"benchmark","slug":"b","items":{"v":504,"src":"S:b"}}`)
	if got := run([]string{"card", "drift", "-root", head, "-base", base}); got != 0 {
		t.Errorf("a move within 1%% exits with %d", got)
	}
	write(head, `{"tag":"BBBBBB","kind":"benchmark","slug":"b","items":{"v":520,"src":"S:b"}}`)
	if got := run([]string{"card", "drift", "-root", head, "-base", base, "-json"}); got != 1 {
		t.Errorf("a move beyond 1%% exits with %d", got)
	}
}

func TestSiteCheck(t *testing.T) {
	out := filepath.Join(t.TempDir(), "site")
	if got := run([]string{"site", "check", "-root", corpus, "-out", out, "-commit", "test"}); got != 0 {
		t.Fatalf("aik site check exits with %d", got)
	}
}

func TestVizCheck(t *testing.T) {
	if got := run([]string{"viz", "check", "-root", corpus}); got != 0 {
		t.Fatalf("aik viz check exits with %d", got)
	}
}

func TestFlagsAfterArguments(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS(corpus)); err != nil {
		t.Fatal(err)
	}
	if got := run([]string{"new", "method", "heap-sort", "-area", "sorting/comparison-sorts", "-root", root}); got != 0 {
		t.Fatalf("aik new exits with %d", got)
	}
	if _, err := os.Stat(filepath.Join(root, "nodes/sorting/comparison-sorts/heap-sort/node.md")); err != nil {
		t.Fatal(err)
	}
}
