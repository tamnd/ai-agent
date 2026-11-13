// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/tamnd/ai-agent/exp"
)

// The run.py of the test needs only the standard library. The loss goes down with dropout, plus a noise from the seed.
const toyRun = `import argparse, json, os, random
ap = argparse.ArgumentParser()
ap.add_argument("--job")
ap.add_argument("--out")
a = ap.parse_args()
job = json.load(open(a.job))
rng = random.Random(job["seed"])
p = job["config"]["p"]
loss = 0.15 - 0.05 * (p / 0.5) + rng.gauss(0, 0.004)
json.dump({"final": {"test_loss": loss}, "device": "cpu"}, open(os.path.join(a.out, "metrics.json"), "w"))
`

const toyDesign = `id: toy-dropout
claims: [c-fits]
question: "Does the toy loss go down with dropout?"
scale_note: {here: "A formula with noise.", source: "The source reports it on MNIST."}
arms:
  - {id: none, config: {p: 0.0}}
  - {id: dropout, config: {p: 0.5}}
seeds: {n: 4, base: 100}
budget: cpu-small
metrics: [test_loss]
`

const toyExpect = `claim: c-fits
primary: {metric: test_loss, compare: {treatment: dropout, control: none}, direction: lower, test: welch-t, alpha: 0.01, min_effect: {kind: relative, value: 0.1}}
`

func TestExpRun(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the test runs python3")
	}
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("no python3")
	}
	root := copyCorpus(t)
	dir := filepath.Join(root, "nodes/foundations/models-of-computation/comparison-model/exp/toy-dropout")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, s := range map[string]string{exp.DesignFile: toyDesign, exp.ExpectFile: toyExpect, exp.Script: toyRun} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if got := run([]string{"exp", "run", "-root", root, "-unsafe-local", "comparison-model"}); got != 0 {
		t.Fatalf("aik exp run exits with %d", got)
	}
	if _, err := os.Stat(filepath.Join(dir, exp.ResultsFile)); err == nil {
		t.Fatal("aik exp run with no -update wrote results.json")
	}
	if got := run([]string{"exp", "run", "-root", root, "-unsafe-local", "-update", "comparison-model/toy-dropout"}); got != 0 {
		t.Fatalf("aik exp run -update exits with %d", got)
	}
	b, err := os.ReadFile(filepath.Join(dir, exp.ResultsFile))
	if err != nil {
		t.Fatal(err)
	}
	var r exp.Results
	if err := json.Unmarshal(b, &r); err != nil {
		t.Fatal(err)
	}
	if r.Verdict != exp.Holds || r.Node != "9CMPMD" || len(r.Arms["dropout"]["test_loss"].PerSeed) != 4 {
		t.Errorf("results.json is %s", b)
	}
	if got := run([]string{"exp", "list", "-root", root}); got != 0 {
		t.Errorf("aik exp list exits with %d", got)
	}
	if got := run([]string{"exp", "run", "-root", root, "-unsafe-local", "comparison-model/none"}); got != 2 {
		t.Errorf("an experiment that is not there exits with %d, not 2", got)
	}
}
