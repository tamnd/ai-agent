// SPDX-License-Identifier: Apache-2.0

package exp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
)

// Verification is the result of aik exp verify on one experiment (spec 9.6, rules R05 and R06).
type Verification struct {
	Experiment string `json:"experiment"`
	Run        string `json:"run,omitempty"`
	// Stale tells why the results do not match the current inputs: the experiment drops to R1 until it runs again.
	Stale string `json:"stale,omitempty"`
	// Outputs is the directory of the run whose metrics were compared, or "" when they were not there.
	Outputs  string   `json:"outputs,omitempty"`
	Problems []string `json:"problems,omitempty"`
}

// OK tells if results.json agrees with itself, with the tests of expect.yaml and with the outputs of the run; strict
// also needs it to be current.
func (v Verification) OK(strict bool) bool {
	return len(v.Problems) == 0 && (!strict || v.Stale == "")
}

// Verify checks results.json of an experiment: its seeds are those of the design, its summaries are those of its
// values, the tests of expect.yaml give its verdicts on them, its figures are there, and, when outputs is the
// directory of the recorded run (arm/seed/metrics.json), each value is the one that run.py wrote. Results that pass are
// what aik exp run -update wrote (rule R05), unless a person edited the values and the verdicts alike; the outputs
// check rules that out while they are kept (9.9).
func Verify(ref string, e *Experiment, outputs string) Verification {
	v := Verification{Experiment: ref}
	r, d := e.Results, e.Design
	if r == nil {
		v.Problems = append(v.Problems, "the experiment has no "+ResultsFile)
		return v
	}
	v.Run = r.Run
	if d == nil || e.Expect == nil {
		v.Problems = append(v.Problems, fmt.Sprintf("the experiment needs %s and %s to check %s", DesignFile, ExpectFile, ResultsFile))
		return v
	}
	v.Stale = e.Stale()
	bad := func(format string, a ...any) { v.Problems = append(v.Problems, fmt.Sprintf(format, a...)) }
	if r.Experiment != d.ID {
		bad("the experiment is %q, but %s has the id %q", r.Experiment, DesignFile, d.ID)
	}
	if v.Stale == "" && !slices.Equal(r.Seeds, d.Seeds.List()) {
		bad("the seeds are %v, not the %d seeds of the design from %d", r.Seeds, d.Seeds.N, d.Seeds.Base)
	}
	values := Values{}
	for _, a := range d.Arms {
		ms, ok := r.Arms[a.ID]
		if !ok {
			bad("the arm %s has no results", a.ID)
			continue
		}
		values[a.ID] = map[string][]float64{}
		for _, m := range d.Metrics {
			s, ok := ms[m]
			if !ok {
				bad("the arm %s has no metric %s", a.ID, m)
				continue
			}
			if len(s.PerSeed) != len(r.Seeds) {
				bad("the arm %s has %d values of %s for %d seeds", a.ID, len(s.PerSeed), m, len(r.Seeds))
			}
			if want := Summarize(s.PerSeed); !same(want.Mean, s.Mean) || !same(want.SD, s.SD) {
				bad("the arm %s: %s has the mean %v and the SD %v, but its values give %v and %v", a.ID, m, s.Mean, s.SD, want.Mean, want.SD)
			}
			values[a.ID][m] = s.PerSeed
		}
	}
	for id := range r.Arms {
		if !slices.ContainsFunc(d.Arms, func(a Arm) bool { return a.ID == id }) {
			bad("the arm %s is not an arm of the design", id)
		}
	}
	if len(v.Problems) == 0 && v.Stale == "" {
		primary, secondary, verdict := Decide(d, e.Expect, values)
		if verdict != r.Verdict {
			bad("the verdict is %s, but the tests of %s give %s on its values", r.Verdict, ExpectFile, verdict)
		}
		for _, x := range [][2]Verdicts{{r.Primary, primary}, {r.Secondary, secondary}} {
			if a, b := canon(x[0]), canon(x[1]); a != b {
				bad("the tests of %s give %s on its values, but it has %s", ExpectFile, b, a)
			}
		}
	}
	for _, f := range r.Figures {
		if _, err := os.Stat(filepath.Join(e.Dir, filepath.FromSlash(f))); err != nil {
			bad("the figure %s is not there", f)
		}
	}
	if outputs != "" && len(v.Problems) == 0 {
		v.Outputs = outputs
		v.Problems = append(v.Problems, compareOutputs(d, r, outputs)...)
	}
	return v
}

// compareOutputs compares the values of results.json with the metrics.json of each job of the run.
func compareOutputs(d *Design, r *Results, dir string) []string {
	var out []string
	for i, seed := range r.Seeds {
		for _, a := range d.Arms {
			p := filepath.Join(dir, a.ID, strconv.FormatInt(seed, 10), MetricsFile)
			m, err := readMetrics(p)
			if err != nil {
				out = append(out, fmt.Sprintf("arm %s, seed %d: %v", a.ID, seed, err))
				continue
			}
			for _, name := range d.Metrics {
				got, ok := m.Final[name]
				want := r.Arms[a.ID][name].PerSeed
				switch {
				case !ok:
					out = append(out, fmt.Sprintf("arm %s, seed %d: %s has no %s", a.ID, seed, MetricsFile, name))
				case i < len(want) && !same(got, want[i]):
					out = append(out, fmt.Sprintf("arm %s, seed %d: %s is %v in %s, but %v in the run", a.ID, seed, name, want[i], ResultsFile, got))
				}
			}
		}
	}
	return out
}

// canon writes verdicts as JSON for the comparison: the same numbers give the same bytes.
func canon(vs Verdicts) string {
	if len(vs) == 0 {
		return "no tests"
	}
	b, err := json.Marshal([]Verdict(vs))
	if err != nil {
		return err.Error()
	}
	return string(bytes.TrimSpace(b))
}

// same compares two numbers of results.json, which keeps 4 significant digits of its summaries.
func same(a, b float64) bool {
	return a == b || math.Abs(a-b) <= 1e-12*math.Max(math.Abs(a), math.Abs(b))
}
