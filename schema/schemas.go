// SPDX-License-Identifier: Apache-2.0

package schema

import (
	"fmt"
	"reflect"
	"sort"

	"github.com/tamnd/ai-agent/card"
	"github.com/tamnd/ai-agent/claims"
	"github.com/tamnd/ai-agent/exp"
	"github.com/tamnd/ai-agent/node"
)

// Base is the URL where the site publishes the schemas.
const Base = "https://ai.tamnd.com/data/schema/"

// The patterns of the identifiers (spec 04.1, 04.2).
const (
	TagPattern  = `^([0-9ABCDEFGHJKMNPQRSTVWXYZ]{6}|NEW-[a-z0-9-]+)$`
	SlugPattern = `^[a-z0-9][a-z0-9-]*$`
	AreaPattern = `^[a-z0-9][a-z0-9-]*(/[a-z0-9][a-z0-9-]*)*$`
	// KeyPattern is the key of a card: a slug, or the slug of its owner and its own slug (04.4).
	KeyPattern  = `^[a-z0-9][a-z0-9-]*(/[a-z0-9][a-z0-9.-]*)?$`
	DatePattern = `^[0-9]{4}(-[0-9]{2}(-[0-9]{2})?)?$`
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

// Card is the schema of one line of cards/<kind>/<shard>.jsonl (spec 04.6). The fields of the kind are open: a fact
// field is an object with v and src, a group of facts is an object of fact fields, and anything else is a plain field.
func Card() map[string]any {
	str := map[string]any{"type": "string"}
	harvest := (&Gen{Tag: "json"}).Of(reflect.TypeFor[card.Harvest]())
	harvest["properties"].(map[string]any)["date"] = map[string]any{"type": "string", "pattern": DatePattern}
	notFact := map[string]any{"not": map[string]any{"anyOf": []any{
		map[string]any{"required": []string{"v"}}, map[string]any{"required": []string{"src"}}}}}
	defs := map[string]any{
		// A fact field (04.6): the value, a source id of registers/sources.yaml or a work tag, and the locator.
		"fact": map[string]any{"type": "object", "required": []string{"v", "src"}, "properties": map[string]any{
			"v": map[string]any{}, "src": map[string]any{"type": "string", "minLength": 1}, "loc": str, "note": str,
		}, "additionalProperties": false},
		// A group of facts, such as training with training.tokens in it.
		"group": map[string]any{"type": "object", "allOf": []any{notFact}, "additionalProperties": map[string]any{"$ref": "#/$defs/field"}},
		"field": map[string]any{"anyOf": []any{
			map[string]any{"type": []string{"string", "number", "boolean", "array", "null"}},
			map[string]any{"$ref": "#/$defs/fact"},
			map[string]any{"$ref": "#/$defs/group"},
		}},
		"harvest": harvest,
	}
	props := map[string]any{
		"tag":     map[string]any{"type": "string", "pattern": TagPattern},
		"kind":    map[string]any{"enum": node.Kinds},
		"slug":    map[string]any{"type": "string", "pattern": SlugPattern, "maxLength": 60},
		"owner":   map[string]any{"type": "string", "pattern": KeyPattern},
		"name":    map[string]any{"type": "string", "minLength": 1},
		"role":    str,
		"summary": map[string]any{"type": "string", "description": fmt.Sprintf("at most %d words (rule S10)", card.MaxSummary)},
		"harvest": map[string]any{"oneOf": []any{
			map[string]any{"$ref": "#/$defs/harvest"},
			map[string]any{"type": "array", "items": map[string]any{"$ref": "#/$defs/harvest"}, "minItems": 1},
		}},
		"same_as":  map[string]any{"type": "object", "additionalProperties": map[string]any{"type": []string{"string", "integer", "null"}}},
		"aliases":  map[string]any{"type": "array", "items": str},
		"rules":    map[string]any{"type": "object"},
		"versions": map[string]any{},
	}
	results := map[string]any{}
	for _, k := range card.ResultFields {
		results[k] = map[string]any{}
	}
	s := map[string]any{
		"type":       "object",
		"required":   []string{"tag", "kind"},
		"properties": props,
		"allOf": []any{
			// A result card takes its owner, slug and name from its benchmark, tag and model (04.4); every other card writes them.
			map[string]any{
				"if":   map[string]any{"properties": map[string]any{"kind": map[string]any{"const": "result"}}, "required": []string{"kind"}},
				"then": map[string]any{"properties": results},
				"else": map[string]any{"required": []string{"slug", "name"}},
			},
			map[string]any{
				"if":   map[string]any{"properties": map[string]any{"kind": map[string]any{"const": "model"}}, "required": []string{"kind"}},
				"then": map[string]any{"properties": map[string]any{"role": map[string]any{"enum": card.ModelRoles}}},
			},
		},
		"unevaluatedProperties": map[string]any{"$ref": "#/$defs/field"},
		"$defs":                 defs,
	}
	return head("card.v1.json", "A card",
		"One line of cards/<kind>/<shard>.jsonl (spec 04.6): the fields of the kind, its facts with their sources, a summary, and the harvest records. aik check applies the rules of 15.3 beyond this schema.", s)
}

func keys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Experiment is the schema of experiment.yaml (spec 09.3.1): the design of an experiment. aik reads it with known fields
// only, so an unknown key is an error.
func Experiment() map[string]any {
	g := Gen{Tag: "yaml",
		Enums: map[string][]string{"Design.budget": keys(exp.Budgets)},
		// An experiment that makes its data in run.py, such as a simulation, has no inputs.
		Optional: []string{"Design.data"},
		Fields: map[string]map[string]any{
			"Design.id": {"type": "string", "pattern": SlugPattern},
			// The design of a run that the reader cannot repeat bit for bit says why (9.8).
			"Design.deterministic": {"type": "boolean"},
		},
	}
	s := g.Of(reflect.TypeFor[exp.Design]())
	s["allOf"] = []any{map[string]any{
		"if":   map[string]any{"properties": map[string]any{"deterministic": map[string]any{"const": false}}, "required": []string{"deterministic"}},
		"then": map[string]any{"required": []string{"deterministic_reason"}},
	}}
	data := s["properties"].(map[string]any)["data"].(map[string]any)
	data["items"].(map[string]any)["oneOf"] = []any{
		map[string]any{"required": []string{"pin"}}, map[string]any{"required": []string{"generator"}},
	}
	return head("experiment.v1.json", "experiment.yaml",
		"The design of an experiment (spec 09.3.1): the claims it tests, the arms, the data, the seeds and the budget class. aik check applies rules R01 to R08 beyond this schema.", s)
}

// Expect is the schema of expect.yaml (spec 09.3.2): the test of the claim, fixed before the first full run.
func Expect() map[string]any {
	g := Gen{Tag: "yaml",
		Enums: map[string][]string{
			"Test.test":         keys(exp.TestNames),
			"Test.direction":    {"lower", "higher"},
			"Test.form":         {"power", "linear"},
			"Effect.kind":       {"relative", "absolute"},
			"Expect.correction": {"holm", "none"},
		},
		Fields: map[string]map[string]any{
			"Test.at":    {"type": "string", "pattern": `^(final|best|curve|step:[0-9]+)$`},
			"Test.range": {"type": "array", "items": map[string]any{"type": "number"}, "minItems": 2, "maxItems": 2},
		},
		Types: map[string]map[string]any{"Tests": {"oneOf": []any{
			map[string]any{"$ref": "#/$defs/test"},
			map[string]any{"type": "array", "items": map[string]any{"$ref": "#/$defs/test"}},
		}}},
	}
	s := g.Of(reflect.TypeFor[exp.Expect]())
	s["$defs"] = map[string]any{"test": g.Of(reflect.TypeFor[exp.Test]())}
	set(s, "report", "properties", map[string]any{
		"ci": map[string]any{"type": "number", "exclusiveMinimum": 0, "exclusiveMaximum": 1},
		"bootstrap": map[string]any{"type": "object", "properties": map[string]any{
			"resamples": map[string]any{"type": "integer", "minimum": 1},
			"method":    map[string]any{"enum": []string{"bca", "percentile"}},
			"seed":      map[string]any{"type": "integer", "minimum": 0},
		}, "additionalProperties": false},
	})
	return head("expect.v1.json", "expect.yaml",
		"The test of an experiment (spec 09.3.2): the claim, the primary and secondary tests, the correction and the report. It is fixed before the first full run; results.json records its hash.", s)
}
