// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPromote(t *testing.T) {
	root := copyCorpus(t)
	write := func(rel, s string) {
		t.Helper()
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	keep := `{"tag":"7KQ2MX","kind":"system","slug":"deep-blue","name":"Deep Blue","summary":"A chess computer of IBM."}`
	write("cards/system/_.jsonl", keep+"\n"+`{"tag":"5TQH2W","kind":"system","slug":"logic-theorist","name":"Logic Theorist","developer":["rand-corporation"],"years":{"v":"1956","src":"S:wikidata"},"summary":"An early program that proved theorems of Principia Mathematica.","harvest":{"source":"wikidata","row":"Q1868364","date":"2026-11-10"}}`+"\n")
	write("registers/universe/foundations.jsonl", `{"id":"U1","name":"Logic Theorist","kind":"system","area":"foundations/models-of-computation","outcome":"card","target":"ai://5TQH2W"}`+"\n")

	if got := run([]string{"promote", "-root", root, "-n", "logic-theorist"}); got != 0 {
		t.Fatalf("aik promote -n exits with %d", got)
	}
	if _, err := os.Stat(filepath.Join(root, "nodes/foundations/models-of-computation/logic-theorist")); err == nil {
		t.Fatal("a dry run wrote the node")
	}
	if got := run([]string{"promote", "-root", root, "-author", "github:someone", "5tqh2w"}); got != 0 {
		t.Fatalf("aik promote exits with %d", got)
	}
	dir := filepath.Join(root, "nodes/foundations/models-of-computation/logic-theorist")
	b, err := os.ReadFile(filepath.Join(dir, "node.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"tag: 5TQH2W\n", "kind: system\n", "area: foundations/models-of-computation\n", "summary: An early program"} {
		if !strings.Contains(string(b), s) {
			t.Errorf("node.md lacks %q:\n%s", s, b)
		}
	}
	if strings.Contains(string(b), "years") {
		t.Errorf("a fact stayed in the front matter:\n%s", b)
	}
	fb, err := os.ReadFile(filepath.Join(dir, "facts.yaml"))
	if err != nil || !strings.Contains(string(fb), "id: years") || !strings.Contains(string(fb), "kind: secondary") || !strings.Contains(string(fb), "row: Q1868364") || !strings.Contains(string(fb), "id: developer") {
		t.Fatalf("facts.yaml = %s, %v", fb, err)
	}
	cards, _ := os.ReadFile(filepath.Join(root, "cards/system/_.jsonl"))
	if strings.TrimSpace(string(cards)) != keep {
		t.Errorf("the shard is\n%s", cards)
	}
	reg, _ := os.ReadFile(filepath.Join(root, "registers/universe/foundations.jsonl"))
	if !strings.Contains(string(reg), `"outcome":"node"`) {
		t.Errorf("the universe row is %s", reg)
	}
	pv, _ := os.ReadFile(filepath.Join(root, "prov/5TQH2W.jsonl"))
	if !strings.Contains(string(pv), `"op":"create"`) || !strings.Contains(string(pv), "S:wikidata") {
		t.Errorf("the provenance log is %s", pv)
	}
	// The node now holds the slug and the tag.
	if got := run([]string{"promote", "-root", root, "deep-blue", "-area", "foundations/models-of-computation"}); got != 0 {
		t.Fatalf("aik promote deep-blue exits with %d", got)
	}
	for _, args := range [][]string{
		{"promote", "-root", root, "logic-theorist"},
		{"promote", "-root", root, "no-such-card"},
		{"promote", "-root", root},
	} {
		if got := run(args); got == 0 {
			t.Errorf("aik %v exits with 0", args)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "cards/system/_.jsonl")); err == nil {
		t.Error("an empty shard was kept")
	}
}
