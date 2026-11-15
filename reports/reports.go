// SPDX-License-Identifier: Apache-2.0

// Package reports makes the reports of aik report levels, tests and experiments (20.4): the ladder levels of the nodes,
// the test matrix of the implementations and the experiment matrix (03, reports/). Each report is a function of the
// tree and the build results, with no date, so a report written to reports/ changes only when the state does.
package reports

import (
	"sort"
	"strconv"
	"strings"

	"github.com/tamnd/ai-agent/exp"
	"github.com/tamnd/ai-agent/node"
	"github.com/tamnd/ai-agent/status"
)

// Counts are the number of nodes at each level of a ladder, from level 0.
type Counts []int

func (c Counts) add(level, max int) Counts {
	for len(c) <= max || len(c) <= level {
		c = append(c, 0)
	}
	c[level]++
	return c
}

// Ladder is the count of one ladder of 04.10 over the nodes, in total, by kind and by top area. The I and X ladders
// count each implementation, so a node with three counts three times.
type Ladder struct {
	ID     string            `json:"id"`
	Name   string            `json:"name"`
	Levels []string          `json:"levels"`
	Total  Counts            `json:"total"`
	ByKind map[string]Counts `json:"by_kind"`
	ByArea map[string]Counts `json:"by_area"`
	ByLang map[string]Counts `json:"by_lang,omitempty"` // the I and X ladders
}

// Levels is the report of aik report levels.
type Levels struct {
	Nodes   int      `json:"nodes"`
	Ladders []Ladder `json:"ladders"`
}

// TopArea is the first part of an area path.
func TopArea(area string) string {
	top, _, _ := strings.Cut(area, "/")
	if top == "" {
		return "none"
	}
	return top
}

// MakeLevels counts the ladders of the nodes. st holds the ladders of each node, as aik coverage computes them.
func MakeLevels(c *node.Corpus, st map[string]status.Status) Levels {
	by := map[string]*Ladder{}
	for _, l := range status.Ladders {
		by[l.ID] = &Ladder{ID: l.ID, Name: l.Name, Levels: l.Levels, ByKind: map[string]Counts{}, ByArea: map[string]Counts{}}
	}
	out := Levels{}
	for _, n := range sortedNodes(c) {
		s, ok := st[n.Slug]
		if !ok {
			continue
		}
		out.Nodes++
		area := TopArea(n.Area)
		count := func(r status.Rung, lang string) {
			id := strings.TrimRight(r.Label, "0123456789")
			l := by[id]
			if l == nil {
				return
			}
			l.Total = l.Total.add(r.Level, len(l.Levels)-1)
			l.ByKind[n.Kind] = l.ByKind[n.Kind].add(r.Level, len(l.Levels)-1)
			l.ByArea[area] = l.ByArea[area].add(r.Level, len(l.Levels)-1)
			if lang != "" {
				if l.ByLang == nil {
					l.ByLang = map[string]Counts{}
				}
				l.ByLang[lang] = l.ByLang[lang].add(r.Level, len(l.Levels)-1)
			}
		}
		for _, r := range []status.Rung{s.W, s.G, s.H, s.L, s.V} {
			count(r, "")
		}
		for _, r := range []*status.Rung{s.D, s.R} {
			if r != nil {
				count(*r, "")
			}
		}
		for lang, r := range s.I {
			count(r, lang)
		}
	}
	for _, l := range status.Ladders {
		if x := by[l.ID]; len(x.Total) > 0 {
			out.Ladders = append(out.Ladders, *x)
		}
	}
	return out
}

func sortedNodes(c *node.Corpus) []*node.Node {
	var ns []*node.Node
	for _, n := range c.Nodes {
		if n.Slug != "" && c.BySlug[n.Slug] == n {
			ns = append(ns, n)
		}
	}
	sort.Slice(ns, func(i, j int) bool { return ns[i].Slug < ns[j].Slug })
	return ns
}

// TestRow is one implementation of the test matrix.
type TestRow struct {
	Slug  string `json:"slug"`
	Lang  string `json:"lang"`
	Level string `json:"level"` // the I or X level
	// State is pass, fail or not run: the native tests and the shared cases.
	State    string  `json:"state"`
	Ms       int64   `json:"ms,omitempty"`
	Shared   [2]int  `json:"shared,omitempty"` // the shared cases that passed, of those that ran
	Diff     [2]int  `json:"diff,omitempty"`   // the differential cases that passed, of those that ran
	Mutation float64 `json:"mutation,omitempty"`
}

// LangTests is the count of the implementations of one language by state.
type LangTests struct {
	Lang   string `json:"lang"`
	Impls  int    `json:"impls"`
	Pass   int    `json:"pass"`
	Fail   int    `json:"fail"`
	NotRun int    `json:"not_run"`
}

