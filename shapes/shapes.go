// SPDX-License-Identifier: Apache-2.0

// Package shapes reads the shape contract of an architecture node, shapes.yaml (spec 08.6).
//
// The file states the config variables, the axes of the tensors, the blocks with their shapes, parameter counts and FLOP counts, and the totals of the model, as formulas.
// The package evaluates the formulas exactly, checks them (rule X09), samples small configs for the tests of the implementations, and renders the formulas as TeX for the page (rule X20).
package shapes

import (
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

// FileName is the name of the shape contract in a node directory.
const FileName = "shapes.yaml"

// FixtureFile is where aik shapes sample writes the configs and their expected values, relative to the node directory.
const FixtureFile = "fixtures/shapes.json"

// File is a parsed shapes.yaml.
type File struct {
	Path    string // relative to the root, for messages
	Config  []*Var
	Axes    []*Axis
	Blocks  []*Block
	Model   []*Formula
	Sources []Source
	Configs []*Named
	Note    string

	// Problems are the errors of parsing: an unknown key, a formula that does not parse. Check adds the rest.
	Problems []Problem
}

// Var is a config variable: a size that the user sets (with an example) or one that a formula gives.
type Var struct {
	Name    string
	Meaning string
	Symbol  string // TeX for the formulas of the page; the bound axis letter when empty
	Example int64  // 0 when Formula is set
	Formula string
	Sample  [2]int64 // the range of aik shapes sample; zero when not set
	Line    int
	expr    Expr
}

// Axis is a letter of the tensor shapes. A bound axis is another name for a config variable; a free axis, such as the batch, is an input of the forward pass.
type Axis struct {
	Name    string
	Meaning string
	Var     string // the config variable of a bound axis
	Example int64  // of a free axis
	Sample  [2]int64
	Line    int
}

// Tensor is a named tensor of a block with its shape.
type Tensor struct {
	Name  string
	Shape string
	dims  []Expr
}

// Block is a part of the architecture.
type Block struct {
	ID           string
	Meaning      string
	Eq           []string // the equation anchors of node.md, without the eq- prefix
	Inputs       []Tensor
	Outputs      []Tensor
	Internals    []Tensor
	Of           []string // the blocks that it is made of
	Params       string
	FlopsForward string
	Note         string
	Line         int
	params       Expr
	flops        Expr
}

// Formula is a total of the model, such as params or flops_per_token_forward.
type Formula struct {
	Name    string
	Formula string
	Meaning string
	Symbol  string
	Line    int
	expr    Expr
}

// Source is a work and a locator, as in claims.yaml.
type Source struct {
	Work    string `yaml:"work"`
	Locator string `yaml:"locator"`
}

// Named is a named config, such as the base and big models of a paper.
type Named struct {
	Name    string
	Meaning string
	Values  map[string]int64
	Order   []string
	Source  *Source
	Line    int
}

// Problem is one error of a shapes.yaml.
type Problem struct {
	Line int
	Msg  string
}

func (p Problem) Error() string {
	if p.Line > 0 {
		return fmt.Sprintf("line %d: %s", p.Line, p.Msg)
	}
	return p.Msg
}

func (f *File) bad(line int, format string, args ...any) {
	f.Problems = append(f.Problems, Problem{Line: line, Msg: fmt.Sprintf(format, args...)})
}

// Read reads shapes.yaml of a node directory. It returns nil and no error when the node has none.
func Read(dir, rel string) (*File, error) {
	b, err := os.ReadFile(filepath.Join(dir, FileName))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	f, err := ParseFile(b)
	if err != nil {
		return nil, fmt.Errorf("%s/%s: %w", rel, FileName, err)
	}
	f.Path = rel + "/" + FileName
	return f, nil
}

var nameRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// ParseFile parses the bytes of a shapes.yaml. An error is returned only when the YAML does not parse; the other problems are in File.Problems.
func ParseFile(b []byte) (*File, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	f := &File{}
	if len(doc.Content) == 0 {
		f.bad(0, "the file is empty")
		return f, nil
	}
	top := doc.Content[0]
	if top.Kind != yaml.MappingNode {
		f.bad(top.Line, "the file is not a mapping")
		return f, nil
	}
	pairs(top, func(k string, kn, v *yaml.Node) {
		switch k {
		case "config":
			pairs(v, func(name string, kn, v *yaml.Node) { f.Config = append(f.Config, f.parseVar(name, kn, v)) })
		case "axes":
			pairs(v, func(name string, kn, v *yaml.Node) { f.Axes = append(f.Axes, f.parseAxis(name, kn, v)) })
		case "blocks":
			if v.Kind != yaml.SequenceNode {
				f.bad(v.Line, "blocks is not a list")
				return
			}
			for _, item := range v.Content {
				f.Blocks = append(f.Blocks, f.parseBlock(item))
			}
		case "model":
			pairs(v, func(name string, kn, v *yaml.Node) {
				switch name {
				case "sources":
					if err := v.Decode(&f.Sources); err != nil {
						f.bad(v.Line, "model.sources: %v", err)
					}
				case "note":
					f.Note = v.Value
				default:
					f.Model = append(f.Model, f.parseFormula(name, kn, v))
				}
			})
		case "configs":
			pairs(v, func(name string, kn, v *yaml.Node) { f.Configs = append(f.Configs, f.parseNamed(name, kn, v)) })
		default:
			f.bad(kn.Line, "unknown key %q; the keys are config, axes, blocks, model and configs", k)
		}
	})
	return f, nil
}

// pairs calls fn for each key and value of a mapping, in order.
func pairs(n *yaml.Node, fn func(k string, kn, v *yaml.Node)) {
	if n.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		fn(n.Content[i].Value, n.Content[i], n.Content[i+1])
	}
}

