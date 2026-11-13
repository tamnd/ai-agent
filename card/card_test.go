// SPDX-License-Identifier: Apache-2.0

package card

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestParse(t *testing.T) {
	line := `{"tag":"5TQX4M","kind":"model","slug":"llama-3-1-405b","owner":"llama-3-1","name":"Llama 3.1 405B","role":"release","summary":"A dense model.","params":{"total":{"v":4.05e11,"src":"S:paper","loc":"Table 3"}},"released":{"v":"2024-07-23","src":"S:blog"},"same_as":{"wikidata":"Q1"},"harvest":{"source":"epoch-ai","row":"12","date":"2026-11-01"}}`
	c, err := Parse([]byte(line))
	if err != nil {
		t.Fatal(err)
	}
	if c.Tag != "5TQX4M" || c.Kind != "model" || c.Key() != "llama-3-1/llama-3-1-405b" || c.Role != "release" {
		t.Errorf("the fields are wrong: %+v", c)
	}
	if len(c.Harvest) != 1 || c.Harvest[0].Source != "epoch-ai" || c.Harvest[0].Row != "12" {
		t.Errorf("the harvest record is wrong: %+v", c.Harvest)
	}
	var ids []string
	for _, f := range c.Facts {
		ids = append(ids, f.ID)
	}
	if !slices.Equal(ids, []string{"params.total", "released"}) {
		t.Fatalf("the facts are %v", ids)
	}
	if f := c.Facts[0]; f.V != 4.05e11 || f.Src != "S:paper" || f.Loc != "Table 3" {
		t.Errorf("the fact params.total is %+v", f)
	}
	if len(c.Bad) != 0 {
		t.Errorf("the card has problems: %v", c.Bad)
	}
}

func TestParseHarvestList(t *testing.T) {
	c, err := Parse([]byte(`{"tag":"5TQX4M","harvest":[{"source":"a"},{"source":"b"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Harvest) != 2 || c.Harvest[1].Source != "b" {
		t.Errorf("the harvest records are %+v", c.Harvest)
	}
}

func TestParseBad(t *testing.T) {
	for _, line := range []string{`[1,2]`, `null`, `{"tag":5}`, `{"harvest":"x"}`, `{"tag":`} {
		if _, err := Parse([]byte(line)); err == nil {
			t.Errorf("Parse(%s) gives no error", line)
		}
	}
	c, err := Parse([]byte(`{"tag":"5TQX4M","a":{"src":"S:x"},"b":{"v":1,"src":2},"c":{"v":1,"loc":[1]}}`))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"the fact a has a src but no v", "the src of the fact b is not a string", "the loc of the fact c is not a string"}
	if !slices.Equal(c.Bad, want) {
		t.Errorf("the problems are %q, not %q", c.Bad, want)
	}
}

func TestRead(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, Dir, "model")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	data := "{\"tag\":\"5TQX4M\",\"name\":\"A\"}\n\nnot json\n{\"tag\":\"5TQX4N\",\"name\":\"B\"}\n"
	if err := os.WriteFile(filepath.Join(dir, "5T.jsonl"), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	cs, errs := Read(root)
	if len(cs) != 2 || cs[1].Line != 4 || cs[1].File != "cards/model/5T.jsonl" {
		t.Errorf("Read gives %d cards; the second is on %s:%d", len(cs), cs[len(cs)-1].File, cs[len(cs)-1].Line)
	}
	if len(errs) != 1 {
		t.Fatalf("Read gives the errors %v", errs)
	}
	var e *Error
	if !errors.As(errs[0], &e) || e.Line != 3 {
		t.Errorf("the error is %v", errs[0])
	}
	if cs, errs := Read(t.TempDir()); len(cs) != 0 || len(errs) != 0 {
		t.Errorf("a root with no cards gives %d cards and the errors %v", len(cs), errs)
	}
}

func TestShard(t *testing.T) {
	if s := Shard("cards/model/5T.jsonl"); s != "5T" {
		t.Errorf("Shard gives %q", s)
	}
	for _, tc := range []struct {
		tag, shard string
		want       bool
	}{{"5TQX4M", "5T", true}, {"6TQX4M", "5T", false}, {"6TQX4M", "_", true}} {
		if got := InShard(tc.tag, tc.shard); got != tc.want {
			t.Errorf("InShard(%q, %q) = %v", tc.tag, tc.shard, got)
		}
	}
	if Words("  a  b c ") != 3 {
		t.Error("Words counts the words wrong")
	}
}
