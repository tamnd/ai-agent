// SPDX-License-Identifier: Apache-2.0

package quality

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRates(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		// An old state file: the gate runs come from the history. The first gate failed (a fix-gate follows it).
		"q.json": `{"slug": "q", "status": "failed", "history": [
			{"role": "write", "run": "r1"}, {"role": "fix-gate", "run": "r1"}, {"role": "audit", "run": "r1"},
			{"role": "fix-audit", "run": "r1"}, {"role": "audit", "run": "r2"}],
			"audits": [{"verdict": "fail", "run": "r1"}, {"verdict": "fail", "run": "r2"}]}`,
		// A new state file with the gate runs recorded: the first gate and the first audit passed.
		"d.json": `{"slug": "d", "status": "done", "history": [{"role": "write", "run": "r2"}, {"role": "audit", "run": "r2"}],
			"gates": [{"ok": true, "run": "r2"}, {"ok": true, "run": "r2"}], "audits": [{"verdict": "pass", "run": "r2"}]}`,
		// Blocked before any gate: an item with no rate.
		"b.json": `{"slug": "b", "kind": "metric", "status": "blocked", "history": [{"role": "write", "run": "r1"}]}`,
		// No kind in the tree.
		"x.json": `{"status": "done", "history": [{"role": "write", "run": "r1"}, {"role": "audit", "run": "r1"}],
			"audits": [{"verdict": "pass", "run": "r1"}]}`,
	}
	for name, s := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	items, err := LoadItems(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 4 {
		t.Fatalf("items = %d, want 4", len(items))
	}
	for _, it := range items {
		if it.Slug == "q" && (len(it.Gates) != 3 || it.Gates[0].Pass || !it.Gates[1].Pass || it.Gates[2].Run != "r2") {
			t.Errorf("q gates = %+v, want fail, pass, pass in r2", it.Gates)
		}
		if it.Slug == "x" && it.Status != "done" {
			t.Errorf("x: the slug comes from the file name; got %+v", it)
		}
	}
	kinds := map[string]string{"q": "method", "d": "method", "b": "concept"} // the tree wins over the state file

	r := ComputeRates(items, kinds, "")
	if r.Total.Items != 4 || r.Total.GateFirst != (Rate{2, 3}) || r.Total.AuditPass != (Rate{2, 3}) || r.Total.Done != (Rate{2, 3}) {
		t.Errorf("total = %+v", r.Total)
	}
	if len(r.Kinds) != 3 || r.Kinds[0].Kind != "concept" || r.Kinds[1].Kind != "method" || r.Kinds[2].Kind != "unknown" {
		t.Fatalf("kinds = %+v", r.Kinds)
	}
	m := r.Kinds[1]
	if m.Items != 2 || m.GateFirst != (Rate{1, 2}) || m.AuditPass != (Rate{1, 2}) || m.Done != (Rate{1, 2}) {
		t.Errorf("method = %+v", m)
	}
	if c := r.Kinds[0]; c.Items != 1 || c.GateFirst.Of != 0 || c.AuditPass.Of != 0 {
		t.Errorf("concept = %+v", c)
	}

	// The rates of run r2: q counts by its second audit only, and its audit in r2 says its gate in r2 passed.
	r2 := ComputeRates(items, kinds, "r2")
	if r2.Total.Items != 2 || r2.Total.GateFirst != (Rate{2, 2}) || r2.Total.AuditPass != (Rate{1, 2}) {
		t.Errorf("run r2 total = %+v", r2.Total)
	}
	if got := (Rate{1, 4}).Share(); got != 0.25 {
		t.Errorf("share = %v", got)
	}
	if got := (Rate{}).Share(); got != 0 {
		t.Errorf("share of nothing = %v", got)
	}
	if none, err := LoadItems(filepath.Join(dir, "none")); err != nil || len(none) != 0 {
		t.Errorf("a missing directory: %v, %v", none, err)
	}
}
