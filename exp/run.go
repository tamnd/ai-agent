// SPDX-License-Identifier: Apache-2.0

package exp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Job is one run of run.py: one arm with one seed.
type Job struct {
	Arm    string         `json:"arm"`
	Seed   int64          `json:"seed"`
	Config map[string]any `json:"config"`
	// Data gives the directory of each pinned dataset, by pin id. The directory is read-only in the sandbox.
	Data map[string]string `json:"data,omitempty"`
}

// Jobs gives the jobs of the design: every arm with every seed, in the order of the seeds, so that a run that stops early has the same number of seeds in each arm.
func (d *Design) Jobs() []Job {
	var out []Job
	for _, s := range d.Seeds.List() {
		for i := range d.Arms {
			a := &d.Arms[i]
			out = append(out, Job{Arm: a.ID, Seed: s, Config: d.Config(a)})
		}
	}
	return out
}

// The files of one job directory. aik writes job.json. run.py reads it and writes metrics.json.
const (
	JobFile     = "job.json"
	MetricsFile = "metrics.json"
)

// Metrics is the file metrics.json that run.py writes for one job.
type Metrics struct {
	// Final holds the value of each metric at the end of the run.
	Final map[string]float64 `json:"final"`
	// Device names the device that the run used, such as cpu.
	Device string `json:"device,omitempty"`
}

// Exec runs run.py for one job. dir is the job directory: it holds job.json, and run.py writes metrics.json in it. The caller decides where the script runs, such as in the sandbox.
type Exec func(ctx context.Context, j Job, dir string) error

// Options set a run of an experiment.
type Options struct {
	// Work is the directory of the job directories.
	Work string
	Exec Exec
	// Node is the tag of the node.
	Node string
	// Image is the image of the run, pinned by digest, or "local".
	Image    string
	Hardware Hardware
	Now      func() time.Time
	Log      func(format string, a ...any)
	// Data gives the directory of each pinned dataset, by pin id, from Prepare. Each job gets it in job.json.
	Data map[string]string
}

// ErrBudget means that the run went past the cap of its budget class (rule R09).
var ErrBudget = errors.New("the run went past the cap of its budget class")

// Ready tells why the experiment cannot run, or gives nil.
func (e *Experiment) Ready() error {
	switch {
	case len(e.Errs) > 0:
		return e.Errs[0]
	case e.Design == nil:
		return fmt.Errorf("%s has no %s", e.Rel, DesignFile)
	case e.Expect == nil:
		return fmt.Errorf("%s has no %s; write the test before the first full run (rule R06)", e.Rel, ExpectFile)
	case !e.HasScript:
		return fmt.Errorf("%s has no %s", e.Rel, Script)
	case len(e.Design.Arms) == 0 || e.Design.Seeds.N < 1:
		return fmt.Errorf("%s: the design needs at least one arm and one seed", e.Rel)
	}
	if _, ok := Budgets[e.Design.Budget]; !ok {
		return fmt.Errorf("%s: the budget %q is not a class of spec 9.4.1", e.Rel, e.Design.Budget)
	}
	return nil
}

