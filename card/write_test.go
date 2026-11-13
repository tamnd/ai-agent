// SPDX-License-Identifier: Apache-2.0

package card

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEncode(t *testing.T) {
	in := `{"harvest":{"source":"epoch-ai","date":"2026-11-13"},"same_as":{"wikidata":"Q1","epoch":"GPT-4"},"zeta":1,"name":"GPT-4","alpha":{"x":{"loc":"Parameters","src":"S:epoch-ai","v":1.8e12}},"slug":"gpt-4","kind":"model","tag":"NEW-gpt-4","summary":"A <model> & more."}`
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(in), &m); err != nil {
		t.Fatal(err)
	}
	b, err := Encode(m)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"tag":"NEW-gpt-4","kind":"model","slug":"gpt-4","name":"GPT-4","alpha":{"x":{"v":1.8e12,"src":"S:epoch-ai","loc":"Parameters"}},"zeta":1,"same_as":{"epoch":"GPT-4","wikidata":"Q1"},"summary":"A <model> & more.","harvest":{"date":"2026-11-13","source":"epoch-ai"}}`
	if string(b) != want {
		t.Errorf("Encode:\n got %s\nwant %s", b, want)
	}
	if _, err := Parse(b); err != nil {
		t.Errorf("the encoded line does not parse: %v", err)
	}
}

func TestSlugify(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"Llama 3.1-405B", "llama-3-1-405b"},
		{"C++ & C#", "c-plus-plus-and-c-sharp"},
		{"Kahneman's  System 2", "kahnemans-system-2"},
		{"π0.6 (pi-0.6)", "0-6-pi-0-6"},
		{strings.Repeat("word ", 20), strings.TrimSuffix(strings.Repeat("word-", 12), "-")},
	} {
		got := Slugify(c.in)
		if got != c.want {
			t.Errorf("Slugify(%q) = %q, want %q", c.in, got, c.want)
		}
		if len(got) > 60 {
			t.Errorf("Slugify(%q) has %d characters", c.in, len(got))
		}
	}
}

func TestWriteKind(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "model")
	b := `{"tag": "NEW-b",  "kind":"model","slug":"b","name":"B"}` + "\n" + `{"tag":"NEW-a","kind":"model","slug":"a","name":"A"}` + "\n"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "_.jsonl"), []byte(b), 0o644); err != nil {
		t.Fatal(err)
	}
	lines, err := ReadKind(dir)
	if err != nil || len(lines) != 2 {
		t.Fatalf("ReadKind: %d lines, %v", len(lines), err)
	}
	if err := lines[1].Card.Set("aliases", []string{"Alpha"}); err != nil {
		t.Fatal(err)
	}
	lines[1].Changed = true
	if err := WriteKind(dir, lines, MaxShard); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "_.jsonl"))
	// The changed line is canonical; the other keeps its bytes; the lines are sorted by tag.
	want := `{"tag":"NEW-a","kind":"model","slug":"a","name":"A","aliases":["Alpha"]}` + "\n" + `{"tag": "NEW-b",  "kind":"model","slug":"b","name":"B"}` + "\n"
	if string(got) != want {
		t.Errorf("WriteKind:\n%s", got)
	}

	// A shard over the limit splits by the first character of the tag, and a new tag goes to the shard of its prefix.
	var many []*Line
	for _, tag := range []string{"5TQH2W", "5VAAAA", "A00000", "A11111"} {
		c := &Card{Tag: tag, Kind: "model", Slug: strings.ToLower(tag), Name: tag}
		for k, v := range map[string]string{"tag": tag, "kind": "model", "slug": c.Slug, "name": tag} {
			_ = c.Set(k, v)
		}
		many = append(many, &Line{Card: c, Changed: true})
	}
	if err := WriteKind(dir, many, 150); err != nil {
		t.Fatal(err)
	}
	names := func() string {
		ps, _ := filepath.Glob(filepath.Join(dir, "*.jsonl"))
		var out []string
		for _, p := range ps {
			out = append(out, filepath.Base(p))
		}
		return strings.Join(out, " ")
	}
	if got := names(); got != "5.jsonl A.jsonl" {
		t.Errorf("the shards are %s", got)
	}
	c := &Card{Tag: "ZZZZZZ", Kind: "model", Slug: "z", Name: "Z"}
	for k, v := range map[string]string{"tag": c.Tag, "kind": "model", "slug": "z", "name": "Z"} {
		_ = c.Set(k, v)
	}
	if err := WriteKind(dir, append(many, &Line{Card: c, Changed: true}), 150); err != nil {
		t.Fatal(err)
	}
	if got := names(); got != "5.jsonl A.jsonl Z.jsonl" {
		t.Errorf("the shards after a new tag are %s", got)
	}
	five, _ := os.ReadFile(filepath.Join(dir, "5.jsonl"))
	if !strings.HasPrefix(string(five), `{"tag":"5TQH2W",`) || strings.Count(string(five), "\n") != 2 {
		t.Errorf("5.jsonl is %s", five)
	}
}
