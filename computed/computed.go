// SPDX-License-Identifier: Apache-2.0

// Package computed computes the facts that no source states and no writer may write (spec 04.9, rule D08): the first,
// the largest, the most of a population of cards and nodes, with the population and the date. The definitions are in
// topics/computed.yaml; aik facts compute writes the results to build/facts/computed.json, and the prose names one
// with {fact:computed/<id>}.
package computed

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/tamnd/ai-agent/facts"
)

// File is the file of the definitions, relative to the content root.
const File = "topics/computed.yaml"

// Def is one computed fact.
type Def struct {
	ID string `yaml:"id" json:"id"`
	// Kind is the kind of the cards and nodes of the population, such as model.
	Kind string `yaml:"kind" json:"kind"`
	// Where keeps the items whose field has one of the values: a text or a list of texts, compared without case. A field
	// with a list of values, such as country, matches when one of them matches.
	Where map[string]any `yaml:"where,omitempty" json:"where,omitempty"`
	// Measure is the fact that orders the population, such as params.total or released.
	Measure string `yaml:"measure" json:"measure"`
	// Order is max for the largest or the latest, and min for the smallest or the first.
	Order string `yaml:"order" json:"order"`
	// What names the item that the fact picks, as "the model with the most total parameters".
	What string `yaml:"what" json:"what"`
	// Population names the items it is picked from, as "the model cards with open weights".
	Population string `yaml:"population" json:"population"`
}

// Value is one fact of an item.
type Value struct {
	V    any
	Unit string
	Src  string
	Loc  string
	// AsOf is the date the value was read: the date of the harvest of a card, or the as_of of a source of a node.
	AsOf string
}

// Item is a card or a node of the population.
type Item struct {
	Tag  string
	Key  string // the slug of a node or the key of a card
	Name string
	Node bool
	// Fields are the facts and the plain text fields of the item, by id.
	Fields map[string]Value
}

// Result is a computed fact.
type Result struct {
	ID         string `json:"id"`
	Item       string `json:"item"`
	Tag        string `json:"tag"`
	Name       string `json:"name"`
	Node       bool   `json:"node,omitempty"`
	Value      any    `json:"value"`
	Unit       string `json:"unit,omitempty"`
	Src        string `json:"src,omitempty"`
	Loc        string `json:"loc,omitempty"`
	What       string `json:"what"`
	Population string `json:"population"`
	// Count is the number of items of the population that give the measure, and Missing the number that do not.
	Count   int      `json:"count"`
	Missing int      `json:"missing"`
	Ties    []string `json:"ties,omitempty"`
	AsOf    string   `json:"as_of"`
	// Sentence is the sentence that the page prints (06.10).
	Sentence string `json:"sentence"`
}

var idRe = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// Read reads the definitions of a content root. It returns nil and no error when the file does not exist.
func Read(root string) ([]Def, error) {
	b, err := os.ReadFile(filepath.Join(root, File))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var ds []Def
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(&ds); err != nil {
		return nil, fmt.Errorf("%s: %w", File, err)
	}
	seen := map[string]bool{}
	for i, d := range ds {
		if err := d.valid(); err != nil {
			return nil, fmt.Errorf("%s: entry %d: %w", File, i+1, err)
		}
		if seen[d.ID] {
			return nil, fmt.Errorf("%s: the id %s is defined twice", File, d.ID)
		}
		seen[d.ID] = true
	}
	return ds, nil
}

func (d Def) valid() error {
	switch {
	case !idRe.MatchString(d.ID):
		return fmt.Errorf("the id %q is not lower case words joined by hyphens", d.ID)
	case d.Kind == "":
		return fmt.Errorf("%s has no kind", d.ID)
	case d.Measure == "":
		return fmt.Errorf("%s has no measure", d.ID)
	case d.Order != "max" && d.Order != "min":
		return fmt.Errorf("%s has the order %q; it is max or min", d.ID, d.Order)
	case strings.TrimSpace(d.What) == "" || strings.TrimSpace(d.Population) == "":
		return fmt.Errorf("%s needs what and population, the words of its sentence", d.ID)
	}
	for k, v := range d.Where {
		if _, err := texts(v); err != nil {
			return fmt.Errorf("%s: where.%s: %w", d.ID, k, err)
		}
	}
	return nil
}

// texts gives a value as a list of texts: a text, a number or a list of them.
func texts(v any) ([]string, error) {
	switch x := v.(type) {
	case string:
		return []string{x}, nil
	case bool, int, int64, float64, json.Number:
		return []string{fmt.Sprint(x)}, nil
	case []any:
		var out []string
		for _, e := range x {
			t, err := texts(e)
			if err != nil || len(t) != 1 {
				return nil, errors.New("a list holds a value that is not a text or a number")
			}
			out = append(out, t[0])
		}
		return out, nil
	case []string:
		return x, nil
	}
	return nil, fmt.Errorf("the value %v is not a text, a number or a list of them", v)
}

