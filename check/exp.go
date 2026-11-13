// SPDX-License-Identifier: Apache-2.0

package check

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/tamnd/ai-agent/exp"
	"github.com/tamnd/ai-agent/node"
)

// reproduceRE matches the words that present a small experiment as a reproduction of the result of a paper (rule R04).
var reproduceRE = regexp.MustCompile(`(?i)\b(?:we\s+)?reproduc(?:e|es|ed|ing)\s+(?:the|this|its|their)\s+(?:paper|result|results|finding|findings)\b|\bwe\s+reproduced\b`)

// exps checks the experiments of a node (spec 9.13).
func (r *run) exps(n *node.Node) {
	byID := map[string]bool{}
	for _, c := range n.Claims {
		byID[c.ID] = true
	}
	for _, e := range n.Exps {
		r.exp(n, e, byID)
	}
	if len(n.Exps) > 0 {
		for i, line := range strings.Split(n.Body, "\n") {
			if m := reproduceRE.FindString(line); m != "" {
				r.add("R04", Level, n.Rel+"/node.md", n.BodyLine+i, "%q presents a small-scale experiment as a reproduction; write \"The claim holds at this scale\" (9.2)", m)
			}
		}
	}
}

func (r *run) exp(n *node.Node, e *exp.Experiment, claims map[string]bool) {
	for _, err := range e.Errs {
		rule := "R01"
		if strings.HasPrefix(err.Error(), exp.ResultsFile) {
			rule = "R05"
		}
		r.add(rule, Block, e.Rel, 0, "%v", err)
	}
	d, x := e.Design, e.Expect
	if d == nil {
		if len(e.Errs) == 0 {
			r.add("R01", Level, e.File(exp.DesignFile), 0, "the experiment %s has no %s", e.Slug, exp.DesignFile)
		}
		return
	}
	p := e.File(exp.DesignFile)
	if !e.HasScript {
		r.add("R01", Level, p, 0, "the experiment has no %s", exp.Script)
	}
	if d.ID != e.Slug {
		r.add("R01", Level, p, 0, "the id is %q, but the directory is exp/%s", d.ID, e.Slug)
	}
	if len(d.Claims) == 0 {
		r.add("R01", Level, p, 0, "the experiment tests no claim; name the claims of claims.yaml that it tests")
	}
	for _, c := range d.Claims {
		if !claims[c] {
			r.add("R01", Level, p, 0, "the claim %q is not in claims.yaml", c)
		}
	}
	if strings.TrimSpace(d.Question) == "" {
		r.add("R01", Level, p, 0, "the experiment has no question")
	}
	if _, ok := exp.Budgets[d.Budget]; !ok {
		r.add("R01", Level, p, 0, "the budget %q is not cpu-small, gpu-small or gpu-medium (9.4.1)", d.Budget)
	}
	if d.Seeds.N < 1 {
		r.add("R01", Level, p, 0, "the experiment has no seeds; give seeds: {n, base}")
	}
	if d.Framework != "" && r.c.LangByID[d.Framework] == nil {
		r.add("R01", Level, p, 0, "the framework %q is not in %s", d.Framework, r.c.LangFile)
	}
	if d.UsesImpl != "" {
		if _, err := os.Stat(filepath.Join(e.Dir, filepath.FromSlash(d.UsesImpl))); err != nil {
			r.add("R01", Level, p, 0, "uses_impl: the file %s is not there", d.UsesImpl)
		}
	}
	seen := map[string]bool{}
	for _, a := range d.Arms {
		if a.ID == "" || seen[a.ID] {
			r.add("R01", Level, p, 0, "each arm needs an id of its own; %q is empty or there twice", a.ID)
		}
		seen[a.ID] = true
	}
	if len(d.Arms) == 0 {
		r.add("R01", Level, p, 0, "the experiment has no arms")
	}
	if strings.TrimSpace(d.ScaleNote.Here) == "" || strings.TrimSpace(d.ScaleNote.Source) == "" {
		r.add("R03", Level, p, 0, "scale_note needs here and source: the scale of the experiment and the scale of the claim of the source (9.2)")
	}
	if d.Deterministic != nil && !*d.Deterministic && strings.TrimSpace(d.DeterministicReason) == "" {
		r.add("R01", Level, p, 0, "deterministic: false needs deterministic_reason (9.8)")
	}
	r.pins(e)
	r.tuning(e)
	if x == nil {
		if len(e.Errs) == 0 {
			r.add("R06", Block, e.File(exp.ExpectFile), 0, "the experiment has no %s; write the test before the first full run (9.3.2)", exp.ExpectFile)
		}
	} else {
		r.expect(e)
	}
	r.results(e)
}

