// SPDX-License-Identifier: Apache-2.0

package facts

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func src(kind string, v any) Source { return Source{Src: "s", Kind: kind, Value: v, Loc: "p. 1"} }

func TestCompute(t *testing.T) {
	for _, c := range []struct {
		name    string
		sources []Source
		want    string
	}{
		{"agreed", []Source{src(Primary, 7e9), src(Secondary, 7.03e9)}, Agreed},
		{"primary only", []Source{src(Primary, 7e9)}, PrimaryOnly},
		{"secondary only", []Source{src(Secondary, 7e9)}, SecondaryOnly},
		{"estimated", []Source{src(Estimate, 3e24), src(Secondary, 3e24)}, Estimated},
		{"disputed", []Source{src(Primary, 7e9), src(Secondary, 8e9)}, Disputed},
		{"no source", nil, ""},
	} {
		f := Fact{ID: "params.total", Sources: c.sources}
		Compute(&f)
		if f.Status != c.want {
			t.Errorf("%s: status %q, want %q", c.name, f.Status, c.want)
		}
		if len(c.sources) > 0 && f.Value == nil {
			t.Errorf("%s: Compute did not take the value of the best source", c.name)
		}
	}
}

func TestBest(t *testing.T) {
	f := Fact{Sources: []Source{src(Secondary, 1.0), src(Estimate, 2.0), src(Primary, 3.0)}}
	if b := Best(&f); b == nil || b.Value != 3.0 {
		t.Errorf("Best gives %v, want the primary source", b)
	}
	f.Sources = f.Sources[:2]
	if b := Best(&f); b == nil || b.Value != 2.0 {
		t.Errorf("Best gives %v, want the estimate", b)
	}
}

func TestSame(t *testing.T) {
	for _, c := range []struct {
		a, b any
		want bool
	}{
		{100.0, 100.9, true},
		{100.0, 102.0, false},
		{"2024-07", "2024-07-23", true},
		{"2024-07-01", "2024-07-23", false},
		{"Open Weights", "open weights", true},
		{[]any{"text", "image"}, []any{"image", "text"}, true},
		{[]any{"text"}, []any{"text", "image"}, false},
		{1.0, "1", false},
	} {
		if got := Same(c.a, c.b); got != c.want {
			t.Errorf("Same(%v, %v) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestFormat(t *testing.T) {
	for _, c := range []struct {
		v    any
		unit string
		want string
	}{
		{7e9, "parameters", "7 billion parameters"},
		{1.5e12, "tokens", "1.5 trillion tokens"},
		{3.8e25, "FLOP", "3.8 × 10^25 FLOP"},
		{128000.0, "tokens", "128,000 tokens"},
		{[]any{"text", "image"}, "", "text, image"},
		{"2024-07-23", "", "2024-07-23"},
	} {
		if got := Format(c.v, c.unit); got != c.want {
			t.Errorf("Format(%v, %q) = %q, want %q", c.v, c.unit, got, c.want)
		}
	}
}

func TestMissingAndCovers(t *testing.T) {
	fs := []Fact{{ID: "vendor"}, {ID: "peak_flops.fp16"}, {ID: "memory.bytes"}}
	got := Missing("hardware", fs)
	want := []string{"released", "type", "memory.bandwidth", "power"}
	if !slices.Equal(got, want) {
		t.Errorf("Missing gives %v, want %v", got, want)
	}
	if Covers("params.totally", "params.total") {
		t.Error("Covers matches a longer word")
	}
	if Missing("concept", nil) != nil {
		t.Error("a concept needs no facts")
	}
}

func TestTimeBoundAndLabel(t *testing.T) {
	if !TimeBound("licence") || !TimeBound("price.input") || TimeBound("released") {
		t.Error("TimeBound is wrong")
	}
	for id, want := range map[string]string{
		"params.total":    "Parameters (total)",
		"peak_flops.fp16": "Peak FLOP/s (fp16)",
		"context_window":  "Context window",
	} {
		if got := Label(id); got != want {
			t.Errorf("Label(%q) = %q, want %q", id, got, want)
		}
	}
}

func TestRead(t *testing.T) {
	dir := t.TempDir()
	if fs, err := Read(dir); err != nil || fs != nil {
		t.Fatalf("Read of a directory with no file gives %v, %v", fs, err)
	}
	const y = `- id: released
  sources:
    - {src: paper, kind: primary, loc: "p. 1", value: 2024-07-23}
    - {src: epoch-ai, kind: secondary, row: "42", value: 2024-07}
- id: params.total
  value: 405000000000
  unit: parameters
  sources:
    - {src: paper, kind: primary, loc: "Table 3", value: 405000000000}
`
	if err := os.WriteFile(filepath.Join(dir, File), []byte(y), 0o644); err != nil {
		t.Fatal(err)
	}
	fs, err := Read(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(fs) != 2 {
		t.Fatalf("Read gives %d facts, want 2", len(fs))
	}
	if fs[0].Value != "2024-07-23" || fs[0].Status != Agreed {
		t.Errorf("the date fact is %v with the status %q", fs[0].Value, fs[0].Status)
	}
	if fs[1].Value != 405e9 || fs[1].Status != PrimaryOnly {
		t.Errorf("the number fact is %v (%T) with the status %q", fs[1].Value, fs[1].Value, fs[1].Status)
	}
	if err := os.WriteFile(filepath.Join(dir, File), []byte("- id: x\n  colour: red\n"), 0o644); err == nil {
		if _, err := Read(dir); err == nil {
			t.Error("Read accepts an unknown field")
		}
	}
}
