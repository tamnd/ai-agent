// SPDX-License-Identifier: Apache-2.0

// Package sample draws the weekly sample of spec 15.6: the pages that changed in an ISO week, stratified by risk, then
// 2% of every other page. The draw is seeded by the week, so anyone with the repository can recompute it: each page of a
// stratum gets the key sha256(week, stratum, id), the pages are sorted by key, and the first ones are taken.
package sample

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tamnd/ai-agent/card"
)

// Page is a node or a card that the sample can take.
type Page struct {
	ID        string    `json:"id"`   // the slug of a node, the key of a card
	Form      string    `json:"form"` // node or card
	Kind      string    `json:"kind"`
	Path      string    `json:"path"` // the directory of a node, the shard and line of a card
	Core      bool      `json:"core,omitempty"`
	Gold      bool      `json:"gold,omitempty"`
	Contested bool      `json:"contested,omitempty"` // an open-problem or argument node, or a node with positions.yaml
	People    bool      `json:"people,omitempty"`    // a person page, or a page that names people
	Released  time.Time `json:"-"`                   // the release date of a model, when it has one
	Changed   bool      `json:"changed,omitempty"`   // a file of the page changed in the week
	New       bool      `json:"new,omitempty"`       // a card that did not exist before the week
	Source    string    `json:"source,omitempty"`    // the harvest source of a new card
}

// Stratum is one row of the table of 15.6.
type Stratum struct {
	Name string
	Rate float64
	Min  int // at least this many, or all when there are fewer; per source for the new cards
	Why  string
	in   func(p Page, end time.Time) bool
}

// Strata are the strata of 15.6, in the order of the table. The stratum of the pages written by a model new on the
// ladder is not drawn: the writer of a page is not recorded yet (see NotComputed).
var Strata = []Stratum{
	{Name: "gold", Rate: 1, Why: "the quality bar of the writer and the auditor",
		in: func(p Page, _ time.Time) bool { return p.Changed && p.Gold }},
	{Name: "core", Rate: 0.10, Why: "the pages most readers read",
		in: func(p Page, _ time.Time) bool { return p.Changed && p.Core && p.Form == "node" }},
	{Name: "contested", Rate: 0.15, Why: "the risk of one-sidedness",
		in: func(p Page, _ time.Time) bool { return p.Changed && p.Contested }},
	{Name: "policy", Rate: 0.15, Why: "the risk of wrong legal facts with real effects",
		in: func(p Page, _ time.Time) bool { return p.Changed && (p.Kind == "policy" || p.Kind == "provision") }},
	{Name: "person", Rate: 0.15, Why: "the risk to people (19.6)",
		in: func(p Page, _ time.Time) bool { return p.Changed && p.People }},
	{Name: "recent-model", Rate: 0.10, Why: "facts change fast and are mostly self-reported",
		in: func(p Page, end time.Time) bool {
			return p.Changed && p.Kind == "model" && !p.Released.IsZero() && !p.Released.After(end) && p.Released.After(end.AddDate(0, -6, 0))
		}},
	{Name: "new-cards", Rate: 0.01, Min: 50, Why: "the risk of an adapter error at scale",
		in: func(p Page, _ time.Time) bool { return p.New && p.Source != "" }},
}

// Rest is the rate of the pages that no stratum took, changed or not.
const Rest = 0.02

// NotComputed are the parts of 15.6 that the sample does not do yet.
var NotComputed = []string{
	"the stratum of pages written by a model new on the ladder: the writer of a page is not recorded yet",
	"pages that name living people are found by the people field of a node only, not in the text",
	"the doubling of a stratum's rate after a week over the defect targets: no defect records are read yet",
}

// Pick is a page that the sample took.
type Pick struct {
	ID      string   `json:"id"`
	Form    string   `json:"form"`
	Kind    string   `json:"kind"`
	Path    string   `json:"path"`
	Stratum string   `json:"stratum"`          // the first stratum that took it
	Also    []string `json:"also,omitempty"`   // the other strata that took it
	Source  string   `json:"source,omitempty"` // the harvest source, for a new card
}

// Count is the size of a stratum and how many it took.
type Count struct {
	Stratum string  `json:"stratum"`
	Rate    float64 `json:"rate"`
	Pages   int     `json:"pages"`
	Took    int     `json:"took"`
}

// Result is the sample of a week.
type Result struct {
	Week        string   `json:"week"`
	From        string   `json:"from"` // the first day of the week
	To          string   `json:"to"`   // the last day of the week
	Pages       int      `json:"pages"`
	Changed     int      `json:"changed"`
	New         int      `json:"new"`
	Counts      []Count  `json:"counts"`
	Picks       []Pick   `json:"picks"`
	NotComputed []string `json:"not_computed"`
}

var weekRe = regexp.MustCompile(`^(\d{4})-W(\d{2})$`)

// Week gives the start of an ISO week such as 2027-W14 (a Monday, 00:00 UTC) and the start of the next week.
func Week(s string) (from, to time.Time, err error) {
	m := weekRe.FindStringSubmatch(s)
	if m == nil {
		return from, to, fmt.Errorf("the week %q is not of the form 2027-W14", s)
	}
	y, _ := strconv.Atoi(m[1])
	w, _ := strconv.Atoi(m[2])
	// The 4th of January is always in week 1.
	jan4 := time.Date(y, 1, 4, 0, 0, 0, 0, time.UTC)
	from = jan4.AddDate(0, 0, -((int(jan4.Weekday())+6)%7)+7*(w-1))
	if yy, ww := from.ISOWeek(); w < 1 || yy != y || ww != w {
		return time.Time{}, time.Time{}, fmt.Errorf("the year %d has no week %d", y, w)
	}
	return from, from.AddDate(0, 0, 7), nil
}

