// SPDX-License-Identifier: Apache-2.0

package harvest

import (
	"os"
	"reflect"
	"testing"
)

func TestACLFiles(t *testing.T) {
	b, err := os.ReadFile("testdata/acl-tree.json")
	if err != nil {
		t.Fatal(err)
	}
	venues := map[string]bool{"acl": true, "findings": true, "coling": true}
	fs, err := ACLFiles(b, venues)
	if err != nil {
		t.Fatal(err)
	}
	want := []ACLFile{{"2023.acl.xml", 2023}, {"2020.findings.xml", 2020}, {"P19.xml", 2019}, {"C65.xml", 1965}}
	if !reflect.DeepEqual(fs, want) {
		t.Errorf("files %v, want %v", fs, want)
	}
	if _, err := ACLFiles([]byte(`{"message":"API rate limit exceeded"}`), venues); err == nil {
		t.Error("an answer with no tree is not an error")
	}
	if _, err := ACLFiles([]byte(`{"truncated":true,"tree":[]}`), venues); err == nil {
		t.Error("a truncated tree is not an error")
	}
}

func TestACLID(t *testing.T) {
	for _, c := range []struct{ coll, vol, paper, want string }{
		{"2023.acl", "long", "1", "2023.acl-long.1"},
		{"P19", "1", "78", "P19-1078"},
		{"D19", "50", "1", "D19-5001"},
		{"J19", "1", "1", "J19-1001"},
	} {
		if got := ACLID(c.coll, c.vol, c.paper); got != c.want {
			t.Errorf("ACLID(%s, %s, %s) = %s, want %s", c.coll, c.vol, c.paper, got, c.want)
		}
	}
}

func TestACLRows(t *testing.T) {
	b, err := os.ReadFile("testdata/acl-P19.xml")
	if err != nil {
		t.Fatal(err)
	}
	id, vols, err := ACLCollection(b)
	if err != nil {
		t.Fatal(err)
	}
	rows := ACLRows(id, vols, map[string]bool{"acl": true}, "s")
	if len(rows) != 2 {
		t.Fatalf("%d rows, want 2 (no frontmatter, no workshop): %+v", len(rows), rows)
	}
	r := rows[0]
	if r.ID != "P19-1078" || r.SameAs["doi"] != "10.18653/v1/p19-1078" || r.URL != "https://aclanthology.org/P19-1078/" {
		t.Errorf("ids of %+v", r)
	}
	if r.Fields["venue"] != "acl" || r.Fields["year"] != "2019" ||
		r.Fields["volume"] != "Proceedings of the 57th Annual Meeting of the Association for Computational Linguistics" {
		t.Errorf("fields %v", r.Fields)
	}
	if !reflect.DeepEqual(r.Fields["authors"], []string{"Chien-Sheng Wu", "Andrea Madotto"}) ||
		!reflect.DeepEqual(r.Links["author"], []string{"chien-sheng-wu"}) {
		t.Errorf("authors %v %v", r.Fields["authors"], r.Links)
	}
	if !reflect.DeepEqual(r.Labels, []string{"Outstanding Paper"}) || r.Signals["awards"] != 1 {
		t.Errorf("awards %v %v", r.Labels, r.Signals)
	}
	if rows[1].Name != `Learning with \ell_1 norms` || rows[1].Signals != nil {
		t.Errorf("second row %+v", rows[1])
	}
	if _, _, err := ACLCollection([]byte("<html>not a collection</html>")); err == nil {
		t.Error("a page with no collection id is not an error")
	}
}

func TestMainAward(t *testing.T) {
	for name, want := range map[string]bool{
		"Best Paper": true, "Outstanding Paper Award": true, "Test of Time Award": true, "Best Resource Paper": true,
		"SAC Highlight Award": false, "Best Demo Paper Runner-Up": false, "People's Choice Award": false,
		"SAC Award: Generation": false, "Honorable Mention for Best Paper": false,
	} {
		if MainAward(name) != want {
			t.Errorf("MainAward(%q) = %v, want %v", name, !want, want)
		}
	}
}
