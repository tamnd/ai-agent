// SPDX-License-Identifier: Apache-2.0

// Package queue computes the work queue of spec 14.5 from the coverage report (18.7) and the findings of the audit
// rules: the rows that promise a node or a card that does not exist, the items below the target of a milestone, the
// pending rows of the universe, and the result cards that fail an M rule. The queue is computed, never written by
// hand; only the leases that people and writers take are kept from one build to the next.
package queue

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/tamnd/ai-agent/check"
	"github.com/tamnd/ai-agent/coverage"
)

// File is the queue in the content repository.
const File = "work/state/queue.jsonl"

// Milestones are the milestones of 21 that set targets.
var Milestones = []string{"M1", "M2", "M3", "M4", "M5"}

// The kinds of the items.
const (
	KindWrite  = "write"  // a universe row with outcome node and no node (18.3)
	KindCard   = "card"   // a universe row with outcome card and no valid card
	KindRaise  = "raise"  // a node or card below the target of the milestone
	KindDecide = "decide" // a pending row of the universe that a person or the catalog phase decides (14.3)
	KindFix    = "fix"    // a result card that fails an M rule (10.6)
)

// Item is one item of the queue.
type Item struct {
	ID     string   `json:"id"`   // <kind>:<target>, stable from one build to the next
	Kind   string   `json:"kind"` // write, card, raise, decide or fix
	Target string   `json:"target"`
	Form   string   `json:"form,omitempty"` // node or card, for raise
	Area   string   `json:"area,omitempty"`
	Core   bool     `json:"core,omitempty"`
	Level  string   `json:"level,omitempty"` // the coverage level now, for raise
	Want   string   `json:"want,omitempty"`  // the coverage level that the milestone asks for, for raise
	Why    string   `json:"why"`
	Next   []string `json:"next,omitempty"`
	// Rank is the priority of 14.5: 0 core, 1 a target of the milestone, 2 the rest. Demand (18.6) is not collected
	// yet, so it does not rank.
	Rank int `json:"rank"`
	// Days is the age of a pending row, from the date that the harvester added it.
	Days  int    `json:"days,omitempty"`
	Lease *Lease `json:"lease,omitempty"`
}

// Lease is the hold of a person or a writer on an item, until a time.
type Lease struct {
	By    string `json:"by"`
	Until string `json:"until"` // RFC 3339
}

// Goal is the coverage level that a milestone asks of an item (21). M1 asks every node
// at C2 (W2) and every card at C1; M2 every core item at C2; M3 the core at C3 and the core methods, architectures
// and metrics at C4; M4 and M5 the core at C5.
func Goal(milestone string, it coverage.Item) int {
	base := 1
	if it.Form == "node" {
		base = 2
	}
	if !it.Core {
		return base
	}
	switch milestone {
	case "M1", "M2":
		return 2
	case "M3":
		if it.Kind == "method" || it.Kind == "architecture" || it.Kind == "metric" {
			return 4
		}
		return 3
	case "M4", "M5":
		return 5
	}
	return base
}

// Input is what the queue reads.
type Input struct {
	Report    *coverage.Report
	Findings  []check.Finding
	Milestone string
	Area      string // when not empty, only the items of this top area; the findings of the M rules have no area
	Old       []Item // the last queue, for its leases
	Now       time.Time
}

