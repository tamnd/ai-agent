// SPDX-License-Identifier: Apache-2.0

// Package coverage computes the coverage level C0 to C6 of each item (spec 18.2) and the coverage report of 18.7.
//
// The level of a node comes from its ladders (04.10), and the level of a card from its fields and facts. Nobody
// writes a level by hand. The targets of C5 and C6 are the defaults of 04.10, in code: schema/targets.v1.yaml is not
// read yet.
package coverage

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/tamnd/ai-agent/card"
	"github.com/tamnd/ai-agent/facts"
	"github.com/tamnd/ai-agent/node"
	"github.com/tamnd/ai-agent/status"
)

// Names are the names of the levels C0 to C6.
var Names = []string{"registered", "carded", "paged", "grounded", "evidenced", "complete", "reviewed"}

// Row is one row of the universe register (18.3). The signals and the same_as ids are not read here.
type Row struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Kind      string   `json:"kind"`
	Area      string   `json:"area"`
	Aliases   []string `json:"aliases,omitempty"`
	Sources   []string `json:"sources,omitempty"`
	Outcome   string   `json:"outcome"`
	Target    string   `json:"target,omitempty"`
	Reason    string   `json:"reason,omitempty"`
	DecidedBy string   `json:"decided_by,omitempty"`
	Decided   string   `json:"decided,omitempty"`
	Core      bool     `json:"core,omitempty"`
	// Added is the date that the harvester added the row, from which a pending row ages.
	Added string `json:"added,omitempty"`
	File  string `json:"-"`
}

// Outcomes are the outcomes of 18.3.
var Outcomes = []string{"node", "card", "alias", "part", "bridge", "row-only", "out-of-scope", "merged", "pending"}

// LoadUniverse reads registers/universe/*.jsonl. It gives no rows and no error when the register does not exist yet.
func LoadUniverse(root string) ([]Row, error) {
	files, err := filepath.Glob(filepath.Join(root, "registers", "universe", "*.jsonl"))
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	var rows []Row
	for _, p := range files {
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		sc := bufio.NewScanner(bytes.NewReader(b))
		sc.Buffer(make([]byte, 1<<20), 1<<24)
		for line := 1; sc.Scan(); line++ {
			t := bytes.TrimSpace(sc.Bytes())
			if len(t) == 0 {
				continue
			}
			var r Row
			if err := json.Unmarshal(t, &r); err != nil {
				return nil, fmt.Errorf("%s:%d: %w", p, line, err)
			}
			if !slices.Contains(Outcomes, r.Outcome) {
				return nil, fmt.Errorf("%s:%d: %s has the outcome %q, which is not one of 18.3", p, line, r.ID, r.Outcome)
			}
			r.File = filepath.Base(p)
			rows = append(rows, r)
		}
		if err := sc.Err(); err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
	}
	return rows, nil
}

// codeKinds are the kinds whose nodes have code in the tier-1 frameworks (18.2, C4; 08.2).
var codeKinds = map[string]bool{"method": true, "architecture": true, "metric": true}

// tier1 are the tier-1 frameworks with the X level of each of C4, C5 and C6.
var tier1 = []struct {
	id         string
	c4, c5, c6 int
}{{"numpy", 2, 3, 4}, {"pytorch", 2, 3, 4}, {"jax", 2, 2, 4}}