// pins checks the data of an experiment against the register of the data pins (rule R02).
func (r *run) pins(e *exp.Experiment) {
	p := e.File(exp.DesignFile)
	for i, d := range e.Design.Data {
		switch {
		case d.Generator != "":
			if _, err := os.Stat(filepath.Join(e.Dir, filepath.FromSlash(d.Generator))); err != nil {
				r.add("R02", Block, p, 0, "data %d: the generator %s is not in the experiment directory", i+1, d.Generator)
			}
		case d.Pin == "":
			r.add("R02", Block, p, 0, "data %d: give a pin of %s or a generator", i+1, exp.PinsFile)
		default:
			pin := r.c.Pins[exp.PinID(d)]
			switch {
			case pin == nil:
				r.add("R02", Block, p, 0, "data %d: the pin %q is not in %s", i+1, d.Pin, exp.PinsFile)
			case pin.Licence.SPDX == "" || strings.EqualFold(pin.Licence.SPDX, "unknown") || pin.Mirror == "unknown" || pin.Mirror == "":
				r.add("R02", Block, p, 0, "data %d: the licence of the pin %s is not resolved; an experiment does not use it until it is (9.5)", i+1, pin.ID)
			case len(pin.Files) == 0:
				r.add("R02", Block, p, 0, "data %d: the pin %s lists no files with their hashes", i+1, pin.ID)
			}
		}
	}
}

// tuning checks that every arm has the same seeds and the same tuning budget (rule R08).
func (r *run) tuning(e *exp.Experiment) {
	p := e.File(exp.DesignFile)
	var budget any
	first := ""
	for _, a := range e.Design.Arms {
		if a.Seeds != nil {
			r.add("R08", Level, p, 0, "the arm %s has seeds of its own; every arm runs with the seeds of the design", a.ID)
		}
		if a.Tuning == nil {
			continue
		}
		b := a.Tuning["budget"]
		if first == "" {
			budget, first = b, a.ID
		} else if fmt.Sprint(b) != fmt.Sprint(budget) {
			r.add("R08", Level, p, 0, "the arm %s has the tuning budget %v, but the arm %s has %v; every arm has the same budget", a.ID, b, first, budget)
		}
	}
}

// expect checks expect.yaml (rules R01 and R06).
func (r *run) expect(e *exp.Experiment) {
	d, x := e.Design, e.Expect
	p := e.File(exp.ExpectFile)
	if !slices.Contains(d.Claims, x.Claim) {
		r.add("R01", Level, p, 0, "the claim %q is not a claim of %s", x.Claim, exp.DesignFile)
	}
	if len(x.Primary) == 0 {
		r.add("R01", Level, p, 0, "expect.yaml has no primary test")
	}
	for _, t := range append(append(exp.Tests{}, x.Primary...), x.Secondary...) {
		r.test(e, t)
	}
}

