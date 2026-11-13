// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResultsTable(t *testing.T) {
	root := copyCorpus(t)
	files := map[string]string{
		"cards/benchmark/_.jsonl": `{"tag":"9MMLU0","kind":"benchmark","slug":"mmlu","name":"MMLU","versions":[{"id":"original-test","name":"MMLU, test set"}]}` + "\n",
		"cards/result/_.jsonl": `{"tag":"9R0001","kind":"result","model":"model-a","benchmark":"mmlu","version":"original-test","metric":"accuracy","score":{"v":0.886,"unit":"fraction"},"setting":{"shots":5},"source":{"src":"S:a","loc":"Table 2","date":"2024-07-23"},"checks":{"cite":"found"}}` + "\n" +
			`{"tag":"9R0002","kind":"result","model":"model-b","benchmark":"mmlu","version":"original-test","metric":"accuracy","score":{"v":0.5,"unit":"fraction"},"setting":{"shots":"unknown"},"source":{"src":"S:b","loc":"Table 1","date":"2024-07-23"},"checks":{"cite":"found"}}` + "\n",
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
	for _, c := range []struct {
		args []string
		want int
	}{
		{[]string{"results", "table", "-root", root, "mmlu@original-test"}, 0},
		{[]string{"results", "table", "-root", root, "-json", "mmlu@original-test"}, 0},
		{[]string{"results", "table", "-root", root, "mmlu@pro"}, 2},
		{[]string{"results", "table", "-root", root, "mmlu"}, 2},
		{[]string{"results"}, 2},
	} {
		if got := run(c.args); got != c.want {
			t.Errorf("aik %v exits with %d, want %d", c.args, got, c.want)
		}
	}
}
