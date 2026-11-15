// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/tamnd/ai-agent/cite"
)

func TestCopycheck(t *testing.T) {
	root := filepath.Join(t.TempDir(), "corpus")
	if err := os.CopyFS(root, os.DirFS("../../testdata/corpus")); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "nodes", "foundations", "models-of-computation", "toy-model")
	page := func(body string) string {
		return "---\ntag: 7TQYMD\nkind: model\nslug: toy-model\nname: Toy model\narea: foundations/models-of-computation\nlevel: intro\nsummary: A small model.\n---\n\n## Overview\n\n" + body
	}
	source := "1 Introduction\nThe key idea is to randomly drop units, along with their connections, from the neural network during training. This prevents units from co-adapting too much.\n"
	store := filepath.Join(t.TempDir(), "raw")
	h := "ab12"
	files := map[string]string{
		filepath.Join(dir, "claims.yaml"):                  "- id: c-drop\n  type: definitional\n  text: Dropout drops units at random.\n  evidence:\n    - {work: paper, role: original, stance: for, locator: \"Section 1\"}\n",
		filepath.Join(root, "manifests", "works.jsonl"):    `{"work":"paper","hash":"ab12","url":"https://example.org/paper.pdf","extractor":"x"}` + "\n",
		filepath.Join(store, h[:2], h+".doc.json"):         mustJSON(t, cite.Doc{Extractor: "x", Text: source}),
		filepath.Join(root, "refs", "wikipedia.jsonl"):     mustJSON(t, refText{Name: "wikipedia:Other", Items: []string{"other-node"}, Text: source}) + "\n",
		filepath.Join(dir, "node.md"):                      page("The key idea is to randomly drop units, along with their connections, from the neural network during training {#c-drop}.\n"),
		filepath.Join(root, "refs", "notes.txt"):           "not a reference file",
		filepath.Join(root, "manifests", "unrelated.json"): "{}",
	}
	for p, s := range files {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	args := []string{"copycheck", "-root", root, "-store", store, "-ref", filepath.Join(root, "refs")}
	if got := run(append(args, "toy-model")); got != 1 {
		t.Errorf("a copied sentence: aik copycheck exits with %d, want 1", got)
	}
	if got := run(append(args, filepath.Join(dir, "node.md"))); got != 1 {
		t.Errorf("a path in the node: aik copycheck exits with %d, want 1", got)
	}
	if err := os.WriteFile(filepath.Join(dir, "node.md"), []byte(page("While the network trains, it switches off a random part of its units at each step {#c-drop}.\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := run(append(args, "toy-model")); got != 0 {
		t.Errorf("a sentence written afresh: aik copycheck exits with %d, want 0", got)
	}
	if got := run(append(args, "no-such-node")); got != 2 {
		t.Errorf("an unknown slug: aik copycheck exits with %d, want 2", got)
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
