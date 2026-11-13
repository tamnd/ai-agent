// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/ai-agent/node"
)

func TestGateStub(t *testing.T) {
	root := copyCorpus(t)
	if got := run([]string{"new", "method", "heap-sort", "-area", "sorting/comparison-sorts", "-root", root}); got != 0 {
		t.Fatalf("aik new exits with %d", got)
	}
	if got := run([]string{"gate", "-root", root, "-no-tests", "-no-exp", "heap-sort"}); got != 1 {
		t.Errorf("the gate on a stub exits with %d, want 1", got)
	}
	if got := run([]string{"gate", "-root", root, "no-such-node"}); got != 2 {
		t.Errorf("the gate on an unknown slug exits with %d, want 2", got)
	}
	if got := run([]string{"gate", "-root", root}); got != 2 {
		t.Errorf("the gate with no slug exits with %d, want 2", got)
	}
}

func TestGateText(t *testing.T) {
	dir := t.TempDir()
	long := strings.Repeat("word ", 151)
	files := map[string]string{
		"node.md":       "---\nslug: x\n---\n\n## In plain words\n\nA rate $p$ and a sum $a + b$ and " + long + "\n\n## Definition\n\n$$x = y$$ is fine here.\n",
		"claims.yaml":   "claims:\n\t- id: c-a\n",
		"impl/a/x.py":   "x = 1  # a dash \u2014 here\n",
		"impl/Makefile": "all:\n\techo ok\n",
		"blob.bin":      "\x00\x01\t",
	}
	for p, s := range files {
		full := filepath.Join(dir, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var got []string
	for _, p := range textProblems(dir, &node.Node{Dir: dir}) {
		got = append(got, p.Step+" "+p.Path+" "+p.Msg)
	}
	all := strings.Join(got, "\n")
	for _, want := range []string{"plain words node.md 'In plain words' has 159 words", "the formula $a + b$", "characters claims.yaml:2 holds a tab", "characters impl/a/x.py:1 holds an em dash"} {
		if !strings.Contains(all, want) {
			t.Errorf("the problems do not say %q:\n%s", want, all)
		}
	}
	for _, not := range []string{"$p$", "Makefile", "blob.bin", "$$x = y$$"} {
		if strings.Contains(all, not) {
			t.Errorf("the problems name %q:\n%s", not, all)
		}
	}
}
