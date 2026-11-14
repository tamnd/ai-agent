// SPDX-License-Identifier: Apache-2.0

package harvest

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
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

type crossrefDateParts struct {
	DateParts [][]int `json:"date-parts"`
}

type crossrefWork struct {
	DOI           string            `json:"DOI"`
	Title         []string          `json:"title"`
	Subtitle      []string          `json:"subtitle"`
	Issued        crossrefDateParts `json:"issued"`
	Approved      crossrefDateParts `json:"approved"`
	StandardsBody *struct {
		Name    string `json:"name"`
		Acronym string `json:"acronym"`
	} `json:"standards-body"`
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

// crossrefDate gives a date of a work as YYYY, YYYY-MM or YYYY-MM-DD; Crossref writes a missing date as [[null]].
func crossrefDate(d crossrefDateParts) string {
	if len(d.DateParts) == 0 || len(d.DateParts[0]) == 0 || d.DateParts[0][0] < 1000 {
		return ""
	}
	p := d.DateParts[0]
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
		setFact(&row, "issued", crossrefDate(w.Issued), "issued")
		rows = append(rows, row)
	}
	return rows, p.Message.NextCursor, len(p.Message.Items), skipped, nil
}

// IEEEStandards is the Crossref list of the standards of IEEE's DOI prefix: the IEEE Standards Association deposits
// each standard, with its committee and its approval date, as a work of type standard (10.1109/ieeestd.<year>.<n>).
const IEEEStandards = "https://api.crossref.org/types/standard/works"

// IEEEURL is the URL of a page of IEEE's standards: 1,000 a page by Crossref's cursor, from a date if since is set.
func IEEEURL(since, cursor string) string {
	f := "prefix:10.1109"
	if since != "" {
		f += ",from-pub-date:" + since
	}
	return IEEEStandards + "?" + url.Values{"filter": {f}, "rows": {"1000"}, "cursor": {cursor},
		"select": {"DOI,title,subtitle,issued,approved,standards-body"}}.Encode()
}

var (
	ieeeDOI = regexp.MustCompile(`^10\.1109/(ieeestd\.\d{4}\.\d+)$`)
	// ieeeAI adds to AITitle the IEEE titles about autonomous and intelligent systems and ethical design, the 7000
	// series (7000 ethical concerns in system design, 7001 transparency of autonomous systems, 7007 ethically driven
	// robotics, 7009 fail-safe design, 7010 well-being), whose titles do not say AI.
	ieeeAI = regexp.MustCompile(`(?i)autonomous and intelligent systems|autonomous systems|ethical concerns during system design|ethically driven robotics`)
	// ieeeStatus reads the status that IEEE puts at the start of a title.
	ieeeStatus = regexp.MustCompile(`(?i)^\((replaced|withdrawn|superseded)\)\s*`)
)

// IEEEPage reads a page of Crossref's list into the rows of the IEEE standards about AI: a standard with its committee,
// its approval date and its status (final, draft, or the replaced or withdrawn that IEEE writes before the title). A
// DOI outside 10.1109/ieeestd is skipped and counted.
func IEEEPage(b []byte, snap string) (rows []Row, next string, n, skipped int, err error) {
	var p crossrefPage
	if err := json.Unmarshal(b, &p); err != nil {
		return nil, "", 0, 0, &ErrSchema{"standards", "the Crossref page is not JSON: " + err.Error()}
	}
	if p.Message == nil {
		return nil, "", 0, 0, &ErrSchema{"standards", "the Crossref page has no message"}
	}
	for _, w := range p.Message.Items {
		title := clean(html.UnescapeString(strings.TrimRight(strings.Join(w.Title, " "), " :")))
		if sub := clean(html.UnescapeString(strings.Join(w.Subtitle, " "))); sub != "" {
			title += ": " + sub
		}
		if !AITitle(title) && !ieeeAI.MatchString(title) {
			continue
		}
		doi := strings.ToLower(strings.TrimSpace(w.DOI))
		m := ieeeDOI.FindStringSubmatch(doi)
		if m == nil {
			skipped++
			continue
		}
		status := "final"
		if st := ieeeStatus.FindStringSubmatch(title); st != nil {
			status = strings.ToLower(st[1])
			title = title[len(st[0]):]
		} else if strings.Contains(title, "Draft") {
			status = "draft"
		}
		row := Row{ID: m[1], Name: title, Kind: "standard", URL: "https://doi.org/" + doi, Snapshot: snap,
			SameAs: map[string]string{"doi": doi}, Fields: map[string]any{"body": "IEEE", "status": status}}
		if w.StandardsBody != nil {
			if c := clean(html.UnescapeString(w.StandardsBody.Name)); c != "" {
				row.Fields["committee"] = c
			}
		}
		setFact(&row, "approved", crossrefDate(w.Approved), "approved")
		setFact(&row, "issued", crossrefDate(w.Issued), "issued")
		rows = append(rows, row)
	}
	return rows, p.Message.NextCursor, len(p.Message.Items), skipped, nil
}

// crossrefPages reads the pages of a Crossref list by cursor, 1,000 a page, until a short page or the limit.
func crossrefPages(ctx context.Context, r *Run, o Options, what string, page func(since, cursor string) string,
	read func(b []byte, snap string) ([]Row, string, int, int, error)) (out []Row, skipped int, err error) {
	cursor := "*"
	for i := 1; ; i++ {
		b, snap, err := r.Get(ctx, page(o.Since, cursor), 64<<20)
		if err != nil {
			return nil, 0, fmt.Errorf("standards: %s page %d: %w", what, i, err)
		}
		rows, next, n, s, err := read(b, snap)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, rows...)
		skipped += s
		if n < 1000 || next == "" || o.Limit > 0 && len(out) >= o.Limit {
			return out, skipped, nil
		}
		cursor = next
	}
}

// harvestStandards reads the publications of the standards bodies about AI, from Crossref: every report of NIST's
// prefix 10.6028 and every standard of IEEE's prefix 10.1109, 1,000 a page, of which the ones with an AI title are
// kept. -since keeps the works issued on or after the date.
func harvestStandards(ctx context.Context, r *Run, o Options) ([]Row, error) {
	nist, skipped, err := crossrefPages(ctx, r, o, "NIST", NISTURL, NISTPage)
	if err != nil {
		return nil, err
	}
	r.Skip(skipped, "translations of NIST reports and placeholder DOIs")
	ieee, skipped, err := crossrefPages(ctx, r, o, "IEEE", IEEEURL, IEEEPage)
	if err != nil {
		return nil, err
	}
	r.Skip(skipped, "IEEE standards with a DOI outside 10.1109/ieeestd")
	out := append(nist, ieee...)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return keep(out, o.Limit), nil
}
