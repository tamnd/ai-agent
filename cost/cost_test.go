// SPDX-License-Identifier: Apache-2.0

package cost

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const events = `{"t": "2026-11-14T02:26:40", "job": "write", "slug": "q-learning", "role": "write", "backend": "codex", "model": "gpt-6-luna", "ok": true, "secs": 10, "cost": null, "tokens": 1000000}
{"t": "2026-11-14T02:27:40", "job": "write", "slug": "q-learning", "role": "audit", "backend": "claude", "model": "claude-sonnet-5-5", "ok": true, "secs": 5, "cost": 1.5, "tokens_in": 1000, "tokens_out": 200}
{"t": "2026-11-14T02:28:40", "job": "write", "slug": "q-learning", "blocked": "budget"}
{"t": "2027-01-02T09:00:00", "job": "verify", "slug": "dropout", "role": "verifier", "backend": "codex", "model": "gpt-6-sol", "ok": false, "secs": 3, "cost": null, "limited": true, "tokens_in": 100, "tokens_out": 10}

{"t": "2026-11-01T00:00:00", "job": "catalog", "role": "", "backend": "claude", "model": "claude-opus-5-5", "ok": true, "cost": 0.25}
`

func TestMilestoneOf(t *testing.T) {
	for day, want := range map[string]string{
		"2026-11-12": "before M0", "2026-11-13": "M0", "2026-12-31": "M0", "2027-01-01": "M1",
		"2027-07-31": "M2", "2027-08-01": "M3", "2028-12-31": "M5", "2029-01-01": "after M5",
	} {
		if got := MilestoneOf(day); got != want {
			t.Errorf("MilestoneOf(%s) = %s, want %s", day, got, want)
		}
	}
}

func TestMake(t *testing.T) {
	calls, err := Read(strings.NewReader(events))
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 4 {
		t.Fatalf("%d calls, want 4 (the line without a backend is not a call)", len(calls))
	}
	if calls[3].Role != "catalog" {
		t.Errorf("a call with no role takes its job: %q", calls[3].Role)
	}
	r := Make(calls, nil)
	if r.From != "2026-11-01" || r.To != "2027-01-02" {
		t.Errorf("window %s to %s", r.From, r.To)
	}
	if r.Total.Calls != 4 || r.Total.Unpriced != 2 || r.Total.Failed != 1 || r.Total.Limited != 1 || math.Abs(r.Total.Metered-1.75) > 1e-9 {
		t.Errorf("total = %+v", r.Total)
	}
	if r.Total.Tokens != 1000000+1200+110 {
		t.Errorf("tokens = %d", r.Total.Tokens)
	}
	var keys []string
	for _, m := range r.Milestones {
		keys = append(keys, m.Key)
	}
	if strings.Join(keys, ",") != "before M0,M0,M1" {
		t.Errorf("milestones = %v", keys)
	}
	if r.Milestones[1].Roles[0].Key != "audit" || r.Milestones[1].Roles[1].Key != "write" {
		t.Errorf("roles of M0 = %+v", r.Milestones[1].Roles)
	}
	if r.Written != 1 || math.Abs(r.PerItem-1.5) > 1e-9 {
		t.Errorf("per item = %v over %d", r.PerItem, r.Written)
	}
	if r.Models[0].Key != "claude-sonnet-5-5" || r.Models[len(r.Models)-1].Key != "codex:gpt-6-sol" {
		t.Errorf("models = %+v", r.Models)
	}

	f := filepath.Join(t.TempDir(), "prices.yaml")
	if err := os.WriteFile(f, []byte("models:\n  gpt-6-luna: {total: 2}\n  gpt-6-sol: {in: 1000, out: 10000}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := LoadPrices(f)
	if err != nil {
		t.Fatal(err)
	}
	r = Make(calls, p)
	if r.Total.Unpriced != 0 || math.Abs(r.Total.Estimated-(2+0.1+0.1)) > 1e-9 {
		t.Errorf("with prices: %+v", r.Total)
	}
	if math.Abs(r.PerItem-3.5) > 1e-9 {
		t.Errorf("per item with prices = %v", r.PerItem)
	}
}

func TestReadError(t *testing.T) {
	if _, err := Read(strings.NewReader("{\"t\": 1\n")); err == nil || !strings.Contains(err.Error(), "line 1") {
		t.Errorf("err = %v", err)
	}
}
