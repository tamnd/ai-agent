// SPDX-License-Identifier: Apache-2.0

package site

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/ai-agent/node"
)

func TestPolicyPage(t *testing.T) {
	root := filepath.Join(t.TempDir(), "corpus")
	if err := os.CopyFS(root, os.DirFS("../testdata/corpus")); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"nodes/foundations/models-of-computation/toy-act/node.md": `---
tag: 9TQX4P
kind: policy
slug: toy-act
name: Toy Act
area: foundations/models-of-computation
level: core
summary: A small law for the tests of the policy page.
official_text: "http://data.europa.eu/eli/reg/2024/1689/oj"
---

## What it does

It regulates toys.

## Main provisions

The notes on the groups of articles follow.

## References
`,
		"cards/provision/_.jsonl": `{"tag":"9TQX4R","kind":"provision","slug":"article-50","owner":"toy-act","name":"Article 50","summary":"Transparency duties.","applies":{"general":{"v":"2026-08-02","src":"S:act","loc":"Article 113"},"existing_systems":{"v":"2026-12-02","src":"S:act","loc":"Article 111(4)"}}}` + "\n" +
			`{"tag":"9TQX4S","kind":"provision","slug":"article-5","owner":"toy-act","name":"Article 5","summary":"Prohibited practices."}` + "\n" +
			`{"tag":"9TQX4T","kind":"provision","slug":"article-1","owner":"other-act","name":"Article 1 of another act","summary":"Not of this act."}` + "\n",
	}
	for p, s := range files {
		full := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	c, err := node.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "site")
	if _, err := Build(c, Options{Out: out}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(out, "toy-act", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	page := string(b)
	for _, want := range []string{
		"This page is a summary, not legal advice.",
		`href="http://data.europa.eu/eli/reg/2024/1689/oj"`,
		"The provision cards of Toy Act",
		"2026-08-02",
		"2026-08-02 <span class=\"card-loc\">(general)</span>",
		`href="/kind/provision/#toy-act--article-50"`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the policy page does not contain %q", want)
		}
	}
	if g, e := strings.Index(page, "(general)"), strings.Index(page, "(existing systems)"); g < 0 || e < g {
		t.Error("the dates of a provision are not in the order of time")
	}
	if strings.Contains(page, "Article 1 of another act") {
		t.Error("the policy page lists a provision of another policy")
	}
	h2, list, notes := strings.Index(page, `id="main-provisions"`), strings.Index(page, "The provision cards of Toy Act"), strings.Index(page, "The notes on the groups")
	if h2 < 0 || h2 > list || list > notes {
		t.Error("the list of provisions is not between the heading of Main provisions and the prose of the section")
	}
	if a5, a50 := strings.Index(page, ">Article 5<"), strings.Index(page, ">Article 50<"); a5 < 0 || a5 > a50 {
		t.Error("Article 5 is not listed before Article 50")
	}
	b, err = os.ReadFile(filepath.Join(out, "binary-search", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "not legal advice") {
		t.Error("a page that is not a policy carries the legal notice")
	}
}

func TestNaturalCmp(t *testing.T) {
	for _, s := range [][2]string{{"article-5", "article-50"}, {"article-50", "article-113"}, {"article-5", "article-5a"}, {"annex-iii", "article-1"}, {"govern-1-1", "govern-1-2"}, {"govern-1-2", "govern-1-10"}} {
		if naturalCmp(s[0], s[1]) >= 0 || naturalCmp(s[1], s[0]) <= 0 {
			t.Errorf("%s does not come before %s", s[0], s[1])
		}
	}
	if naturalCmp("article-5", "article-5") != 0 {
		t.Error("a slug does not equal itself")
	}
}
