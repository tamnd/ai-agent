// SPDX-License-Identifier: Apache-2.0

// Package reviewtest writes review records that count, for the tests of the ladders.
package reviewtest

import (
	"os"
	"path/filepath"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/tamnd/ai-agent/node"
	"github.com/tamnd/ai-agent/review"
)

// Reviewer is the reviewer of Pass, who has every skill.
const Reviewer = "github:reviewer"

// Pass writes a passing review of the part of n that hashes files, with a checklist of one question and a register
// that holds the reviewer.
func Pass(t *testing.T, root string, n *node.Node, part string, files ...string) {
	t.Helper()
	write := func(path string, v any) {
		t.Helper()
		b, err := yaml.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	any_, all := review.Skills(n, part)
	skills := append(append([]string{}, any_...), all...)
	for i, s := range skills {
		if s == "code:*" || s == "law:*" {
			skills[i] = s[:len(s)-1] + "x"
		}
	}
	write(filepath.Join(root, filepath.FromSlash(review.Register)), []review.Reviewer{{ID: Reviewer, Name: "A Reviewer", Skills: skills, Key: "ssh-ed25519 AAAA"}})
	if err := os.MkdirAll(filepath.Join(root, "checklists", "test"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(review.ChecklistFile(root, "test/v1"), []byte("1. Is it right?\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := review.Record{Tag: n.Tag, Part: part, Hashes: map[string]string{}, Checklist: "test/v1", Reviewer: Reviewer,
		Verdict: "pass", Date: "2026-11-14", Items: []review.Item{{Check: "1", Result: "pass"}}, Interests: "none"}
	for _, f := range files {
		h, err := review.Hash(f)
		if err != nil {
			t.Fatal(err)
		}
		r.Hashes[review.Key(root, n, f)] = h
	}
	write(review.File(root, n.Tag, part), r)
}
