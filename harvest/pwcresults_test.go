// SPDX-License-Identifier: Apache-2.0

package harvest

import (
	"os"
	"testing"
)

func TestPwCScore(t *testing.T) {
	for _, c := range []struct {
		in   string
		v    float64
		unit string
		ok   bool
	}{
		{"89.79", 89.79, "number", true}, {"84.1%", 84.1, "percent", true}, {"1,234.5 ± 0.2", 1234.5, "number", true},
		{".5", 0.5, "number", true}, {"-", 0, "", false}, {"None", 0, "", false}, {"No", 0, "", false},
	} {
		v, unit, ok := PwCScore(c.in)
		if v != c.v || unit != c.unit || ok != c.ok {
			t.Errorf("PwCScore(%q) = %v %q %v, want %v %q %v", c.in, v, unit, ok, c.v, c.unit, c.ok)
		}
	}
}

func TestPwCResultRows(t *testing.T) {
	b, err := os.ReadFile("testdata/pwc-eval.json")
	if err != nil {
		t.Fatal(err)
	}
	rows, skipped, err := PwCResultRows(b, "b3:snap", Options{})
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]Row{}
	for _, r := range rows {
		by[r.ID] = r
	}
	if len(rows) != 5 {
		t.Fatalf("%d rows, want 5: %v", len(rows), by)
	}
	if skipped["values of rows that name no paper"] != 1 || skipped["values that are not numbers"] != 1 {
		t.Errorf("skipped %v", skipped)
	}
	x := by["squad1-1-dev/1911.02855/xlnet-plus-dsc/em"]
	if len(x.Labels) != 2 || x.Labels[0] != "Question Answering" || x.Labels[1] != "Reading Comprehension" {
		t.Errorf("the row of two tasks has the labels %v", x.Labels)
	}
	src := x.Fields["source"].(map[string]any)
	if src["src"] != "arxiv:1911.02855" || src["kind"] != "primary" || src["date"] != "2019-11-07" {
		t.Errorf("source %v", src)
	}
	if x.Fields["benchmark_name"] != "SQuAD1.1 dev" || x.Fields["metric_name"] != "EM" || x.Fields["model"] != "XLNet+DSC" {
		t.Errorf("fields %v", x.Fields)
	}
	if f := x.Fields["flags"].([]string); len(f) != 1 || f[0] != PwCUnverified {
		t.Errorf("flags %v", f)
	}
	// Two numbers for one model, paper and metric are two rows.
	b1, b2 := by["squad1-1-dev/1810.04805/bert-large/em"], by["squad1-1-dev/1810.04805/bert-large/em~2"]
	if b1.Fields["score"].(map[string]any)["unit"] != "percent" || b2.Fields["score"].(map[string]any)["v"] != 85.0 {
		t.Errorf("bert rows %v %v", b1.Fields["score"], b2.Fields["score"])
	}
	if b1.Fields["setting"].(map[string]any)["notes"] != "Papers with Code: uses extra training data" {
		t.Errorf("setting %v", b1.Fields["setting"])
	}
	// A paper that is not on arXiv is cited by its URL and named in the id by its title.
	v := by["squad1-1-dev/a-venue-paper/venue/f1"]
	if v.Fields["source"].(map[string]any)["src"] != "https://aclanthology.org/2020.acl-main.1/" {
		t.Errorf("venue row %v", v)
	}
	since, _, _ := PwCResultRows(b, "b3:snap", Options{Since: "2019-01-01"})
	if len(since) != 3 {
		t.Errorf("-since 2019: %d rows, want 3 (XLNet EM and F1, Venue F1)", len(since))
	}
	if _, _, err := PwCResultRows([]byte(`[]`), "b3:x", Options{}); err == nil {
		t.Error("no error for tables with no task")
	}
}
