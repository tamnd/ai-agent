// SPDX-License-Identifier: Apache-2.0

package exp

import (
	"math"
	"strings"
	"testing"
)

func pilotOf(tr, co []float64) *Results {
	return &Results{Arms: map[string]map[string]Summary{
		"a": {"loss": Summarize(tr)},
		"b": {"loss": Summarize(co)},
	}}
}

func powerExp(n int, tests ...Test) *Experiment {
	for i := range tests {
		tests[i].Metric = "loss"
		tests[i].Compare = &Compare{Treatment: "a", Control: "b"}
	}
	return &Experiment{Design: &Design{Budget: "cpu-small", Seeds: Seeds{N: n, Base: 1}}, Expect: &Expect{Primary: tests}}
}

// The power of the t test of two means that differ by one SD, at a one-sided alpha of 0.05, against the noncentral t
// distribution of SciPy 1.18.1: 1 - nct.cdf(t.ppf(0.95, 2n-2), 2n-2, √(n/2)), which is 0.797 at 13 seeds and 0.824 at 14.
func TestPowerWelch(t *testing.T) {
	one := &Effect{Kind: "absolute", Value: 1}
	for n, want := range map[int]float64{10: 0.6936, 20: 0.9279} {
		e := powerExp(n, Test{Test: "welch-t", Direction: "higher"})
		got := PowerOf(e, pilotOf([]float64{1}, []float64{0}), PowerOptions{SD: one})
		if len(got) != 1 || got[0].Error != "" {
			t.Fatalf("%+v", got)
		}
		if p := got[0]; math.Abs(p.Power-want) > 0.03 || p.Effect != 1 || p.EffectFrom != "pilot" {
			t.Errorf("n = %d: power %.3f, want about %.3f; %+v", n, p.Power, want, p)
		}
		if p := got[0]; p.Needed < 13 || p.Needed > 15 || p.OK() != (n >= p.Needed) {
			t.Errorf("n = %d: %d seeds needed, want 13 or 14", n, p.Needed)
		}
	}
	// A loss that the claim lowers: the effect in the claimed direction is the control mean less the treatment's.
	e := powerExp(10, Test{Test: "welch-t", Direction: "lower"})
	if p := PowerOf(e, pilotOf([]float64{4, 5, 6}, []float64{5, 6, 7}), PowerOptions{})[0]; p.Effect != 1 || p.SD != [2]float64{1, 1} || p.Power < 0.6 {
		t.Errorf("lower: %+v", p)
	}
}

func TestPowerMinEffect(t *testing.T) {
	sd := &Effect{Kind: "absolute", Value: 0.1}
	// The true effect is the least effect of the claim, so half the runs fall short of it at any number of seeds.
	e := powerExp(50, Test{Test: "welch-t", Direction: "higher", MinEffect: &Effect{Kind: "relative", Value: 0.1}})
	p := PowerOf(e, pilotOf([]float64{11}, []float64{10}), PowerOptions{SD: sd, Max: 64})[0]
	if p.Need != 1 || p.Power > 0.55 || p.Needed != 0 || p.OK() || !strings.Contains(p.Note, "at most half") {
		t.Errorf("%+v", p)
	}
	// A claimed effect twice the least one reaches the target with a few seeds.
	p = PowerOf(e, pilotOf([]float64{11}, []float64{10}), PowerOptions{SD: sd, Effect: &Effect{Kind: "relative", Value: 0.2}})[0]
	if p.EffectFrom != "given" || p.Effect != 2 || p.Power < 0.99 || p.Needed > 3 || !p.OK() {
		t.Errorf("%+v", p)
	}
}

