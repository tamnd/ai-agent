// SPDX-License-Identifier: Apache-2.0

// Package results reads the results register: the result cards of cards/result/ and the versions of the benchmarks (spec 10). It computes the comparable group and the evidence label of each result, and the results table of a benchmark version.
package results

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/tamnd/ai-agent/card"
	"github.com/tamnd/ai-agent/node"
)

// Kind is the kind of a result card.
const Kind = "result"

// ReleasesFile is the file of the versions of a benchmark node (spec 10.3). A benchmark card holds them in its field versions.
const ReleasesFile = "releases.yaml"

// The evidence labels of a result (spec 10.6.1).
const (
	Reported     = "reported"
	Replicated   = "replicated"
	Contested    = "contested"
	MeasuredHere = "measured-here"
)

// The values of checks.cite. Only a result whose number the citation check found is in a table (rule M08).
const (
	CiteFound    = "found"
	CiteNotFound = "not-found"
)

// Unknown is the value of a setting key that the source does not state (rule M03).
const Unknown = "unknown"

// SettingKeys are the keys of a setting (spec 10.5), in the order of the table.
var SettingKeys = []string{"shots", "prompt", "cot", "decoding", "temperature", "top_p", "n", "pass_at", "tools", "scaffold", "compute", "split", "quantization", "model_variant", "harness", "date_run", "notes"}

// HardKeys are the keys of a setting that make two results not comparable when their values differ (spec 10.5).
var HardKeys = []string{"shots", "prompt", "cot", "decoding", "temperature", "top_p", "n", "pass_at", "tools", "scaffold", "compute", "split", "model_variant"}

// Score is the number of a result with its unit. A rating also has its interval and the number of votes (spec 10.10).
type Score struct {
	V        *float64  `json:"v"`
	Unit     string    `json:"unit"`
	Interval []float64 `json:"interval,omitempty"`
	Votes    int       `json:"votes,omitempty"`
}

// Source is the source of a result with its locator.
type Source struct {
	Src  string `json:"src"`
	Loc  string `json:"loc"`
	Kind string `json:"kind"`
	Date string `json:"date"`
}

// Checks are the results of the checks of a card: the citation check and the comparable group.
type Checks struct {
	Cite            string `json:"cite,omitempty"`
	ComparableGroup string `json:"comparable_group,omitempty"`
}

// Result is one result card.
type Result struct {
	Card         *card.Card     `json:"-"`
	Tag          string         `json:"tag"`
	Model        string         `json:"model"`
	ModelVariant string         `json:"model_variant,omitempty"`
	Benchmark    string         `json:"benchmark"`
	Version      string         `json:"version"`
	Metric       string         `json:"metric"`
	Score        Score          `json:"score"`
	Setting      map[string]any `json:"setting"`
	Source       Source         `json:"source"`
	SelfReported bool           `json:"self_reported,omitempty"`
	Flags        []string       `json:"flags,omitempty"`
	// Arena and Snapshot name the arena and the date of the snapshot of a rating (rule M14).
	Arena    string `json:"arena,omitempty"`
	Snapshot string `json:"snapshot,omitempty"`
	Checks   Checks `json:"checks"`
	// Evidence is computed (spec 10.6.1).
	Evidence string `json:"evidence,omitempty"`
	// Group is the comparable group, computed from the version, the metric and the hard keys of the setting (spec 10.6.3).
	Group string `json:"-"`
	// Unknown lists the hard keys whose value is unknown. Such a result is in a group of its own.
	Unknown []string `json:"-"`
	// Err is set when the fields of the card do not have the types of 10.6.1.
	Err error `json:"-"`
}

// Version is one version of a benchmark (spec 10.3).
type Version struct {
	ID            string `json:"id" yaml:"id"`
	Name          string `json:"name" yaml:"name"`
	Metric        string `json:"metric,omitempty" yaml:"metric,omitempty"`
	Date          any    `json:"date,omitempty" yaml:"date,omitempty"`
	DerivedFrom   any    `json:"derived_from,omitempty" yaml:"derived_from,omitempty"`
	Size          any    `json:"size,omitempty" yaml:"size,omitempty"`
	Data          any    `json:"data,omitempty" yaml:"data,omitempty"`
	Protocol      any    `json:"protocol,omitempty" yaml:"protocol,omitempty"`
	Ceiling       any    `json:"ceiling,omitempty" yaml:"ceiling,omitempty"`
	HumanBaseline any    `json:"human_baseline,omitempty" yaml:"human_baseline,omitempty"`
	KnownIssues   any    `json:"known_issues,omitempty" yaml:"known_issues,omitempty"`
}

