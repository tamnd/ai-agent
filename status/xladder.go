// SPDX-License-Identifier: Apache-2.0

package status

import (
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/tamnd/ai-agent/node"
)

// Frameworks are the frameworks of spec 08.2 whose versions climb the execution ladder X0 to X4 (08.10) instead of the
// ladder of the algorithm implementations. lang.Frameworks maps each one to the language of its code.
var Frameworks = map[string]bool{"numpy": true, "pytorch": true, "jax": true}

// Reference is the framework of the reference version, which every other framework agrees with (rule X01).
const Reference = "numpy"

// XMinScore is the mutation score of rule X06: lower than the 85% of I04, because numerical mutants near the
// tolerances are harder to kill without brittle tests (08.8).
const XMinScore = 0.80

// MinScoreFor is the mutation score that an implementation in lang needs.
func MinScoreFor(lang string) float64 {
	if Frameworks[lang] {
		return XMinScore
	}
	return MinScore
}

// LintTools are the checks of 8.3.2 that X3 needs, by the name that Lint.Tool has.
var LintTools = []string{"ruff format", "ruff check", "pyright"}

var xNames = []string{"not checked yet", "runs and passes its tests", "agrees with the reference", "clean and mutation-tested", "reviewed"}

// A required test of X2, found by name in the JUnit report (08.10): the test passes when a test with the name, or
// with the name and a suffix after _, passed and none failed.
type xTest struct {
	name, rule, why string
}

var hasBackward = regexp.MustCompile(`(?m)^def backward\(`)

// xTests lists the tests that X1 and X2 need from a framework version of a node of the kind.
func xTests(n *node.Node, im node.Impl) (x1, x2 []xTest) {
	ref := im.Lang == Reference
	switch n.Kind {
	case "method", "architecture":
		if ref {
			if backward(im) {
				x1 = append(x1, xTest{"test_gradient", "X04", "the hand-written backward against finite differences (8.7.4)"})
			}
			x2 = append(x2, xTest{"test_shared_fixture", "X02", "the shared fixtures that the reference made (8.7.2)"})
		} else {
			x2 = append(x2,
				xTest{"test_shared_fixture", "X02", "agreement with the NumPy reference on the shared fixtures (8.7.2)"},
				xTest{"test_gradient", "X04", "the autodiff gradients against finite differences (8.7.4)"})
		}
	}
	if n.Kind == "architecture" {
		x2 = append(x2,
			xTest{"test_shapes", "X09", "the shapes of shapes.yaml at each config (8.6)"},
			xTest{"test_param_count", "X09", "the parameter count of shapes.yaml at each config (8.6)"})
		// Learning needs gradients: a NumPy reference with no backward cannot train, and the frameworks with autodiff
		// check that the model learns.
		if !ref || backward(im) {
			x2 = append(x2, xTest{"test_learns", "8.7.6", "the learning smoke test: the model fits a tiny fixed batch on CPU"})
		}
	}
	return x1, x2
}

// backward tells if the main file of the implementation writes a backward pass by hand (8.3.4).
func backward(im node.Impl) bool {
	m := im.Main()
	return m != nil && hasBackward.MatchString(m.Content)
}

func (x xTest) passed(t Test) bool {
	match := func(s string) bool { return s == x.name || strings.HasPrefix(s, x.name+"_") }
	return slices.ContainsFunc(t.Passed, match) && !slices.ContainsFunc(t.Failed, match)
}

func missing(t Test, need []xTest) []string {
	var out []string
	for _, x := range need {
		if !x.passed(t) {
			out = append(out, fmt.Sprintf("a passing %s: %s (%s)", x.name, x.why, x.rule))
		}
	}
	return out
}

