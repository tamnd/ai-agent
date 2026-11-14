// SPDX-License-Identifier: Apache-2.0

package viz

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	_ "embed"

	"go.yaml.in/yaml/v3"

	"github.com/tamnd/ai-agent/shapes"
)

// ArchFile is the architecture scene of a node, relative to the node directory (spec 12.2, 12.6).
const ArchFile = "viz/shapes.scene.yaml"

// ArchSpec is viz/shapes.scene.yaml: how the page lays out the blocks of shapes.yaml and which sizes the reader
// changes. The shapes, the parameters and the FLOPs come from shapes.yaml alone, so that the scene, the tests and
// the prose cannot disagree (rule V14).
type ArchSpec struct {
	Title    string            `yaml:"title"`
	Controls []Control         `yaml:"controls"`
	Presets  []string          `yaml:"presets"`          // example and the named configs of shapes.yaml
	Stack    []string          `yaml:"stack"`            // the blocks of the top level, in the order of the data
	Repeat   map[string]string `yaml:"repeat,omitempty"` // a block that the model repeats, with the config variable that counts it
	Links    map[string]string `yaml:"links,omitempty"`  // a block and the node that explains it, as slug or slug@al
}

// Control is a size that the reader sets, with the values that the page suggests; any whole number of at least 1 is allowed.
type Control struct {
	Name    string  `yaml:"name"`
	Choices []int64 `yaml:"choices,flow"`
}

// ParseArch reads viz/shapes.scene.yaml. An unknown key is an error.
func ParseArch(b []byte) (*ArchSpec, error) {
	var s ArchSpec
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(&s); err != nil {
		return nil, err
	}
	return &s, nil
}

// GenArch writes the default scene of a shape contract: every size that a config sets is a control, with the
// values of the example, the named configs and the sample range; the presets are the example and each named
// config; the top level is each block that no other block is made of.
func GenArch(f *shapes.File, title string) *ArchSpec {
	s := &ArchSpec{Title: title, Presets: []string{"example"}}
	for _, c := range f.Configs {
		s.Presets = append(s.Presets, c.Name)
	}
	ex := f.Example()
	for _, name := range f.Free() {
		set := map[int64]bool{ex[name]: true}
		for _, c := range f.Configs {
			if v, ok := c.Values[name]; ok {
				set[v] = true
			}
		}
		lo, hi := sampleRange(f, name)
		if lo > 0 {
			set[lo], set[hi] = true, true
		}
		var cs []int64
		for v := range set {
			cs = append(cs, v)
		}
		slices.Sort(cs)
		s.Controls = append(s.Controls, Control{Name: name, Choices: cs})
	}
	inner := map[string]bool{}
	for _, b := range f.Blocks {
		for _, c := range b.Of {
			inner[c] = true
		}
	}
	for _, b := range f.Blocks {
		if !inner[b.ID] {
			s.Stack = append(s.Stack, b.ID)
		}
	}
	return s
}

func sampleRange(f *shapes.File, name string) (int64, int64) {
	if v := f.Var(name); v != nil {
		return v.Sample[0], v.Sample[1]
	}
	if a := f.Axis(name); a != nil {
		return a.Sample[0], a.Sample[1]
	}
	return 0, 0
}

// Encode writes the scene with a header that says where it comes from.
func (s *ArchSpec) Encode() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("# The architecture scene (spec 12.6), first written by aik viz shapes from shapes.yaml.\n")
	buf.WriteString("# It sets the layout and the controls only; every shape and count comes from shapes.yaml.\n")
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(s); err != nil {
		return nil, err
	}
	return buf.Bytes(), enc.Close()
}

// ArchData is the compiled scene that the page embeds and that arch.js evaluates.
type ArchData struct {
	Slug     string        `json:"slug"`
	Title    string        `json:"title"`
	Controls []ArchControl `json:"controls"`
	Presets  []ArchPreset  `json:"presets"`
	Derived  []ArchFormula `json:"derived"` // the config variables that a formula gives
	Axes     []ArchAxis    `json:"axes"`    // the bound axes
	Blocks   []ArchBlock   `json:"blocks"`
	Stack    []string      `json:"stack"`
	Model    []ArchFormula `json:"model"`
}

// ArchControl is a size that the reader sets.
type ArchControl struct {
	Name    string  `json:"name"`
	Meaning string  `json:"meaning,omitempty"`
	Choices []int64 `json:"choices"`
}

// ArchPreset is a config of shapes.yaml with every size that it sets.
type ArchPreset struct {
	Name   string           `json:"name"`
	Values map[string]int64 `json:"values"`
}

// ArchFormula is a named formula.
type ArchFormula struct {
	Name    string `json:"name"`
	Meaning string `json:"meaning,omitempty"`
	Expr    any    `json:"expr"`
}