// Register is the results register of a corpus.
type Register struct {
	Results []*Result
	// Versions maps a benchmark slug to its versions by id.
	Versions map[string]map[string]*Version
	// VersionErrs are the files of versions that do not parse.
	VersionErrs []error
	// Groups maps a comparable group to its results, in the order of the cards.
	Groups map[string][]*Result
}

// Parse reads a result from its card.
func Parse(c *card.Card) *Result {
	r := &Result{Card: c, Tag: c.Tag}
	b, err := json.Marshal(c.Fields)
	if err == nil {
		err = json.Unmarshal(b, r)
	}
	if err != nil {
		r.Err = fmt.Errorf("the fields do not have the types of a result card (10.6.1): %w", err)
	}
	return r
}

// Load reads the result cards and the versions of the benchmarks of the corpus, and computes the groups and the evidence.
func Load(c *node.Corpus) *Register {
	reg := &Register{Versions: map[string]map[string]*Version{}, Groups: map[string][]*Result{}}
	add := func(bench string, vs []*Version) {
		if reg.Versions[bench] == nil {
			reg.Versions[bench] = map[string]*Version{}
		}
		for _, v := range vs {
			if v != nil && v.ID != "" {
				reg.Versions[bench][v.ID] = v
			}
		}
	}
	for _, cd := range c.Cards {
		if cd.Kind == "benchmark" {
			if raw, ok := cd.Fields["versions"]; ok {
				var vs []*Version
				if err := json.Unmarshal(raw, &vs); err != nil {
					reg.VersionErrs = append(reg.VersionErrs, fmt.Errorf("%s:%d: the field versions is not a list of versions (10.3)", cd.File, cd.Line))
				}
				add(cd.Slug, vs)
			}
		}
		if cd.Kind == Kind {
			reg.Results = append(reg.Results, Parse(cd))
		}
	}
	for _, n := range c.Kind("benchmark") {
		b, err := os.ReadFile(filepath.Join(n.Dir, ReleasesFile))
		if err != nil {
			continue
		}
		var vs []*Version
		if err := yaml.Unmarshal(b, &vs); err != nil {
			reg.VersionErrs = append(reg.VersionErrs, fmt.Errorf("%s/%s: %w", n.Rel, ReleasesFile, err))
		}
		add(n.Slug, vs)
	}
	for _, r := range reg.Results {
		r.Group, r.Unknown = GroupOf(r)
		reg.Groups[r.Group] = append(reg.Groups[r.Group], r)
	}
	for _, rs := range reg.Groups {
		for _, r := range rs {
			r.Evidence = evidence(r, rs)
		}
	}
	return reg
}

// value gives the value of a hard key, as the group writes it. model_variant is a field of the card.
func (r *Result) value(k string) (string, bool) {
	if k == "model_variant" && r.ModelVariant != "" {
		return r.ModelVariant, true
	}
	v, ok := r.Setting[k]
	if !ok || v == nil {
		return "", false
	}
	switch x := v.(type) {
	case string:
		return x, true
	case float64:
		return fmt.Sprint(x), true
	}
	b, _ := json.Marshal(v)
	return string(b), true
}

// GroupOf gives the comparable group of a result: the benchmark version, the metric, and the value of each hard key that the setting has. A result with an unknown hard key is in a group of its own (spec 10.6.3). A rating is comparable only inside one snapshot of one arena, and every entrant of that snapshot is on one scale whatever its variant, so its group is the arena and the snapshot alone (10.10).
func GroupOf(r *Result) (string, []string) {
	if r.Metric == "rating" {
		return r.Benchmark + "@" + r.Version + "/rating/arena=" + r.Arena + ",snapshot=" + r.Snapshot, nil
	}
	var parts, unknown []string
	for _, k := range HardKeys {
		v, ok := r.value(k)
		if !ok {
			continue
		}
		if v == Unknown {
			unknown = append(unknown, k)
		}
		parts = append(parts, k+"="+v)
	}
	g := r.Benchmark + "@" + r.Version + "/" + r.Metric + "/" + strings.Join(parts, ",")
	if len(unknown) > 0 {
		g += "/unknown:" + r.Tag
	}
	return g, unknown
}

// Tolerance gives how far apart two scores of a unit can be and still agree (spec 10.6.1): 1 point of percentage for a fraction or a percentage, and 1% of the score for the other units.
func Tolerance(unit string, v float64) float64 {
	switch unit {
	case "fraction":
		return 0.01
	case "percent", "points":
		return 1
	}
	return math.Abs(v) * 0.01
}

