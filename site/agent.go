// SPDX-License-Identifier: Apache-2.0

package site

import (
	"encoding/json"

	"github.com/tamnd/ai-agent/card"
	"github.com/tamnd/ai-agent/cite"
	"github.com/tamnd/ai-agent/node"
)

// ForAgents prepares a corpus as the site shows it, for the MCP server (16.10), so that an agent reads what a person
// reads: what the quarantine hides is not there (15.7), each claim has the labels of the citation check (07.12), and
// index is the search.json of the palette (17.5), for search.Ranker. The claims of the given corpus get their labels.
func ForAgents(c *node.Corpus) (shown *node.Corpus, index []byte, err error) {
	shown, wd, err := withdraw(c)
	if err != nil {
		return nil, nil, err
	}
	ix, err := cite.Load(shown.Root)
	if err != nil {
		return nil, nil, err
	}
	computeClaims(shown, ix)
	es := []searchEntry{}
	for _, n := range shown.Nodes {
		if w, ok := wd[n.Slug]; !ok || !w.Whole {
			es = append(es, searchOf(shown, n))
		}
	}
	sortSearch(es)
	index, err = json.Marshal(es)
	return shown, index, err
}

// CardURL is the path of the page that a card owns (16.2), or "" for a result card, which has none.
func CardURL(c *card.Card) string { return cardURL(c) }
