// SPDX-License-Identifier: Apache-2.0

package harvest

import (
	"encoding/json"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestOECD(t *testing.T) {
	b, err := os.ReadFile("testdata/oecd-page.json")
	if err != nil {
		t.Fatal(err)
	}
	items, last, err := OECDPage(b)
	if err != nil || last != 27 || len(items) != 3 {
		t.Fatalf("%d initiatives, last page %d, %v", len(items), last, err)
	}
	var rows []Row
	for _, it := range items {
		rows = append(rows, OECDRow(it, "s"))
	}
	use, law, decl := rows[0], rows[1], rows[2]
	if use.ID != "2646" || use.Kind != "policy" || use.SameAs["oecd_ai"] != "2646" || use.Signals != nil ||
		use.Fields["jurisdiction"] != "SRB" || use.URL != "https://oecd.ai/en/dashboards/policy-initiatives/ai-egovernment-chatbots" ||
		!reflect.DeepEqual(use.Aliases, []string{"Digitalni asistenti u javnoj upravi"}) {
		t.Errorf("a use case in the public sector is a policy row but not an instrument: %+v", use)
	}
	if law.Signals["instrument"] != 1 || law.Fields["binding"] != "Binding" || law.Fields["jurisdiction"] != "CAN" ||
		law.Facts["started"].V != "2024" || !reflect.DeepEqual(law.Links["country"], []string{"CAN"}) {
		t.Errorf("a law is a binding instrument of Canada: %+v", law)
	}
	if _, ok := law.Facts["ended"]; ok {
		t.Errorf("a law with no endYear has no end: %v", law.Facts)
	}
	if decl.Signals["instrument"] != 1 || decl.Fields["jurisdiction"] != "UNESCO" ||
		!reflect.DeepEqual(decl.Links["organisation"], []string{"UNESCO"}) {
		t.Errorf("a declaration of UNESCO is an intergovernmental instrument: %+v", decl)
	}
	j, _ := json.Marshal(rows)
	for _, never := range []string{"editor@example.org", "An Editor", "Not kept."} {
		if strings.Contains(string(j), never) {
			t.Errorf("a row keeps %q", never)
		}
	}
	if _, _, err := OECDPage([]byte(`{"message":"Too Many Requests"}`)); err == nil {
		t.Error("an error answer is not an error")
	}
	if _, _, err := OECDPage([]byte(`{"data":[{"id":1}],"lastPage":1}`)); err == nil {
		t.Error("an initiative without a name is not an error")
	}
	u, _ := url.Parse(OECDURL(3))
	if u.Query().Get("page") != "3" || u.Query().Get("perPage") != "100" {
		t.Errorf("the URL %s", u)
	}
}
