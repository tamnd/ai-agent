// SPDX-License-Identifier: Apache-2.0

package check

import (
	"strings"
	"testing"
)

const bridgeShard = `{"tag":"5KMEAN","site":"al","kind":"algorithm","slug":"k-means","name":"k-means","url":"https://algorithm.tamnd.com/k-means/","old_slugs":["lloyd"]}
{"tag":"K8YWXT","site":"al","kind":"problem","slug":"2-sat","name":"2-SAT","url":"https://algorithm.tamnd.com/2-sat/","retired":"2026-11-08"}
`

func TestBridgeTargets(t *testing.T) {
	md := strings.Replace(modelNode, "summary:", "see_also: [k-means@al, 2-sat@al, lloyd@al, nothing@al, nothing@m2]\nsummary:", 1)
	md += "\nIt uses [[al:k-means]], [[al:lloyd|Lloyd]], [[al:2-sat]] and [[pg:numpy]].\n"
	c := fixture(t, map[string]string{
		"cards/bridge/al/_.jsonl": bridgeShard,
		"cards/bridge/pg/_.jsonl": `{"tag":"NUMPY1","site":"al","slug":"numpy","name":"NumPy","url":"https://programming.tamnd.com/numpy/"}` + "\n",
		modelDir + "node.md":      md,
	})
	fs := Run(c, Options{})
	e01 := fired(fs, "E01")
	if len(e01) != 2 || !strings.Contains(strings.Join(e01, "\n"), `no bridge card has the slug "nothing" on math2.tamnd.com`) {
		t.Errorf("E01 gives %q", e01)
	}
	l02 := strings.Join(fired(fs, "L02"), "\n")
	for _, want := range []string{
		"see_also: 2-sat@al is not on algorithm.tamnd.com since the sync of 2026-11-08 (broken-bridge)",
		"see_also: lloyd@al is now k-means on algorithm.tamnd.com; write k-means@al",
		"the link [[al:lloyd]] uses an old slug; write [[al:k-means]]",
		"the link [[al:2-sat]] names an item that is not on algorithm.tamnd.com",
		"the link [[pg:numpy]] has no bridge card on programming.tamnd.com",
	} {
		if !strings.Contains(l02, want) {
			t.Errorf("L02 does not say %q:\n%s", want, l02)
		}
	}
	if strings.Contains(l02, "[[al:k-means]] has") {
		t.Errorf("a link to a current bridge card fires L02:\n%s", l02)
	}
	t01 := strings.Join(fired(fs, "T01"), "\n")
	if !strings.Contains(t01, `the card NUMPY1 has the site "al", but it is in cards/bridge/pg/`) {
		t.Errorf("T01 does not report the card in the wrong directory:\n%s", t01)
	}
	if strings.Contains(t01, "5KMEAN") || strings.Contains(strings.Join(fired(fs, "T02"), "\n"), "5KMEAN") {
		t.Errorf("the rules of our cards run on the bridge cards:\n%s\n%s", t01, fired(fs, "T02"))
	}
}