// ArchAxis is a bound axis: another name of a config variable.
type ArchAxis struct {
	Name string `json:"name"`
	Var  string `json:"var"`
}

// ArchTensor is an input, an output or an internal of a block.
type ArchTensor struct {
	Role  string `json:"role"`
	Name  string `json:"name"`
	Shape string `json:"shape"`
	Dims  []any  `json:"dims"`
}

// ArchBlock is a block. A null params is the sum over Of; a null flops is that sum, or 0 for a block with no parts.
type ArchBlock struct {
	ID      string       `json:"id"`
	Meaning string       `json:"meaning,omitempty"`
	Of      []string     `json:"of,omitempty"`
	Repeat  string       `json:"repeat,omitempty"`
	Link    string       `json:"link,omitempty"`
	Tensors []ArchTensor `json:"tensors,omitempty"`
	Params  any          `json:"params"`
	Flops   any          `json:"flops"`
}

// Links resolves the slug of a link, such as multi-head-attention or layer-normalization@al, to a URL; ok is false
// when no node or bridge card has it.
type Links func(slug string) (url string, ok bool)

func parse(s string) (any, error) {
	if s == "" {
		return nil, nil
	}
	e, err := shapes.Parse(s)
	if err != nil {
		return nil, err
	}
	return shapes.JSON(e), nil
}

// CompileArch joins a scene and its shape contract into the data of the page. It fails on what the page cannot
// show: a control or preset that the contract does not have, a block of the stack that does not exist.
func CompileArch(slug string, f *shapes.File, s *ArchSpec, links Links) (*ArchData, []string) {
	var fails []string
	bad := func(format string, args ...any) { fails = append(fails, fmt.Sprintf(format, args...)) }
	if ps := f.Check(); len(ps) > 0 {
		return nil, []string{"shapes.yaml does not pass its own check: " + ps[0].Error() + " (X09)"}
	}
	// Every list starts empty, not nil, so that the JSON has [] and arch.js can map over it.
	d := &ArchData{Slug: slug, Title: s.Title, Stack: s.Stack, Controls: []ArchControl{}, Presets: []ArchPreset{},
		Derived: []ArchFormula{}, Axes: []ArchAxis{}, Blocks: []ArchBlock{}, Model: []ArchFormula{}}
	if d.Stack == nil {
		d.Stack = []string{}
	}
	free := f.Free()
	for _, c := range s.Controls {
		if !slices.Contains(free, c.Name) {
			bad("the control %s is not a size that a config of shapes.yaml sets (V10)", c.Name)
			continue
		}
		if len(c.Choices) == 0 {
			bad("the control %s has no choices (V10)", c.Name)
		}
		for _, v := range c.Choices {
			if v < 1 {
				bad("the control %s has the choice %d; a size is at least 1 (V10)", c.Name, v)
			}
		}
		meaning := ""
		if v := f.Var(c.Name); v != nil {
			meaning = v.Meaning
		} else if a := f.Axis(c.Name); a != nil {
			meaning = a.Meaning
		}
		d.Controls = append(d.Controls, ArchControl{Name: c.Name, Meaning: meaning, Choices: c.Choices})
	}
	for _, name := range free {
		if !slices.ContainsFunc(s.Controls, func(c Control) bool { return c.Name == name }) {
			bad("the size %s has no control, so the reader cannot change it (V10)", name)
		}
	}
	for _, p := range s.Presets {
		var in map[string]int64
		if p == "example" {
			in = f.Example()
		} else if c, ok := f.NamedConfig(p); ok {
			in = c
		} else {
			bad("the preset %s is not example or a config of shapes.yaml (V10)", p)
			continue
		}
		d.Presets = append(d.Presets, ArchPreset{Name: p, Values: in})
	}
	if len(s.Presets) < 3 {
		bad("the scene has %d of the 3 presets that it needs (V10)", len(s.Presets))
	}
	for _, v := range f.Config {
		if v.Formula != "" {
			e, err := parse(v.Formula)
			if err != nil {
				bad("config.%s: %v", v.Name, err)
			}
			d.Derived = append(d.Derived, ArchFormula{Name: v.Name, Meaning: v.Meaning, Expr: e})
		}
	}
	for _, a := range f.Axes {
		if a.Var != "" {
			d.Axes = append(d.Axes, ArchAxis{Name: a.Name, Var: a.Var})
		}
	}
	for id := range s.Repeat {
		if f.Block(id) == nil {
			bad("repeat names %s, which is not a block of shapes.yaml", id)
		}
	}
	for id := range s.Links {
		if f.Block(id) == nil {
			bad("links names %s, which is not a block of shapes.yaml", id)
		}
	}
	for _, b := range f.Blocks {
		ab := ArchBlock{ID: b.ID, Meaning: b.Meaning, Of: b.Of, Repeat: s.Repeat[b.ID]}
		if r := ab.Repeat; r != "" && f.Var(r) == nil {
			bad("repeat: the block %s repeats %s times, which is not a config variable", b.ID, r)
		}
		if l := s.Links[b.ID]; l != "" {
			url, ok := "", false
			if links != nil {
				url, ok = links(l)
			}
			if !ok {
				bad("the link of the block %s names %s, which is not a node or a bridge card", b.ID, l)
			}
			ab.Link = url
		}
		if len([]rune(b.Meaning)) > 140 {
			bad("the meaning of the block %s has %d characters; a note has at most 140 (V09)", b.ID, len([]rune(b.Meaning)))
		}
		for _, io := range []struct {
			role string
			ts   []shapes.Tensor
		}{{"in", b.Inputs}, {"out", b.Outputs}, {"internal", b.Internals}} {
			for _, t := range io.ts {
				es, err := shapes.ParseShape(t.Shape)
				if err != nil {
					bad("%s.%s: %v", b.ID, t.Name, err)
					continue
				}
				dims := []any{}
				for _, e := range es {
					dims = append(dims, shapes.JSON(e))
				}
				ab.Tensors = append(ab.Tensors, ArchTensor{Role: io.role, Name: t.Name, Shape: t.Shape, Dims: dims})
			}
		}
		var err error
		if ab.Params, err = parse(b.Params); err != nil {
			bad("%s.params: %v", b.ID, err)
		}
		if ab.Flops, err = parse(b.FlopsForward); err != nil {
			bad("%s.flops_forward: %v", b.ID, err)
		}
		d.Blocks = append(d.Blocks, ab)
	}
	for _, id := range s.Stack {
		if f.Block(id) == nil {
			bad("the stack names %s, which is not a block of shapes.yaml", id)
		}
	}
	if len(s.Stack) == 0 {
		bad("the stack is empty, so the scene shows no block")
	}
	for _, m := range f.Model {
		e, err := parse(m.Formula)
		if err != nil {
			bad("model.%s: %v", m.Name, err)
		}
		d.Model = append(d.Model, ArchFormula{Name: m.Name, Meaning: m.Meaning, Expr: e})
	}
	return d, fails
}

