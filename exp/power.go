// SPDX-License-Identifier: Apache-2.0

package exp

import (
	"fmt"
	"math"
	"math/rand/v2"
)

// PowerTarget is the least power of a primary test for the claimed effect size (rule R07).
const PowerTarget = 0.8

// PowerOptions are the assumptions of a power analysis.
type PowerOptions struct {
	// Effect is the true effect in the claimed direction, absolute or relative to the control mean. Nil takes the
	// effect of the pilot.
	Effect *Effect
	// SD is the spread of one seed in each arm, absolute or relative to the control mean. Nil takes the SD of the pilot
	// values of each arm.
	SD *Effect
	// Seeds is the number of seeds the power is computed at. 0 takes the seeds of the design, or the default of its
	// budget class.
	Seeds  int
	Target float64 // 0 takes PowerTarget
	Sims   int     // the number of simulated runs for each number of seeds; 0 takes 2000
	Max    int     // the most seeds the search tries; 0 takes 100
	Seed   uint64  // the seed of the simulation; 0 takes 1
}

// Power is the power of one primary test of an experiment.
type Power struct {
	Metric    string  `json:"metric"`
	Test      string  `json:"test"`
	Treatment string  `json:"treatment,omitempty"`
	Control   string  `json:"control,omitempty"`
	Direction string  `json:"direction,omitempty"`
	Alpha     float64 `json:"alpha,omitempty"` // the alpha the test is simulated at: alpha/k under the correction of Holm
	// Pilot is the number of pilot values of the treatment and the control.
	Pilot [2]int `json:"pilot,omitzero"`
	// Means and SD are the assumed mean and spread of one seed of the treatment and the control.
	Means [2]float64 `json:"means,omitzero"`
	SD    [2]float64 `json:"sd,omitzero"`
	// Effect is the assumed true effect in the claimed direction, and Need the least effect of min_effect, both
	// absolute. For tost, Effect is the true difference and Need the margin.
	Effect     float64 `json:"effect"`
	EffectFrom string  `json:"effect_from,omitempty"` // pilot | given
	Need       float64 `json:"need,omitempty"`
	Seeds      int     `json:"seeds,omitempty"`
	Power      float64 `json:"power"`
	// Needed is the least number of seeds whose power reaches the target, and 0 when Max seeds do not.
	Needed int     `json:"needed"`
	Target float64 `json:"target"`
	Note   string  `json:"note,omitempty"`
	Error  string  `json:"error,omitempty"`
}

// OK tells if the power reaches the target at the seeds of the design (rule R07). A test whose power aik cannot
// compute is not a failure of the rule; its Error says why.
func (p Power) OK() bool { return p.Error != "" || p.Power >= p.Target }

// PowerOf computes the power of each comparative primary test of the experiment from a pilot run: the values of one
// seed of each arm are taken as normal, with the means of the pilot and its spread, and the test of expect.yaml runs on
// simulated runs, with its min_effect, at the seeds of the design and at more or fewer seeds until the power reaches the
// target (spec 09.3.1, rule R07).
func PowerOf(e *Experiment, pilot *Results, o PowerOptions) []Power {
	if o.Target == 0 {
		o.Target = PowerTarget
	}
	if o.Sims == 0 {
		o.Sims = 2000
	}
	if o.Max == 0 {
		o.Max = 100
	}
	if o.Seed == 0 {
		o.Seed = 1
	}
	if o.Seeds == 0 && e.Design != nil {
		o.Seeds = e.Design.Seeds.N
		if o.Seeds == 0 {
			o.Seeds = Budgets[e.Design.Budget].Seeds
		}
	}
	if e.Expect == nil {
		return []Power{{Target: o.Target, Error: "the experiment has no " + ExpectFile}}
	}
	level := e.Expect.Report.CI
	if level == 0 {
		level = 0.95
	}
	k := len(e.Expect.Primary)
	var out []Power
	for _, t := range e.Expect.Primary {
		p := Power{Metric: t.Metric, Test: t.Test, Direction: t.Direction, Seeds: o.Seeds, Target: o.Target, Alpha: t.Alpha}
		if p.Alpha == 0 {
			p.Alpha = 0.05
		}
		if e.Expect.Correction != "none" && k > 1 {
			p.Alpha /= float64(k)
			p.Note = fmt.Sprintf("alpha/%d, the strictest step of the correction of Holm", k)
		}
		if !Comparative[t.Test] {
			p.Error = fmt.Sprintf("aik exp power computes the power of a comparative test, not of the test %s", t.Test)
			out = append(out, p)
			continue
		}
		if t.Compare == nil {
			p.Error = "the test names no arms to compare"
			out = append(out, p)
			continue
		}
		p.Treatment, p.Control = t.Compare.Treatment, t.Compare.Control
		if err := p.assume(t, pilot, o); err != nil {
			p.Error = err.Error()
			out = append(out, p)
			continue
		}
		sims := o.Sims
		bs := Bootstrap{Resamples: 1}
		if t.Test == "bootstrap-diff" {
			// The bootstrap of each simulated run is the most costly part, so it has fewer runs and resamples, and the
			// percentile interval stands for the BCa interval.
			sims, bs.Resamples = max(sims/2, 1), 400
		}
		power := func(n int) float64 {
			return simulate(t, p.Alpha, level, p.Means, p.SD, n, sims, o.Seed, bs)
		}
		if p.Seeds >= 2 {
			p.Power = round(power(p.Seeds))
		}
		p.Needed = needed(power, o.Target, o.Max)
		switch {
		case t.Test != "tost" && p.Effect <= 0:
			p.Note = joinNote(p.Note, "the assumed effect is not in the claimed direction; give the claimed effect with -effect")
		case t.Test != "tost" && p.Effect <= p.Need && p.Need > 0:
			p.Note = joinNote(p.Note, "the assumed effect is not more than min_effect, so the test passes at most half the time at any number of seeds")
		case t.Test == "tost" && math.Abs(p.Effect) >= p.Need:
			p.Note = joinNote(p.Note, "the assumed difference is not inside the margin, so the test cannot pass")
		}
		out = append(out, p)
	}
	return out
}

