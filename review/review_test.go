// SPDX-License-Identifier: Apache-2.0

package review_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/tamnd/ai-agent/internal/reviewtest"
	"github.com/tamnd/ai-agent/node"
	"github.com/tamnd/ai-agent/review"
)

// A record counts when it passes every item of its checklist, hashes the current files, and comes from a reviewer of
// the register with the skill of the part who is not an author; each of these, broken, tells why it does not count.
func TestCheck(t *testing.T) {
	root := t.TempDir()
	n := &node.Node{Slug: "dropout", Tag: "2NT5ZY", Rel: "nodes/deep-learning-training/regularization/dropout"}
	dir := filepath.Join(root, filepath.FromSlash(n.Rel))
	text, claims := filepath.Join(dir, "node.md"), filepath.Join(dir, "claims.yaml")
	lean := filepath.Join(root, "lean", "AI", "Dropout.lean")
	for f, s := range map[string]string{text: "# Dropout\n", claims: "- id: c\n", lean: "theorem t : True := trivial\n"} {
		if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if ok, why := review.Check(root, n, "text", text); ok || !strings.Contains(why, "no review record reviews/2NT5ZY/text.yaml") {
		t.Errorf("no record: %v %q", ok, why)
	}
	reviewtest.Pass(t, root, n, "text", text, claims)
	if ok, why := review.Check(root, n, "text", text); !ok {
		t.Fatalf("a passing review does not count: %s", why)
	}
	if k := review.Key(root, n, lean); k != "/lean/AI/Dropout.lean" {
		t.Errorf("the key of a Lean module is %q", k)
	}
	reviewtest.Pass(t, root, n, "lean-statement", lean)
	if ok, why := review.Check(root, n, "lean-statement", lean); !ok {
		t.Errorf("a review of a Lean module does not count: %s", why)
	}
	edit := func(f func(r *review.Record)) {
		t.Helper()
		reviewtest.Pass(t, root, n, "text", text, claims)
		r, err := review.Load(root, n.Tag, "text")
		if err != nil {
			t.Fatal(err)
		}
		f(r)
		b, _ := yaml.Marshal(r)
		if err := os.WriteFile(review.File(root, n.Tag, "text"), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for want, f := range map[string]func(r *review.Record){
		"verdict is \"changes\"":          func(r *review.Record) { r.Verdict = "changes" },
		"an older version of claims.yaml": func(r *review.Record) { r.Hashes["claims.yaml"] = "b3:00" },
		"does not hash node.md":           func(r *review.Record) { delete(r.Hashes, "node.md") },
		"item 1 of the checklist is \"fail\"": func(r *review.Record) {
			r.Items[0].Result = "fail"
		},
		"does not answer item 1":         func(r *review.Record) { r.Items = nil },
		"is not in registers":            func(r *review.Record) { r.Reviewer = "github:someone" },
		"states no interests":            func(r *review.Record) { r.Interests = "" },
		"the checklist text/v9 does not": func(r *review.Record) { r.Checklist = "text/v9" },
		"not 2NT5ZY text":                func(r *review.Record) { r.Part = "facts" },
	} {
		edit(f)
		if ok, why := review.Check(root, n, "text", text); ok || !strings.Contains(why, want) {
			t.Errorf("%s: %v %q", want, ok, why)
		}
	}
	// A reviewer without the skill of the part, or who wrote the node.
	reviewtest.Pass(t, root, n, "text", text, claims)
	reg := []review.Reviewer{{ID: reviewtest.Reviewer, Skills: []string{"code:pytorch"}, Key: "k"}}
	b, _ := yaml.Marshal(reg)
	if err := os.WriteFile(filepath.Join(root, review.Register), b, 0o644); err != nil {
		t.Fatal(err)
	}
	if ok, why := review.Check(root, n, "text", text); ok || !strings.Contains(why, "none of the skills domain:deep-learning-training") {
		t.Errorf("no skill: %v %q", ok, why)
	}
	reviewtest.Pass(t, root, n, "text", text, claims)
	if err := os.MkdirAll(filepath.Join(root, "prov"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "prov", n.Tag+".jsonl"), []byte(`{"author":"reviewer","date":"2026-11-13","file":"claims.yaml","op":"create"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if ok, why := review.Check(root, n, "text", text); ok || !strings.Contains(why, "is an author of claims.yaml") {
		t.Errorf("an author: %v %q", ok, why)
	}
}

// The template hashes the files of the part and asks each question of the newest checklist of the kind.
func TestTemplate(t *testing.T) {
	root := t.TempDir()
	n := &node.Node{Slug: "dropout", Tag: "2NT5ZY", Kind: "method", Rel: "nodes/dl/dropout"}
	dir := filepath.Join(root, filepath.FromSlash(n.Rel))
	for f, s := range map[string]string{
		filepath.Join(dir, "node.md"): "# Dropout\n", filepath.Join(dir, "claims.yaml"): "- id: c\n",
		filepath.Join(dir, "impl", "numpy", "dropout.py"): "x = 1\n", filepath.Join(dir, "impl", "numpy", "__pycache__", "d.pyc"): "",
		filepath.Join(root, "checklists", "text", "method", "v1.md"): "1. Old.\n",
		filepath.Join(root, "checklists", "text", "method", "v2.md"): "Read the page.\n\n1. The lead says what it is.\n2. The claims are scoped.\n",
	} {
		if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if cl := review.Latest(root, n, "text"); cl != "text/method/v2" {
		t.Errorf("the newest checklist is %q", cl)
	}
	out, err := review.Template(root, n, "text", "github:alice", "text/method/v2", "2026-11-14")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"node.md": b3:`, `"claims.yaml": b3:`, `# 2. The claims are scoped.`, `- {check: "2", result: ""}`, "verdict: changes"} {
		if !strings.Contains(out, want) {
			t.Errorf("the template has no %q:\n%s", want, out)
		}
	}
	var r review.Record
	if err := yaml.Unmarshal([]byte(out), &r); err != nil || len(r.Items) != 2 || len(r.Hashes) != 2 {
		t.Errorf("the template parses to %+v (%v)", r, err)
	}
	fs, err := review.Files(root, n, "impl-numpy")
	if err != nil || len(fs) != 1 || filepath.Base(fs[0]) != "dropout.py" {
		t.Errorf("the files of impl-numpy are %v (%v)", fs, err)
	}
	if _, err := review.Files(root, n, "impl-jax"); err == nil || !strings.Contains(err.Error(), "no files for the part impl-jax") {
		t.Errorf("a part with no files: %v", err)
	}
}