// ArchValues is what arch.js gives for one config, every number as a decimal string: the sizes, the parameters
// and FLOPs of each block, the dims of each tensor, and the totals.
type ArchValues struct {
	Error   string              `json:"error,omitempty"`
	Sizes   map[string]string   `json:"sizes"`
	Params  map[string]string   `json:"params"`
	Flops   map[string]string   `json:"flops"`
	Tensors map[string][]string `json:"tensors"` // block.role.name
	Model   map[string]string   `json:"model"`
}

// Expected computes with the Go evaluator of shapes.yaml what arch.js must give for a config.
func Expected(f *shapes.File, d *ArchData, in map[string]int64) (*ArchValues, error) {
	v := f.At(in)
	out := &ArchValues{Sizes: map[string]string{}, Params: map[string]string{}, Flops: map[string]string{}, Tensors: map[string][]string{}, Model: map[string]string{}}
	get := func(name string) (string, error) {
		x, err := v.Int(name)
		if err != nil {
			return "", err
		}
		return x.String(), nil
	}
	var err error
	for _, x := range f.Config {
		if out.Sizes[x.Name], err = get(x.Name); err != nil {
			return nil, err
		}
	}
	for _, a := range f.Axes {
		if out.Sizes[a.Name], err = get(a.Name); err != nil {
			return nil, err
		}
	}
	for _, b := range f.Blocks {
		if out.Params[b.ID], err = get(b.ID + ".params"); err != nil {
			return nil, err
		}
		if out.Flops[b.ID], err = get(b.ID + ".flops_forward"); err != nil {
			return nil, err
		}
		for _, io := range []struct {
			role string
			ts   []shapes.Tensor
		}{{"in", b.Inputs}, {"out", b.Outputs}, {"internal", b.Internals}} {
			for _, t := range io.ts {
				ds, err := v.Dims(t)
				if err != nil {
					return nil, err
				}
				s := []string{}
				for _, x := range ds {
					s = append(s, big.NewInt(x).String())
				}
				out.Tensors[b.ID+"."+io.role+"."+t.Name] = s
			}
		}
	}
	for _, m := range f.Model {
		if out.Model[m.Name], err = get(m.Name); err != nil {
			return nil, err
		}
	}
	return out, nil
}

//go:embed arch.js
var archJS string

// ArchJS is the evaluator and the view of the architecture scene, the same file in the page and in the check.
func ArchJS() string { return archJS }

