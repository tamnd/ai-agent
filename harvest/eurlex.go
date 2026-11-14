// SPDX-License-Identifier: Apache-2.0

package harvest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"sort"
	"strings"
)

// EurLexSPARQL is the SPARQL endpoint of the Cellar, the repository of the Publications Office of the EU that EUR-Lex
// reads. It needs no key. The site eur-lex.europa.eu answers scripts with a challenge, so the adapter never fetches it.
const EurLexSPARQL = "https://publications.europa.eu/webapi/rdf/sparql"

// EurLexConcept is the EuroVoc concept "artificial intelligence" (3030), which the Publications Office indexes the
// works about AI under.
const EurLexConcept = "http://eurovoc.europa.eu/3030"

// eurLexMax is the most rows that the endpoint gives for one query; a full answer means that rows were cut.
const eurLexMax = 10000

// EurLexQuery is the query of the works about AI that have a CELEX number, with their type, date, English title, ELI,
// dates of entry into force, end of validity and the acts they amend. A work has one line for each combination of its
// values, which EurLexWorks joins. since, when a date, keeps the works of that date or later.
func EurLexQuery(since string) string {
	filter := ""
	if d := date(since); d != "" {
		filter = fmt.Sprintf("\n  FILTER (?date >= %q^^<http://www.w3.org/2001/XMLSchema#date>)", d)
	}
	return `PREFIX cdm: <http://publications.europa.eu/ontology/cdm#>
SELECT DISTINCT ?celex ?type ?date ?title ?eli ?force ?end ?amends WHERE {
  ?w cdm:work_is_about_concept_eurovoc <` + EurLexConcept + `> ; cdm:resource_legal_id_celex ?celex .
  OPTIONAL { ?w cdm:work_has_resource-type ?type }
  OPTIONAL { ?w cdm:work_date_document ?date }
  OPTIONAL { ?w cdm:resource_legal_eli ?eli }
  OPTIONAL { ?w cdm:resource_legal_date_entry-into-force ?force }
  OPTIONAL { ?w cdm:resource_legal_date_end-of-validity ?end }
  OPTIONAL { ?w cdm:resource_legal_amends_resource_legal ?a . ?a cdm:resource_legal_id_celex ?amends }
  OPTIONAL { ?x cdm:expression_belongs_to_work ?w ;
    cdm:expression_uses_language <http://publications.europa.eu/resource/authority/language/ENG> ;
    cdm:expression_title ?title }` + filter + `
} ORDER BY ?celex LIMIT ` + fmt.Sprint(eurLexMax)
}

// EurLexURL is the GET URL of a query, with the answer as SPARQL JSON.
func EurLexURL(query string) string {
	return EurLexSPARQL + "?" + url.Values{"query": {query}, "format": {"application/sparql-results+json"}}.Encode()
}

// EurLexWork is a work of the Cellar, its lines joined.
type EurLexWork struct {
	CELEX  string
	Type   string   // the code of the resource type, such as REG, DEC_IMPL or PROP_REG
	Date   string   // the date of the document: the adoption of an act, the date of a proposal
	Title  string   // the English title
	ELI    string   // the European Legislation Identifier
	Force  []string // the dates of entry into force, sorted: the first is the entry into force, the others the dates that parts apply from
	End    string   // the end of validity, when it is not open (9999-12-31)
	Amends []string // the CELEX numbers of the acts that the work amends
}

// EurLexWorks reads the answer of EurLexQuery and joins the lines of each work.
func EurLexWorks(b []byte) ([]EurLexWork, error) {
	var p struct {
		Head *struct {
			Vars []string `json:"vars"`
		} `json:"head"`
		Results struct {
			Bindings []map[string]struct {
				Value string `json:"value"`
			} `json:"bindings"`
		} `json:"results"`
	}
	if err := json.Unmarshal(b, &p); err != nil {
		return nil, &ErrSchema{"eur-lex", "the answer is not SPARQL JSON: " + err.Error()}
	}
	if p.Head == nil || !slices.Contains(p.Head.Vars, "celex") {
		return nil, &ErrSchema{"eur-lex", "the answer has no variable celex"}
	}
	if len(p.Results.Bindings) >= eurLexMax {
		return nil, &ErrSchema{"eur-lex", fmt.Sprintf("the answer has %d lines, the most the endpoint gives, so works may be cut", eurLexMax)}
	}
	at := map[string]*EurLexWork{}
	var order []string
	for _, l := range p.Results.Bindings {
		c := strings.TrimSpace(l["celex"].Value)
		if c == "" {
			return nil, &ErrSchema{"eur-lex", "a line of the answer has no CELEX number"}
		}
		w := at[c]
		if w == nil {
			w = &EurLexWork{CELEX: c}
			at[c] = w
			order = append(order, c)
		}
		if t := l["type"].Value; t != "" && w.Type == "" {
			w.Type = t[strings.LastIndex(t, "/")+1:]
		}
		if d := date(l["date"].Value); d != "" && (w.Date == "" || d < w.Date) {
			w.Date = d
		}
		if t := clean(strings.ReplaceAll(l["title"].Value, "\u00a0", " ")); t != "" && (w.Title == "" || t < w.Title) {
			w.Title = t
		}
		if e := l["eli"].Value; e != "" && w.ELI == "" {
			w.ELI = e
		}
		if d := date(l["force"].Value); d != "" && !slices.Contains(w.Force, d) {
			w.Force = append(w.Force, d)
		}
		if d := date(l["end"].Value); d != "" && d != "9999-12-31" && (w.End == "" || d < w.End) {
			w.End = d
		}
		if a := strings.TrimSpace(l["amends"].Value); a != "" && !slices.Contains(w.Amends, a) {
			w.Amends = append(w.Amends, a)
		}
	}
	out := make([]EurLexWork, 0, len(order))
	for _, c := range order {
		w := at[c]
		sort.Strings(w.Force)
		sort.Strings(w.Amends)
		out = append(out, *w)
	}
	return out, nil
}

