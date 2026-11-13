// SPDX-License-Identifier: Apache-2.0

package exp

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
)

// The verdicts of spec 9.7.
const (
	Holds        = "holds"
	DidNotHold   = "did-not-hold"
	Inconclusive = "inconclusive"
)

// Summary is one metric of one arm over the seeds.
type Summary struct {
	Mean    float64   `json:"mean"`
	SD      float64   `json:"sd"`
	PerSeed []float64 `json:"per_seed"`
}

// Summarize gives the summary of the values of the seeds.
func Summarize(xs []float64) Summary {
	if len(xs) == 0 {
		return Summary{PerSeed: []float64{}}
	}
	return Summary{Mean: round(Mean(xs)), SD: round(SD(xs)), PerSeed: xs}
}

// Verdict is the outcome of one test of expect.yaml.
type Verdict struct {
	Metric string `json:"metric"`
	Test   string `json:"test"`
	// P is the p-value. For a primary test with a correction it is the adjusted p-value, and Raw is the p-value before the correction.
	P              *float64   `json:"p,omitempty"`
	Raw            *float64   `json:"p_raw,omitempty"`
	Effect         *float64   `json:"effect,omitempty"`
	EffectRelative *float64   `json:"effect_relative,omitempty"`
	CI95           []float64  `json:"ci95,omitempty"`
	R2             *float64   `json:"r2,omitempty"`
	Passed         bool       `json:"passed"`
	Verdict        string     `json:"verdict"`
	Note           string     `json:"note,omitempty"`
	alpha          float64    // the alpha of the test, for the correction
	compatible     bool       // the data do not rule out the claimed effect
	ci             [2]float64 // the interval before rounding
}

// Verdicts is a list of verdicts. One verdict is written as an object, as in spec 9.6, and more than one as a list.
type Verdicts []Verdict

// MarshalJSON writes one verdict as an object.
func (v Verdicts) MarshalJSON() ([]byte, error) {
	if len(v) == 1 {
		return json.Marshal(v[0])
	}
	return json.Marshal([]Verdict(v))
}

// UnmarshalJSON reads an object or a list.
func (v *Verdicts) UnmarshalJSON(b []byte) error {
	if len(b) > 0 && b[0] == '{' {
		var one Verdict
		if err := json.Unmarshal(b, &one); err != nil {
			return err
		}
		*v = Verdicts{one}
		return nil
	}
	var many []Verdict
	if err := json.Unmarshal(b, &many); err != nil {
		return err
	}
	*v = many
	return nil
}

// Hardware is the machine of the run.
type Hardware struct {
	Class   string `json:"class"`
	CPU     string `json:"cpu,omitempty"`
	Threads int    `json:"threads,omitempty"`
	Device  string `json:"device,omitempty"`
	OS      string `json:"os,omitempty"`
}

// Results is the file results.json (spec 9.6). Only aik exp run -update writes it (rule R05).
type Results struct {
	Experiment string                        `json:"experiment"`
	Node       string                        `json:"node"`
	Run        string                        `json:"run"`
	Date       string                        `json:"date"`
	Image      string                        `json:"image"`
	Hardware   Hardware                      `json:"hardware"`
	InputsHash string                        `json:"inputs_hash"`
	ExpectHash string                        `json:"expect_hash"`
	Seeds      []int64                       `json:"seeds"`
	Arms       map[string]map[string]Summary `json:"arms"`
	Primary    Verdicts                      `json:"primary"`
	Secondary  Verdicts                      `json:"secondary,omitempty"`
	Verdict    string                        `json:"verdict"`
	Figures    []string                      `json:"figures,omitempty"`
	WallTimeS  float64                       `json:"wall_time_s"`
	CostUSD    float64                       `json:"cost_usd"`
}

// Values are the values of each metric of each arm, one per seed in the order of the seeds.
type Values map[string]map[string][]float64

// Decide runs the tests of expect.yaml on the values. The design gives the configurations of the arms for the test fit.
func Decide(d *Design, x *Expect, v Values) (primary, secondary Verdicts, verdict string) {
	level := x.Report.CI
	if level == 0 {
		level = 0.95
	}
	bs := Bootstrap{Resamples: x.Report.Bootstrap.Resamples, Seed: x.Report.Bootstrap.Seed}
	if bs.Resamples == 0 {
		bs.Resamples = 10000
	}
	if bs.Seed == 0 {
		bs.Seed = uint64(max(d.Seeds.Base, 1))
	}
	bca := x.Report.Bootstrap.Method != "percentile"
	for _, t := range x.Primary {
		primary = append(primary, decide(d, t, v, level, bs, bca))
	}
	for _, t := range x.Secondary {
		secondary = append(secondary, decide(d, t, v, level, bs, bca))
	}
	if x.Correction != "none" && len(primary) > 1 {
		correct(primary)
	}
	verdict = Holds
	for _, p := range primary {
		switch p.Verdict {
		case DidNotHold:
			verdict = DidNotHold
		case Inconclusive:
			if verdict == Holds {
				verdict = Inconclusive
			}
		}
	}
	if len(primary) == 0 {
		verdict = Inconclusive
	}
	return primary, secondary, verdict
}

