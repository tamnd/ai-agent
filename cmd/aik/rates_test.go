// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/tamnd/ai-agent/quality"
)

func TestReportRates(t *testing.T) {
	dir := t.TempDir()
	st := `{"slug": "comparison-model", "status": "done", "history": [{"role": "write", "run": "r1"}, {"role": "fix-gate", "run": "r1"},
		{"role": "audit", "run": "r1"}], "audits": [{"verdict": "pass", "run": "r1"}]}`
	if err := os.WriteFile(filepath.Join(dir, "comparison-model.json"), []byte(st), 0o644); err != nil {
		t.Fatal(err)
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	code := run([]string{"report", "rates", "-root", corpus, "-state", dir, "-run", "r1", "-json"})
	os.Stdout = old
	w.Close()
	out, _ := io.ReadAll(r)
	if code != 0 {
		t.Fatalf("aik report rates exits with %d: %s", code, out)
	}
	var got quality.Rates
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if len(got.Kinds) != 1 || got.Kinds[0].Kind == "unknown" || got.Total.GateFirst != (quality.Rate{Pass: 0, Of: 1}) ||
		got.Total.AuditPass != (quality.Rate{Pass: 1, Of: 1}) || got.Total.Done != (quality.Rate{Pass: 1, Of: 1}) {
		t.Errorf("rates = %+v", got)
	}
	if code := run([]string{"report", "rates", "-root", corpus, "-state", dir}); code != 0 {
		t.Errorf("the table exits with %d", code)
	}
}