// NodeLevel is the coverage level of a node from its ladders, with what the next level needs. The node stands for its
// own card: its front matter is the card of 18.2, so a node is at least C1.
func NodeLevel(n *node.Node, st status.Status) (int, []string) {
	need := func(r status.Rung, lo int, ladder, what string) []string {
		if r.Level >= lo {
			return nil
		}
		return []string{fmt.Sprintf("%s at %s%d (now %s%d)", what, ladder, lo, ladder, r.Level)}
	}
	opt := func(r *status.Rung, lo int, ladder, what string) []string {
		if r == nil {
			return nil
		}
		return need(*r, lo, ladder, what)
	}
	x := func(level func(int, int, int) int) []string {
		if !codeKinds[n.Kind] {
			return nil
		}
		var out []string
		for _, fw := range tier1 {
			lo := level(fw.c4, fw.c5, fw.c6)
			r, ok := st.I[fw.id]
			if !ok {
				out = append(out, fmt.Sprintf("the %s code at X%d (none yet)", fw.id, lo))
				continue
			}
			out = append(out, need(r, lo, "X", "the "+fw.id+" code")...)
		}
		return out
	}
	levels := []func() []string{
		2: func() []string {
			return slices.Concat(need(st.W, 2, "W", "the writing"), need(st.G, 1, "G", "the grounding"))
		},
		3: func() []string {
			return slices.Concat(need(st.W, 3, "W", "the writing"), need(st.G, 2, "G", "the grounding"), opt(st.D, 2, "D", "the data facts"))
		},
		4: func() []string {
			return slices.Concat(x(func(a, _, _ int) int { return a }), opt(st.R, 1, "R", "the experiments"), need(st.G, 3, "G", "the grounding"))
		},
		5: func() []string {
			out := slices.Concat(need(st.W, 4, "W", "the writing"), need(st.G, 3, "G", "the grounding"), x(func(_, b, _ int) int { return b }),
				opt(st.R, 2, "R", "the experiments"), opt(st.D, 2, "D", "the data facts"))
			if n.Kind == "theorem" && len(n.Lean) > 0 {
				out = append(out, need(st.L, 2, "L", "the Lean proof")...)
			}
			if n.Scene != "" || n.ArchScene != "" {
				out = append(out, need(st.V, 2, "V", "the visualization")...)
			}
			return out
		},
		6: func() []string {
			return slices.Concat(need(st.W, 5, "W", "the writing"), x(func(_, _, c int) int { return c }), opt(st.R, 4, "R", "the experiments"), opt(st.D, 3, "D", "the data facts"))
		},
	}
	level := 1
	for l := 2; l < len(levels); l++ {
		if miss := levels[l](); len(miss) > 0 {
			return level, miss
		}
		level = l
	}
	return level, nil
}

// CardLevel is the coverage level of a card: C1 when it is valid and each of its required facts has a source, and C0
// otherwise. Card-only items stop at C1 (18.2).
func CardLevel(c *card.Card) (int, []string) {
	var miss []string
	for _, b := range c.Bad {
		miss = append(miss, "fix the card: "+b)
	}
	for _, r := range facts.Required[c.Kind] {
		i := slices.IndexFunc(c.Facts, func(f card.Fact) bool { return facts.Covers(f.ID, r) })
		switch {
		case i < 0:
			miss = append(miss, "the required fact "+r)
		case c.Facts[i].Src == "":
			miss = append(miss, "a source for the fact "+c.Facts[i].ID)
		}
	}
	if len(miss) > 0 {
		return 0, miss
	}
	return 1, nil
}

// Item is one node or card with its coverage level.
type Item struct {
	ID    string   `json:"id"`
	Form  string   `json:"form"` // node or card
	Kind  string   `json:"kind"`
	Area  string   `json:"area,omitempty"` // the top area of a node
	Core  bool     `json:"core,omitempty"`
	Level int      `json:"level"`
	Label string   `json:"label"`
	Next  []string `json:"next,omitempty"`
}

// Counts are the counts of the items at C0 to C6.
type Counts [7]int

// Report is the coverage report of 18.7.
type Report struct {
	Date    string `json:"date"`
	Targets string `json:"targets"`

	Universe Universe  `json:"universe"`
	Promises []Promise `json:"promises"`
	Levels   Levels    `json:"levels"`
	Core     []Item    `json:"core"`
	// Gaps holds the ids of each gap type of 18.7.
	Gaps  map[string][]string `json:"gaps"`
	Canon []Canon             `json:"canon"`
	// Courses is the coverage of each course of 5.7, as Canon is of each canon work.
	Courses []Canon `json:"courses"`
	Items   []Item  `json:"items"`
	// Fails are the promises of a person that are broken: a core node row below the milestone target after the
	// milestone date. They are the only part of the report that fails the build.
	Fails []string `json:"fails"`
	// NotComputed names the parts of 18.7 that this version does not compute yet.
	NotComputed []string `json:"not_computed"`
}

// Universe is the summary of the universe register.
type Universe struct {
	Rows        int            `json:"rows"`
	BySource    map[string]int `json:"by_source"`
	ByOutcome   map[string]int `json:"by_outcome"`
	ByArea      map[string]int `json:"by_area"`
	Pending     []Pending      `json:"pending"`
	Unharvested []string       `json:"unharvested"`
	// Closed are the sources of 18.4 that spec 13 gives no adapter, each with the reason: a source that the harvester
	// cannot read under its rules (13.1) is not a gap of harvest, but the report still names it.
	Closed map[string]string `json:"closed"`
	// Unregistered are the nodes and cards that no row names: an item that is not in the register is a failure of
	// harvest (18.1).
	Unregistered int `json:"unregistered"`
}

// Pending is a pending row and its age in days, from the date that it was added.
type Pending struct {
	ID   string `json:"id"`
	Days int    `json:"days,omitempty"`
	Area string `json:"area,omitempty"` // the top area of the file of the row
}