// Build computes the queue: core first, then the targets of the milestone, then the rest; within a rank the older
// pending rows first, then by area and id. A lease of the last queue stays on its item while it runs.
func Build(in Input) []Item {
	r := in.Report
	var out []Item
	for _, p := range r.Promises {
		switch p.Gap {
		case "missing-node":
			out = append(out, Item{ID: KindWrite + ":" + p.Row, Kind: KindWrite, Target: p.Row, Area: p.Area, Core: p.Core,
				Why: "the row " + p.Row + " has the outcome node and no node is at " + p.Target})
		case "missing-card":
			out = append(out, Item{ID: KindCard + ":" + p.Row, Kind: KindCard, Target: p.Row, Area: p.Area, Core: p.Core,
				Why: "the row " + p.Row + " has the outcome card and no valid card is at " + p.Target})
		}
	}
	for _, it := range r.Items {
		g := Goal(in.Milestone, it)
		if it.Level >= g {
			continue
		}
		out = append(out, Item{ID: KindRaise + ":" + it.ID, Kind: KindRaise, Target: it.ID, Form: it.Form, Area: it.Area, Core: it.Core,
			Level: it.Label, Want: fmt.Sprintf("C%d", g), Next: it.Next,
			Why: fmt.Sprintf("the %s %s is at %s; %s asks C%d", it.Form, it.ID, it.Label, in.Milestone, g)})
	}
	for _, p := range r.Universe.Pending {
		out = append(out, Item{ID: KindDecide + ":" + p.ID, Kind: KindDecide, Target: p.ID, Area: p.Area, Days: p.Days,
			Why: "the row " + p.ID + " is pending: it needs an outcome of 18.3"})
	}
	seen := map[string]int{}
	for _, f := range in.Findings {
		if !strings.HasPrefix(f.Rule, "M0") {
			continue
		}
		id := KindFix + ":" + f.Path + ":" + f.Rule
		if f.Line > 0 {
			id = fmt.Sprintf("%s:%d:%s", KindFix+":"+f.Path, f.Line, f.Rule)
		}
		if i, ok := seen[id]; ok {
			out[i].Next = append(out[i].Next, f.Msg)
			continue
		}
		seen[id] = len(out)
		out = append(out, Item{ID: id, Kind: KindFix, Target: f.Path, Why: f.Rule + ": " + check.Rules[f.Rule], Next: []string{f.Msg}})
	}

	// The area filter, the ranks and the leases.
	old := map[string]*Lease{}
	for _, o := range in.Old {
		if o.Lease != nil {
			if t, err := time.Parse(time.RFC3339, o.Lease.Until); err == nil && t.After(in.Now) {
				old[o.ID] = o.Lease
			}
		}
	}
	kept := out[:0]
	for _, it := range out {
		if in.Area != "" && it.Area != in.Area {
			continue
		}
		switch {
		case it.Core:
			it.Rank = 0
		case it.Kind != KindDecide:
			it.Rank = 1
		default:
			it.Rank = 2
		}
		it.Lease = old[it.ID]
		kept = append(kept, it)
	}
	out = kept
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Rank != b.Rank {
			return a.Rank < b.Rank
		}
		if a.Days != b.Days {
			return a.Days > b.Days
		}
		if a.Area != b.Area {
			return a.Area < b.Area
		}
		return a.ID < b.ID
	})
	return out
}

// Read reads the queue of a content repository. It gives nil and no error when there is none.
func Read(root string) ([]Item, error) {
	p := filepath.Join(root, filepath.FromSlash(File))
	b, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Item
	sc := bufio.NewScanner(bytes.NewReader(b))
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for line := 1; sc.Scan(); line++ {
		t := bytes.TrimSpace(sc.Bytes())
		if len(t) == 0 {
			continue
		}
		var it Item
		if err := json.Unmarshal(t, &it); err != nil {
			return nil, fmt.Errorf("%s:%d: %w", File, line, err)
		}
		out = append(out, it)
	}
	return out, sc.Err()
}

// Write writes the queue, one item a line.
func Write(root string, items []Item) error {
	var buf bytes.Buffer
	for _, it := range items {
		b, err := json.Marshal(it)
		if err != nil {
			return err
		}
		buf.Write(b)
		buf.WriteByte('\n')
	}
	p := filepath.Join(root, filepath.FromSlash(File))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, buf.Bytes(), 0o644)
}

// NotComputed are the sources of 14.5 that the queue does not read yet.
var NotComputed = []string{
	"claims labelled reported on core nodes that a cheap experiment could raise (07, 09.2)",
	"facts with status disputed (04.9)",
	"the new items of the daily job (14.9)",
	"search misses and the missing signals of 18.6, so demand does not rank",
	"reader reports (16.19)",
	"regressions after a pin update or a changed snapshot",
	"the reading order of 5.7 inside a sub-area",
}

// Counts are the number of items of each kind.
func Counts(items []Item) map[string]int {
	m := map[string]int{}
	for _, it := range items {
		m[it.Kind]++
	}
	return m
}
