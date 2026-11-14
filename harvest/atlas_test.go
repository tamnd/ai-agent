// SPDX-License-Identifier: Apache-2.0

package harvest

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestATLASRows(t *testing.T) {
	rows, version, skipped, err := ATLASRows(fixture(t, "atlas.yaml"), "b3:x")
	if err != nil {
		t.Fatal(err)
	}
	if version != "5.6.0" || skipped != 1 || len(rows) != 8 {
		t.Fatalf("got version %q, %d rows and %d skipped, want 5.6.0, 8 and 1: %+v", version, len(rows), skipped, rows)
	}
	by := map[string]Row{}
	for _, r := range rows {
		by[r.ID] = r
		if r.SameAs["atlas"] != r.ID || r.Snapshot != "b3:x" || !strings.HasPrefix(r.URL, "https://atlas.mitre.org/") {
			t.Errorf("row %s: %+v", r.ID, r)
		}
		for k := range r.Fields {
			if k == "description" || k == "actor" || k == "summary" {
				t.Errorf("row %s keeps %s", r.ID, k)
			}
		}
	}
	if r := by["AML.TA0002"]; r.Name != "Reconnaissance" || r.Kind != "concept" || r.Fields["attack"] != "TA0043" || len(r.SameAs) != 1 ||
		r.URL != "https://atlas.mitre.org/tactics/AML.TA0002" || r.Facts["created"].V != "2022-01-24" {
		t.Errorf("the tactic is %+v", r)
	}
	if r := by["AML.T0015"]; r.Kind != "risk" || r.Facts["maturity"].V != "realized" ||
		!reflect.DeepEqual(r.Links["tactic"], []string{"AML.TA0004", "AML.TA0007", "AML.TA0011"}) {
		t.Errorf("the technique is %+v", r)
	}
	if r := by["AML.T0000.000"]; !reflect.DeepEqual(r.Links, map[string][]string{"specializes": {"AML.T0000"}}) || r.Fields["attack"] != nil {
		t.Errorf("the sub-technique is %+v", r)
	}
	if r := by["AML.M0000"]; r.Kind != "method" || !reflect.DeepEqual(r.Links["mitigates"], []string{"AML.T0000", "AML.T0003"}) ||
		!reflect.DeepEqual(r.Facts["category"].V, []string{"Policy"}) || r.URL != "https://atlas.mitre.org/mitigations/AML.M0000" {
		t.Errorf("the mitigation is %+v", r)
	}
	ex, inc := by["AML.CS0000"], by["AML.CS0002"]
	if ex.Kind != "finding" || ex.Fields["case_study_type"] != "exercise" || ex.Facts["date"].V != "2020-01-01" ||
		ex.Facts["date_granularity"].V != "YEAR" || len(ex.Links["technique"]) == 0 || ex.URL != "https://atlas.mitre.org/studies/AML.CS0000" {
		t.Errorf("the exercise is %+v", ex)
	}
	if inc.Kind != "incident" || inc.Fields["case_study_type"] != "incident" {
		t.Errorf("the incident is %+v", inc)
	}
	for _, r := range rows {
		for _, f := range r.Facts {
			if s, ok := f.V.(string); ok && strings.Contains(s, "Trimmed actor") {
				t.Errorf("row %s keeps the actor", r.ID)
			}
		}
	}
	var e *ErrSchema
	for _, bad := range []string{"id: ATLAS\n", "id: OTHER\nmatrices: [{id: x}]\n", "[1, 2]"} {
		if _, _, _, err := ATLASRows([]byte(bad), ""); !errors.As(err, &e) {
			t.Errorf("%q: %v", bad, err)
		}
	}
}