// Promise is a row whose outcome promises a node or a card that is absent or below its target.
type Promise struct {
	Row     string `json:"row"`
	Outcome string `json:"outcome"`
	Target  string `json:"target"`
	Core    bool   `json:"core,omitempty"`
	Gap     string `json:"gap"` // missing-node, missing-card or below-target
	Level   string `json:"level,omitempty"`
	Area    string `json:"area,omitempty"` // the top area of the file of the row
}

// Levels are the counts of C0 to C6.
type Levels struct {
	Total  Counts            `json:"total"`
	ByArea map[string]Counts `json:"by_area"`
	ByKind map[string]Counts `json:"by_kind"`
	ByCore map[string]Counts `json:"by_core"` // core and other
	// Since is the last release, and Change the change of each level since it.
	Since  string  `json:"since,omitempty"`
	Change *[7]int `json:"change,omitempty"`
}

// Canon is the coverage of one canon work: the share of its named items with a node at C2 or higher.
type Canon struct {
	ID    string  `json:"id"`
	Title string  `json:"title"`
	Named int     `json:"named"`
	AtC2  int     `json:"at_c2"`
	Share float64 `json:"share"`
}

// Sources are the sources of 18.4. A source with no row is unharvested. A row's sources field starts with the id of the
// harvest source that read it (13.2), which is the id of 18.4 or a part of one; sourceGroup gives the parts.
var Sources = []string{"epoch", "hf-hub", "openalex", "arxiv", "semantic-scholar", "dblp", "openreview", "acl-anthology", "cvf",
	"pwc-dump", "wikidata", "aiid", "risk-taxonomies", "oecd-ai", "eur-lex", "federal-register", "standards", "leaderboards", "canon", "surveys",
	"courses", "github", "awards"}

// sourceGroup gives the source of 18.4 that a harvest source is a part of, where the ids differ: the Epoch AI data on
// models is epoch, its Benchmarking Hub is one of the leaderboards.
var sourceGroup = map[string]string{
	"epoch-ai":             "epoch",
	"epoch-benchmarks":     "leaderboards",
	"huggingface":          "hf-hub",
	"acl":                  "acl-anthology",
	"pwc-results":          "pwc-dump",
	"swe-bench":            "leaderboards",
	"open-llm-leaderboard": "leaderboards",
	"lmarena":              "leaderboards",
	"helm":                 "leaderboards",
	"mitre-atlas":          "risk-taxonomies",
}

// ClosedSources are the sources of 18.4 that have no adapter, by the rules of 13.1, with the reason (spec 13.2). They
// are reported as closed, not as unharvested; when a source opens, its entry goes and its adapter comes.
var ClosedSources = map[string]string{
	"openreview": "the API answers a client with no account 403 ChallengeRequiredError, and the harvester never answers a " +
		"challenge; ICLR and TMLR come from DBLP",
	"semantic-scholar": "the API with no key answers 429 from a shared pool, and the datasets need a key (401); the works " +
		"overlap OpenAlex, and S2 ids come in same_as from other sources",
}

// SourceOf gives the source of 18.4 of a harvest source id.
func SourceOf(id string) string {
	if g, ok := sourceGroup[id]; ok {
		return g
	}
	return id
}

// M3End is the end of the window of M3 (21): after it, a core item below C3 is a failed promise.
var M3End = time.Date(2027, 12, 31, 23, 59, 59, 0, time.UTC)

// Input is what the report reads.
type Input struct {
	Corpus *node.Corpus
	Status map[string]status.Status // by slug
	Rows   []Row
	// Canon are the canon works of registers/canon.yaml, and Courses the courses of topics/courses.
	Canon   []Canon
	Courses []Canon
	// Last is the coverage of the last release, and LastVersion its version. Last is nil before the first release.
	Last        *Counts
	LastVersion string
	Now         time.Time
}

