// SPDX-License-Identifier: Apache-2.0

package check

import (
	"strings"
	"testing"
)

func TestK04(t *testing.T) {
	body := `
Llama is a state-of-the-art model.
It is currently free.
Its SOTA score is high.
It is the largest open model.
A leading lab trained it.
It is at the frontier.
It is a [[frontier-model|frontier model]].
It is a [frontier model](/frontier-model/).
The largest eigenvalue of the matrix is 2.
The Pareto frontier has three points.
The paper says "the best model".
> The authors call it state of the art.
Write ` + "`SOTA`" + ` in code only.
It is leading to a new result.

` + "```" + `
currently = 1
` + "```" + `
`
	c := fixture(t, map[string]string{modelDir + "node.md": strings.Replace(modelNode, "\nA small model for the tests of the fact rules.\n", body, 1)})
	var got []string
	lines := map[int]bool{}
	for _, f := range Run(c, Options{}) {
		if f.Rule != "K04" {
			continue
		}
		got = append(got, f.String())
		if f.Severity != Level {
			t.Errorf("K04 is not a level rule: %s", f)
		}
		lines[f.Line] = true
	}
	// The front matter and the empty line take lines 1 to 10, so the first sentence is on line 11.
	for _, want := range []int{11, 12, 13, 14, 15, 16} {
		if !lines[want] {
			t.Errorf("K04 does not report line %d; it reports %q", want, got)
		}
	}
	if len(got) != 6 {
		t.Errorf("K04 reports %d findings, want 6: %q", len(got), got)
	}
}
