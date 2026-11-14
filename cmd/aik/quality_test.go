// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestQualityLoaders(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, s string) {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("state/a.json", `{"slug":"a","audits":[{"verdict":"fail"},{"verdict":"pass"}]}`)
	write("state/b.json", `{"status":"queued"}`)
	write("state/c.json", `{"audits":[{"verdict":"pass"}]}`)
	au, err := loadAudits(filepath.Join(dir, "state"))
	if err != nil {
		t.Fatal(err)
	}
	if len(au) != 2 || len(au["a"]) != 2 || au["a"][0] != "fail" || au["c"][0] != "pass" {
		t.Errorf("audits %v", au)
	}
	if au, err := loadAudits(filepath.Join(dir, "none")); err != nil || len(au) != 0 {
		t.Errorf("a missing state directory gives %v, %v", au, err)
	}

	if l, err := loadLinks(filepath.Join(dir, "reports", "audit.json")); err != nil || l != nil {
		t.Errorf("a missing report gives %v, %v", l, err)
	}
	write("reports/audit.json", `{"check":{},"cite":{},"size":{},"links":{"checked":10,"dead":[{"url":"https://example.org/x","file":"nodes/a/node.md"}]}}`)
	l, err := loadLinks(filepath.Join(dir, "reports", "audit.json"))
	if err != nil || l == nil || l.Checked != 10 || len(l.Dead) != 1 {
		t.Errorf("links %+v, %v", l, err)
	}

	if ws, err := loadWaivers(dir); err != nil || ws != nil {
		t.Errorf("a missing register gives %v, %v", ws, err)
	}
	write("registers/waivers.yaml", "- rule: W01\n  item: NEW-a\n  end: 2027-01-01\n")
	if ws, err := loadWaivers(dir); err != nil || len(ws) != 1 || ws[0].End != "2027-01-01" {
		t.Errorf("waivers %v, %v", ws, err)
	}
	write("registers/waivers.yaml", "waivers:\n  - rule: W01\n    end: 2027-01-01\n  - rule: W02\n    end: 2026-01-01\n")
	if ws, err := loadWaivers(dir); err != nil || len(ws) != 2 {
		t.Errorf("waivers under a key %v, %v", ws, err)
	}
	write("registers/waivers.yaml", "waivers: []\n")
	if ws, err := loadWaivers(dir); err != nil || ws == nil || len(ws) != 0 {
		t.Errorf("an empty register %v, %v", ws, err)
	}
}
