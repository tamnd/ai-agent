package check

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/ai-agent/node"
)

func TestCorpusIsClean(t *testing.T) {
	c, err := node.Load("../testdata/corpus")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range Run(c, Options{Main: true}) {
		if f.Severity == Block {
			t.Errorf("%s:%d %s %s", f.Path, f.Line, f.Rule, f.Msg)
		}
	}
}

func TestPseudoIDs(t *testing.T) {
	body := "## The algorithm\n\n```pseudo id=main\nF(A)\nL1  x ← 0\nL2  return x\n```\n"
	ids, bad := PseudoIDs(body)
	if len(bad) != 0 {
		t.Fatalf("findings: %v", bad)
	}
	if got := ids["main"]; len(got) != 2 || got[0] != "L1" || got[1] != "L2" {
		t.Errorf("ids are %v, want [L1 L2]", got)
	}
}

func TestMarkers(t *testing.T) {
	got := Markers("x = 0  # @line L1\nreturn x  # @line L2, L3\n")
	want := []string{"L1", "L2", "L3"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestParseClaim(t *testing.T) {
	for _, s := range []string{"Θ(n log n)", "O(1)", "Ω(n)"} {
		if err := ParseClaim(s); err != nil {
			t.Errorf("ParseClaim(%q): %v", s, err)
		}
	}
}

func TestMissingSlug(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../testdata/corpus")); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(root, "nodes/searching/sequential-and-binary-search/binary-search/node.md")
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.Replace(string(data), "see_also: [loop-invariant]", "see_also: [no-such-node]", 1))
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := node.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range Run(c, Options{}) {
		if f.Rule == "E01" && f.Severity == Block && strings.Contains(f.Msg, "no-such-node") {
			return
		}
	}
	t.Error("a see_also link to a missing node gave no G05 finding")
}

func TestBuildFile(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../testdata/corpus")); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(root, "nodes", "searching", "sequential-and-binary-search", "binary-search", "impl", "go", "go.mod")
	if err := os.WriteFile(p, []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := node.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range Run(c, Options{}) {
		if f.Rule == "X12" && f.Severity == Block && strings.HasSuffix(f.Path, "impl/go/go.mod") {
			found = true
		}
	}
	if !found {
		t.Error("no X12 finding for impl/go/go.mod")
	}
}

func TestGroups(t *testing.T) {
	c := &node.Corpus{Areas: []node.Area{
		{ID: "good", Group: "D", Children: []node.Area{{ID: "fine", Parent: "good"}}},
		{ID: "none"},
		{ID: "bad", Group: "Z"},
		{ID: "top", Group: "A", Children: []node.Area{{ID: "sub", Parent: "top", Group: "A"}}},
	}}
	r := &run{c: c}
	r.groups()
	want := map[string]bool{`"none" has no group`: false, `"bad" has the group "Z"`: false, `"top/sub" has a group`: false}
	for _, f := range r.out {
		for w := range want {
			if strings.Contains(f.Msg, w) {
				want[w] = true
			}
		}
	}
	for w, ok := range want {
		if !ok {
			t.Errorf("no finding says %s; got %v", w, r.out)
		}
	}
	if len(r.out) != 3 {
		t.Errorf("got %d findings, want 3: %v", len(r.out), r.out)
	}
}

// A global slug is at /<slug>/, the URL of a top area; a sub-area is at /<top>/<sub>/, so only a top area's id is
// taken (16.2).
func TestSlugOfAnArea(t *testing.T) {
	for _, tc := range []struct {
		slug string
		want bool
	}{{"searching", true}, {"sequential-and-binary-search", false}} {
		c, err := node.Load("../testdata/corpus")
		if err != nil {
			t.Fatal(err)
		}
		n := c.BySlug["binary-search"]
		n.Slug = tc.slug
		r := &run{c: c, root: c.Root}
		r.front(n)
		var got bool
		for _, f := range r.out {
			got = got || (f.Rule == "T04" && strings.Contains(f.Msg, "top area"))
		}
		if got != tc.want {
			t.Errorf("slug %q: T04 %v, want %v", tc.slug, got, tc.want)
		}
	}
}

// A language id is not a slug, except for the software page of that language or framework (16.2).
func TestSlugOfALanguage(t *testing.T) {
	for _, tc := range []struct {
		kind string
		want bool
	}{{"method", true}, {"software", false}} {
		c, err := node.Load("../testdata/corpus")
		if err != nil {
			t.Fatal(err)
		}
		n := c.BySlug["binary-search"]
		n.Slug, n.Kind = "python", tc.kind
		r := &run{c: c, root: c.Root}
		r.front(n)
		var got bool
		for _, f := range r.out {
			got = got || (f.Rule == "T04" && strings.Contains(f.Msg, "language id"))
		}
		if got != tc.want {
			t.Errorf("a %s at /python/: T04 %v, want %v", tc.kind, got, tc.want)
		}
	}
}

// A {fact:} reference names a fact that its item holds (D01); a computed fact needs aik facts compute (D08).
func TestFactRefs(t *testing.T) {
	c, err := node.Load("../testdata/corpus")
	if err != nil {
		t.Fatal(err)
	}
	n := c.BySlug["binary-search"]
	n.Body += "\nIt has {fact:no-such-item/params.total} and is {fact:computed/largest-open-weights-dense-by-params}.\n"
	r := &run{c: c, root: c.Root}
	r.factRefs(n)
	var rules []string
	for _, f := range r.out {
		rules = append(rules, f.Rule+" "+f.Msg)
	}
	got := strings.Join(rules, "\n")
	if !strings.Contains(got, "D01 {fact:no-such-item/params.total}: no node or card is no-such-item") || !strings.Contains(got, "D08 {fact:computed/") {
		t.Errorf("findings:\n%s", got)
	}
}