func TestPowerTests(t *testing.T) {
	one := &Effect{Kind: "absolute", Value: 1}
	pilot := pilotOf([]float64{1}, []float64{0})
	// The rank test has 3/π of the efficiency of the t test on normal values, so it needs a seed or two more.
	mw := PowerOf(powerExp(10, Test{Test: "mann-whitney", Direction: "higher"}), pilot, PowerOptions{SD: one})[0]
	if mw.Error != "" || mw.Needed < 13 || mw.Needed > 18 {
		t.Errorf("mann-whitney: %+v", mw)
	}
	// The interval of 95% excludes zero on one side: a one-sided alpha of 0.025, which needs about 16 seeds.
	bd := PowerOf(powerExp(20, Test{Test: "bootstrap-diff", Direction: "higher"}), pilot, PowerOptions{SD: one, Sims: 800})[0]
	if bd.Error != "" || bd.Power < 0.85 || bd.Needed < 14 || bd.Needed > 19 {
		t.Errorf("bootstrap-diff: %+v", bd)
	}
	// Equivalence of two arms with the same mean, within a margin of one SD: 0.856 by 200,000 runs in NumPy.
	same := pilotOf([]float64{0}, []float64{0})
	tost := PowerOf(powerExp(20, Test{Test: "tost", Margin: one}), same, PowerOptions{SD: one})[0]
	if want := 0.856; tost.Error != "" || math.Abs(tost.Power-want) > 0.03 {
		t.Errorf("tost: power %.3f, want about %.3f; %+v", tost.Power, want, tost)
	}
	out := PowerOf(powerExp(20, Test{Test: "tost", Margin: one}), pilotOf([]float64{2}, []float64{0}), PowerOptions{SD: one})[0]
	if out.Power > 0.06 || !strings.Contains(out.Note, "not inside the margin") {
		t.Errorf("tost outside the margin: %+v", out)
	}
}

func TestPowerErrors(t *testing.T) {
	e := powerExp(10, Test{Test: "welch-t", Direction: "higher"}, Test{Test: "welch-t", Direction: "higher"}, Test{Test: "fit"})
	e.Expect.Primary[2].Compare = nil
	got := PowerOf(e, pilotOf([]float64{1}, []float64{0}), PowerOptions{})
	if len(got) != 3 || !strings.Contains(got[0].Error, "one value of loss in each arm") || !got[0].OK() {
		t.Fatalf("one pilot value per arm: %+v", got)
	}
	if !strings.Contains(got[2].Error, "not of the test fit") {
		t.Errorf("fit: %+v", got[2])
	}
	got = PowerOf(e, pilotOf([]float64{1, 2}, []float64{0}), PowerOptions{})
	// The control takes the spread of the treatment, and the alpha is a third under the correction of Holm.
	if p := got[0]; p.Error != "" || p.SD[1] != p.SD[0] || math.Abs(p.Alpha-0.05/3) > 1e-12 || !strings.Contains(p.Note, "Holm") {
		t.Errorf("%+v", p)
	}
	if p := PowerOf(e, pilotOf(nil, []float64{0}), PowerOptions{})[0]; !strings.Contains(p.Error, "no values of loss") {
		t.Errorf("%+v", p)
	}
	if p := PowerOf(e, pilotOf([]float64{1, 1}, []float64{1, 1}), PowerOptions{})[0]; !strings.Contains(p.Error, "do not vary") {
		t.Errorf("%+v", p)
	}
	// The design names no seeds: the default of the class.
	e.Design.Seeds.N = 0
	if p := PowerOf(e, pilotOf([]float64{1}, []float64{0}), PowerOptions{SD: &Effect{Kind: "absolute", Value: 1}})[0]; p.Seeds != 10 {
		t.Errorf("%+v", p)
	}
}

func TestNeeded(t *testing.T) {
	step := func(at int) func(int) float64 {
		return func(n int) float64 {
			if n >= at {
				return 1
			}
			return 0
		}
	}
	for _, at := range []int{2, 3, 7, 16, 17, 100} {
		if got := needed(step(at), 0.8, 100); got != at {
			t.Errorf("needed(%d) = %d", at, got)
		}
	}
	if got := needed(step(101), 0.8, 100); got != 0 {
		t.Errorf("past the most: %d", got)
	}
}
