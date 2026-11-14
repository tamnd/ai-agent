// SPDX-License-Identifier: Apache-2.0

// Package ingest turns the rows of the inventories into cards: aik ingest results makes the result cards of 10.6 from
// the result rows of the leaderboard adapters.
package ingest

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"go.yaml.in/yaml/v3"

	"github.com/tamnd/ai-agent/card"
	"github.com/tamnd/ai-agent/cite"
	"github.com/tamnd/ai-agent/harvest"
	"github.com/tamnd/ai-agent/node"
	"github.com/tamnd/ai-agent/results"
)

// CrosswalkFile maps the names that sources give to models, benchmark versions and metrics, for the names that do not
// match the name or an alias of one node or card: models to a model slug, or to "none" for a system that is not a
// model, whose rows are not ingested; benchmarks to "benchmark@version"; metrics to a metric slug.
const CrosswalkFile = "crosswalk/results.yaml"

// ReportDir holds the record of each run, reports/ingest/results-<date>.json.
const ReportDir = "reports/ingest"

// Options are the options of a run.
type Options struct {
	Root    string
	Sources []string // the inventories to read; all when empty
	Store   cite.Store
	Date    string
	Version string
	DryRun  bool // compute the report, write nothing
}

// Miss is a name that did not resolve, with the reason and the number of rows.
type Miss struct {
	Reason string `json:"reason"`
	Name   string `json:"name"`
	Rows   int    `json:"rows"`
}

// Report is the record of a run.
type Report struct {
	Date       string         `json:"date"`
	Aik        string         `json:"aik"`
	Sources    []string       `json:"sources"`
	Rows       int            `json:"rows"`
	New        int            `json:"new"`
	Updated    int            `json:"updated"`
	Unchanged  int            `json:"unchanged"`
	Duplicates int            `json:"duplicates"`
	Unresolved int            `json:"unresolved"`
	Skipped    int            `json:"skipped"`
	Cite       map[string]int `json:"cite"`
	Missing    []Miss         `json:"missing,omitempty"`
}

// The reasons of a miss.
const (
	NoModel       = "no model card or node has this name; add the model, an alias, or a line of " + CrosswalkFile
	Ambiguous     = "the name matches more than one model; add a line of " + CrosswalkFile
	BadCrosswalk  = CrosswalkFile + " maps the name to a slug of another kind"
	NoBenchmark   = "not a benchmark node or card (M02)"
	NoBenchName   = "no line of " + CrosswalkFile + " gives the benchmark and version of this name"
	NoMetricName  = "no metric node or card has this name; add an alias or a line of " + CrosswalkFile
	NoVersion     = "the benchmark has no such version (10.3)"
	NoMetric      = "not a metric node or card (M01)"
	NotResultRow  = "the row lacks a field of 10.6.1"
	citeUnchecked = "unchecked"
)

// Resolver finds the slugs of the models, benchmarks and metrics of a corpus.
type Resolver struct {
	kind     map[string]string                     // slug to kind
	names    map[string]map[string]map[string]bool // kind, name key, slugs
	cross    map[string]map[string]string          // section (models, benchmarks, metrics), name as written, slug
	versions map[string]map[string]*results.Version
}

// NewResolver indexes the corpus and reads the crosswalk.
func NewResolver(c *node.Corpus) (*Resolver, error) {
	r := &Resolver{kind: map[string]string{}, names: map[string]map[string]map[string]bool{},
		cross: map[string]map[string]string{}, versions: results.Load(c).Versions}
	named := map[string]bool{"model": true, "metric": true}
	addName := func(kind, name, slug string) {
		if k := NameKey(name); k != "" {
			if r.names[kind] == nil {
				r.names[kind] = map[string]map[string]bool{}
			}
			if r.names[kind][k] == nil {
				r.names[kind][k] = map[string]bool{}
			}
			r.names[kind][k][slug] = true
		}
	}
	for _, n := range c.Nodes {
		r.kind[n.Slug] = n.Kind
		if named[n.Kind] {
			addName(n.Kind, n.Slug, n.Slug)
			addName(n.Kind, n.Name, n.Slug)
			for _, a := range n.Aliases {
				addName(n.Kind, a, n.Slug)
			}
		}
	}
	for _, cd := range c.Cards {
		if cd.Owner != "" || cd.Kind == results.Kind {
			continue
		}
		if _, ok := r.kind[cd.Slug]; !ok {
			r.kind[cd.Slug] = cd.Kind
		}
		if named[cd.Kind] {
			addName(cd.Kind, cd.Slug, cd.Slug)
			addName(cd.Kind, cd.Name, cd.Slug)
			var aliases []string
			_ = json.Unmarshal(cd.Fields["aliases"], &aliases)
			for _, a := range aliases {
				addName(cd.Kind, a, cd.Slug)
			}
		}
	}
	b, err := os.ReadFile(filepath.Join(c.Root, CrosswalkFile))
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return nil, err
	default:
		var doc map[string]map[string]string
		if err := yaml.Unmarshal(b, &doc); err != nil {
			return nil, fmt.Errorf("%s: %w", CrosswalkFile, err)
		}
		for sec, m := range doc {
			if sec != "models" && sec != "benchmarks" && sec != "metrics" {
				return nil, fmt.Errorf("%s: the section %q is not models, benchmarks or metrics", CrosswalkFile, sec)
			}
			r.cross[sec] = map[string]string{}
			for name, slug := range m {
				r.cross[sec][strings.TrimSpace(name)] = strings.TrimSpace(slug)
			}
		}
	}
	return r, nil
}

