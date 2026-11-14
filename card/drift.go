// SPDX-License-Identifier: Apache-2.0

package card

import (
	"fmt"
	"sort"

	"github.com/tamnd/ai-agent/facts"
)

// Move is a fact of a card that exists on both sides of a change and whose value moved by more than the tolerance of
// spec 04.9 (facts.Same), or that the change dropped.
type Move struct {
	Kind string `json:"kind"`
	Key  string `json:"key"`
	Tag  string `json:"tag,omitempty"`
	Fact string `json:"fact"`
	Old  any    `json:"old"`
	New  any    `json:"new,omitempty"`
	// Gone is set when the fact or the whole card is not on the new side.
	Gone bool `json:"gone,omitempty"`
}

func (m Move) String() string {
	if m.Gone {
		return fmt.Sprintf("%s/%s %s: %v is gone", m.Kind, m.Key, m.Fact, m.Old)
	}
	return fmt.Sprintf("%s/%s %s: %v -> %v", m.Kind, m.Key, m.Fact, m.Old, m.New)
}

// Drift compares the cards of a change (spec 14.9): the daily job merges a pull request by itself only when no fact of
// an existing card moved beyond its tolerance and none was dropped. A new card and a new fact of an existing card are
// not moves; they are what a harvest adds. A card is the same card on both sides when it has the same kind and key.
func Drift(old, cur []*Card) []Move {
	type id struct{ kind, key string }
	now := make(map[id]*Card, len(cur))
	for _, c := range cur {
		now[id{c.Kind, c.Key()}] = c
	}
	var out []Move
	for _, o := range old {
		c := now[id{o.Kind, o.Key()}]
		have := map[string]Fact{}
		if c != nil {
			for _, f := range c.Facts {
				have[f.ID] = f
			}
		}
		for _, f := range o.Facts {
			n, ok := have[f.ID]
			switch {
			case !ok:
				out = append(out, Move{Kind: o.Kind, Key: o.Key(), Tag: o.Tag, Fact: f.ID, Old: f.V, Gone: true})
			case !facts.Same(f.V, n.V):
				out = append(out, Move{Kind: o.Kind, Key: o.Key(), Tag: o.Tag, Fact: f.ID, Old: f.V, New: n.V})
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.Key != b.Key {
			return a.Key < b.Key
		}
		return a.Fact < b.Fact
	})
	return out
}
