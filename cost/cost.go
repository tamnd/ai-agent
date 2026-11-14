// SPDX-License-Identifier: Apache-2.0

// Package cost sums the metered model calls of the agent runs (spec 14.8) by milestone, role, model and item.
//
// The calls are the lines of work/logs/events.jsonl that work/lib.py writes for each model call (14.7): the time, the
// job, the item, the role, the backend, the model, the tokens and the cost in USD. A backend that does not report a
// cost (codex reports only its tokens) gives an unmetered call. Prices, when given, turn its tokens into an estimate;
// the spec fixes no price, so they come from the vendors' lists at the time of the run.
package cost

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Milestone is a milestone of 21.1 with the last day of its window. A call is in the first milestone whose window
// ends on or after its day.
type Milestone struct {
	Name  string
	First string // YYYY-MM-DD
	Last  string
}

// Milestones are the windows of 21.2 to 21.7.
var Milestones = []Milestone{
	{"M0", "2026-11-13", "2026-12-31"},
	{"M1", "2027-01-01", "2027-03-31"},
	{"M2", "2027-04-01", "2027-07-31"},
	{"M3", "2027-08-01", "2027-12-31"},
	{"M4", "2028-01-01", "2028-06-30"},
	{"M5", "2028-07-01", "2028-12-31"},
}

// MilestoneOf gives the milestone of a day, "before M0" or "after M5".
func MilestoneOf(day string) string {
	if day < Milestones[0].First {
		return "before M0"
	}
	for _, m := range Milestones {
		if day <= m.Last {
			return m.Name
		}
	}
	return "after M5"
}

// Call is one model call of events.jsonl.
type Call struct {
	T         string   `json:"t"`
	Job       string   `json:"job"`
	Slug      string   `json:"slug"`
	Role      string   `json:"role"`
	Backend   string   `json:"backend"`
	Model     string   `json:"model"`
	OK        bool     `json:"ok"`
	Secs      float64  `json:"secs"`
	TokensIn  *int64   `json:"tokens_in"`
	TokensOut *int64   `json:"tokens_out"`
	Tokens    *int64   `json:"tokens"` // the total, from a backend that does not split it
	Cost      *float64 `json:"cost"`
	Limited   bool     `json:"limited"`
	Models    string   `json:"models_version"`
}

// Read gives the model calls of an events log: the lines with a backend. Other lines record the end of a job.
func Read(r io.Reader) ([]Call, error) {
	var out []Call
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), 16<<20)
	for ln := 1; sc.Scan(); ln++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var c Call
		if err := json.Unmarshal([]byte(line), &c); err != nil {
			return nil, fmt.Errorf("line %d: %w", ln, err)
		}
		if c.Backend == "" {
			continue
		}
		if c.Role == "" {
			c.Role = c.Job
		}
		out = append(out, c)
	}
	return out, sc.Err()
}

// Price is the price of a model in USD per million tokens. Total prices a call that gives only its total tokens.
type Price struct {
	In    float64 `yaml:"in" json:"in"`
	Out   float64 `yaml:"out" json:"out"`
	Total float64 `yaml:"total" json:"total,omitempty"`
}

// Prices are the prices by model id, as models.yaml names the model without its backend.
type Prices map[string]Price

// LoadPrices reads a prices file: a map from a model id to its prices, under the key models.
func LoadPrices(file string) (Prices, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var f struct {
		Models Prices `yaml:"models"`
	}
	if err := yaml.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	return f.Models, nil
}

// estimate gives the cost of an unmetered call from its tokens, or false.
func (p Prices) estimate(c Call) (float64, bool) {
	pr, ok := p[c.Model]
	if !ok {
		return 0, false
	}
	if c.TokensIn != nil && c.TokensOut != nil && (pr.In > 0 || pr.Out > 0) {
		return (float64(*c.TokensIn)*pr.In + float64(*c.TokensOut)*pr.Out) / 1e6, true
	}
	if c.Tokens != nil && pr.Total > 0 {
		return float64(*c.Tokens) * pr.Total / 1e6, true
	}
	return 0, false
}

