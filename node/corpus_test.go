package node

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	c, err := Load("../testdata/corpus")
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Errors) != 0 {
		t.Fatalf("errors: %v", c.Errors)
	}
	if got := len(c.Nodes); got != 5 {
		t.Fatalf("got %d nodes, want 5", got)
	}
	n := c.BySlug["binary-search"]
	if n == nil {
		t.Fatal("binary-search is missing")
	}
	if n.Kind != "method" {
		t.Errorf("kind is %q, want method", n.Kind)
	}
	if c.ByTag["2B7QDX"] != n {
		t.Error("the tag 2B7QDX does not give binary-search")
	}
	if c.LangByID["python"] == nil || c.LangByID["go"] == nil {
		t.Error("the language register is not loaded")
	}
	if c.Frameworks || c.LangFile != "registers/languages.yaml" {
		t.Errorf("a tree with no framework register reads %s", c.LangFile)
	}
}

func TestFrameworkRegister(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../testdata/corpus")); err != nil {
		t.Fatal(err)
	}
	reg := "- {id: jax, name: JAX, tier: 1, language: python, ext: [py], packages: jax-ref, types: {tool: pyright, mode: basic}, determinism: [x]}\n- {id: c, name: C, tier: 2, ext: [c, h]}\n"
	if err := os.WriteFile(filepath.Join(root, "registers", "frameworks.yaml"), []byte(reg), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if !c.Frameworks || c.LangFile != "registers/frameworks.yaml" || len(c.Langs) != 2 {
		t.Fatalf("the corpus reads %s with %d entries", c.LangFile, len(c.Langs))
	}
	jax := c.LangByID["jax"]
	if jax == nil || jax.Lang() != "python" || jax.Packages != "jax-ref" || jax.Types.Tool != "pyright" || len(jax.Determinism) != 1 {
		t.Errorf("the entry of jax is %+v", jax)
	}
	if c.LangByID["c"].Lang() != "c" || c.LangByID["python"] != nil {
		t.Error("the framework register does not replace the language register")
	}
}

func TestIsKind(t *testing.T) {
	if len(KindTable) != 28 {
		t.Errorf("the kind table has %d kinds, want 28", len(KindTable))
	}
	for _, k := range []string{"method", "architecture", "task", "concept", "definition", "model", "theorem", "result"} {
		if !IsKind(k) {
			t.Errorf("IsKind(%q) is false", k)
		}
	}
	for _, k := range []string{"recipe", "algorithm", "problem", "data-structure"} {
		if IsKind(k) {
			t.Errorf("IsKind(%q) is true", k)
		}
	}
}

func TestSplitFront(t *testing.T) {
	front, body, line, err := SplitFront([]byte("---\ntag: X\n---\n\n## A\n"))
	if err != nil {
		t.Fatal(err)
	}
	if string(front) != "tag: X\n" {
		t.Errorf("front is %q", front)
	}
	if string(body) != "\n## A\n" {
		t.Errorf("body is %q", body)
	}
	if line != 4 {
		t.Errorf("body line is %d, want 4", line)
	}
	if _, _, _, err := SplitFront([]byte("no front matter\n")); err == nil {
		t.Error("a file without front matter gave no error")
	}
}

func TestImplSkipsCaches(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../testdata/corpus")); err != nil {
		t.Fatal(err)
	}
	impl := filepath.Join(root, "nodes/searching/sequential-and-binary-search/binary-search/impl/python")
	for _, f := range []string{"__pycache__/binary_search.cpython-314.pyc", ".hypothesis/constants/a1", ".pytest_cache/v/cache/lastfailed", "node_modules/x/index.js"} {
		p := filepath.Join(impl, f)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	c, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, im := range c.BySlug["binary-search"].Impls {
		if im.Lang != "python" {
			continue
		}
		for _, f := range im.Files {
			names = append(names, f.Name)
		}
	}
	if len(names) != 2 || names[0] != "binary_search.py" || names[1] != "test_binary_search.py" {
		t.Errorf("the python files are %v, want binary_search.py and test_binary_search.py", names)
	}
}

// A theorem's lean field holds expected: true, a YAML bool, beside the names, and its derivations are read with their
// front matter; a file without one is skipped.
func TestTheoremDerivations(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "nodes", "t", "pac-bound")
	if err := os.MkdirAll(filepath.Join(dir, "derivations"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"node.md": "---\nkind: theorem\nslug: pac-bound\nname: A bound\nlean:\n  module: AI.T.PacBound\n  statement: AI.T.PacBound.bound\n" +
			"  expected: true\n  steps: derivations/proof.md\n---\n\nThe body.\n",
		"derivations/proof.md":    "---\nid: proof\nkind: proof\nstatus: full\nderives: [c-bound]\nlean: AI.T.PacBound.bound\n---\n\n### Statement\n",
		"derivations/agnostic.md": "---\nid: agnostic\nkind: bound\nstatus: sketch\nlean: null\n---\n",
		"derivations/notes.md":    "Notes without front matter.\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	n, err := Read(root, dir)
	if err != nil {
		t.Fatal(err)
	}
	if n.Lean["expected"] != "true" || n.Lean["steps"] != "derivations/proof.md" {
		t.Errorf("lean = %v", n.Lean)
	}
	if len(n.Derivations) != 2 {
		t.Fatalf("derivations = %+v", n.Derivations)
	}
	a, p := n.Derivations[0], n.Derivations[1]
	if a.ID != "agnostic" || a.Front.Kind != "bound" || a.Front.Lean != "" {
		t.Errorf("agnostic = %+v", a)
	}
	if p.ID != "proof" || p.Front.Kind != "proof" || p.Front.Status != "full" || p.Path != "nodes/t/pac-bound/derivations/proof.md" || p.Front.Derives[0] != "c-bound" {
		t.Errorf("proof = %+v", p)
	}
}
