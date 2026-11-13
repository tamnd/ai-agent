// SPDX-License-Identifier: Apache-2.0

package status

import (
	"testing"

	"github.com/tamnd/ai-agent/bridge"
	"github.com/tamnd/ai-agent/node"
)

// A prerequisite on a neighbour exists when a bridge card names it and the card is not retired (5.9).
func TestWritingPrereqOnNeighbour(t *testing.T) {
	cards := bridge.NewSet([]*bridge.Card{
		{Site: "al", Slug: "embedding"},
		{Site: "m2", Slug: "dot-product", Retired: "2026-10-01"},
	})
	c := &node.Corpus{BySlug: map[string]*node.Node{"dropout": {Slug: "dropout"}}, Bridges: cards}
	for _, tc := range []struct {
		prereqs []string
		want    bool
	}{
		{[]string{"dropout", "embedding@al"}, true},
		{[]string{"missing@al"}, false},
		{[]string{"dot-product@m2"}, false},
		{[]string{"missing"}, false},
	} {
		n := &node.Node{Slug: "x", Prereqs: tc.prereqs}
		if got := prereqsExist(n, c); got != tc.want {
			t.Errorf("prereqs %v: prereqsExist is %v, want %v", tc.prereqs, got, tc.want)
		}
	}
}

// A pitfall callout counts as an item of the Pitfalls section, as a list item does.
func TestCalloutsAreItems(t *testing.T) {
	sec := "> [!pitfall]\n> **One.** Text.\n\n> [!pitfall]\n> **Two.** Text.\n\n- three\n"
	if got := len(listItem.FindAllString(sec, -1)) + len(callout.FindAllString(sec, -1)); got != 3 {
		t.Errorf("items: got %d, want 3", got)
	}
}
