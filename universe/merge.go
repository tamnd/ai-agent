// SPDX-License-Identifier: Apache-2.0

package universe

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/tamnd/ai-agent/card"
	"github.com/tamnd/ai-agent/harvest"
)

// Priority orders the sources when two rows of one item disagree on a name or a kind: the first source wins.
var Priority = []string{"federal-register", "epoch-ai", "aiid", "pwc-dump", "acl", "arxiv", "openalex", "wikidata", "huggingface", "github"}

func priority(source string) int {
	for i, s := range Priority {
		if s == source {
			return i
		}
	}
	return len(Priority)
}

// ReportFile is the record of the last merge.
const ReportFile = "reports/universe.json"

// Conflict is a join that the merge did not make. The curator decides it (18.3).
type Conflict struct {
	Key string `json:"key"`
	A   string `json:"a"`
	B   string `json:"b"`
	Why string `json:"why"`
}

// Report is the record of a merge.
type Report struct {
	Date      string         `json:"date"`
	Rows      int            `json:"rows"`
	Items     int            `json:"items"`
	New       int            `json:"new"`
	Updated   int            `json:"updated"`
	Unchanged int            `json:"unchanged"`
	Decided   map[string]int `json:"decided_by_rule,omitempty"`
	ByOutcome map[string]int `json:"by_outcome"`
	ByKind    map[string]int `json:"by_kind"`
	BySource  map[string]int `json:"by_source"`
	Conflicts int            `json:"conflicts"`
	// Sample holds the first conflicts, so the record stays small.
	Sample []Conflict `json:"sample,omitempty"`
}

// member is one register row or one inventory row of the merge.
type member struct {
	item *Item
	row  *harvest.Row
}

func (m member) label() string {
	if m.item != nil {
		return m.item.ID
	}
	return m.row.Source + ":" + m.row.ID
}

func (m member) kind() string {
	if m.item != nil {
		return m.item.Kind
	}
	return m.row.Kind
}

// cluster is the state of one set of the union-find.
type cluster struct {
	sameAs  map[string]string
	sources map[string]bool
	items   int
	kind    string
}

type merger struct {
	m         []member
	parent    []int
	cl        []*cluster
	conflicts []Conflict
	refused   map[[2]int]bool // the pairs of sets whose join is refused, so each conflict is listed once
}

func (g *merger) find(i int) int {
	for g.parent[i] != i {
		g.parent[i] = g.parent[g.parent[i]]
		i = g.parent[i]
	}
	return i
}

// union joins two sets unless that would put two values of one id, two rows of one source, two register rows or
// two kinds in one item. check is false for the join of a row with the register row that already lists it.
func (g *merger) union(a, b int, key string, check bool) {
	ra, rb := g.find(a), g.find(b)
	if ra == rb {
		return
	}
	ca, cb := g.cl[ra], g.cl[rb]
	if check {
		why := ""
		switch {
		case ca.kind != "" && cb.kind != "" && ca.kind != cb.kind:
			why = "the kinds differ: " + ca.kind + " and " + cb.kind
		case ca.items > 0 && cb.items > 0:
			why = "both are rows of the register"
		}
		for k, v := range ca.sameAs {
			if w, ok := cb.sameAs[k]; ok && w != v && why == "" {
				why = "the " + k + " ids differ: " + v + " and " + w
			}
		}
		for s := range ca.sources {
			if cb.sources[s] && why == "" {
				why = "both have a row of " + s
			}
		}
		if why != "" {
			pair := [2]int{min(ra, rb), max(ra, rb)}
			if g.refused[pair] {
				return
			}
			if g.refused == nil {
				g.refused = map[[2]int]bool{}
			}
			g.refused[pair] = true
			g.conflicts = append(g.conflicts, Conflict{Key: key, A: g.m[a].label(), B: g.m[b].label(), Why: why})
			return
		}
	}
	if len(ca.sameAs)+len(ca.sources) < len(cb.sameAs)+len(cb.sources) {
		ra, rb, ca, cb = rb, ra, cb, ca
	}
	for k, v := range cb.sameAs {
		if _, ok := ca.sameAs[k]; !ok {
			ca.sameAs[k] = v
		}
	}
	for s := range cb.sources {
		ca.sources[s] = true
	}
	ca.items += cb.items
	if ca.kind == "" {
		ca.kind = cb.kind
	}
	g.parent[rb] = ra
	g.cl[rb] = nil
}

var arxivVersion = regexp.MustCompile(`v\d+$`)

// NormID puts an id in the form that joins: DOIs in lowercase, arXiv ids without a version, QIDs in uppercase.
func NormID(k, v string) string {
	v = strings.TrimSpace(v)
	switch k {
	case "doi":
		return strings.ToLower(strings.TrimPrefix(strings.TrimPrefix(v, "https://doi.org/"), "doi:"))
	case "arxiv":
		return arxivVersion.ReplaceAllString(strings.TrimPrefix(v, "arXiv:"), "")
	case "wikidata":
		return strings.ToUpper(v)
	case "github":
		return strings.ToLower(v) // GitHub names do not tell case apart
	}
	return v
}

