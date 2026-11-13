// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func copyCorpus(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS(corpus)); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestMint(t *testing.T) {
	tagSource = bytes.NewReader(bytes.Repeat([]byte{7, 1, 200, 33, 90, 14, 250, 3}, 64))
	t.Cleanup(func() { tagSource = cryptoSource })
	root := copyCorpus(t)
	for _, s := range []string{"heap-sort", "shell-sort"} {
		if got := run([]string{"new", "method", s, "-area", "sorting/comparison-sorts", "-root", root}); got != 0 {
			t.Fatalf("aik new %s exits with %d", s, got)
		}
	}
	reg := filepath.Join(root, "registers", "tags.reg")
	before, _ := os.ReadFile(reg)
	if got := run([]string{"mint", "-dry-run", "-root", root}); got != 0 {
		t.Fatalf("aik mint -dry-run exits with %d", got)
	}
	if after, _ := os.ReadFile(reg); !bytes.Equal(before, after) {
		t.Fatal("aik mint -dry-run changed tags.reg")
	}
	if got := run([]string{"mint", "-root", root, "-author", "tester"}); got != 0 {
		t.Fatalf("aik mint exits with %d", got)
	}
	tagRE := regexp.MustCompile(`(?m)^tag: ([0-9A-HJKMNP-TV-Z]{6})$`)
	for _, s := range []string{"heap-sort", "shell-sort"} {
		b, err := os.ReadFile(filepath.Join(root, "nodes/sorting/comparison-sorts", s, "node.md"))
		if err != nil {
			t.Fatal(err)
		}
		m := tagRE.FindSubmatch(b)
		if m == nil {
			t.Fatalf("%s has no permanent tag:\n%s", s, b)
		}
		tag := string(m[1])
		regText, _ := os.ReadFile(reg)
		if !strings.Contains(string(regText), tag+" "+s+"\n") {
			t.Errorf("tags.reg has no line %q", tag+" "+s)
		}
		prov, err := os.ReadFile(filepath.Join(root, "prov", tag+".jsonl"))
		if err != nil || !strings.Contains(string(prov), `"author":"tester"`) {
			t.Errorf("prov/%s.jsonl is %q, %v", tag, prov, err)
		}
	}
	if got := run([]string{"check", "-root", root}); got != 0 {
		t.Fatalf("aik check after mint exits with %d", got)
	}
	again, _ := os.ReadFile(reg)
	if got := run([]string{"mint", "-root", root}); got != 0 {
		t.Fatalf("a second aik mint exits with %d", got)
	}
	if now, _ := os.ReadFile(reg); !bytes.Equal(again, now) {
		t.Fatal("a second aik mint changed tags.reg")
	}
}

func TestMintSkipsUsedTags(t *testing.T) {
	used := map[string]bool{}
	for range 200 {
		tag, err := newTag(used)
		if err != nil {
			t.Fatal(err)
		}
		if len(tag) != 6 || strings.ContainsAny(tag, "ILOU") {
			t.Fatalf("bad tag %q", tag)
		}
	}
	if len(used) != 200 {
		t.Fatalf("got %d different tags, want 200", len(used))
	}
}

func TestFmt(t *testing.T) {
	root := copyCorpus(t)
	p := filepath.Join(root, "nodes/searching/sequential-and-binary-search/binary-search/node.md")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, append(bytes.ReplaceAll(b, []byte("\n## "), []byte("  \n\n\n## ")), '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := run([]string{"fmt", "-check", "-root", root}); got != 1 {
		t.Fatalf("aik fmt -check on a changed file exits with %d, want 1", got)
	}
	if got := run([]string{"fmt", "-root", root}); got != 0 {
		t.Fatalf("aik fmt exits with %d", got)
	}
	if after, _ := os.ReadFile(p); !bytes.Equal(after, b) {
		t.Fatalf("aik fmt does not give back the canonical file")
	}
	if got := run([]string{"fmt", "-check", "-root", root}); got != 0 {
		t.Fatalf("aik fmt -check after aik fmt exits with %d", got)
	}
}

func TestMintCards(t *testing.T) {
	tagSource = bytes.NewReader(bytes.Repeat([]byte{9, 4, 120, 33, 70, 14, 230, 5}, 64))
	t.Cleanup(func() { tagSource = cryptoSource })
	root := copyCorpus(t)
	shard := filepath.Join(root, "cards", "model", "_.jsonl")
	if err := os.MkdirAll(filepath.Dir(shard), 0o755); err != nil {
		t.Fatal(err)
	}
	lines := `{"tag":"NEW-gpt-4","kind":"model","slug":"gpt-4","name":"GPT-4"}` + "\n" +
		`{"tag": "NEW-llama-2", "kind": "model", "slug": "llama-2", "name": "Llama 2"}` + "\n"
	if err := os.WriteFile(shard, []byte(lines), 0o644); err != nil {
		t.Fatal(err)
	}
	reg := filepath.Join(root, "registers", "universe", "_.jsonl")
	if err := os.MkdirAll(filepath.Dir(reg), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(reg, []byte(`{"id":"u:epoch:GPT-4","name":"GPT-4","kind":"model","outcome":"card","target":"NEW-gpt-4"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := run([]string{"mint", "-root", root, "-author", "tester", "llama-2"}); got != 0 {
		t.Fatalf("aik mint llama-2 exits with %d", got)
	}
	b, _ := os.ReadFile(shard)
	if !strings.Contains(string(b), `"tag":"NEW-gpt-4"`) || strings.Contains(string(b), "NEW-llama-2") {
		t.Fatalf("aik mint llama-2 minted the wrong card:\n%s", b)
	}
	if got := run([]string{"mint", "-root", root}); got != 0 {
		t.Fatalf("aik mint exits with %d", got)
	}
	b, _ = os.ReadFile(shard)
	re := regexp.MustCompile(`^\{"tag": ?"([0-9A-HJKMNP-TV-Z]{6})",`)
	got := strings.Split(strings.TrimSpace(string(b)), "\n")
	tags := map[string]string{}
	var order []string
	for _, ln := range got {
		m := re.FindStringSubmatch(ln)
		if m == nil {
			t.Fatalf("the card line has no permanent tag: %s", ln)
		}
		slug := "gpt-4"
		if strings.Contains(ln, "llama-2") {
			slug = "llama-2"
			// The other bytes of the line stay as they were.
			if want := `{"tag": "` + m[1] + `", "kind": "model", "slug": "llama-2", "name": "Llama 2"}`; ln != want {
				t.Errorf("the line is %s, want %s", ln, want)
			}
		}
		tags[slug] = m[1]
		order = append(order, m[1])
	}
	if len(order) != 2 || order[0] > order[1] {
		t.Errorf("the shard is not sorted by tag:\n%s", b)
	}
	regText, _ := os.ReadFile(filepath.Join(root, "registers", "tags.reg"))
	for slug, tag := range tags {
		if !strings.Contains(string(regText), tag+" "+slug+"\n") {
			t.Errorf("tags.reg has no line %q", tag+" "+slug)
		}
	}
	u, _ := os.ReadFile(reg)
	if !strings.Contains(string(u), `"target":"ai://`+tags["gpt-4"]+`"`) {
		t.Errorf("the universe item has not the new tag %s: %s", tags["gpt-4"], u)
	}
	if got := run([]string{"check", "-root", root}); got != 0 {
		t.Fatalf("aik check after mint of the cards exits with %d", got)
	}
}
