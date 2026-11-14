// SPDX-License-Identifier: Apache-2.0

package sample

import (
	"fmt"
	"reflect"
	"testing"
	"time"
)

func TestWeek(t *testing.T) {
	for _, c := range []struct{ w, from string }{
		{"2027-W14", "2027-04-05"},
		{"2026-W01", "2025-12-29"},
		{"2026-W53", "2026-12-28"},
		{"2021-W01", "2021-01-04"},
	} {
		from, to, err := Week(c.w)
		if err != nil || from.Format("2006-01-02") != c.from || to.Sub(from) != 7*24*time.Hour {
			t.Errorf("Week(%s) = %v %v %v, want %s", c.w, from, to, err, c.from)
		}
		if got := WeekOf(from.Add(36 * time.Hour)); got != c.w {
			t.Errorf("WeekOf(%v) = %s", from, got)
		}
	}
	for _, w := range []string{"2027-14", "2027-W00", "2027-W53", "27-W01"} {
		if _, _, err := Week(w); err == nil {
			t.Errorf("Week(%s) gives no error", w)
		}
	}
}

func TestDraw(t *testing.T) {
	var pages []Page
	for i := range 40 {
		pages = append(pages, Page{ID: fmt.Sprintf("n%02d", i), Form: "node", Kind: "method", Changed: i < 20, Core: i < 10})
	}
	pages = append(pages,
		Page{ID: "dropout", Form: "node", Kind: "method", Changed: true, Gold: true, Core: true},
		Page{ID: "eu-ai-act", Form: "node", Kind: "policy", Changed: true},
		Page{ID: "chinese-room", Form: "node", Kind: "argument", Changed: true, Contested: true},
		Page{ID: "alan-turing", Form: "card", Kind: "person", Changed: true, People: true},
		Page{ID: "old-model", Form: "card", Kind: "model", Changed: true, Released: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)},
		Page{ID: "new-model", Form: "card", Kind: "model", Changed: true, Released: time.Date(2027, 2, 1, 0, 0, 0, 0, time.UTC)},
	)
	for i := range 120 {
		pages = append(pages, Page{ID: fmt.Sprintf("e%03d", i), Form: "card", Kind: "model", New: true, Changed: true, Source: "epoch"})
	}
	for i := range 10 {
		pages = append(pages, Page{ID: fmt.Sprintf("h%d", i), Form: "card", Kind: "result", New: true, Changed: true, Source: "helm"})
	}
	r, err := Draw("2027-W14", pages)
	if err != nil {
		t.Fatal(err)
	}
	took := map[string]int{}
	for _, c := range r.Counts {
		took[c.Stratum] = c.Took
	}
	want := map[string]int{
		"gold": 1, "core": 2, // 11 changed core nodes, 10% rounded up
		"contested": 1, "policy": 1, "person": 1, "recent-model": 1,
		"new-cards": 60, // 50 of epoch's 120, all 10 of helm's
	}
	for k, v := range want {
		if took[k] != v {
			t.Errorf("the stratum %s took %d, want %d", k, took[k], v)
		}
	}
	at := map[string]Pick{}
	for _, p := range r.Picks {
		if _, dup := at[p.Form+":"+p.ID]; dup {
			t.Errorf("%s is taken twice", p.ID)
		}
		at[p.Form+":"+p.ID] = p
	}
	if p := at["node:dropout"]; p.Stratum != "gold" {
		t.Errorf("the gold page %+v", p)
	}
	if p, ok := at["card:new-model"]; !ok || p.Stratum != "recent-model" {
		t.Errorf("the recent model %+v", p)
	}
	if p, ok := at["card:old-model"]; ok && p.Stratum == "recent-model" {
		t.Error("a model of 2020 is in the recent-model stratum")
	}
	if r.Changed != len(pages)-20 || r.New != 130 {
		t.Errorf("changed %d new %d", r.Changed, r.New)
	}

	// The same week gives the same sample; another week another one.
	again, _ := Draw("2027-W14", pages)
	if !reflect.DeepEqual(r, again) {
		t.Error("the sample of a week is not the same twice")
	}
	other, _ := Draw("2027-W15", pages)
	if reflect.DeepEqual(r.Picks, other.Picks) {
		t.Error("two weeks give the same sample")
	}
}

func TestCardChanges(t *testing.T) {
	diff := `diff --git a/cards/model/_.jsonl b/cards/model/_.jsonl
--- a/cards/model/_.jsonl
+++ b/cards/model/_.jsonl
@@ -1 +1,2 @@
-{"tag":"AAAAAA","kind":"model","slug":"a","name":"A"}
+{"tag":"AAAAAA","kind":"model","slug":"a","name":"A2"}
+{"tag":"BBBBBB","kind":"model","slug":"b","name":"B","harvest":{"source":"epoch"}}
diff --git a/cards/model/C.jsonl b/cards/model/C.jsonl
@@ -3 +0,0 @@
-{"tag":"CCCCCC","kind":"model","slug":"c","name":"C"}
`
	changed, added := CardChanges(diff)
	if !reflect.DeepEqual(changed, map[string]bool{"AAAAAA": true, "BBBBBB": true}) || !reflect.DeepEqual(added, map[string]bool{"BBBBBB": true}) {
		t.Errorf("changed %v added %v", changed, added)
	}
}