// xImpl is the execution ladder of one framework version (08.10).
func xImpl(n *node.Node, im node.Impl, env Env) (r Rung) {
	r = Rung{Max: 4}
	defer func() { r.Label, r.Name = fmt.Sprintf("X%d", r.Level), xNames[r.Level] }()
	if im.Lang != Reference && !slices.ContainsFunc(n.Impls, func(o node.Impl) bool { return o.Lang == Reference }) {
		r.Next = append(r.Next, "the NumPy reference comes before any other framework version (X01)")
		return r
	}
	t, ran := env.Results[n.Slug][im.Lang]
	switch {
	case !im.HasTests():
		r.Next = append(r.Next, "add native tests (8.7.1)")
		return r
	case !ran:
		r.Next = append(r.Next, "the tests did not run in this build")
		return r
	case !t.Pass || len(t.Failed) > 0:
		why := "the native tests fail (X03)"
		if len(t.Failed) > 0 {
			why += ": " + strings.Join(t.Failed, ", ")
		}
		if t.Vectors != nil && !t.Vectors.OK() {
			why = vectorsNext(*t.Vectors)
		}
		r.Next = append(r.Next, why)
		return r
	}
	x1, x2 := xTests(n, im)
	if m := missing(t, x1); len(m) > 0 {
		r.Next = append(r.Next, m...)
		return r
	}
	r.Level = 1
	need := missing(t, x2)
	if n.Kind == "metric" {
		switch {
		case t.Vectors == nil:
			need = append(need, "the test vectors did not run in this build (X05)")
		case !t.Vectors.OK():
			need = append(need, vectorsNext(*t.Vectors))
		}
	}
	if len(need) > 0 {
		r.Next = append(r.Next, prefix("X2 needs ", need)...)
		return r
	}
	r.Level = 2
	need = lintNext(t)
	switch m := t.Mutation; {
	case m == nil:
		need = append(need, "the mutation run (aik test -kind mutate) did not run in this build (X06)")
	case m.Error != "":
		need = append(need, "the mutation run failed: "+firstLine(m.Error))
	case m.Score() < XMinScore:
		need = append(need, fmt.Sprintf("the tests kill %d of %d mutants (%.0f%%), and X3 needs %.0f%% (X06); the first survivor is %s",
			m.Killed, m.Mutants-m.Ignored, 100*m.Score(), 100*XMinScore, first(m.Survivors)))
	}
	if len(need) > 0 {
		r.Next = append(r.Next, prefix("X3 needs ", need)...)
		return r
	}
	r.Level = 3
	main := im.Main()
	if main == nil {
		return r
	}
	ok, why := Reviewed(env.root(), n.Tag, "impl-"+im.Lang, filepath.Join(env.root(), filepath.FromSlash(main.Path)))
	if !ok {
		r.Next = append(r.Next, "X4 needs a review of this exact file by a person who is not the author (15.5): "+why)
		return r
	}
	r.Level = 4
	return r
}

func vectorsNext(v Vectors) string {
	switch {
	case v.Error != "":
		return "the test vectors did not run (X05): " + firstLine(v.Error)
	case len(v.Failures) > 0:
		f := v.Failures[0]
		return fmt.Sprintf("%d of %d test vectors fail (X05), the first is %s: %s", v.Cases-v.Passed, v.Cases, f.Case, f.Reason)
	default:
		return "the metric has no test vectors (X05)"
	}
}

func lintNext(t Test) []string {
	var out []string
	for _, tool := range LintTools {
		i := slices.IndexFunc(t.Lint, func(l Lint) bool { return l.Tool == tool })
		switch {
		case i < 0:
			out = append(out, tool+" did not run in this build (X13)")
		case t.Lint[i].Missing:
			out = append(out, tool+" is not installed in the image (X13)")
		case !t.Lint[i].Pass:
			out = append(out, tool+" is not clean (X13): "+firstLine(t.Lint[i].Output))
		}
	}
	return out
}

func prefix(p string, s []string) []string {
	out := make([]string, len(s))
	for i, x := range s {
		out[i] = p + x
	}
	return out
}

func first(s []string) string {
	if len(s) == 0 {
		return "not recorded"
	}
	return s[0]
}
