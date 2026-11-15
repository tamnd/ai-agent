// SPDX-License-Identifier: Apache-2.0

package exp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// verifiedRun runs the toy experiment, writes results.json and reads it back, as aik exp run -update does.
func verifiedRun(t *testing.T) (*Experiment, string) {
	t.Helper()
	_, e := writeExp(t, map[string]string{DesignFile: design, ExpectFile: expect, Script: "print(1)\n", "moons.py": "\n", "figures/loss.svg": "<svg/>"})
	work := t.TempDir()
	r, err := Run(context.Background(), e, Options{Work: work, Exec: fakeRun, Node: "3KD9QX", Image: "local"})
	if err != nil {
		t.Fatal(err)
	}
	if err := Write(e, r); err != nil {
		t.Fatal(err)
	}
	back, err := ReadResults(filepath.Join(e.Dir, ResultsFile))
	if err != nil {
		t.Fatal(err)
	}
	e.Results = back
	return e, work
}

func TestVerify(t *testing.T) {
	e, work := verifiedRun(t)
	if v := Verify("dropout/mlp-toy", e, work); !v.OK(true) || v.Outputs != work || v.Run == "" {
		t.Fatalf("a fresh run: %+v", v)
	}
	// A value that the run did not write, with its mean and SD made to agree, so that only the outputs show it.
	s := e.Results.Arms["dropout"]["train_loss"]
	s.PerSeed = append([]float64(nil), s.PerSeed...)
	s.PerSeed[3] += 0.5
	sum := Summarize(s.PerSeed)
	s.Mean, s.SD = sum.Mean, sum.SD
	e.Results.Arms["dropout"]["train_loss"] = s
	v := Verify("dropout/mlp-toy", e, "")
	if len(v.Problems) != 1 || !strings.Contains(v.Problems[0], "the tests of expect.yaml give") {
		t.Errorf("an edited value changes the secondary test: %+v", v)
	}
	e, work = verifiedRun(t)
	s = e.Results.Arms["dropout"]["test_loss"]
	s.Mean += 0.01
	e.Results.Arms["dropout"]["test_loss"] = s
	e.Results.Verdict = DidNotHold
	e.Results.Figures = append(e.Results.Figures, "figures/none.svg")
	v = Verify("dropout/mlp-toy", e, work)
	want := []string{"dropout: test_loss has the mean", "figures/none.svg is not there"}
	if len(v.Problems) != len(want) || v.Outputs != "" {
		t.Fatalf("%+v", v)
	}
	for i, w := range want {
		if !strings.Contains(v.Problems[i], w) {
			t.Errorf("problem %d is %q, not %q", i, v.Problems[i], w)
		}
	}
	// The verdict alone.
	e, work = verifiedRun(t)
	e.Results.Verdict = Inconclusive
	if v := Verify("dropout/mlp-toy", e, work); len(v.Problems) != 1 || !strings.Contains(v.Problems[0], "the verdict is inconclusive, but the tests of expect.yaml give holds") {
		t.Errorf("%+v", v)
	}
	// The outputs of the run differ: a seed that wrote another value, and one that wrote nothing.
	e, work = verifiedRun(t)
	if err := os.WriteFile(filepath.Join(work, "dropout", "20261115", MetricsFile), []byte(`{"final": {"test_loss": 9, "train_loss": 0.08}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(work, "no-dropout", "20261113", MetricsFile)); err != nil {
		t.Fatal(err)
	}
	v = Verify("dropout/mlp-toy", e, work)
	if len(v.Problems) != 2 || !strings.Contains(v.Problems[0], "arm no-dropout, seed 20261113: run.py wrote no metrics.json") ||
		!strings.Contains(v.Problems[1], "arm dropout, seed 20261115: test_loss is 0.1 in results.json, but 9 in the run") {
		t.Errorf("%+v", v.Problems)
	}
	// Changed inputs: stale, which only -strict fails, and the tests are not decided again.
	e, _ = verifiedRun(t)
	if err := os.WriteFile(filepath.Join(e.Dir, Script), []byte("print(2)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	e.Results.Verdict = Inconclusive
	if v := Verify("dropout/mlp-toy", e, ""); v.Stale != "the inputs changed after the run" || !v.OK(false) || v.OK(true) {
		t.Errorf("%+v", v)
	}
	e.Results = nil
	if v := Verify("dropout/mlp-toy", e, ""); v.OK(false) {
		t.Errorf("no results: %+v", v)
	}
}
