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
	}
	for _, c := range cases {
		if got := run(c.args); got != c.want {
			t.Errorf("aik %v exits with %d, want %d", c.args, got, c.want)
		}
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
	if got := run([]string{"new", "algorithm", "heap-sort", "-area", "sorting/comparison-sorts", "-root", root}); got != 0 {
		t.Fatalf("aik new exits with %d", got)
	}
	if _, err := os.Stat(filepath.Join(root, "nodes/sorting/comparison-sorts/heap-sort/node.md")); err != nil {
		t.Fatal(err)
	}
}
