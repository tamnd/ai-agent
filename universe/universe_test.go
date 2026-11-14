// SPDX-License-Identifier: Apache-2.0

package universe

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/ai-agent/harvest"
)

const scope = `- id: S-M1
  kind: model
  outcome: card
  rule: Each model in the database of Epoch AI is a card. More words.
  source: epoch-ai
- id: S-W1
  kind: work
  outcome: card
  rule: A cited work is a card.
  source: arxiv
  thresholds: {top: 0.02}
`

func row(src, id, name, kind string, same map[string]string, facts map[string]harvest.Fact) harvest.Row {
	return harvest.Row{Source: src, ID: id, Name: name, Kind: kind, SameAs: same, Facts: facts, Snapshot: "b3:aa", Date: "2026-11-13"}
}

// tree writes a content root with the scope rules and the inventories of a small harvest.
func tree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "registers"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ScopeFile), []byte(scope), 0o644); err != nil {
		t.Fatal(err)
	}
	inv := map[string][]harvest.Row{
		"epoch-ai": {
			row("epoch-ai", "GPT-4", "GPT-4", "model", map[string]string{"epoch": "GPT-4"},
				map[string]harvest.Fact{"released": {V: "2023-03-15", Loc: "Publication date"}, "params.total": {V: 1.8e12, Loc: "Parameters"}}),
			row("epoch-ai", "SWE 1.6", "SWE 1.6", "model", map[string]string{"epoch": "SWE 1.6"}, nil),
			row("epoch-ai", "SWE-1.6", "SWE-1.6", "model", map[string]string{"epoch": "SWE-1.6"}, nil),
			row("epoch-ai", "Llama 2", "Llama 2", "model", map[string]string{"epoch": "Llama 2"},
				map[string]harvest.Fact{"params.total": {V: 70000000000, Loc: "Parameters"}}),
		},
		"wikidata": {
			row("wikidata", "Q1", "GPT 4", "model", map[string]string{"wikidata": "q1"}, map[string]harvest.Fact{"inception": {V: "2023-03-14"}}),
			row("wikidata", "Q2", "GPT-4", "concept", map[string]string{"wikidata": "Q2"}, nil),
			row("wikidata", "Q3", "Dropout", "method", map[string]string{"wikidata": "Q3", "arxiv": "1207.0580"}, nil),
		},
		"arxiv": {
			row("arxiv", "1207.0580", "Improving neural networks by preventing co-adaptation", "work", map[string]string{"arxiv": "1207.0580v1", "doi": "10.1/X"}, nil),
		},
		"pwc-dump": {
			row("pwc-dump", "method/dropout", "Dropout", "method", map[string]string{"pwc": "method/dropout"}, nil),
		},
	}
	for s, rows := range inv {
		if err := harvest.WriteInventory(root, s, rows); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func byID(t *testing.T, root string) map[string]*Item {
	t.Helper()
	items, err := Read(root)
	if err != nil {
		t.Fatal(err)
	}
	m := map[string]*Item{}
	for _, it := range items {
		m[it.ID] = it
	}
	return m
}

func TestMerge(t *testing.T) {
	root := tree(t)
	rep, err := Merge(root, "2026-11-13")
	if err != nil {
		t.Fatal(err)
	}
	if rep.Rows != 9 || rep.Items != 7 || rep.New != 7 {
		t.Errorf("rows %d, items %d, new %d; want 9, 7, 7", rep.Rows, rep.Items, rep.New)
	}
	m := byID(t, root)
	gpt := m["u:epoch:GPT-4"]
	if gpt == nil {
		t.Fatalf("no item u:epoch:GPT-4 in %v", m)
	}
	// The name join puts the Wikidata model with the Epoch model, but not the Wikidata concept of the same name.
	if strings.Join(gpt.Sources, " ") != "epoch-ai:GPT-4 wikidata:Q1" || gpt.SameAs["wikidata"] != "Q1" {
		t.Errorf("GPT-4 has the sources %v and the ids %v", gpt.Sources, gpt.SameAs)
	}
	if gpt.Outcome != "card" || gpt.DecidedBy != "rule:S-M1" || gpt.Target != "ai://NEW-gpt-4" || gpt.Decided != "2026-11-13" ||
		gpt.Reason != "S-M1: Each model in the database of Epoch AI is a card." {
		t.Errorf("GPT-4 is decided %+v", gpt)
	}
	if gpt.Added != "2026-11-13" {
		t.Errorf("GPT-4 was added on %q", gpt.Added)
	}
	if m["u:wikidata:Q2"] == nil || m["u:wikidata:Q2"].Kind != "concept" || m["u:wikidata:Q2"].Outcome != "pending" {
		t.Errorf("the concept GPT-4 is %+v", m["u:wikidata:Q2"])
	}
	// The arXiv id joins the work across versions; a method and a work of one id stay apart.
	if w := m["u:arxiv:1207.0580"]; w == nil || w.Outcome != "pending" {
		t.Errorf("the work is %+v; no signal of it meets the thresholds of S-W1", w)
	}
	if d := m["u:wikidata:Q3"]; d == nil || strings.Join(d.Sources, " ") != "pwc-dump:method/dropout wikidata:Q3" {
		t.Errorf("dropout is %+v", d)
	}
	// Two rows of one source are two items, and the curator sees the conflict.
	if m["u:epoch:SWE 1.6"] == nil || m["u:epoch:SWE-1.6"] == nil {
		t.Errorf("the two SWE rows are not two items")
	}
	want := map[string]bool{"the epoch ids differ: SWE 1.6 and SWE-1.6": false, "the kinds differ: work and method": false}
	for _, c := range rep.Sample {
		if _, ok := want[c.Why]; ok {
			want[c.Why] = true
		}
	}
	for why, seen := range want {
		if !seen {
			t.Errorf("no conflict %q in %+v", why, rep.Sample)
		}
	}

	// A second merge changes nothing, and a decision of a person stays.
	gpt.Outcome, gpt.DecidedBy, gpt.Target = "node", "person:tamnd", "GPT4NO"
	items := make([]*Item, 0, len(m))
	for _, it := range m {
		items = append(items, it)
	}
	if err := Write(root, items); err != nil {
		t.Fatal(err)
	}
	rep, err = Merge(root, "2026-11-14")
	if err != nil {
		t.Fatal(err)
	}
	if rep.New != 0 || rep.Items != 7 || len(rep.Decided) != 0 {
		t.Errorf("the second merge: %+v", rep)
	}
	if g := byID(t, root)["u:epoch:GPT-4"]; g.Outcome != "node" || g.DecidedBy != "person:tamnd" {
		t.Errorf("the decision of a person is lost: %+v", g)
	}
	before, _ := os.ReadFile(filepath.Join(root, Dir, "_.jsonl"))
	rep, err = Merge(root, "2026-11-15")
	if err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(filepath.Join(root, Dir, "_.jsonl"))
	if rep.Unchanged != rep.Items || !bytes.Equal(before, after) {
		t.Errorf("a merge with no new rows is not stable: %+v", rep)
	}
}

func TestNorm(t *testing.T) {
	for _, c := range []struct{ k, v, want string }{
		{"doi", "https://doi.org/10.1000/ABC", "10.1000/abc"},
		{"doi", "doi:10.1/X", "10.1/x"},
		{"arxiv", "arXiv:1706.03762v7", "1706.03762"},
		{"wikidata", "q42", "Q42"},
		{"epoch", "GPT-4", "GPT-4"},
	} {
		if got := NormID(c.k, c.v); got != c.want {
			t.Errorf("NormID(%q, %q) = %q, want %q", c.k, c.v, got, c.want)
		}
	}
	if got := NormName("Qwen 3.6-27B"); got != "qwen3627b" {
		t.Errorf("NormName = %q", got)
	}
}

const handCard = `{"tag": "NEW-llama-2", "kind": "model", "slug": "llama-2", "name": "Llama 2", "params": {"total": {"v": 6.9e10, "src": "S:meta-paper"}}}`
const otherCard = `{"kind":"model","tag":"NEW-other","slug":"other","name":"Other"}`

func TestSync(t *testing.T) {
	root := tree(t)
	if _, err := Merge(root, "2026-11-13"); err != nil {
		t.Fatal(err)
	}
	shard := filepath.Join(root, "cards", "model", "_.jsonl")
	if err := os.MkdirAll(filepath.Dir(shard), 0o755); err != nil {
		t.Fatal(err)
	}
	// The card that a person wrote for Llama 2 has the fact of another source and no same_as yet.
	if err := os.WriteFile(shard, []byte(handCard+"\n"+otherCard+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rep, err := Sync(root, nil, nil, "2026-11-13", SyncOptions{Kind: "model", Source: "wikidata"})
	if err != nil || rep.Items != 1 || rep.Created != 1 {
		t.Fatalf("the sync of the Wikidata models: %+v, %v", rep, err)
	}
	rep, err = Sync(root, nil, nil, "2026-11-13", SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Items != 4 || rep.Created != 2 || rep.Updated != 1 || rep.Unchanged != 1 {
		t.Errorf("sync: %+v", rep)
	}
	cards := map[string]map[string]any{}
	raw := map[string]string{}
	b, _ := os.ReadFile(shard)
	for _, ln := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		var c map[string]any
		if err := json.Unmarshal([]byte(ln), &c); err != nil {
			t.Fatal(err)
		}
		cards[c["slug"].(string)] = c
		raw[c["slug"].(string)] = ln
	}
	if raw["other"] != otherCard {
		t.Errorf("an untouched card changed:\n%s", raw["other"])
	}
	if !strings.HasPrefix(raw["gpt-4"], `{"tag":"NEW-gpt-4","kind":"model","slug":"gpt-4","name":"GPT-4","aliases":["GPT 4"],"inception":`) {
		t.Errorf("the GPT-4 card is not in canonical order:\n%s", raw["gpt-4"])
	}
	g := cards["gpt-4"]
	if f := g["released"].(map[string]any); f["v"] != "2023-03-15" || f["src"] != "S:epoch-ai" || f["loc"] != "Publication date" {
		t.Errorf("released is %v", f)
	}
	if f := g["inception"].(map[string]any); f["src"] != "S:wikidata" {
		t.Errorf("inception is %v", f)
	}
	if h, ok := g["harvest"].([]any); !ok || len(h) != 2 {
		t.Errorf("the harvest records are %v", g["harvest"])
	}
	// The fact of the person stays, and the sync counts the disagreement.
	l := cards["llama-2"]
	if v := l["params"].(map[string]any)["total"].(map[string]any); v["src"] != "S:meta-paper" || rep.Disagree != 1 {
		t.Errorf("the Llama fact is %v, %d disagreements", v, rep.Disagree)
	}
	if l["same_as"].(map[string]any)["epoch"] != "Llama 2" {
		t.Errorf("the Llama card has no epoch id: %v", l)
	}
	// Two items of one name have two cards.
	if cards["swe-1-6"] == nil || cards["swe-1-6-model"] == nil {
		t.Errorf("the SWE cards are %v", cards)
	}
	if it := byID(t, root)["u:epoch:Llama 2"]; it.Target != "ai://NEW-llama-2" {
		t.Errorf("the Llama item has the target %q", it.Target)
	}

	// A second sync changes nothing.
	rep, err = Sync(root, nil, nil, "2026-11-14", SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(shard)
	if rep.Unchanged != 4 || rep.Added+rep.Changed != 0 || !bytes.Equal(b, after) {
		t.Errorf("the second sync: %+v", rep)
	}

	// The curator makes a card of the pending work and merges the pending concept into the method.
	cur := []Curated{
		{Slug: "dropout-paper", Kind: "work", Name: "Improving neural networks", Universe: "u:arxiv:1207.0580", Outcome: "card", DecidedBy: "curator:x@1"},
		{Slug: "dropout-card", Kind: "method", Name: "Dropout", Universe: "u:wikidata:Q3", Merged: []string{"u:wikidata:Q2", "u:epoch:GPT-4"}, DecidedBy: "curator:x@1"},
		{Slug: "nameless", Kind: "nope"},
	}
	rep, err = Sync(root, nil, cur, "2026-11-14", SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Created != 2 || rep.Merged != 1 || len(rep.Notes) != 2 {
		t.Errorf("the curated sync: %+v", rep)
	}
	m := byID(t, root)
	if w := m["u:arxiv:1207.0580"]; w.Outcome != "card" || w.Target != "ai://NEW-dropout-paper" || w.DecidedBy != "curator:x@1" {
		t.Errorf("the work is %+v", w)
	}
	if q := m["u:wikidata:Q2"]; q.Outcome != "merged" || q.Target != "u:wikidata:Q3" {
		t.Errorf("the merged concept is %+v", q)
	}
	if g := m["u:epoch:GPT-4"]; g.Outcome != "card" {
		t.Errorf("a card item is merged by the sync: %+v", g)
	}
	w, _ := os.ReadFile(filepath.Join(root, "cards", "work", "_.jsonl"))
	if !strings.Contains(string(w), `"same_as":{"arxiv":"1207.0580","doi":"10.1/x"}`) {
		t.Errorf("the work card is %s", w)
	}
}

func TestRetarget(t *testing.T) {
	root := tree(t)
	if _, err := Merge(root, "2026-11-13"); err != nil {
		t.Fatal(err)
	}
	if err := Retarget(root, map[string]string{"NEW-gpt-4": "ABC123"}); err != nil {
		t.Fatal(err)
	}
	if g := byID(t, root)["u:epoch:GPT-4"]; g.Target != "ai://ABC123" {
		t.Errorf("the target is %q", g.Target)
	}
}

func TestThresholds(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "registers"), 0o755); err != nil {
		t.Fatal(err)
	}
	rules := `- id: S-M2
  kind: model
  outcome: card
  rule: A Hub repository is a card when it has 10,000 downloads or 100 likes.
  source: huggingface
  thresholds: {downloads_30d: 10000, likes: 100}
- id: S-D1
  kind: dataset
  outcome: card
  rule: A dataset is a card when it has 1,000 downloads or 50 likes.
  thresholds: {downloads_30d: 1000, likes: 50}
- id: S-S1
  kind: software
  outcome: card
  rule: A repository with 5,000 stars is a card.
  source: github
  thresholds: {stars: 5000}
`
	if err := os.WriteFile(filepath.Join(root, ScopeFile), []byte(rules), 0o644); err != nil {
		t.Fatal(err)
	}
	sig := func(r harvest.Row, s map[string]any) harvest.Row { r.Signals = s; return r }
	inv := map[string][]harvest.Row{
		"huggingface": {
			sig(row("huggingface", "acme/big", "acme/big", "model", map[string]string{"hub": "acme/big"}, nil), map[string]any{"downloads_30d": 20000, "likes": 3}),
			sig(row("huggingface", "acme/small", "acme/small", "model", map[string]string{"hub": "acme/small"}, nil), map[string]any{"downloads_30d": 50, "likes": 5}),
			sig(row("huggingface", "datasets/acme/liked", "acme/liked", "dataset", map[string]string{"hub_dataset": "acme/liked"}, nil), map[string]any{"downloads_30d": 10, "likes": 60}),
		},
		"github": {
			sig(row("github", "Acme/Tool", "Acme/Tool", "software", map[string]string{"github": "Acme/Tool"}, nil), map[string]any{"stars": 6000}),
		},
		// A software row of another source with stars meets S-S1, but has no row of github.
		"wikidata": {
			sig(row("wikidata", "Q9", "Other tool", "software", map[string]string{"wikidata": "Q9"}, nil), map[string]any{"stars": 9000}),
		},
	}
	for s, rows := range inv {
		if err := harvest.WriteInventory(root, s, rows); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Merge(root, "2026-11-14"); err != nil {
		t.Fatal(err)
	}
	m := byID(t, root)
	for id, want := range map[string]string{
		"u:hub:acme/big":           "rule:S-M2",
		"u:hub:acme/small":         "",
		"u:hub_dataset:acme/liked": "rule:S-D1",
		"u:github:acme/tool":       "rule:S-S1",
		"u:wikidata:Q9":            "",
	} {
		it := m[id]
		if it == nil {
			t.Errorf("no item %s in %v", id, m)
			continue
		}
		if it.DecidedBy != want || (want == "") != (it.Outcome == "pending") {
			t.Errorf("%s is decided by %q with %s, want %q", id, it.DecidedBy, it.Outcome, want)
		}
	}
	// The signals read back from JSON are float64.
	r := Rule{Thresholds: map[string]any{"citation_percentile": 98}}
	if !r.Meets(map[string]any{"citation_percentile": 98.0}) || r.Meets(map[string]any{"citation_percentile": 97.99}) || r.Meets(map[string]any{"citation_percentile": "99"}) {
		t.Error("Meets compares numbers wrongly")
	}
}