var (
	caseRE  = regexp.MustCompile(`([a-z])([0-9])|([0-9])([a-z])`)
	parenRE = regexp.MustCompile(`^(.*\S)\s*\(([^()]+)\)$`)
	// sizeRE is a size of a model, a number with the letter of a unit and no letter before it: 7b, 1.5b, 350m, 2t.
	sizeRE = regexp.MustCompile(`\b\d+(?:\.\d+)?[bmkt]\b`)
)

// NameKey is the key that two names of one model share: the words and the sizes ("7b", "1.5b", "350m") of the name in
// lowercase in any order, then its other numbers in their order. So "Claude 4.5 Opus" and "claude-opus-4.5" have one
// key, as do "Vicuna v1.3 (7B)" and "Vicuna-7B-v1.3", and "GPT-4.5" and "GPT-5.4" do not. A "+" is the word plus, so
// "Command A+" and "Command A Plus" have one key and "Command A" another.
func NameKey(name string) string {
	s := strings.ReplaceAll(strings.ToLower(name), "+", " plus ")
	sizes := sizeRE.FindAllString(s, -1)
	s = sizeRE.ReplaceAllString(s, " ")
	s = caseRE.ReplaceAllString(caseRE.ReplaceAllString(s, "$1$3 $2$4"), "$1$3 $2$4")
	words, nums := sizes, []string(nil)
	for _, w := range strings.FieldsFunc(s, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		if unicode.IsDigit([]rune(w)[0]) {
			nums = append(nums, w)
		} else {
			words = append(words, w)
		}
	}
	sort.Strings(words)
	if len(nums) == 0 {
		return strings.Join(words, " ")
	}
	return strings.TrimSpace(strings.Join(words, " ") + " | " + strings.Join(nums, " "))
}

// Model resolves the name of a model as a source writes it: the crosswalk, then the names of the models, then the
// name without a last part in parentheses, which is then the variant ("Claude 3.5 Sonnet (20241022)"). It gives the
// slug, the variant and the reason of a miss. A slug of "none" means the name is not a model.
func (r *Resolver) Model(name string) (slug, variant, miss string) {
	name = strings.TrimSpace(name)
	try := func(n string) (string, string) { return r.named("models", "model", n, NoModel) }
	slug, miss = try(name)
	if miss != NoModel {
		return slug, "", miss
	}
	if m := parenRE.FindStringSubmatch(name); m != nil {
		if s, why := try(m[1]); why == "" {
			return s, strings.TrimSpace(m[2]), ""
		}
	}
	return "", "", NoModel
}

// named resolves a name of a kind: the section of the crosswalk, then the names of the nodes and cards of the kind.
func (r *Resolver) named(sec, kind, name, none string) (string, string) {
	if s, ok := r.cross[sec][name]; ok {
		if s != "none" && r.kind[s] != kind {
			return "", BadCrosswalk
		}
		return s, ""
	}
	set := r.names[kind][NameKey(name)]
	switch len(set) {
	case 0:
		return "", none
	case 1:
		for s := range set {
			return s, ""
		}
	}
	return "", Ambiguous
}