func (f *File) parseExpr(line int, what, s string) Expr {
	e, err := Parse(s)
	if err != nil {
		f.bad(line, "%s: %v", what, err)
		return nil
	}
	return e
}

func (f *File) sample(line int, what string, v *yaml.Node) [2]int64 {
	var r []int64
	if err := v.Decode(&r); err != nil || len(r) != 2 || r[0] < 1 || r[0] > r[1] {
		f.bad(line, "%s: sample is a range [low, high] of whole numbers with 1 <= low <= high", what)
		return [2]int64{}
	}
	return [2]int64{r[0], r[1]}
}

func (f *File) parseVar(name string, kn, v *yaml.Node) *Var {
	x := &Var{Name: name, Line: kn.Line}
	if !nameRE.MatchString(name) {
		f.bad(kn.Line, "the config name %q is not a name of letters, digits and _", name)
	}
	if v.Kind == yaml.ScalarNode {
		if err := v.Decode(&x.Example); err != nil {
			x.Formula = v.Value
		}
	} else {
		pairs(v, func(k string, kn, v *yaml.Node) {
			var err error
			switch k {
			case "meaning":
				x.Meaning = v.Value
			case "symbol":
				x.Symbol = v.Value
			case "example":
				err = v.Decode(&x.Example)
			case "formula":
				x.Formula = v.Value
			case "sample":
				x.Sample = f.sample(kn.Line, "config."+name, v)
			default:
				f.bad(kn.Line, "config.%s: unknown key %q; the keys are meaning, symbol, example, formula and sample", name, k)
			}
			if err != nil {
				f.bad(kn.Line, "config.%s.%s: %v", name, k, err)
			}
		})
	}
	switch {
	case x.Formula != "" && x.Example != 0:
		f.bad(x.Line, "config.%s has both an example and a formula; a variable is set or computed, not both", name)
	case x.Formula == "" && x.Example < 1:
		f.bad(x.Line, "config.%s needs an example of at least 1, or a formula", name)
	}
	if x.Formula != "" {
		x.expr = f.parseExpr(x.Line, "config."+name, x.Formula)
	}
	return x
}

func (f *File) parseAxis(name string, kn, v *yaml.Node) *Axis {
	a := &Axis{Name: name, Line: kn.Line}
	if !nameRE.MatchString(name) {
		f.bad(kn.Line, "the axis %q is not a name", name)
	}
	if v.Kind == yaml.ScalarNode {
		a.Meaning = v.Value
	} else {
		pairs(v, func(k string, kn, v *yaml.Node) {
			var err error
			switch k {
			case "meaning":
				a.Meaning = v.Value
			case "var":
				a.Var = v.Value
			case "example":
				err = v.Decode(&a.Example)
			case "sample":
				a.Sample = f.sample(kn.Line, "axes."+name, v)
			default:
				f.bad(kn.Line, "axes.%s: unknown key %q; the keys are meaning, var, example and sample", name, k)
			}
			if err != nil {
				f.bad(kn.Line, "axes.%s.%s: %v", name, k, err)
			}
		})
	}
	return a
}