//go:embed archcheck.js
var archCheckJS string

// MaxArchBytes is the budget of the compiled scene after gzip, the budget of one scene bundle (12.13, V08).
const MaxArchBytes = 20 << 10

// CheckArch checks a compiled scene (V02, V07, V08, V14): arch.js, run in Node, gives for each preset and for the
// samples of shapes.yaml the same sizes, shapes, parameters, FLOPs and totals as the Go evaluator. Without Node,
// the check is skipped and says so.
func CheckArch(f *shapes.File, d *ArchData, samples []map[string]int64) Report {
	r := Report{Slug: d.Slug}
	raw, err := json.Marshal(d)
	if err != nil {
		r.Fails = append(r.Fails, err.Error())
		return r
	}
	var gz bytes.Buffer
	w := gzip.NewWriter(&gz)
	_, _ = w.Write(raw)
	_ = w.Close()
	if gz.Len() > MaxArchBytes {
		r.Fails = append(r.Fails, fmt.Sprintf("the scene is %d bytes after gzip; the budget is %d (V08)", gz.Len(), MaxArchBytes))
	}
	runs := slices.Clone(d.Presets)
	for i, s := range samples {
		runs = append(runs, ArchPreset{fmt.Sprintf("sample-%d", i+1), s})
	}
	nodeBin, err := exec.LookPath("node")
	if err != nil {
		r.Skipped = "node is not installed"
		return r
	}
	dir, err := os.MkdirTemp("", "aik-arch-")
	if err != nil {
		r.Fails = append(r.Fails, err.Error())
		return r
	}
	defer os.RemoveAll(dir)
	rj, _ := json.Marshal(runs)
	for name, b := range map[string][]byte{"arch.js": []byte(archJS), "check.js": []byte(archCheckJS), "scene.json": raw, "runs.json": rj} {
		if err := os.WriteFile(filepath.Join(dir, name), b, 0o600); err != nil {
			r.Fails = append(r.Fails, err.Error())
			return r
		}
	}
	cmd := exec.Command(nodeBin, "check.js")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		msg := err.Error()
		var ee *exec.ExitError
		if errors.As(err, &ee) && len(ee.Stderr) > 0 {
			msg = strings.TrimSpace(string(ee.Stderr))
		}
		r.Fails = append(r.Fails, "the scene does not run: "+msg)
		return r
	}
	var got []struct {
		Values ArchValues `json:"values"`
		Again  bool       `json:"again"`
	}
	if err := json.Unmarshal(out, &got); err != nil || len(got) != len(runs) {
		r.Fails = append(r.Fails, fmt.Sprintf("the check output does not parse: %v", err))
		return r
	}
	for i, ru := range runs {
		r.Steps = append(r.Steps, 1)
		if !got[i].Again {
			r.Fails = append(r.Fails, fmt.Sprintf("%s: two runs gave different values (V02)", ru.Name))
		}
		want, err := Expected(f, d, ru.Values)
		if err != nil {
			if got[i].Values.Error == "" {
				r.Fails = append(r.Fails, fmt.Sprintf("%s: shapes.yaml gives no value (%v), but the scene shows one", ru.Name, err))
			}
			continue
		}
		if g := got[i].Values; g.Error != "" {
			r.Fails = append(r.Fails, fmt.Sprintf("%s: the scene fails: %s", ru.Name, g.Error))
			continue
		}
		r.Fails = append(r.Fails, diffValues(ru.Name, want, &got[i].Values)...)
	}
	if len(r.Fails) > 8 {
		r.Fails = append(r.Fails[:8], fmt.Sprintf("and %d more", len(r.Fails)-8))
	}
	return r
}

func diffValues(run string, want, got *ArchValues) []string {
	var out []string
	cmp := func(what, rule string, w, g map[string]string) {
		for _, k := range sortedKeys(w) {
			if w[k] != g[k] {
				out = append(out, fmt.Sprintf("%s: the scene shows %s %s = %q; shapes.yaml gives %s (%s)", run, what, k, g[k], w[k], rule))
			}
		}
	}
	cmp("the size", "V14", want.Sizes, got.Sizes)
	cmp("the parameters of", "V14", want.Params, got.Params)
	cmp("the FLOPs of", "V14", want.Flops, got.Flops)
	cmp("the total", "V14", want.Model, got.Model)
	for _, k := range sortedKeys(want.Tensors) {
		if !slices.Equal(want.Tensors[k], got.Tensors[k]) {
			out = append(out, fmt.Sprintf("%s: the scene shows the shape of %s as %v; shapes.yaml gives %v (V07)", run, k, got.Tensors[k], want.Tensors[k]))
		}
	}
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	slices.Sort(ks)
	return ks
}