// NormName is the name key of the second join: the letters and digits of the name in lowercase.
func NormName(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Merge reads the register and the inventories, joins the rows that name one item, and writes the register and its
// report. It joins by a shared same_as id first and then by the name and the kind. A new item is pending unless a
// scope rule decides it from its source alone; a decided item keeps its outcome.
func Merge(root, date string) (*Report, error) {
	items, err := Read(root)
	if err != nil {
		return nil, err
	}
	inv, err := harvest.ReadInventories(root)
	if err != nil {
		return nil, err
	}
	rules, err := ReadRules(root)
	if err != nil {
		return nil, err
	}
	g := &merger{}
	for _, it := range items {
		g.add(member{item: it})
	}
	names := make([]string, 0, len(inv))
	for s := range inv {
		names = append(names, s)
	}
	sort.Slice(names, func(i, j int) bool {
		return priority(names[i]) < priority(names[j]) || priority(names[i]) == priority(names[j]) && names[i] < names[j]
	})
	rows := 0
	for _, s := range names {
		for i := range inv[s] {
			g.add(member{row: &inv[s][i]})
			rows++
		}
	}
	// The three joins, in order: a row with the register row that lists it, then shared ids, then names.
	for pass := range 3 {
		index := map[string]int{}
		for i, m := range g.m {
			for _, k := range g.keys(m, pass) {
				if j, ok := index[k]; ok {
					g.union(j, i, k, pass > 0)
				} else {
					index[k] = i
				}
			}
		}
	}
	rep := &Report{Date: date, Rows: rows, ByOutcome: map[string]int{}, ByKind: map[string]int{}, BySource: map[string]int{}, Decided: map[string]int{}}
	groups := map[int][]member{}
	var roots []int
	for i := range g.m {
		r := g.find(i)
		if _, ok := groups[r]; !ok {
			roots = append(roots, r)
		}
		groups[r] = append(groups[r], g.m[i])
	}
	taken := map[string]bool{}
	for _, it := range items {
		taken[it.ID] = true
	}
	var out []*Item
	for _, r := range roots {
		it, state := build(groups[r], rules, date, taken)
		switch state {
		case "new":
			rep.New++
		case "updated":
			rep.Updated++
		default:
			rep.Unchanged++
		}
		if strings.HasPrefix(it.DecidedBy, "rule:") && it.Decided == date && state != "unchanged" {
			rep.Decided[strings.TrimPrefix(it.DecidedBy, "rule:")]++
		}
		out = append(out, it)
	}
	for _, it := range out {
		rep.ByOutcome[it.Outcome]++
		rep.ByKind[it.Kind]++
		for _, s := range it.Sources {
			src, _, _ := strings.Cut(s, ":")
			rep.BySource[src]++
		}
	}
	rep.Items = len(out)
	rep.Conflicts = len(g.conflicts)
	rep.Sample = g.conflicts[:min(len(g.conflicts), 200)]
	if err := Write(root, out); err != nil {
		return nil, err
	}
	b, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(root, filepath.Dir(ReportFile)), 0o755); err != nil {
		return nil, err
	}
	return rep, os.WriteFile(filepath.Join(root, ReportFile), append(b, '\n'), 0o644)
}

func (g *merger) add(m member) {
	c := &cluster{sameAs: map[string]string{}, sources: map[string]bool{}, kind: m.kind()}
	var same map[string]string
	if m.item != nil {
		c.items = 1
		same = m.item.SameAs
		for _, s := range m.item.Sources {
			src, _, _ := strings.Cut(s, ":")
			c.sources[src] = true
		}
	} else {
		same = m.row.SameAs
		c.sources[m.row.Source] = true
	}
	for k, v := range same {
		if v != "" {
			c.sameAs[k] = NormID(k, v)
		}
	}
	g.parent = append(g.parent, len(g.m))
	g.m = append(g.m, m)
	g.cl = append(g.cl, c)
}

// keys are the join keys of a member in a pass: 0 the source rows, 1 the same_as ids, 2 the name and the kind.
func (g *merger) keys(m member, pass int) []string {
	var out []string
	switch pass {
	case 0:
		if m.item != nil {
			for _, s := range m.item.Sources {
				out = append(out, "src="+s)
			}
		} else {
			out = append(out, "src="+m.row.Source+":"+m.row.ID)
		}
	case 1:
		var same map[string]string
		if m.item != nil {
			same = m.item.SameAs
		} else {
			same = m.row.SameAs
		}
		for k, v := range same {
			if v != "" {
				out = append(out, k+"="+NormID(k, v))
			}
		}
		sort.Strings(out)
	case 2:
		name := ""
		if m.item != nil {
			name = m.item.Name
		} else {
			name = m.row.Name
		}
		if n := NormName(name); n != "" {
			out = append(out, "name="+m.kind()+"="+n)
		}
	}
	return out
}

