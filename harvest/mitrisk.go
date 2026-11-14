// SPDX-License-Identifier: Apache-2.0

package harvest

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// The MIT AI Risk Repository is one Google Sheet (CC BY 4.0) with a sheet for each table. A new version of the
// repository is a new spreadsheet, and the old one says it is outdated, so the id is pinned here and the adapter stops
// when the sheet says so. The export of a sheet as CSV is allowed by the robots.txt of docs.google.com (Allow:
// /spreadsheet).
const (
	MITRiskSheet     = "15LeHcpeuZC9txkvcaMoh3sUhkMvdMMry69xxXL46DT0" // AI Risk Database v4, updated 3 December 2025
	MITRiskDomainGID = "1760303638"                                   // Domain Taxonomy of AI Risks
	MITRiskDBGID     = "1710822211"                                   // AI Risk Database v4
	MITRiskPapersGID = "2013041365"                                   // Included resources
)

// MITRiskURL is the CSV export of one sheet of the repository.
func MITRiskURL(gid string) string {
	return "https://docs.google.com/spreadsheets/d/" + MITRiskSheet + "/export?format=csv&gid=" + gid
}

const mitRiskPage = "https://airisk.mit.edu/"

var (
	mitRiskLead    = regexp.MustCompile(`^(?i:type|risk|harm|category|area)\s+\d+[a-z]?\s*[:.\-–]\s*`)
	mitRiskCode    = regexp.MustCompile(`^(\d+)(?:\.(\d+))?\s*>?\s*`)
	mitRiskUpdated = regexp.MustCompile(`Updated:\s*(\d{1,2} \w+ \d{4})`)
	digitsRE       = regexp.MustCompile(`^\d+$`)
)

// mitRiskTable is a sheet read as CSV: the rows after its header, and the column of each header name.
type mitRiskTable struct {
	col     map[string]int
	rows    [][]string
	updated string
}

func (t *mitRiskTable) get(r []string, name string) string {
	i, ok := t.col[name]
	if !ok || i >= len(r) {
		return ""
	}
	return clean(r[i])
}

// at gives the cell of a row, or "" past its end.
func at(r []string, i int) string {
	if i < 0 || i >= len(r) {
		return ""
	}
	return clean(r[i])
}

// readMITRiskSheet finds the header row, the first row that has every one of the names, and the date of the sheet.
func readMITRiskSheet(b []byte, what string, names ...string) (*mitRiskTable, error) {
	cr := csv.NewReader(bytes.NewReader(b))
	cr.FieldsPerRecord = -1
	all, err := cr.ReadAll()
	if err != nil {
		return nil, &ErrSchema{"mit-ai-risk", what + " is not CSV: " + err.Error()}
	}
	t := &mitRiskTable{}
	for i, r := range all {
		line := strings.Join(r, " ")
		if t.updated == "" {
			if m := mitRiskUpdated.FindStringSubmatch(line); m != nil {
				t.updated = m[1]
			}
		}
		if t.col == nil && strings.Contains(strings.ToLower(line), "now outdated") {
			return nil, &ErrSchema{"mit-ai-risk", what + " says that this version of the repository is outdated: MITRiskSheet needs the id of the new one"}
		}
		col := map[string]int{}
		for j, c := range r {
			if _, ok := col[clean(c)]; !ok {
				col[clean(c)] = j
			}
		}
		ok := true
		for _, n := range names {
			if _, has := col[n]; !has {
				ok = false
				break
			}
		}
		if ok {
			t.col, t.rows = col, all[i+1:]
			return t, nil
		}
	}
	return nil, &ErrSchema{"mit-ai-risk", what + " has no header row with " + strings.Join(names, ", ")}
}

// mitRiskName cuts the numbering of a risk name ("Type 1: Diffusion of responsibility") and its end punctuation.
func mitRiskName(s string) string {
	return strings.TrimRight(clean(mitRiskLead.ReplaceAllString(clean(s), "")), ";:,. ")
}

// mitRiskDomain gives the domain id of a code of the taxonomy: "7.3 > Lack of capability" is domain:7.3, "4.0 > Malicious
// use" is domain:4, "X.1 > Excluded" is none.
func mitRiskDomain(s string) string {
	m := mitRiskCode.FindStringSubmatch(clean(s))
	if m == nil {
		return ""
	}
	if m[2] == "" || m[2] == "0" {
		return "domain:" + m[1]
	}
	return "domain:" + m[1] + "." + m[2]
}

