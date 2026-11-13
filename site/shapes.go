// SPDX-License-Identifier: Apache-2.0

package site

import (
	"html/template"
	"strings"

	"github.com/tamnd/ai-agent/node"
	"github.com/tamnd/ai-agent/shapes"
)

// ShapesView is the generated section "Shapes and parameters" of an architecture (spec 06.3, 08.6).
type ShapesView struct {
	Configs []string // the column of each config: example, then the named configs
	Sizes   []ShapeRow
	Blocks  []ShapeBlock
	Totals  []ShapeRow
	Sources []string
	Note    string
	File    string // the link to shapes.yaml
	Arch    *ArchView
}

// ShapeRow is a size or a total of the model.
type ShapeRow struct {
	Symbol  template.HTML
	Name    string
	Meaning string
	Formula template.HTML // empty for a size that a config sets
	Values  []string
}

// ShapeBlock is a block with its shapes and formulas.
type ShapeBlock struct {
	ID      string
	Meaning string
	Shapes  []ShapeIO
	Params  template.HTML
	Value   string // the parameters at the example config
	Flops   template.HTML
}

// ShapeIO is an input, an output or an internal of a block, with its shape in the symbols of the formulas.
type ShapeIO struct {
	Role  string // in, out or internal
	Name  string
	Shape template.HTML
}

func (b *builder) texHTML(tex string) template.HTML {
	if tex == "" {
		return ""
	}
	out, err := b.math.Render(tex, false)
	if err != nil {
		b.report.MathErrors = append(b.report.MathErrors, tex+": "+err.Error())
		return template.HTML(`<code>` + template.HTMLEscapeString(tex) + `</code>`)
	}
	return template.HTML(out)
}

func (b *builder) shapesView(n *node.Node) *ShapesView {
	f := n.Shapes
	if f == nil || len(f.Check()) > 0 {
		return nil
	}
	v := &ShapesView{Configs: []string{"example"}, Note: f.Note, File: b.opt.Repo + "/blob/main/" + n.Rel + "/" + shapes.FileName, Arch: b.archView(n)}
	for _, c := range f.Configs {
		v.Configs = append(v.Configs, c.Name)
	}
	values := func(name string) []string {
		var out []string
		for _, c := range v.Configs {
			_, x, err := f.Ref(name, c)
			if err != nil {
				out = append(out, "?")
				continue
			}
			out = append(out, shapes.Group(x))
		}
		return out
	}
	tex := func(name string) template.HTML {
		t, err := f.TeX(name)
		if err != nil {
			return ""
		}
		return b.texHTML(t)
	}
	for _, x := range f.Config {
		row := ShapeRow{Symbol: b.texHTML(f.Symbol(x.Name)), Name: x.Name, Meaning: x.Meaning, Values: values(x.Name)}
		if x.Formula != "" {
			row.Formula = tex(x.Name)
		}
		v.Sizes = append(v.Sizes, row)
	}
	for _, a := range f.Axes {
		if a.Var == "" {
			v.Sizes = append(v.Sizes, ShapeRow{Symbol: b.texHTML(a.Name), Name: a.Name, Meaning: a.Meaning, Values: values(a.Name)})
		}
	}
	ex := f.At(f.Example())
	for _, bl := range f.Blocks {
		sb := ShapeBlock{ID: bl.ID, Meaning: bl.Meaning, Params: tex(bl.ID + ".params"), Flops: tex(bl.ID + ".flops_forward")}
		for _, io := range []struct {
			role string
			ts   []shapes.Tensor
		}{{"in", bl.Inputs}, {"out", bl.Outputs}, {"internal", bl.Internals}} {
			for _, t := range io.ts {
				st, err := f.ShapeTeX(t.Shape)
				if err != nil {
					st = t.Shape
				}
				sb.Shapes = append(sb.Shapes, ShapeIO{Role: io.role, Name: t.Name, Shape: b.texHTML(st)})
			}
		}
		if x, err := ex.Int(bl.ID + ".params"); err == nil {
			sb.Value = shapes.Group(x)
		}
		v.Blocks = append(v.Blocks, sb)
	}
	for _, m := range f.Model {
		v.Totals = append(v.Totals, ShapeRow{Symbol: b.texHTML(f.Symbol(m.Name)), Name: m.Name, Meaning: m.Meaning, Formula: tex(m.Name), Values: values(m.Name)})
	}
	for _, s := range f.Sources {
		v.Sources = append(v.Sources, s.Work+", "+s.Locator)
	}
	return v
}

// shapeRefs replaces each {shape:...} of the prose, outside code, with the formula as inline math or with the value at a config (rule X20).
func shapeRefs(f *shapes.File, body string) string {
	if !strings.Contains(body, "{shape:") {
		return body
	}
	lines := strings.Split(body, "\n")
	fence := ""
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
			if fence == "" {
				fence = t[:3]
			} else if strings.HasPrefix(t, fence) {
				fence = ""
			}
			continue
		}
		if fence != "" {
			continue
		}
		parts := strings.Split(l, "`")
		for j := 0; j < len(parts); j += 2 { // the even parts are outside code spans
			parts[j] = shapes.RefRE.ReplaceAllStringFunc(parts[j], func(m string) string {
				sm := shapes.RefRE.FindStringSubmatch(m)
				if f == nil {
					return m
				}
				tex, v, err := f.Ref(sm[1], sm[2])
				switch {
				case err != nil:
					return m
				case v != nil:
					return shapes.Group(v)
				}
				return "$" + tex + "$"
			})
		}
		lines[i] = strings.Join(parts, "`")
	}
	return strings.Join(lines, "\n")
}