// Sum is the total of a group of calls.
type Sum struct {
	Key       string  `json:"key"`
	Calls     int     `json:"calls"`
	Failed    int     `json:"failed,omitempty"`
	Limited   int     `json:"limited,omitempty"`
	Tokens    int64   `json:"tokens"`
	Metered   float64 `json:"metered_usd"`
	Estimated float64 `json:"estimated_usd,omitempty"`
	// Unpriced counts the calls with no cost and no price, whose cost is unknown.
	Unpriced int     `json:"unpriced,omitempty"`
	Secs     float64 `json:"secs"`
}

// USD is the metered and the estimated cost.
func (s Sum) USD() float64 { return s.Metered + s.Estimated }

func (s *Sum) add(c Call, p Prices) {
	s.Calls++
	if !c.OK {
		s.Failed++
	}
	if c.Limited {
		s.Limited++
	}
	switch {
	case c.TokensIn != nil || c.TokensOut != nil:
		s.Tokens += deref(c.TokensIn) + deref(c.TokensOut)
	case c.Tokens != nil:
		s.Tokens += *c.Tokens
	}
	s.Secs += c.Secs
	if c.Cost != nil {
		s.Metered += *c.Cost
	} else if v, ok := p.estimate(c); ok {
		s.Estimated += v
	} else {
		s.Unpriced++
	}
}

func deref(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

// Report is the cost report of 14.8.
type Report struct {
	From  string `json:"from,omitempty"`
	To    string `json:"to,omitempty"`
	Total Sum    `json:"total"`
	// Milestones holds one entry per milestone with calls, each with its roles.
	Milestones []MilestoneSum `json:"milestones"`
	Models     []Sum          `json:"models"`
	Items      []Sum          `json:"items"`
	// PerItem is the mean cost of an item that had a writer call, the cost per node of 14.8.
	PerItem float64 `json:"per_item_usd"`
	Written int     `json:"written"`
}

// MilestoneSum is the total of a milestone and of each role in it.
type MilestoneSum struct {
	Sum
	Roles []Sum `json:"roles"`
}

// Make sums the calls.
func Make(calls []Call, p Prices) Report {
	var r Report
	ms := map[string]*Sum{}
	roles := map[string]map[string]*Sum{}
	models := map[string]*Sum{}
	items := map[string]*Sum{}
	written := map[string]bool{}
	get := func(m map[string]*Sum, k string) *Sum {
		if m[k] == nil {
			m[k] = &Sum{Key: k}
		}
		return m[k]
	}
	for _, c := range calls {
		day := c.T
		if len(day) > 10 {
			day = day[:10]
		}
		if r.From == "" || day < r.From {
			r.From = day
		}
		if day > r.To {
			r.To = day
		}
		m := MilestoneOf(day)
		r.Total.add(c, p)
		get(ms, m).add(c, p)
		if roles[m] == nil {
			roles[m] = map[string]*Sum{}
		}
		get(roles[m], c.Role).add(c, p)
		model := c.Model
		if c.Backend != "" && c.Backend != "claude" {
			model = c.Backend + ":" + c.Model
		}
		get(models, model).add(c, p)
		if c.Slug != "" {
			get(items, c.Slug).add(c, p)
			if c.Role == "write" || c.Role == "writer" {
				written[c.Slug] = true
			}
		}
	}
	r.Total.Key = "total"
	order := func(m string) int {
		for i, x := range Milestones {
			if x.Name == m {
				return i + 1
			}
		}
		if m == "before M0" {
			return 0
		}
		return len(Milestones) + 1
	}
	for k, s := range ms {
		r.Milestones = append(r.Milestones, MilestoneSum{Sum: *s, Roles: sorted(roles[k])})
	}
	sort.Slice(r.Milestones, func(i, j int) bool { return order(r.Milestones[i].Key) < order(r.Milestones[j].Key) })
	r.Models, r.Items = sorted(models), sorted(items)
	var sum float64
	for k := range written {
		sum += items[k].USD()
	}
	if r.Written = len(written); r.Written > 0 {
		r.PerItem = sum / float64(r.Written)
	}
	if r.Milestones == nil {
		r.Milestones = []MilestoneSum{}
	}
	return r
}

// sorted gives the sums from the most costly down, then by key.
func sorted(m map[string]*Sum) []Sum {
	out := []Sum{}
	for _, s := range m {
		out = append(out, *s)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].USD() != out[j].USD() {
			return out[i].USD() > out[j].USD()
		}
		if out[i].Calls != out[j].Calls {
			return out[i].Calls > out[j].Calls
		}
		return out[i].Key < out[j].Key
	})
	return out
}