// Benchmark resolves the benchmark and version of a row: the fields benchmark and version, or the name that the
// source gives (benchmark_name) by the crosswalk. It gives the name to report with a miss.
func (r *Resolver) Benchmark(f map[string]any) (bench, version, name, miss string) {
	bench, _ = f["benchmark"].(string)
	version, _ = f["version"].(string)
	if bench == "" {
		name, _ = f["benchmark_name"].(string)
		x, ok := r.cross["benchmarks"][strings.TrimSpace(name)]
		if !ok {
			return "", "", name, NoBenchName
		}
		if bench, version, ok = strings.Cut(x, "@"); !ok {
			return "", "", name, BadCrosswalk
		}
	}
	switch {
	case r.kind[bench] != "benchmark":
		return "", "", bench, NoBenchmark
	case r.versions[bench][version] == nil:
		return "", "", bench + "@" + version, NoVersion
	}
	return bench, version, "", ""
}

// Metric resolves the metric of a row: the field metric, which must be a metric slug, or the name that the source
// gives (metric_name) by the crosswalk and the names of the metrics.
func (r *Resolver) Metric(f map[string]any) (slug, name, miss string) {
	if m, _ := f["metric"].(string); m != "" {
		if r.kind[m] != "metric" {
			return "", m, NoMetric
		}
		return m, "", ""
	}
	name, _ = f["metric_name"].(string)
	slug, miss = r.named("metrics", "metric", strings.TrimSpace(name), NoMetricName)
	if miss != "" {
		return "", name, miss
	}
	return slug, "", ""
}

// row are the fields of a result row.
type row struct {
	Model        string         `json:"model"`
	ModelVariant string         `json:"model_variant"`
	Score        results.Score  `json:"score"`
	Setting      map[string]any `json:"setting"`
	Source       results.Source `json:"source"`
}

// Key is the identity of a result (10.6.2): the model, the version, the metric, the setting and the source.
func Key(fields map[string]any) string {
	pick := map[string]any{}
	for _, k := range []string{"model", "model_variant", "benchmark", "version", "metric", "setting"} {
		pick[k] = fields[k]
	}
	if s, ok := fields["source"].(map[string]any); ok {
		pick["source"] = map[string]any{"src": s["src"], "loc": s["loc"]}
	}
	b, _ := json.Marshal(pick)
	return string(b)
}

// citer checks the numbers of the rows in their snapshots: in a JSON snapshot, the number must be a value of the
// record (an object) that also has the entry id of the row as a value. In a zip of CSV files, the number must be a
// cell of a row that also has the entry id as a cell, in the file that the locator names first ("gpqa_diamond.csv,
// row ..., column ...").
type citer struct {
	store cite.Store
	snaps map[string]map[string][]float64 // snapshot to the numbers of the records of each string
}

func (ct *citer) check(snap, loc, anchor string, v float64) string {
	idx, ok := ct.snaps[snap]
	if !ok {
		idx = nil
		// The rows name a snapshot as the manifest does, "b3:<hash>"; the store keys it by the hash.
		if b, err := ct.store.Get(strings.TrimPrefix(snap, "b3:")); err == nil {
			if idx = jsonRecords(b); idx == nil {
				idx = zipRecords(b)
			}
		}
		ct.snaps[snap] = idx
	}
	if idx == nil {
		return citeUnchecked
	}
	if _, zipped := idx[zipMark]; zipped {
		file, _, _ := strings.Cut(loc, ",")
		anchor = strings.TrimSpace(file) + "\x00" + anchor
	}
	for _, n := range idx[anchor] {
		if math.Abs(n-v) <= 1e-9*math.Max(1, math.Abs(v)) {
			return results.CiteFound
		}
	}
	return results.CiteNotFound
}

// zipMark is the key of the index of a zip snapshot, whose other keys are "<file>\x00<cell>".
const zipMark = "\x00zip"

// zipRecords maps each cell of each row of each CSV file of a zip to the numbers of that row, or is nil when the
// document is not a zip with a CSV file.
func zipRecords(b []byte) map[string][]float64 {
	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		return nil
	}
	out := map[string][]float64{}
	for _, f := range zr.File {
		if !strings.HasSuffix(f.Name, ".csv") || f.UncompressedSize64 > 64<<20 {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			continue
		}
		r := csv.NewReader(io.LimitReader(rc, 64<<20))
		r.FieldsPerRecord = -1
		recs, err := r.ReadAll()
		_ = rc.Close() // a read-only file of the zip; the records are read already
		if err != nil {
			continue
		}
		out[zipMark] = nil
		for _, rec := range recs {
			var nums []float64
			for _, c := range rec {
				if n, err := strconv.ParseFloat(strings.TrimSpace(c), 64); err == nil {
					nums = append(nums, n)
				}
			}
			for _, c := range rec {
				if c = strings.TrimSpace(c); c != "" {
					k := f.Name + "\x00" + c
					out[k] = append(out[k], nums...)
				}
			}
		}
	}
	if _, ok := out[zipMark]; !ok {
		return nil
	}
	return out
}

