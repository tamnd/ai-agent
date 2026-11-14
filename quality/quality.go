// SPDX-License-Identifier: Apache-2.0

// Package quality computes the metrics of spec 15.9 that show whether the checks, the audit and the review work: the
// shares of grounded and supported claims, the strength of the labels on core nodes, the reviewed share of the core,
// the waivers, the stale claims, the dead links, the first-pass rate of the audit and the time to fix a finding. A metric
// whose records the project does not keep yet is listed with the reason, so that the dashboard never shows a guess.
package quality

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/tamnd/ai-agent/audit"
	"github.com/tamnd/ai-agent/cite"
	"github.com/tamnd/ai-agent/claims"
	"github.com/tamnd/ai-agent/node"
	"github.com/tamnd/ai-agent/status"
)

// Target is the bound that a metric must meet at a milestone. A metric with an upper bound must stay under it; any
// other must reach it.
type Target struct {
	Value float64 `json:"value"`
	Under bool    `json:"under,omitempty"`
}

// Met tells if a value meets the target.
func (t Target) Met(v float64) bool {
	if t.Under {
		return v < t.Value
	}
	return v >= t.Value
}

func (t Target) String(unit string) string {
	s := format(t.Value, unit)
	if t.Under {
		return "under " + s
	}
	return s
}

// The units of the metrics.
const (
	Share   = "share"    // a fraction from 0 to 1
	PerK    = "per_1000" // a count per 1,000 nodes
	Missing = ""
)

// Metric is one row of the table of 15.9.
type Metric struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Unit string `json:"unit"`
	// Num and Den are the counts behind the value: the claims, the nodes or the findings that meet the metric and all of
	// them.
	Num   int      `json:"num"`
	Den   int      `json:"den"`
	Value *float64 `json:"value"`
	// M3 and M5 are the targets of the milestones; a metric that is only tracked has none.
	M3 *Target `json:"m3,omitempty"`
	M5 *Target `json:"m5,omitempty"`
	// MetM3 and MetM5 tell if the value meets each target; they are null without a value or a target.
	MetM3 *bool `json:"met_m3"`
	MetM5 *bool `json:"met_m5"`
	// NotComputed says why the metric has no value.
	NotComputed string `json:"not_computed,omitempty"`
	Note        string `json:"note,omitempty"`
}

// Report is the output of aik report quality.
type Report struct {
	Date    string   `json:"date"`
	Nodes   int      `json:"nodes"`
	Core    int      `json:"core"`
	Metrics []Metric `json:"metrics"`
}

// Issue is a finding of 15.8 as gh issue list --json number,state,labels,createdAt,closedAt gives it.
type Issue struct {
	Number    int       `json:"number"`
	State     string    `json:"state"`
	Labels    []Label   `json:"labels"`
	CreatedAt time.Time `json:"createdAt"`
	ClosedAt  time.Time `json:"closedAt"`
}

// Label is a label of an issue.
type Label struct {
	Name string `json:"name"`
}

// Severity is the severity label of a finding, S1, S2 or S3, or "" when the issue is not a finding or has none.
func (i Issue) Severity() string {
	finding, sev := false, ""
	for _, l := range i.Labels {
		n := strings.ToUpper(strings.TrimSpace(l.Name))
		if n == "FINDING" {
			finding = true
		}
		for _, s := range []string{"S1", "S2", "S3"} {
			if n == s || strings.HasSuffix(n, ":"+s) || strings.HasSuffix(n, "/"+s) || strings.HasSuffix(n, " "+s) {
				sev = s
			}
		}
	}
	if !finding {
		return ""
	}
	return sev
}

// Waiver is a row of registers/waivers.yaml (15.3). Only the end date matters here.
type Waiver struct {
	Rule string `yaml:"rule" json:"rule"`
	Item string `yaml:"item" json:"item"`
	End  string `yaml:"end" json:"end"`
}

// Input is what the metrics are computed from.
type Input struct {
	// Corpus is the corpus as the site shows it, with the computed labels of the claims (site.ForAgents).
	Corpus *node.Corpus
	Status map[string]status.Status
	Cite   *cite.Index
	// Core holds the ids of the core items of the universe (18.7), by slug or tag.
	Core map[string]bool
	Now  time.Time
	// Audits holds, for each item that the writer ran, the verdicts of its audits in order.
	Audits map[string][]string
	// Links is the link check of the last weekly audit, or nil.
	Links *audit.Links
	// Issues are the findings, or nil when they were not given.
	Issues []Issue
	// Waivers are the rows of registers/waivers.yaml, or nil when the register does not exist.
	Waivers []Waiver
}

