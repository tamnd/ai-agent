// SPDX-License-Identifier: Apache-2.0

package check

import (
	"slices"
	"strings"
	"testing"
)

const claimNode = `---
tag: 7TQYMD
kind: model
slug: toy-model
name: Toy model
area: foundations/models-of-computation
level: intro
summary: A small model for the tests of the claim rules.
---

The toy model fits the data {#c-fits}.

It fits again {#c-fits}, and {#c-missing} names no claim.

The old result {#c-old} is superseded.

One {#c-fits} {#c-old} {#c-new} sentence.

` + "```" + `
{#c-in-code}
` + "```" + `

::claim{ref=toy-model#c-fits}

::claim{ref=no-such-node#c-fits}

::claim{ref=toy-model#c-nothing}
`

const claimFile = `- id: c-fits
  type: empirical
  text: The toy model fits the data.
  scope: {models: toy, data: [toy-set], scale: small, setting: training}
  evidence:
    - {work: paper, role: original, stance: for, locator: "Table 2"}
- id: c-old
  type: historical
  text: An old result.
  superseded_by: c-new
  evidence:
    - {work: paper, role: original, stance: for}
- id: c-new
  type: historical
  text: A new result.
  labels: [replicated]
  evidence:
    - {work: paper, role: primary-document, stance: maybe}
- id: c-new
  type: definitional
  text: A duplicate.
  scope: {models: toy}
  evidence:
    - {work: paper, role: original, stance: for, reuses: code, locator: "p. 1"}
- id: Bad
  type: guess
  text: ""
- id: c-theorem
  type: mathematical
  text: A bound.
- id: c-proof
  type: mathematical
  text: A proved bound.
  assumes: [convex loss]
  derivation: full
- id: c-wide
  type: empirical
  text: It works everywhere.
  evidence:
    - {role: review, stance: for, locator: "p. 3"}
    - {work: other, role: law, stance: for, locator: "p. 4"}
- id: c-guess
  type: forecast
  text: It will scale.
  review_after: 2020-01-01
  evidence:
    - {work: blog, role: official-text, stance: for, locator: "the blog, near the end"}
- id: c-gone
  type: empirical
  text: Gone.
  scope: {models: unknown}
  superseded_by: c-nowhere
  as_of: 13-11-2026
  evidence:
    - {work: paper, role: replication, stance: for, reuses: everything, locator: "p. 1"}
`

func TestClaimRules(t *testing.T) {
	c := fixture(t, map[string]string{
		modelDir + "node.md":     claimNode,
		modelDir + "claims.yaml": claimFile,
	})
	fs := Run(c, Options{})
	for rule, want := range map[string][]string{
		"T01": {
			`the claim "c-new" is in the file more than once`,
			`the claim id "Bad"`,
			`the claim "Bad" has the type "guess"`,
			`the claim "Bad" has no text`,
			`the empirical claim "c-wide" has no scope`,
			`the mathematical claim "c-theorem" has no assumes`,
			`the source 1 of the claim "c-new" has the stance "maybe"`,
			`the source 2 of the claim "c-wide" has the role "law"`,
			`reuses "everything"`,
			`the as_of of the claim "c-gone"`,
			`superseded by "c-nowhere"`,
		},
		"T05": {`the claim "c-new" has the field "labels"`},
		"T02": {`the definitional claim "c-new" has a scope`, `the source 1 of the claim "c-new" has reuses`},
		"G01": {`the source 1 of the claim "c-wide" names no work`},
		"G02": {`the claim "c-theorem" has no source`, `the claim "Bad" has no source`},
		"G03": {`the source 1 of the claim "c-old" has no locator`, `the locator "the blog, near the end" of the source 1 of the claim "c-guess" is not in the grammar`},
		"K01": {
			"the claim c-fits is anchored twice",
			"the anchor {#c-missing} names no claim",
			"a sentence holds more than two claims",
			`names the node "no-such-node"`,
			"the node toy-model has no claim c-nothing",
		},
		"K03": {"a forecast claim cannot cite a source with the role official-text"},
		"K07": {`the forecast "c-guess" needs the date`},
		"K09": {"the claim c-old is superseded by c-new"},
		"K13": {`the claim "c-guess" was due for review on 2020-01-01`},
	} {
		got := fired(fs, rule)
		for _, w := range want {
			if !slices.ContainsFunc(got, func(s string) bool { return strings.Contains(s, w) }) {
				t.Errorf("%s does not report %q; it reports %q", rule, w, got)
			}
		}
	}
	for _, s := range fired(fs, "G02") {
		if strings.Contains(s, "c-proof") {
			t.Errorf("G02 reports a claim with a derivation: %q", s)
		}
	}
	for _, s := range fired(fs, "K01") {
		if strings.Contains(s, "c-in-code") {
			t.Errorf("K01 reads an anchor in code: %q", s)
		}
	}
	for rule, sev := range map[string]string{"G02": "level", "G03": "level", "K01": "block", "K03": "block", "K07": "level", "K09": "level", "K13": "warn"} {
		for _, s := range fired(fs, rule) {
			if !strings.Contains(s, ": "+sev+": ") {
				t.Errorf("%s is not a %s rule: %q", rule, sev, s)
			}
		}
	}
}

func TestSmallScale(t *testing.T) {
	cy := "- id: c-fits\n  type: empirical\n  text: The toy model fits the data.\n  scope: {models: toy}\n  small_scale: maybe\n  evidence:\n    - {work: paper, role: original, stance: for, locator: \"Table 2\"}\n- id: c-toy\n  type: definitional\n  text: A toy is a small model.\n  small_scale: yes\n  evidence:\n    - {work: paper, role: original, stance: for, locator: \"p. 1\"}\n"
	fs := Run(fixture(t, map[string]string{modelDir + "node.md": claimNode, modelDir + "claims.yaml": cy}), Options{})
	if got := fired(fs, "T01"); !slices.ContainsFunc(got, func(s string) bool { return strings.Contains(s, `small_scale: "maybe"`) }) {
		t.Errorf("T01 = %q", got)
	}
	if got := fired(fs, "T02"); !slices.ContainsFunc(got, func(s string) bool { return strings.Contains(s, `the definitional claim "c-toy" has small_scale`) }) {
		t.Errorf("T02 = %q", got)
	}
}
