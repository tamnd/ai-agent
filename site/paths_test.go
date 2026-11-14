// SPDX-License-Identifier: Apache-2.0

package site

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tamnd/ai-agent/node"
)

func TestPaths(t *testing.T) {
	root := filepath.Join(t.TempDir(), "corpus")
	if err := os.CopyFS(root, os.DirFS("../testdata/corpus")); err != nil {
		t.Fatal(err)
	}
	md := "---\ntag: 7TQYMD\nkind: method\nslug: toy-search\nname: Toy search\narea: searching/sequential-and-binary-search\nlevel: core\nsummary: A toy.\n" +
		"prereqs: [binary-search, matrix-multiplication@m2]\n---\n\n## Overview\n\nA toy search reads " + strings.Repeat("word ", 600) + "\n"
	files := map[string]string{
		"nodes/searching/sequential-and-binary-search/toy-search/node.md": md,
		"cards/bridge/m2/_.jsonl": `{"tag":"M2MM01","site":"m2","kind":"theorem","slug":"matrix-multiplication","name":"Matrix multiplication","url":"https://math2.tamnd.com/matrix-multiplication/"}` + "\n",
		"topics/courses/searching.yaml": "id: searching\ntitle: Searching\nbasis: the order of a textbook\nsections:\n  - heading: First ideas\n    items:\n      - {name: Loop invariant, node: loop-invariant}\n      - {name: Interpolation search, kind: method}\n      - {name: asymptotic notation, kind: definition}\n  - heading: Search\n    items:\n      - {name: Binary search, kind: method, node: binary-search}\n",
		"topics/courses/to.yaml": "id: to\ntitle: A course with a reserved id\nsections: []\n",
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
	if _, err := Build(c, Options{Out: out, Version: "test", Now: time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatal(err)
	}
	read := func(p string) string {
		b, err := os.ReadFile(filepath.Join(out, p))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	to := read("paths/to/toy-search/index.html")
	order := []string{`data-step="loop-invariant"`, `data-step="asymptotic-notation"`, `data-step="binary-search"`, `data-step="m2:matrix-multiplication"`, `data-step="toy-search"`}
	at := -1
	for _, s := range order {
		i := strings.Index(to, s)
		if i <= at {
			t.Fatalf("the path to toy-search does not have %s after the step before: %d <= %d", s, i, at)
		}
		at = i
	}
	for _, want := range []string{"<title>From zero to Toy search", `class="target"`, "https://math2.tamnd.com/matrix-multiplication/", "· 5 min", `data-path="to/toy-search"`, "/assets/paths.js", "H0 L0"} {
		if !strings.Contains(to, want) {
			t.Errorf("/paths/to/toy-search/ does not contain %q", want)
		}
	}
	co := read("paths/searching/index.html")
	for _, want := range []string{"<h2>First ideas</h2>", "<h2>Search</h2>", "It follows the order of a textbook.", `<span class="missing" title="This page is not written yet.">Interpolation search</span>`, `href="/binary-search/"`, `href="/asymptotic-notation/"`, "(3 written)"} {
		if !strings.Contains(co, want) {
			t.Errorf("/paths/searching/ does not contain %q", want)
		}
	}
	if _, err := os.Stat(filepath.Join(out, "paths", "to", "index.html")); err == nil {
		t.Error("a course took the reserved id to")
	}
	area := read("paths/area/searching/index.html")
	if i, j := strings.Index(area, `data-step="binary-search"`), strings.Index(area, `data-step="toy-search"`); i < 0 || j < i {
		t.Errorf("the area path of searching does not have binary-search before toy-search: %d %d", i, j)
	}
	idx := read("paths/index.html")
	for _, want := range []string{`href="/paths/searching/"`, `href="/paths/area/searching/"`, `href="/paths/to/toy-search/"`, "From zero to a target"} {
		if !strings.Contains(idx, want) {
			t.Errorf("/paths/ does not contain %q", want)
		}
	}
	if page := read("toy-search/index.html"); !strings.Contains(page, `<a class="all" href="/paths/to/toy-search/">`) || !strings.Contains(page, "data-path-strip") {
		t.Error("the page of toy-search has no link to its reading path or no strip")
	}
}
