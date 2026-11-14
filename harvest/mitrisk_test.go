// SPDX-License-Identifier: Apache-2.0

package harvest

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestMITRiskRows(t *testing.T) {
	rows, notes, skipped, err := MITRiskRows(fixture(t, "mitrisk-domains.csv"), fixture(t, "mitrisk-db.csv"),
		fixture(t, "mitrisk-papers.csv"), [3]string{"b3:d", "b3:r", "b3:p"})
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]Row{}
	kinds := map[string]int{}
	for _, r := range rows {
		by[r.ID] = r
		kinds[r.Kind]++
		for k := range r.Fields {
			if k == "description" {
				t.Errorf("row %s keeps the description", r.ID)
			}
		}
	}
	// 2 domains and 5 sub-domains, 3 documents, 6 categories of Critch2023 and 4 categories and 13 sub-categories of
	// Cui2024.
	if len(rows) != 33 || kinds["work"] != 3 || skipped != 1 {
		t.Fatalf("got %d rows (%v) and %d skipped: %+v", len(rows), kinds, skipped, rows)
	}
	if !reflect.DeepEqual(notes, []string{"AI Risk Database updated 03 December 2025",
		"the documents paper:1, paper:3 share the DOI 10.48550/arxiv.2306.06924, which is dropped from them",
		"12 rows of additional evidence are quotes, not risks, and are not read"}) {
		t.Errorf("notes %q", notes)
	}
	if d := by["domain:1"]; d.Name != "Discrimination & Toxicity" || d.Kind != "risk" || d.Snapshot != "b3:d" || d.Fields["level"] != "domain" {
		t.Errorf("domain 1 is %+v", d)
	}
	if d := by["domain:4.2"]; d.Name != "Cyberattacks, weapon development or use, and mass harm" || d.Links["domain"][0] != "domain:4" {
		t.Errorf("sub-domain 4.2 is %+v", d)
	}
	if by["domain:2.1"].ID != "" {
		t.Error("a sub-domain outside the fixture was read")
	}
	if w := by["paper:1"]; w.Kind != "work" || w.SameAs != nil || w.Facts["year"].V != "2023" ||
		w.URL != "https://arxiv.org/abs/2306.06924" || w.Snapshot != "b3:p" {
		t.Errorf("paper 1, whose DOI paper 3 shares, is %+v", w)
	}
	if w := by["paper:2"]; w.SameAs["arxiv"] != "2401.05778" || w.SameAs["doi"] != "10.48550/arxiv.2401.05778" {
		t.Errorf("paper 2 is %+v", w)
	}
	c := by["risk:01.01.00"]
	if c.Name != "Diffusion of responsibility" || !reflect.DeepEqual(c.Aliases, []string{"Type 1: Diffusion of responsibility"}) ||
		c.Links["named_in"][0] != "paper:1" || c.Links["domain"][0] != "domain:6.5" || c.Fields["level"] != "risk category" ||
		c.Facts["entity"].V != "2 - AI" || c.Snapshot != "b3:r" {
		t.Errorf("risk 01.01.00 is %+v", c)
	}
	s := by["risk:02.01.01"]
	if s.Name != "Bias" || s.Links["category"][0] != "risk:02.01.00" || s.Fields["level"] != "risk sub-category" || s.Fields["quickref"] != "Cui2024" {
		t.Errorf("risk 02.01.01 is %+v", s)
	}
	for _, r := range rows {
		if strings.HasSuffix(r.ID, ".a") || r.ID == "risk:01.00.00" {
			t.Errorf("row %s is not a risk", r.ID)
		}
	}
	var e *ErrSchema
	if _, _, _, err := MITRiskRows([]byte("a,b\n"), nil, nil, [3]string{}); !errors.As(err, &e) {
		t.Errorf("a sheet with no header: %v", err)
	}
	old := []byte("x,⚠️IMPORTANT NOTE: This version of the repository is now outdated.\n,,Domain,,Sub-domain\n")
	if _, _, _, err := MITRiskRows(old, nil, nil, [3]string{}); !errors.As(err, &e) || !strings.Contains(err.Error(), "outdated") {
		t.Errorf("an outdated sheet: %v", err)
	}
}

func TestMITRiskName(t *testing.T) {
	for in, want := range map[string]string{
		"Type 1: Diffusion of responsibility": "Diffusion of responsibility",
		"Erosion of international law;":       "Erosion of international law",
		"Risk 3 - Misuse":                     "Misuse",
		"Hate/Toxicity (Harassment)":          "Hate/Toxicity (Harassment)",
		"Typecasting":                         "Typecasting",
	} {
		if got := mitRiskName(in); got != want {
			t.Errorf("mitRiskName(%q) = %q, want %q", in, got, want)
		}
	}
	for in, want := range map[string]string{"7.3 > Lack of capability": "domain:7.3", "4.0 > Malicious use": "domain:4", "X.1 > Excluded": "", "": ""} {
		if got := mitRiskDomain(in); got != want {
			t.Errorf("mitRiskDomain(%q) = %q, want %q", in, got, want)
		}
	}
}
