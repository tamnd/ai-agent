package site

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tamnd/ai-agent/node"
)

func buildTest(t *testing.T) string {
	t.Helper()
	c, err := node.Load("../testdata/corpus")
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "site")
	r, err := Build(c, Options{Out: out, Commit: "0123456789abcdef", Version: "test", Now: time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.MathErrors) != 0 {
		t.Errorf("math errors: %v", r.MathErrors)
	}
	if r.Pages == 0 || r.Files < r.Pages {
		t.Errorf("the report is wrong: %d pages, %d files", r.Pages, r.Files)
	}
	return out
}

func TestBuild(t *testing.T) {
	out := buildTest(t)
	for _, f := range []string{
		"index.html", "404.html", "binary-search/index.html", "search-in-sorted-array/index.html",
		"searching/index.html", "spec/index.html", "search.json", "sitemap.xml", "robots.txt",
		"opensearch.xml", "assets/site.css", "assets/player.js",
	} {
		if _, err := os.Stat(filepath.Join(out, f)); err != nil {
			t.Errorf("%s is missing", f)
		}
	}
	page, err := os.ReadFile(filepath.Join(out, "binary-search/index.html"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(page)
	for _, want := range []string{
		"<title>Binary search",
		`id="see-it-run"`,
		`data-scene="binary-search"`,
		`id="references"`,
		"https://ai.tamnd.com/binary-search/",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("the binary search page does not contain %q", want)
		}
	}
	if strings.Count(s, `id="references"`) != 1 {
		t.Error("the binary search page has more than one References section")
	}
}

func TestBuildReplacesOutput(t *testing.T) {
	c, err := node.Load("../testdata/corpus")
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "site")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(out, "stale.html")
	if err := os.WriteFile(stale, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(c, Options{Out: out}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Error("the build kept a file from the last build")
	}
	if _, err := os.Stat(out + ".next"); !os.IsNotExist(err) {
		t.Error("the build left its work directory")
	}
}

func TestAreasByGroup(t *testing.T) {
	out := buildTest(t)
	page, err := os.ReadFile(filepath.Join(out, "areas/index.html"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(page)
	a := strings.Index(s, `id="group-a"`)
	b := strings.Index(s, `id="group-b"`)
	if a < 0 || b < a {
		t.Fatalf("the areas page has no group A before group B")
	}
	if i := strings.Index(s, `href="/sorting/"`); i < b {
		t.Errorf("sorting is not under group B")
	}
	if strings.Contains(s, "group-other") {
		t.Errorf("the areas page has a shelf of areas with no group")
	}
	sub, err := os.ReadFile(filepath.Join(out, "searching/index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(sub), `href="/areas/#group-b"`) {
		t.Errorf("the crumbs of the searching area do not link to its group")
	}
}

func TestFactTable(t *testing.T) {
	root := filepath.Join(t.TempDir(), "corpus")
	if err := os.CopyFS(root, os.DirFS("../testdata/corpus")); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "nodes", "foundations", "models-of-computation", "toy-model")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	md := "---\ntag: 7TQYMD\nkind: model\nslug: toy-model\nname: Toy model\narea: foundations/models-of-computation\nlevel: intro\nsummary: A small model.\n---\n\n## Overview\n\nA small model.\n"
	fy := "- id: params.total\n  unit: parameters\n  sources:\n    - {src: card, kind: primary, loc: \"Table 1\", value: 7000000000}\n    - {src: epoch-ai, kind: secondary, row: \"12\", value: 8000000000}\n"
	for name, s := range map[string]string{"node.md": md, "facts.yaml": fy} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(s), 0o644); err != nil {
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
	page, err := os.ReadFile(filepath.Join(out, "toy-model", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`id="facts"`, "Parameters (total)", "7 billion parameters", "status-disputed", "Table 1", ": 8 billion parameters", "row 12", "Data facts"} {
		if !strings.Contains(string(page), want) {
			t.Errorf("the model page does not contain %q", want)
		}
	}
	api, err := os.ReadFile(filepath.Join(out, "api", "nodes.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(api), `"facts":[{"id":"params.total"`) || !strings.Contains(string(api), `"status":"disputed"`) {
		t.Error("nodes.json does not hold the facts with their status")
	}
}
