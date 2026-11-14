// SPDX-License-Identifier: Apache-2.0

package harvest

import (
	"context"
	"encoding/csv"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Adapter reads one source into inventory rows.
type Adapter struct {
	Source string
	// Gap is the least time between two requests to one host; it matches the rate of the source's row.
	Gap time.Duration
	Run func(ctx context.Context, r *Run, o Options) ([]Row, error)
}

// Adapters are the adapters by source id.
var Adapters = map[string]Adapter{
	"epoch-ai":             {Source: "epoch-ai", Gap: 2 * time.Second, Run: harvestEpoch},
	"epoch-benchmarks":     {Source: "epoch-benchmarks", Gap: 2 * time.Second, Run: harvestEpochBench},
	"pwc-dump":             {Source: "pwc-dump", Gap: 2 * time.Second, Run: harvestPwC},
	"pwc-results":          {Source: "pwc-results", Gap: 2 * time.Second, Run: harvestPwCResults},
	"aiid":                 {Source: "aiid", Gap: 2 * time.Second, Run: harvestAIID},
	"arxiv":                {Source: "arxiv", Gap: 3 * time.Second, Run: harvestArxiv},
	"wikidata":             {Source: "wikidata", Gap: 2 * time.Second, Run: harvestWikidata},
	"openalex":             {Source: "openalex", Gap: time.Second, Run: harvestOpenAlex},
	"huggingface":          {Source: "huggingface", Gap: time.Second, Run: harvestHub},
	"github":               {Source: "github", Gap: 7 * time.Second, Run: harvestGitHub},
	"acl":                  {Source: "acl", Gap: time.Second, Run: harvestACL},
	"federal-register":     {Source: "federal-register", Gap: time.Second, Run: harvestFedReg},
	"oecd-ai":              {Source: "oecd-ai", Gap: time.Second, Run: harvestOECD},
	"swe-bench":            {Source: "swe-bench", Gap: time.Second, Run: harvestSWEBench},
	"open-llm-leaderboard": {Source: "open-llm-leaderboard", Gap: time.Second, Run: harvestOpenLLM},
}

// Names are the source ids of the adapters, sorted.
func Names() []string {
	var out []string
	for k := range Adapters {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ErrSchema means that a source changed the shape of its data, so the adapter stops before it writes a row (13.4).
type ErrSchema struct {
	Source string
	What   string
}

func (e *ErrSchema) Error() string {
	return fmt.Sprintf("the schema of %s changed: %s; the adapter writes nothing until it is updated", e.Source, e.What)
}

// table is a CSV file read by its header.
type table struct {
	col  map[string]int
	rows [][]string
}

func readTable(source string, b []byte, need ...string) (*table, error) {
	rd := csv.NewReader(strings.NewReader(strings.TrimPrefix(string(b), "\ufeff")))
	rd.FieldsPerRecord = -1
	rd.LazyQuotes = true
	all, err := rd.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", source, err)
	}
	if len(all) == 0 {
		return nil, &ErrSchema{source, "the file is empty"}
	}
	t := &table{col: map[string]int{}, rows: all[1:]}
	for i, h := range all[0] {
		t.col[strings.TrimSpace(h)] = i
	}
	for _, n := range need {
		if _, ok := t.col[n]; !ok {
			return nil, &ErrSchema{source, fmt.Sprintf("the CSV has no column %q", n)}
		}
	}
	return t, nil
}

func (t *table) get(row []string, name string) string {
	i, ok := t.col[name]
	if !ok || i >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[i])
}

// number reads a number of a CSV cell: "1.75e+11", "3,000" or "" for none.
func number(s string) (float64, bool) {
	s = strings.ReplaceAll(strings.TrimSpace(s), ",", "")
	if s == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(s, 64)
	return f, err == nil
}

// num keeps integral values as integers, so 175000000000 is not written 1.75e+11 with a float's noise.
func num(f float64) any {
	if f == float64(int64(f)) && f < 1e15 && f > -1e15 {
		return int64(f)
	}
	return f
}

var dateRE = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}`)

// date gives the YYYY-MM-DD start of a cell, or "".
func date(s string) string {
	if m := dateRE.FindString(strings.TrimSpace(s)); m != "" {
		if _, err := time.Parse("2006-01-02", m); err == nil {
			return m
		}
	}
	return ""
}

// list splits a cell of comma-separated values.
func list(s string) []string {
	var out []string
	for p := range strings.SplitSeq(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

var spaceRE = regexp.MustCompile(`\s+`)

// clean puts one space between words.
func clean(s string) string { return strings.TrimSpace(spaceRE.ReplaceAllString(s, " ")) }

// keep cuts rows to the limit.
func keep(rows []Row, limit int) []Row {
	if limit > 0 && len(rows) > limit {
		return rows[:limit]
	}
	return rows
}

func setFact(r *Row, path string, v any, loc string) {
	if v == nil {
		return
	}
	if s, ok := v.(string); ok && s == "" {
		return
	}
	if l, ok := v.([]string); ok && len(l) == 0 {
		return
	}
	if r.Facts == nil {
		r.Facts = map[string]Fact{}
	}
	r.Facts[path] = Fact{V: v, Loc: loc}
}

func setSignal(r *Row, k string, v any) {
	if r.Signals == nil {
		r.Signals = map[string]any{}
	}
	r.Signals[k] = v
}
