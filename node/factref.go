// SPDX-License-Identifier: Apache-2.0

package node

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/tamnd/ai-agent/computed"
	"github.com/tamnd/ai-agent/facts"
)

// FactRefRE finds a reference of the prose to a fact (spec 06.2): {fact:llama-3-1-405b/params.total} names the fact
// params.total of the item llama-3-1-405b. The item is a node slug or a card key; the field is the last part.
var FactRefRE = regexp.MustCompile(`\{fact:([a-z0-9][a-z0-9/-]*)/([a-z][a-z0-9_.-]*)\}`)

// ErrComputed is the error of a reference to a computed fact, {fact:computed/<id>}, that aik facts compute does not
// produce (rule D08): it has no definition in topics/computed.yaml, or its definition gives no result.
var ErrComputed = errors.New("a computed fact comes from aik facts compute (D08)")

// Fact finds the fact field of an item: a record of the facts.yaml of the node with that slug, or else a fact field
// of the card with that key, as a record with the card's source.
func (c *Corpus) Fact(item, field string) (facts.Fact, error) {
	if item == "computed" {
		if r := c.Computed[field]; r != nil {
			return facts.Fact{ID: field, Value: r.Value, Unit: r.Unit, AsOf: r.AsOf,
				Sources: []facts.Source{{Src: r.Src, Loc: r.Loc, Value: r.Value, AsOf: r.AsOf}}}, nil
		}
		if err := c.ComputedErrs[field]; err != nil {
			return facts.Fact{}, fmt.Errorf("%w: %w", ErrComputed, err)
		}
		return facts.Fact{}, fmt.Errorf("%w: %s has no definition in %s", ErrComputed, field, computed.File)
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
