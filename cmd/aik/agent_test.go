// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAgent(t *testing.T) {
	root := t.TempDir()
	reg := `version: m-test
classes:
  medium: [claude-sonnet-5-5]
  other: {writer: "codex:gpt-6-luna", auditor: "codex:gpt-6-sol"}
writer_ladder: [other.writer, claude-sonnet-5-5]
writer_ladder_hard: [claude-sonnet-5-5]
auditor_for: {"claude-*": other.auditor, "codex:*": claude-sonnet-5-5}
verifier: other.auditor
cataloguer: [claude-sonnet-5-5, other.auditor]
curator: claude-sonnet-5-5
harvester_extract: claude-sonnet-5-5
coder: claude-sonnet-5-5
experimenter: claude-sonnet-5-5
reviewer_assistant: claude-sonnet-5-5
`
	p := filepath.Join(root, "registers", "models.yaml")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(reg), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AI_MODELS", "")
	if code := run([]string{"agent", "models", "-root", root}); code != 0 {
		t.Errorf("a good register exits with %d", code)
	}
	bad := filepath.Join(root, "bad.yaml")
	if err := os.WriteFile(bad, []byte("classes: {}\nwriter_ladder: [claude-sonnet-5-5]\nauditor_for: {\"claude-*\": claude-opus-5-5}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := run([]string{"agent", "models", "-models", bad}); code != 1 {
		t.Errorf("a register with a same-family auditor exits with %d, want 1", code)
	}
	if code := run([]string{"agent", "runs", "-root", root, "-json"}); code != 0 {
		t.Errorf("no runs exits with %d", code)
	}
	if code := run([]string{"agent", "prompts", "-dir", filepath.Join("..", "..", "work", "prompts")}); code != 0 {
		t.Errorf("prompts exits with %d", code)
	}
	if code := run([]string{"agent", "nothing"}); code != 2 {
		t.Errorf("an unknown subcommand exits with %d", code)
	}
}