// independent tells if two results are from independent sources: the sources differ, and the developer does not report both.
func independent(a, b *Result) bool {
	return a.Source.Src != b.Source.Src && (!a.SelfReported || !b.SelfReported)
}

// MeasuredBy tells if our harness produced the result (spec 10.4).
func (r *Result) MeasuredBy() bool {
	return r.Source.Kind == "harness" || strings.HasPrefix(r.Source.Src, "exp:")
}

func evidence(r *Result, group []*Result) string {
	if r.MeasuredBy() {
		return MeasuredHere
	}
	if r.Score.V == nil {
		return Reported
	}
	label := Reported
	for _, o := range group {
		if o == r || o.Score.V == nil || o.Model != r.Model || o.ModelVariant != r.ModelVariant || o.Checks.Cite == CiteNotFound || !independent(r, o) {
			continue
		}
		if math.Abs(*o.Score.V-*r.Score.V) > Tolerance(r.Score.Unit, *r.Score.V)+1e-12 {
			return Contested
		}
		label = Replicated
	}
	return label
}

// Shown tells if a table shows the result: the citation check found its number (rule M08).
func (r *Result) Shown() bool { return r.Checks.Cite == CiteFound }

// Table is the computed results table of a benchmark version (spec 10.7).
type Table struct {
	Benchmark string  `json:"benchmark"`
	Version   string  `json:"version"`
	Groups    []Group `json:"groups"`
	// Other holds the results with an unknown hard key, each in a group of its own.
	Other []Group `json:"other,omitempty"`
	// Count is the number of results in the table, and Hidden the number of results that the citation check did not find.
	Count  int `json:"count"`
	Hidden int `json:"hidden"`
}

// Group is one comparable group of a table, with its rows from the best score down.
type Group struct {
	Key     string    `json:"key"`
	Metric  string    `json:"metric"`
	Setting string    `json:"setting"` // the hard keys, as the header of the group prints them
	Rows    []*Result `json:"rows"`
}

// TableOf computes the table of one benchmark version. The rows of a group go from the highest score down, except for a metric in lower, where the lowest is the best.
func (reg *Register) TableOf(bench, version string, lower map[string]bool) Table {
	t := Table{Benchmark: bench, Version: version}
	byKey := map[string]*Group{}
	var keys []string
	for _, r := range reg.Results {
		if r.Benchmark != bench || r.Version != version || r.Err != nil {
			continue
		}
		if !r.Shown() {
			t.Hidden++
			continue
		}
		g := byKey[r.Group]
		if g == nil {
			g = &Group{Key: r.Group, Metric: r.Metric, Setting: r.SettingText()}
			byKey[r.Group] = g
			keys = append(keys, r.Group)
		}
		g.Rows = append(g.Rows, r)
		t.Count++
	}
	sort.Strings(keys)
	for _, k := range keys {
		g := byKey[k]
		down := !lower[g.Metric]
		sort.SliceStable(g.Rows, func(i, j int) bool {
			a, b := score(g.Rows[i]), score(g.Rows[j])
			if down {
				return a > b
			}
			return a < b
		})
		if len(g.Rows) > 0 && len(g.Rows[0].Unknown) > 0 {
			t.Other = append(t.Other, *g)
		} else {
			t.Groups = append(t.Groups, *g)
		}
	}
	return t
}

func score(r *Result) float64 {
	if r.Score.V == nil {
		return math.Inf(-1)
	}
	return *r.Score.V
}

// SettingText gives the hard keys of the setting as a header, such as "shots 5, prompt custom, cot none", and for a
// rating its arena and snapshot, such as "arena LMArena text, snapshot 2026-11-09".
func (r *Result) SettingText() string {
	if r.Metric == "rating" {
		return "arena " + r.Arena + ", snapshot " + r.Snapshot
	}
	var parts []string
	for _, k := range HardKeys {
		if v, ok := r.value(k); ok {
			parts = append(parts, strings.ReplaceAll(k, "_", " ")+" "+v)
		}
	}
	if len(parts) == 0 {
		return "no setting given"
	}
	return strings.Join(parts, ", ")
}

// Benchmarks gives the slugs of the benchmarks that have versions or results, sorted.
func (reg *Register) Benchmarks() []string {
	var out []string
	for b := range reg.Versions {
		out = append(out, b)
	}
	for _, r := range reg.Results {
		if r.Benchmark != "" && !slices.Contains(out, r.Benchmark) {
			out = append(out, r.Benchmark)
		}
	}
	sort.Strings(out)
	return out
}
