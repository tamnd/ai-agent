// SPDX-License-Identifier: Apache-2.0

package node

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFormatNode(t *testing.T) {
	in := "---\r\nname: Heap sort   \r\nslug: heap-sort\r\n# the kind\r\nkind: algorithm\r\nsummary: >-\r\n  One.\r\n\r\n  Two.\r\ntag: NEW-heap-sort\r\nzzz: 1\r\nlevel: core\r\n---\r\n\r\n\r\n## Problem\r\n\r\n\r\n\r\nText.  \r\n\r\n```go\r\na\r\n\r\n\r\nb\r\n```\r\n\r\n\r\n"
	want := "---\ntag: NEW-heap-sort\n# the kind\nkind: algorithm\nslug: heap-sort\nname: Heap sort\nlevel: core\nsummary: >-\n  One.\n\n  Two.\nzzz: 1\n---\n\n## Problem\n\nText.\n\n```go\na\n\n\nb\n```\n"
	got, err := FormatNode([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("got\n%q\nwant\n%q", got, want)
	}
	again, err := FormatNode(got)
	if err != nil || string(again) != want {
		t.Fatalf("a second pass changes the text: %q, %v", again, err)
	}
}

func TestFormatNodeRefusesUnsafeChange(t *testing.T) {
	// A flow sequence over two lines, with the second line at column 0, looks like two keys. Reordering them breaks the YAML.
	in := "---\nzz: [1,\nkind: 2]\ntag: X\n---\n"
	if _, err := FormatNode([]byte(in)); err == nil {
		t.Fatal("FormatNode accepts a front matter that it cannot reorder safely")
	}
}

func TestCorpusIsFormatted(t *testing.T) {
	paths, err := filepath.Glob("../testdata/corpus/nodes/*/*/*/node.md")
	if err != nil || len(paths) == 0 {
		t.Fatalf("no nodes: %v", err)
	}
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		got, err := FormatNode(b)
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		if string(got) != string(b) {
			t.Errorf("%s is not in canonical form", p)
		}
	}
}
