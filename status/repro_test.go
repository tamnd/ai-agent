// SPDX-License-Identifier: Apache-2.0

package status

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	claimpkg "github.com/tamnd/ai-agent/claims"
	"github.com/tamnd/ai-agent/exp"
	"github.com/tamnd/ai-agent/internal/reviewtest"
	"github.com/tamnd/ai-agent/node"
)

const reproDesign = `id: %ID%
claims: [c-d]
question: "Does dropout lower the test loss of a small MLP?"
scale_note: {here: "%HERE%", source: "The source reports it on MNIST."}
arms:
  - {id: none, config: {p: 0.0}}
  - {id: dropout, config: {p: 0.5}}
data:
  - {generator: moons.py}
seeds: {n: 6, base: 7}
budget: cpu-small
metrics: [test_loss]
`

const reproExpect = `claim: c-d
primary: {metric: test_loss, compare: {treatment: dropout, control: none}, direction: lower, test: welch-t, alpha: 0.01, min_effect: {kind: relative, value: 0.1}}
`

// fakeRun gives a test loss that is lower with dropout, plus a small noise from the seed.
func fakeRun(_ context.Context, j exp.Job, dir string) error {
	loss := 0.15 + float64(j.Seed%5)*0.001
	if j.Config["p"] == 0.5 {
		loss -= 0.05
	}
	b, _ := json.Marshal(exp.Metrics{Final: map[string]float64{"test_loss": loss}, Device: "cpu"})
	return os.WriteFile(filepath.Join(dir, exp.MetricsFile), b, 0o644)
}

func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, s := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestReproLadder(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "nodes", "dropout")
	env := Env{Corpus: &node.Corpus{Root: root}}
	n := &node.Node{Dir: dir, Front: node.Front{Kind: "method", Tag: "3KD9QX"}}
	if got := Repro(n, env); got != nil {
		t.Fatalf("a node with no experiment and no small-scale claim has the R ladder: %+v", got)
	}
	n.Claims = []claimpkg.Claim{{ID: "c-d", Type: claimpkg.Empirical, Text: "Dropout lowers the test loss.", SmallScale: "yes"}}
	level := func(want int, next string) {
		t.Helper()
		r := Repro(n, env)
		if r == nil || r.Level != want || r.Label != "R"+string(rune('0'+want)) {
			t.Fatalf("level %+v, want R%d", r, want)
		}
		if next != "" && !strings.Contains(strings.Join(r.Next, "; "), next) {
			t.Errorf("R%d next is %q, want %q", want, r.Next, next)
		}
	}
	read := func() {
		t.Helper()
		es, err := exp.Read(dir, "nodes/dropout")
		if err != nil {
			t.Fatal(err)
		}
		n.Exps = es
	}
	add := func(id, here string) {
		t.Helper()
		d := strings.NewReplacer("%ID%", id, "%HERE%", here).Replace(reproDesign)
		writeFiles(t, filepath.Join(dir, exp.Dir, id), map[string]string{exp.DesignFile: d, exp.ExpectFile: reproExpect, exp.Script: "\n", "moons.py": "\n"})
	}
	runAll := func() {
		t.Helper()
		read()
		for _, e := range n.Exps {
			r, err := exp.Run(context.Background(), e, exp.Options{Work: t.TempDir(), Exec: fakeRun, Node: n.Tag, Image: "local"})
			if err != nil {
				t.Fatal(err)
			}
			if err := exp.Write(e, r); err != nil {
				t.Fatal(err)
			}
		}
		read()
	}
	level(0, "write an experiment for the claim c-d")
	add("mlp-toy", "A small MLP on two moons.")
	read()
	level(0, "(R05)")
	if m := Measured(n, env)["c-d"]; m[0] || m[1] {
		t.Errorf("an experiment that has not run gives %v", m)
	}
	runAll()
	level(2, "(R13)")
	if m := Measured(n, env)["c-d"]; !m[0] || m[1] {
		t.Errorf("an experiment at R2 gives %v", m)
	}
	writeFiles(t, filepath.Join(dir, exp.Dir, "mlp-toy"), map[string]string{"moons.py": "changed\n"})
	level(1, "the inputs changed after the run")
	add("mlp-wide", "A wider MLP on two moons.")
	runAll()
	level(3, "reviews the design and the results")
	for _, e := range n.Exps {
		reviewtest.Pass(t, root, n, "exp/"+e.Slug, filepath.Join(e.Dir, exp.ResultsFile))
	}
	level(4, "")
	if r := Repro(n, env); r.Name != "reviewed experiment" || r.Next != nil {
		t.Errorf("R4 is %+v", r)
	}
}

func TestReproSameScale(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "nodes", "dropout")
	for _, id := range []string{"a", "b"} {
		d := strings.NewReplacer("%ID%", id, "%HERE%", "The same scale.").Replace(reproDesign)
		writeFiles(t, filepath.Join(dir, exp.Dir, id), map[string]string{exp.DesignFile: d, exp.ExpectFile: reproExpect, exp.Script: "\n", "moons.py": "\n"})
	}
	es, err := exp.Read(dir, "nodes/dropout")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range es {
		r, err := exp.Run(context.Background(), e, exp.Options{Work: t.TempDir(), Exec: fakeRun, Image: "local"})
		if err != nil {
			t.Fatal(err)
		}
		e.Results = r
	}
	n := &node.Node{Dir: dir, Exps: es, Claims: []claimpkg.Claim{{ID: "c-d", Type: claimpkg.Empirical}}}
	if r := Repro(n, Env{Corpus: &node.Corpus{Root: root}}); r.Level != 2 {
		t.Errorf("two experiments at the same scale on the same data give %s, not R2", r.Label)
	}
}
