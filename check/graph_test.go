// SPDX-License-Identifier: Apache-2.0

package check

import (
	"slices"
	"strings"
	"testing"
)

func TestGraphRules(t *testing.T) {
	core := func(slug, name, front, body string) string {
		return "---\ntag: 7TQ" + strings.ToUpper(slug[:3]) + "\nkind: concept\nslug: " + slug + "\nname: " + name +
			"\narea: searching/sequential-and-binary-search\nlevel: core\nsummary: A toy.\n" + front + "---\n\n## In plain words\n\n" + body + "\n"
	}
	dir := "nodes/searching/sequential-and-binary-search/"
	c := fixture(t, map[string]string{
		// No path to a foundation: binary-search leads only to the foundations of the test corpus.
		dir + "far/node.md": core("far", "Far", "prereqs: [binary-search]\nrelated_to: [loop-invariant, comparison-model]\n",
			"It differs from the [comparison model](/comparison-model/) in one way.\n\n```\nloop invariant\n```"),
		// A record kind with a full page has no prerequisites to check.
		dir + "kay/node.md": strings.Replace(core("kay", "Kay", "", "A person."), "kind: concept", "kind: person", 1),
		// A path through math2, and a related node named in the text.
		dir + "near/node.md": core("near", "Near", "prereqs: [far, matrix-multiplication@m2]\nrelated_to: [loop-invariant, comparison-model]\n",
			"Unlike a loop invariant, it is a toy. It is also not [[comparison-model|the model of comparisons]]."),
		"cards/bridge/m2/_.jsonl": `{"tag":"M2MM01","site":"m2","kind":"theorem","slug":"matrix-multiplication","name":"Matrix multiplication","url":"https://math2.tamnd.com/matrix-multiplication/"}` + "\n",
		"cards/benchmark/_.jsonl": `{"tag":"9MMLU0","kind":"benchmark","slug":"mmlu","name":"MMLU","versions":[{"id":"original-test","name":"MMLU, test set"}]}` + "\n",
		"cards/model/_.jsonl": `{"tag":"9MA000","kind":"model","slug":"model-a","name":"Model A","fine_tuned_from":"model-c"}` + "\n" +
			`{"tag":"9MB000","kind":"model","slug":"model-b","name":"Model B","fine_tuned_from":"model-a"}` + "\n" +
			`{"tag":"9MC000","kind":"model","slug":"model-c","name":"Model C","fine_tuned_from":"model-b"}` + "\n" +
			`{"tag":"9MD000","kind":"model","slug":"model-d","name":"Model D","architecture":"binary-search","distilled_from":"mmlu","merged_from":["unknown-model"],"evaluated_on":"mmlu@original-test","members":["model-a"]}` + "\n",
		"cards/policy/_.jsonl": `{"tag":"9P0000","kind":"policy","slug":"act","name":"An act","amended_by":["act-2"],"regulates":[{"slug":"model-a","source":"S:act"},{"slug":"model-b","source":"S:act","locator":"Article 3"}]}` + "\n",
		"cards/result/_.jsonl": `{"tag":"9R0001","kind":"result","model":"model-a","benchmark":"mmlu","version":"original-test","metric":"accuracy","score":{"v":0.9,"unit":"fraction"},"source":{"src":"S:paper","loc":"Table 2","date":"2024-07-23"},"evidence_for":[{"slug":"comparison-model","role":"replication"},{"slug":"far","claim":"c-1","role":"original"}]}` + "\n",
	})
	fs := Run(c, Options{})
	want := map[string][]string{
		"E04": {"cards/model/_.jsonl: block: the fine_tuned_from edges have a cycle: model-a -> model-c -> model-b -> model-a"},
		"E06": {"cards/model/_.jsonl: block: evaluated_on: mmlu@original-test is written by hand; the build derives evaluated_on from the result cards (10.6)"},
		"E07": {
			`cards/model/_.jsonl: block: architecture: "binary-search" is a method, not an architecture`,
			`cards/model/_.jsonl: block: distilled_from: "mmlu" is a benchmark, not a model`,
		},
		"E09": {dir + "far/node.md: level: the core node has no path of prereqs back to ai-foundations, math-for-ai, deep-learning-foundations, or to a bridge card of math2 or algorithm (5.7)"},
		"E10": {dir + "far/node.md: level: related_to loop-invariant has no note: no sentence of the text names it; add one on the difference"},
		"E11": {"cards/model/_.jsonl: block: the card model-d has members, the reverse of member_of; the build derives reverse edges, so write member_of on the other end"},
		"E13": {"cards/result/_.jsonl: block: evidence_for comparison-model has no claim; write {slug: comparison-model, claim: <claim id>, role: <role>} (07.3)"},
		"E14": {"cards/policy/_.jsonl: level: regulates model-a has no locator; write {slug: model-a, source: <work>, locator: <locator>}"},
	}
	for rule, w := range want {
		got := fired(fs, rule)
		slices.Sort(got)
		slices.Sort(w)
		if !slices.Equal(got, w) {
			t.Errorf("%s gives\n  %s\nwant\n  %s", rule, strings.Join(got, "\n  "), strings.Join(w, "\n  "))
		}
	}
	// Only limits the node rules to its slugs, and the card rules do not run.
	fs = Run(c, Options{Only: map[string]bool{"near": true}})
	for _, rule := range []string{"E04", "E06", "E07", "E09", "E10", "E11", "E13", "E14"} {
		if got := fired(fs, rule); len(got) > 0 {
			t.Errorf("with Only near, %s gives %q", rule, got)
		}
	}
}
