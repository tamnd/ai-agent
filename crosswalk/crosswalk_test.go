// SPDX-License-Identifier: Apache-2.0

package crosswalk

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/ai-agent/node"
)

const ccs = `system: ACM CCS 2012
version: "2012"
source: https://dl.acm.org/ccs
licence: free to use with credit
paths:
  - {id: acm-ccs-ml, title: Machine learning by ACM CCS, under: "10010257", depth: 1}
rows:
  - {id: "10010147", name: Computing methodologies}
  - {id: "10010257", name: Machine learning, parent: "10010147"}
  - id: "10010258"
    name: Learning paradigms
    parent: "10010257"
    maps: [{to: binary-search, match: related}]
  - id: "10010259"
    name: Supervised learning
    parent: "10010258"
    maps: [{to: area:searching, match: broader}]
  - {id: "10010293", name: Machine learning approaches, parent: "10010257"}
  - {id: "10010178", name: Artificial intelligence, parent: "10010147", note: the AI part has its own path}
`

func corpus(t *testing.T, files map[string]string) *node.Corpus {
	t.Helper()
	root := filepath.Join(t.TempDir(), "corpus")
	if err := os.CopyFS(root, os.DirFS("../testdata/corpus")); err != nil {
		t.Fatal(err)
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
	return c
}

func TestLoadAndSections(t *testing.T) {
	c := corpus(t, map[string]string{
		"crosswalk/acm-ccs-2012.yaml": ccs,
		"crosswalk/dblp.yaml":         "venues: [iclr]\n", // another kind of file
	})
	cws, err := Load(c.Root)
	if err != nil {
		t.Fatal(err)
	}
	if len(cws) != 1 || cws[0].Name != "acm-ccs-2012" || len(cws[0].Rows) != 6 {
		t.Fatalf("crosswalks = %+v", cws)
	}
	if probs := Check(c, cws); len(probs) != 0 {
		t.Errorf("a good crosswalk has problems: %v", probs)
	}
	gaps := Gaps(c, cws)
	var ids []string
	for _, g := range gaps {
		ids = append(ids, g.ID)
	}
	// The rows above the leaves map to nothing and are gaps too; the row with a note is not.
	if strings.Join(ids, " ") != "10010147 10010257 10010293" {
		t.Errorf("gaps = %v", ids)
	}
	secs, err := cws[0].Sections(c, cws[0].Paths[0])
	if err != nil {
		t.Fatal(err)
	}
	if len(secs) != 2 || secs[0].Row.ID != "10010258" || secs[1].Row.ID != "10010293" {
		t.Fatalf("sections = %+v", secs)
	}
	if s := secs[0]; len(s.Items) != 2 || s.Items[0].Node == nil || s.Items[0].Node.Slug != "binary-search" || s.Items[1].Area != "searching" || len(s.Rows) != 2 {
		t.Errorf("the section of depth 1 does not join its child: %+v", s)
	}
	all, _ := cws[0].Sections(c, Path{ID: "x", Title: "x"})
	if len(all) != 6 {
		t.Errorf("a path with no under and no depth has a section for each row: %d", len(all))
	}
	if _, err := cws[0].Sections(c, Path{ID: "x", Under: "nothing"}); err == nil {
		t.Error("a path under no row has sections")
	}
}

func TestCheck(t *testing.T) {
	bad := strings.NewReplacer("system: ACM CCS 2012\n", "", "match: related", "match: same", "area:searching", "area:nowhere",
		`parent: "10010257"}`, `parent: "99"}`, `under: "10010257"`, `under: "88"`).Replace(ccs) +
		"  - {id: \"10010258\", name: twice}\n  - {id: \"\", name: no id}\n"
	c := corpus(t, map[string]string{"crosswalk/acm-ccs-2012.yaml": bad, "crosswalk/two.yaml": "system: two\npaths: [{id: acm-ccs-ml, title: again}]\nrows: []\n"})
	cws, err := Load(c.Root)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(Check(c, cws), "\n")
	for _, want := range []string{"no system", `the match "same"`, "area:nowhere is not", "the parent 99", "under 88", "the id is twice", "needs an id and a name", "also in crosswalk/acm-ccs-2012.yaml"} {
		if !strings.Contains(got, want) {
			t.Errorf("Check misses %q:\n%s", want, got)
		}
	}
	c = corpus(t, map[string]string{"crosswalk/x.yaml": "system: x\nrows: []\nextra: 1\n"})
	if _, err := Load(c.Root); err == nil {
		t.Error("a crosswalk with an unknown key loads")
	}
}
