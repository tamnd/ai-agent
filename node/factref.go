// SPDX-License-Identifier: Apache-2.0

package node

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/tamnd/ai-agent/facts"
)

// FactRefRE finds a reference of the prose to a fact (spec 06.2): {fact:llama-3-1-405b/params.total} names the fact
// params.total of the item llama-3-1-405b. The item is a node slug or a card key; the field is the last part.
var FactRefRE = regexp.MustCompile(`\{fact:([a-z0-9][a-z0-9/-]*)/([a-z][a-z0-9_.-]*)\}`)

// ErrComputed is the error of a reference to a computed fact, {fact:computed/<id>}, which only aik facts compute
// produces (rule D08).
var ErrComputed = errors.New("computed facts come from aik facts compute, which does not produce them yet (D08)")

// Fact finds the fact field of an item: a record of the facts.yaml of the node with that slug, or else a fact field
// of the card with that key, as a record with the card's source.
func (c *Corpus) Fact(item, field string) (facts.Fact, error) {
	if item == "computed" {
		return facts.Fact{}, ErrComputed
	}
	if n := c.BySlug[item]; n != nil {
		for _, f := range n.Facts {
			if f.ID == field {
				return f, nil
			}
		}
		return facts.Fact{}, fmt.Errorf("the node %s has no fact %s in facts.yaml", item, field)
	}
	if k := c.CardByKey[item]; k != nil {
		for _, f := range k.Facts {
			if f.ID == field {
				var src []facts.Source
				if f.Src != "" {
					src = []facts.Source{{Src: f.Src, Loc: f.Loc, Value: f.V}}
				}
				return facts.Fact{ID: f.ID, Value: f.V, Unit: facts.CardUnits[f.ID], Sources: src}, nil
			}
		}
		return facts.Fact{}, fmt.Errorf("the card %s has no fact %s", item, field)
	}
	return facts.Fact{}, fmt.Errorf("no node or card is %s", item)
}
