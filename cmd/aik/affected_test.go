// SPDX-License-Identifier: Apache-2.0

package main

import (
	"testing"

	"github.com/tamnd/ai-agent/node"
	"github.com/tamnd/ai-agent/status"
)

func TestFindAffected(t *testing.T) {
	c, err := node.Load(corpus)
	if err != nil {
		t.Fatal(err)
	}
	bs := c.BySlug["binary-search"]
	const dir = "nodes/searching/sequential-and-binary-search/"
	cases := []struct {
		name  string
		files []string
		all   bool
		has   map[string]bool // lang of binary-search
	}{
		{"one go file", []string{dir + "binary-search/impl/go/binary_search.go"}, false, map[string]bool{"go": true, "python": false}},
		{"node.md", []string{dir + "binary-search/node.md"}, false, map[string]bool{"go": true, "python": true}},
		{"a case file", []string{dir + "search-in-sorted-array/cases/basic.jsonl"}, false, map[string]bool{"go": true, "python": true}},
		{"a register", []string{"registers/languages.yaml"}, true, map[string]bool{"go": true, "python": true}},
		{"the readme", []string{"README.md"}, false, map[string]bool{"go": false, "python": false}},
	}
	for _, tc := range cases {
		a := findAffected(c, tc.files)
		if a.All != tc.all {
			t.Errorf("%s: All = %v", tc.name, a.All)
		}
		for l, want := range tc.has {
			if got := a.Has(bs, l); got != want {
				t.Errorf("%s: Has(binary-search, %s) = %v, want %v", tc.name, l, got, want)
			}
		}
	}
}

func TestTestCache(t *testing.T) {
	c, err := node.Load(corpus)
	if err != nil {
		t.Fatal(err)
	}
	bs := c.BySlug["binary-search"]
	im := *bs.Lang("go")
	tc, err := newTestCache(t.TempDir(), corpus)
	if err != nil {
		t.Fatal(err)
	}
	k1 := tc.key("go1.27", bs, im, nil)
	if k1 != tc.key("go1.27", bs, im, nil) {
		t.Fatal("the key is not stable")
	}
	if k1 == tc.key("go1.28", bs, im, nil) {
		t.Error("the key does not change with the toolchain")
	}
	changed := im
	changed.Files = append([]node.File(nil), im.Files...)
	changed.Files[0].Content += "\n"
	if k1 == tc.key("go1.27", bs, changed, nil) {
		t.Error("the key does not change with a file")
	}
	if err := tc.Put("go", "bad", status.Test{Slug: "binary-search", Pass: false}); err != nil {
		t.Fatal(err)
	}
	if _, ok := tc.Get("go", "bad"); ok {
		t.Error("a failed result is in the cache")
	}
	good := status.Test{Slug: "binary-search", Lang: "go", Pass: true}
	if err := tc.Put("go", k1, good); err != nil {
		t.Fatal(err)
	}
	if got, ok := tc.Get("go", k1); !ok || !got.Pass {
		t.Errorf("Get = %v, %v", got, ok)
	}
	tc2, _ := newTestCache(tc.dir, corpus)
	tc2.Prune([]string{"go"})
	if _, ok := tc2.Get("go", k1); ok {
		t.Error("Prune kept an entry that the run did not use")
	}
}
