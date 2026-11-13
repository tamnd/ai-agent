// SPDX-License-Identifier: Apache-2.0

package site

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/tamnd/ai-agent/exp"
	"github.com/tamnd/ai-agent/node"
	"github.com/tamnd/ai-agent/status"
)

// ExpView is the experiment box of a node page (spec 9.7).
type ExpView struct {
	ID       string
	Claim    string // the text of the claim, as the page states it
	ClaimID  string
	Question string
	Here     string
	Source   string
	Level    string // the R level of the experiment, for example "R2"
	Run      bool   // results.json is there
	Stale    string
	Metric   string
	Arms     []ExpArm
	Tests    []ExpTest
	Verdict  string // the sentence of the verdict, from the template of 9.7
	Class    string
	Figures  []string
	Links    []Link
	Command  string
	Small    bool // the experiment is a small-scale check, not a rerun of the setting of the source (rule R04)
}

// ExpArm is the primary metric of one arm: its mean and its 95% confidence interval over the seeds.
type ExpArm struct {
	ID    string
	Mean  string
	CI    string
	Seeds int
}

// ExpTest is one primary test of expect.yaml with its result.
type ExpTest struct {
	Test    string
	Metric  string
	P       string
	CI      string
	Effect  string
	Verdict string
}

var verdictText = map[string]string{
	exp.Holds:        "The claim holds at this scale.",
	exp.DidNotHold:   "The claim did not hold at this scale.",
	exp.Inconclusive: "Inconclusive at this scale (the test had too little power).",
}

// expViews gives the experiment boxes of the node, and copies the figures of the runs to the site.
func (b *builder) expViews(n *node.Node) []ExpView {
	text := map[string]string{}
	for _, c := range n.Claims {
		text[c.ID] = c.Text
	}
	out := make([]ExpView, 0, len(n.Exps))
	for _, e := range n.Exps {
		d := e.Design
		if d == nil {
			continue
		}
		lv, _ := status.ExpLevel(b.c, e)
		v := ExpView{ID: e.Slug, Question: d.Question, Here: d.ScaleNote.Here, Source: d.ScaleNote.Source, Level: fmt.Sprintf("R%d", lv), Small: d.Replicates == nil}
		v.ClaimID = firstOf(d.Claims)
		if e.Expect != nil && e.Expect.Claim != "" {
			v.ClaimID = e.Expect.Claim
		}
		v.Claim = text[v.ClaimID]
		v.Command = "aik exp run " + n.Slug + "/" + e.Slug
		blob := fmt.Sprintf("%s/blob/main/%s/", b.opt.Repo, e.Rel)
		v.Links = []Link{{Name: exp.Script, URL: blob + exp.Script}, {Name: exp.DesignFile, URL: blob + exp.DesignFile}, {Name: exp.ExpectFile, URL: blob + exp.ExpectFile}}
		if r := e.Results; r != nil {
			v.Run = true
			v.Stale = e.Stale()
			v.Links = append(v.Links, Link{Name: exp.ResultsFile, URL: blob + exp.ResultsFile})
			v.Verdict, v.Class = verdictText[r.Verdict], "verdict-"+r.Verdict
			if e.Expect != nil && len(e.Expect.Primary) > 0 {
				v.Metric = e.Expect.Primary[0].Metric
			}
			v.Arms = armViews(d, r, v.Metric)
			for _, t := range r.Primary {
				v.Tests = append(v.Tests, testView(t))
			}
			v.Figures = b.copyFigures(n, e)
		}
		out = append(out, v)
	}
	return out
}

func firstOf(xs []string) string {
	if len(xs) == 0 {
		return ""
	}
	return xs[0]
}

func armViews(d *exp.Design, r *exp.Results, metric string) []ExpArm {
	var out []ExpArm
	for _, a := range d.Arms {
		s, ok := r.Arms[a.ID][metric]
		if !ok {
			continue
		}
		v := ExpArm{ID: a.ID, Mean: num(s.Mean), Seeds: len(s.PerSeed)}
		if k := len(s.PerSeed); k > 1 {
			h := exp.TQuantile(0.975, float64(k-1)) * s.SD / math.Sqrt(float64(k))
			v.CI = "[" + num(s.Mean-h) + ", " + num(s.Mean+h) + "]"
		}
		out = append(out, v)
	}
	return out
}

func testView(t exp.Verdict) ExpTest {
	v := ExpTest{Test: t.Test, Metric: t.Metric, Verdict: t.Verdict}
	if t.P != nil {
		v.P = fmt.Sprintf("%.3g", *t.P)
	}
	if len(t.CI95) == 2 {
		v.CI = "[" + num(t.CI95[0]) + ", " + num(t.CI95[1]) + "]"
	}
	switch {
	case t.EffectRelative != nil:
		v.Effect = fmt.Sprintf("%+.1f%%", 100**t.EffectRelative)
	case t.Effect != nil:
		v.Effect = num(*t.Effect)
	}
	return v
}

func num(x float64) string { return fmt.Sprintf("%.4g", x) }

// copyFigures copies the SVG figures of the run to the page directory of the node, and gives their paths on the site.
func (b *builder) copyFigures(n *node.Node, e *exp.Experiment) []string {
	var out []string
	for _, f := range e.Results.Figures {
		if !strings.HasPrefix(f, exp.Figures+"/") || strings.Contains(f, "..") || filepath.Ext(f) != ".svg" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(e.Dir, filepath.FromSlash(f)))
		if err != nil {
			continue
		}
		p := "/" + n.Slug + "/exp/" + e.Slug + "/" + f
		if err := b.write(p, data); err != nil {
			b.opt.Log("figure %s: %v", p, err)
			continue
		}
		out = append(out, p)
	}
	slices.Sort(out)
	return out
}
