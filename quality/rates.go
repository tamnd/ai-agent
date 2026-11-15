// SPDX-License-Identifier: Apache-2.0

package quality

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// The gate and audit pass rates by kind of 14.7 and 21.3: the share of items whose first gate passed with no fix
// round, the share whose first audit passed, and the share that ended done. The writer keeps one state file per item
// (work/state/<slug>.json); the rates come from those files and the kinds from the content tree.

// Item is what the rates need of the state file of one item.
type Item struct {
	Slug   string
	Kind   string // the kind the writer recorded, for an item the content tree no longer has
	Status string
	Gates  []Outcome // the gate runs in order
	Audits []Outcome // the audits in order
}

// Outcome is one gate run or one audit, with the run of the writer that made it.
type Outcome struct {
	Pass bool
	Run  string
}

// state is the part of a state file of the writer (work/writer.py) that the rates read.
type state struct {
	Slug    string `json:"slug"`
	Kind    string `json:"kind"`
	Status  string `json:"status"`
	History []struct {
		Role string `json:"role"`
		Run  string `json:"run"`
	} `json:"history"`
	Gates []struct {
		OK  bool   `json:"ok"`
		Run string `json:"run"`
	} `json:"gates"`
	Audits []struct {
		Verdict string `json:"verdict"`
		Run     string `json:"run"`
	} `json:"audits"`
}

// LoadItems reads the state files of the writer in dir. A missing directory gives no items.
func LoadItems(dir string) ([]Item, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	var out []Item
	for _, p := range files {
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		var st state
		if err := json.Unmarshal(b, &st); err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		if st.Slug == "" {
			st.Slug = strings.TrimSuffix(filepath.Base(p), ".json")
		}
		it := Item{Slug: st.Slug, Kind: st.Kind, Status: st.Status}
		if len(st.Gates) > 0 {
			for _, g := range st.Gates {
				it.Gates = append(it.Gates, Outcome{Pass: g.OK, Run: g.Run})
			}
		} else {
			it.Gates = gatesOf(st)
		}
		for _, a := range st.Audits {
			it.Audits = append(it.Audits, Outcome{Pass: a.Verdict == "pass", Run: a.Run})
		}
		out = append(out, it)
	}
	return out, nil
}

// gatesOf recovers the gate runs of a state file written before the writer recorded them. The writer runs the gate
// before each audit and before each gate fix: a fix-gate call follows a failed gate, and an audit call follows a
// passed one. A gate that passed and led to done with no audit after it, as after a fix-minor round, is not seen, so
// the rates count the first gate, which this recovers exactly.
func gatesOf(st state) []Outcome {
	var out []Outcome
	for _, h := range st.History {
		switch h.Role {
		case "fix-gate":
			out = append(out, Outcome{Pass: false, Run: h.Run})
		case "audit":
			out = append(out, Outcome{Pass: true, Run: h.Run})
		}
	}
	return out
}

// Rate is a count of passes out of a number of items.
type Rate struct {
	Pass int `json:"pass"`
	Of   int `json:"of"`
}

// Share is the rate as a fraction; a rate of no items is 0.
func (r Rate) Share() float64 {
	if r.Of == 0 {
		return 0
	}
	return float64(r.Pass) / float64(r.Of)
}

// KindRates are the rates of the items of one kind.
type KindRates struct {
	Kind      string `json:"kind"`
	Items     int    `json:"items"`
	GateFirst Rate   `json:"gate_first_try"`  // items whose first gate passed, of the items with a gate
	AuditPass Rate   `json:"audit_first_try"` // items whose first audit passed, of the items with an audit
	Done      Rate   `json:"done"`            // items that ended done, of the items with an audit
}

// Rates are the pass rates of a set of items, by kind and in total.
type Rates struct {
	Run   string      `json:"run,omitempty"`
	Kinds []KindRates `json:"kinds"`
	Total KindRates   `json:"total"`
}

// ComputeRates counts the rates of the items. With a run, only the gates and audits of that run count, and an item counts
// when the run gated or audited it: the rates of one batch of the writer, for its run record. The kind of an item
// comes from kinds, else from its state file; an item with neither is counted as unknown.
func ComputeRates(items []Item, kinds map[string]string, run string) Rates {
	by := map[string]*KindRates{}
	total := KindRates{Kind: "total"}
	pick := func(all []Outcome) []Outcome {
		if run == "" {
			return all
		}
		var out []Outcome
		for _, o := range all {
			if o.Run == run {
				out = append(out, o)
			}
		}
		return out
	}
	for _, it := range items {
		gs, as := pick(it.Gates), pick(it.Audits)
		if run != "" && len(gs) == 0 && len(as) == 0 {
			continue
		}
		k := kinds[it.Slug]
		if k == "" {
			k = it.Kind
		}
		if k == "" {
			k = "unknown"
		}
		kr := by[k]
		if kr == nil {
			kr = &KindRates{Kind: k}
			by[k] = kr
		}
		for _, r := range []*KindRates{kr, &total} {
			r.Items++
			if len(gs) > 0 {
				r.GateFirst.Of++
				if gs[0].Pass {
					r.GateFirst.Pass++
				}
			}
			if len(as) > 0 {
				r.AuditPass.Of++
				if as[0].Pass {
					r.AuditPass.Pass++
				}
				r.Done.Of++
				if it.Status == "done" {
					r.Done.Pass++
				}
			}
		}
	}
	out := Rates{Run: run, Total: total, Kinds: []KindRates{}}
	for _, kr := range by {
		out.Kinds = append(out.Kinds, *kr)
	}
	sort.Slice(out.Kinds, func(i, j int) bool { return out.Kinds[i].Kind < out.Kinds[j].Kind })
	return out
}