// idOrder is the order of the ids that name a new item: "u:arxiv:1207.0580" for a work, "u:epoch:GPT-4" for a model.
var idOrder = map[bool][]string{
	true:  {"arxiv", "doi", "wikidata", "openalex", "acl", "pwc"},
	false: {"epoch", "aiid", "federal_register", "wikidata", "pwc", "hub", "hub_dataset", "github", "arxiv", "doi"},
}

func build(ms []member, rules []Rule, date string, taken map[string]bool) (*Item, string) {
	var it *Item
	var rows []*harvest.Row
	for _, m := range ms {
		if m.item != nil {
			it = m.item
		} else {
			rows = append(rows, m.row)
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if a, b := priority(rows[i].Source), priority(rows[j].Source); a != b {
			return a < b
		}
		return rows[i].ID < rows[j].ID
	})
	state := "unchanged"
	var before []byte
	if it == nil {
		r := rows[0]
		it = &Item{Name: r.Name, Kind: r.Kind, Outcome: "pending", Added: date}
		same := map[string]string{}
		for _, r := range rows {
			for k, v := range r.SameAs {
				if _, ok := same[k]; !ok && v != "" {
					same[k] = NormID(k, v)
				}
			}
		}
		for _, k := range idOrder[it.Kind == "work"] {
			if v := same[k]; v != "" {
				it.ID = "u:" + k + ":" + v
				break
			}
		}
		if it.ID == "" {
			it.ID = "u:" + r.Source + ":" + r.ID
		}
		for base, n := it.ID, 2; taken[it.ID]; n++ {
			it.ID = base + "#" + strconv.Itoa(n)
		}
		taken[it.ID] = true
		state = "new"
	} else {
		before, _ = json.Marshal(it)
	}
	for _, r := range rows {
		ref := r.Source + ":" + r.ID
		if !slices.Contains(it.Sources, ref) {
			it.Sources = append(it.Sources, ref)
		}
		for k, v := range r.SameAs {
			if v == "" {
				continue
			}
			if it.SameAs == nil {
				it.SameAs = map[string]string{}
			}
			if _, ok := it.SameAs[k]; !ok {
				it.SameAs[k] = NormID(k, v)
			}
		}
		for _, a := range append([]string{r.Name}, r.Aliases...) {
			if a != "" && !strings.EqualFold(a, it.Name) && !containsFold(it.Aliases, a) && len(it.Aliases) < 20 {
				it.Aliases = append(it.Aliases, a)
			}
		}
	}
	sort.Strings(it.Sources)
	sort.Strings(it.Aliases)
	// The signals of the rows replace the old ones; the first source wins a key that two rows have.
	for i := len(rows) - 1; i >= 0; i-- {
		for k, v := range rows[i].Signals {
			if it.Signals == nil {
				it.Signals = map[string]any{}
			}
			it.Signals[k] = v
		}
	}
	if it.Outcome == "" {
		it.Outcome = "pending"
	}
	if it.Outcome == "pending" {
		for _, rule := range rules {
			if rule.Kind != it.Kind || rule.Outcome == "" {
				continue
			}
			// A rule decides when it has a source and the item has a row of it, and its thresholds, if any, are met
			// by the signals of the item.
			switch {
			case rule.Unconditional():
				if !hasSource(rows, rule.Source) {
					continue
				}
			case len(rule.Thresholds) > 0:
				if rule.Source != "" && !hasSource(rows, rule.Source) || !rule.Meets(it.Signals) {
					continue
				}
			default:
				continue
			}
			it.Outcome, it.DecidedBy, it.Decided, it.Reason = rule.Outcome, "rule:"+rule.ID, date, rule.ID+": "+firstSentence(rule.Rule)
			if rule.Outcome == "card" || rule.Outcome == "node" {
				it.Target = URI("NEW-" + SlugFor(it))
			}
			break
		}
	}
	if state != "new" {
		after, _ := json.Marshal(it)
		if string(after) != string(before) {
			state = "updated"
		}
	}
	return it, state
}

// SlugFor is the slug that a new page or card of the item gets: "aiid-<id>" for an incident (04.4), else the slug of
// its name.
func SlugFor(it *Item) string {
	if it.Kind == "incident" && it.SameAs["aiid"] != "" {
		return "aiid-" + it.SameAs["aiid"]
	}
	if s := card.Slugify(it.Name); s != "" {
		return s
	}
	return card.Slugify(it.Kind + " " + strings.NewReplacer(":", " ", "/", " ").Replace(it.ID))
}

func hasSource(rows []*harvest.Row, s string) bool {
	for _, r := range rows {
		if r.Source == s {
			return true
		}
	}
	return false
}

func firstSentence(s string) string {
	if i := strings.Index(s, ". "); i > 0 {
		return s[:i+1]
	}
	return s
}

func containsFold(l []string, s string) bool {
	for _, x := range l {
		if strings.EqualFold(x, s) {
			return true
		}
	}
	return false
}
