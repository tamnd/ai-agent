// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// inProcess makes aik build run its steps in this process, since the test binary is not aik.
func inProcess(t *testing.T) {
	t.Helper()
	old := runSelf
	runSelf = func(args []string) ([]byte, error) {
		if code := run(args); code != 0 {
			return []byte(fmt.Sprintf("aik %s exits with %d\n", args[0], code)), fmt.Errorf("exit %d", code)
		}
		return nil, nil
	}
	t.Cleanup(func() { runSelf = old })
}

func TestBuild(t *testing.T) {
	inProcess(t)
	root := copyCorpus(t)
	if got := run([]string{"build", "-root", root, "-no-tests"}); got != 0 {
		t.Fatalf("aik build exits with %d", got)
	}
	for _, p := range []string{"build/status.json", "build/coverage.json", "build/cache/build.json", "build/logs/check.log"} {
		if _, err := os.Stat(filepath.Join(root, p)); err != nil {
			t.Errorf("aik build wrote no %s", p)
		}
	}
	var cache buildCache
	b, err := os.ReadFile(filepath.Join(root, "build/cache/build.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &cache); err != nil || cache["check"] == "" || cache["coverage"] == "" {
		t.Errorf("the cache %s does not hold check and coverage: %v", b, err)
	}

	// A step whose output is gone runs again; a changed tree runs every step again.
	if err := os.Remove(filepath.Join(root, "build/status.json")); err != nil {
		t.Fatal(err)
	}
	if got := run([]string{"build", "-root", root, "-no-tests"}); got != 0 {
		t.Fatalf("aik build again exits with %d", got)
	}
	if _, err := os.Stat(filepath.Join(root, "build/status.json")); err != nil {
		t.Errorf("the coverage step did not run again when its output was gone")
	}

	// A failing step fails the build at the end, and the steps after it still run.
	runSelf = func(args []string) ([]byte, error) {
		if args[0] == "check" {
			return []byte("T08 a problem\n"), fmt.Errorf("exit 1")
		}
		return nil, nil
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := run([]string{"build", "-root", root, "-no-tests"}); got != 1 {
		t.Errorf("aik build with a failing check exits with %d, want 1", got)
	}
	log, _ := os.ReadFile(filepath.Join(root, "build/logs/check.log"))
	if !strings.Contains(string(log), "T08") {
		t.Errorf("the log of check is %q", log)
	}
}

func TestCoverageAndReleaseUsage(t *testing.T) {
	root := copyCorpus(t)
	for _, tc := range []struct {
		args []string
		want int
	}{
		{[]string{"coverage", "-root", root}, 0},
		{[]string{"coverage", "-root", root, "-json", "-o", "build/coverage.json"}, 0},
		{[]string{"coverage", "-root", root, "extra"}, 2},
		{[]string{"build", "-root", root, "extra"}, 2},
		{[]string{"release"}, 2},
		{[]string{"release", "cut", "-root", root}, 2},
		{[]string{"release", "cut", "-root", root, "2027.3"}, 2},
		{[]string{"release", "cut", "-root", root, "2027.03"}, 1}, // the corpus is not a git repository
	} {
		if got := run(tc.args); got != tc.want {
			t.Errorf("aik %s exits with %d, want %d", strings.Join(tc.args, " "), got, tc.want)
		}
	}
	var r struct {
		Levels struct {
			Total [7]int `json:"total"`
		} `json:"levels"`
	}
	b, err := os.ReadFile(filepath.Join(root, "build/coverage.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &r); err != nil || r.Levels.Total[1] == 0 {
		t.Errorf("build/coverage.json has no C1 items: %v", err)
	}
}

func TestReleaseCut(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	for _, k := range []string{"GIT_AUTHOR_NAME", "GIT_COMMITTER_NAME"} {
		t.Setenv(k, "test")
	}
	for _, k := range []string{"GIT_AUTHOR_EMAIL", "GIT_COMMITTER_EMAIL"} {
		t.Setenv(k, "test@example.invalid")
	}
	root := copyCorpus(t)
	for _, a := range [][]string{{"init", "-q"}, {"add", "."}, {"commit", "-q", "-m", "content"}} {
		cmd := exec.Command("git", append([]string{"-C", root}, a...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", a, err, out)
		}
	}
	if got := run([]string{"release", "cut", "-root", root, "-no-tag", "2027.03"}); got != 0 {
		t.Fatalf("aik release cut exits with %d", got)
	}
	b, err := os.ReadFile(filepath.Join(root, "manifests/releases/2027.03.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`version: "2027.03"`, "commit: ", "aik: ", "coverage: [0, 5, 0, 0, 0, 0, 0]"} {
		if !strings.Contains(string(b), want) {
			t.Errorf("the manifest has no %q:\n%s", want, b)
		}
	}
	if out, _ := exec.Command("git", "-C", root, "tag").Output(); len(out) > 0 {
		t.Errorf("-no-tag made the tags %s", out)
	}
	if got := run([]string{"release", "cut", "-root", root, "-no-tag", "-allow-dirty", "2027.03"}); got != 1 {
		t.Errorf("cutting a release again exits with %d, want 1", got)
	}
}
