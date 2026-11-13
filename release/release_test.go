// SPDX-License-Identifier: Apache-2.0

package release

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tamnd/ai-agent/card"
	"github.com/tamnd/ai-agent/coverage"
	"github.com/tamnd/ai-agent/exp"
	"github.com/tamnd/ai-agent/node"
)

func TestValid(t *testing.T) {
	for _, tc := range []struct {
		v  string
		ok bool
	}{
		{"2027.03", true},
		{"2027.03.1", true},
		{"2027.12.10", true},
		{"2027.3", false},
		{"2027.13", false},
		{"2027.00", false},
		{"2027.03.0", false},
		{"v2027.03", false},
	} {
		if Valid(tc.v) != tc.ok {
			t.Errorf("Valid(%q) = %v", tc.v, !tc.ok)
		}
	}
	for _, tc := range [][2]string{{"2027.03", "2027.03.1"}, {"2027.03.1", "2027.03.2"}, {"2027.03.9", "2027.04"}, {"2027.12", "2028.01"}} {
		if !Less(tc[0], tc[1]) || Less(tc[1], tc[0]) {
			t.Errorf("%s < %s does not hold", tc[0], tc[1])
		}
	}
}

func run(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

func write(t *testing.T, root, p, s string) {
	t.Helper()
	p = filepath.Join(root, p)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCut(t *testing.T) {
	for _, k := range []string{"GIT_AUTHOR_NAME", "GIT_COMMITTER_NAME"} {
		t.Setenv(k, "test")
	}
	for _, k := range []string{"GIT_AUTHOR_EMAIL", "GIT_COMMITTER_EMAIL"} {
		t.Setenv(k, "test@example.invalid")
	}
	root := t.TempDir()
	write(t, root, "registers/models.yaml", "version: m-2026.11\n")
	write(t, root, "registers/canon.yaml", "version: c-2026.11\nworks: []\n")
	write(t, root, "manifests/images.yaml", "numpy: ghcr.io/tamnd/numpy@sha256:"+strings.Repeat("a", 64)+"\n")
	write(t, root, "lean/lean-toolchain", "leanprover/lean4:v4.20.0\n")
	write(t, root, "lean/lake-manifest.json", `{"packages": [{"name": "mathlib", "rev": "abc123"}]}`)
	run(t, root, "init", "-q")
	run(t, root, "add", ".")
	run(t, root, "commit", "-q", "-m", "content")

	c := &node.Corpus{
		Nodes: []*node.Node{{Front: node.Front{Tag: "3F2K", Slug: "dropout", Area: "deep-learning-training/regularization"}}},
		Cards: []*card.Card{{Kind: "work", Slug: "w"}},
		Pins:  map[string]*exp.Pin{"mnist": {ID: "mnist", Dataset: "MNIST", Version: "1", Files: []exp.PinFile{{Name: "train.gz", SHA256: "ff"}}}},
	}
	cov := &coverage.Report{Levels: coverage.Levels{Total: coverage.Counts{0, 1, 0, 1}}, Items: []coverage.Item{{ID: "dropout"}}}
	o := Options{Root: root, Version: "2027.03", Corpus: c, Coverage: cov, Aik: "v0.1.0", Now: time.Date(2027, 3, 1, 0, 0, 0, 0, time.UTC), Tag: true}

	if _, err := Cut(Options{Root: root, Version: "2027.3", Corpus: c}); err == nil || !strings.Contains(err.Error(), "YYYY.MM") {
		t.Errorf("bad version: %v", err)
	}
	r, err := Cut(o)
	if err != nil {
		t.Fatal(err)
	}
	m := r.Manifest
	if len(m.Commit) != 40 || m.Aik != "v0.1.0" || m.Models != "m-2026.11" || m.Canon != "c-2026.11" || len(m.Images) != 1 {
		t.Errorf("manifest %+v", m)
	}
	if m.Lean == nil || m.Lean.Toolchain != "leanprover/lean4:v4.20.0" || m.Lean.Mathlib != "abc123" {
		t.Errorf("lean %+v", m.Lean)
	}
	if len(m.Data) != 1 || m.Data[0].Files["train.gz"] != "ff" || m.Tags["3F2K"] != "dropout" || m.Nodes["deep-learning-training"] != 1 {
		t.Errorf("data %+v, tags %v, nodes %v", m.Data, m.Tags, m.Nodes)
	}
	if !r.Tagged || r.Previous != nil || len(r.Files) != 3 {
		t.Errorf("tagged %v, previous %v, files %v", r.Tagged, r.Previous, r.Files)
	}
	got, err := Latest(filepath.Join(root, Dir), "")
	if err != nil || got == nil || got.Commit != m.Commit || got.Coverage != m.Coverage {
		t.Errorf("read back: %+v, %v", got, err)
	}

	// The release is frozen, and a dirty tree is refused.
	if _, err := Cut(o); err == nil || !strings.Contains(err.Error(), "frozen") {
		t.Errorf("cut again: %v", err)
	}
	o.Version = "2027.03.1"
	if _, err := Cut(o); err == nil || !strings.Contains(err.Error(), "not committed") {
		t.Errorf("dirty tree: %v", err)
	}
	run(t, root, "add", ".")
	run(t, root, "commit", "-q", "-m", "release 2027.03")

	// A patch release names the previous release, the new nodes and the retired tags.
	c.Nodes = []*node.Node{{Front: node.Front{Tag: "7QXA", Slug: "batch-norm", Area: "deep-learning-training/normalization"}}}
	r, err = Cut(o)
	if err != nil {
		t.Fatal(err)
	}
	if r.Previous == nil || r.Previous.Version != "2027.03" {
		t.Fatalf("previous %+v", r.Previous)
	}
	notes, err := os.ReadFile(filepath.Join(root, Dir, "2027.03.1.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"since 2027.03", "batch-norm (7QXA)", "- 3F2K (dropout)", "| deep-learning-training | 1 | +0 |", "Mathlib abc123"} {
		if !strings.Contains(string(notes), want) {
			t.Errorf("the notes have no %q:\n%s", want, notes)
		}
	}
}