// MITRiskRows gives the rows of the repository from its three sheets: a risk row for each domain and sub-domain of
// the Domain Taxonomy, a work row for each included document, and a risk row for each risk category and sub-category
// that a document names, linked to the document and to its domain. Descriptions and quotes are not kept: names, ids,
// the causal codes (entity, intent, timing) and the links are.
func MITRiskRows(domains, db, papers []byte, snaps [3]string) (rows []Row, notes []string, skipped int, err error) {
	dt, err := readMITRiskSheet(domains, "the Domain Taxonomy sheet", "Domain", "Sub-domain")
	if err != nil {
		return nil, nil, 0, err
	}
	var domain string
	for _, r := range dt.rows {
		if c := at(r, dt.col["Domain"]-1); c != "" && dt.get(r, "Domain") != "" {
			if !digitsRE.MatchString(c) {
				break // the notes under the table
			}
			domain = c
			row := Row{ID: "domain:" + c, Name: dt.get(r, "Domain"), Kind: "risk", URL: mitRiskPage, Snapshot: snaps[0],
				Labels: []string{"MIT AI Risk domain"}, Fields: map[string]any{"level": "domain", "code": c}}
			rows = append(rows, row)
		}
		code := at(r, dt.col["Sub-domain"]-1)
		if domain == "" || !strings.HasPrefix(code, domain+".") || dt.get(r, "Sub-domain") == "" {
			continue
		}
		rows = append(rows, Row{ID: "domain:" + code, Name: dt.get(r, "Sub-domain"), Kind: "risk", URL: mitRiskPage,
			Snapshot: snaps[0], Labels: []string{"MIT AI Risk sub-domain"}, Fields: map[string]any{"level": "sub-domain", "code": code},
			Links: map[string][]string{"domain": {"domain:" + domain}}})
	}
	if len(rows) < 7 {
		return nil, nil, 0, &ErrSchema{"mit-ai-risk", fmt.Sprintf("the Domain Taxonomy sheet has %d domains and sub-domains", len(rows))}
	}
	pt, err := readMITRiskSheet(papers, "the Included resources sheet", "Paper_ID", "QuickRef", "Title", "Year", "DOI")
	if err != nil {
		return nil, nil, 0, err
	}
	seen := map[string]bool{}
	for _, r := range pt.rows {
		id, title := pt.get(r, "Paper_ID"), pt.get(r, "Title")
		if id == "" {
			continue // a blank row, or the notes under the table
		}
		if !digitsRE.MatchString(id) || title == "" {
			skipped++
			continue
		}
		w := Row{ID: "paper:" + id, Name: title, Kind: "work", URL: pt.get(r, "URL"), Snapshot: snaps[2],
			Labels: []string{"MIT AI Risk document"}, SameAs: map[string]string{},
			Fields: map[string]any{"quickref": pt.get(r, "QuickRef"), "paper_id": id}}
		if !strings.HasPrefix(w.URL, "http") {
			w.URL = mitRiskPage
		}
		if d := strings.ToLower(pt.get(r, "DOI")); doiRE.MatchString(d) {
			w.SameAs["doi"] = d
			if a, ok := strings.CutPrefix(d, "10.48550/arxiv."); ok {
				w.SameAs["arxiv"] = a
			}
		}
		if len(w.SameAs) == 0 {
			w.SameAs = nil
		}
		setFact(&w, "year", pt.get(r, "Year"), "Included resources, Paper_ID "+id+", Year")
		setFact(&w, "authors", pt.get(r, "Authors (full)"), "Included resources, Paper_ID "+id+", Authors (full)")
		setFact(&w, "item_type", pt.get(r, "Item type"), "Included resources, Paper_ID "+id+", Item type")
		if pt.get(r, "Included") != "" {
			w.Fields["included"] = pt.get(r, "Included")
		}
		seen[id] = true
		rows = append(rows, w)
	}
	notes = append(notes, mitRiskSharedDOIs(rows)...)
	t, err := readMITRiskSheet(db, "the AI Risk Database sheet", "QuickRef", "Ev_ID", "Paper_ID", "Category level",
		"Risk category", "Risk subcategory", "Entity", "Intent", "Timing", "Domain", "Sub-domain")
	if err != nil {
		return nil, nil, 0, err
	}
	if t.updated != "" {
		notes = append([]string{"AI Risk Database updated " + t.updated}, notes...)
	}
	var evidence int
	for _, r := range t.rows {
		level := t.get(r, "Category level")
		switch level {
		case "Risk Category", "Risk Sub-Category":
		case "Additional evidence":
			evidence++
			continue
		default:
			continue // a document's own row, or the notes under the table
		}
		ev, paper := t.get(r, "Ev_ID"), strings.TrimSuffix(t.get(r, "Paper_ID"), ".0")
		name := t.get(r, "Risk category")
		if level == "Risk Sub-Category" && t.get(r, "Risk subcategory") != "" {
			name = t.get(r, "Risk subcategory")
		}
		n := mitRiskName(name)
		if ev == "" || n == "" {
			skipped++
			continue
		}
		row := Row{ID: "risk:" + ev, Name: n, Kind: "risk", URL: mitRiskPage, Snapshot: snaps[1],
			Labels: []string{"MIT AI Risk Database"}, Links: map[string][]string{},
			Fields: map[string]any{"level": strings.ToLower(level), "quickref": t.get(r, "QuickRef")}}
		if n != clean(name) {
			row.Aliases = []string{clean(name)}
		}
		if seen[paper] {
			row.Links["named_in"] = []string{"paper:" + paper}
		}
		if parts := strings.Split(ev, "."); level == "Risk Sub-Category" && len(parts) == 3 {
			row.Links["category"] = []string{"risk:" + parts[0] + "." + parts[1] + ".00"}
		}
		sub := t.get(r, "Sub-domain")
		if d := mitRiskDomain(sub); d != "" {
			row.Links["domain"] = []string{d}
		} else if strings.HasPrefix(sub, "X.") {
			row.Fields["excluded"] = true
		}
		for _, k := range []string{"Entity", "Intent", "Timing"} {
			setFact(&row, strings.ToLower(k), t.get(r, k), "AI Risk Database, Ev_ID "+ev+", "+k)
		}
		if len(row.Links) == 0 {
			row.Links = nil
		}
		rows = append(rows, row)
	}
	if evidence > 0 {
		notes = append(notes, fmt.Sprintf("%d rows of additional evidence are quotes, not risks, and are not read", evidence))
	}
	return rows, notes, skipped, nil
}

