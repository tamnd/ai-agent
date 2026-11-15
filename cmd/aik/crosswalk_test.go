// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCrosswalk(t *testing.T) {
	root := copyCorpus(t)
	file := filepath.Join(root, "crosswalk", "cs2023-ai.yaml")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	good := "system: CS2023 AI\nrows:\n  - {id: AI-Search, name: Search, maps: [{to: binary-search, match: narrower}]}\n  - {id: AI-Robo, name: Robotics}\n"
	if err := os.WriteFile(file, []byte(good), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := run([]string{"crosswalk", "check", "-root", root}); code != 0 {
		t.Errorf("check of a good crosswalk exits %d", code)
	}
	if code := run([]string{"crosswalk", "gaps", "-root", root, "-json"}); code != 0 {
		t.Errorf("gaps exits %d", code)
	}
	if err := os.WriteFile(file, []byte(good+"  - {id: AI-ML, name: ML, maps: [{to: nothing-here, match: exact}]}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := run([]string{"crosswalk", "check", "-root", root}); code != 1 {
		t.Errorf("check of a crosswalk with a target that names nothing exits %d, want 1", code)
	}
	if code := run([]string{"crosswalk", "nothing"}); code != 2 {
		t.Errorf("an unknown subcommand exits %d, want 2", code)
	}
}
