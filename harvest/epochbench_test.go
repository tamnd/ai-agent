// SPDX-License-Identifier: Apache-2.0

package harvest

import (
	"archive/zip"
	"bytes"
	"testing"
)

func epochZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, s := range files {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte(s)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestEpochBenchRows(t *testing.T) {
	b := epochZip(t, map[string]string{
		"README.md": "CC BY 4.0",
		"benchmark_metadata.csv": "benchmark,in_eci,source_file,score_column,scale,random_baseline,score_ceiling,release_date,superseded_by\n" +
			"GPQA diamond,True,gpqa_diamond.csv,Best score (across scorers),1.0,0.25,0.97,2023-11-20,\n" +
			"Aider polyglot,True,aider_polyglot_external.csv,Percent correct,0.01,0,1,2024-12-21,\n" +
			"Writing,False,writing_external.csv,Score,0.1,,,,\n" +
			"HLE,False,,,1.0,,,,\n" +
			"Gone,True,gone.csv,Score,1.0,,,,\n",
		"model_metadata.csv": "model_version,model_group,date,display_name,organization,country,accessibility,training_compute_flop\n" +
			"claude-opus-4-8_max,Claude Opus 4.8,2026-05-28,Claude Opus 4.8,Anthropic,,API access,\n" +
			"gemini-x_32K,Gemini X,2026-01-01,Gemini X,Google,,API access,\n" +
			"old-model,Old Model,2024-01-01,Old Model,Lab,,,\n",
		"gpqa_diamond.csv": "Model version,mean_score,Best score (across scorers),Release date,Organization,Country,Training compute (FLOP),Training compute notes,stderr,Log viewer,Logs,Started at,id\n" +
			"claude-opus-4-8_max,0.87,0.875,2026-05-28,Anthropic,,,,0.016,https://logs.epoch.ai/v?x=1,,2026-06-08T00:20:40.000Z,RUN1\n" +
			"gemini-x_32K,0.5,0.5,2026-01-01,Google,,,,0.02,,,2025-12-01T00:00:00.000Z,RUN2\n" +
			"unmapped-model,0.4,0.4,2026-01-01,X,,,,,,,2026-02-01T00:00:00.000Z,RUN3\n" +
			"old-model,,,2024-01-01,Lab,,,,,,,2024-02-01T00:00:00.000Z,RUN4\n",
		"aider_polyglot_external.csv": "Model version,Percent correct,Release date,Organization,Country,Training compute (FLOP),Training compute notes,Name,Shots,Source,Source link,Notes,id\n" +
			"old-model,61.5,2024-01-01,Lab,,,,Old Model,5,Aider leaderboard,https://aider.chat/docs/leaderboards/,whole edit format,\n" +
			"old-model,60.0,2024-01-01,Lab,,,,Old Model,0-shot,Some paper,https://arxiv.org/abs/2401.00001v2,,\n" +
			",12.0,2024-01-01,Lab,,,,Nameless Model,few,https://example.org/board,,,\n",
		"writing_external.csv": "Model version,Score\nold-model,7.5\n",
		"stray_external.csv":   "Model version,Score\nold-model,0.5\n",
	})
	rows, skipped, err := EpochBenchRows(b, "b3:snap", "2026-11-14", Options{})
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]Row{}
	for _, r := range rows {
		by[r.ID] = r
	}
	if len(rows) != 5 {
		t.Fatalf("%d rows, want 5: %v", len(rows), by)
	}
	for why, n := range map[string]int{
		"rows with no score": 1, "rows whose model version has no model in model_metadata.csv": 1,
		"benchmarks whose scale is not a fraction or a percentage": 1, "benchmarks whose file is not in the zip": 1,
		"files that benchmark_metadata.csv does not map to a benchmark": 1,
	} {
		if skipped[why] != n {
			t.Errorf("skipped %q: %d, want %d (%v)", why, skipped[why], n, skipped)
		}
	}
	run := by["gpqa_diamond/RUN1"]
	f, set, src := run.Fields, run.Fields["setting"].(map[string]any), run.Fields["source"].(map[string]any)
	if f["model"] != "Claude Opus 4.8" || f["benchmark_name"] != "GPQA diamond" ||
		f["metric_name"] != "GPQA diamond: Best score (across scorers)" {
		t.Errorf("run fields %v", f)
	}
	if sc := f["score"].(map[string]any); sc["v"] != 0.875 || sc["unit"] != "fraction" {
		t.Errorf("run score %v", sc)
	}
	if set["cot"] != "reasoning-model (effort max)" || set["date_run"] != "2026-06-08" ||
		set["notes"] != "Epoch AI model version claude-opus-4-8_max; standard error 0.016" {
		t.Errorf("run setting %v", set)
	}
	if src["src"] != EpochHub || src["date"] != "2026-06-08" || f["original"] != nil || run.URL != "https://logs.epoch.ai/v?x=1" {
		t.Errorf("run source %v, original %v, url %s", src, f["original"], run.URL)
	}
	if c := by["gpqa_diamond/RUN2"].Fields["setting"].(map[string]any)["cot"]; c != "reasoning-model (budget 32K tokens)" {
		t.Errorf("budget cot %v", c)
	}
	// A copied score: the hub is the source as of the snapshot, the leaderboard is the original.
	ext := by["aider_polyglot/old-model"]
	if sc := ext.Fields["score"].(map[string]any); sc["v"] != 61.5 || sc["unit"] != "percent" {
		t.Errorf("external score %v", sc)
	}
	if o := ext.Fields["original"].(map[string]any); o["name"] != "Aider leaderboard" || o["src"] != "https://aider.chat/docs/leaderboards/" {
		t.Errorf("external original %v", o)
	}
	if s := ext.Fields["setting"].(map[string]any); s["shots"] != 5 || s["harness"] != nil ||
		s["notes"] != "Epoch AI model version old-model; whole edit format" {
		t.Errorf("external setting %v", s)
	}
	if d := ext.Fields["source"].(map[string]any)["date"]; d != "2026-11-14" {
		t.Errorf("external source date %v, want the date of the snapshot", d)
	}
	paper := by["aider_polyglot/old-model~2"]
	if o := paper.Fields["original"].(map[string]any); o["src"] != "arxiv:2401.00001" ||
		paper.Fields["setting"].(map[string]any)["shots"] != 0 {
		t.Errorf("paper row %v", paper.Fields)
	}
	nameless := by["aider_polyglot/nameless-model"]
	if o := nameless.Fields["original"].(map[string]any); o["src"] != "https://example.org/board" || o["name"] != nil ||
		nameless.Fields["setting"].(map[string]any)["shots"] != nil {
		t.Errorf("nameless row %v", nameless.Fields)
	}
	since, _, _ := EpochBenchRows(b, "b3:snap", "2026-11-14", Options{Since: "2026-01-01"})
	if len(since) != 4 {
		t.Errorf("-since 2026: %d rows, want 4 (RUN1 and the three copied scores)", len(since))
	}
	if _, _, err := EpochBenchRows([]byte("not a zip"), "b3:x", "2026-11-14", Options{}); err == nil {
		t.Error("no error for data that is not a zip")
	}
	if _, _, err := EpochBenchRows(epochZip(t, map[string]string{"a.csv": "x\n"}), "b3:x", "2026-11-14", Options{}); err == nil {
		t.Error("no error for a zip with no metadata")
	}
}