// Compute makes the report.
func Compute(in Input) *Report {
	c := in.Corpus
	r := &Report{
		Date:    in.Now.UTC().Format("2006-01-02"),
		Targets: "the defaults of spec 04.10; schema/targets.v1.yaml is not read yet",
		Universe: Universe{Rows: len(in.Rows), BySource: map[string]int{}, ByOutcome: map[string]int{}, ByArea: map[string]int{},
			Pending: []Pending{}, Unharvested: []string{}, Closed: map[string]string{}},
		Promises: []Promise{},
		Levels:   Levels{ByArea: map[string]Counts{}, ByKind: map[string]Counts{}, ByCore: map[string]Counts{}},
		Core:     []Item{},
		Gaps:     map[string][]string{},
		Canon:    []Canon{},
		Courses:  []Canon{},
		Items:    []Item{},
		Fails:    []string{},
		NotComputed: []string{
			"freshness: the refresh periods of the sources and the as_of periods of facts are not read yet",
			"stale and demand gaps: the demand signals of 18.6 are not collected yet",
		},
	}
	for _, g := range []string{"unharvested", "pending", "missing-node", "missing-card", "below-target", "stale", "broken-bridge", "demand"} {
		r.Gaps[g] = []string{}
	}

	// The levels of the nodes and the cards.
	core := map[string]bool{} // node slugs and card keys that a core row targets
	named := map[string]bool{}
	for _, row := range in.Rows {
		if id := targetID(row.Target); id != "" {
			named[id] = true
			if row.Core {
				core[id] = true
			}
		}
	}
	bySlug := map[string]Item{}
	// A card target is the slug of a NEW- id before aik mint and the tag after it.
	cardByTag := map[string]*card.Card{}
	for _, cd := range c.Cards {
		if _, dup := cardByTag[cd.Tag]; !dup && cd.Tag != "" {
			cardByTag[cd.Tag] = cd
		}
	}
	add := func(it Item) {
		it.Label = fmt.Sprintf("C%d", it.Level)
		r.Items = append(r.Items, it)
		r.Levels.Total[it.Level]++
		if it.Area != "" {
			inc(r.Levels.ByArea, it.Area, it.Level)
		}
		inc(r.Levels.ByKind, it.Kind, it.Level)
		k := "other"
		if it.Core {
			k = "core"
			r.Core = append(r.Core, it)
		}
		inc(r.Levels.ByCore, k, it.Level)
	}
	for _, n := range c.Nodes {
		lv, next := NodeLevel(n, in.Status[n.Slug])
		it := Item{ID: n.Slug, Form: "node", Kind: n.Kind, Area: strings.Split(n.Area, "/")[0], Core: core[n.Slug] || core[n.Tag], Level: lv, Next: next}
		bySlug[n.Slug] = it
		if n.Tag != "" {
			bySlug[n.Tag] = it
		}
		if !named[n.Slug] && !named[n.Tag] {
			r.Universe.Unregistered++
		}
		add(it)
	}
	for _, cd := range c.Cards {
		if cd.Kind == "bridge" {
			continue // a bridge card is a link to a neighbour's item, not an item of ours
		}
		lv, next := CardLevel(cd)
		add(Item{ID: cd.Key(), Form: "card", Kind: cd.Kind, Core: core[cd.Key()], Level: lv, Next: next})
		if !named[cd.Key()] && !named[cd.Tag] {
			r.Universe.Unregistered++
		}
	}
	if in.Last != nil {
		var d [7]int
		for i := range d {
			d[i] = r.Levels.Total[i] - in.Last[i]
		}
		r.Levels.Since, r.Levels.Change = in.LastVersion, &d
	}

	// The universe and the promises.
	harvested := map[string]bool{}
	for _, row := range in.Rows {
		area := strings.TrimSuffix(row.File, ".jsonl")
		r.Universe.ByOutcome[row.Outcome]++
		r.Universe.ByArea[area]++
		if area == "_" {
			area = "" // the file of the rows that no area holds yet
		}
		for _, s := range row.Sources {
			src, _, _ := strings.Cut(s, ":")
			harvested[SourceOf(src)] = true
			r.Universe.BySource[src]++
		}
		switch row.Outcome {
		case "pending":
			p := Pending{ID: row.ID, Area: area}
			if t, err := time.Parse("2006-01-02", row.Added); err == nil {
				p.Days = int(in.Now.Sub(t).Hours() / 24)
			}
			r.Universe.Pending = append(r.Universe.Pending, p)
			r.Gaps["pending"] = append(r.Gaps["pending"], row.ID)
		case "node":
			it, ok := bySlug[targetID(row.Target)]
			p := Promise{Row: row.ID, Outcome: row.Outcome, Target: row.Target, Core: row.Core, Area: area}
			switch {
			case !ok:
				p.Gap = "missing-node"
			case row.Core && it.Level < 3:
				p.Gap, p.Level = "below-target", it.Label
			default:
				continue
			}
			r.Promises = append(r.Promises, p)
			r.Gaps[p.Gap] = append(r.Gaps[p.Gap], row.ID)
			if row.Core && in.Now.After(M3End) {
				r.Fails = append(r.Fails, fmt.Sprintf("%s: a core node row is below C3 after M3 (%s)", row.ID, p.Gap))
			}
		case "card":
			cd := c.CardByKey[targetID(row.Target)]
			if cd == nil {
				cd = cardByTag[targetID(row.Target)]
			}
			if cd == nil || len(cd.Bad) > 0 {
				r.Promises = append(r.Promises, Promise{Row: row.ID, Outcome: row.Outcome, Target: row.Target, Core: row.Core, Gap: "missing-card", Area: area})
				r.Gaps["missing-card"] = append(r.Gaps["missing-card"], row.ID)
			}
		}
	}
	for _, s := range Sources {
		switch why, closed := ClosedSources[s]; {
		case harvested[s]:
		case closed:
			r.Universe.Closed[s] = why
		default:
			r.Universe.Unharvested = append(r.Universe.Unharvested, s)
		}
	}
	r.Gaps["unharvested"] = r.Universe.Unharvested
	for _, cd := range c.Cards {
		if cd.Kind == "bridge" && len(cd.Fields["retired"]) > 0 && string(cd.Fields["retired"]) != "null" {
			r.Gaps["broken-bridge"] = append(r.Gaps["broken-bridge"], cd.Key())
		}
	}

	// Canon and course coverage: the rows that a canon work or a course names, and which of them have a node at C2
	// or higher.
	share := func(source string, w Canon) Canon {
		w.Named, w.AtC2 = 0, 0
		for _, row := range in.Rows {
			if !slices.ContainsFunc(row.Sources, func(s string) bool {
				return s == source+":"+w.ID || strings.HasPrefix(s, source+":"+w.ID+":")
			}) {
				continue
			}
			w.Named++
			if it, ok := bySlug[targetID(row.Target)]; ok && it.Form == "node" && it.Level >= 2 {
				w.AtC2++
			}
		}
		if w.Named > 0 {
			w.Share = float64(w.AtC2) / float64(w.Named)
		}
		return w
	}
	for _, w := range in.Canon {
		r.Canon = append(r.Canon, share("canon", w))
	}
	for _, w := range in.Courses {
		r.Courses = append(r.Courses, share("courses", w))
	}
	sort.Slice(r.Items, func(i, j int) bool {
		return r.Items[i].Form > r.Items[j].Form || r.Items[i].Form == r.Items[j].Form && r.Items[i].ID < r.Items[j].ID
	})
	return r
}

