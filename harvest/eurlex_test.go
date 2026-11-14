// SPDX-License-Identifier: Apache-2.0

package harvest

import (
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestEurLex(t *testing.T) {
	b, err := os.ReadFile("testdata/eurlex-answer.json")
	if err != nil {
		t.Fatal(err)
	}
	works, err := EurLexWorks(b)
	if err != nil || len(works) != 5 {
		t.Fatalf("%d works, %v", len(works), err)
	}
	act := works[0]
	if act.CELEX != "32024R1689" || act.Type != "REG" || act.Date != "2024-06-13" || act.End != "" ||
		!reflect.DeepEqual(act.Force, []string{"2024-08-01", "2025-02-02", "2027-08-02"}) ||
		!reflect.DeepEqual(act.Amends, []string{"32008R0300", "32018R1139"}) {
		t.Errorf("the six lines of the act are not joined: %+v", act)
	}
	var rows []Row
	var skips []string
	for _, w := range works {
		if why := EurLexSkip(w); why != "" {
			skips = append(skips, w.CELEX)
			continue
		}
		rows = append(rows, EurLexRow(w, "s"))
	}
	if !reflect.DeepEqual(skips, []string{"32024R1689R(01)", "62024CN0001"}) {
		t.Errorf("skipped %v, want the corrigendum and the case", skips)
	}
	if len(rows) != 3 {
		t.Fatalf("%d rows", len(rows))
	}
	r := rows[0]
	if r.Kind != "policy" || r.Fields["jurisdiction"] != "EU" || r.Fields["type"] != "REG" || r.SameAs["celex"] != "32024R1689" ||
		r.SameAs["eli"] != "http://data.europa.eu/eli/reg/2024/1689/oj" || r.Signals["instrument"] != 1 ||
		r.Facts["adopted"].V != "2024-06-13" || r.Facts["in_force"].V != "2024-08-01" ||
		!reflect.DeepEqual(r.Facts["applies"].V, []string{"2025-02-02", "2027-08-02"}) ||
		!reflect.DeepEqual(r.Links["amends"], []string{"32008R0300", "32018R1139"}) ||
		!strings.HasSuffix(r.URL, "CELEX:32024R1689") {
		t.Errorf("the act %+v", r)
	}
	if _, ok := r.Facts["end_of_validity"]; ok {
		t.Error("an open end of validity (9999-12-31) is not a fact")
	}
	prop := rows[1]
	if prop.Signals["instrument"] != 1 || prop.Facts["proposed"].V != "2021-04-21" || prop.Facts["adopted"].V != nil {
		t.Errorf("a proposal is an instrument with the date proposed: %+v", prop)
	}
	if paper := rows[2]; paper.Signals != nil || paper.Facts["adopted"].V != "2020-02-19" {
		t.Errorf("a white paper is not an instrument: %+v", paper)
	}

	if _, err := EurLexWorks([]byte(`<html>challenge</html>`)); err == nil {
		t.Error("an answer that is not JSON is not an error")
	}
	if _, err := EurLexWorks([]byte(`{"head":{"vars":["x"]},"results":{"bindings":[]}}`)); err == nil {
		t.Error("an answer with no celex variable is not an error")
	}
	w, err := EurLexWorks([]byte(`{"head":{"vars":["celex","title"]},"results":{"bindings":[{"celex":{"value":"32020H0001"},"title":{"value":"A title  of it"}}]}}`))
	if err != nil || w[0].Title != "A title of it" {
		t.Errorf("the no-break spaces of a title: %+v %v", w, err)
	}

	q := EurLexQuery("2024-01-01")
	if !strings.Contains(q, `FILTER (?date >= "2024-01-01"^^`) || !strings.Contains(q, "<"+EurLexConcept+">") {
		t.Errorf("the query %s", q)
	}
	if strings.Contains(EurLexQuery(""), "FILTER") || strings.Contains(EurLexQuery("not a date"), "FILTER") {
		t.Error("a query with no date filters")
	}
	u, err := url.Parse(EurLexURL(q))
	if err != nil || u.Query().Get("query") != q || u.Query().Get("format") != "application/sparql-results+json" {
		t.Errorf("the URL %s %v", u, err)
	}
}
