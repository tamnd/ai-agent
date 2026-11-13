// SPDX-License-Identifier: Apache-2.0

package site

import (
	"encoding/json"
	"fmt"

	"github.com/tamnd/ai-agent/results"
)

// ResultsPage is the page of one benchmark version with its computed results table (spec 10.7).
type ResultsPage struct {
	Base
	Heading   string
	Benchmark Link
	Summary   string // the line with the date, the count and the number of groups
	Hidden    string // the line on the results that the citation check did not find
	Groups    []ResultGroup
	Other     []ResultGroup
	Versions  []Link // the other versions of the benchmark; a table never merges them (10.6.3)
	JSON      string
}

// ResultGroup is one comparable group of the table.
type ResultGroup struct {
	Metric    string
	Setting   string
	Rows      []ResultRow
	Contested []ResultRow // the contested rows, which the page shows behind a toggle
}

// ResultRow is one row of a results table.
type ResultRow struct {
	Rank     int
	Anchor   string
	Model    Link
	Variant  string
	Score    string
	Source   string
	Loc      string
	Date     string
	Evidence string
	Self     bool
}

// resultTables writes the page and the JSON file of each benchmark version.
func (b *builder) resultTables() error {
	reg := results.Load(b.c)
	for _, bench := range reg.Benchmarks() {
		vs := reg.Versions[bench]
		name, benchURL := bench, ""
		if n := b.c.BySlug[bench]; n != nil {
			name, benchURL = n.Name, "/"+n.Slug+"/"
		} else if c := b.c.CardByKey[bench]; c != nil {
			name, benchURL = c.Name, cardURL(c)
		}
		ids := sortedKeys(vs)
		for _, id := range ids {
			v := vs[id]
			p := "/benchmarks/" + bench + "/" + id + "/"
			t := reg.TableOf(bench, id, nil)
			heading := v.Name
			if heading == "" {
				heading = name + ", " + id
			}
			rp := ResultsPage{Base: b.base(heading, "The computed results table of "+heading+".", p), Heading: heading, Benchmark: Link{Name: name, URL: benchURL}, JSON: p + "results.json"}
			switch {
			case t.Hidden == 1:
				rp.Hidden = "1 more result is not shown, because the citation check did not find its number in the source."
			case t.Hidden > 1:
				rp.Hidden = fmt.Sprintf("%d more results are not shown, because the citation check did not find their numbers in the source.", t.Hidden)
			}
			rp.Crumbs = []Link{{Name: "Benchmarks", URL: "/benchmarks/"}, {Name: name, URL: benchURL}}
			rp.Summary = fmt.Sprintf("Results as of %s. %d %s in %d comparable %s.", b.meta.Date, t.Count, plural(t.Count, "result", "results"), len(t.Groups)+len(t.Other), plural(len(t.Groups)+len(t.Other), "group", "groups"))
			for _, g := range t.Groups {
				rp.Groups = append(rp.Groups, b.resultGroup(g))
			}
			for _, g := range t.Other {
				rp.Other = append(rp.Other, b.resultGroup(g))
			}
			for _, o := range ids {
				if o != id {
					rp.Versions = append(rp.Versions, Link{Name: vs[o].Name, URL: "/benchmarks/" + bench + "/" + o + "/"})
				}
			}
			if err := b.page("results", p, rp); err != nil {
				return err
			}
			data, err := json.Marshal(t)
			if err != nil {
				return err
			}
			if err := b.write(rp.JSON, data); err != nil {
				return err
			}
		}
	}
	return nil
}

func (b *builder) resultGroup(g results.Group) ResultGroup {
	out := ResultGroup{Metric: g.Metric, Setting: g.Setting}
	rank := 0
	for _, r := range g.Rows {
		row := ResultRow{Anchor: "r-" + r.Tag, Model: b.modelLink(r.Model), Variant: r.ModelVariant, Score: scoreText(r.Score), Source: r.Source.Src, Loc: r.Source.Loc, Date: r.Source.Date, Evidence: r.Evidence, Self: r.SelfReported}
		if r.Evidence == results.Contested {
			out.Contested = append(out.Contested, row)
			continue
		}
		rank++
		row.Rank = rank
		out.Rows = append(out.Rows, row)
	}
	return out
}

// modelLink links a model to its page or to the row of its card.
func (b *builder) modelLink(slug string) Link {
	if n := b.c.BySlug[slug]; n != nil {
		return Link{Name: n.Name, URL: "/" + n.Slug + "/"}
	}
	for _, c := range b.c.Cards {
		if c.Kind == "model" && c.Slug == slug {
			return Link{Name: c.Name, URL: cardURL(c)}
		}
	}
	return Link{Name: slug, Missing: true}
}

func scoreText(s results.Score) string {
	switch {
	case s.V == nil:
		return "?"
	case s.Unit == "fraction":
		return fmt.Sprintf("%.1f%%", 100**s.V)
	case len(s.Interval) == 2:
		return fmt.Sprintf("%g [%g, %g]", *s.V, s.Interval[0], s.Interval[1])
	}
	return fmt.Sprintf("%g", *s.V)
}
