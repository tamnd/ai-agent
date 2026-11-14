// SPDX-License-Identifier: Apache-2.0

package node

import (
	"encoding/json"

	"github.com/tamnd/ai-agent/computed"
	"github.com/tamnd/ai-agent/facts"
)

// Items are the cards and the nodes of a kind, as the population of a computed fact (04.9). A card gives its facts with
// the date of its harvest, and its plain fields of text; a node gives the facts of its facts.yaml. A card whose tag is
// the tag of a node is left out, so that an item is counted once.
func (c *Corpus) Items(kind string) []computed.Item {
	var out []computed.Item
	for _, n := range c.Nodes {
		if n.Kind != kind {
			continue
		}
		it := computed.Item{Tag: n.Tag, Key: n.Slug, Name: n.Name, Node: true, Fields: map[string]computed.Value{}}
		for _, f := range n.Facts {
			v := computed.Value{V: f.Value, Unit: f.Unit, AsOf: facts.AsOf(f)}
			if b := facts.Best(&f); b != nil {
				v.Src, v.Loc = b.Src, b.Loc
				if v.AsOf == "" {
					v.AsOf = b.AsOf
				}
			}
			it.Fields[f.ID] = v
		}
		out = append(out, it)
	}
	for _, cd := range c.Cards {
		if cd.Kind != kind || cd.Slug == "" || c.ByTag[cd.Tag] != nil {
			continue
		}
		date := ""
		if len(cd.Harvest) > 0 {
			date = cd.Harvest[0].Date
		}
		name := cd.Name
		if name == "" {
			name = cd.Slug
		}
		it := computed.Item{Tag: cd.Tag, Key: cd.Key(), Name: name, Fields: map[string]computed.Value{}}
		for k, raw := range cd.Fields {
			var v any
			if json.Unmarshal(raw, &v) != nil {
				continue
			}
			switch v.(type) {
			case string, []any:
				it.Fields[k] = computed.Value{V: v, AsOf: date}
			}
		}
		for _, f := range cd.Facts {
			it.Fields[f.ID] = computed.Value{V: f.V, Unit: facts.CardUnits[f.ID], Src: f.Src, Loc: f.Loc, AsOf: date}
		}
		out = append(out, it)
	}
	return out
}

// compute computes the facts of topics/computed.yaml. A definition that cannot be computed keeps its error, which
// rule D08 reports where the prose names the fact.
func (c *Corpus) compute() {
	c.Computed = map[string]*computed.Result{}
	c.ComputedErrs = map[string]error{}
	items := map[string][]computed.Item{}
	for _, d := range c.ComputedDefs {
		if _, ok := items[d.Kind]; !ok {
			items[d.Kind] = c.Items(d.Kind)
		}
		r, err := computed.Compute(d, items[d.Kind])
		if err != nil {
			c.ComputedErrs[d.ID] = err
			continue
		}
		c.Computed[d.ID] = r
	}
}