func (r *run) test(e *exp.Experiment, t exp.Test) {
	d := e.Design
	p := e.File(exp.ExpectFile)
	if !exp.TestNames[t.Test] {
		r.add("R01", Level, p, 0, "%s: the test %q is not welch-t, mann-whitney, bootstrap-diff, tost, fit, shape or rate (9.3.2)", t.Metric, t.Test)
		return
	}
	if !slices.Contains(d.Metrics, t.Metric) {
		r.add("R01", Level, p, 0, "the metric %q is not in the metrics of %s", t.Metric, exp.DesignFile)
	}
	if t.At != "" && t.At != "final" {
		r.add("R01", Warn, p, 0, "%s: aik reads only the final value of a metric yet, not %q", t.Metric, t.At)
	}
	if t.Test == "shape" {
		r.add("R01", Warn, p, 0, "%s: aik cannot run the test shape yet; the experiment stays inconclusive", t.Metric)
	}
	if exp.Comparative[t.Test] {
		switch {
		case t.Compare == nil:
			r.add("R01", Level, p, 0, "%s: the test %s needs compare: {treatment, control}", t.Metric, t.Test)
		default:
			for _, a := range []string{t.Compare.Treatment, t.Compare.Control} {
				if d.Arm(a) == nil {
					r.add("R01", Level, p, 0, "%s: the arm %q is not an arm of %s; a comparative claim needs its control arm", t.Metric, a, exp.DesignFile)
				}
			}
		}
		if t.Test != "tost" && t.Direction != "lower" && t.Direction != "higher" {
			r.add("R01", Level, p, 0, "%s: direction is lower or higher, not %q", t.Metric, t.Direction)
		}
		if t.Test == "tost" && t.Margin == nil {
			r.add("R01", Level, p, 0, "%s: the test tost needs a margin", t.Metric)
		}
	}
	if t.Test == "fit" {
		if len(d.Arms) < 5 {
			r.add("R01", Level, p, 0, "%s: a trend needs at least 5 points of the variable; the design has %d arms (9.3.1)", t.Metric, len(d.Arms))
		}
		if t.X == "" {
			r.add("R01", Level, p, 0, "%s: the test fit needs x, the path of the variable in the configuration", t.Metric)
		}
	}
	if t.Test == "rate" && d.Arm(t.Arm) == nil {
		r.add("R01", Level, p, 0, "%s: the test rate needs arm, an arm of %s", t.Metric, exp.DesignFile)
	}
}

// results checks results.json (rules R05, R06 and R09).
func (r *run) results(e *exp.Experiment) {
	res := e.Results
	if res == nil {
		return
	}
	d := e.Design
	p := e.File(exp.ResultsFile)
	if res.Run == "" || res.ExpectHash == "" || res.InputsHash == "" {
		r.add("R05", Block, p, 0, "results.json has no run id or no hashes; only aik exp run -update writes it")
	}
	if res.Experiment != d.ID {
		r.add("R05", Block, p, 0, "results.json is for the experiment %q, not %q", res.Experiment, d.ID)
	}
	if !slices.Equal(res.Seeds, d.Seeds.List()) {
		r.add("R05", Block, p, 0, "the seeds of results.json are not the seeds of the design")
	}
	for _, a := range d.Arms {
		if _, ok := res.Arms[a.ID]; !ok {
			r.add("R05", Block, p, 0, "results.json has no values of the arm %s", a.ID)
		}
	}
	if e.Expect != nil && res.ExpectHash != "" {
		if h, err := e.ExpectHash(); err == nil && h != res.ExpectHash {
			r.add("R06", Block, p, 0, "expect.yaml changed after the run; the test is fixed before the run (9.3.2)")
		} else if why := e.Stale(); why != "" {
			r.add("R05", Warn, p, 0, "results.json is stale: %s; run aik exp run -update", why)
		}
	}
	if b, ok := exp.Budgets[d.Budget]; ok && res.WallTimeS > float64(b.Seconds) {
		r.add("R09", Level, p, 0, "the run took %.0f seconds; %s allows %d", res.WallTimeS, d.Budget, b.Seconds)
	}
	if res.Hardware.Class != d.Budget {
		r.add("R09", Level, p, 0, "the run was in the class %q, but the design asks for %s", res.Hardware.Class, d.Budget)
	}
}
