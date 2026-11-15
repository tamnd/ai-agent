// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestReportMatrices(t *testing.T) {
	root := copyCorpus(t)
	for _, r := range []string{"levels", "tests", "experiments"} {
		out := filepath.Join("reports", r+".json")
		if code := run([]string{"report", r, "-root", root, "-o", out}); code != 0 {
			t.Fatalf("aik report %s exits with %d", r, code)
		}
		b, err := os.ReadFile(filepath.Join(root, out))
		if err != nil {
			t.Fatal(err)
		}
		var v map[string]any
		if err := json.Unmarshal(b, &v); err != nil {
			t.Fatalf("%s: %v", out, err)
		}
	}
	if code := run([]string{"report", "levels", "-root", root, "-by", "nothing"}); code == 0 {
		t.Error("-by nothing is accepted")
	}
}