// Run runs every job of the experiment and decides its claim. It stops the run and returns ErrBudget when the run goes past the cap of the budget class.
func Run(ctx context.Context, e *Experiment, o Options) (*Results, error) {
	if err := e.Ready(); err != nil {
		return nil, err
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.Log == nil {
		o.Log = func(string, ...any) {}
	}
	d := e.Design
	b := Budgets[d.Budget]
	ctx, cancel := context.WithTimeout(ctx, time.Duration(b.Seconds)*time.Second)
	defer cancel()
	start := o.Now()
	values := Values{}
	device := ""
	for _, j := range d.Jobs() {
		j.Data = o.Data
		dir := filepath.Join(o.Work, j.Arm, fmt.Sprint(j.Seed))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
		jb, err := json.MarshalIndent(j, "", "  ")
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(filepath.Join(dir, JobFile), jb, 0o644); err != nil {
			return nil, err
		}
		_ = os.Remove(filepath.Join(dir, MetricsFile))
		o.Log("run %s arm %s seed %d", e.Slug, j.Arm, j.Seed)
		err = o.Exec(ctx, j, dir)
		if ctx.Err() != nil {
			return nil, fmt.Errorf("%s: %w: %s allows %d seconds", e.Rel, ErrBudget, d.Budget, b.Seconds)
		}
		if err != nil {
			return nil, fmt.Errorf("%s: arm %s, seed %d: %w", e.Rel, j.Arm, j.Seed, err)
		}
		m, err := readMetrics(filepath.Join(dir, MetricsFile))
		if err != nil {
			return nil, fmt.Errorf("%s: arm %s, seed %d: %w", e.Rel, j.Arm, j.Seed, err)
		}
		if device == "" {
			device = m.Device
		}
		if values[j.Arm] == nil {
			values[j.Arm] = map[string][]float64{}
		}
		for _, name := range d.Metrics {
			v, ok := m.Final[name]
			if !ok {
				return nil, fmt.Errorf("%s: arm %s, seed %d: %s has no final value of the metric %s", e.Rel, j.Arm, j.Seed, MetricsFile, name)
			}
			values[j.Arm][name] = append(values[j.Arm][name], v)
		}
	}
	wall := o.Now().Sub(start).Seconds()
	r, err := e.results(values, o)
	if err != nil {
		return nil, err
	}
	r.WallTimeS = round(wall)
	if r.Hardware.Device == "" {
		r.Hardware.Device = device
	}
	return r, nil
}

// results builds results.json from the values of the seeds.
func (e *Experiment) results(values Values, o Options) (*Results, error) {
	d := e.Design
	in, err := e.InputsHash()
	if err != nil {
		return nil, err
	}
	xh, err := e.ExpectHash()
	if err != nil {
		return nil, err
	}
	now := o.Now().UTC()
	r := &Results{
		Experiment: d.ID, Node: o.Node, Image: o.Image, Hardware: o.Hardware,
		Date: now.Format("2006-01-02"), InputsHash: in, ExpectHash: xh,
		Seeds: d.Seeds.List(), Arms: map[string]map[string]Summary{},
	}
	r.Hardware.Class = d.Budget
	r.Run = fmt.Sprintf("r-%s-%s", r.Date, strings.TrimPrefix(in, "sha256:")[:6])
	for arm, ms := range values {
		r.Arms[arm] = map[string]Summary{}
		for m, xs := range ms {
			r.Arms[arm][m] = Summarize(xs)
		}
	}
	r.Primary, r.Secondary, r.Verdict = Decide(d, e.Expect, values)
	figs, _ := filepath.Glob(filepath.Join(e.Dir, Figures, "*.svg"))
	for _, f := range figs {
		r.Figures = append(r.Figures, Figures+"/"+filepath.Base(f))
	}
	sort.Strings(r.Figures)
	return r, nil
}

func readMetrics(path string) (*Metrics, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("run.py wrote no %s", MetricsFile)
	}
	if err != nil {
		return nil, err
	}
	var m Metrics
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("%s: %w", MetricsFile, err)
	}
	return &m, nil
}

// Write writes results.json into the experiment directory. Only aik exp run -update calls it (rule R05).
func Write(e *Experiment, r *Results) error {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(e.Dir, ResultsFile), append(b, '\n'), 0o644)
}

// Stale tells why the results do not match the current files, or gives "".
func (e *Experiment) Stale() string {
	if e.Results == nil {
		return ""
	}
	if h, err := e.ExpectHash(); err == nil && h != e.Results.ExpectHash {
		return "expect.yaml changed after the run"
	}
	if h, err := e.InputsHash(); err == nil && h != e.Results.InputsHash {
		return "the inputs changed after the run"
	}
	return ""
}
