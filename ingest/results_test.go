// SPDX-License-Identifier: Apache-2.0

package ingest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/ai-agent/card"
	"github.com/tamnd/ai-agent/cite"
	"github.com/tamnd/ai-agent/harvest"
	"github.com/tamnd/ai-agent/node"
	"github.com/tamnd/ai-agent/results"
)

func TestNameKey(t *testing.T) {
	for _, c := range [][2]string{
		{"Claude 4.5 Opus", "claude-opus-4-5"},
		{"GPT 5.2", "GPT-5.2"},
		{"Qwen2.5-72B", "qwen 2.5 72b"},
	} {
		if NameKey(c[0]) != NameKey(c[1]) {
			t.Errorf("NameKey(%q) = %q, NameKey(%q) = %q; want one key", c[0], NameKey(c[0]), c[1], NameKey(c[1]))
		}
	}
	if NameKey("GPT-4o") == NameKey("GPT-4") {
		t.Error("GPT-4o and GPT-4 have one key")
	}
}

func write(t *testing.T, path, s string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

// snapshot is the leaderboard that the rows were read from.
const snapshot = `{"leaderboards":[{"name":"Verified","results":[
 {"name":"mini-SWE-agent + Claude 4.5 Opus (medium)","folder":"20251124_mini_opus","resolved":74.4},
 {"name":"Sonar","folder":"20250101_sonar","resolved":60.0},
 {"name":"Old","folder":"20240101_old","resolved":12.5}]}]}`

func resultRow(id, model string, v float64, snap string) harvest.Row {
	folder := id[strings.LastIndexByte(id, '/')+1:]
	return harvest.Row{Source: "swe-bench", ID: id, Name: model, Kind: "result", SameAs: map[string]string{"swe_bench": id},
		Snapshot: snap, Date: "2026-11-14",
		Fields: map[string]any{"model": model, "benchmark": "swe-bench", "version": "verified", "metric": "percent-resolved",
			"score":   map[string]any{"v": v, "unit": "percent"},
			"setting": map[string]any{"split": "test", "cot": "reasoning-model (effort medium)"},
			"source": map[string]any{"src": "S:swe-bench-leaderboard", "kind": "leaderboard", "date": "2025-11-24",
				"loc": "Verified leaderboard, entry " + folder + ", field resolved"}}}
}

func setup(t *testing.T) (string, cite.Store, string) {
	t.Helper()
	root := t.TempDir()
	write(t, filepath.Join(root, "topics", "areas.yaml"), "[]\n")
	write(t, filepath.Join(root, "registers", "languages.yaml"), "[]\n")
	write(t, filepath.Join(root, "registers", "reserved.yaml"), "[]\n")
	if err := os.MkdirAll(filepath.Join(root, "nodes"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(root, "cards", "model", "_.jsonl"),
		`{"tag":"AAAAA1","kind":"model","slug":"claude-opus-4-5","name":"Claude Opus 4.5"}`+"\n"+
			`{"tag":"AAAAA2","kind":"model","slug":"claude-3-5-sonnet","name":"Claude 3.5 Sonnet"}`+"\n"+
			`{"tag":"AAAAA3","kind":"model","slug":"gpt-4","name":"GPT-4"}`+"\n"+
			`{"tag":"AAAAA4","kind":"model","slug":"gpt-4-turbo","name":"GPT 4 Turbo","aliases":["GPT 4"]}`+"\n")
	write(t, filepath.Join(root, "cards", "benchmark", "_.jsonl"),
		`{"tag":"BBBBB1","kind":"benchmark","slug":"swe-bench","name":"SWE-bench","versions":[{"id":"verified","name":"SWE-bench Verified"}]}`+"\n")
	write(t, filepath.Join(root, "cards", "metric", "_.jsonl"),
		`{"tag":"CCCCC1","kind":"metric","slug":"percent-resolved","name":"Percent resolved"}`+"\n")
	write(t, filepath.Join(root, ModelsFile), "models:\n  Sonar: none\n  Old Opus: claude-opus-4-5\n  Bad: swe-bench\n")
	st := cite.Store{Dir: filepath.Join(root, "raw")}
	snap, err := st.Put([]byte(snapshot), cite.Meta{})
	if err != nil {
		t.Fatal(err)
	}
	return root, st, snap
}

func run(t *testing.T, root string, st cite.Store, rows []harvest.Row) *Report {
	t.Helper()
	if err := harvest.WriteInventory(root, "swe-bench", rows); err != nil {
		t.Fatal(err)
	}
	c, err := node.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	rep, err := Results(c, Options{Root: root, Store: st, Date: "2026-11-14", Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	return rep
}

func TestResults(t *testing.T) {
	root, st, snap := setup(t)
	opus := resultRow("verified/20251124_mini_opus", "Claude 4.5 Opus (medium)", 74.4, snap)
	dup := resultRow("verified/20251124_mini_opus", "Claude Opus 4.5 (medium)", 74.4, snap)
	dup.ID = "verified/copy"
	sonnet := resultRow("verified/20250101_x", "Claude 3.5 Sonnet (20241022)", 50, snap)
	old := resultRow("verified/20240101_old", "Old Opus", 12.5, snap)
	sonar := resultRow("verified/20250101_sonar", "Sonar", 60, snap)
	gpt := resultRow("verified/g", "GPT 4", 30, snap)
	mystery := resultRow("verified/m", "Mystery 9", 1, snap)
	bad := resultRow("verified/b", "Bad", 1, snap)
	lite := resultRow("lite2/l", "Claude Opus 4.5", 1, snap)
	lite.Fields["version"] = "lite2"
	empty := resultRow("verified/e", "Claude Opus 4.5", 1, snap)
	delete(empty.Fields, "setting")
	rep := run(t, root, st, []harvest.Row{opus, dup, sonnet, old, sonar, gpt, mystery, bad, lite, empty})
	if rep.Rows != 10 || rep.New != 3 || rep.Duplicates != 1 || rep.Skipped != 1 || rep.Unresolved != 5 {
		t.Fatalf("report %+v", rep)
	}
	if rep.Cite[results.CiteFound] != 2 || rep.Cite[results.CiteNotFound] != 1 {
		t.Errorf("cite %v; want 2 found (opus, old) and 1 not found (sonnet: 50 is not in the record)", rep.Cite)
	}
	want := map[string]string{"GPT 4": Ambiguous, "Mystery 9": NoModel, "Bad": BadCrosswalk, "swe-bench@lite2": NoVersion,
		"swe-bench:verified/e": NotResultRow}
	for _, m := range rep.Missing {
		if want[m.Name] != m.Reason {
			t.Errorf("miss %q: %q, want %q", m.Name, m.Reason, want[m.Name])
		}
		delete(want, m.Name)
	}
	if len(want) > 0 {
		t.Errorf("no miss for %v", want)
	}
	lines, err := card.ReadKind(filepath.Join(root, "cards", "result"))
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 3 {
		t.Fatalf("%d result cards, want 3", len(lines))
	}
	got := map[string]map[string]any{}
	for _, l := range lines {
		if !strings.HasPrefix(l.Card.Tag, "NEW-r") || l.Card.Owner != "swe-bench" {
			t.Errorf("card %s owner %q", l.Card.Tag, l.Card.Owner)
		}
		var f map[string]any
		b, _ := json.Marshal(l.Card.Fields)
		_ = json.Unmarshal(b, &f)
		got[f["harvest"].(map[string]any)["row"].(string)] = f
	}
	o := got["verified/20251124_mini_opus"]
	if o["model"] != "claude-opus-4-5" || o["model_variant"] != nil {
		t.Errorf("opus: model %v, variant %v; the effort is not a variant", o["model"], o["model_variant"])
	}
	if s := got["verified/20250101_x"]; s["model"] != "claude-3-5-sonnet" || s["model_variant"] != "20241022" {
		t.Errorf("sonnet: model %v, variant %v", s["model"], s["model_variant"])
	}
	c, err := node.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	shown := 0
	for _, r := range results.Load(c).Results {
		if r.Err != nil {
			t.Errorf("%s: %v", r.Tag, r.Err)
		}
		if r.Shown() {
			shown++
		}
	}
	if shown != 2 {
		t.Errorf("%d results shown, want the 2 that the citation check found", shown)
	}
	// A second run with the same rows changes nothing; a new score updates the card and keeps its tag.
	again := run(t, root, st, []harvest.Row{opus, sonnet, old})
	if again.Unchanged != 3 || again.New != 0 || again.Updated != 0 {
		t.Errorf("second run %+v", again)
	}
	opus.Fields["score"] = map[string]any{"v": 74.6, "unit": "percent"}
	third := run(t, root, st, []harvest.Row{opus})
	if third.Updated != 1 || third.Cite[results.CiteNotFound] != 1 {
		t.Errorf("third run %+v", third)
	}
	after, _ := card.ReadKind(filepath.Join(root, "cards", "result"))
	if len(after) != 3 {
		t.Errorf("%d cards after the update, want 3", len(after))
	}
}
