// SPDX-License-Identifier: Apache-2.0

package check

import (
	"strings"
	"testing"
)

const benchCards = `{"tag":"9MMLU0","kind":"benchmark","slug":"mmlu","name":"MMLU","versions":[{"id":"original-test","name":"MMLU, test set"}]}
`

const badResults = `{"tag":"9R0001","kind":"result","model":"model-a","benchmark":"mmlu","version":"original-test","metric":"accuracy","score":{"v":0.886,"unit":"fraction"},"setting":{"shots":5,"prompt":"custom"},"source":{"src":"S:paper-a","loc":"Table 2","kind":"primary","date":"2024-07-23"},"checks":{"cite":"found"}}
{"tag":"9R0002","kind":"result","model":"model-x","benchmark":"mmlu","version":"hidden","metric":"bleu","score":{"unit":"fraction"},"setting":{"shot":5,"cot":""},"source":{"src":"S:paper-b","date":"July 2024"},"checks":{"cite":"maybe"}}
{"tag":"9R0003","kind":"result","model":"model-a","benchmark":"image-classification","version":"v1","metric":"accuracy","score":{"v":0.5,"unit":"fraction"},"source":{"src":"S:p","loc":"Table 1","date":"2024-01-01"},"checks":{"cite":"not-found"}}
{"tag":"9R0004","kind":"result","model":"model-a","benchmark":"mmlu","version":"original-test","metric":"rating","score":{"v":1200,"unit":"rating"},"setting":{"split":"test"},"source":{"src":"S:arena","loc":"leaderboard","date":"2025-01-01"},"checks":{"cite":"found"}}
{"tag":"9R0005","kind":"result","model":"model-a","benchmark":"mmlu","version":"original-test","metric":"accuracy","score":"high"}
`

func TestResultRules(t *testing.T) {
	fs := Run(fixture(t, map[string]string{
		"cards/benchmark/_.jsonl": benchCards,
		"cards/metric/_.jsonl":    "{\"tag\":\"9ACC00\",\"kind\":\"metric\",\"slug\":\"accuracy\",\"name\":\"Accuracy\"}\n{\"tag\":\"9RAT00\",\"kind\":\"metric\",\"slug\":\"rating\",\"name\":\"Rating\"}\n",
		"cards/model/_.jsonl":     "{\"tag\":\"9MA000\",\"kind\":\"model\",\"slug\":\"model-a\",\"name\":\"Model A\"}\n",
		"cards/task/_.jsonl":      "{\"tag\":\"9IMG00\",\"kind\":\"task\",\"slug\":\"image-classification\",\"name\":\"Image classification\"}\n",
		"cards/result/_.jsonl":    badResults,
	}), Options{})
	for rule, want := range map[string][]string{
		"M01": {
			"the result 9R0002 has no source.loc",
			"the result 9R0002 has no score.v",
			`the source date "July 2024"`,
			`the model "model-x" of the result 9R0002 is not a model`,
			`the metric "bleu" of the result 9R0002 is not a metric`,
			"the result 9R0003 has no setting",
		},
		"M02": {`the benchmark mmlu has no version "hidden"`, `names "image-classification", which is not a benchmark`},
		"M03": {`the setting key "shot" of the result 9R0002 is not a key of 10.5`, "the setting key cot of the result 9R0002 is empty"},
		"M08": {"the result 9R0003"},
		"M14": {"the rating 9R0004 needs arena"},
		"T01": {`the result 9R0002 has checks.cite "maybe"`, "the result 9R0005: the fields do not have the types"},
	} {
		got := fired(fs, rule)
		for _, w := range want {
			if len(only(got, w)) == 0 {
				t.Errorf("%s does not report %q; it gives %q", rule, w, got)
			}
		}
	}
	for _, f := range fired(fs, "M01") {
		if strings.Contains(f, "9R0001") {
			t.Errorf("a good result fires M01: %s", f)
		}
	}
	for _, f := range fired(fs, "D01") {
		if strings.Contains(f, "result") || strings.Contains(f, "9r000") {
			t.Errorf("the score of a result is read as a fact: %s", f)
		}
	}
}
