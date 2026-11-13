// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckLimits(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	big, err := os.Create(filepath.Join(dir, "big.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if err := big.Truncate(maxFileBytes + 1); err != nil {
		t.Fatal(err)
	}
	big.Close()
	files, problems, err := checkLimits(dir)
	if err != nil || files != 2 || len(problems) != 1 || !strings.HasPrefix(problems[0], "big.bin is") {
		t.Errorf("checkLimits = %d, %q, %v", files, problems, err)
	}
}

func TestAlias(t *testing.T) {
	for in, want := range map[string]string{
		"main":                                "main",
		"Lean-Page":                           "lean-page",
		"feat/site_deploy":                    "feat-site-deploy",
		"a-very-long-branch-name-that-is-cut": "a-very-long-branch-name-that",
	} {
		if got := alias(in); got != want {
			t.Errorf("alias(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDeployNeedsCredentials(t *testing.T) {
	t.Setenv("CLOUDFLARE_API_TOKEN", "")
	t.Setenv("CLOUDFLARE_ACCOUNT_ID", "")
	err := deploy(deployOptions{Dir: t.TempDir(), Project: "p", Branch: "main"})
	if err == nil || !strings.Contains(err.Error(), "CLOUDFLARE_API_TOKEN and CLOUDFLARE_ACCOUNT_ID") {
		t.Errorf("deploy without credentials = %v", err)
	}
	t.Setenv("CLOUDFLARE_API_TOKEN", "x")
	t.Setenv("CLOUDFLARE_ACCOUNT_ID", "y")
	if err := deploy(deployOptions{Dir: t.TempDir(), Project: "p", Branch: "dev"}); err == nil || !strings.Contains(err.Error(), "-preview") {
		t.Errorf("a production deploy of a branch = %v", err)
	}
}

func TestCheckLimitsRedirects(t *testing.T) {
	dir := t.TempDir()
	var s strings.Builder
	for i := 0; i <= maxDynamicRedirects; i++ {
		fmt.Fprintf(&s, "/:slug/l%d/ /:slug/#code-l%d 301\n", i, i)
	}
	for i := 0; i <= maxStaticRedirects; i++ {
		fmt.Fprintf(&s, "/old%d /new%d/ 301\n", i, i)
	}
	if err := os.WriteFile(filepath.Join(dir, "_redirects"), []byte(s.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	_, problems, err := checkLimits(dir)
	if err != nil || len(problems) != 2 {
		t.Errorf("checkLimits = %q, %v", problems, err)
	}
}