// assume sets the means, spreads and effect of the simulation from the pilot and the options.
func (p *Power) assume(t Test, pilot *Results, o PowerOptions) error {
	var tr, co []float64
	if pilot != nil {
		tr, co = pilot.Arms[p.Treatment][t.Metric].PerSeed, pilot.Arms[p.Control][t.Metric].PerSeed
	}
	if len(co) == 0 || (len(tr) == 0 && o.Effect == nil) {
		return fmt.Errorf("the pilot has no values of %s for the arms %s and %s", t.Metric, p.Treatment, p.Control)
	}
	p.Pilot = [2]int{len(tr), len(co)}
	control := Mean(co)
	sign := 1.0
	if t.Direction == "lower" && t.Test != "tost" {
		sign = -1
	}
	if o.Effect != nil {
		p.Effect, p.EffectFrom = o.Effect.Of(control), "given"
	} else {
		p.Effect, p.EffectFrom = sign*(Mean(tr)-control), "pilot"
	}
	p.Means = [2]float64{round(control + sign*p.Effect), round(control)}
	if o.SD != nil {
		s := o.SD.Of(control)
		p.SD = [2]float64{s, s}
	} else {
		st, sc := spread(tr), spread(co)
		switch {
		case math.IsNaN(st) && math.IsNaN(sc):
			return fmt.Errorf("the pilot has one value of %s in each arm, so it gives no spread; give it with -sd", t.Metric)
		case math.IsNaN(st):
			st = sc
		case math.IsNaN(sc):
			sc = st
		}
		p.SD = [2]float64{round(st), round(sc)}
	}
	if p.SD[0] <= 0 && p.SD[1] <= 0 {
		return fmt.Errorf("the pilot values of %s do not vary, so they give no spread; give it with -sd", t.Metric)
	}
	p.Effect = round(p.Effect)
	if t.Test == "tost" {
		if t.Margin == nil {
			return fmt.Errorf("the test tost needs a margin")
		}
		p.Need = round(t.Margin.Of(control))
	} else {
		p.Need = round(t.MinEffect.Of(control))
	}
	return nil
}

// spread is the SD of the values, and NaN for fewer than two.
func spread(xs []float64) float64 {
	if len(xs) < 2 {
		return math.NaN()
	}
	return SD(xs)
}

// simulate gives the share of simulated runs of n seeds per arm that pass the test. Each number of seeds draws from the
// same stream, so that the power changes smoothly with n.
func simulate(t Test, alpha, level float64, means, sd [2]float64, n, sims int, seed uint64, bs Bootstrap) float64 {
	r := rand.New(rand.NewPCG(seed, seed^0x2545f4914f6cdd1d)) //nolint:gosec // a seeded generator is the point here
	tr, co := make([]float64, n), make([]float64, n)
	pass := 0
	for i := range sims {
		for j := range n {
			tr[j] = means[0] + sd[0]*r.NormFloat64()
			co[j] = means[1] + sd[1]*r.NormFloat64()
		}
		bs.Seed = seed + uint64(i)
		if comparative(Verdict{alpha: alpha}, t, tr, co, level, bs, false).Passed {
			pass++
		}
	}
	return float64(pass) / float64(sims)
}

// needed gives the least number of seeds, from 2 to most, whose power reaches the target, and 0 when most do not: it
// doubles the seeds until the power reaches the target, then bisects.
func needed(power func(int) float64, target float64, most int) int {
	lo, hi := 1, 2 // the power at lo is under the target, and hi is the next to try
	for power(hi) < target {
		if hi >= most {
			return 0
		}
		lo, hi = hi, min(2*hi, most)
	}
	for hi-lo > 1 {
		mid := (lo + hi) / 2
		if power(mid) >= target {
			hi = mid
		} else {
			lo = mid
		}
	}
	return hi
}
