// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBridgeSync(t *testing.T) {
	root := copyCorpus(t)
	idx := filepath.Join(t.TempDir(), "nodes.json")
	data := `{"commit":"abc","nodes":[{"tag":"5KMEAN","kind":"algorithm","slug":"k-means","name":"k-means","area":"machine-learning/clustering","url":"https://algorithm.tamnd.com/k-means/"},{"tag":"QSORT1","kind":"algorithm","slug":"quicksort","name":"Quicksort","area":"sorting","url":"https://algorithm.tamnd.com/quicksort/"}]}`
	if err := os.WriteFile(idx, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	shard := filepath.Join(root, "cards", "bridge", "al", "_.jsonl")
	if code := run([]string{"bridge", "sync", "-root", root, "-n", "-from", idx, "al"}); code != 0 {
		t.Fatalf("the dry run exits with %d", code)
	}
	if _, err := os.Stat(shard); !os.IsNotExist(err) {
		t.Fatalf("the dry run wrote the shard: %v", err)
	}
	if code := run([]string{"bridge", "sync", "-root", root, "-from", idx, "al"}); code != 0 {
		t.Fatalf("the sync exits with %d", code)
	}
	b, err := os.ReadFile(shard)
	if err != nil {
		t.Fatal(err)
	}
	if s := string(b); !strings.Contains(s, `"slug":"k-means"`) || strings.Contains(s, "quicksort") {
		t.Errorf("the shard is:\n%s", s)
	}
	if code := run([]string{"bridge", "sync", "-root", root, "xx"}); code != 2 {
		t.Errorf("an unknown site exits with %d, not 2", code)
	}
	if code := run([]string{"check", "-root", root}); code != 0 {
		t.Errorf("aik check exits with %d after the sync", code)
	}
}
