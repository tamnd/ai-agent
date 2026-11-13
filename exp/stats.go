// SPDX-License-Identifier: Apache-2.0

package exp

import (
	"math"
	"math/rand/v2"
	"slices"
	"sort"
)

// Mean gives the mean of xs, or NaN for no values.
func Mean(xs []float64) float64 {
	if len(xs) == 0 {
		return math.NaN()
	}
	s := 0.0
	for _, x := range xs {
		s += x
	}
	return s / float64(len(xs))
}

// Var gives the sample variance of xs, with n-1 in the denominator. It is 0 for fewer than 2 values.
func Var(xs []float64) float64 {
	if len(xs) < 2 {
		return 0
	}
	m := Mean(xs)
	s := 0.0
	for _, x := range xs {
		s += (x - m) * (x - m)
	}
	return s / float64(len(xs)-1)
}

// SD gives the sample standard deviation of xs.
func SD(xs []float64) float64 { return math.Sqrt(Var(xs)) }

// Median gives the median of xs.
func Median(xs []float64) float64 {
	if len(xs) == 0 {
		return math.NaN()
	}
	s := slices.Clone(xs)
	sort.Float64s(s)
	n := len(s)
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2
}

// NormCDF is the distribution function of the standard normal distribution.
func NormCDF(z float64) float64 { return 0.5 * math.Erfc(-z/math.Sqrt2) }

// NormQuantile is the inverse of NormCDF.
func NormQuantile(p float64) float64 { return math.Sqrt2 * math.Erfinv(2*p-1) }

// TCDF is the distribution function of the Student t distribution with df degrees of freedom.
func TCDF(t, df float64) float64 {
	if math.IsInf(t, 1) {
		return 1
	}
	if math.IsInf(t, -1) {
		return 0
	}
	tail := 0.5 * betaInc(df/2, 0.5, df/(df+t*t))
	if t > 0 {
		return 1 - tail
	}
	return tail
}

// TQuantile is the inverse of TCDF. It finds the value by bisection.
func TQuantile(p, df float64) float64 {
	if p == 0.5 {
		return 0
	}
	if p < 0.5 {
		return -TQuantile(1-p, df)
	}
	lo, hi := 0.0, 1.0
	for TCDF(hi, df) < p && hi < 1e12 {
		hi *= 2
	}
	for range 200 {
		mid := (lo + hi) / 2
		if TCDF(mid, df) < p {
			lo = mid
		} else {
			hi = mid
		}
	}
	return (lo + hi) / 2
}

// betaInc is the regularized incomplete beta function I_x(a, b), by the continued fraction of Lentz.
func betaInc(a, b, x float64) float64 {
	switch {
	case x <= 0:
		return 0
	case x >= 1:
		return 1
	}
	la, _ := math.Lgamma(a + b)
	lb, _ := math.Lgamma(a)
	lc, _ := math.Lgamma(b)
	front := math.Exp(la - lb - lc + a*math.Log(x) + b*math.Log(1-x))
	if x > (a+1)/(a+b+2) {
		return 1 - front*betaCF(b, a, 1-x)/b
	}
	return front * betaCF(a, b, x) / a
}

func betaCF(a, b, x float64) float64 {
	const tiny = 1e-300
	c, d := 1.0, 1-(a+b)*x/(a+1)
	if math.Abs(d) < tiny {
		d = tiny
	}
	d = 1 / d
	h := d
	for m := 1; m <= 500; m++ {
		fm := float64(m)
		num := fm * (b - fm) * x / ((a + 2*fm - 1) * (a + 2*fm))
		d = 1 + num*d
		if math.Abs(d) < tiny {
			d = tiny
		}
		c = 1 + num/c
		if math.Abs(c) < tiny {
			c = tiny
		}
		d = 1 / d
		h *= d * c
		num = -(a + fm) * (a + b + fm) * x / ((a + 2*fm) * (a + 2*fm + 1))
		d = 1 + num*d
		if math.Abs(d) < tiny {
			d = tiny
		}
		c = 1 + num/c
		if math.Abs(c) < tiny {
			c = tiny
		}
		d = 1 / d
		del := d * c
		h *= del
		if math.Abs(del-1) < 1e-15 {
			break
		}
	}
	return h
}

// Welch is the result of a Welch t test of treatment minus control.
type Welch struct {
	Diff, SE, T, DF float64
}

