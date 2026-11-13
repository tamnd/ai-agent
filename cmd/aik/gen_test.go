// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGen(t *testing.T) {
	out := t.TempDir()
	if got := run([]string{"gen", "-root", corpus, "-out", out, "-lang", "go,python"}); got != 0 {
		t.Fatalf("aik gen exits with %d", got)
	}
	dir := filepath.Join(out, "go", "searching", "sequential-and-binary-search", "binary-search")
	for _, f := range []string{"go.mod", "binary_search.go", "binary_search_test.go", "aik-test.sh"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Errorf("the tree has no %s: %v", f, err)
		}
	}
	b, err := os.ReadFile(filepath.Join(out, "go", "SOURCES"))
	if err != nil {
		t.Fatal(err)
	}
	want := "searching/sequential-and-binary-search/binary-search/binary_search.go\tnodes/searching/sequential-and-binary-search/binary-search/impl/go/binary_search.go\n"
	if !strings.Contains(string(b), want) {
		t.Errorf("SOURCES has no line %q:\n%s", want, b)
	}
	if _, err := os.Stat(filepath.Join(corpus, "nodes", "searching", "sequential-and-binary-search", "binary-search", "impl", "go", "go.mod")); err == nil {
		t.Error("aik gen wrote go.mod into the content tree")
	}

	// A second run for one node keeps the SOURCES lines of the other nodes.
	if got := run([]string{"gen", "-root", corpus, "-out", out, "-lang", "go", "binary-search"}); got != 0 {
		t.Fatalf("aik gen binary-search exits with %d", got)
	}
	b2, err := os.ReadFile(filepath.Join(out, "go", "SOURCES"))
	if err != nil {
		t.Fatal(err)
	}
	if string(b2) != string(b) {
		t.Errorf("SOURCES changed after a run for one node:\n%s\nwas\n%s", b2, b)
	}
}
