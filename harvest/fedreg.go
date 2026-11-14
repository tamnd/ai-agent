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
)

// FedRegAPI is the documents search of the API of the Federal Register. It needs no key.
const FedRegAPI = "https://www.federalregister.gov/api/v1/documents.json"

// FedRegTerm is the phrase that the search looks for in the full text of the documents.
const FedRegTerm = `"artificial intelligence"`

// fedRegFields are the fields that the adapter asks for: no abstract and no text.
var fedRegFields = []string{"title", "type", "subtype", "document_number", "publication_date", "agencies",
	"executive_order_number", "citation", "signing_date", "effective_on", "html_url"}

// FedRegURL is page n (from 1) of the documents with the phrase, the oldest first, published on or after since when
// it is a date.
func FedRegURL(since string, page int) string {
	v := url.Values{"conditions[term]": {FedRegTerm}, "order": {"oldest"}, "per_page": {"1000"},
		"page": {strconv.Itoa(page)}, "fields[]": fedRegFields}
	if since != "" {
		v.Set("conditions[publication_date][gte]", since)
	}
	return FedRegAPI + "?" + v.Encode()
}

// FedRegDoc is a document of a page.
type FedRegDoc struct {
	Title     string `json:"title"`
	Type      string `json:"type"`
	Subtype   string `json:"subtype"`
	Number    string `json:"document_number"`
	Published string `json:"publication_date"`
	Agencies  []struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	} `json:"agencies"`
	EO        string `json:"executive_order_number"`
	Citation  string `json:"citation"`
	Signed    string `json:"signing_date"`
	Effective string `json:"effective_on"`
	URL       string `json:"html_url"`
}

// FedRegPage reads a page: the documents, the number of pages, and the count of the query.
func FedRegPage(b []byte) ([]FedRegDoc, int, int, error) {
	var p struct {
		Count   *int        `json:"count"`
		Pages   int         `json:"total_pages"`
		Results []FedRegDoc `json:"results"`
		Message string      `json:"message"`
	}
	if err := json.Unmarshal(b, &p); err != nil {
		return nil, 0, 0, &ErrSchema{"federal-register", "the page is not JSON: " + err.Error()}
	}
	if p.Count == nil {
		return nil, 0, 0, &ErrSchema{"federal-register", "the page has no count: " + p.Message}
	}
	for _, d := range p.Results {
		if d.Number == "" || d.Title == "" {
			return nil, 0, 0, &ErrSchema{"federal-register", "a document of the page has no document_number or no title"}
		}
	}
	return p.Results, p.Pages, *p.Count, nil
}

var (
	aiTitleRE      = regexp.MustCompile(`\bAI\b`)
	aiTitleWordsRE = regexp.MustCompile(`(?i)artificial intelligence|machine learning|automated decision|algorithmic|` +
		`large language model|generative model|deepfake|deep learning|neural network`)
)

// AITitle tells if a title is about AI: a document that only mentions AI in its text, such as a notice of a meeting
// that lists it among other topics, is not a policy of S-L1.
func AITitle(t string) bool { return aiTitleRE.MatchString(t) || aiTitleWordsRE.MatchString(t) }

// FedRegRow is the row of a document: a policy with its type, agencies and dates.
func FedRegRow(d FedRegDoc, snap string) Row {
	row := Row{ID: d.Number, Name: clean(d.Title), Kind: "policy", URL: d.URL,
		SameAs:   map[string]string{"federal_register": d.Number},
		Fields:   map[string]any{"type": d.Type, "jurisdiction": "US"},
		Snapshot: snap}
	if row.URL == "" {
		row.URL = "https://www.federalregister.gov/d/" + d.Number
	}
	if d.Subtype != "" {
		row.Fields["subtype"] = d.Subtype
	}
	if d.Citation != "" {
		row.Fields["citation"] = d.Citation
	}
	if d.EO != "" {
		row.SameAs["executive_order"] = d.EO
		row.Aliases = []string{"Executive Order " + d.EO}
	}
	var agencies []string
	for _, a := range d.Agencies {
		if a.Slug != "" {
			agencies = append(agencies, a.Slug)
		} else if n := clean(a.Name); n != "" {
			agencies = append(agencies, n)
		}
	}
	if len(agencies) > 0 {
		row.Links = map[string][]string{"agency": agencies}
	}
	if d.Type != "Notice" {
		// A rule, a proposed rule or a presidential document is an instrument of S-L1; a notice is one only when a
		// curator says so (guidance, a framework), since most notices of AI are meetings and requests for comment.
		setSignal(&row, "instrument", 1)
	}
	setFact(&row, "published", date(d.Published), "publication_date")
	setFact(&row, "signed", date(d.Signed), "signing_date")
	setFact(&row, "effective", date(d.Effective), "effective_on")
	return row
}

// harvestFedReg reads the documents of the Federal Register that have the phrase "artificial intelligence", and keeps
// those whose title is about AI. -since keeps the documents published on or after the date; -limit keeps at most that
// many rows.
func harvestFedReg(ctx context.Context, r *Run, o Options) ([]Row, error) {
	var out []Row
	other := 0
	for page := 1; ; page++ {
		b, snap, err := r.Get(ctx, FedRegURL(o.Since, page), 32<<20)
		if err != nil {
			return nil, fmt.Errorf("federal register page %d: %w", page, err)
		}
		docs, pages, _, err := FedRegPage(b)
		if err != nil {
			return nil, err
		}
		for _, d := range docs {
			if !AITitle(d.Title) {
				other++
				continue
			}
			out = append(out, FedRegRow(d, snap))
		}
		if page >= pages || len(docs) == 0 || o.Limit > 0 && len(out) >= o.Limit {
			break
		}
	}
	if other > 0 {
		r.Skip(other, "documents that mention artificial intelligence in their text but not in their title")
	}
	out = keep(out, o.Limit)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}