func (f *File) tensors(id, part string, v *yaml.Node) []Tensor {
	var out []Tensor
	pairs(v, func(k string, kn, v *yaml.Node) {
		t := Tensor{Name: k, Shape: v.Value}
		d, err := ParseShape(v.Value)
		if err != nil {
			f.bad(kn.Line, "blocks.%s.%s.%s: %v", id, part, k, err)
		}
		t.dims = d
		out = append(out, t)
	})
	return out
}

func (f *File) parseBlock(n *yaml.Node) *Block {
	b := &Block{Line: n.Line}
	if n.Kind != yaml.MappingNode {
		f.bad(n.Line, "a block is a mapping with an id")
		return b
	}
	pairs(n, func(k string, kn, v *yaml.Node) {
		if k == "id" {
			b.ID = v.Value
		}
	})
	pairs(n, func(k string, kn, v *yaml.Node) {
		var err error
		switch k {
		case "id":
		case "meaning":
			b.Meaning = v.Value
		case "note":
			b.Note = v.Value
		case "eq":
			err = v.Decode(&b.Eq)
		case "of":
			err = v.Decode(&b.Of)
		case "inputs":
			b.Inputs = f.tensors(b.ID, k, v)
		case "outputs":
			b.Outputs = f.tensors(b.ID, k, v)
		case "internals":
			b.Internals = f.tensors(b.ID, k, v)
		case "params":
			b.Params = v.Value
			b.params = f.parseExpr(kn.Line, "blocks."+b.ID+".params", v.Value)
		case "flops_forward":
			b.FlopsForward = v.Value
			b.flops = f.parseExpr(kn.Line, "blocks."+b.ID+".flops_forward", v.Value)
		default:
			f.bad(kn.Line, "blocks.%s: unknown key %q; the keys are id, meaning, eq, of, inputs, outputs, internals, params, flops_forward and note", b.ID, k)
		}
		if err != nil {
			f.bad(kn.Line, "blocks.%s.%s: %v", b.ID, k, err)
		}
	})
	if !nameRE.MatchString(b.ID) {
		f.bad(b.Line, "the block id %q is not a name of letters, digits and _", b.ID)
	}
	return b
}

func (f *File) parseFormula(name string, kn, v *yaml.Node) *Formula {
	x := &Formula{Name: name, Line: kn.Line}
	if !nameRE.MatchString(name) {
		f.bad(kn.Line, "the model formula %q is not a name", name)
	}
	if v.Kind == yaml.ScalarNode {
		x.Formula = v.Value
	} else {
		pairs(v, func(k string, kn, v *yaml.Node) {
			switch k {
			case "formula":
				x.Formula = v.Value
			case "meaning":
				x.Meaning = v.Value
			case "symbol":
				x.Symbol = v.Value
			default:
				f.bad(kn.Line, "model.%s: unknown key %q; the keys are formula, meaning and symbol", name, k)
			}
		})
	}
	x.expr = f.parseExpr(x.Line, "model."+name, x.Formula)
	return x
}

func (f *File) parseNamed(name string, kn, v *yaml.Node) *Named {
	c := &Named{Name: name, Line: kn.Line, Values: map[string]int64{}}
	pairs(v, func(k string, kn, v *yaml.Node) {
		switch k {
		case "meaning":
			c.Meaning = v.Value
		case "source":
			c.Source = &Source{}
			if err := v.Decode(c.Source); err != nil {
				f.bad(kn.Line, "configs.%s.source: %v", name, err)
			}
		default:
			var x int64
			if err := v.Decode(&x); err != nil {
				f.bad(kn.Line, "configs.%s.%s: %v", name, k, err)
				return
			}
			c.Values[k] = x
			c.Order = append(c.Order, k)
		}
	})
	return c
}

// Var finds a config variable.
func (f *File) Var(name string) *Var {
	for _, v := range f.Config {
		if v.Name == name {
			return v
		}
	}
	return nil
}

