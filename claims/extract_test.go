// SPDX-License-Identifier: Apache-2.0

package claims

import (
	"reflect"
	"strings"
	"testing"
)

func TestExtract(t *testing.T) {
	body := strings.Join([]string{
		"## Overview", // 10
		"",            // 11
		"Smith et al. (2019) report a gain of 2.5 points. {#c-gain} It is small.",    // 12
		"A **binary search** halves the range at each step, e.g. from 8 to 4.",       // 13
		"This section gives the algorithm. See [[binary-search|the page]] for more.", // 14
		"",                                     // 15
		"$$ x = \\frac{1}{2}. y $$ {#eq-half}", // 16
		"After the display, the cost is $O(\\log n)$. Then we stop.", // 17
		"$$",                                 // 18
		"a. B",                               // 19
		"$$ {#eq-two}",                       // 20
		"```python",                          // 21
		"x = 1. Y = 2.",                      // 22
		"```",                                // 23
		"> A quoted line. It says things.",   // 24
		"> [!note]",                          // 25
		"> The note was introduced in 2020.", // 26
		"- An item with 3.5% more.",          // 27
		"| a | b |",                          // 28
		"::claim{ref=x#c-y}",                 // 29
	}, "\n")
	got := Extract(body, 10)
	want := []Sentence{
		{12, "Smith et al. (2019) report a gain of 2.5 points. {#c-gain}", ClassClaim, []string{"c-gain"}, []string{"year", "citation", "number", "verb"}},
		{12, "It is small.", ClassClaim, nil, nil},
		{13, "A **binary search** halves the range at each step, e.g. from 8 to 4.", ClassDefinition, nil, nil},
		{14, "This section gives the algorithm.", ClassStructure, nil, nil},
		{14, "See [[binary-search|the page]] for more.", ClassStructure, nil, nil},
		{17, "After the display, the cost is $O(\\log n)$.", ClassClaim, nil, nil},
		{17, "Then we stop.", ClassClaim, nil, nil},
		{24, "A quoted line.", ClassQuote, nil, nil},
		{24, "It says things.", ClassQuote, nil, nil},
		{26, "The note was introduced in 2020.", ClassClaim, nil, []string{"year", "verb"}},
		{27, "An item with 3.5% more.", ClassClaim, nil, []string{"number"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Extract =\n%v\nwant\n%v", got, want)
	}
	if got[0].Candidate() {
		t.Error("an anchored sentence is a K02 candidate")
	}
	if got[1].Candidate() {
		t.Error("a sentence with no signal is a K02 candidate")
	}
	if !got[9].Candidate() {
		t.Error("an unanchored sentence with a year is not a K02 candidate")
	}
}

func TestPlain(t *testing.T) {
	for in, want := range map[string]string{
		"See [[binary-search]] and [[a-b|the A]] and [x](http://y). {#c-z}": "See binary search and the A and x.",
		"The **term** in `code`.": "The term in code.",
	} {
		if got := Plain(in); got != want {
			t.Errorf("Plain(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPhrases(t *testing.T) {
	byID := map[string]*Claim{
		"c-emp":  {ID: "c-emp", Type: Empirical},
		"c-math": {ID: "c-math", Type: Mathematical},
		"c-attr": {ID: "c-attr", Type: Attributive},
		"c-self": {ID: "c-self", Type: Empirical, SelfReported: true},
	}
	cases := []struct {
		text string
		want string // a part of the message, or "" for no phrase
	}{
		{"It is well established that dropout helps.", `"well established"`},
		{"It is now known that the bound is tight.", `known that"`},
		{"They trace some well-known bugs.", ""},
		{"Turc et al. show that pre-training matters. {#c-emp}", `"show" is for a proved claim`},
		{"VGG nets showed in 2014 that depth helps.", `"showed" is for a proved claim`},
		{"The sum shows that the bound holds. {#c-math}", ""},
		{"The proof shows that the bound holds.", ""},
		{"They show in theory that the error falls.", ""},
		{"The table shows that the error falls.", ""},
		{"The test risk shows the classical U shape.", ""},
		{"The experiment below shows what that changes.", ""},
		{"Hardt and Ma proved that no local optima exist. {#c-attr}", ""},
		{"The study proves that the method works. {#c-emp}", `"proves" is for a proved claim, and the claim here is not mathematical`},
		{"Watkins and Dayan prove convergence.", ""},
		{"The model reached 67.5% on MMLU. {#c-self}", "c-self is a self-report"},
		{"Meta reports 67.5% on MMLU. {#c-self}", ""},
		{`He wrote "this is well established".`, ""},
	}
	for _, c := range cases {
		got := Phrases(Extract(c.text, 1), byID)
		switch {
		case c.want == "" && len(got) > 0:
			t.Errorf("%q: %v", c.text, got)
		case c.want != "" && (len(got) != 1 || !strings.Contains(got[0].Msg, c.want)):
			t.Errorf("%q: %v, want %q", c.text, got, c.want)
		}
	}
	quote := Phrases(Extract("> Smith shows that it is well established.", 1), byID)
	if len(quote) > 0 {
		t.Errorf("a block quote has phrases: %v", quote)
	}
}
