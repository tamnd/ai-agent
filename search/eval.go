// SPDX-License-Identifier: Apache-2.0

// Package search evaluates the search of the site (spec 17.10). The evaluation set, registers/search-eval.yaml, is a
// list of queries, each with the class of 17.10 and the items it expects. Eval runs the ranking of the browser, the
// script find.js of the site, in goja on the built index, and measures the gates of 17.10.
package search

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/dop251/goja"
	"go.yaml.in/yaml/v3"
)

// File is the evaluation set in the content repository.
const File = "registers/search-eval.yaml"

// The classes of the queries, after the rows of the set of 17.10.
const (
	ClassTitle    = "title"    // the title of a node, with a typo or a change of case: the node at rank 1
	ClassAlias    = "alias"    // an alias of a node, the same
	ClassID       = "id"       // an id of a scheme of 17.4: the item at rank 1
	ClassAcronym  = "acronym"  // one sense: the target at rank 1; several: every sense in the top 5
	ClassVersion  = "version"  // a model or benchmark name with a version or size: the release at rank 1
	ClassQuestion = "question" // a question or task, written and reviewed: an answering node in the top 3
	ClassLog      = "log"      // a query of the logs (17.9), reviewed: the clicked item in the top 3
)

// Classes are the classes that a query can have.
var Classes = []string{ClassTitle, ClassAlias, ClassID, ClassAcronym, ClassVersion, ClassQuestion, ClassLog}

// Query is one query of the set.
type Query struct {
	Q     string `yaml:"q" json:"q"`
	Class string `yaml:"class" json:"class"`
	// Want are the expected items, each a slug, a tag or the path of a page such as /transformer/.
	Want Want   `yaml:"want" json:"want"`
	Note string `yaml:"note,omitempty" json:"note,omitempty"`
	Line int    `yaml:"-" json:"line"`
}

// Want is one item or a list of them.
type Want []string

// UnmarshalYAML reads a string or a list of strings.
func (w *Want) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode {
		*w = Want{n.Value}
		return nil
	}
	var l []string
	if err := n.Decode(&l); err != nil {
		return err
	}
	*w = l
	return nil
}

// Read reads the evaluation set of a content repository and checks each query. It gives nil and no error when there
// is no set.
func Read(root string) ([]Query, error) {
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(File)))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return Parse(b)
}

// Parse reads an evaluation set.
func Parse(b []byte) ([]Query, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", File, err)
	}
	if len(doc.Content) == 0 {
		return nil, nil
	}
	l := doc.Content[0]
	if l.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("%s: the set is a list of queries", File)
	}
	var out []Query
	var errs []string
	seen := map[string]int{}
	for _, n := range l.Content {
		var q Query
		if err := n.Decode(&q); err != nil {
			errs = append(errs, fmt.Sprintf("%s:%d: %v", File, n.Line, err))
			continue
		}
		q.Line = n.Line
		switch {
		case strings.TrimSpace(q.Q) == "":
			errs = append(errs, fmt.Sprintf("%s:%d: the query q is empty", File, n.Line))
		case !slices.Contains(Classes, q.Class):
			errs = append(errs, fmt.Sprintf("%s:%d: the class %q is not one of %s", File, n.Line, q.Class, strings.Join(Classes, ", ")))
		case len(q.Want) == 0 || slices.Contains(q.Want, ""):
			errs = append(errs, fmt.Sprintf("%s:%d: want names no item", File, n.Line))
		}
		k := q.Class + "\x00" + q.Q
		if at, ok := seen[k]; ok {
			errs = append(errs, fmt.Sprintf("%s:%d: the query %q of class %s is also at line %d", File, n.Line, q.Q, q.Class, at))
		}
		seen[k] = n.Line
		out = append(out, q)
	}
	if len(errs) > 0 {
		return nil, errors.New(strings.Join(errs, "\n"))
	}
	return out, nil
}

// Entry is an entry of search.json.
type Entry struct {
	Name string  `json:"name"`
	URL  string  `json:"url"`
	Kind string  `json:"kind"`
	Tag  string  `json:"tag,omitempty"`
	Sc   float64 `json:"score"`
}

// Is tells whether the entry is the item w: its tag, its slug or the path of its page.
func (e Entry) Is(w string) bool {
	return w == e.Tag || w == e.URL || "/"+strings.Trim(w, "/")+"/" == e.URL
}

// Ranker ranks the entries of an index for a query with the script of the browser.
type Ranker struct {
	vm    *goja.Runtime
	rank  goja.Callable
	index goja.Value
	n     int
	all   []Entry
}

