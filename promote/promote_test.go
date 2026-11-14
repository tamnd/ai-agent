// SPDX-License-Identifier: Apache-2.0

package promote

import (
	"strings"
	"testing"

	"github.com/tamnd/ai-agent/card"
	"github.com/tamnd/ai-agent/facts"
)

const model = `{"tag":"8RWQ2D","kind":"model","slug":"llama-3-1-405b","owner":"llama","name":"Llama 3.1 405B","role":"release","developer":["meta-ai"],"released":{"v":"2024-07-23","src":"S:meta-llama31-blog"},"architecture":"decoder-only-transformer","params":{"total":{"v":4.05e11,"src":"S:epoch-ai-models","loc":"Table 3"}},"training":{"tokens":{"v":1.56e13,"src":"S:epoch-ai-models"}},"access":{"v":"open-weights","src":"S:epoch-ai-models"},"same_as":{"epoch":"Llama 3.1-405B","hub":"meta-llama/Llama-3.1-405B","wikidata":null},"colour":"blue","summary":"The largest model of the Llama 3.1 release.","harvest":{"source":"epoch-ai-models","row":"Llama 3.1-405B","date":"2026-11-12"},"rules":{"version":"r-2026.11","passed":true}}`

func TestMake(t *testing.T) {
	c, err := card.Parse([]byte(model))
	if err != nil {
		t.Fatal(err)
	}
	p, err := Make(c, "foundation-models/open-weight-llms", "core", []string{"Overview", "Releases"})
	if err != nil {
		t.Fatal(err)
	}
	if p.Rel != "nodes/foundation-models/open-weight-llms/llama-3-1-405b" {
		t.Errorf("Rel = %s", p.Rel)
	}
	want := `---
tag: 8RWQ2D
kind: model
slug: llama-3-1-405b
name: Llama 3.1 405B
area: foundation-models/open-weight-llms
level: core
summary: The largest model of the Llama 3.1 release.
member_of: llama
role: release
same_as:
  epoch: Llama 3.1-405B
  hub: meta-llama/Llama-3.1-405B
---

## Overview

## Releases
`
	if p.Front != want {
		t.Errorf("Front =\n%s\nwant\n%s", p.Front, want)
	}
	if got := strings.Join(p.Dropped, "\n"); !strings.Contains(got, "colour: not a field of a model node") || !strings.Contains(got, "harvest:") || !strings.Contains(got, "rules:") {
		t.Errorf("Dropped = %q", p.Dropped)
	}
	byID := map[string]facts.Fact{}
	for _, f := range p.Facts {
		byID[f.ID] = f
	}
	if len(p.Facts) != 6 {
		t.Fatalf("Facts = %+v", p.Facts)
	}
	pt := byID["params.total"]
	if pt.Unit != "parameters" || pt.Value != 4.05e11 || pt.Sources[0].Kind != facts.Secondary || pt.Sources[0].Row != "Llama 3.1-405B" || pt.Sources[0].Loc != "Table 3" || pt.Sources[0].AsOf != "2026-11-12" || pt.AsOf != "" {
		t.Errorf("params.total = %+v", pt)
	}
	if r := byID["released"]; r.Sources[0].Kind != facts.Primary || r.Sources[0].Row != "" {
		t.Errorf("released = %+v", r)
	}
	// developer and architecture are plain fields of the card and required facts of a model: the harvested row is
	// their source.
	if d := byID["developer"]; d.Sources[0].Src != "S:epoch-ai-models" || d.Sources[0].Kind != facts.Secondary || d.Sources[0].Row != "Llama 3.1-405B" {
		t.Errorf("developer = %+v", d)
	}
	if byID["architecture"].Value != "decoder-only-transformer" {
		t.Errorf("architecture = %+v", byID["architecture"])
	}
	// access is true only at a date, so it takes the date of the harvest (D07).
	if a := byID["access"]; a.AsOf != "2026-11-12" {
		t.Errorf("access = %+v", a)
	}
	if strings.Join(p.Sources, " ") != "S:epoch-ai-models S:meta-llama31-blog" {
		t.Errorf("Sources = %q", p.Sources)
	}
	b, err := FactFile(p.Facts)
	if err != nil {
		t.Fatal(err)
	}
	if s := string(b); !strings.Contains(s, "- id: access\n  value: open-weights\n  as_of: \"2026-11-12\"\n") || strings.Contains(s, "status") {
		t.Errorf("facts.yaml =\n%s", s)
	}
}

func TestMakeRefuses(t *testing.T) {
	for _, line := range []string{
		`{"tag":"NEW-x","kind":"model","slug":"x"}`,
		`{"tag":"ABCDEF","kind":"nonsense","slug":"x"}`,
		`{"tag":"ABCDEF","kind":"model"}`,
	} {
		c, err := card.Parse([]byte(line))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Make(c, "a/b", "core", nil); err == nil {
			t.Errorf("Make took %s", line)
		}
	}
	c, _ := card.Parse([]byte(`{"tag":"ABCDEF","kind":"model","slug":"x"}`))
	if _, err := Make(c, "a", "core", nil); err == nil {
		t.Error("Make took an area with no sub-area")
	}
}
