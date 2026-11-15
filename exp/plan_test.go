// SPDX-License-Identifier: Apache-2.0

package exp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const toyCompute = `provider: example-cloud
caps:
  - {from: 2026-11, usd: 100}
  - {from: 2027-07, usd: 250}
classes:
  gpu-small: {instance: g6.xlarge, gpu: L4, usd_per_hour: 0.8}
  gpu-medium: {instance: p5.xlarge, gpu: H100, usd_per_hour: 4}
`

func writeFile(t *testing.T, dir, name, s string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestReadCompute(t *testing.T) {
	dir := t.TempDir()
	c, err := ReadCompute(writeFile(t, dir, "c.yaml", toyCompute))
	if err != nil {
		t.Fatal(err)
	}
	for month, want := range map[string]float64{"2026-11": 100, "2027-06": 100, "2027-07": 250, "2030-01": 250} {
		if got, ok := c.CapOf(month); !ok || got != want {
			t.Errorf("the cap of %s is %v, not %v", month, got, want)
		}
	}
	if _, ok := c.CapOf("2026-10"); ok {
		t.Error("a month before the first cap has a cap")
	}
	bad := `caps: [{from: 2027-1, usd: -1}, {from: 2026-01, usd: 1}]
classes: {cpu-small: {instance: x, usd_per_hour: 1}, gpu-small: {instance: g}}
`
	_, err = ReadCompute(writeFile(t, dir, "bad.yaml", bad))
	for _, want := range []string{"YYYY-MM", "usd is 0 or more", "in the order", `"cpu-small" is not a GPU class`, "gpu-small has no usd_per_hour"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("the error %v has no %q", err, want)
		}
	}
	if _, err := ReadCompute(filepath.Join(dir, "none.yaml")); err == nil || !strings.Contains(err.Error(), "is not there") {
		t.Errorf("no register: %v", err)
	}
}

func TestReadLedger(t *testing.T) {
	dir := t.TempDir()
	l, err := ReadLedger(writeFile(t, dir, "l.jsonl", `{"batch":"b-1","date":"2026-11-02","class":"gpu-small","experiments":["a/x"],"gpu_hours":10,"usd":8,"runs":["r-1"]}

{"batch":"b-2","date":"2026-10-30","class":"gpu-small","experiments":["a/y"],"gpu_hours":5,"usd":4}
{"batch":"b-3","date":"2026-11-09","class":"gpu-medium","experiments":["b/z"],"gpu_hours":2,"usd":8.5}
`))
	if err != nil || len(l) != 3 {
		t.Fatal(err, l)
	}
	if got := Spent(l, "2026-11"); got != 16.5 {
		t.Errorf("spent %v", got)
	}
	if l, err := ReadLedger(filepath.Join(dir, "none.jsonl")); err != nil || l != nil {
		t.Errorf("no ledger: %v, %v", l, err)
	}
	for _, line := range []string{`{"batch":"b","date":"2026-11","usd":1}`, `{"batch":"b","date":"2026-11-01","usd":1,"who":"x"}`} {
		if _, err := ReadLedger(writeFile(t, dir, "bad.jsonl", line)); err == nil || !strings.Contains(err.Error(), "bad.jsonl:1") {
			t.Errorf("%s: %v", line, err)
		}
	}
}

func TestNeeds(t *testing.T) {
	small := &Experiment{Design: &Design{Budget: "gpu-small"}}
	it, ok := Needs("a/x", small)
	if !ok || it.Hours != 2 || it.Estimate != "the cap of the class" || it.Why != "not run" || it.Approval {
		t.Errorf("%+v", it)
	}
	if it, _ := Needs("a/m", &Experiment{Design: &Design{Budget: "gpu-medium"}}); it.Hours != 8 || !it.Approval {
		t.Errorf("a new gpu-medium experiment: %+v", it)
	}
	if it, _ := Needs("a/c", &Experiment{Design: &Design{Budget: "cpu-small"}}); it.Hours != 0 || it.Estimate != "free, in CI" {
		t.Errorf("cpu-small: %+v", it)
	}
	// A run of 0.8 hours whose expect.yaml changed: 1.0 hours, with a quarter more.
	dir := t.TempDir()
	writeFile(t, dir, ExpectFile, "claim: c\n")
	ran := &Experiment{Dir: dir, Design: &Design{Budget: "gpu-medium"}, Results: &Results{ExpectHash: "sha256:old", WallTimeS: 2880}}
	if it, ok := Needs("a/r", ran); !ok || it.Hours != 1 || it.Why != "expect.yaml changed after the run" || it.Approval {
		t.Errorf("a stale run: %+v", it)
	}
	ran.Results.WallTimeS = 8 * 3600
	if it, _ := Needs("a/r", ran); it.Hours != 8 {
		t.Errorf("at most the cap: %+v", it)
	}
	h, err := ran.ExpectHash()
	if err != nil {
		t.Fatal(err)
	}
	ran.Results.ExpectHash = h
	if in, err := ran.InputsHash(); err == nil {
		ran.Results.InputsHash = in
	}
	if _, ok := Needs("a/r", ran); ok {
		t.Error("current results need a run")
	}
}

func TestMakePlan(t *testing.T) {
	c, err := ReadCompute(writeFile(t, t.TempDir(), "c.yaml", toyCompute))
	if err != nil {
		t.Fatal(err)
	}
	items := []PlanItem{
		{Experiment: "a/changed", Class: "gpu-small", Why: "the inputs changed after the run", Hours: 1},
		{Experiment: "a/big", Class: "gpu-medium", Hours: 8, new: true},  // $32
		{Experiment: "a/new", Class: "gpu-small", Hours: 2, new: true},   // $1.60
		{Experiment: "a/huge", Class: "gpu-medium", Hours: 8, new: true}, // $32, past the remaining
		{Experiment: "a/cpu", Class: "cpu-small", new: true},
	}
	ledger := []Batch{{Date: "2026-11-01", USD: 60}, {Date: "2026-10-31", USD: 99}}
	p, err := MakePlan(items, c, ledger, time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if p.Cap != 100 || p.Spent != 60 || p.Remaining != 40 || len(p.CPU) != 1 {
		t.Errorf("%+v", p)
	}
	if len(p.Batches) != 2 || p.Batches[0].Class != "gpu-medium" || p.Batches[0].USD != 32 || p.Batches[0].ID != "b-2026-11-15-gpu-medium" {
		t.Fatalf("%+v", p.Batches)
	}
	// The new experiment comes before the changed one, and the changed one still fits after the big one.
	if b := p.Batches[1]; strings.Join(b.Experiments, " ") != "a/new a/changed" || b.USD != 2.4 || b.GPUHours != 3 {
		t.Errorf("%+v", b)
	}
	if len(p.Waiting) != 1 || p.Waiting[0].Experiment != "a/huge" || p.Waiting[0].USD != 32 {
		t.Errorf("waiting: %+v", p.Waiting)
	}
	if _, err := MakePlan(items, nil, nil, time.Now()); err == nil || !strings.Contains(err.Error(), ComputeFile) {
		t.Errorf("no register: %v", err)
	}
	if p, err := MakePlan(items[4:], nil, nil, time.Now()); err != nil || len(p.CPU) != 1 {
		t.Errorf("cpu-small needs no register: %v", err)
	}
	if _, err := MakePlan(items, c, nil, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)); err == nil || !strings.Contains(err.Error(), "no cap for 2026-01") {
		t.Errorf("before the first cap: %v", err)
	}
}
