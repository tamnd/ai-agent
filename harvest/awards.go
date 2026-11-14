// SPDX-License-Identifier: Apache-2.0

package harvest

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Award is an award of the field whose recipients are notable (S-P1, 19.6): its Wikidata QID, its name, and a note
// when it is not given for AI work alone, so that a node checks that the recipient's work is AI.
type Award struct {
	QID  string
	Name string
	Note string
}

// Awards are the awards that the adapter reads. The Turing Award covers all of computing; the others are AI awards.
// Each QID was checked on 2026-11-15 to have its recipients on P166 (award received).
var Awards = []Award{
	{"Q185667", "Turing Award", "all of computing: a node needs AI work"},
	{"Q5971529", "IJCAI Computers and Thought Award", ""},
	{"Q5971528", "IJCAI Award for Research Excellence", ""},
	{"Q30514449", "ACM-AAAI Allen Newell Award", ""},
	{"Q5970364", "IEEE Frank Rosenblatt Award", ""},
	{"Q2175611", "Rumelhart Prize", ""},
	{"Q18748042", "AAAI Fellow", ""},
	{"Q94282044", "EurAI Fellow", ""},
	{"Q6772765", "Marr Prize", ""},
	{"Q4832915", "Azriel Rosenfeld Award", ""},
}

// AwardsQuery is the SPARQL query of the recipients of the awards: one row per recipient and award, with the years of
// the award (P585 on the statement) and the class of the recipient.
func AwardsQuery(awards []Award, limit int) string {
	var vals []string
	for _, a := range awards {
		vals = append(vals, "wd:"+a.QID)
	}
	q := `PREFIX wd: <http://www.wikidata.org/entity/>
PREFIX wdt: <http://www.wikidata.org/prop/direct/>
PREFIX p: <http://www.wikidata.org/prop/>
PREFIX ps: <http://www.wikidata.org/prop/statement/>
PREFIX pq: <http://www.wikidata.org/prop/qualifier/>
PREFIX rdfs: <http://www.w3.org/2000/01/rdf-schema#>
PREFIX wikibase: <http://wikiba.se/ontology#>
SELECT ?item (SAMPLE(?l) AS ?label) ?award (GROUP_CONCAT(DISTINCT ?t; SEPARATOR="|") AS ?times)
  (SAMPLE(?h) AS ?is) (SAMPLE(?n) AS ?sitelinks)
WHERE {
  VALUES ?award { ` + strings.Join(vals, " ") + ` }
  ?item p:P166 ?st . ?st ps:P166 ?award .
  OPTIONAL { ?st pq:P585 ?t0 . BIND(STR(?t0) AS ?t) }
  OPTIONAL { ?item rdfs:label ?l . FILTER(LANG(?l) = "en") }
  OPTIONAL { ?item wdt:P31 ?h0 . FILTER(?h0 IN (wd:Q5, wd:Q13442814)) BIND(STR(?h0) AS ?h) }
  OPTIONAL { ?item wikibase:sitelinks ?n }
}
GROUP BY ?item ?award
ORDER BY DESC(?sitelinks) ?item ?award`
	if limit > 0 {
		q += "\nLIMIT " + strconv.Itoa(limit)
	}
	return q
}

// AwardRows reads the SPARQL JSON of the recipients into one row per recipient: the awards as labels and links, the
// years of each award as the fact award.<QID>, and the number of awards as the signal awards. A recipient with no
// English label, or that is neither a person nor a work, is skipped.
func AwardRows(b []byte, snap string, awards []Award) (rows []Row, skipped int, err error) {
	var r sparqlResult
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, 0, &ErrSchema{"awards", "the SPARQL result is not JSON: " + err.Error()}
	}
	names := map[string]string{}
	for _, a := range awards {
		names[a.QID] = a.Name
	}
	byID := map[string]int{}
	seen := map[string]bool{}
	for _, b := range r.Results.Bindings {
		id, award := qid(b["item"].Value), qid(b["award"].Value)
		if !strings.HasPrefix(id, "Q") || names[award] == "" {
			return nil, 0, &ErrSchema{"awards", "a binding has no item or no award of the list"}
		}
		name := clean(b["label"].Value)
		kind := map[string]string{"Q5": "person", "Q13442814": "work"}[qid(b["is"].Value)]
		if name == "" || name == id || kind == "" {
			if !seen[id] {
				seen[id] = true
				skipped++
			}
			continue
		}
		i, ok := byID[id]
		if !ok {
			i = len(rows)
			byID[id] = i
			row := Row{ID: id, Name: name, Kind: kind, URL: "https://www.wikidata.org/wiki/" + id,
				SameAs: map[string]string{"wikidata": id}, Snapshot: snap, Links: map[string][]string{}}
			if n, err := strconv.Atoi(b["sitelinks"].Value); err == nil {
				setSignal(&row, "sitelinks", n)
			}
			rows = append(rows, row)
		}
		row := &rows[i]
		row.Labels = append(row.Labels, award+" "+names[award])
		row.Links["award"] = append(row.Links["award"], award)
		var years []string
		for t := range strings.SplitSeq(b["times"].Value, "|") {
			if len(t) >= 4 && t[:4] >= "1900" && t[:4] <= "2999" {
				years = append(years, t[:4])
			}
		}
		sort.Strings(years)
		if len(years) > 0 {
			setFact(row, "award."+award, years, "P166 "+award+" P585")
		}
	}
	for i := range rows {
		sort.Strings(rows[i].Labels)
		sort.Strings(rows[i].Links["award"])
		setSignal(&rows[i], "awards", len(rows[i].Links["award"]))
	}
	return rows, skipped, nil
}

// harvestAwards reads the recipients of Awards in one query. A limited run asks for that many recipient-award pairs.
func harvestAwards(ctx context.Context, r *Run, o Options) ([]Row, error) {
	b, snap, err := r.Get(ctx, WikidataURL(AwardsQuery(Awards, o.Limit)), 64<<20)
	if err != nil {
		return nil, fmt.Errorf("awards: %w", err)
	}
	rows, skipped, err := AwardRows(b, snap, Awards)
	if err != nil {
		return nil, err
	}
	if skipped > 0 {
		r.Skipped += skipped
		r.Notes = append(r.Notes, fmt.Sprintf("%d recipients with no English label or not a person or a work", skipped))
	}
	sortBy(rows, "awards")
	return keep(rows, o.Limit), nil
}
