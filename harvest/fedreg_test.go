// SPDX-License-Identifier: Apache-2.0

package harvest

import (
	"net/url"
	"os"
	"reflect"
	"testing"
)

func TestFedReg(t *testing.T) {
	b, err := os.ReadFile("testdata/fedreg-page.json")
	if err != nil {
		t.Fatal(err)
	}
	docs, pages, count, err := FedRegPage(b)
	if err != nil || pages != 1 || count != 3 || len(docs) != 3 {
		t.Fatalf("%d docs, %d pages, count %d, %v", len(docs), pages, count, err)
	}
	var rows []Row
	for _, d := range docs {
		if AITitle(d.Title) {
			rows = append(rows, FedRegRow(d, "s"))
		}
	}
	if len(rows) != 2 {
		t.Fatalf("%d rows with an AI title, want 2 (not the notice of an organization)", len(rows))
	}
	eo := rows[0]
	if eo.ID != "2025-14218" || eo.Kind != "policy" || eo.SameAs["executive_order"] != "14320" ||
		!reflect.DeepEqual(eo.Aliases, []string{"Executive Order 14320"}) || eo.Fields["subtype"] != "Executive Order" ||
		eo.Fields["citation"] != "90 FR 35393" || eo.Facts["signed"].V != "2025-07-23" || eo.Facts["published"].V != "2025-07-28" {
		t.Errorf("the executive order %+v", eo)
	}
	if eo.Signals["instrument"] != 1 || rows[1].Signals["instrument"] != 1 {
		t.Errorf("an executive order and a rule are instruments: %v %v", eo.Signals, rows[1].Signals)
	}
	if n := FedRegRow(docs[1], "s"); n.Signals != nil {
		t.Errorf("a notice is not an instrument: %v", n.Signals)
	}
	if !reflect.DeepEqual(eo.Links["agency"], []string{"executive-office-of-the-president"}) {
		t.Errorf("agencies %v", eo.Links)
	}
	if _, ok := rows[1].Facts["effective"]; !ok || rows[1].SameAs["executive_order"] != "" {
		t.Errorf("the rule %+v", rows[1])
	}
	if _, _, _, err := FedRegPage([]byte(`{"status":400,"message":"field 'x' not valid"}`)); err == nil {
		t.Error("an error answer is not an error")
	}
	for title, want := range map[string]bool{
		"Removing Barriers to American Leadership in Artificial Intelligence": true,
		"Promoting the Export of the American AI Technology Stack":            true,
		"Safety Standards for Machine Learning Medical Devices":               true,
		"Statement of Organization, Functions and Delegations of Authority":   false,
		"Airworthiness Directives; Airbus SAS Airplanes (AIRBUS)":             false,
	} {
		if AITitle(title) != want {
			t.Errorf("AITitle(%q) = %v", title, !want)
		}
	}
	u, err := url.Parse(FedRegURL("2024-01-01", 2))
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if q.Get("conditions[publication_date][gte]") != "2024-01-01" || q.Get("page") != "2" || q.Get("conditions[term]") != FedRegTerm || len(q["fields[]"]) != len(fedRegFields) {
		t.Errorf("the URL %s", u)
	}
}