// Tests is the report of aik report tests.
type Tests struct {
	Langs []LangTests `json:"langs"`
	Total LangTests   `json:"total"`
	Rows  []TestRow   `json:"rows"`
}

// MakeTests makes the test matrix: each implementation of each node with the state of its tests in the build results.
func MakeTests(c *node.Corpus, st map[string]status.Status) Tests {
	by := map[string]*LangTests{}
	out := Tests{Total: LangTests{Lang: "total"}, Rows: []TestRow{}, Langs: []LangTests{}}
	for _, n := range sortedNodes(c) {
		s := st[n.Slug]
		langs := make([]string, 0, len(n.Impls))
		for _, im := range n.Impls {
			langs = append(langs, im.Lang)
		}
		sort.Strings(langs)
		for _, lang := range langs {
			row := TestRow{Slug: n.Slug, Lang: lang, Level: s.I[lang].Label, State: "not run"}
			if t, ok := s.Tests[lang]; ok {
				row.Ms = t.Ms
				row.State = "fail"
				if t.OK() {
					row.State = "pass"
				}
				for _, sh := range t.Shared {
					row.Shared[0] += sh.Passed
					row.Shared[1] += sh.Cases - sh.Skipped
				}
				for _, d := range t.Diff {
					row.Diff[0] += d.Passed
					row.Diff[1] += d.Cases - d.Skipped
				}
				if t.Mutation != nil && t.Mutation.Error == "" {
					row.Mutation = t.Mutation.Score()
				}
			}
			out.Rows = append(out.Rows, row)
			lt := by[lang]
			if lt == nil {
				lt = &LangTests{Lang: lang}
				by[lang] = lt
			}
			for _, x := range []*LangTests{lt, &out.Total} {
				x.Impls++
				switch row.State {
				case "pass":
					x.Pass++
				case "fail":
					x.Fail++
				default:
					x.NotRun++
				}
			}
		}
	}
	for _, lt := range by {
		out.Langs = append(out.Langs, *lt)
	}
	sort.Slice(out.Langs, func(i, j int) bool { return out.Langs[i].Lang < out.Langs[j].Lang })
	return out
}

// ExpRow is one experiment of the experiment matrix.
type ExpRow struct {
	Node    string   `json:"node"`
	Exp     string   `json:"exp"`
	Claims  []string `json:"claims,omitempty"`
	Budget  string   `json:"budget,omitempty"`
	Arms    int      `json:"arms"`
	Seeds   int      `json:"seeds"`
	Level   string   `json:"level"`             // R0 to R2, the level of this experiment alone (9.11)
	Verdict string   `json:"verdict,omitempty"` // the verdict of results.json, or empty when it has not run
	Run     string   `json:"run,omitempty"`
	Hours   float64  `json:"hours,omitempty"`
	CostUSD float64  `json:"cost_usd,omitempty"`
	Next    []string `json:"next,omitempty"`
}

// Experiments is the report of aik report experiments.
type Experiments struct {
	Total     int            `json:"total"`
	ByLevel   map[string]int `json:"by_level"`
	ByVerdict map[string]int `json:"by_verdict"`
	ByBudget  map[string]int `json:"by_budget"`
	Hours     float64        `json:"hours"`
	CostUSD   float64        `json:"cost_usd"`
	Rows      []ExpRow       `json:"rows"`
}

// MakeExperiments makes the experiment matrix: each experiment with its claims, its budget, its verdict and its level.
func MakeExperiments(c *node.Corpus) Experiments {
	out := Experiments{ByLevel: map[string]int{}, ByVerdict: map[string]int{}, ByBudget: map[string]int{}, Rows: []ExpRow{}}
	for _, n := range sortedNodes(c) {
		for _, e := range n.Exps {
			row := ExpRow{Node: n.Slug, Exp: e.Slug}
			if d := e.Design; d != nil {
				row.Claims, row.Budget, row.Arms, row.Seeds = d.Claims, d.Budget, len(d.Arms), d.Seeds.N
			}
			lvl, next := status.ExpLevel(c, e)
			row.Level, row.Next = levelLabel(lvl), next
			verdict := "not run"
			if r := e.Results; r != nil {
				row.Verdict, row.Run = r.Verdict, r.Run
				row.Hours, row.CostUSD = r.WallTimeS/3600, r.CostUSD
				verdict = r.Verdict
			}
			out.Rows = append(out.Rows, row)
			out.Total++
			out.ByLevel[row.Level]++
			out.ByVerdict[verdict]++
			budget := row.Budget
			if budget == "" {
				budget = "none"
			}
			out.ByBudget[budget]++
			out.Hours += row.Hours
			out.CostUSD += row.CostUSD
		}
	}
	return out
}

func levelLabel(l int) string { return "R" + strconv.Itoa(l) }

// Verdicts are the verdicts of results.json, for the order of a printed report.
var Verdicts = []string{exp.Holds, exp.DidNotHold, exp.Inconclusive, "not run"}