// WelchT computes the Welch t statistic of x minus y.
func WelchT(x, y []float64) Welch {
	vx, vy := Var(x)/float64(len(x)), Var(y)/float64(len(y))
	w := Welch{Diff: Mean(x) - Mean(y), SE: math.Sqrt(vx + vy)}
	if w.SE == 0 {
		w.T = math.Copysign(math.Inf(1), w.Diff)
		if w.Diff == 0 {
			w.T = 0
		}
		w.DF = float64(len(x) + len(y) - 2)
		return w
	}
	w.T = w.Diff / w.SE
	w.DF = (vx + vy) * (vx + vy) / (vx*vx/float64(len(x)-1) + vy*vy/float64(len(y)-1))
	return w
}

// P gives the one-sided p-value of the test in the direction: lower means that x is less than y.
func (w Welch) P(direction string) float64 {
	if direction == "lower" {
		return TCDF(w.T, w.DF)
	}
	return 1 - TCDF(w.T, w.DF)
}

// CI gives the two-sided confidence interval of the difference at the level, such as 0.95.
func (w Welch) CI(level float64) [2]float64 {
	q := TQuantile(1-(1-level)/2, w.DF) * w.SE
	return [2]float64{w.Diff - q, w.Diff + q}
}

// MannWhitney gives the U statistic of x and the one-sided p-value in the direction, with the normal approximation, the tie correction and the continuity correction, as SciPy computes them.
func MannWhitney(x, y []float64, direction string) (u, p float64) {
	n1, n2 := float64(len(x)), float64(len(y))
	all := make([]float64, 0, len(x)+len(y))
	all = append(append(all, x...), y...)
	ranks, ties := rank(all)
	r1 := 0.0
	for i := range x {
		r1 += ranks[i]
	}
	u1 := r1 - n1*(n1+1)/2
	n := n1 + n2
	mu := n1 * n2 / 2
	s := math.Sqrt(n1 * n2 / 12 * ((n + 1) - ties/(n*(n-1))))
	stat := u1
	if direction == "lower" {
		stat = n1*n2 - u1
	}
	if s == 0 {
		return u1, 1
	}
	return u1, 1 - NormCDF((stat-mu-0.5)/s)
}

// rank gives the mid-ranks of xs, from 1, and the sum of t³-t over the groups of ties.
func rank(xs []float64) ([]float64, float64) {
	idx := make([]int, len(xs))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool { return xs[idx[a]] < xs[idx[b]] })
	ranks := make([]float64, len(xs))
	ties := 0.0
	for i := 0; i < len(idx); {
		j := i
		for j+1 < len(idx) && xs[idx[j+1]] == xs[idx[i]] {
			j++
		}
		r := float64(i+j)/2 + 1
		for k := i; k <= j; k++ {
			ranks[idx[k]] = r
		}
		t := float64(j - i + 1)
		ties += t*t*t - t
		i = j + 1
	}
	return ranks, ties
}

// HodgesLehmann gives the median of the differences x[i]-y[j] over all pairs.
func HodgesLehmann(x, y []float64) float64 {
	d := make([]float64, 0, len(x)*len(y))
	for _, a := range x {
		for _, b := range y {
			d = append(d, a-b)
		}
	}
	return Median(d)
}

// Bootstrap draws resamples of x and y with a fixed seed. Each group is resampled on its own. stat gives the statistic of one pair of samples.
type Bootstrap struct {
	Resamples int
	Seed      uint64
}

func (b Bootstrap) draws(x, y []float64, stat func(x, y []float64) float64) []float64 {
	// The bootstrap needs a generator with a seed, so that the same values give the same interval on each run. It is not used for security.
	r := rand.New(rand.NewPCG(b.Seed, b.Seed^0x9e3779b97f4a7c15)) //nolint:gosec // a seeded generator is the point here
	out := make([]float64, b.Resamples)
	sx, sy := make([]float64, len(x)), make([]float64, len(y))
	for i := range out {
		for j := range sx {
			sx[j] = x[r.IntN(len(x))]
		}
		for j := range sy {
			sy[j] = y[r.IntN(len(y))]
		}
		out[i] = stat(sx, sy)
	}
	sort.Float64s(out)
	return out
}

// Percentile gives the percentile interval of the statistic at the level.
func (b Bootstrap) Percentile(x, y []float64, stat func(x, y []float64) float64, level float64) [2]float64 {
	d := b.draws(x, y, stat)
	a := (1 - level) / 2
	return [2]float64{quantile(d, a), quantile(d, 1-a)}
}