// Axis finds an axis.
func (f *File) Axis(name string) *Axis {
	for _, a := range f.Axes {
		if a.Name == name {
			return a
		}
	}
	return nil
}

// Block finds a block.
func (f *File) Block(id string) *Block {
	for _, b := range f.Blocks {
		if b.ID == id {
			return b
		}
	}
	return nil
}

// Formula finds a total of the model.
func (f *File) Formula(name string) *Formula {
	for _, m := range f.Model {
		if m.Name == name {
			return m
		}
	}
	return nil
}

// Named finds a named config.
func (f *File) Named(name string) *Named {
	for _, c := range f.Configs {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// bind sets Axis.Var for an axis whose meaning is the name of a config variable, as in "H: n_heads".
func (f *File) bind() {
	for _, a := range f.Axes {
		if a.Var == "" && f.Var(a.Meaning) != nil {
			a.Var = a.Meaning
		}
	}
}

// Free lists the names that a config sets: the config variables with an example and the free axes.
func (f *File) Free() []string {
	f.bind()
	var out []string
	for _, v := range f.Config {
		if v.Formula == "" {
			out = append(out, v.Name)
		}
	}
	for _, a := range f.Axes {
		if a.Var == "" {
			out = append(out, a.Name)
		}
	}
	return out
}

// Example is the example config: the example of each config variable and free axis.
func (f *File) Example() map[string]int64 {
	f.bind()
	out := map[string]int64{}
	for _, v := range f.Config {
		if v.Formula == "" {
			out[v.Name] = v.Example
		}
	}
	for _, a := range f.Axes {
		if a.Var == "" {
			out[a.Name] = a.Example
		}
	}
	return out
}

// Config gives the values of a named config, with the example for each value that it does not set.
func (f *File) NamedConfig(name string) (map[string]int64, bool) {
	c := f.Named(name)
	if c == nil {
		return nil, false
	}
	out := f.Example()
	for k, v := range c.Values {
		out[k] = v
	}
	return out, true
}

// Values evaluates the names of a File at one config.
type Values struct {
	f    *File
	in   map[string]int64
	memo map[string]*big.Rat
	busy map[string]bool
}

// At gives the values of the names at a config. The config sets each free name (Free).
func (f *File) At(in map[string]int64) *Values {
	f.bind()
	return &Values{f: f, in: in, memo: map[string]*big.Rat{}, busy: map[string]bool{}}
}

// Get gives the exact value of a name.
func (v *Values) Get(name string) (*big.Rat, error) {
	if x, ok := v.memo[name]; ok {
		return x, nil
	}
	if v.busy[name] {
		return nil, fmt.Errorf("%s depends on itself", name)
	}
	v.busy[name] = true
	defer delete(v.busy, name)
	x, err := v.get(name)
	if err != nil {
		return nil, err
	}
	v.memo[name] = x
	return x, nil
}

func (v *Values) env(name string) (*big.Rat, error) { return v.Get(name) }

func (v *Values) input(name string) (*big.Rat, error) {
	x, ok := v.in[name]
	if !ok {
		return nil, fmt.Errorf("the config does not set %s", name)
	}
	return new(big.Rat).SetInt64(x), nil
}

func (v *Values) get(name string) (*big.Rat, error) {
	f := v.f
	if id, part, ok := strings.Cut(name, "."); ok {
		b := f.Block(id)
		if b == nil {
			return nil, fmt.Errorf("%s names no block", name)
		}
		var e Expr
		switch part {
		case "params":
			e = b.params
		case "flops_forward":
			e = b.flops
		default:
			return nil, fmt.Errorf("%s: a block has params and flops_forward", name)
		}
		if e != nil {
			return e.eval(v.env)
		}
		if len(b.Of) == 0 {
			if part == "flops_forward" {
				return new(big.Rat), nil // a block with no flops_forward has no multiply-adds; its minor FLOPs are not counted (08.6)
			}
			return nil, fmt.Errorf("the block %s has no %s", id, part)
		}
		sum := new(big.Rat)
		for _, c := range b.Of {
			x, err := v.Get(c + "." + part)
			if err != nil {
				return nil, err
			}
			sum.Add(sum, x)
		}
		return sum, nil
	}
	if x := f.Var(name); x != nil {
		if x.Formula == "" {
			return v.input(name)
		}
		if x.expr == nil {
			return nil, fmt.Errorf("config.%s does not parse", name)
		}
		return x.expr.eval(v.env)
	}
	if a := f.Axis(name); a != nil {
		if a.Var != "" {
			return v.Get(a.Var)
		}
		return v.input(name)
	}
	if m := f.Formula(name); m != nil {
		if m.expr == nil {
			return nil, fmt.Errorf("model.%s does not parse", name)
		}
		return m.expr.eval(v.env)
	}
	return nil, fmt.Errorf("%s is not a config variable, an axis, a model formula or block.params", name)
}

// Int gives the value of a name, which must be a whole number.
func (v *Values) Int(name string) (*big.Int, error) {
	x, err := v.Get(name)
	if err != nil {
		return nil, err
	}
	if !x.IsInt() {
		return nil, fmt.Errorf("%s is %s, not a whole number", name, x.RatString())
	}
	return new(big.Int).Set(x.Num()), nil
}

// Dims gives the sizes of a tensor shape at the config.
func (v *Values) Dims(t Tensor) ([]int64, error) {
	out := []int64{}
	for i, d := range t.dims {
		x, err := d.eval(v.env)
		if err != nil {
			return nil, fmt.Errorf("%s axis %d: %w", t.Name, i+1, err)
		}
		if !x.IsInt() || x.Sign() <= 0 || !x.Num().IsInt64() {
			return nil, fmt.Errorf("%s %s: axis %d is %s, not a whole number of at least 1", t.Name, t.Shape, i+1, x.RatString())
		}
		out = append(out, x.Num().Int64())
	}
	return out, nil
}

// known tells whether a name of a formula is defined.
func (f *File) known(name string) bool {
	if id, part, ok := strings.Cut(name, "."); ok {
		return f.Block(id) != nil && (part == "params" || part == "flops_forward")
	}
	return f.Var(name) != nil || f.Axis(name) != nil || f.Formula(name) != nil
}

// Exprs lists each formula of the file with where it is, for the checks.
func (f *File) exprs() []struct {
	where string
	line  int
	e     Expr
} {
	type item = struct {
		where string
		line  int
		e     Expr
	}
	var out []item
	for _, v := range f.Config {
		if v.expr != nil {
			out = append(out, item{"config." + v.Name, v.Line, v.expr})
		}
	}
	for _, b := range f.Blocks {
		if b.params != nil {
			out = append(out, item{"blocks." + b.ID + ".params", b.Line, b.params})
		}
		if b.flops != nil {
			out = append(out, item{"blocks." + b.ID + ".flops_forward", b.Line, b.flops})
		}
		for _, ts := range [][]Tensor{b.Inputs, b.Outputs, b.Internals} {
			for _, t := range ts {
				for _, d := range t.dims {
					out = append(out, item{"blocks." + b.ID + " " + t.Name, b.Line, d})
				}
			}
		}
	}
	for _, m := range f.Model {
		if m.expr != nil {
			out = append(out, item{"model." + m.Name, m.Line, m.expr})
		}
	}
	return out
}

// Check runs the static checks of rule X09 on the file: every name is defined, nothing depends on itself, the sizes are whole numbers at the example and the named configs, and the parts of a block add up to the block.
// It returns the problems of parsing and of the checks.
func (f *File) Check() []Problem {
	f.bind()
	out := slices.Clone(f.Problems)
	bad := func(line int, format string, args ...any) {
		out = append(out, Problem{Line: line, Msg: fmt.Sprintf(format, args...)})
	}
	seen := map[string]int{}
	name := func(n string, line int) {
		if l, ok := seen[n]; ok {
			bad(line, "the name %s is defined twice (also on line %d)", n, l)
		}
		seen[n] = line
	}
	for _, v := range f.Config {
		name(v.Name, v.Line)
	}
	for _, a := range f.Axes {
		name(a.Name, a.Line)
		switch {
		case a.Var != "" && f.Var(a.Var) == nil:
			bad(a.Line, "the axis %s names the config variable %s, which is not in config", a.Name, a.Var)
		case a.Var == "" && a.Example < 1:
			bad(a.Line, "the free axis %s (%s) needs an example of at least 1: write %s: {meaning: %q, example: 2}", a.Name, a.Meaning, a.Name, a.Meaning)
		}
	}
	for _, m := range f.Model {
		name(m.Name, m.Line)
	}
	ids := map[string]bool{}
	for _, b := range f.Blocks {
		if ids[b.ID] {
			bad(b.Line, "the block id %s is used twice", b.ID)
		}
		ids[b.ID] = true
		for _, c := range b.Of {
			if f.Block(c) == nil {
				bad(b.Line, "the block %s is made of %s, which is not a block", b.ID, c)
			}
		}
		if b.params == nil && len(b.Of) == 0 && b.Params == "" {
			bad(b.Line, "the block %s has no params; write a formula, or 0 for a block with no parameters", b.ID)
		}
	}
	if f.Formula("params") == nil {
		bad(0, "model has no params formula")
	}
	for _, x := range f.exprs() {
		for _, n := range Names(x.e) {
			if !f.known(n) {
				bad(x.line, "%s uses %s, which is not a config variable, an axis, a model formula or a block's params or flops_forward", x.where, n)
			}
		}
	}
	for _, s := range f.Sources {
		if s.Work == "" || s.Locator == "" {
			bad(0, "each source of the model needs a work and a locator")
		}
	}
	free := f.Free()
	for _, c := range f.Configs {
		for _, k := range c.Order {
			if !slices.Contains(free, k) {
				bad(c.Line, "the config %s sets %s, which is not a config variable with an example or a free axis", c.Name, k)
			} else if c.Values[k] < 1 {
				bad(c.Line, "the config %s sets %s to %d; a size is at least 1", c.Name, k, c.Values[k])
			}
		}
		if c.Source != nil && (c.Source.Work == "" || c.Source.Locator == "") {
			bad(c.Line, "the source of the config %s needs a work and a locator", c.Name)
		}
	}
	if len(out) > 0 {
		return out
	}
	if err := f.valid("the example config", f.Example()); err != nil {
		bad(0, "%v", err)
	}
	for _, c := range f.Configs {
		in, _ := f.NamedConfig(c.Name)
		if err := f.valid("the config "+c.Name, in); err != nil {
			bad(c.Line, "%v", err)
		}
	}
	return out
}

// valid evaluates every name of the file at a config and checks that sizes, counts and shapes are whole numbers and that the parts of each block add up to it.
func (f *File) valid(what string, in map[string]int64) error {
	v := f.At(in)
	for _, x := range f.Config {
		n, err := v.Int(x.Name)
		if err != nil {
			return fmt.Errorf("%s: %w", what, err)
		}
		if n.Sign() <= 0 {
			return fmt.Errorf("%s: %s is %s; a size is at least 1", what, x.Name, n)
		}
	}
	for _, b := range f.Blocks {
		for _, part := range []string{"params", "flops_forward"} {
			n, err := v.Int(b.ID + "." + part)
			if err != nil {
				return fmt.Errorf("%s: %w", what, err)
			}
			if n.Sign() < 0 {
				return fmt.Errorf("%s: %s.%s is %s, below 0", what, b.ID, part, n)
			}
		}
		if len(b.Of) > 0 {
			for _, part := range []string{"params", "flops_forward"} {
				own := b.params
				if part == "flops_forward" {
					own = b.flops
				}
				if own == nil {
					continue
				}
				sum := new(big.Rat)
				for _, c := range b.Of {
					x, err := v.Get(c + "." + part)
					if err != nil {
						return fmt.Errorf("%s: %s.%s: %w", what, b.ID, part, err)
					}
					sum.Add(sum, x)
				}
				x, _ := v.Get(b.ID + "." + part)
				if x.Cmp(sum) != 0 {
					return fmt.Errorf("%s: %s.%s is %s, but its parts %s add up to %s", what, b.ID, part, x.RatString(), strings.Join(b.Of, ", "), sum.RatString())
				}
			}
		}
		for _, ts := range [][]Tensor{b.Inputs, b.Outputs, b.Internals} {
			for _, t := range ts {
				if _, err := v.Dims(t); err != nil {
					return fmt.Errorf("%s: block %s: %w", what, b.ID, err)
				}
			}
		}
	}
	for _, m := range f.Model {
		if _, err := v.Int(m.Name); err != nil {
			return fmt.Errorf("%s: %w", what, err)
		}
	}
	return nil
}
