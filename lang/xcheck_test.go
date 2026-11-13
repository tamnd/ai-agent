// SPDX-License-Identifier: Apache-2.0

package lang

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/tamnd/ai-agent/node"
	"github.com/tamnd/ai-agent/status"
)

func TestJUnitNames(t *testing.T) {
	b := []byte(`<?xml version="1.0"?><testsuites><testsuite name="pytest">
<testcase classname="test_x" name="test_shapes[base]"/>
<testcase classname="test_x" name="test_shapes[big]"><failure message="boom"/></testcase>
<testcase classname="test_x" name="test_gradient"/>
<testcase classname="test_x" name="test_gpu"><skipped/></testcase>
<testcase classname="test_x" name="test_load"><error message="no file"/></testcase>
</testsuite></testsuites>`)
	passed, failed, err := junitNames(b)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(passed, []string{"test_gradient"}) || !slices.Equal(failed, []string{"test_load", "test_shapes"}) {
		t.Errorf("passed %v, failed %v", passed, failed)
	}
}

func TestFrameworksAgree(t *testing.T) {
	for fw := range Frameworks {
		if !status.Frameworks[fw] {
			t.Errorf("%s is in lang.Frameworks but not in status.Frameworks", fw)
		}
	}
	for fw := range status.Frameworks {
		if Frameworks[fw] == "" {
			t.Errorf("%s is in status.Frameworks but not in lang.Frameworks", fw)
		}
	}
}

func TestLintFirst(t *testing.T) {
	in := "/t/a.py\n  /t/a.py:3:1 - error: bad\n1 error"
	if got := lintFirst(in); !strings.HasPrefix(got, "/t/a.py:3:1 - error: bad\n") {
		t.Errorf("got %q", got)
	}
}

// TestVectors runs the harness on a toy metric: one vector passes, one fails by more than the tolerance, and one
// raises.
func TestVectors(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("no python3")
	}
	nd := t.TempDir()
	files := map[string]string{
		"contract.yaml": "inputs:\n  predictions: {type: \"list[int]\", meaning: p}\n  references: {type: \"list[int]\", meaning: r}\noptions:\n  scale: {type: float, default: 1, meaning: s}\noutput:\n  acc: {type: float, meaning: a}\n  n: {type: int, meaning: n}\ntolerance: {rtol: 1.0e-9, atol: 0}\n",
		"vectors/official.jsonl": `{"id": "half", "predictions": [1, 2], "references": [1, 3], "want": {"acc": 0.5, "n": 2}, "by": "toy 1.0"}` + "\n" +
			`{"id": "scaled", "predictions": [1], "references": [1], "options": {"scale": 2}, "want": {"acc": 1.0, "n": 1}, "by": "toy 1.0"}` + "\n" +
			`{"id": "empty", "predictions": [], "references": [], "want": {"acc": 0, "n": 0}, "by": "toy 1.0"}` + "\n",
	}
	for name, s := range files {
		p := filepath.Join(nd, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	src := "def score(predictions, references, scale=1):\n    return {\"acc\": scale * sum(p == r for p, r in zip(predictions, references)) / len(predictions), \"n\": len(predictions)}\n"
	n := &node.Node{Dir: nd}
	n.Slug, n.Kind = "toy", "metric"
	im := node.Impl{Lang: "numpy", Files: []node.File{{Name: "toy.py", Content: src}}}
	// The run copies the vectors into the work dir with the fixtures (Shared); the test does the same.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "toy.py"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.CopyFS(filepath.Join(dir, "vectors"), os.DirFS(filepath.Join(nd, "vectors"))); err != nil {
		t.Fatal(err)
	}
	v := (&Runner{}).vectors(t.Context(), dir, n, im)
	if v.Error != "" || v.Cases != 3 || v.Passed != 1 || len(v.Failures) != 2 {
		t.Fatalf("%+v", v)
	}
	if f := v.Failures[0]; f.Case != "official.jsonl:scaled" || !strings.Contains(f.Reason, "acc: got 2.0, want 1.0") {
		t.Errorf("the first failure: %+v", f)
	}
	if f := v.Failures[1]; f.Case != "official.jsonl:empty" || !strings.Contains(f.Reason, "ZeroDivisionError") {
		t.Errorf("the second failure: %+v", f)
	}
}
