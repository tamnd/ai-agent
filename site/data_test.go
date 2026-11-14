// SPDX-License-Identifier: Apache-2.0

package site

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tamnd/ai-agent/node"
)

// TestDatasets checks the datasets of 16.10 that a build writes with Options.Data.
func TestDatasets(t *testing.T) {
	c, err := node.Load("../testdata/corpus")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	data := filepath.Join(dir, "data")
	if err := os.MkdirAll(filepath.Join(data, "stale"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(c, Options{Out: filepath.Join(dir, "site"), Data: data, Commit: "0123456789abcdef", Version: "test", Now: time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(data, "stale")); err == nil {
		t.Error("the build did not replace the old datasets")
	}
	b, err := os.ReadFile(filepath.Join(data, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if m.AsOf != "2026-11-03" || m.Commit != "0123456789abcdef" || len(m.NotBuilt) == 0 {
		t.Errorf("manifest = %+v", m)
	}
	files := map[string]DataFile{}
	for _, d := range m.Datasets {
		if d.Licence == "" {
			t.Errorf("%s has no licence", d.Name)
		}
		for _, f := range d.Files {
			files[f.Path] = f
			raw, err := os.ReadFile(filepath.Join(data, filepath.FromSlash(f.Path)))
			if err != nil {
				t.Fatal(err)
			}
			sum := sha256.Sum256(raw)
			if hex.EncodeToString(sum[:]) != f.SHA256 || len(raw) != f.Bytes || bytes.Count(raw, []byte("\n")) != f.Rows {
				t.Errorf("%s: the manifest does not match the file", f.Path)
			}
		}
	}
	n, ok := c.BySlug["binary-search"]
	if !ok {
		t.Fatal("no binary-search in the test corpus")
	}
	items := "items/" + n.Kind + ".jsonl"
	if files[items].Rows == 0 {
		t.Fatalf("%s is missing or empty: %v", items, files)
	}
	found := false
	each(t, filepath.Join(data, items), func(line []byte) {
		var r Record
		if err := json.Unmarshal(line, &r); err != nil {
			t.Fatal(err)
		}
		if r.Key == "binary-search" {
			found = true
			if r.Status == nil || r.URL != "https://ai.tamnd.com/binary-search/" {
				t.Errorf("the record of binary-search: %+v", r.Head)
			}
		}
	})
	if !found {
		t.Error("binary-search is not in the items")
	}
	solves := false
	each(t, filepath.Join(data, "graph.jsonl"), func(line []byte) {
		var e Edge
		if err := json.Unmarshal(line, &e); err != nil {
			t.Fatal(err)
		}
		solves = solves || e.From == "binary-search" && e.Type == "solves" && e.To == "search-in-sorted-array"
	})
	if !solves {
		t.Error("graph.jsonl does not have the solves edge of binary-search")
	}
	if want := len(n.Claims); want > 0 && files["claims.jsonl"].Rows < want {
		t.Errorf("claims.jsonl has %d rows, binary-search alone has %d claims", files["claims.jsonl"].Rows, want)
	}
}

func each(t *testing.T, file string, fn func([]byte)) {
	t.Helper()
	fh, err := os.Open(file)
	if err != nil {
		t.Fatal(err)
	}
	defer fh.Close()
	sc := bufio.NewScanner(fh)
	sc.Buffer(make([]byte, 1<<20), 16<<20)
	for sc.Scan() {
		fn(sc.Bytes())
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
}