// NewRanker loads the script find.js and the index search.json in a new runtime.
func NewRanker(findJS, index []byte) (*Ranker, error) {
	vm := goja.New()
	if _, err := vm.RunScript("find.js", string(findJS)); err != nil {
		return nil, fmt.Errorf("find.js: %w", err)
	}
	f, ok := vm.Get("AlgoFind").(*goja.Object)
	if !ok {
		return nil, errors.New("find.js defines no AlgoFind")
	}
	rank, ok := goja.AssertFunction(f.Get("rank"))
	if !ok {
		return nil, errors.New("find.js defines no AlgoFind.rank")
	}
	parse, _ := goja.AssertFunction(vm.Get("JSON").(*goja.Object).Get("parse"))
	idx, err := parse(goja.Undefined(), vm.ToValue(string(index)))
	if err != nil {
		return nil, fmt.Errorf("search.json: %w", err)
	}
	o, ok := idx.(*goja.Object)
	if !ok || o.ClassName() != "Array" {
		return nil, errors.New("search.json is not a list")
	}
	r := &Ranker{vm: vm, rank: rank, index: idx}
	for _, e := range idx.Export().([]any) {
		r.all = append(r.all, entry(e, 0))
	}
	r.n = len(r.all)
	return r, nil
}

// Len is the number of entries of the index.
func (r *Ranker) Len() int { return r.n }

// Has tells whether an entry of the index is the item w.
func (r *Ranker) Has(w string) bool {
	return slices.ContainsFunc(r.all, func(e Entry) bool { return e.Is(w) })
}

// Rank gives the entries that match q, best first, as the browser shows them.
func (r *Ranker) Rank(q string) ([]Entry, error) {
	v, err := r.rank(goja.Undefined(), r.index, r.vm.ToValue(q))
	if err != nil {
		return nil, err
	}
	l, _ := v.Export().([]any)
	out := make([]Entry, 0, len(l))
	for _, x := range l {
		p, _ := x.([]any)
		if len(p) != 2 {
			return nil, errors.New("AlgoFind.rank gave an item that is not [score, entry]")
		}
		s, _ := p[0].(float64)
		if i, ok := p[0].(int64); ok {
			s = float64(i)
		}
		out = append(out, entry(p[1], s))
	}
	return out, nil
}

func entry(x any, score float64) Entry {
	m, _ := x.(map[string]any)
	str := func(k string) string { s, _ := m[k].(string); return s }
	return Entry{Name: str("n"), URL: str("u"), Kind: str("k"), Tag: str("t"), Sc: score}
}

// Outcome is the result of one query.
type Outcome struct {
	Query
	Ranks   []int    `json:"ranks"`             // the rank of each item of want, 0 when it is not in the results
	Missing []string `json:"missing,omitempty"` // the items of want that are not in the index, which the query skips
	OK      bool     `json:"ok"`
	RR      float64  `json:"rr"`  // the reciprocal rank at 10 of the best item of want
	Top     []Entry  `json:"top"` // the first 5 results
}

// Expect is what a query of a class must do.
func Expect(q Query) string {
	switch q.Class {
	case ClassAcronym:
		if len(q.Want) > 1 {
			return "every sense in the top 5"
		}
		return "at rank 1"
	case ClassQuestion, ClassLog:
		return "in the top 3"
	}
	return "at rank 1"
}

// judge measures one query against its results, for the items of want that are in the index.
func judge(q Query, want Want, res []Entry) Outcome {
	q.Want = want
	o := Outcome{Query: q, Ranks: make([]int, len(want))}
	best := 0
	for i, w := range want {
		for j, e := range res {
			if e.Is(w) {
				o.Ranks[i] = j + 1
				break
			}
		}
		if r := o.Ranks[i]; r > 0 && (best == 0 || r < best) {
			best = r
		}
	}
	if best > 0 && best <= 10 {
		o.RR = 1 / float64(best)
	}
	within := func(n int, all bool) bool {
		for _, r := range o.Ranks {
			ok := r > 0 && r <= n
			if all && !ok {
				return false
			}
			if !all && ok {
				return true
			}
		}
		return all
	}
	switch Expect(q) {
	case "every sense in the top 5":
		o.OK = within(5, true)
	case "in the top 3":
		o.OK = within(3, false)
	default:
		o.OK = within(1, false)
	}
	o.Top = res[:min(5, len(res))]
	return o
}

// Gate is a gate of 17.10 and its measure.
type Gate struct {
	Name    string   `json:"name"`
	Classes []string `json:"classes,omitempty"`
	N       int      `json:"n"` // the number of queries, or of bytes for the transfer
	Got     float64  `json:"got"`
	Want    float64  `json:"want"`
	AtMost  bool     `json:"at_most,omitempty"` // the gate is a ceiling, not a floor
	OK      bool     `json:"ok"`
	Note    string   `json:"note,omitempty"`
}