// strong are the headline labels above reported that count for the core: a proof, a measurement here or an independent
// replication. Contested comes before reported in HeadlineOrder, but it marks a claim in dispute, not a strong one.
var strong = map[string]bool{claims.Proved: true, claims.MeasuredHere: true, claims.Replicated: true}

// Compute computes the metrics of 15.9.
func Compute(in Input) Report {
	today := in.Now.UTC().Format("2006-01-02")
	r := Report{Date: today, Nodes: len(in.Corpus.Nodes), Core: len(in.Core)}

	var g2, g2n, g3, g3n, head, headn, stale, all int
	for _, n := range in.Corpus.Nodes {
		w := in.Status[n.Slug].W.Level
		core := in.Core[n.Slug] || (n.Tag != "" && in.Core[n.Tag])
		for i := range n.Claims {
			c := &n.Claims[i]
			all++
			if c.ReviewAfter != "" && c.ReviewAfter < today {
				stale++
			}
			if core {
				headn++
				if strong[c.Headline] {
					head++
				}
			}
			works := 0
			grounded, supported := true, true
			for _, e := range c.Evidence {
				if e.Work == "" {
					continue
				}
				works++
				rec, ok := in.Cite.Current(n.Tag, c, e)
				if !ok || !rec.Pass {
					grounded = false
				}
				if !ok || rec.Verdict != cite.Supported {
					supported = false
				}
			}
			if works == 0 {
				continue
			}
			g2n++
			if grounded {
				g2++
			}
			if w >= 3 {
				g3n++
				if supported {
					g3++
				}
			}
		}
	}

	add := func(m Metric) { r.Metrics = append(r.Metrics, m) }
	pct := func(v float64) *Target { return &Target{Value: v} }
	under := func(v float64) *Target { return &Target{Value: v, Under: true} }

	add(missing("critical-defects", "critical defect rate in the sample, upper Wilson bound", under(0.01), under(0.005),
		"the weekly sample (15.6) records its picks but not yet the defects that the reviewers find in them"))
	add(missing("major-defects", "major defect rate in the sample, upper Wilson bound", under(0.05), under(0.03),
		"the weekly sample (15.6) records its picks but not yet the defects that the reviewers find in them"))
	add(share("g2", "share of published claims that pass the citation check (G2)", g2, g2n, pct(0.95), pct(0.99),
		"claims with at least one cited work; a claim passes when the current record of each of its works passes"))
	add(share("g3", "share of claims on W3 pages with the verifier's supported (G3)", g3, g3n, pct(0.90), pct(0.97),
		"claims with a cited work on nodes at W3 or above; a claim counts when the verifier said supported for each of its works"))
	add(share("headline", "share of claims with a headline label above reported on core nodes", head, headn, pct(0.30), pct(0.45),
		"proved, measured_here or replicated; contested does not count"))

	if len(in.Audits) == 0 {
		add(missing("audit-first-pass", "audit first-pass rate (pass with no fix round)", nil, nil,
			"no writer state with an audit; give -state"))
	} else {
		first, n := 0, 0
		for _, vs := range in.Audits {
			if len(vs) == 0 {
				continue
			}
			n++
			if vs[0] == "pass" {
				first++
			}
		}
		add(share("audit-first-pass", "audit first-pass rate (pass with no fix round)", first, n, nil, nil,
			"items whose first audit by the writer passed"))
	}

	add(missing("disagreement", "person disagreement with audit verdicts (15.4)", under(0.10), under(0.05),
		"the audit of the audit does not record the verdicts of the people yet"))
	add(missing("canary", "canary catch rate of each auditor and verifier model", pct(0.90), pct(0.95),
		"no run of the auditors on registers/canaries.yaml is recorded yet"))
	add(missing("s1-hidden", "S1 findings hidden within 24 hours", pct(1), pct(1),
		"the quarantine lines do not name the finding they answer, so the time to hide is not known"))

	if in.Issues == nil {
		add(missing("s2-fixed", "S2 findings fixed within 7 days", pct(0.90), pct(0.95), "give -issues"))
	} else {
		fixed, n := 0, 0
		week := 7 * 24 * time.Hour
		for _, i := range in.Issues {
			if i.Severity() != "S2" {
				continue
			}
			closed := strings.EqualFold(i.State, "closed") && !i.ClosedAt.IsZero()
			switch {
			case closed:
				n++
				if i.ClosedAt.Sub(i.CreatedAt) <= week {
					fixed++
				}
			case in.Now.Sub(i.CreatedAt) > week:
				n++
			}
		}
		add(share("s2-fixed", "S2 findings fixed within 7 days", fixed, n, pct(0.90), pct(0.95),
			"S2 findings that are closed or more than 7 days old"))
	}

	w5 := 0
	seen := map[string]bool{}
	for _, n := range in.Corpus.Nodes {
		id := n.Slug
		if !in.Core[id] {
			if n.Tag == "" || !in.Core[n.Tag] {
				continue
			}
			id = n.Tag
		}
		if !seen[id] && in.Status[n.Slug].W.Level >= 5 {
			seen[id] = true
			w5++
		}
	}
	add(share("w5", "reviewed nodes (W5) among core nodes", w5, len(in.Core), pct(0.20), pct(0.60),
		"core items of the universe, written or not"))

	if in.Waivers == nil {
		add(Metric{ID: "waivers", Name: "active waivers per 1,000 nodes", Unit: PerK, Den: r.Nodes, M3: under(10), M5: under(5),
			Note: "registers/waivers.yaml does not exist", Value: ptr(0)}.met())
	} else {
		active := 0
		for _, w := range in.Waivers {
			if w.End == "" || w.End >= today {
				active++
			}
		}
		m := Metric{ID: "waivers", Name: "active waivers per 1,000 nodes", Unit: PerK, Num: active, Den: r.Nodes,
			M3: under(10), M5: under(5), Note: "waivers whose end date has not passed"}
		if r.Nodes > 0 {
			m.Value = ptr(float64(active) * 1000 / float64(r.Nodes))
		}
		add(m.met())
	}

	add(share("stale", "claims past review_after", stale, all, under(0.02), under(0.01),
		"all published claims; the freshness windows of the kinds (07.9) are not counted yet"))

	if in.Links == nil {
		add(missing("dead-links", "external links that fail and have no archive", under(0.01), under(0.005),
			"the audit report has no link check; run aik audit -links"))
	} else {
		add(share("dead-links", "external links that fail and have no archive", len(in.Links.Dead), in.Links.Checked,
			under(0.01), under(0.005), "every dead link counts, as the nodes do not record archived copies; unsure links do not"))
	}
	return r
}

