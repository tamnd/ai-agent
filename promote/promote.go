// SPDX-License-Identifier: Apache-2.0

// Package promote turns a card into a node directory (spec 04.1): the front matter from the fields of the card, the
// fact file from its facts, and the sections of its kind. The tag does not change, so the URL does not change (16.1).
package promote

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/tamnd/ai-agent/card"
	"github.com/tamnd/ai-agent/facts"
	"github.com/tamnd/ai-agent/node"
)

// Plan is the node that a card becomes.
type Plan struct {
	Card *card.Card
	// Rel is the node directory, relative to the content root: nodes/<top>/<sub>/<slug>.
	Rel string
	// Front is node.md: the front matter and the headings of the sections of the kind.
	Front string
	// Facts are the records of facts.yaml, one per fact of the card.
	Facts []facts.Fact
	// Dropped are the fields of the card that a node of its kind does not have, with the reason.
	Dropped []string
	// Sources are the distinct sources of the facts, for the provenance log.
	Sources []string
}

// meta are the fields of a card that say where it came from or how it was checked, not what the item is. The fact
// sources keep the harvest rows.
var meta = map[string]string{
	"harvest": "the harvest records become the rows of the fact sources",
	"rules":   "the build checks the node again",
}

// Make plans the node of a card in an area. head lists the section headings of the kind, in order.
func Make(c *card.Card, area, level string, head []string) (*Plan, error) {
	if !node.IsKind(c.Kind) {
		return nil, fmt.Errorf("the card %s has the kind %q, which is not a kind of node", c.Tag, c.Kind)
	}
	if c.Tag == "" || strings.HasPrefix(c.Tag, "NEW-") {
		return nil, fmt.Errorf("the card %s has no permanent tag; run aik mint first", c.Key())
	}
	if c.Slug == "" {
		return nil, fmt.Errorf("the card %s has no slug", c.Tag)
	}
	if strings.Count(area, "/") != 1 {
		return nil, fmt.Errorf("the area %q is not of the form top/sub", area)
	}
	p := &Plan{Card: c, Rel: "nodes/" + area + "/" + c.Slug}

	isFact := func(k string) bool {
		return slices.ContainsFunc(c.Facts, func(f card.Fact) bool { return facts.Covers(f.ID, k) })
	}
	// The front matter, in the order of aik new, then the fields of the kind in the order of 04.5, then same_as.
	var fm yaml.Node
	fm.Kind = yaml.MappingNode
	add := func(k string, v any) error {
		var vn yaml.Node
		if err := vn.Encode(v); err != nil {
			return fmt.Errorf("the field %s: %w", k, err)
		}
		fm.Content = append(fm.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: k}, &vn)
		return nil
	}
	name := c.Name
	if name == "" {
		name = strings.ToUpper(c.Slug[:1]) + strings.ReplaceAll(c.Slug[1:], "-", " ")
	}
	summary := c.Summary
	if summary == "" {
		summary = "Write one sentence that says what this is."
	}
	head0 := []struct {
		k string
		v any
	}{{"tag", c.Tag}, {"kind", c.Kind}, {"slug", c.Slug}, {"name", name}}
	for _, x := range head0 {
		if err := add(x.k, x.v); err != nil {
			return nil, err
		}
	}
	if raw, ok := c.Fields["aliases"]; ok {
		var as []string
		if err := json.Unmarshal(raw, &as); err != nil {
			return nil, fmt.Errorf("the aliases of the card %s are not a list of strings", c.Tag)
		}
		if len(as) > 0 {
			if err := add("aliases", as); err != nil {
				return nil, err
			}
		}
	}
	for _, x := range []struct {
		k string
		v any
	}{{"area", area}, {"level", level}, {"summary", summary}} {
		if err := add(x.k, x.v); err != nil {
			return nil, err
		}
	}
	used := map[string]bool{"tag": true, "kind": true, "slug": true, "name": true, "aliases": true, "summary": true, "same_as": true, "area": true, "level": true}
	// A checkpoint card is owned by its family (04.4); the node says so with member_of.
	if c.Owner != "" {
		used["owner"] = true
		switch {
		case c.Kind == "model" && c.Fields["member_of"] == nil:
			if err := add("member_of", c.Owner); err != nil {
				return nil, err
			}
			used["member_of"] = true
		case c.Kind != "model":
			p.Dropped = append(p.Dropped, "owner: a node of the kind "+c.Kind+" has no owner; link "+c.Owner+" in the text")
		}
	}
	// A plain field that is a required fact of the kind (10.11), as the developer of a model, came from the harvested
	// row, so it becomes a fact with that row as its source. With no harvest record it has no source and stays in the
	// front matter.
	var plain []card.Fact
	for _, k := range node.KindFields[c.Kind] {
		raw, ok := c.Fields[k]
		if !ok || used[k] {
			continue
		}
		used[k] = true
		if isFact(k) {
			continue
		}
		var v any
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, fmt.Errorf("the field %s: %w", k, err)
		}
		if v == nil {
			continue
		}
		if slices.Contains(facts.Required[c.Kind], k) && len(c.Harvest) > 0 {
			plain = append(plain, card.Fact{ID: k, V: v, Src: "S:" + c.Harvest[0].Source})
			continue
		}
		if err := add(k, v); err != nil {
			return nil, err
		}
	}
	if raw, ok := c.Fields["same_as"]; ok {
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			return nil, fmt.Errorf("the same_as of the card %s is not an object", c.Tag)
		}
		for k, v := range m {
			if v == nil || v == "" {
				delete(m, k)
			}
		}
		if len(m) > 0 {
			if err := add("same_as", m); err != nil {
				return nil, err
			}
		}
	}
	var rest []string
	for k := range c.Fields {
		if !used[k] {
			rest = append(rest, k)
		}
	}
	sort.Strings(rest)
	for _, k := range rest {
		switch {
		case meta[k] != "":
			p.Dropped = append(p.Dropped, k+": "+meta[k])
		case isFact(k):
			// A fact that is not a field of the kind still moves to facts.yaml.
		default:
			p.Dropped = append(p.Dropped, k+": not a field of a "+c.Kind+" node (04.5)")
		}
	}

	var b bytes.Buffer
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	if err := enc.Encode(&fm); err != nil {
		return nil, err
	}
	var page strings.Builder
	page.WriteString("---\n")
	page.Write(b.Bytes())
	page.WriteString("---\n\n")
	for _, h := range head {
		fmt.Fprintf(&page, "## %s\n\n", h)
	}
	p.Front = strings.TrimRight(page.String(), "\n") + "\n"

	p.Facts, p.Sources = Facts(c, plain...)
	return p, nil
}

