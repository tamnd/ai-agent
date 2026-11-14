// SPDX-License-Identifier: Apache-2.0

package site

import (
	"strings"
	"testing"

	"github.com/tamnd/ai-agent/card"
	"github.com/tamnd/ai-agent/computed"
	"github.com/tamnd/ai-agent/facts"
	"github.com/tamnd/ai-agent/node"
)

func TestFactRefs(t *testing.T) {
	agreed := facts.Fact{ID: "params.total", Value: 405e9, Unit: "parameters", Status: facts.Agreed,
		Sources: []facts.Source{{Src: "dubey-2024-llama-3", Loc: "Table 3", Value: 405e9, Kind: facts.Primary}}}
	disputed := facts.Fact{ID: "training.tokens", Value: 15.6e12, Unit: "tokens", Status: facts.Disputed,
		Sources: []facts.Source{{Src: "dubey-2024-llama-3", Loc: "Section 3", Value: 15.6e12, Kind: facts.Primary}, {Src: "epoch-2025", Value: 15e12, Kind: facts.Secondary}}}
	c := &node.Corpus{
		BySlug:    map[string]*node.Node{"llama-3-1-405b": {Slug: "llama-3-1-405b", Facts: []facts.Fact{agreed, disputed}}},
		CardByKey: map[string]*card.Card{"gpt-2-xl": {Slug: "gpt-2-xl", Facts: []card.Fact{{ID: "params.total", V: 1.5e9, Src: "radford-2019", Loc: "Table 2"}}}},
	}
	body := "It has {fact:llama-3-1-405b/params.total} and saw {fact:llama-3-1-405b/training.tokens}.\n" +
		"GPT-2 had {fact:gpt-2-xl/params.total}; again {fact:llama-3-1-405b/params.total}.\n" +
		"`{fact:llama-3-1-405b/params.total}` and {fact:nothing/params.total}\n"
	got := factRefs(c, body)
	for _, want := range []string{
		"It has 405 billion parameters[^fact-1] and",
		"15.6 trillion tokens[^fact-2] or 15 trillion tokens[^fact-3] (the sources differ)",
		"GPT-2 had 1.5 billion parameters[^fact-4]; again 405 billion parameters[^fact-1].",
		"`{fact:llama-3-1-405b/params.total}`",
		"{fact:nothing/params.total}",
		"[^fact-1]: dubey-2024-llama-3, Table 3\n",
		"[^fact-3]: epoch-2025\n",
		"[^fact-4]: radford-2019, Table 2\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("factRefs: no %q in\n%s", want, got)
		}
	}
	if strings.Count(got, "[^fact-1]:") != 1 {
		t.Errorf("the footnote of a source is written once:\n%s", got)
	}
}

func TestComputedRef(t *testing.T) {
	c := &node.Corpus{Computed: map[string]*computed.Result{"biggest": {
		ID: "biggest", Sentence: "Among the model cards, the model with the most total parameters is Big (2 trillion parameters), as of 2026-11-14.",
		Src: "S:epoch-ai", Loc: "Parameters", AsOf: "2026-11-14"}}}
	got := factRefs(c, "{fact:computed/biggest}. And {fact:computed/other}.\n")
	for _, want := range []string{
		"Among the model cards, the model with the most total parameters is Big (2 trillion parameters), as of 2026-11-14[^fact-1]. And {fact:computed/other}.",
		"[^fact-1]: S:epoch-ai, Parameters (2026-11-14)\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("factRefs: no %q in\n%s", want, got)
		}
	}
}