// jsonRecords maps each string value of an object of a JSON document to the numbers of that object, or is nil when
// the document is not JSON.
func jsonRecords(b []byte) map[string][]float64 {
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	var v any
	if d.Decode(&v) != nil {
		return nil
	}
	out := map[string][]float64{}
	var walk func(any)
	walk = func(x any) {
		switch t := x.(type) {
		case map[string]any:
			var strs []string
			var nums []float64
			for _, c := range t {
				switch y := c.(type) {
				case string:
					strs = append(strs, y)
				case json.Number:
					if f, err := y.Float64(); err == nil {
						nums = append(nums, f)
					}
				default:
					walk(c)
				}
			}
			for _, s := range strs {
				out[s] = append(out[s], nums...)
			}
		case []any:
			for _, c := range t {
				walk(c)
			}
		}
	}
	walk(v)
	return out
}

// Results makes and updates the result cards of the result rows of the inventories.
func Results(c *node.Corpus, o Options) (*Report, error) {
	res, err := NewResolver(c)
	if err != nil {
		return nil, err
	}
	inv, err := harvest.ReadInventories(o.Root)
	if err != nil {
		return nil, err
	}
	srcs := o.Sources
	if len(srcs) == 0 {
		for s := range inv {
			srcs = append(srcs, s)
		}
	}
	sort.Strings(srcs)
	dir := filepath.Join(o.Root, card.Dir, results.Kind)
	lines, err := card.ReadKind(dir)
	if err != nil {
		return nil, err
	}
	bySA, byKey, tags := map[string]*card.Line{}, map[string]*card.Line{}, map[string]bool{}
	for _, l := range lines {
		tags[l.Card.Tag] = true
		var f map[string]any
		_ = json.Unmarshal(mustJSON(l.Card.Fields), &f)
		byKey[Key(f)] = l
		var same map[string]string
		_ = json.Unmarshal(l.Card.Fields["same_as"], &same)
		for k, v := range same {
			bySA[k+"="+v] = l
		}
	}
	rep := &Report{Date: o.Date, Aik: o.Version, Sources: srcs, Cite: map[string]int{}}
	misses := map[[2]string]int{}
	ct := &citer{store: o.Store, snaps: map[string]map[string][]float64{}}
	seen := map[string]bool{}
	changed := false
	for _, src := range srcs {
		rows, ok := inv[src]
		if !ok {
			return nil, fmt.Errorf("no inventory %s/%s.jsonl", harvest.InventoryDir, src)
		}
		for _, hr := range rows {
			if hr.Kind != results.Kind {
				continue
			}
			rep.Rows++
			var rr row
			fields := map[string]any{}
			if json.Unmarshal(mustJSON(hr.Fields), &rr) != nil || json.Unmarshal(mustJSON(hr.Fields), &fields) != nil ||
				rr.Model == "" || rr.Score.V == nil || rr.Score.Unit == "" ||
				rr.Source.Src == "" || rr.Source.Loc == "" || len(rr.Setting) == 0 {
				misses[[2]string{NotResultRow, src + ":" + hr.ID}]++
				rep.Unresolved++
				continue
			}
			slug, variant, why := res.Model(rr.Model)
			if why != "" {
				misses[[2]string{why, rr.Model}]++
				rep.Unresolved++
				continue
			}
			if slug == "none" {
				rep.Skipped++
				continue
			}
			bench, version, bad, why := res.Benchmark(fields)
			if why == "" {
				fields["benchmark"], fields["version"] = bench, version
				var metric string
				if metric, bad, why = res.Metric(fields); why == "" {
					fields["metric"] = metric
				}
			}
			if why != "" {
				misses[[2]string{why, bad}]++
				rep.Unresolved++
				continue
			}
			delete(fields, "benchmark_name")
			delete(fields, "metric_name")
			fields["model"] = slug
			if rr.ModelVariant == "" && variant != "" && !effort(rr.Setting, variant) {
				fields["model_variant"] = variant
			}
			key := Key(fields)
			if seen[key] {
				rep.Duplicates++
				continue
			}
			seen[key] = true
			// The snapshot is the source of a leaderboard row. A row whose source is the paper (a row of Papers with
			// Code) is checked against the paper (07.5): the dump that lists its number is not the citation.
			cited := citeUnchecked
			if rr.Source.Kind != "primary" {
				anchor := hr.ID[strings.LastIndexByte(hr.ID, '/')+1:]
				cited = ct.check(hr.Snapshot, rr.Source.Loc, anchor, *rr.Score.V)
			}
			rep.Cite[cited]++
			if cited != citeUnchecked {
				fields["checks"] = map[string]any{"cite": cited}
			}
			same := map[string]string{}
			for k, v := range hr.SameAs {
				same[k] = v
			}
			fields["same_as"] = same
			fields["harvest"] = card.Harvest{Source: src, Snapshot: hr.Snapshot, Row: hr.ID, Date: hr.Date}
			l := matchLine(bySA, byKey, hr.SameAs, key)
			if l == nil {
				tag := newTag(key, tags)
				tags[tag] = true
				fields["tag"], fields["kind"] = tag, results.Kind
				cd := &card.Card{Tag: tag, Kind: results.Kind, Fields: map[string]json.RawMessage{}}
				if err := setAll(cd, fields); err != nil {
					return nil, err
				}
				l = &card.Line{Card: cd, Changed: true}
				lines = append(lines, l)
				byKey[key] = l
				rep.New++
				changed = true
				continue
			}
			fields["tag"], fields["kind"] = l.Card.Tag, results.Kind
			if sameFields(l.Card.Fields, fields) {
				rep.Unchanged++
				continue
			}
			l.Card.Fields = map[string]json.RawMessage{}
			if err := setAll(l.Card, fields); err != nil {
				return nil, err
			}
			l.Changed = true
			rep.Updated++
			changed = true
		}
	}
	for k, n := range misses {
		rep.Missing = append(rep.Missing, Miss{Reason: k[0], Name: k[1], Rows: n})
	}
	sort.Slice(rep.Missing, func(i, j int) bool {
		a, b := rep.Missing[i], rep.Missing[j]
		if a.Rows != b.Rows {
			return a.Rows > b.Rows
		}
		if a.Reason != b.Reason {
			return a.Reason < b.Reason
		}
		return a.Name < b.Name
	})
	if o.DryRun {
		return rep, nil
	}
	if changed {
		if err := card.WriteKind(dir, lines, card.MaxShard); err != nil {
			return nil, err
		}
	}
	return rep, writeReport(o.Root, rep)
}

