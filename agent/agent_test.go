// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const register = `version: m-2026.11
classes:
  large: [claude-opus-5-5, claude-fable-5-2]
  medium: [claude-sonnet-5-5]
  small: [claude-haiku-5-5]
  other:
    writer: codex:gpt-6-luna
    auditor: codex:gpt-6-sol
writer_ladder: [other.writer, claude-haiku-5-5, claude-sonnet-5-5, claude-opus-5-5]
writer_ladder_hard: [claude-sonnet-5-5, large]
auditor_for:
  claude-*: other.auditor
  codex:*: claude-sonnet-5-5
  fallback_if_other_unavailable: claude-opus-5-5
verifier: other.auditor
cataloguer: [claude-opus-5-5, other.auditor]
curator: claude-sonnet-5-5
harvester_extract: claude-haiku-5-5
coder: claude-sonnet-5-5
experimenter: claude-sonnet-5-5
reviewer_assistant: claude-sonnet-5-5
`

func write(t *testing.T, dir, name, s string) string {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestModels(t *testing.T) {
	m, err := LoadModels(write(t, t.TempDir(), "models.yaml", register))
	if err != nil {
		t.Fatal(err)
	}
	if m.Version != "m-2026.11" {
		t.Errorf("version = %q", m.Version)
	}
	if probs := m.Check(); len(probs) != 0 {
		t.Errorf("a good register has problems: %v", probs)
	}
	l, _ := m.Ladder(false)
	if len(l) != 4 || l[0] != (Ref{"codex", "gpt-6-luna"}) || l[1] != (Ref{"claude", "claude-haiku-5-5"}) {
		t.Errorf("ladder = %v", l)
	}
	if h, _ := m.Ladder(true); len(h) != 2 || h[1] != (Ref{"claude", "claude-opus-5-5"}) {
		t.Errorf("a class gives its first model; hard ladder = %v", h)
	}
	a, fb, err := m.AuditorFor([]Ref{{"codex", "gpt-6-luna"}})
	if err != nil || a != (Ref{"claude", "claude-sonnet-5-5"}) || fb == nil {
		t.Errorf("auditor of codex = %v, %v, %v; the fallback is claude, not a writer's family, so it counts", a, fb, err)
	}
	a, fb, err = m.AuditorFor([]Ref{{"claude", "claude-haiku-5-5"}})
	if err != nil || a != (Ref{"codex", "gpt-6-sol"}) || fb != nil {
		t.Errorf("auditor of claude = %v, %v, %v; the fallback is claude, so the item waits", a, fb, err)
	}
	if _, _, err := m.AuditorFor([]Ref{{"claude", "claude-haiku-5-5"}, {"codex", "gpt-6-luna"}}); err == nil {
		t.Error("writers of both families have no auditor")
	}
}

func TestModelsFallback(t *testing.T) {
	m, err := LoadModels(write(t, t.TempDir(), "models.yaml", register))
	if err != nil {
		t.Fatal(err)
	}
	// The fallback claude-opus-5-5 counts for a codex writer only when the primary auditor is not claude.
	_, fb, _ := m.AuditorFor([]Ref{{"codex", "gpt-6-luna"}})
	if fb == nil || *fb != (Ref{"claude", "claude-opus-5-5"}) {
		t.Errorf("fallback for codex = %v", fb)
	}
}

func TestCheck(t *testing.T) {
	bad := strings.NewReplacer("version: m-2026.11\n", "", "codex:*: claude-sonnet-5-5", "codex:*: other.writer",
		"cataloguer: [claude-opus-5-5, other.auditor]", "cataloguer: [claude-opus-5-5]", "coder: claude-sonnet-5-5\n", "").Replace(register)
	m, err := LoadModels(write(t, t.TempDir(), "models.yaml", bad))
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(m.Check(), "\n")
	for _, want := range []string{"no version", "the role coder", "the rung codex:gpt-6-luna", "two families"} {
		if !strings.Contains(got, want) {
			t.Errorf("Check misses %q:\n%s", want, got)
		}
	}
	if _, err := LoadModels(write(t, t.TempDir(), "models.yaml", "version: x\n")); err == nil {
		t.Error("a register with no classes loads")
	}
}

func TestPromptsAndRuns(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "prompts/writer/v1.md", "one")
	write(t, dir, "prompts/writer/v10.md", "ten")
	write(t, dir, "prompts/writer/v2.md", "two")
	write(t, dir, "prompts/audit/v1.md", "audit")
	ps, err := LoadPrompts(filepath.Join(dir, "prompts"))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, p := range ps {
		got = append(got, p.Role+"/"+p.Version)
	}
	if strings.Join(got, " ") != "audit/v1 writer/v1 writer/v2 writer/v10" || len(ps[0].SHA256) != 64 {
		t.Errorf("prompts = %v", got)
	}

	write(t, dir, "ai/prov/runs/w-1.yaml", `run: w-1
date: '2026-11-14'
role: writer
items:
- {slug: a, kind: method, status: done}
- {slug: b, kind: concept, status: failed}
models: {codex:gpt-6-luna: {calls: 3, cost: 0.0}}
spent: 1.5
rates: {total: {kind: total, items: 2}}
`)
	write(t, dir, "ai/prov/runs/v-1.yaml", `run: v-1
date: '2026-11-15'
role: verifier
items:
- {slug: a, claim: c1, verdict: supported}
- {slug: a, claim: c2, verdict: unsupported}
`)
	runs, err := LoadRuns(filepath.Join(dir, "ai"))
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 2 || runs[0].Run != "v-1" || runs[0].Slugs != 1 || runs[0].Verdict["supported"] != 1 {
		t.Fatalf("runs = %+v", runs)
	}
	if w := runs[1]; w.Items != 2 || w.Status["done"] != 1 || w.Spent != 1.5 || w.Rates == nil {
		t.Errorf("writer run = %+v", w)
	}
	if none, err := LoadRuns(filepath.Join(dir, "nothing")); err != nil || len(none) != 0 {
		t.Errorf("no runs: %v %v", none, err)
	}
}
