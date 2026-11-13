// SPDX-License-Identifier: Apache-2.0

package site

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/ai-agent/node"
)

func TestBridgeLinks(t *testing.T) {
	root := filepath.Join(t.TempDir(), "corpus")
	if err := os.CopyFS(root, os.DirFS("../testdata/corpus")); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(root, "nodes", "searching", "sequential-and-binary-search", "binary-search", "node.md")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	md := strings.Replace(string(b), "see_also: [loop-invariant]", "see_also: [loop-invariant, binary-search-tree@al, gone@al]", 1)
	md += "\nThe tree form is in [[al:binary-search-tree]], and [[al:gone|this item]] has left.\n"
	files := map[string]string{
		p: md,
		filepath.Join(root, "cards", "bridge", "al", "_.jsonl"): `{"tag":"7BST00","site":"al","kind":"data-structure","slug":"binary-search-tree","name":"Binary search tree","url":"https://algorithm.tamnd.com/binary-search-tree/"}` + "\n" +
			`{"tag":"7GONE0","site":"al","kind":"algorithm","slug":"gone","name":"Gone","url":"https://algorithm.tamnd.com/gone/","retired":"2026-11-08"}` + "\n",
	}
	for f, s := range files {
		if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	c, err := node.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "site")
	if _, err := Build(c, Options{Out: out}); err != nil {
		t.Fatal(err)
	}
	b, err = os.ReadFile(filepath.Join(out, "binary-search", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	page := string(b)
	for _, want := range []string{
		`<a href="https://algorithm.tamnd.com/binary-search-tree/">Binary search tree</a> <span class="elsewhere">on algorithm.tamnd.com</span>`,
		`in <a href="https://algorithm.tamnd.com/binary-search-tree/">Binary search tree</a>, and this item has left.`,
		`<span class="missing" title="This item is not on algorithm.tamnd.com.">Gone</span>`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the page does not have %q", want)
		}
	}
}
