// SPDX-License-Identifier: Apache-2.0

package site

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestRecordTwin checks the JSON record that the build writes next to each page (16.10).
func TestRecordTwin(t *testing.T) {
	out := buildTest(t)
	b, err := os.ReadFile(filepath.Join(out, "binary-search.json"))
	if err != nil {
		t.Fatal(err)
	}
	var r Record
	if err := json.Unmarshal(b, &r); err != nil {
		t.Fatal(err)
	}
	if r.URL != "https://ai.tamnd.com/binary-search/" || r.Kind == "" || r.Key != "binary-search" || r.AsOf == "" {
		t.Errorf("head = %+v", r.Head)
	}
	if r.Markdown != "https://ai.tamnd.com/binary-search.md" {
		t.Errorf("markdown_url = %q", r.Markdown)
	}
	if r.Status == nil {
		t.Error("the record has no ladders")
	}
	if !slices.ContainsFunc(r.Out, func(e Edge) bool { return e.Type == "solves" && e.To == "search-in-sorted-array" && e.Known }) {
		t.Errorf("edges out = %+v", r.Out)
	}
	if strings.Contains(string(b), "## ") {
		t.Error("the record holds the prose")
	}
	b, err = os.ReadFile(filepath.Join(out, "search-in-sorted-array.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &r); err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(r.In, func(e Edge) bool { return e.From == "binary-search" && e.Type == "solves" }) {
		t.Errorf("edges in = %+v", r.In)
	}
	llms, err := os.ReadFile(filepath.Join(out, "llms.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(llms), "/<slug>.json") {
		t.Error("llms.txt does not name the JSON records")
	}
}
