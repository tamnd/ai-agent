// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMv(t *testing.T) {
	root := copyCorpus(t)
	if got := run([]string{"mv", "binary-search", "binary-search-on-arrays", "-root", root}); got != 0 {
		t.Fatalf("aik mv exits with %d", got)
	}
	newDir := filepath.Join(root, "nodes/searching/sequential-and-binary-search/binary-search-on-arrays")
	if _, err := os.Stat(filepath.Join(newDir, "impl")); err != nil {
		t.Fatalf("the node did not move with its files: %v", err)
	}
	if exists(filepath.Join(root, "nodes/searching/sequential-and-binary-search/binary-search")) {
		t.Fatal("the old directory is still there")
	}
	md, _ := os.ReadFile(filepath.Join(newDir, "node.md"))
	if !strings.Contains(string(md), "\nslug: binary-search-on-arrays\n") || !strings.Contains(string(md), "\ntag: 2B7QDX\n") {
		t.Errorf("node.md has the wrong slug or tag:\n%s", md[:200])
	}
	li, _ := os.ReadFile(filepath.Join(root, "nodes/foundations/correctness-and-invariants/loop-invariant/node.md"))
	if !strings.Contains(string(li), "](/binary-search-on-arrays/)") || strings.Contains(string(li), "](/binary-search/)") {
		t.Error("the link in loop-invariant still names the old slug")
	}
	reg, _ := os.ReadFile(filepath.Join(root, "registers/tags.reg"))
	if !strings.Contains(string(reg), "2B7QDX binary-search-on-arrays\n") {
		t.Errorf("tags.reg is\n%s", reg)
	}
	al, _ := os.ReadFile(filepath.Join(root, "registers/aliases.reg"))
	if !strings.HasSuffix(string(al), "\nbinary-search binary-search-on-arrays\n") {
		t.Errorf("aliases.reg is\n%s", al)
	}
	if got := run([]string{"check", "-root", root}); got != 0 {
		t.Fatalf("aik check exits with %d after the move", got)
	}
	out := t.TempDir()
	if got := run([]string{"site", "build", "-root", root, "-out", out}); got != 0 {
		t.Fatalf("aik site build exits with %d", got)
	}
	red, _ := os.ReadFile(filepath.Join(out, "_redirects"))
	for _, want := range []string{"/binary-search/* /binary-search-on-arrays/:splat 301\n", "/binary-search /binary-search-on-arrays/ 301\n"} {
		if !strings.Contains(string(red), want) {
			t.Errorf("_redirects has no line %q", want)
		}
	}
	if moved, _ := os.ReadFile(filepath.Join(out, "api", "moved.json")); !strings.Contains(string(moved), `"/t/2B7QDX":"/binary-search-on-arrays/"`) {
		t.Errorf("moved.json has no tag of the moved node:\n%s", moved)
	}

	// A second move points the first old slug to the newest slug, so no redirect is a chain.
	if got := run([]string{"mv", "binary-search-on-arrays", "bisection", "-root", root}); got != 0 {
		t.Fatalf("the second aik mv exits with %d", got)
	}
	al, _ = os.ReadFile(filepath.Join(root, "registers/aliases.reg"))
	if !strings.Contains(string(al), "\nbinary-search bisection\nbinary-search-on-arrays bisection\n") {
		t.Errorf("aliases.reg after two moves is\n%s", al)
	}
	if got := run([]string{"mv", "bisection", "binary-search", "-root", root}); got != 2 {
		t.Errorf("a move back to an old slug exits with %d, not 2", got)
	}
}

func TestMvArea(t *testing.T) {
	root := copyCorpus(t)
	if got := run([]string{"mv", "loop-invariant", "foundations/lower-bounds", "-root", root}); got != 0 {
		t.Fatalf("aik mv exits with %d", got)
	}
	md, err := os.ReadFile(filepath.Join(root, "nodes/foundations/lower-bounds/loop-invariant/node.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(md), "\narea: foundations/lower-bounds\n") {
		t.Errorf("the area line did not change:\n%s", md[:200])
	}
	if exists(filepath.Join(root, "registers/aliases.reg")) {
		t.Error("a move to another area wrote aliases.reg, but the slug did not change")
	}
	if exists(filepath.Join(root, "nodes/foundations/correctness-and-invariants")) {
		t.Error("the empty area directory is still there")
	}
	if got := run([]string{"check", "-root", root}); got != 0 {
		t.Fatalf("aik check exits with %d after the move", got)
	}
	for _, args := range [][]string{
		{"mv", "loop-invariant", "no/such-area"},
		{"mv", "loop-invariant", "Bad_Slug"},
		{"mv", "loop-invariant", "binary-search"},
		{"mv", "no-such-node", "x"},
	} {
		if got := run(append(args, "-root", root)); got != 2 {
			t.Errorf("aik %s exits with %d, not 2", strings.Join(args, " "), got)
		}
	}
}
