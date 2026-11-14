// SPDX-License-Identifier: Apache-2.0

package schema

import (
	"reflect"
	"sort"

	"github.com/tamnd/ai-agent/claims"
	"github.com/tamnd/ai-agent/node"
)

// Base is the URL where the site publishes the schemas.
const Base = "https://ai.tamnd.com/data/schema/"

// The patterns of the identifiers (spec 04.1, 04.2).
const (
	TagPattern  = `^([0-9ABCDEFGHJKMNPQRSTVWXYZ]{6}|NEW-[a-z0-9-]+)$`
	SlugPattern = `^[a-z0-9][a-z0-9-]*$`
	AreaPattern = `^[a-z0-9][a-z0-9-]*(/[a-z0-9][a-z0-9-]*)*$`
)

func head(name, title, desc string, s map[string]any) map[string]any {
	s["$schema"] = Draft
	s["$id"] = Base + name
	s["title"] = title
	s["description"] = desc
	return s
}

func set(s map[string]any, prop, key string, v any) {
	p := s["properties"].(map[string]any)[prop].(map[string]any)
	p[key] = v
}

// Node is the schema of the front matter of node.md (spec 04.4, 04.5). The fields of a kind are allowed only on a node
// of that kind.
func Node() map[string]any {
	var kinds []string
	for _, k := range node.KindTable {
		if !k.CardOnly() {
			kinds = append(kinds, k.ID)
		}
	}
	g := Gen{
		Tag:   "yaml",
		Enums: map[string][]string{"Front.kind": kinds, "Front.level": node.Levels},
		Fields: map[string]map[string]any{
			// The Lean binding of 11.4: the module, the statement, the steps file, and whether a proof is expected.
			"Front.lean": {"type": "object", "properties": map[string]any{
				"module":    map[string]any{"type": "string"},
				"statement": map[string]any{"type": "string"},
				"steps":     map[string]any{"type": "string"},
				"expected":  map[string]any{"type": []string{"boolean", "string"}},
			}, "additionalProperties": map[string]any{"type": "string"}},
		},
		Types: map[string]map[string]any{
			// A solves entry is the slug of a problem or a mapping with the problem (node.Solve.UnmarshalYAML).
			"Solve": {"oneOf": []any{
				map[string]any{"type": "string", "pattern": SlugPattern},
				(&Gen{Tag: "yaml"}).Of(reflect.TypeFor[struct {
					Problem  string         `yaml:"problem"`
					Requires map[string]any `yaml:"requires,omitempty"`
					Time     string         `yaml:"time,omitempty"`
					Note     string         `yaml:"note,omitempty"`
				}]()),
			}},
		},
	}
	s := g.Of(reflect.TypeFor[node.Front]())
	set(s, "tag", "pattern", TagPattern)
	set(s, "slug", "pattern", SlugPattern)
	set(s, "area", "pattern", AreaPattern)
	delete(s, "additionalProperties")
	s["unevaluatedProperties"] = false
	ids := make([]string, 0, len(node.KindFields))
	for k := range node.KindFields {
		ids = append(ids, k)
	}
	sort.Strings(ids)
	var all []any
	for _, k := range ids {
		props := map[string]any{}
		for _, f := range node.KindFields[k] {
			props[f] = map[string]any{}
		}
		all = append(all, map[string]any{
			"if":   map[string]any{"properties": map[string]any{"kind": map[string]any{"const": k}}, "required": []string{"kind"}},
			"then": map[string]any{"properties": props},
		})
	}
	s["allOf"] = all
	return head("node.v1.json", "Front matter of node.md",
		"The YAML front matter of a node (spec 04.4). The fields of a kind (04.5) are allowed only on a node of that kind; their values are checked by aik check, and a number that changes is a fact of facts.yaml (04.9), never a field.", s)
}

// Claims is the schema of claims.yaml (spec 07.3): a list of claim records. The labels are computed and never written
// (rule T05), so they are not in it.
func Claims() map[string]any {
	g := Gen{Tag: "yaml", Enums: map[string][]string{
		"Claim.type":        claims.Types,
		"Claim.small_scale": {"yes", "no", "partial"},
		"Evidence.role":     claims.Roles,
		"Evidence.stance":   claims.Stances,
		"Evidence.reuses":   claims.Reuses,
	}}
	s := map[string]any{"type": "array", "items": g.Of(reflect.TypeFor[claims.Claim]())}
	return head("claims.v1.json", "claims.yaml",
		"The claim records of a node (spec 07.3). The evidence labels are computed by the build from the evidence and the checks (07.4); nobody writes them.", s)
}