func ptr(v float64) *float64 { return &v }

func missing(id, name string, m3, m5 *Target, why string) Metric {
	return Metric{ID: id, Name: name, Unit: Share, M3: m3, M5: m5, NotComputed: why}
}

func share(id, name string, num, den int, m3, m5 *Target, note string) Metric {
	m := Metric{ID: id, Name: name, Unit: Share, Num: num, Den: den, M3: m3, M5: m5, Note: note}
	if den > 0 {
		m.Value = ptr(float64(num) / float64(den))
	} else {
		m.NotComputed = "nothing to count yet"
	}
	return m.met()
}

func (m Metric) met() Metric {
	if m.Value == nil {
		return m
	}
	if m.M3 != nil {
		b := m.M3.Met(*m.Value)
		m.MetM3 = &b
	}
	if m.M5 != nil {
		b := m.M5.Met(*m.Value)
		m.MetM5 = &b
	}
	return m
}

// Off gives the metrics that miss a target of the milestone m3 or m5, sorted by id.
func (r Report) Off(m5 bool) []Metric {
	var out []Metric
	for _, m := range r.Metrics {
		met := m.MetM3
		if m5 {
			met = m.MetM5
		}
		if met != nil && !*met {
			out = append(out, m)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Text gives the value of a metric as it is printed.
func (m Metric) Text() string {
	if m.Value == nil {
		return "-"
	}
	return format(*m.Value, m.Unit)
}

// TargetText gives a target as it is printed.
func (m Metric) TargetText(t *Target) string {
	if t == nil {
		return "tracked"
	}
	return t.String(m.Unit)
}

func format(v float64, unit string) string {
	if unit == PerK {
		return fmt.Sprintf("%.1f", v)
	}
	s := fmt.Sprintf("%.1f%%", v*100)
	return strings.Replace(s, ".0%", "%", 1)
}