func (d Def) matches(it Item) bool {
	for k, want := range d.Where {
		f, ok := it.Fields[k]
		if !ok {
			return false
		}
		have, err := texts(f.V)
		if err != nil {
			return false
		}
		ws, _ := texts(want)
		if !slices.ContainsFunc(have, func(h string) bool {
			return slices.ContainsFunc(ws, func(w string) bool { return strings.EqualFold(h, w) })
		}) {
			return false
		}
	}
	return true
}

// key is a value of the measure in a form that orders: a number, or a text such as a date.
type key struct {
	num   float64
	text  string
	isNum bool
}

func keyOf(v any) (key, bool) {
	switch x := v.(type) {
	case float64:
		return key{num: x, isNum: true}, true
	case int:
		return key{num: float64(x), isNum: true}, true
	case int64:
		return key{num: float64(x), isNum: true}, true
	case json.Number:
		f, err := x.Float64()
		return key{num: f, isNum: true}, err == nil
	case string:
		if f, err := strconv.ParseFloat(x, 64); err == nil && !dateRe.MatchString(x) {
			return key{num: f, isNum: true}, true
		}
		return key{text: x}, x != ""
	}
	return key{}, false
}

var dateRe = regexp.MustCompile(`^\d{4}(?:-\d{2}(?:-\d{2})?)?$`)

func (a key) cmp(b key) int {
	if a.isNum {
		switch {
		case a.num < b.num:
			return -1
		case a.num > b.num:
			return 1
		}
		return 0
	}
	return strings.Compare(a.text, b.text)
}

// Compute picks the item of the population with the largest or the smallest measure. A tie names every tied item.
func Compute(d Def, items []Item) (*Result, error) {
	r := &Result{ID: d.ID, What: d.What, Population: d.Population}
	type cand struct {
		it Item
		v  Value
		k  key
	}
	var cs []cand
	for _, it := range items {
		if !d.matches(it) {
			continue
		}
		v, ok := it.Fields[d.Measure]
		if !ok || v.V == nil {
			r.Missing++
			continue
		}
		k, ok := keyOf(v.V)
		if !ok {
			r.Missing++
			continue
		}
		if len(cs) > 0 && cs[0].k.isNum != k.isNum {
			return nil, fmt.Errorf("%s: the measure %s is a number for %s and a text for %s", d.ID, d.Measure, cs[0].it.Key, it.Key)
		}
		cs = append(cs, cand{it, v, k})
		if v.AsOf > r.AsOf {
			r.AsOf = v.AsOf
		}
	}
	if len(cs) == 0 {
		return nil, fmt.Errorf("%s: no %s of the population gives %s", d.ID, d.Kind, d.Measure)
	}
	if r.AsOf == "" {
		return nil, fmt.Errorf("%s: no item of the population has the date of its data, so the fact would have no as_of", d.ID)
	}
	sign := 1
	if d.Order == "min" {
		sign = -1
	}
	// The best first; among equal values, by key, so that the result does not depend on the order of the files.
	slices.SortStableFunc(cs, func(a, b cand) int {
		if c := sign * b.k.cmp(a.k); c != 0 {
			return c
		}
		return strings.Compare(a.it.Key, b.it.Key)
	})
	best := cs[0]
	r.Count = len(cs)
	r.Item, r.Tag, r.Name, r.Node = best.it.Key, best.it.Tag, best.it.Name, best.it.Node
	r.Value, r.Unit, r.Src, r.Loc = best.v.V, best.v.Unit, best.v.Src, best.v.Loc
	for _, c := range cs[1:] {
		if c.k.cmp(best.k) != 0 {
			break
		}
		r.Ties = append(r.Ties, c.it.Name)
	}
	v := facts.Format(r.Value, r.Unit)
	if len(r.Ties) > 0 {
		r.Sentence = fmt.Sprintf("Among %s, %s are %s (%s each), as of %s.", d.Population, plural(d.What), list(append([]string{best.it.Name}, r.Ties...)), v, r.AsOf)
	} else {
		r.Sentence = fmt.Sprintf("Among %s, %s is %s (%s), as of %s.", d.Population, d.What, best.it.Name, v, r.AsOf)
	}
	return r, nil
}

// plural makes "the model with the most parameters" into "the models with the most parameters", for a tie.
func plural(what string) string {
	w := strings.Fields(what)
	for i, x := range w {
		if i > 0 && w[i-1] == "the" && !strings.HasSuffix(x, "s") {
			w[i] = x + "s"
			break
		}
	}
	return strings.Join(w, " ")
}

func list(xs []string) string {
	if len(xs) < 2 {
		return strings.Join(xs, "")
	}
	return strings.Join(xs[:len(xs)-1], ", ") + " and " + xs[len(xs)-1]
}
