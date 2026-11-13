// SPDX-License-Identifier: Apache-2.0

package status

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/tamnd/ai-agent/exp"
	"github.com/tamnd/ai-agent/node"
)

var rNames = []string{"no experiment", "experiment run", "holds at small scale", "holds at two scales", "reviewed experiment"}

// ExpLevel gives the level R0 to R2 of one experiment, and what the next level needs (spec 9.11). R3 and R4 need more than one experiment, so Repro computes them.
func ExpLevel(c *node.Corpus, e *exp.Experiment) (int, []string) {
	if err := e.Ready(); err != nil {
		return 0, []string{err.Error() + " (R01)"}
	}
	var next []string
	for i, d := range e.Design.Data {
		if d.Generator != "" {
			continue
		}
		p := c.Pins[exp.PinID(d)]
		if p == nil || p.Licence.SPDX == "" || strings.EqualFold(p.Licence.SPDX, "unknown") || len(p.Files) == 0 {
			next = append(next, fmt.Sprintf("pin the data %d of %s in %s with a resolved licence (R02)", i+1, e.Slug, exp.PinsFile))
		}
	}
	r := e.Results
	if r == nil {
		next = append(next, "run aik exp run -update on "+e.Slug+" (R05)")
		return 0, next
	}
	if len(r.Seeds) == 0 || len(r.Primary) == 0 {
		next = append(next, "rerun "+e.Slug+"; results.json has no seeds or no test (R05)")
	}
	if b, ok := exp.Budgets[e.Design.Budget]; !ok || r.Hardware.Class != e.Design.Budget || r.WallTimeS > float64(b.Seconds) {
		next = append(next, "run "+e.Slug+" within the cap of its budget class (R09)")
	}
	if len(next) > 0 {
		return 0, next
	}
	if why := e.Stale(); why != "" {
		return 1, []string{"rerun " + e.Slug + ": " + why}
	}
	if strings.TrimSpace(e.Design.ScaleNote.Here) == "" || strings.TrimSpace(e.Design.ScaleNote.Source) == "" {
		return 1, []string{"give " + e.Slug + " a scale note with here and source (R03)"}
	}
	switch r.Verdict {
	case exp.Holds:
		return 2, nil
	case exp.DidNotHold:
		return 1, []string{"the claim did not hold at the scale of " + e.Slug}
	}
	return 1, []string{"the test of " + e.Slug + " is inconclusive; it needs more seeds or a smaller claim (R07)"}
}

// smallClaims gives the claims whose R level counts: the claims marked small_scale: yes, or, when there are none, the claims that an experiment tests.
func smallClaims(n *node.Node) []string {
	var out []string
	for _, c := range n.Claims {
		if c.SmallScale == "yes" {
			out = append(out, c.ID)
		}
	}
	if len(out) > 0 {
		return out
	}
	for _, e := range n.Exps {
		if e.Design != nil {
			for _, id := range e.Design.Claims {
				if !slices.Contains(out, id) {
					out = append(out, id)
				}
			}
		}
	}
	return out
}

// Repro computes the R ladder of a node (spec 9.11). It returns nil when the node has no experiment and no claim with a small-scale form.
func Repro(n *node.Node, env Env) *Rung {
	ids := smallClaims(n)
	if len(n.Exps) == 0 && len(ids) == 0 {
		return nil
	}
	r := &Rung{Max: 4, Level: 4}
	defer func() { r.Label, r.Name = fmt.Sprintf("R%d", r.Level), rNames[r.Level] }()
	if len(ids) == 0 {
		r.Level = 0
		r.Next = []string{"name the claims of claims.yaml that the experiments test (R01)"}
		return r
	}
	levels := map[*exp.Experiment]int{}
	nexts := map[*exp.Experiment][]string{}
	for _, e := range n.Exps {
		levels[e], nexts[e] = ExpLevel(env.Corpus, e)
	}
	for _, id := range ids {
		lv, next := claimLevel(n, id, levels, nexts, env)
		if lv < r.Level {
			r.Level, r.Next = lv, nil
		}
		if lv == r.Level {
			r.Next = append(r.Next, next...)
		}
	}
	if r.Level == 4 {
		r.Next = nil
	}
	return r
}

// claimLevel gives the R level of one claim: the best experiment that tests it, and R3 and R4 from the passing experiments at more than one scale.
func claimLevel(n *node.Node, id string, levels map[*exp.Experiment]int, nexts map[*exp.Experiment][]string, env Env) (int, []string) {
	var tests, passed []*exp.Experiment
	for _, e := range n.Exps {
		if e.Design != nil && slices.Contains(e.Design.Claims, id) {
			tests = append(tests, e)
			if levels[e] >= 2 {
				passed = append(passed, e)
			}
		}
	}
	if len(tests) == 0 {
		return 0, []string{"write an experiment for the claim " + id}
	}
	best, next := 0, []string(nil)
	for _, e := range tests {
		if levels[e] > best || next == nil {
			if levels[e] >= best {
				best, next = levels[e], nexts[e]
			}
		}
	}
	if best < 2 {
		return best, next
	}
	if !twoScales(passed) {
		return 2, []string{"pass a second experiment of the claim " + id + " at another scale or on other data (R13)"}
	}
	for _, e := range passed {
		ok, why := Reviewed(env.root(), n.Tag, "exp/"+e.Slug, filepath.Join(e.Dir, exp.ResultsFile))
		if !ok {
			return 3, []string{"a person reviews the design and the results of " + e.Slug + ": " + why}
		}
	}
	return 4, nil
}

// twoScales tells if two of the passing experiments differ in their data or in their scale.
func twoScales(es []*exp.Experiment) bool {
	for i, a := range es {
		for _, b := range es[i+1:] {
			if a.Design.ScaleNote.Here != b.Design.ScaleNote.Here || fmt.Sprint(a.Design.Data) != fmt.Sprint(b.Design.Data) {
				return true
			}
		}
	}
	return false
}

// Measured tells, for each claim of the node, if an experiment at R2 or above tests it, and if an experiment that reruns the setting of the source finds that it did not hold (spec 9.7). It is the Measured of the claims.Env of the build.
func Measured(n *node.Node, env Env) map[string][2]bool {
	out := map[string][2]bool{}
	for _, e := range n.Exps {
		lv, _ := ExpLevel(env.Corpus, e)
		if e.Design == nil {
			continue
		}
		for _, id := range e.Design.Claims {
			m := out[id]
			if lv >= 2 {
				m[0] = true
			}
			if lv >= 1 && e.Results != nil && e.Results.Verdict == exp.DidNotHold && e.Design.Replicates != nil {
				m[1] = true
			}
			out[id] = m
		}
	}
	return out
}
