// SPDX-License-Identifier: Apache-2.0

package site

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/ai-agent/node"
)

func TestResultsPage(t *testing.T) {
	root := filepath.Join(t.TempDir(), "corpus")
	if err := os.CopyFS(root, os.DirFS("../testdata/corpus")); err != nil {
		t.Fatal(err)
	}
	set := `"setting":{"shots":5,"prompt":"custom","cot":"none","decoding":"greedy","split":"test"}`
	files := map[string]string{
		"cards/benchmark/_.jsonl": `{"tag":"9MMLU0","kind":"benchmark","slug":"mmlu","name":"MMLU","versions":[{"id":"original-test","name":"MMLU, test set"},{"id":"pro","name":"MMLU-Pro"}]}` + "\n",
		"cards/model/_.jsonl":     `{"tag":"9MA000","kind":"model","slug":"model-a","name":"Model A"}` + "\n",
		"cards/result/_.jsonl": `{"tag":"9R0001","kind":"result","model":"model-a","benchmark":"mmlu","version":"original-test","metric":"accuracy","score":{"v":0.886,"unit":"fraction"},` + set + `,"source":{"src":"S:paper-a","loc":"Table 2","date":"2024-07-23"},"self_reported":true,"checks":{"cite":"found"}}` + "\n" +
			`{"tag":"9R0002","kind":"result","model":"model-b","benchmark":"mmlu","version":"original-test","metric":"accuracy","score":{"v":0.79,"unit":"fraction"},` + set + `,"source":{"src":"S:paper-b","loc":"Table 1","date":"2024-01-02"},"checks":{"cite":"found"}}` + "\n" +
			`{"tag":"9R0003","kind":"result","model":"model-b","benchmark":"mmlu","version":"original-test","metric":"accuracy","score":{"v":0.70,"unit":"fraction"},` + set + `,"source":{"src":"S:other","loc":"Table 4","date":"2024-03-02"},"checks":{"cite":"found"}}` + "\n" +
			`{"tag":"9R0004","kind":"result","model":"model-c","benchmark":"mmlu","version":"original-test","metric":"accuracy","score":{"v":0.65,"unit":"fraction"},"setting":{"shots":"unknown"},"source":{"src":"S:c","loc":"Table 3","date":"2023-05-02"},"checks":{"cite":"found"}}` + "\n" +
			`{"tag":"9R0005","kind":"result","model":"model-c","benchmark":"mmlu","version":"original-test","metric":"accuracy","score":{"v":0.99,"unit":"fraction"},` + set + `,"source":{"src":"S:blog","loc":"Figure 1","date":"2023-05-02"},"checks":{"cite":"not-found"}}` + "\n",
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
	b, err := os.ReadFile(filepath.Join(out, "benchmarks", "mmlu", "original-test", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	page := string(b)
	for _, want := range []string{
		"<h1>MMLU, test set</h1>",
		"4 results in 2 comparable groups.",
		"1 more result is not shown",
		"accuracy · shots 5, prompt custom, cot none, decoding greedy, split test",
		`<tr id="r-9R0001"><td>1</td><th scope="row"><a href="/models/#model-a">Model A</a>`,
		"88.6%",
		"self-reported",
		"2 contested results",
		`<h2 id="other-settings">Other settings</h2>`,
		"accuracy · shots unknown",
		`<a href="/benchmarks/mmlu/pro/">MMLU-Pro</a>`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the page does not contain %q", want)
		}
	}
	if strings.Contains(page, "r-9R0005") {
		t.Error("the page shows a result that the citation check did not find")
	}
	data, err := os.ReadFile(filepath.Join(out, "benchmarks", "mmlu", "original-test", "results.json"))
	if err != nil {
		t.Fatal(err)
	}
	var tb struct {
		Count  int `json:"count"`
		Hidden int `json:"hidden"`
	}
	if err := json.Unmarshal(data, &tb); err != nil || tb.Count != 4 || tb.Hidden != 1 {
		t.Errorf("results.json gives %+v, %v", tb, err)
	}
	if _, err := os.Stat(filepath.Join(out, "benchmarks", "mmlu", "pro", "index.html")); err != nil {
		t.Errorf("a version with no results has no page: %v", err)
	}
}

// A benchmark node with versions and no benchmark card gets the catalogue /benchmarks/, which the crumbs of its
// results pages link to.
func TestResultsCatalogueWithoutCards(t *testing.T) {
	root := filepath.Join(t.TempDir(), "corpus")
	if err := os.CopyFS(root, os.DirFS("../testdata/corpus")); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "nodes", "searching", "sequential-and-binary-search", "toy-bench")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"node.md":       "---\ntag: 6TQX4P\nkind: benchmark\nslug: toy-bench\nname: Toy bench\narea: searching/sequential-and-binary-search\nlevel: intro\nsummary: A small benchmark.\n---\n\nA small benchmark.\n",
		"releases.yaml": "- id: full\n  name: Toy bench, full set\n  metric: accuracy\n",
	}
	for p, s := range files {
		if err := os.WriteFile(filepath.Join(dir, p), []byte(s), 0o644); err != nil {
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
	v, err := os.ReadFile(filepath.Join(out, "benchmarks", "toy-bench", "full", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(v), `href="/benchmarks/"`) {
		t.Error("the results page has no crumb to /benchmarks/")
	}
	cat, err := os.ReadFile(filepath.Join(out, "benchmarks", "index.html"))
	if err != nil {
		t.Fatalf("a benchmark node with no card has no catalogue: %v", err)
	}
	if !strings.Contains(string(cat), `href="/toy-bench/"`) || !strings.Contains(string(cat), "0 cards, 1 page") {
		t.Error("the catalogue does not list the benchmark page")
	}
}
