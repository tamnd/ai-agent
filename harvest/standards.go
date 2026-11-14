// SPDX-License-Identifier: Apache-2.0

package harvest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// NISTWorks is the Crossref list of the works of NIST's DOI prefix. The technical series of NIST (the AI, SP, IR, TN
// and other series) are reports with DOIs 10.6028/NIST.<series>.<number>; Crossref's metadata is CC0, while the
// release files of NIST's own metadata are on hosts whose robots.txt disallows downloads.
const NISTWorks = "https://api.crossref.org/prefixes/10.6028/works"

// NISTURL is the URL of a page of NIST's reports: 1,000 a page by Crossref's cursor, from a date if since is set.
func NISTURL(since, cursor string) string {
	f := "type:report"
	if since != "" {
		f += ",from-pub-date:" + since
	}
	return NISTWorks + "?" + url.Values{"filter": {f}, "rows": {"1000"}, "cursor": {cursor},
		"select": {"DOI,title,subtitle,issued,type"}}.Encode()
}

type crossrefWork struct {
	DOI      string   `json:"DOI"`
	Title    []string `json:"title"`
	Subtitle []string `json:"subtitle"`
	Issued   struct {
		DateParts [][]int `json:"date-parts"`
	} `json:"issued"`
}

type crossrefPage struct {
	Message *struct {
		NextCursor string         `json:"next-cursor"`
		Items      []crossrefWork `json:"items"`
	} `json:"message"`
}

var (
	// nistDOI splits a DOI of NIST's series: the series, the number and the suffixes of a draft (ipd, iprd, 2pd,
	// fpd, -draft) or of a translation (a code of three letters).
	nistDOI    = regexp.MustCompile(`^10\.6028/(nist|nbs)\.([a-z]+)\.(.+)$`)
	nistDraft  = regexp.MustCompile(`\.(ipd|iprd|[0-9]pd|fpd|draft)$|-draft$`)
	nistLang   = regexp.MustCompile(`\.(ara|chi|fre|ger|jpn|kor|por|spa|ukr|vie)$`)
	nistSeries = map[string]string{"ai": "NIST AI", "sp": "NIST SP", "ir": "NIST IR", "tn": "NIST TN", "ams": "NIST AMS",
		"gcr": "NIST GCR", "cswp": "NIST CSWP", "fips": "NIST FIPS", "hb": "NIST HB"}
)

// crossrefDate gives the issued date of a work as YYYY, YYYY-MM or YYYY-MM-DD.
func crossrefDate(w crossrefWork) string {
	if len(w.Issued.DateParts) == 0 || len(w.Issued.DateParts[0]) == 0 || w.Issued.DateParts[0][0] < 1000 {
		return ""
	}
	p := w.Issued.DateParts[0]
	s := strconv.Itoa(p[0])
	for _, n := range p[1:min(len(p), 3)] {
		s += fmt.Sprintf("-%02d", n)
	}
	return s
}

// NISTPage reads a page of Crossref's list into the rows of the reports whose title is about AI (AITitle). A report
// of the NIST AI series is a standard (S-L2 decides which are cards); a report of another series is a work. A
// translation, or a DOI that is a placeholder, is skipped and counted.
func NISTPage(b []byte, snap string) (rows []Row, next string, n, skipped int, err error) {
	var p crossrefPage
	if err := json.Unmarshal(b, &p); err != nil {
		return nil, "", 0, 0, &ErrSchema{"standards", "the Crossref page is not JSON: " + err.Error()}
	}
	if p.Message == nil {
		return nil, "", 0, 0, &ErrSchema{"standards", "the Crossref page has no message"}
	}
	for _, w := range p.Message.Items {
		title := clean(strings.TrimRight(strings.Join(w.Title, " "), " :"))
		if sub := clean(strings.Join(w.Subtitle, " ")); sub != "" {
			title += ": " + sub
		}
		if !AITitle(title) {
			continue
		}
		doi := strings.ToLower(strings.TrimSpace(w.DOI))
		m := nistDOI.FindStringSubmatch(doi)
		if m == nil || strings.Contains(m[3], "xxx") || nistLang.MatchString(m[3]) {
			skipped++
			continue
		}
		id := strings.ToUpper(m[1]) + "." + strings.ToUpper(m[2]) + "." + m[3]
		row := Row{ID: id, Name: title, Kind: "work", URL: "https://doi.org/" + doi, Snapshot: snap,
			SameAs: map[string]string{"doi": doi}, Fields: map[string]any{"body": "NIST", "status": "final"}}
		if s := nistSeries[m[2]]; s != "" {
			row.Fields["series"] = s
		}
		if m[1] == "nbs" {
			row.Fields["series"] = "NBS " + strings.ToUpper(m[2])
		}
		if m[1] == "nist" && m[2] == "ai" {
			row.Kind = "standard"
		}
		if nistDraft.MatchString(m[3]) {
			row.Fields["status"] = "draft"
		}
		setFact(&row, "issued", crossrefDate(w), "issued")
		rows = append(rows, row)
	}
	return rows, p.Message.NextCursor, len(p.Message.Items), skipped, nil
}

// harvestStandards reads the publications of the standards bodies about AI. Its first part is NIST, from Crossref:
// every report of the prefix 10.6028, 1,000 a page, of which the ones with an AI title are kept. -since keeps the
// reports issued on or after the date.
func harvestStandards(ctx context.Context, r *Run, o Options) ([]Row, error) {
	var out []Row
	skipped := 0
	cursor := "*"
	for page := 1; ; page++ {
		b, snap, err := r.Get(ctx, NISTURL(o.Since, cursor), 64<<20)
		if err != nil {
			return nil, fmt.Errorf("standards: NIST page %d: %w", page, err)
		}
		rows, next, n, s, err := NISTPage(b, snap)
		if err != nil {
			return nil, err
		}
		out = append(out, rows...)
		skipped += s
		if n < 1000 || next == "" || o.Limit > 0 && len(out) >= o.Limit {
			break
		}
		cursor = next
	}
	r.Skip(skipped, "translations of NIST reports and placeholder DOIs")
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return keep(out, o.Limit), nil
}
