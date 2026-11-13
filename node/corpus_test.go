package node

import "testing"

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