// effort tells if a variant is the reasoning effort that the setting already has, as "Claude 4.5 Opus (medium)".
func effort(setting map[string]any, variant string) bool {
	cot, _ := setting["cot"].(string)
	return cot != "" && strings.Contains(strings.ToLower(cot), "effort "+strings.ToLower(variant))
}

func matchLine(bySA, byKey map[string]*card.Line, sameAs map[string]string, key string) *card.Line {
	ids := make([]string, 0, len(sameAs))
	for k, v := range sameAs {
		ids = append(ids, k+"="+v)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if l := bySA[id]; l != nil {
			return l
		}
	}
	return byKey[key]
}

// newTag is the provisional tag of a new result card, from its key, until aik mint gives it a permanent one.
func newTag(key string, used map[string]bool) string {
	h := sha256.Sum256([]byte(key))
	x := hex.EncodeToString(h[:])
	for n := 10; ; n++ {
		if t := "NEW-r" + x[:n]; !used[t] {
			return t
		}
	}
}

func setAll(c *card.Card, fields map[string]any) error {
	for k, v := range fields {
		if err := c.Set(k, v); err != nil {
			return fmt.Errorf("%s: %w", k, err)
		}
	}
	return nil
}

// sameFields tells if a card has these fields already.
func sameFields(have map[string]json.RawMessage, want map[string]any) bool {
	a, err := card.Encode(have)
	if err != nil {
		return false
	}
	c := &card.Card{Fields: map[string]json.RawMessage{}}
	if setAll(c, want) != nil {
		return false
	}
	b, err := card.Encode(c.Fields)
	return err == nil && bytes.Equal(a, b)
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func writeReport(root string, rep *Report) error {
	b, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Join(root, ReportDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "results-"+rep.Date+".json"), append(b, '\n'), 0o644)
}
