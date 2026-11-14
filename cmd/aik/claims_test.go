// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/ai-agent/claims"
)

func TestClaimsExtract(t *testing.T) {
	root := copyCorpus(t)
	dir := filepath.Join(root, "nodes/searching/sequential-and-binary-search/binary-search")
	b, err := os.ReadFile(filepath.Join(dir, "node.md"))
	if err != nil {
		t.Fatal(err)
	}
	prose := "\nKnuth (1998) reports that the first correct version came out in 1962. {#c-first}\n" +
		"Bentley found that most programmers wrote it wrong.\n"
	if err := os.WriteFile(filepath.Join(dir, "node.md"), append(b, prose...), 0o644); err != nil {
		t.Fatal(err)
	}
	cl := "- id: c-first\n  type: historical\n  text: The first correct version was published in 1962.\n" +
		"- id: c-never\n  type: historical\n  text: A claim the prose does not anchor.\n"
	if err := os.WriteFile(filepath.Join(dir, "claims.yaml"), []byte(cl), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := run([]string{"claims", "extract", "-root", root, "binary-search"}); got != 0 {
		t.Fatalf("aik claims extract exits with %d", got)
	}
	out, err := os.ReadFile(filepath.Join(root, "build/claims/binary-search.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var ss []claims.Sentence
	for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		var s claims.Sentence
		if err := json.Unmarshal([]byte(l), &s); err != nil {
			t.Fatalf("%q: %v", l, err)
		}
		ss = append(ss, s)
	}
	last := ss[len(ss)-1]
	if !strings.HasPrefix(last.Text, "Bentley found") || !last.Candidate() {
		t.Fatalf("the last sentence is %+v", last)
	}
	if got := ss[len(ss)-2].Anchors; len(got) != 1 || got[0] != "c-first" {
		t.Fatalf("the anchors of the sentence before it are %v", got)
	}
	// -strict fails on the unanchored claim sentence.
	if got := run([]string{"claims", "extract", "-root", root, "-strict", "binary-search"}); got != 1 {
		t.Fatalf("aik claims extract -strict exits with %d", got)
	}

	// A K05 phrase fails.
	if err := os.WriteFile(filepath.Join(dir, "node.md"), append(b, "\nIt is well established that it works.\n"...), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := run([]string{"claims", "extract", "-root", root, "-all"}); got != 1 {
		t.Fatalf("aik claims extract with a K05 phrase exits with %d", got)
	}
	for _, args := range [][]string{{"claims"}, {"claims", "extract", "-root", root}, {"claims", "extract", "-root", root, "-all", "x"}, {"claims", "extract", "-root", root, "nope"}} {
		if got := run(args); got != 2 {
			t.Errorf("aik %v exits with %d", args, got)
		}
	}
}
