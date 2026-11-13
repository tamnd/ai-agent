// SPDX-License-Identifier: Apache-2.0

package node

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestKindFields(t *testing.T) {
	write := func(front string) (*Node, error) {
		root := t.TempDir()
		dir := filepath.Join(root, "nodes", "x")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "node.md"), []byte("---\n"+front+"---\n\nBody.\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		return Read(root, dir)
	}
	head := "tag: T\nslug: x\nname: X\narea: a/b\nlevel: core\nsummary: s\n"

	n, err := write("kind: argument\n" + head + "premises: [p1, p2]\nconclusion: c\nreplies:\n  - {name: r, summary: s}\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(n.Fields) != 3 || n.Fields["conclusion"] != "c" || n.Name != "X" {
		t.Errorf("the fields of an argument are %v", n.Fields)
	}

	n, err = write("kind: finding\n" + head + "claim: c\nreplications: [w1]\nsources: [{book: b}]\n")
	if err != nil || n.Fields["claim"] != "c" || len(n.Sources) != 1 {
		t.Errorf("a finding reads %v, %v", n, err)
	}

	// The evidence of an open problem is written; on other kinds it is a status (T05).
	n, err = write("kind: open-problem\n" + head + "question: q\nevidence: [c-a]\n")
	if err != nil || n.Fields["evidence"] == nil || len(n.Forbidden) != 0 {
		t.Errorf("an open problem reads %v, forbidden %v, %v", n.Fields, n.Forbidden, err)
	}
	if n, err = write("kind: method\n" + head + "evidence: [c-a]\n"); err != nil || len(n.Forbidden) != 1 {
		t.Errorf("a method with evidence reads forbidden %v, %v", n.Forbidden, err)
	}

	// A field of another kind, or a misspelt one, is an error.
	for _, f := range []string{"kind: method\n" + head + "premises: [p]\n", "kind: argument\n" + head + "premise: [p]\n"} {
		if _, err := write(f); err == nil || !strings.Contains(err.Error(), "not found") {
			t.Errorf("%q: err %v, want an unknown field", f, err)
		}
	}
}

func TestWorkNodeCard(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../testdata/corpus")); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "nodes", "w")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	front := "tag: 2W7QDX\nkind: work\nslug: vaswani-2017-attention\nname: Attention Is All You Need\narea: a/b\nlevel: core\nsummary: s\n" +
		"type: paper\nauthors: [Ashish Vaswani]\nyear: 2017\nstanding: peer-reviewed\nlicence_class: D\n" +
		"cited: {version: arXiv 1706.03762v7, url: \"https://arxiv.org/pdf/1706.03762v7\", format: pdf}\n"
	if err := os.WriteFile(filepath.Join(dir, "node.md"), []byte("---\n"+front+"---\n\nBody.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Errors) != 0 {
		t.Fatalf("load: %v", c.Errors)
	}
	cd := c.CardByKey["vaswani-2017-attention"]
	if cd == nil || cd.Kind != "work" || cd.Tag != "2W7QDX" || cd.File != "nodes/w/node.md" {
		t.Fatalf("the work node has the card %+v", cd)
	}
	if !strings.Contains(string(cd.Fields["cited"]), "1706.03762v7") || !strings.Contains(string(cd.Fields["authors"]), "Vaswani") {
		t.Errorf("the card fields are %s", cd.Fields)
	}
	for _, x := range c.Cards {
		if x == cd {
			t.Error("the work card of the node is in Cards")
		}
	}
}
