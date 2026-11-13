// SPDX-License-Identifier: Apache-2.0

package check

import (
	"slices"
	"strings"
	"testing"
)

const modelDir = "nodes/foundations/models-of-computation/toy-model/"

const modelNode = `---
tag: 7TQYMD
kind: model
slug: toy-model
name: Toy model
area: foundations/models-of-computation
level: intro
summary: A small model for the tests of the fact rules.
---

A small model for the tests of the fact rules.
`

const modelFacts = `- id: params.total
  value: 8000000000
  unit: parameters
  sources:
    - {src: paper, kind: primary, value: 7000000000}
    - {src: epoch-ai, kind: secondary, row: "12", value: 7000000000}
- id: Bad-ID
  sources:
    - {src: paper, kind: primary, loc: "p. 2", value: x}
- id: released
  status: agreed
  sources:
    - {src: paper, kind: primary, loc: "p. 1", value: 2024-07}
    - {src: blog, kind: secondary, value: 2024-09}
- id: training.compute_flop
  unit: FLOPS
  sources:
    - {src: epoch-ai, kind: estimate, value: 3.8e25}
- id: licence
  sources:
    - {kind: secondary, value: open}
- id: context_length
  as_of: 13-11-2026
  sources:
    - {src: card, kind: official, value: 128000}
`

func TestFactRules(t *testing.T) {
	c := fixture(t, map[string]string{
		modelDir + "node.md":    modelNode,
		modelDir + "facts.yaml": modelFacts,
	})
	fs := Run(c, Options{})
	for rule, want := range map[string][]string{
		"T01": {`the fact id "Bad-ID"`, `the as_of of the fact "context_length"`, `has the kind "official"`},
		"T05": {`the fact "released" has the status "agreed"`},
		"D01": {`the fact "params.total" has the value 8e+09, but its best source gives 7e+09`, `the source 1 of the fact "licence" has no src`},
		"D03": {"a model needs the facts developer, architecture, access, modalities"},
		"D04": {`the primary source 1 of the fact "params.total" has no locator`, `the fact "licence" has no primary source`},
		"D05": {`the fact "released"`},
		"D06": {`the estimate of the fact "training.compute_flop"`},
		"D07": {`the fact "licence" is true only at a date`},
		"D09": {`the unit "FLOPS"`},
	} {
		got := fired(fs, rule)
		for _, w := range want {
			if !slices.ContainsFunc(got, func(s string) bool { return strings.Contains(s, w) }) {
				t.Errorf("%s does not report %q; it reports %q", rule, w, got)
			}
		}
	}
	for rule, sev := range map[string]string{"D03": "level", "D04": "level", "D05": "warn", "D06": "block", "D07": "block"} {
		for _, s := range fired(fs, rule) {
			if !strings.Contains(s, ": "+sev+": ") {
				t.Errorf("%s is not a %s rule: %q", rule, sev, s)
			}
		}
	}
}

func TestD02(t *testing.T) {
	c := fixture(t, map[string]string{modelDir + "node.md": modelNode})
	if got := fired(Run(c, Options{}), "D02"); len(got) != 1 || !strings.Contains(got[0], "toy-model/node.md") {
		t.Errorf("D02 reports %q, want one finding on the model", got)
	}
}
