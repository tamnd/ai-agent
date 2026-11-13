// SPDX-License-Identifier: Apache-2.0

package exp

import (
	"math"
	"testing"
)

// The reference values come from scipy.stats.
var (
	control   = []float64{0.1532, 0.1601, 0.1499, 0.1555, 0.1580, 0.1470, 0.1610, 0.1522, 0.1544, 0.1507}
	treatment = []float64{0.0958, 0.0990, 0.0921, 0.1003, 0.0944, 0.0977, 0.0912, 0.0969, 0.0951, 0.0985}
)

func near(t *testing.T, what string, got, want, tol float64) {
	t.Helper()
	if math.Abs(got-want) > tol*math.Max(1, math.Abs(want)) {
		t.Errorf("%s = %.10g, want %.10g", what, got, want)
	}
}

func TestTDistribution(t *testing.T) {
	near(t, "TCDF(-2.1, 7.3)", TCDF(-2.1, 7.3), 0.0361233567124267, 1e-9)
	near(t, "TCDF(0.7, 3)", TCDF(0.7, 3), 0.7328365008476182, 1e-9)
	near(t, "TCDF(-5, 1.5)", TCDF(-5, 1.5), 0.03268788381057809, 1e-9)
	near(t, "TQuantile(0.975, 7.3)", TQuantile(0.975, 7.3), 2.345066736547703, 1e-8)
	near(t, "TQuantile(0.995, 2)", TQuantile(0.995, 2), 9.924843200918287, 1e-8)
	near(t, "TQuantile(0.025, 7.3)", TQuantile(0.025, 7.3), -2.345066736547703, 1e-8)
}

func TestWelchT(t *testing.T) {
	w := WelchT(treatment, control)
	near(t, "t", w.T, -33.93594845325957, 1e-9)
	near(t, "df", w.DF, 15.50587591020409, 1e-9)
	near(t, "p", w.P("lower"), 2.8302424089875893e-16, 1e-6)
	ci := w.CI(0.95)
	near(t, "ci low", ci[0], -0.06173880382910681, 1e-8)
	near(t, "ci high", ci[1], -0.05446119617089327, 1e-8)
	w = WelchT([]float64{1, 2, 3, 4, 5, 6}, []float64{2, 4, 5, 4, 5, 7})
	near(t, "t", w.T, -0.9837387536759296, 1e-9)
	near(t, "df", w.DF, 9.836233367451381, 1e-9)
	near(t, "p", w.P("lower"), 0.17441103182391846, 1e-8)
}

func TestMannWhitney(t *testing.T) {
	u, p := MannWhitney(treatment, control, "lower")
	near(t, "U", u, 0, 1e-12)
	near(t, "p", p, 9.133589555477501e-05, 1e-8)
	u, p = MannWhitney([]float64{1, 2, 3, 4, 5, 6}, []float64{2, 4, 5, 4, 5, 7}, "lower")
	near(t, "U with ties", u, 12.5, 1e-12)
	near(t, "p with ties", p, 0.2079532296585322, 1e-8)
	near(t, "Hodges-Lehmann", HodgesLehmann([]float64{1, 2, 3}, []float64{0, 0, 1}), 2, 1e-12)
}

func TestFitLine(t *testing.T) {
	var x, y []float64
	for i, n := range []float64{1e3, 3e3, 1e4, 3e4, 1e5} {
		x = append(x, math.Log(n))
		y = append(y, math.Log([]float64{5.0, 4.1, 3.4, 2.8, 2.3}[i]))
	}
	l := FitLine(x, y)
	near(t, "slope", l.B, -0.16797154227109767, 1e-9)
	near(t, "r2", l.R2, 0.9995776358499479, 1e-9)
	near(t, "se", l.SEB, 0.001993472923906218, 1e-8)
}

func TestWilson(t *testing.T) {
	// 8 of 10 at z = 1.96: (0.4902, 0.9433).
	w := Wilson(8, 10, 1.959963984540054)
	near(t, "low", w[0], 0.4901625, 1e-6)
	near(t, "high", w[1], 0.9433178, 1e-6)
}

func TestHolm(t *testing.T) {
	got := Holm([]float64{0.01, 0.04, 0.03})
	for i, want := range []float64{0.03, 0.06, 0.06} {
		near(t, "holm", got[i], want, 1e-12)
	}
}

func TestBootstrap(t *testing.T) {
	b := Bootstrap{Resamples: 4000, Seed: 7}
	for _, ci := range [][2]float64{b.BCa(treatment, control, DiffMeans, 0.95), b.Percentile(treatment, control, DiffMeans, 0.95)} {
		if !(ci[0] < -0.0581 && ci[1] > -0.0581 && ci[1] < 0) {
			t.Errorf("the interval %v does not hold the difference -0.0581 below 0", ci)
		}
	}
	if b.BCa(treatment, control, DiffMeans, 0.95) != b.BCa(treatment, control, DiffMeans, 0.95) {
		t.Error("the same seed gives another interval")
	}
}