// WeekOf is the ISO week of a time, as 2027-W14.
func WeekOf(t time.Time) string {
	y, w := t.UTC().ISOWeek()
	return fmt.Sprintf("%d-W%02d", y, w)
}

// key orders the pages of a stratum for a week.
func key(week, stratum, id string) string {
	h := sha256.Sum256([]byte(week + "\x00" + stratum + "\x00" + id))
	return hex.EncodeToString(h[:])
}

// take gives the first n pages of a stratum by key: the rate of the pages, rounded up, and at least min of them.
func take(week, stratum string, ps []Page, rate float64, min int) []Page {
	n := int(math.Ceil(rate * float64(len(ps))))
	if n < min {
		n = min
	}
	if n > len(ps) {
		n = len(ps)
	}
	ks := make([]string, len(ps))
	idx := make([]int, len(ps))
	for i, p := range ps {
		ks[i], idx[i] = key(week, stratum, p.Form+":"+p.ID), i
	}
	sort.Slice(idx, func(a, b int) bool { return ks[idx[a]] < ks[idx[b]] })
	out := make([]Page, n)
	for i := range out {
		out[i] = ps[idx[i]]
	}
	return out
}

// Draw draws the sample of a week from the pages.
func Draw(week string, pages []Page) (*Result, error) {
	from, to, err := Week(week)
	if err != nil {
		return nil, err
	}
	end := to.Add(-time.Nanosecond)
	r := &Result{Week: week, From: from.Format("2006-01-02"), To: end.Format("2006-01-02"), Pages: len(pages),
		Counts: []Count{}, Picks: []Pick{}, NotComputed: NotComputed}
	for _, p := range pages {
		if p.Changed {
			r.Changed++
		}
		if p.New {
			r.New++
		}
	}
	at := map[string]int{} // form:id to the index of its pick
	add := func(stratum string, p Page) {
		k := p.Form + ":" + p.ID
		if i, ok := at[k]; ok {
			r.Picks[i].Also = append(r.Picks[i].Also, stratum)
			return
		}
		at[k] = len(r.Picks)
		r.Picks = append(r.Picks, Pick{ID: p.ID, Form: p.Form, Kind: p.Kind, Path: p.Path, Stratum: stratum, Source: p.Source})
	}
	for _, s := range Strata {
		var in []Page
		for _, p := range pages {
			if s.in(p, end) {
				in = append(in, p)
			}
		}
		c := Count{Stratum: s.Name, Rate: s.Rate, Pages: len(in)}
		if s.Name == "new-cards" {
			// The minimum is per source, so that a small adapter is checked as well as a large one.
			bySrc := map[string][]Page{}
			var srcs []string
			for _, p := range in {
				if bySrc[p.Source] == nil {
					srcs = append(srcs, p.Source)
				}
				bySrc[p.Source] = append(bySrc[p.Source], p)
			}
			sort.Strings(srcs)
			for _, src := range srcs {
				for _, p := range take(week, s.Name+"\x00"+src, bySrc[src], s.Rate, s.Min) {
					add(s.Name, p)
					c.Took++
				}
			}
		} else {
			for _, p := range take(week, s.Name, in, s.Rate, s.Min) {
				add(s.Name, p)
				c.Took++
			}
		}
		r.Counts = append(r.Counts, c)
	}
	var rest []Page
	for _, p := range pages {
		if _, ok := at[p.Form+":"+p.ID]; !ok {
			rest = append(rest, p)
		}
	}
	c := Count{Stratum: "rest", Rate: Rest, Pages: len(rest)}
	for _, p := range take(week, "rest", rest, Rest, 0) {
		add("rest", p)
		c.Took++
	}
	r.Counts = append(r.Counts, c)
	return r, nil
}

// CardChanges reads a unified diff of the cards directory (git diff -U0) and gives the tags of the cards whose line
// changed and, of those, the ones that are new: a tag on an added line that no removed line has. A card that moved
// from one shard to another is changed, not new.
func CardChanges(diff string) (changed, added map[string]bool) {
	plus, minus := map[string]bool{}, map[string]bool{}
	sc := bufio.NewScanner(strings.NewReader(diff))
	sc.Buffer(make([]byte, 1<<20), 1<<26)
	for sc.Scan() {
		l := sc.Text()
		if strings.HasPrefix(l, "+++") || strings.HasPrefix(l, "---") || len(l) < 2 || l[1] != '{' {
			continue
		}
		set := plus
		switch l[0] {
		case '+':
		case '-':
			set = minus
		default:
			continue
		}
		if cd, err := card.Parse([]byte(l[1:])); err == nil && cd.Tag != "" {
			set[cd.Tag] = true
		}
	}
	changed, added = map[string]bool{}, map[string]bool{}
	for t := range plus {
		changed[t] = true
		if !minus[t] {
			added[t] = true
		}
	}
	return changed, added
}