// eurLexInstruments are the resource types that are instruments of S-L1: the acts of the Union, binding or a
// recommendation, the international agreements it makes, and the proposals of acts.
var eurLexInstruments = map[string]bool{
	"REG": true, "REG_IMPL": true, "REG_DEL": true, "DIR": true, "DIR_IMPL": true, "DIR_DEL": true,
	"DEC": true, "DEC_IMPL": true, "DEC_DEL": true, "RECO": true, "CONVENTION": true,
	"PROP_REG": true, "PROP_DIR": true, "PROP_DEC": true,
}

// EurLexSkip tells why a work is not a row, or "" when it is one. The rows are the acts, the international
// agreements and the preparatory documents (sectors 1 to 5 of CELEX); a corrigendum corrects the text of a row and is
// not a policy of its own.
func EurLexSkip(w EurLexWork) string {
	switch {
	case w.Type == "CORRIGENDUM" || strings.Contains(w.CELEX, "R("):
		return "corrigenda, which correct the text of an act"
	case w.CELEX[0] < '1' || w.CELEX[0] > '5':
		return "works of the case law, the national measures, the parliamentary questions and the notices of the Official Journal (CELEX sectors 6 to 9, C and E)"
	case w.Type == "ANNOUNC":
		return "announcements of the Official Journal"
	case w.Title == "":
		return "works with no English title"
	}
	return ""
}

// EurLexRow is the row of a work: a policy of the EU with its type, dates and the acts it amends.
func EurLexRow(w EurLexWork, snap string) Row {
	row := Row{ID: w.CELEX, Name: w.Title, Kind: "policy",
		URL:      "https://eur-lex.europa.eu/legal-content/EN/TXT/?uri=CELEX:" + w.CELEX,
		SameAs:   map[string]string{"celex": w.CELEX},
		Fields:   map[string]any{"type": w.Type, "jurisdiction": "EU"},
		Snapshot: snap}
	if w.ELI != "" {
		row.SameAs["eli"] = w.ELI
	}
	if len(w.Amends) > 0 {
		row.Links = map[string][]string{"amends": w.Amends}
	}
	if eurLexInstruments[w.Type] {
		setSignal(&row, "instrument", 1)
	}
	if strings.HasPrefix(w.Type, "PROP_") {
		setFact(&row, "proposed", w.Date, "cdm:work_date_document")
	} else {
		setFact(&row, "adopted", w.Date, "cdm:work_date_document")
	}
	if len(w.Force) > 0 {
		setFact(&row, "in_force", w.Force[0], "cdm:resource_legal_date_entry-into-force")
		setFact(&row, "applies", w.Force[1:], "cdm:resource_legal_date_entry-into-force")
	}
	setFact(&row, "end_of_validity", w.End, "cdm:resource_legal_date_end-of-validity")
	return row
}

// harvestEurLex reads the works of the Cellar about AI and keeps the acts, agreements and preparatory documents.
// -since keeps the works of that date or later; -limit keeps at most that many rows.
func harvestEurLex(ctx context.Context, r *Run, o Options) ([]Row, error) {
	b, snap, err := r.Get(ctx, EurLexURL(EurLexQuery(o.Since)), 64<<20)
	if err != nil {
		return nil, fmt.Errorf("eur-lex: %w", err)
	}
	works, err := EurLexWorks(b)
	if err != nil {
		return nil, err
	}
	skipped := map[string]int{}
	var out []Row
	for _, w := range works {
		if why := EurLexSkip(w); why != "" {
			skipped[why]++
			continue
		}
		out = append(out, EurLexRow(w, snap))
	}
	whys := make([]string, 0, len(skipped))
	for why := range skipped {
		whys = append(whys, why)
	}
	sort.Strings(whys)
	for _, why := range whys {
		r.Skip(skipped[why], why)
	}
	out = keep(out, o.Limit)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}
