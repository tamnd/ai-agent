// SPDX-License-Identifier: Apache-2.0

package exp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const design = `id: mlp-toy
claims: [c-dropout-reduces-overfitting]
question: "Does dropout lower the test loss of a small MLP?"
scale_note:
  here: "A 1-hidden-layer MLP on 600 points of two moons."
  source: "Srivastava et al. (2014) report it on MNIST with larger nets."
replicates: null
arms:
  - {id: no-dropout, config: {p: 0.0}}
  - {id: dropout, config: {p: 0.5}}
base_config:
  model: {hidden: [64], activation: relu}
  epochs: 30
data:
  - {generator: moons.py}
seeds: {n: 10, base: 20261113}
budget: cpu-small
framework: numpy
metrics: [test_loss, train_loss]
`

const expect = `claim: c-dropout-reduces-overfitting
primary:
  metric: test_loss
  at: final
  compare: {treatment: dropout, control: no-dropout}
  direction: lower
  test: welch-t
  alpha: 0.01
  min_effect: {kind: relative, value: 0.05}
secondary:
  - {metric: train_loss, direction: higher, test: bootstrap-diff, compare: {treatment: dropout, control: no-dropout}}
report: {ci: 0.95, bootstrap: {resamples: 2000}}
`

func writeExp(t *testing.T, files map[string]string) (string, *Experiment) {
	t.Helper()
	nodeDir := t.TempDir()
	for name, s := range files {
		p := filepath.Join(nodeDir, Dir, "mlp-toy", name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	es, err := Read(nodeDir, "nodes/a/b/dropout")
	if err != nil || len(es) != 1 {
		t.Fatalf("Read gives %d experiments and %v", len(es), err)
	}
	return nodeDir, es[0]
}

func TestRead(t *testing.T) {
	_, e := writeExp(t, map[string]string{DesignFile: design, ExpectFile: expect, Script: "print(1)\n", "moons.py": "\n"})
	if len(e.Errs) > 0 {
		t.Fatal(e.Errs)
	}
	if e.Rel != "nodes/a/b/dropout/exp/mlp-toy" || !e.HasScript || e.Design.ID != "mlp-toy" || len(e.Expect.Primary) != 1 || len(e.Expect.Secondary) != 1 {
		t.Fatalf("the experiment reads as %+v", e)
	}
	cfg := e.Design.Config(e.Design.Arm("dropout"))
	if v, _ := Lookup(cfg, "model.activation"); v != "relu" || cfg["p"] != 0.5 {
		t.Errorf("the merged configuration is %v", cfg)
	}
	if got := e.Inputs(); strings.Join(got, ",") != "experiment.yaml,moons.py,run.py" {
		t.Errorf("the inputs are %v", got)
	}
	if jobs := e.Design.Jobs(); len(jobs) != 20 || jobs[0].Seed != 20261113 || jobs[1].Arm != "dropout" || jobs[19].Seed != 20261122 {
		t.Errorf("the jobs are %v", jobs)
	}

	_, bad := writeExp(t, map[string]string{DesignFile: design + "colour: red\n", ResultsFile: "{"})
	if len(bad.Errs) != 2 || bad.Design != nil {
		t.Errorf("an unknown field and a bad results.json give %v", bad.Errs)
	}
}

func TestDecide(t *testing.T) {
	_, e := writeExp(t, map[string]string{DesignFile: design, ExpectFile: expect})
	v := Values{
		"dropout":    {"test_loss": treatment, "train_loss": control},
		"no-dropout": {"test_loss": control, "train_loss": treatment},
	}
	p, s, verdict := Decide(e.Design, e.Expect, v)
	if verdict != Holds || len(p) != 1 || !p[0].Passed || *p[0].EffectRelative > -0.3 {
		t.Fatalf("the primary test gives %s, %+v", verdict, p)
	}
	if len(s) != 1 || !s[0].Passed {
		t.Errorf("the secondary test gives %+v", s)
	}

	// The same values with the arms swapped: dropout raises the loss, so the claim did not hold.
	v["dropout"]["test_loss"], v["no-dropout"]["test_loss"] = control, treatment
	if _, _, verdict := Decide(e.Design, e.Expect, v); verdict != DidNotHold {
		t.Errorf("a loss in the wrong direction gives %s", verdict)
	}

	// A small effect with noise: the test does not pass, but the data allow the claimed effect.
	v["dropout"]["test_loss"] = []float64{0.150, 0.162, 0.139, 0.158, 0.141}
	v["no-dropout"]["test_loss"] = []float64{0.153, 0.160, 0.149, 0.156, 0.158}
	if p, _, verdict := Decide(e.Design, e.Expect, v); verdict != Inconclusive || p[0].Passed {
		t.Errorf("a weak test gives %s, %+v", verdict, p)
	}
}

func TestDecideOtherTests(t *testing.T) {
	d := &Design{Seeds: Seeds{N: 10, Base: 1}}
	for _, w := range []float64{1e3, 3e3, 1e4, 3e4, 1e5} {
		d.Arms = append(d.Arms, Arm{ID: fmt.Sprint(w), Config: map[string]any{"model": map[string]any{"params": w}}})
	}
	v := Values{}
	for i, y := range []float64{5.0, 4.1, 3.4, 2.8, 2.3} {
		v[d.Arms[i].ID] = map[string][]float64{"loss": {y * 0.99, y, y * 1.01}}
	}
	fit := &Expect{Primary: Tests{{Metric: "loss", Test: "fit", X: "model.params", R2Min: 0.98, Range: []float64{-0.2, -0.1}}}}
	if p, _, verdict := Decide(d, fit, v); verdict != Holds {
		t.Errorf("the power law gives %s, %+v", verdict, p)
	}
	fit.Primary[0].Range = []float64{-0.5, -0.4}
	if _, _, verdict := Decide(d, fit, v); verdict != DidNotHold {
		t.Errorf("an exponent outside the range gives %s", verdict)
	}

	d.Arms = []Arm{{ID: "agent"}}
	rate := &Expect{Primary: Tests{{Metric: "hacked", Test: "rate", Arm: "agent", Rate: 0.3, Alpha: 0.05}}}
	v = Values{"agent": {"hacked": {1, 1, 1, 1, 1, 1, 1, 1, 1, 0}}}
	if p, _, verdict := Decide(d, rate, v); verdict != Holds {
		t.Errorf("9 of 10 against 0.3 gives %s, %+v", verdict, p)
	}
	v["agent"]["hacked"] = []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	if _, _, verdict := Decide(d, rate, v); verdict != DidNotHold {
		t.Errorf("0 of 10 against 0.3 gives %s", verdict)
	}

	d.Arms = []Arm{{ID: "sgd-l2"}, {ID: "sgd-wd"}}
	tost := &Expect{Primary: Tests{{Metric: "loss", Test: "tost", Compare: &Compare{Treatment: "sgd-wd", Control: "sgd-l2"}, Margin: &Effect{Kind: "absolute", Value: 0.01}, Alpha: 0.05}}}
	v = Values{"sgd-l2": {"loss": {0.300, 0.302, 0.298, 0.301, 0.299}}, "sgd-wd": {"loss": {0.301, 0.299, 0.300, 0.302, 0.298}}}
	if p, _, verdict := Decide(d, tost, v); verdict != Holds {
		t.Errorf("two equal arms give %s, %+v", verdict, p)
	}

	shape := &Expect{Primary: Tests{{Metric: "loss", Test: "shape"}}}
	if p, _, verdict := Decide(d, shape, v); verdict != Inconclusive || !strings.Contains(p[0].Note, "shape") {
		t.Errorf("the test shape gives %s, %+v", verdict, p)
	}

	two := &Expect{Primary: Tests{
		{Metric: "loss", Test: "welch-t", Compare: &Compare{Treatment: "sgd-wd", Control: "sgd-l2"}, Direction: "lower", Alpha: 0.05},
		{Metric: "loss", Test: "tost", Compare: &Compare{Treatment: "sgd-wd", Control: "sgd-l2"}, Margin: &Effect{Kind: "absolute", Value: 0.01}, Alpha: 0.05},
	}}
	p, _, _ := Decide(d, two, v)
	if p[1].Raw == nil || *p[1].P < *p[1].Raw {
		t.Errorf("the correction of Holm does not adjust the p-values: %+v", p)
	}
}

// fakeRun writes metrics.json as run.py does: the loss goes down by a fixed amount with dropout, plus a small noise from the seed.
func fakeRun(_ context.Context, j Job, dir string) error {
	noise := float64(j.Seed%7) * 0.001
	m := Metrics{Final: map[string]float64{"test_loss": 0.15 + noise, "train_loss": 0.05 + noise}, Device: "cpu"}
	if j.Config["p"] == 0.5 {
		m.Final["test_loss"] = 0.10 + noise
		m.Final["train_loss"] = 0.08 + noise
	}
	b, _ := json.Marshal(m)
	return os.WriteFile(filepath.Join(dir, MetricsFile), b, 0o644)
}

func TestRun(t *testing.T) {
	_, e := writeExp(t, map[string]string{DesignFile: design, ExpectFile: expect, Script: "print(1)\n", "moons.py": "\n", "figures/loss.svg": "<svg/>"})
	now := time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)
	clock := func() time.Time { now = now.Add(time.Second); return now }
	r, err := Run(context.Background(), e, Options{Work: t.TempDir(), Exec: fakeRun, Node: "3KD9QX", Image: "local", Now: clock})
	if err != nil {
		t.Fatal(err)
	}
	if r.Verdict != Holds || r.Run != "r-2026-11-13-"+strings.TrimPrefix(r.InputsHash, "sha256:")[:6] || len(r.Seeds) != 10 || r.Hardware.Class != "cpu-small" || r.Hardware.Device != "cpu" {
		t.Fatalf("the run gives %+v", r)
	}
	if got := r.Arms["dropout"]["test_loss"]; len(got.PerSeed) != 10 || got.Mean > 0.11 {
		t.Errorf("the dropout arm is %+v", got)
	}
	if len(r.Figures) != 1 || r.Figures[0] != "figures/loss.svg" {
		t.Errorf("the figures are %v", r.Figures)
	}
	if err := Write(e, r); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(e.Dir, ResultsFile))
	if !strings.Contains(string(b), `"primary": {`) {
		t.Errorf("results.json does not write one primary test as an object:\n%s", b)
	}
	var back Results
	if err := json.Unmarshal(b, &back); err != nil || len(back.Primary) != 1 {
		t.Fatalf("results.json reads back as %+v, %v", back, err)
	}
	e.Results = &back
	if s := e.Stale(); s != "" {
		t.Errorf("fresh results are stale: %s", s)
	}
	if err := os.WriteFile(filepath.Join(e.Dir, "moons.py"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if s := e.Stale(); s != "the inputs changed after the run" {
		t.Errorf("a changed generator gives %q", s)
	}
	if err := os.WriteFile(filepath.Join(e.Dir, ExpectFile), []byte(expect+"correction: none\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if s := e.Stale(); s != "expect.yaml changed after the run" {
		t.Errorf("a changed expect.yaml gives %q", s)
	}
}

func TestRunErrors(t *testing.T) {
	_, e := writeExp(t, map[string]string{DesignFile: design, Script: "\n"})
	if _, err := Run(context.Background(), e, Options{Work: t.TempDir(), Exec: fakeRun}); err == nil || !strings.Contains(err.Error(), "R06") {
		t.Errorf("a run with no expect.yaml gives %v", err)
	}
	_, e = writeExp(t, map[string]string{DesignFile: design, ExpectFile: expect, Script: "\n"})
	none := func(context.Context, Job, string) error { return nil }
	if _, err := Run(context.Background(), e, Options{Work: t.TempDir(), Exec: none}); err == nil || !strings.Contains(err.Error(), "wrote no metrics.json") {
		t.Errorf("a script with no metrics gives %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	slow := func(ctx context.Context, _ Job, _ string) error { cancel(); <-ctx.Done(); return ctx.Err() }
	if _, err := Run(ctx, e, Options{Work: t.TempDir(), Exec: slow}); !errors.Is(err, ErrBudget) {
		t.Errorf("a run that is stopped gives %v", err)
	}
}

func TestPins(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "pins.yaml")
	if err := os.WriteFile(p, []byte("- {id: mnist-ai-data, dataset: mnist, version: lecun-1998, licence: {spdx: CC-BY-SA-3.0}, mirror: allowed, files: [{name: a.gz, sha256: abc}]}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pins, err := LoadPins(p)
	if err != nil || pins["mnist-ai-data"] == nil || pins["mnist-ai-data"].Files[0].SHA256 != "abc" {
		t.Fatalf("LoadPins gives %v, %v", pins, err)
	}
	if PinID(Data{Pin: "S:mnist-ai-data"}) != "mnist-ai-data" {
		t.Error("PinID keeps the prefix S:")
	}
	if pins, err := LoadPins(filepath.Join(dir, "none.yaml")); pins != nil || err != nil {
		t.Errorf("a register that is not there gives %v, %v", pins, err)
	}
}