// correct adjusts the p-values of the primary tests by the method of Holm, and decides each test again on the adjusted value.
func correct(vs Verdicts) {
	var idx []int
	var ps []float64
	for i, v := range vs {
		if v.P != nil {
			idx = append(idx, i)
			ps = append(ps, *v.P)
		}
	}
	for j, q := range Holm(ps) {
		v := &vs[idx[j]]
		raw := *v.P
		v.Raw, v.P = num(raw), num(q)
		if v.Passed && q >= v.alpha {
			v.Passed = false
			v.Verdict = Inconclusive
			if !v.compatible {
				v.Verdict = DidNotHold
			}
			v.Note = joinNote(v.Note, "the test does not pass after the correction of Holm")
		}
	}
}

func decide(d *Design, t Test, v Values, level float64, bs Bootstrap, bca bool) (out Verdict) {
	out = Verdict{Metric: t.Metric, Test: t.Test, alpha: t.Alpha, Note: t.Note}
	if out.alpha == 0 {
		out.alpha = 0.05
	}
	defer func() {
		if out.Verdict == "" {
			out.Verdict = Inconclusive
			if out.Passed {
				out.Verdict = Holds
			} else if !out.compatible {
				out.Verdict = DidNotHold
			}
		}
		if finite(out.ci[0]) && finite(out.ci[1]) && out.ci != [2]float64{} {
			out.CI95 = []float64{round(out.ci[0]), round(out.ci[1])}
		}
	}()
	fail := func(note string) Verdict {
		out.Verdict, out.Note = Inconclusive, joinNote(out.Note, note)
		return out
	}
	switch t.Test {
	case "welch-t", "mann-whitney", "bootstrap-diff", "tost":
		if t.Compare == nil {
			return fail("the test names no arms to compare")
		}
		tr, co := v[t.Compare.Treatment][t.Metric], v[t.Compare.Control][t.Metric]
		if len(tr) < 2 || len(co) < 2 {
			return fail(fmt.Sprintf("the arms %s and %s need at least 2 values of %s", t.Compare.Treatment, t.Compare.Control, t.Metric))
		}
		return comparative(out, t, tr, co, level, bs, bca)
	case "rate":
		return rateTest(out, t, v, level)
	case "fit":
		return fitTest(out, d, t, v, level)
	case "shape":
		return fail("aik cannot run the test shape yet")
	}
	return fail(fmt.Sprintf("the test %q is not a test of spec 9.3.2", t.Test))
}

// comparative runs a test of a treatment arm against a control arm.
func comparative(out Verdict, t Test, tr, co []float64, level float64, bs Bootstrap, bca bool) Verdict {
	sign := 1.0
	if t.Direction == "lower" {
		sign = -1
	}
	control := Mean(co)
	need := t.MinEffect.Of(control) // the least size of the effect in the claimed direction
	switch t.Test {
	case "welch-t":
		w := WelchT(tr, co)
		p := w.P(t.Direction)
		out.P, out.Effect = num(p), num(w.Diff)
		out.ci = w.CI(level)
		out.Passed = p < out.alpha && sign*w.Diff >= need
	case "mann-whitney":
		_, p := MannWhitney(tr, co, t.Direction)
		hl := HodgesLehmann(tr, co)
		control = Median(co)
		need = t.MinEffect.Of(control)
		out.P, out.Effect = num(p), num(hl)
		out.ci = bs.Percentile(tr, co, HodgesLehmann, level)
		out.Passed = p < out.alpha && sign*hl >= need
	case "bootstrap-diff":
		diff := DiffMeans(tr, co)
		out.Effect = num(diff)
		if bca {
			out.ci = bs.BCa(tr, co, DiffMeans, level)
		} else {
			out.ci = bs.Percentile(tr, co, DiffMeans, level)
		}
		lo, hi := out.ci[0], out.ci[1]
		out.Passed = (sign > 0 && lo > 0 || sign < 0 && hi < 0) && sign*diff >= need
	case "tost":
		if t.Margin == nil {
			out.Verdict, out.Note = Inconclusive, joinNote(out.Note, "the test tost needs a margin")
			return out
		}
		m := t.Margin.Of(control)
		w := WelchT(tr, co)
		p1 := 1 - TCDF((w.Diff+m)/w.SE, w.DF)
		p2 := TCDF((w.Diff-m)/w.SE, w.DF)
		p := math.Max(p1, p2)
		out.P, out.Effect = num(p), num(w.Diff)
		out.ci = w.CI(1 - 2*out.alpha)
		out.Passed = p < out.alpha
		out.compatible = out.ci[0] < m && out.ci[1] > -m
		if control != 0 {
			out.EffectRelative = num(w.Diff / math.Abs(control))
		}
		return out
	}
	if control != 0 && out.Effect != nil {
		out.EffectRelative = num(*out.Effect / math.Abs(control))
	}
	// The claimed effect is still possible when the end of the interval in the claimed direction reaches it.
	end := out.ci[1]
	if sign < 0 {
		end = out.ci[0]
	}
	out.compatible = sign*end >= need
	return out
}