// Facts turns the facts of a card into fact records (04.9). A fact whose src is the harvest source of the card came
// from a database row, so it is a secondary source with the row and the date of the harvest; another src names a
// document or a work, and is recorded as primary for the writer to check. A fact that is true only at a date takes the
// date of its harvest as as_of. extra are more facts of the card, after its own.
func Facts(c *card.Card, extra ...card.Fact) ([]facts.Fact, []string) {
	harvest := map[string]card.Harvest{}
	for _, h := range c.Harvest {
		harvest["S:"+h.Source] = h
		harvest[h.Source] = h
	}
	var out []facts.Fact
	seen := map[string]bool{}
	var srcs []string
	for _, f := range append(slices.Clip(c.Facts), extra...) {
		v := value(f.V)
		s := facts.Source{Src: f.Src, Loc: f.Loc, Value: v, Kind: facts.Primary}
		if h, ok := harvest[f.Src]; ok {
			s.Kind, s.Row, s.AsOf = facts.Secondary, h.Row, h.Date
		}
		r := facts.Fact{ID: f.ID, Value: v, Unit: facts.CardUnits[f.ID], Sources: []facts.Source{s}}
		if facts.TimeBound(f.ID) {
			r.AsOf = s.AsOf
		}
		facts.Compute(&r)
		out = append(out, r)
		if f.Src != "" && !seen[f.Src] {
			seen[f.Src] = true
			srcs = append(srcs, f.Src)
		}
	}
	return out, srcs
}

// value gives a JSON number as a float64, as the fact reader does for YAML.
func value(v any) any {
	if n, ok := v.(json.Number); ok {
		f, _ := n.Float64()
		return f
	}
	return v
}

// FactFile is facts.yaml for the records, in the order of the card.
func FactFile(fs []facts.Fact) ([]byte, error) {
	if len(fs) == 0 {
		return nil, nil
	}
	var b bytes.Buffer
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	if err := enc.Encode(fs); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