// BCa gives the bias-corrected and accelerated interval of the statistic at the level. The acceleration comes from the jackknife over the values of both groups.
func (b Bootstrap) BCa(x, y []float64, stat func(x, y []float64) float64, level float64) [2]float64 {
	d := b.draws(x, y, stat)
	theta := stat(x, y)
	below := 0.0
	for _, v := range d {
		switch {
		case v < theta:
			below++
		case v == theta:
			below += 0.5
		}
	}
	prop := below / float64(len(d))
	lim := 1 / (2 * float64(len(d)))
	prop = math.Min(math.Max(prop, lim), 1-lim)
	z0 := NormQuantile(prop)
	var jack []float64
	for i := range x {
		jack = append(jack, stat(without(x, i), y))
	}
	for i := range y {
		jack = append(jack, stat(x, without(y, i)))
	}
	jm := Mean(jack)
	num, den := 0.0, 0.0
	for _, j := range jack {
		num += (jm - j) * (jm - j) * (jm - j)
		den += (jm - j) * (jm - j)
	}
	acc := 0.0
	if den > 0 {
		acc = num / (6 * math.Pow(den, 1.5))
	}
	adj := func(alpha float64) float64 {
		z := NormQuantile(alpha)
		return NormCDF(z0 + (z0+z)/(1-acc*(z0+z)))
	}
	a := (1 - level) / 2
	return [2]float64{quantile(d, adj(a)), quantile(d, adj(1-a))}
}

func without(xs []float64, i int) []float64 {
	out := make([]float64, 0, len(xs)-1)
	out = append(out, xs[:i]...)
	return append(out, xs[i+1:]...)
}

// quantile gives the quantile of sorted values, with linear interpolation.
func quantile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 || math.IsNaN(p) {
		return math.NaN()
	}
	h := p * float64(len(sorted)-1)
	lo := int(math.Floor(h))
	if lo < 0 {
		return sorted[0]
	}
	if lo >= len(sorted)-1 {
		return sorted[len(sorted)-1]
	}
	return sorted[lo] + (h-float64(lo))*(sorted[lo+1]-sorted[lo])
}

// DiffMeans is the statistic mean(x) - mean(y).
func DiffMeans(x, y []float64) float64 { return Mean(x) - Mean(y) }

// Wilson gives the Wilson score interval of k successes in n trials, for the normal quantile z.
func Wilson(k, n int, z float64) [2]float64 {
	if n == 0 {
		return [2]float64{0, 1}
	}
	p, fn := float64(k)/float64(n), float64(n)
	den := 1 + z*z/fn
	mid := (p + z*z/(2*fn)) / den
	half := z * math.Sqrt(p*(1-p)/fn+z*z/(4*fn*fn)) / den
	return [2]float64{math.Max(0, mid-half), math.Min(1, mid+half)}
}

// Line is the result of a least-squares fit y = A + B x.
type Line struct {
	A, B, R2, SEB float64
	N             int
}

// FitLine fits a line by least squares.
func FitLine(x, y []float64) Line {
	n := float64(len(x))
	mx, my := Mean(x), Mean(y)
	sxx, sxy, syy := 0.0, 0.0, 0.0
	for i := range x {
		sxx += (x[i] - mx) * (x[i] - mx)
		sxy += (x[i] - mx) * (y[i] - my)
		syy += (y[i] - my) * (y[i] - my)
	}
	l := Line{N: len(x)}
	if sxx == 0 {
		return l
	}
	l.B = sxy / sxx
	l.A = my - l.B*mx
	sse := 0.0
	for i := range x {
		e := y[i] - l.A - l.B*x[i]
		sse += e * e
	}
	if syy > 0 {
		l.R2 = 1 - sse/syy
	}
	if n > 2 {
		l.SEB = math.Sqrt(sse / (n - 2) / sxx)
	}
	return l
}

// CI gives the two-sided confidence interval of the slope at the level.
func (l Line) CI(level float64) [2]float64 {
	if l.N <= 2 {
		return [2]float64{math.Inf(-1), math.Inf(1)}
	}
	q := TQuantile(1-(1-level)/2, float64(l.N-2)) * l.SEB
	return [2]float64{l.B - q, l.B + q}
}

// Holm gives the p-values adjusted by the step-down method of Holm.
func Holm(ps []float64) []float64 {
	idx := make([]int, len(ps))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool { return ps[idx[a]] < ps[idx[b]] })
	out := make([]float64, len(ps))
	run := 0.0
	for r, i := range idx {
		v := math.Min(1, float64(len(ps)-r)*ps[i])
		run = math.Max(run, v)
		out[i] = run
	}
	return out
}
