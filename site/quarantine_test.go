// SPDX-License-Identifier: Apache-2.0

package site

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tamnd/ai-agent/node"
	"github.com/tamnd/ai-agent/prov"
)

func TestQuarantine(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../testdata/corpus")); err != nil {
		t.Fatal(err)
	}
	by := prov.Author{Login: "tamnd"}
	// The Python file of binary search is hidden, then the whole node of search-in-sorted-array; a line of a file
	// that a later line clears is not hidden.
	if err := prov.Append(root, "2B7QDX", []prov.Line{
		{Date: "2026-11-15", File: "impl/python/binary_search.py", Op: prov.Quarantine, Author: by, Note: "reported copy"},
		{Date: "2026-11-15", File: "node.md", Op: prov.Quarantine, Author: by},
		{Date: "2026-11-16", File: "node.md", Op: prov.Confirm, Author: by, Hash: prov.Hash(nil)},
	}); err != nil {
		t.Fatal(err)
	}
	c, err := node.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	other := c.BySlug["search-in-sorted-array"]
	if other == nil || other.Tag == "" {
		t.Fatal("no node search-in-sorted-array with a tag")
	}
	if err := prov.Append(root, other.Tag, []prov.Line{{Date: "2026-11-14", File: prov.Whole, Op: prov.Quarantine, Author: by}}); err != nil {
		t.Fatal(err)
	}
	bs := c.BySlug["binary-search"]
	pyBefore := bs.Lang("python") != nil

	cc, wd, err := withdraw(c)
	if err != nil {
		t.Fatal(err)
	}
	if c.BySlug["binary-search"] != bs || (bs.Lang("python") != nil) != pyBefore {
		t.Error("withdraw changed the corpus that it was given")
	}
	if w := wd["binary-search"]; w.Whole || len(w.Files) != 1 || w.Files[0] != "impl/python/binary_search.py" {
		t.Errorf("binary-search: %+v", w)
	}
	if pyBefore && cc.BySlug["binary-search"].Lang("python") != nil {
		t.Error("the quarantined Python file is still on the page")
	}
	if cc.BySlug["binary-search"].Body == "" || cc.BySlug["binary-search"].Lang("go") == nil {
		t.Error("the rest of binary-search is hidden too")
	}
	w := wd["search-in-sorted-array"]
	n := cc.BySlug["search-in-sorted-array"]
	if !w.Whole || w.Since != "2026-11-14" || n.Body != "" || n.Summary != "Withdrawn for review." || cc.ByTag[n.Tag] != n {
		t.Errorf("search-in-sorted-array: %+v, body %q", w, n.Body)
	}
	if !strings.Contains(w.Notice(), "since 2026-11-14") || strings.Contains(w.Notice(), "reported copy") {
		t.Errorf("the notice %q", w.Notice())
	}

	out := filepath.Join(t.TempDir(), "site")
	if _, err := Build(c, Options{Out: out, Commit: "0123456789abcdef", Version: "test", Now: time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatal(err)
	}
	page, err := os.ReadFile(filepath.Join(out, "search-in-sorted-array", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(page), "This page is withdrawn while a person reviews") || !strings.Contains(string(page), `<meta name="robots" content="noindex">`) {
		t.Error("the withdrawn page has no notice or is indexed")
	}
	if _, err := os.Stat(filepath.Join(out, "search-in-sorted-array.md")); err == nil {
		t.Error("the withdrawn page has a Markdown copy")
	}
	idx, err := os.ReadFile(filepath.Join(out, "search.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(idx), "/search-in-sorted-array/") {
		t.Error("the withdrawn page is in the search index")
	}
	bp, err := os.ReadFile(filepath.Join(out, "binary-search", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(bp), "Part of this page is withdrawn") || strings.Contains(string(bp), `content="noindex"`) {
		t.Error("the binary search page has no notice of its hidden part")
	}
}
