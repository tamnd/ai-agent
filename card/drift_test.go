// SPDX-License-Identifier: Apache-2.0

package card

import (
	"strings"
	"testing"
)

func cards(t *testing.T, lines ...string) []*Card {
	t.Helper()
	var out []*Card
	for _, l := range lines {
		c, err := Parse([]byte(l))
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, c)
	}
	return out
}

func TestDrift(t *testing.T) {
	old := cards(t,
		`{"tag":"AAAAAA","kind":"model","slug":"m","owner":"f","params":{"total":{"v":1000,"src":"S:a"}},"released":{"v":"2024-07","src":"S:a"},"name_list":{"v":["a","b"]}}`,
		`{"tag":"BBBBBB","kind":"benchmark","slug":"b","items":{"v":500,"src":"S:b"},"license":{"v":"MIT"}}`,
		`{"tag":"CCCCCC","kind":"model","slug":"gone","params":{"total":{"v":7}}}`,
	)
	cur := cards(t,
		// Within 1%, a more precise date, a list in another order and a new fact: no move.
		`{"tag":"AAAAAA","kind":"model","slug":"m","owner":"f","params":{"total":{"v":1009,"src":"S:b"}},"released":{"v":"2024-07-23","src":"S:a"},"name_list":{"v":["b","a"]},"context":{"v":8192}}`,
		// Beyond 1%, and a fact dropped.
		`{"tag":"BBBBBB","kind":"benchmark","slug":"b","items":{"v":520,"src":"S:b"}}`,
		// A new card is not a move.
		`{"tag":"DDDDDD","kind":"model","slug":"new","params":{"total":{"v":3}}}`,
		// The same slug in another kind is another card.
		`{"tag":"EEEEEE","kind":"dataset","slug":"gone","params":{"total":{"v":7}}}`,
	)
	got := Drift(old, cur)
	var s []string
	for _, m := range got {
		s = append(s, m.String())
	}
	want := []string{
		"benchmark/b items: 500 -> 520",
		"benchmark/b license: MIT is gone",
		"model/gone params.total: 7 is gone",
	}
	if strings.Join(s, "\n") != strings.Join(want, "\n") {
		t.Errorf("got\n%s\nwant\n%s", strings.Join(s, "\n"), strings.Join(want, "\n"))
	}
	if got[0].Tag != "BBBBBB" || got[0].Gone || !got[1].Gone {
		t.Errorf("the moves are %+v", got)
	}
	if m := Drift(old, old); len(m) != 0 {
		t.Errorf("a tree drifts from itself: %v", m)
	}
}