// rateTest decides a failure or risk demonstration: each seed gives 1 when the failure happened and 0 when it did not.
func rateTest(out Verdict, t Test, v Values, level float64) Verdict {
	xs := v[t.Arm][t.Metric]
	if len(xs) == 0 {
		out.Verdict, out.Note = Inconclusive, joinNote(out.Note, fmt.Sprintf("the arm %q has no values of %s", t.Arm, t.Metric))
		return out
	}
	k := 0
	for _, x := range xs {
		if x >= 0.5 {
			k++
		}
	}
	z := NormQuantile(1 - out.alpha)
	one := Wilson(k, len(xs), z)
	out.Effect = num(float64(k) / float64(len(xs)))
	out.ci = Wilson(k, len(xs), NormQuantile(1-(1-level)/2))
	out.Passed = one[0] > t.Rate
	out.compatible = one[1] > t.Rate
	return out
}

// fitTest decides a trend: a power law (a line on log-log axes) or a line, over the arms, with one point for each seed.
func fitTest(out Verdict, d *Design, t Test, v Values, level float64) Verdict {
	var xs, ys []float64
	power := t.Form != "linear"
	for i := range d.Arms {
		a := &d.Arms[i]
		raw, ok := Lookup(d.Config(a), t.X)
		x, isNum := toFloat(raw)
		if !ok || !isNum {
			out.Verdict, out.Note = Inconclusive, joinNote(out.Note, fmt.Sprintf("the arm %s has no number at %s", a.ID, t.X))
			return out
		}
		for _, y := range v[a.ID][t.Metric] {
			if power {
				if x <= 0 || y <= 0 {
					out.Verdict, out.Note = Inconclusive, joinNote(out.Note, "a power law needs values above 0")
					return out
				}
				xs, ys = append(xs, math.Log(x)), append(ys, math.Log(y))
			} else {
				xs, ys = append(xs, x), append(ys, y)
			}
		}
	}
	if len(xs) < 3 {
		out.Verdict, out.Note = Inconclusive, joinNote(out.Note, "the fit needs at least 3 points")
		return out
	}
	l := FitLine(xs, ys)
	out.Effect, out.R2 = num(l.B), num(l.R2)
	out.ci = l.CI(level)
	inside, overlap := true, true
	if len(t.Range) == 2 {
		inside = out.ci[0] >= t.Range[0] && out.ci[1] <= t.Range[1]
		overlap = out.ci[1] >= t.Range[0] && out.ci[0] <= t.Range[1]
	}
	out.Passed = l.R2 >= t.R2Min && inside
	out.compatible = l.R2 >= t.R2Min && overlap
	return out
}

func toFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case float64:
		return x, true
	case string:
		f, err := strconv.ParseFloat(x, 64)
		return f, err == nil
	}
	return 0, false
}

func finite(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }

func joinNote(a, b string) string {
	if a == "" {
		return b
	}
	return a + "; " + b
}

// num gives a pointer to the rounded value, or nil for a value that JSON cannot hold.
func num(x float64) *float64 {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return nil
	}
	r := round(x)
	return &r
}

// round keeps 4 significant digits, so that results.json does not change with the last bits of a sum.
func round(x float64) float64 {
	if x == 0 || math.IsNaN(x) || math.IsInf(x, 0) {
		return x
	}
	f, _ := strconv.ParseFloat(strconv.FormatFloat(x, 'g', 4, 64), 64)
	return f
}

// SortedArms gives the arm ids of the results in order.
func (r *Results) SortedArms() []string {
	out := make([]string, 0, len(r.Arms))
	for a := range r.Arms {
		out = append(out, a)
	}
	sort.Strings(out)
	return out
}
