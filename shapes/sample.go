// SPDX-License-Identifier: Apache-2.0

package shapes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math/big"
	"math/rand/v2"
)

// Samples is the number of random small configs that aik shapes sample adds to the example config (spec 08.6).
const Samples = 2

// rangeOf is the range that a free name is sampled from: the sample range of the file, or [min(2, example), min(example, 8)].
func (f *File) rangeOf(name string) [2]int64 {
	var r [2]int64
	var ex int64
	if v := f.Var(name); v != nil {
		r, ex = v.Sample, v.Example
	} else if a := f.Axis(name); a != nil {
		r, ex = a.Sample, a.Example
	}
	if r != [2]int64{} {
		return r
	}
	return [2]int64{min(2, ex), max(1, min(ex, 8))}
}

// Sample gives n small random configs with which every formula gives whole numbers, distinct from the example and from each other when the ranges allow it.
// The seed makes it the same on every machine; aik shapes sample uses the hash of the node's path.
func (f *File) Sample(seed uint64, n int) ([]map[string]int64, error) {
	free := f.Free()
	rng := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
	key := func(c map[string]int64) string {
		b, _ := json.Marshal(c)
		return string(b)
	}
	seen := map[string]bool{key(f.Example()): true}
	var out []map[string]int64
	var last error
	for try := 0; len(out) < n && try < 20000; try++ {
		c := map[string]int64{}
		for _, name := range free {
			r := f.rangeOf(name)
			c[name] = r[0] + rng.Int64N(r[1]-r[0]+1)
		}
		if err := f.valid("a sampled config", c); err != nil {
			last = err
			continue
		}
		k := key(c)
		if seen[k] && try < 10000 {
			continue
		}
		seen[k] = true
		out = append(out, c)
	}
	if len(out) < n {
		return nil, fmt.Errorf("found %d of %d valid sampled configs; widen the sample ranges in shapes.yaml (last: %v)", len(out), n, last)
	}
	return out, nil
}

// Seed is the seed of Sample for a node: a hash of its path.
func Seed(rel string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(rel))
	return h.Sum64()
}

// FixtureConfig is one config of fixtures/shapes.json with every value that the tests of the implementations check.
type FixtureConfig struct {
	Name string `json:"name"`
	// Kind is "example", "sample" or "named". The tests run forward on the example and the samples, and check param_count on all.
	Kind   string                  `json:"kind"`
	Config map[string]int64        `json:"config"` // every config variable and axis, the derived ones too
	Blocks map[string]FixtureBlock `json:"blocks"`
	Model  map[string]*big.Int     `json:"model"`
}

// FixtureBlock is the expected values of a block at a config.
type FixtureBlock struct {
	Params       *big.Int           `json:"params"`
	FlopsForward *big.Int           `json:"flops_forward"`
	Inputs       map[string][]int64 `json:"inputs,omitempty"`
	Outputs      map[string][]int64 `json:"outputs,omitempty"`
	Internals    map[string][]int64 `json:"internals,omitempty"`
}

// Fixture is the content of fixtures/shapes.json.
type Fixture struct {
	Note    string          `json:"note"`
	Configs []FixtureConfig `json:"configs"`
}

func (f *File) fixtureConfig(name, kind string, in map[string]int64) (FixtureConfig, error) {
	v := f.At(in)
	fc := FixtureConfig{Name: name, Kind: kind, Config: map[string]int64{}, Blocks: map[string]FixtureBlock{}, Model: map[string]*big.Int{}}
	for _, x := range f.Config {
		n, err := v.Int(x.Name)
		if err != nil {
			return fc, err
		}
		fc.Config[x.Name] = n.Int64()
	}
	for _, a := range f.Axes {
		n, err := v.Int(a.Name)
		if err != nil {
			return fc, err
		}
		fc.Config[a.Name] = n.Int64()
	}
	shapes := func(ts []Tensor) (map[string][]int64, error) {
		if len(ts) == 0 {
			return nil, nil
		}
		m := map[string][]int64{}
		for _, t := range ts {
			d, err := v.Dims(t)
			if err != nil {
				return nil, err
			}
			m[t.Name] = d
		}
		return m, nil
	}
	for _, b := range f.Blocks {
		var fb FixtureBlock
		var err error
		if fb.Params, err = v.Int(b.ID + ".params"); err != nil {
			return fc, err
		}
		if fb.FlopsForward, err = v.Int(b.ID + ".flops_forward"); err != nil {
			return fc, err
		}
		if fb.Inputs, err = shapes(b.Inputs); err != nil {
			return fc, err
		}
		if fb.Outputs, err = shapes(b.Outputs); err != nil {
			return fc, err
		}
		if fb.Internals, err = shapes(b.Internals); err != nil {
			return fc, err
		}
		fc.Blocks[b.ID] = fb
	}
	for _, m := range f.Model {
		n, err := v.Int(m.Name)
		if err != nil {
			return fc, err
		}
		fc.Model[m.Name] = n
	}
	return fc, nil
}

// MakeFixture gives the fixture of a node: the example config, Samples sampled configs, and each named config.
func (f *File) MakeFixture(seed uint64) (*Fixture, error) {
	if ps := f.Check(); len(ps) > 0 {
		return nil, fmt.Errorf("%s: %v", f.Path, ps[0])
	}
	fx := &Fixture{Note: "Written by aik shapes sample -w from shapes.yaml; do not edit. The tests of each implementation check shapes, param_count and flops against these values (spec 08.6, rule X09)."}
	add := func(name, kind string, in map[string]int64) error {
		c, err := f.fixtureConfig(name, kind, in)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		fx.Configs = append(fx.Configs, c)
		return nil
	}
	if err := add("example", "example", f.Example()); err != nil {
		return nil, err
	}
	samples, err := f.Sample(seed, Samples)
	if err != nil {
		return nil, err
	}
	for i, s := range samples {
		if err := add(fmt.Sprintf("sample-%d", i+1), "sample", s); err != nil {
			return nil, err
		}
	}
	for _, c := range f.Configs {
		in, _ := f.NamedConfig(c.Name)
		if err := add(c.Name, "named", in); err != nil {
			return nil, err
		}
	}
	return fx, nil
}

// Encode writes the fixture as indented JSON with sorted keys and a final newline.
func (fx *Fixture) Encode() ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(fx); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