func harvestMITRisk(ctx context.Context, r *Run, o Options) ([]Row, error) {
	var bs [3][]byte
	var snaps [3]string
	for i, gid := range []string{MITRiskDomainGID, MITRiskDBGID, MITRiskPapersGID} {
		b, snap, err := r.Get(ctx, MITRiskURL(gid), 32<<20)
		if err != nil {
			return nil, fmt.Errorf("mit-ai-risk: %w", err)
		}
		bs[i], snaps[i] = b, snap
	}
	rows, notes, skipped, err := MITRiskRows(bs[0], bs[1], bs[2], snaps)
	if err != nil {
		return nil, err
	}
	r.Skip(skipped, "rows with no id or no name")
	r.Notes = append(r.Notes, notes...)
	return keep(rows, o.Limit), nil
}

// mitRiskSharedDOIs drops a DOI that two documents of the sheet share (in v4 the documents 33 and 38 have one DOI), from
// all of them, since the sheet cannot say whose it is, and a shared same_as id would join two works into one item.
func mitRiskSharedDOIs(rows []Row) (notes []string) {
	by := map[string][]int{}
	for i, r := range rows {
		if d := r.SameAs["doi"]; r.Kind == "work" && d != "" {
			by[d] = append(by[d], i)
		}
	}
	dois := make([]string, 0, len(by))
	for d, is := range by {
		if len(is) > 1 {
			dois = append(dois, d)
		}
	}
	sort.Strings(dois)
	for _, d := range dois {
		var ids []string
		for _, i := range by[d] {
			ids = append(ids, rows[i].ID)
			delete(rows[i].SameAs, "doi")
			delete(rows[i].SameAs, "arxiv")
			if len(rows[i].SameAs) == 0 {
				rows[i].SameAs = nil
			}
		}
		notes = append(notes, "the documents "+strings.Join(ids, ", ")+" share the DOI "+d+", which is dropped from them")
	}
	return notes
}
