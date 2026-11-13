// SPDX-License-Identifier: Apache-2.0

package check

import (
	"slices"
	"strings"

	"github.com/tamnd/ai-agent/facts"
	"github.com/tamnd/ai-agent/results"
)

// register checks the result cards and the versions of the benchmarks (spec 10.13).
func (r *run) register() {
	reg := results.Load(r.c)
	for _, err := range reg.VersionErrs {
		r.add("T01", Block, rel(r.c.Root, err.Error()), 0, "%v", err)
	}
	for _, res := range reg.Results {
		r.result(reg, res)
	}
}

func (r *run) kindOf(slug string) string {
	if n := r.c.BySlug[slug]; n != nil {
		return n.Kind
	}
	if c := r.c.CardByKey[slug]; c != nil {
		return c.Kind
	}
	for _, c := range r.c.Cards {
		if c.Slug == slug && c.Kind == "model" {
			return c.Kind
		}
	}
	return ""
}

func (r *run) result(reg *results.Register, res *results.Result) {
	c := res.Card
	p, l := c.File, c.Line
	if res.Err != nil {
		r.add("T01", Block, p, l, "the result %s: %v", c.Tag, res.Err)
		return
	}
	need := []struct{ name, v string }{
		{"model", res.Model}, {"benchmark", res.Benchmark}, {"version", res.Version}, {"metric", res.Metric},
		{"score.unit", res.Score.Unit}, {"source.src", res.Source.Src}, {"source.loc", res.Source.Loc}, {"source.date", res.Source.Date},
	}
	for _, f := range need {
		if strings.TrimSpace(f.v) == "" {
			r.add("M01", Block, p, l, "the result %s has no %s", c.Tag, f.name)
		}
	}
	if res.Score.V == nil {
		r.add("M01", Block, p, l, "the result %s has no score.v", c.Tag)
	}
	if len(res.Setting) == 0 {
		r.add("M01", Block, p, l, "the result %s has no setting; write the keys of 10.5 that the source states", c.Tag)
	}
	if res.Source.Date != "" && !facts.IsDate(res.Source.Date) {
		r.add("M01", Block, p, l, "the result %s has the source date %q; write a date such as 2024-07-23", c.Tag, res.Source.Date)
	}
	if res.Model != "" && r.kindOf(res.Model) != "model" {
		r.add("M01", Block, p, l, "the model %q of the result %s is not a model node or card", res.Model, c.Tag)
	}
	if res.Metric != "" && r.kindOf(res.Metric) != "metric" {
		r.add("M01", Block, p, l, "the metric %q of the result %s is not a metric node or card", res.Metric, c.Tag)
	}
	if res.Benchmark != "" {
		switch {
		case r.kindOf(res.Benchmark) != "benchmark":
			r.add("M02", Block, p, l, "the result %s names %q, which is not a benchmark; a result attaches to a version of a benchmark", c.Tag, res.Benchmark)
		case res.Version != "" && reg.Versions[res.Benchmark][res.Version] == nil:
			r.add("M02", Block, p, l, "the benchmark %s has no version %q; add it to its versions (10.3)", res.Benchmark, res.Version)
		}
	}
	for k, v := range res.Setting {
		if !slices.Contains(results.SettingKeys, k) {
			r.add("M03", Level, p, l, "the setting key %q of the result %s is not a key of 10.5", k, c.Tag)
		}
		if s, ok := v.(string); v == nil || ok && strings.TrimSpace(s) == "" {
			r.add("M03", Level, p, l, "the setting key %s of the result %s is empty; write %s when the source does not state it", k, c.Tag, results.Unknown)
		}
	}
	switch res.Checks.Cite {
	case "", results.CiteFound:
	case results.CiteNotFound:
		r.add("M08", Warn, p, l, "the citation check did not find the number of the result %s in its source; no table shows it", c.Tag)
	default:
		r.add("T01", Block, p, l, "the result %s has checks.cite %q; it is %s or %s", c.Tag, res.Checks.Cite, results.CiteFound, results.CiteNotFound)
	}
	if res.Metric == "rating" {
		if res.Arena == "" || !facts.IsDate(res.Snapshot) || res.Score.Votes < 1 || len(res.Score.Interval) != 2 {
			r.add("M14", Level, p, l, "the rating %s needs arena, snapshot (a date), score.votes and score.interval (10.10)", c.Tag)
		}
	}
}