func inc(m map[string]Counts, k string, level int) {
	c := m[k]
	c[level]++
	m[k] = c
}

// targetID is the slug, tag or card key that a row's target names: "ai://NEW-dropout" is dropout, "ai://3F2K" is
// 3F2K, and the anchor of a part is cut.
func targetID(t string) string {
	t = strings.TrimPrefix(t, "ai://")
	t, _, _ = strings.Cut(t, "#")
	t, _, _ = strings.Cut(t, "@")
	return strings.TrimPrefix(t, "NEW-")
}

// LoadCanon reads the works of registers/canon.yaml. It gives nil and no error when the register does not exist.
func LoadCanon(root string) ([]Canon, error) {
	b, err := os.ReadFile(filepath.Join(root, "registers", "canon.yaml"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var reg struct {
		Works []struct {
			ID    string `yaml:"id"`
			Title string `yaml:"title"`
		} `yaml:"works"`
	}
	if err := yaml.Unmarshal(b, &reg); err != nil {
		return nil, fmt.Errorf("registers/canon.yaml: %w", err)
	}
	var out []Canon
	for _, w := range reg.Works {
		out = append(out, Canon{ID: w.ID, Title: w.Title})
	}
	return out, nil
}

// LoadCourses reads the ids and titles of the courses of topics/courses (5.7). It gives nil and no error when there is
// no course.
func LoadCourses(root string) ([]Canon, error) {
	files, err := filepath.Glob(filepath.Join(root, "topics", "courses", "*.yaml"))
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	var out []Canon
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		var c struct {
			ID    string `yaml:"id"`
			Title string `yaml:"title"`
		}
		if err := yaml.Unmarshal(b, &c); err != nil {
			return nil, fmt.Errorf("topics/courses/%s: %w", filepath.Base(f), err)
		}
		if c.ID == "" {
			c.ID = strings.TrimSuffix(filepath.Base(f), ".yaml")
		}
		out = append(out, Canon{ID: c.ID, Title: c.Title})
	}
	return out, nil
}

// CoreIDs are the node slugs, node tags and card keys that a core row of the universe targets.
func CoreIDs(rows []Row) map[string]bool {
	core := map[string]bool{}
	for _, row := range rows {
		if id := targetID(row.Target); id != "" && row.Core {
			core[id] = true
		}
	}
	return core
}
