// SPDX-License-Identifier: Apache-2.0

package check

import (
	"os"
	"strings"
	"testing"
)

// cardTags adds the tags of the test cards to the tags of the test corpus.
func cardTags(t *testing.T, lines ...string) string {
	t.Helper()
	b, err := os.ReadFile("../testdata/corpus/registers/tags.reg")
	if err != nil {
		t.Fatal(err)
	}
	return string(b) + strings.Join(lines, "\n") + "\n"
}

func TestCardsClean(t *testing.T) {
	c := fixture(t, map[string]string{
		"registers/tags.reg": cardTags(t, "5TQX4M llama-3-1", "5TQX4N llama-3-1/llama-3-1-405b"),
		"cards/model/5T.jsonl": `{"tag":"5TQX4M","kind":"model","slug":"llama-3-1","name":"Llama 3.1","role":"family","summary":"A family of open-weight models."}
{"tag":"5TQX4N","kind":"model","slug":"llama-3-1-405b","owner":"llama-3-1","name":"Llama 3.1 405B","role":"release","summary":"The largest model of the family.","params":{"total":{"v":4.05e11,"src":"S:paper","loc":"Table 3"}},"harvest":{"source":"epoch-ai","date":"2026-11-01"}}
`,
	})
	if len(c.Cards) != 2 || c.CardByKey["llama-3-1/llama-3-1-405b"] == nil {
		t.Fatalf("the corpus has %d cards", len(c.Cards))
	}
	for _, f := range Run(c, Options{Main: true}) {
		if strings.HasPrefix(f.Path, "cards/") {
			t.Errorf("a clean card gives %s %s", f.Rule, f.Msg)
		}
	}
}

func TestCardRules(t *testing.T) {
	long := strings.Repeat("word ", 31)
	c := fixture(t, map[string]string{
		"registers/tags.reg": cardTags(t, "5TQX4M family-a", "5TQX4P wrong-slug", "5TQX4R dup-a", "5TQX4S dup-a", "5TQX4T binary-search", "5TQX4V role-a", "5TQX4W fact-a", "6TQX4M shard-a"),
		"cards/model/5T.jsonl": `{"tag":"5TQX4M","kind":"model","slug":"family-a","name":"Family A","summary":"` + long + `"}
not json
{"tag":"5TQX4P","kind":"model","slug":"right-slug","name":"B"}
{"tag":"5TQX4N","kind":"model","slug":"no-reg","name":"C"}
{"tag":"5TQX4R","kind":"model","slug":"dup-a","name":"D"}
{"tag":"5TQX4S","kind":"model","slug":"dup-a","name":"E"}
{"tag":"5TQX4T","kind":"model","slug":"binary-search","name":"F"}
{"tag":"5TQX4V","kind":"model","slug":"role-a","name":"G","role":"base"}
{"tag":"5TQX4W","kind":"model","slug":"fact-a","name":"H","params":{"total":{"v":null}},"harvest":[{"date":"13-11-2026"}]}
{"tag":"6TQX4M","kind":"model","slug":"shard-a","name":"I"}
{"tag":"5TQX4X","kind":"model","slug":"Bad_Slug","owner":"nobody","name":"J"}
{"tag":"2B7QDX","kind":"dataset","slug":"no-name"}
{"tag":"5TQX4Y","kind":"robot","slug":"robot-a","name":"K"}
{"tag":"NEW-x","kind":"model","slug":"new-a","name":"L"}
{"tag":"5TQX4","kind":"model","slug":"short-a","name":"M"}
{"tag":"5TQX4Z","kind":"model","slug":"sorted-a","name":"N"}
{"tag":"5TQX40","kind":"model","slug":"sorted-b","name":"O"}
`,
	})
	fs := Run(c, Options{Main: true})
	for rule, want := range map[string][]string{
		"T01": {
			"cards/model/5T.jsonl: block: the line is not a card",
			"the summary of the card family-a has 31 words",
			`the model card role-a has the role "base"`,
			"the harvest record 1 of the card fact-a names no source",
			`the harvest record 1 of the card fact-a has the date "13-11-2026"`,
			"the tag 6TQX4M is not in the shard 5T",
			"the card no-name has no name",
			"the card no-name is a dataset, but it is in cards/model/",
			`the card robot-a has the kind "robot"`,
			"the lines are not sorted by tag",
		},
		"T02": {
			`registers/tags.reg gives the tag 5TQX4P to "wrong-slug", not "right-slug"`,
			"the tag 5TQX4N is not in registers/tags.reg",
			"the tag 2B7QDX is also the tag of the node",
			`the tag "5TQX4" is not 6 characters`,
		},
		"T03": {
			`the slug "dup-a" is also the slug of the card on cards/model/5T.jsonl:5`,
			`the slug "binary-search" is also the slug of the node`,
			`the slug "Bad_Slug" is not lowercase kebab-case`,
		},
		"T06": {"the provisional id NEW-x is on main"},
		"E01": {`owner: no node or card has the slug "nobody"`},
		"D01": {"the fact params.total of the card fact-a has no src"},
	} {
		got := strings.Join(fired(fs, rule), "\n")
		for _, w := range want {
			if !strings.Contains(got, w) {
				t.Errorf("%s does not give %q; it gives:\n%s", rule, w, got)
			}
		}
	}
	if n := strings.Count(strings.Join(fired(fs, "T01"), "\n"), "not sorted"); n != 1 {
		t.Errorf("the sort finding fires %d times", n)
	}
}

func TestCardEdgeTarget(t *testing.T) {
	md := strings.Replace(modelNode, "summary:", "see_also: [toy-bench]\npeople: [toy-bench]\nsummary:", 1)
	c := fixture(t, map[string]string{
		"registers/tags.reg":       cardTags(t, "5TQX4M toy-bench"),
		"cards/benchmark/5T.jsonl": `{"tag":"5TQX4M","kind":"benchmark","slug":"toy-bench","name":"Toy bench"}` + "\n",
		modelDir + "node.md":       md,
	})
	got := fired(Run(c, Options{}), "E01")
	if len(got) != 1 || !strings.Contains(got[0], `people: "toy-bench" is a benchmark card, not a person`) {
		t.Errorf("E01 gives %q", got)
	}
}
