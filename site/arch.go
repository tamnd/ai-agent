// SPDX-License-Identifier: Apache-2.0

package site

import (
	"encoding/json"
	"html/template"
	"math/big"
	"strings"

	"github.com/tamnd/ai-agent/node"
	"github.com/tamnd/ai-agent/shapes"
	"github.com/tamnd/ai-agent/viz"
)

// ArchView is the architecture scene of 12.6 as the page draws it at its first preset. arch.js recomputes it when
// the reader changes a size; with no script, the page shows the first preset.
type ArchView struct {
	Title    string
	Data     template.JS // the compiled scene, for arch.js
	Presets  []string
	Controls []ArchControlView
	Stack    []ArchBlockView
	Totals   []ArchTotalView
}

// ArchControlView is a size that the reader sets.
type ArchControlView struct {
	Name    string
	Symbol  template.HTML
	Meaning string
	Value   int64
	Choices []int64
}

// ArchBlockView is a block of the stack, with the blocks that it is made of.
type ArchBlockView struct {
	ID      string
	Meaning string
	Link    string
	Count   int    // how many times the block appears in its parent, as norm twice in a layer
	Repeat  string // the config variable that counts the block in the model
	Times   string // its value
	Tensors []ArchTensorView
	Params  string
	Flops   string
	Parts   []ArchBlockView
}

// ArchTensorView is a tensor of a block with its shape at the preset.
type ArchTensorView struct {
	Role, Name, Key, Dims string
}

// ArchTotalView is a total of the model.
type ArchTotalView struct {
	Name, Meaning, Value string
	Symbol               template.HTML
}

func (b *builder) archView(n *node.Node) *ArchView {
	d := b.arch[n.Slug]
	f := n.Shapes
	if d == nil || f == nil || len(d.Presets) == 0 {
		return nil
	}
	vals, err := viz.Expected(f, d, d.Presets[0].Values)
	if err != nil {
		return nil
	}
	raw, err := json.Marshal(d)
	if err != nil {
		return nil
	}
	v := &ArchView{Title: d.Title, Data: template.JS(raw)}
	for _, p := range d.Presets {
		v.Presets = append(v.Presets, p.Name)
	}
	for _, c := range d.Controls {
		v.Controls = append(v.Controls, ArchControlView{Name: c.Name, Symbol: b.texHTML(f.Symbol(c.Name)), Meaning: c.Meaning, Value: d.Presets[0].Values[c.Name], Choices: c.Choices})
	}
	blocks := map[string]viz.ArchBlock{}
	for _, bl := range d.Blocks {
		blocks[bl.ID] = bl
	}
	group := func(s string) string {
		x, ok := new(big.Int).SetString(s, 10)
		if !ok {
			return s
		}
		return shapes.Group(x)
	}
	var view func(id string, count, depth int) ArchBlockView
	view = func(id string, count, depth int) ArchBlockView {
		bl := blocks[id]
		bv := ArchBlockView{ID: id, Meaning: bl.Meaning, Link: bl.Link, Count: count, Repeat: bl.Repeat, Params: group(vals.Params[id]), Flops: group(vals.Flops[id])}
		if bl.Repeat != "" {
			bv.Times = group(vals.Sizes[bl.Repeat])
		}
		for _, t := range bl.Tensors {
			key := id + "." + t.Role + "." + t.Name
			var ds []string
			for _, x := range vals.Tensors[key] {
				ds = append(ds, group(x))
			}
			bv.Tensors = append(bv.Tensors, ArchTensorView{Role: t.Role, Name: t.Name, Key: key, Dims: "(" + strings.Join(ds, ", ") + ")"})
		}
		if depth < 3 {
			var order []string
			counts := map[string]int{}
			for _, c := range bl.Of {
				if counts[c] == 0 {
					order = append(order, c)
				}
				counts[c]++
			}
			for _, c := range order {
				bv.Parts = append(bv.Parts, view(c, counts[c], depth+1))
			}
		}
		return bv
	}
	for _, id := range d.Stack {
		v.Stack = append(v.Stack, view(id, 1, 0))
	}
	for _, m := range d.Model {
		v.Totals = append(v.Totals, ArchTotalView{Name: m.Name, Meaning: m.Meaning, Value: group(vals.Model[m.Name]), Symbol: b.texHTML(f.Symbol(m.Name))})
	}
	return v
}
