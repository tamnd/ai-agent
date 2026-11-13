// SPDX-License-Identifier: Apache-2.0

package site

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/ai-agent/node"
)

func TestCollections(t *testing.T) {
	root := filepath.Join(t.TempDir(), "corpus")
	if err := os.CopyFS(root, os.DirFS("../testdata/corpus")); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"cards/model/5T.jsonl":    `{"tag":"5TQX4M","kind":"model","slug":"llama-3-1","name":"Llama 3.1","role":"family","summary":"A family of open-weight models."}` + "\n" + `{"tag":"5TQX4N","kind":"model","slug":"llama-3-1-405b","owner":"llama-3-1","name":"Llama 3.1 405B","summary":"The largest model.","params":{"total":{"v":4.05e11,"src":"S:paper","loc":"Table 3"}}}` + "\n",
		"cards/benchmark/_.jsonl": `{"tag":"6TQX4M","kind":"benchmark","slug":"toy-bench","name":"Toy bench","summary":"A small benchmark."}` + "\n",
		"cards/metric/_.jsonl":    `{"tag":"7TQX4M","kind":"metric","slug":"toy-score","name":"Toy score","summary":"A small score."}` + "\n",
	}
	for p, s := range files {
		full := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	c, err := node.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "site")
	if _, err := Build(c, Options{Out: out}); err != nil {
		t.Fatal(err)
	}
	read := func(p string) string {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(p)))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	page := read("models/index.html")
	for _, want := range []string{`id="llama-3-1--llama-3-1-405b"`, "Llama 3.1 405B", "405 billion parameters", "Table 3", "S:paper", "2 cards", `href="/api/cards/model.json"`} {
		if !strings.Contains(page, want) {
			t.Errorf("the models page does not contain %q", want)
		}
	}
	if strings.Index(page, "Llama 3.1<") > strings.Index(page, "Llama 3.1 405B") {
		t.Error("the rows are not sorted by name")
	}
	if !strings.Contains(read("kind/model/index.html"), `href="/models/"`) {
		t.Error("the model kind page does not link to the models catalogue")
	}
	if !strings.Contains(read("benchmarks/index.html"), `id="toy-bench"`) {
		t.Error("the benchmarks page has no row for toy-bench")
	}
	if !strings.Contains(read("kind/metric/index.html"), `id="toy-score"`) {
		t.Error("a kind with no catalogue route has no card table on /kind/<kind>/")
	}
	var recs []map[string]any
	if err := json.Unmarshal([]byte(read("api/cards/model.json")), &recs); err != nil || len(recs) != 2 {
		t.Fatalf("model.json holds %d records: %v", len(recs), err)
	}
	var moved map[string]string
	if err := json.Unmarshal([]byte(read("api/moved.json")), &moved); err != nil {
		t.Fatal(err)
	}
	for from, to := range map[string]string{
		"/t/5TQX4N":                        "/models/#llama-3-1--llama-3-1-405b",
		"/models/llama-3-1/llama-3-1-405b": "/models/#llama-3-1--llama-3-1-405b",
		"/models/llama-3-1":                "/models/#llama-3-1",
		"/toy-score":                       "/kind/metric/#toy-score",
	} {
		if moved[from] != to {
			t.Errorf("moved.json sends %s to %q, not %q", from, moved[from], to)
		}
	}
}