// The thresholds of 17.10.
const (
	GateExact    = 0.99
	GateAcronym  = 0.95
	GateVersion  = 0.95
	GateQuestion = 0.80
	GateMRR      = 0.85
	GateDrop     = 0.01
	GateTransfer = 350_000 // bytes gzip
)

// NotComputed are the gates of 17.10 that Eval does not measure.
var NotComputed = []string{
	"keystroke latency on the reference phone, shard in cache (at most 30 ms at p95): it needs the phone",
	"first results after the palette opens, cold, on fast 4G (at most 400 ms at p75): it needs the network",
}

// Report is the result of an evaluation.
type Report struct {
	Queries     int       `json:"queries"` // the queries of the set, pending ones too
	Entries     int       `json:"entries"`
	MRR         float64   `json:"mrr"`
	Baseline    *float64  `json:"baseline,omitempty"`
	Gates       []Gate    `json:"gates"`
	Failed      []Outcome `json:"failed"`
	Pending     []Query   `json:"pending"` // the queries none of whose items is in the index; no gate counts them
	NotComputed []string  `json:"not_computed"`
}

// OK tells whether every gate passed.
func (r *Report) OK() bool {
	for _, g := range r.Gates {
		if !g.OK {
			return false
		}
	}
	return true
}

// Eval runs the set on the index with the script find.js. baseline, when not nil, is the MRR at 10 of the last
// release, which the MRR may not drop more than 0.01 below.
func Eval(set []Query, findJS, index []byte, baseline *float64) (*Report, error) {
	r, err := NewRanker(findJS, index)
	if err != nil {
		return nil, err
	}
	rep := &Report{Queries: len(set), Entries: r.Len(), Baseline: baseline, Gates: []Gate{}, Failed: []Outcome{}, Pending: []Query{}, NotComputed: NotComputed}
	ok, n := map[string]int{}, map[string]int{}
	var rr float64
	for _, q := range set {
		res, err := r.Rank(q.Q)
		if err != nil {
			return nil, fmt.Errorf("%s:%d: %q: %w", File, q.Line, q.Q, err)
		}
		var have Want
		for _, w := range q.Want {
			if r.Has(w) {
				have = append(have, w)
			}
		}
		if len(have) == 0 {
			// The item is not built yet, or it moved: the query waits, and the report lists it.
			rep.Pending = append(rep.Pending, q)
			continue
		}
		o := judge(q, have, res)
		if len(have) < len(q.Want) {
			for _, w := range q.Want {
				if !slices.Contains(have, w) {
					o.Missing = append(o.Missing, w)
				}
			}
		}
		n[q.Class]++
		rr += o.RR
		if o.OK {
			ok[q.Class]++
		} else {
			rep.Failed = append(rep.Failed, o)
		}
	}
	rate := func(name string, want float64, classes ...string) {
		g := Gate{Name: name, Classes: classes, Want: want, OK: true}
		k := 0
		for _, c := range classes {
			g.N += n[c]
			k += ok[c]
		}
		if g.N == 0 {
			g.Note = "no queries"
		} else {
			g.Got = float64(k) / float64(g.N)
			g.OK = g.Got >= want
		}
		rep.Gates = append(rep.Gates, g)
	}
	rate("exact title, alias and id queries: success at 1", GateExact, ClassTitle, ClassAlias, ClassID)
	rate("acronym queries: success at 1 for one sense, every sense in the top 5 for several", GateAcronym, ClassAcronym)
	rate("model and benchmark version queries: success at 1", GateVersion, ClassVersion)
	rate("question, task and log queries: success at 3", GateQuestion, ClassQuestion, ClassLog)
	measured := len(set) - len(rep.Pending)
	g := Gate{Name: "the whole set: mean reciprocal rank at 10", N: measured, Want: GateMRR, OK: true}
	if measured == 0 {
		g.Note = "no queries"
	} else {
		rep.MRR = rr / float64(measured)
		g.Got = rep.MRR
		g.OK = g.Got >= GateMRR
		if baseline != nil {
			g.Note = fmt.Sprintf("the last release had %.4f", *baseline)
			if rep.MRR < *baseline-GateDrop {
				g.OK = false
				g.Note += fmt.Sprintf(", a drop of more than %.2f", GateDrop)
			}
		} else {
			g.Note = "no baseline, so the drop from the last release is not measured"
		}
	}
	rep.Gates = append(rep.Gates, g)
	var z bytes.Buffer
	w, _ := gzip.NewWriterLevel(&z, gzip.BestCompression)
	_, _ = w.Write(index)
	_ = w.Close()
	rep.Gates = append(rep.Gates, Gate{Name: "palette transfer before the first result (gzip)", N: z.Len(), Got: float64(z.Len()),
		Want: GateTransfer, AtMost: true, OK: z.Len() <= GateTransfer,
		Note: "the palette loads the whole search.json until the index is sharded (17.5)"})
	return rep, nil
}
