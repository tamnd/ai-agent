// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFactsCompute(t *testing.T) {
	root := copyCorpus(t)
	write := func(rel, s string) {
		t.Helper()
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	h := `"harvest":{"source":"epoch-ai","row":"r","date":"2026-11-14"}`
	write("cards/model/_.jsonl", strings.Join([]string{
		`{"tag":"7KQ2MX","kind":"model","slug":"small","name":"Small","accessibility":{"v":"Open weights (unrestricted)","src":"S:epoch-ai"},"params":{"total":{"v":7000000000,"src":"S:epoch-ai","loc":"Parameters"}},` + h + `}`,
		`{"tag":"5TQH2W","kind":"model","slug":"big","name":"Big","accessibility":{"v":"API access","src":"S:epoch-ai"},"params":{"total":{"v":1800000000000,"src":"S:epoch-ai","loc":"Parameters"}},` + h + `}`,
	}, "\n")+"\n")
	write("topics/computed.yaml", "- id: most-params-open\n  kind: model\n  where: {accessibility: Open weights (unrestricted)}\n  measure: params.total\n  order: max\n  what: the model with the most total parameters\n  population: the model cards with open weights\n")

	if got := run([]string{"facts", "compute", "-root", root}); got != 0 {
		t.Fatalf("aik facts compute exits with %d", got)
	}
	b, err := os.ReadFile(filepath.Join(root, "build/facts/computed.json"))
	if err != nil {
		t.Fatal(err)
	}
	var res computedFile
	if err := json.Unmarshal(b, &res); err != nil {
		t.Fatal(err)
	}
	if len(res.Facts) != 1 || res.Facts[0].Item != "small" || res.Facts[0].Sentence != "Among the model cards with open weights, the model with the most total parameters is Small (7 billion parameters), as of 2026-11-14." {
		t.Fatalf("computed.json:\n%s", b)
	}

	// The prose may name the computed fact; a fact that is not defined blocks (D08).
	md := filepath.Join(root, "nodes/searching/sequential-and-binary-search/binary-search/node.md")
	page, err := os.ReadFile(md)
	if err != nil {
		t.Fatal(err)
	}
	write("nodes/searching/sequential-and-binary-search/binary-search/node.md", string(page)+"\n{fact:computed/most-params-open}.\n")
	if got := run([]string{"check", "-root", root, "-rule", "D08"}); got != 0 {
		t.Fatalf("aik check -rule D08 with a computed fact exits with %d", got)
	}
	write("nodes/searching/sequential-and-binary-search/binary-search/node.md", string(page)+"\n{fact:computed/most-params-closed}.\n")
	if got := run([]string{"check", "-root", root, "-rule", "D08"}); got != 1 {
		t.Fatalf("aik check -rule D08 with an undefined computed fact exits with %d", got)
	}

	// A definition with no result fails the command.
	write("topics/computed.yaml", "- id: none\n  kind: model\n  where: {accessibility: Nope}\n  measure: params.total\n  order: max\n  what: x\n  population: y\n")
	if got := run([]string{"facts", "compute", "-root", root}); got != 1 {
		t.Fatalf("aik facts compute with no result exits with %d", got)
	}
	for _, args := range [][]string{{"facts"}, {"facts", "compute", "-root", root, "x"}} {
		if got := run(args); got != 2 {
			t.Errorf("aik %v exits with %d", args, got)
		}
	}
}
