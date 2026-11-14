// SPDX-License-Identifier: Apache-2.0

package site

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tamnd/ai-agent/node"
	"github.com/tamnd/ai-agent/quality"
)

func TestDashboards(t *testing.T) {
	root := filepath.Join(t.TempDir(), "corpus")
	if err := os.CopyFS(root, os.DirFS("../testdata/corpus")); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "nodes", "foundations", "models-of-computation", "toy-model")
	md := "---\ntag: 7TQYMD\nkind: model\nslug: toy-model\nname: Toy model\narea: foundations/models-of-computation\nlevel: intro\nsummary: A small model.\n---\n\n## Overview\n\nThe toy model fits the data {#c-fits}. A toy is a small model {#c-toy}.\n"
	cy := "- id: c-fits\n  type: empirical\n  text: The toy model fits the data.\n  review_after: 2026-01-01\n  evidence:\n    - {work: paper, role: original, stance: for, locator: \"Table 2\"}\n- id: c-toy\n  type: definitional\n  text: A toy is a small model.\n  evidence:\n    - {work: paper, role: original, stance: for}\n"
	files := map[string]string{
		filepath.Join(dir, "node.md"):                           md,
		filepath.Join(dir, "claims.yaml"):                       cy,
		filepath.Join(root, "registers", "waivers.yaml"):        "- {rule: W01, item: 7TQYMD, end: 2027-01-01}\n",
		filepath.Join(root, "reports", "audit.json"):            `{"links":{"checked":4,"dead":[{"url":"https://example.org/gone","file":"x"}]}}`,
		filepath.Join(root, "registers", "universe", "a.jsonl"): `{"id":"U1","name":"Toy model","kind":"model","area":"foundations","outcome":"node","target":"ai://7TQYMD","core":true}` + "\n",
	}
	for p, s := range files {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	c, err := node.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "site")
	if _, err := Build(c, Options{Out: out, Version: "test", Now: time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatal(err)
	}
	read := func(p string) string {
		b, err := os.ReadFile(filepath.Join(out, p))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	var r quality.Report
	if err := json.Unmarshal([]byte(read("about/quality.json")), &r); err != nil {
		t.Fatal(err)
	}
	by := map[string]quality.Metric{}
	for _, m := range r.Metrics {
		by[m.ID] = m
	}
	if r.Date != "2026-11-15" || r.Core != 1 || len(r.Metrics) != 14 {
		t.Fatalf("report %s, %d core, %d metrics", r.Date, r.Core, len(r.Metrics))
	}
	for id, want := range map[string][2]int{"g2": {0, 2}, "headline": {0, 2}, "stale": {1, 2}, "waivers": {1, len(c.Nodes)}, "dead-links": {1, 4}} {
		m := by[id]
		if m.Value == nil || m.Num != want[0] || m.Den != want[1] {
			t.Errorf("%s: %d/%d (%v), want %d/%d", id, m.Num, m.Den, m.NotComputed, want[0], want[1])
		}
	}
	if m := by["audit-first-pass"]; m.Value != nil {
		t.Errorf("the build computed the audit first-pass rate without the writer state: %+v", m)
	}
	page := read("about/quality/index.html")
	for _, want := range []string{"<title>Quality", `id="g2"`, "not computed: ", `class="num miss"`, "under 0.5%"} {
		if !strings.Contains(page, want) {
			t.Errorf("/about/quality/ does not contain %q", want)
		}
	}
	ev := read("about/evidence/index.html")
	for _, want := range []string{"<title>Evidence", "Claims by area and headline label", "All areas", `title="2 of 2"`} {
		if !strings.Contains(ev, want) {
			t.Errorf("/about/evidence/ does not contain %q", want)
		}
	}

	// A report of aik report quality takes the place of the metrics that the build computes.
	given := filepath.Join(t.TempDir(), "quality.json")
	v := 0.5
	data, _ := json.Marshal(quality.Report{Date: "2026-11-09", Metrics: []quality.Metric{{ID: "audit-first-pass", Name: "audit first-pass rate", Unit: quality.Share, Num: 1, Den: 2, Value: &v}}})
	if err := os.WriteFile(given, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(c, Options{Out: out, Version: "test", Quality: given, Now: time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatal(err)
	}
	if page := read("about/quality/index.html"); !strings.Contains(page, "Computed on 2026-11-09") || !strings.Contains(page, "50%") {
		t.Error("/about/quality/ does not show the given report")
	}
	// A missing report falls back to the build's own metrics.
	if _, err := Build(c, Options{Out: out, Version: "test", Quality: filepath.Join(t.TempDir(), "none.json"), Now: time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatal(err)
	}
	if page := read("about/quality/index.html"); !strings.Contains(page, "Computed on 2026-11-15") {
		t.Error("a missing report does not fall back to the computed metrics")
	}
}
