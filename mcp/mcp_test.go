// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/tamnd/ai-agent/node"
	"github.com/tamnd/ai-agent/site"
)

// corpus is the test corpus with a benchmark, three models, one fine-tuned from another, and their results.
func corpus(t *testing.T) *Data {
	t.Helper()
	root := filepath.Join(t.TempDir(), "corpus")
	if err := os.CopyFS(root, os.DirFS("../testdata/corpus")); err != nil {
		t.Fatal(err)
	}
	set := `"setting":{"shots":5,"prompt":"custom","cot":"none","decoding":"greedy","split":"test"}`
	files := map[string]string{
		"cards/benchmark/_.jsonl": `{"tag":"9MMLU0","kind":"benchmark","slug":"mmlu","name":"MMLU","versions":[{"id":"original-test","name":"MMLU, test set"},{"id":"pro","name":"MMLU-Pro"}],"same_as":{"arxiv":"2009.03300"}}` + "\n",
		"cards/model/_.jsonl": `{"tag":"9MA000","kind":"model","slug":"model-a","name":"Model A","params":{"total":{"v":7e9,"src":"S:paper-a","loc":"Table 1"}},"released":{"v":"2024-07-23","src":"S:paper-a"}}` + "\n" +
			`{"tag":"9MB000","kind":"model","slug":"model-b","name":"Model B","fine_tuned_from":"model-a"}` + "\n" +
			`{"tag":"9MC000","kind":"model","slug":"model-c","name":"Model C","fine_tuned_from":"model-b","distilled_from":"secret-model"}` + "\n",
		"cards/result/_.jsonl": `{"tag":"9R0001","kind":"result","model":"model-a","benchmark":"mmlu","version":"original-test","metric":"accuracy","score":{"v":0.886,"unit":"fraction"},` + set + `,"source":{"src":"S:paper-a","loc":"Table 2","date":"2024-07-23"},"checks":{"cite":"found"}}` + "\n" +
			`{"tag":"9R0002","kind":"result","model":"model-b","benchmark":"mmlu","version":"original-test","metric":"accuracy","score":{"v":0.79,"unit":"fraction"},` + set + `,"source":{"src":"S:paper-b","loc":"Table 1","date":"2024-01-02"},"checks":{"cite":"found"}}` + "\n" +
			`{"tag":"9R0003","kind":"result","model":"model-c","benchmark":"mmlu","version":"original-test","metric":"accuracy","score":{"v":0.99,"unit":"fraction"},` + set + `,"source":{"src":"S:blog","loc":"Figure 1","date":"2023-05-02"},"checks":{"cite":"not-found"}}` + "\n",
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
	d, err := Load(c, Options{Version: "test", AsOf: "2026-11-15"})
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestResolve(t *testing.T) {
	d := corpus(t)
	for id, want := range map[string]string{
		"binary-search":                         "binary-search",
		"2B7QDX":                                "binary-search",
		"/t/2B7QDX":                             "binary-search",
		"https://ai.tamnd.com/binary-search/":   "binary-search",
		"/binary-search.md":                     "binary-search",
		"/binary-search/#c-1":                   "binary-search",
		"Bisection search":                      "binary-search",
		"model-b":                               "model-b",
		"9MB000":                                "model-b",
		"https://ai.tamnd.com/benchmarks/mmlu/": "mmlu",
		"arxiv:2009.03300":                      "mmlu",
	} {
		it, ok := d.Resolve(id)
		if !ok {
			t.Errorf("Resolve(%q) found nothing, want %s", id, want)
			continue
		}
		if it.Key() != want {
			t.Errorf("Resolve(%q) = %s, want %s", id, it.Key(), want)
		}
	}
	for _, id := range []string{"", "no-such-item", "arxiv:1"} {
		if it, ok := d.Resolve(id); ok {
			t.Errorf("Resolve(%q) = %s, want nothing", id, it.Key())
		}
	}
}

func TestRecord(t *testing.T) {
	d := corpus(t)
	it, _ := d.Resolve("binary-search")
	r := d.Record(it)
	if r.URL != "https://ai.tamnd.com/binary-search/" || r.AsOf != "2026-11-15" || r.Markdown != "https://ai.tamnd.com/binary-search.md" {
		t.Errorf("head = %+v, markdown %q", r.Head, r.Markdown)
	}
	if !slices.ContainsFunc(r.Out, func(e site.Edge) bool { return e.Type == "solves" && e.To == "search-in-sorted-array" && e.Known }) {
		t.Errorf("edges out = %+v, want solves search-in-sorted-array", r.Out)
	}
	task, _ := d.Resolve("search-in-sorted-array")
	if !slices.ContainsFunc(d.Record(task).In, func(e site.Edge) bool { return e.From == "binary-search" && e.Type == "solves" }) {
		t.Errorf("edges in = %+v, want binary-search solves it", d.Record(task).In)
	}
	m, _ := d.Resolve("model-a")
	r = d.Record(m)
	if !slices.Equal(r.Results, []string{"9R0001"}) {
		t.Errorf("results = %v, want 9R0001", r.Results)
	}
	if r.URL != "https://ai.tamnd.com/models/model-a/" {
		t.Errorf("url = %q", r.URL)
	}
	fs := site.FactsOf(m)
	if len(fs) != 2 || fs[0].Sources[0].Src != "S:paper-a" {
		t.Errorf("facts = %+v", fs)
	}
	if _, err := d.Markdown(m); err == nil {
		t.Error("a card has a Markdown twin")
	}
	md, err := d.Markdown(it)
	if err != nil || !strings.Contains(md, "slug: binary-search") {
		t.Errorf("markdown = %.80q, %v", md, err)
	}
}

func TestSearch(t *testing.T) {
	d := corpus(t)
	hits, err := d.Search(SearchIn{Query: "binary search", Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 || hits[0].Key != "binary-search" || len(hits) > 2 {
		t.Fatalf("hits = %+v", hits)
	}
	hits, _ = d.Search(SearchIn{Query: "search", Kind: "task"})
	for _, h := range hits {
		if h.Kind != "task" {
			t.Errorf("kind filter let %s (%s) through", h.Key, h.Kind)
		}
	}
	hits, _ = d.Search(SearchIn{Query: "search", Area: "foundations"})
	for _, h := range hits {
		if !strings.HasPrefix(h.Area, "foundations/") {
			t.Errorf("area filter let %s (%s) through", h.Key, h.Area)
		}
	}
	if _, err := d.Search(SearchIn{}); err == nil {
		t.Error("an empty query is not an error")
	}
}

func TestResults(t *testing.T) {
	d := corpus(t)
	tab, err := d.Results(ResultsIn{Benchmark: "mmlu"})
	if err != nil {
		t.Fatal(err)
	}
	if tab.Table != nil || !slices.Equal(tab.Versions, []string{"original-test", "pro"}) {
		t.Fatalf("with no version: %+v", tab)
	}
	tab, err = d.Results(ResultsIn{Benchmark: "https://ai.tamnd.com/benchmarks/mmlu/original-test/"})
	if err != nil {
		t.Fatal(err)
	}
	if tab.URL != "https://ai.tamnd.com/benchmarks/mmlu/original-test/" || len(tab.Table.Groups) != 1 {
		t.Fatalf("table = %+v", tab)
	}
	rows := tab.Table.Groups[0].Rows
	if len(rows) != 2 || rows[0].Tag != "9R0001" || rows[1].Tag != "9R0002" {
		t.Errorf("rows = %v, want 9R0001 then 9R0002 (9R0003 was not found by the citation check)", rows)
	}
	tab, _ = d.Results(ResultsIn{Benchmark: "mmlu", Version: "original-test", Model: "model-b", Setting: map[string]string{"shots": "5"}})
	if len(tab.Table.Groups) != 1 || len(tab.Table.Groups[0].Rows) != 1 || tab.Table.Groups[0].Rows[0].Tag != "9R0002" {
		t.Errorf("filtered = %+v", tab.Table)
	}
	tab, _ = d.Results(ResultsIn{Benchmark: "mmlu", Version: "original-test", Setting: map[string]string{"shots": "0"}})
	if len(tab.Table.Groups) != 0 {
		t.Errorf("a setting that no result has kept %+v", tab.Table.Groups)
	}
	if _, err := d.Results(ResultsIn{Benchmark: "mmlu", Version: "nope"}); err == nil {
		t.Error("an unknown version is not an error")
	}
	if _, err := d.Results(ResultsIn{Benchmark: "gsm8k"}); err == nil {
		t.Error("an unknown benchmark is not an error")
	}
}

func TestLineage(t *testing.T) {
	d := corpus(t)
	b, _ := d.Resolve("model-b")
	l, err := d.Lineage(b, LineageIn{})
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	for _, n := range l.Nodes {
		keys = append(keys, n.Key)
	}
	slices.Sort(keys)
	if !slices.Equal(keys, []string{"model-a", "model-b", "model-c", "secret-model"}) {
		t.Errorf("nodes = %v", keys)
	}
	for _, n := range l.Nodes {
		if (n.Key == "secret-model") == n.Known {
			t.Errorf("%s known = %v", n.Key, n.Known)
		}
	}
	l, _ = d.Lineage(b, LineageIn{Direction: "up"})
	if len(l.Nodes) != 2 || len(l.Edges) != 1 || l.Edges[0] != (site.Edge{Type: "fine_tuned_from", From: "model-b", To: "model-a", Known: true}) {
		t.Errorf("up = %+v", l)
	}
	c, _ := d.Resolve("model-c")
	l, _ = d.Lineage(c, LineageIn{Direction: "up", Depth: 1, Types: []string{"fine_tuned_from"}})
	if len(l.Nodes) != 2 {
		t.Errorf("depth 1, fine_tuned_from only = %+v", l.Nodes)
	}
	if _, err := d.Lineage(c, LineageIn{Direction: "sideways"}); err == nil {
		t.Error("a bad direction is not an error")
	}
}

// TestServer calls the tools through an MCP client, as an agent does.
func TestServer(t *testing.T) {
	d := corpus(t)
	ctx := context.Background()
	st, ct := sdk.NewInMemoryTransports()
	ss, err := New(d).Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()
	cs, err := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	tools, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tl := range tools.Tools {
		names = append(names, tl.Name)
		if tl.Annotations == nil || !tl.Annotations.ReadOnlyHint {
			t.Errorf("%s is not marked read-only", tl.Name)
		}
	}
	if !slices.Equal(names, []string{"get_claims", "get_facts", "get_item", "get_results", "lineage", "search"}) {
		t.Errorf("tools = %v", names)
	}
	call := func(name string, args map[string]any) (*sdk.CallToolResult, map[string]any) {
		t.Helper()
		res, err := cs.CallTool(ctx, &sdk.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		var m map[string]any
		if b, err := json.Marshal(res.StructuredContent); err == nil {
			_ = json.Unmarshal(b, &m)
		}
		return res, m
	}
	res, m := call("get_facts", map[string]any{"item": "9MA000", "ids": []string{"released"}})
	if res.IsError {
		t.Fatalf("get_facts: %v", res.Content)
	}
	fs, _ := m["facts"].([]any)
	if len(fs) != 1 || fs[0].(map[string]any)["value"] != "2024-07-23" {
		t.Errorf("get_facts = %v", m)
	}
	if item, _ := m["item"].(map[string]any); item["url"] != "https://ai.tamnd.com/models/model-a/" || item["as_of"] != "2026-11-15" {
		t.Errorf("item = %v", m["item"])
	}
	res, _ = call("get_item", map[string]any{"id": "binary-search", "format": "markdown"})
	if res.IsError || !strings.Contains(res.Content[0].(*sdk.TextContent).Text, "name: Binary search") {
		t.Errorf("markdown = %v", res.Content)
	}
	res, _ = call("get_item", map[string]any{"id": "no-such-item"})
	if !res.IsError {
		t.Error("an unknown item is not an error")
	}
	res, _ = call("get_item", map[string]any{"id": "binary-search", "format": "html"})
	if !res.IsError {
		t.Error("an unknown format is not an error")
	}
	_, m = call("search", map[string]any{"query": "binary search", "limit": 1})
	if rs, _ := m["results"].([]any); len(rs) != 1 {
		t.Errorf("search = %v", m)
	}
}
