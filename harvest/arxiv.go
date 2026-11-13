// SPDX-License-Identifier: Apache-2.0

package harvest

import (
	"bytes"
	"context"
	"encoding/xml"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

// arXiv's robots.txt closes export.arxiv.org to robots, so the adapter reads the OAI-PMH interface, which arXiv
// provides for harvesting metadata in bulk and which its robots.txt leaves open.
const ArxivOAI = "https://oaipmh.arxiv.org/oai"

// ArxivCategories are the categories of crosswalk/arxiv.yaml (05.8), used when the content tree has no crosswalk.
var ArxivCategories = []string{"cs.AI", "cs.LG", "cs.CL", "cs.CV", "cs.NE", "cs.RO", "cs.MA", "cs.IR", "cs.SD", "cs.CY", "cs.CR", "stat.ML", "eess.AS", "eess.IV"}

// ArxivCrosswalk is the crosswalk register of arXiv categories.
const ArxivCrosswalk = "crosswalk/arxiv.yaml"

// arxivCategories reads the ids of crosswalk/arxiv.yaml: a list of rows with an `id`, or a map with a `rows` list.
func arxivCategories(root string) ([]string, error) {
	b, err := os.ReadFile(filepath.Join(root, ArxivCrosswalk))
	if os.IsNotExist(err) {
		return ArxivCategories, nil
	}
	if err != nil {
		return nil, err
	}
	type row struct {
		ID string `yaml:"id"`
	}
	var rows []row
	if yaml.Unmarshal(b, &rows) != nil {
		var doc struct {
			Rows []row `yaml:"rows"`
		}
		if err := yaml.Unmarshal(b, &doc); err != nil {
			return nil, &ErrSchema{"arxiv", ArxivCrosswalk + " is neither a list of rows nor a map with rows"}
		}
		rows = doc.Rows
	}
	var out []string
	for _, r := range rows {
		if catRE.MatchString(r.ID) {
			out = append(out, r.ID)
		}
	}
	if len(out) == 0 {
		return ArxivCategories, nil
	}
	return out, nil
}

var catRE = regexp.MustCompile(`^[a-z-]+\.[A-Za-z-]+$`)

// ArxivSet is the OAI set of a category: "cs.LG" is "cs:cs:LG", "stat.ML" is "stat:stat:ML".
func ArxivSet(cat string) string {
	arch, sub, _ := strings.Cut(cat, ".")
	return arch + ":" + arch + ":" + sub
}

// ArxivURL is the URL of a ListRecords request: the first page of a set from a date, or the page of a token.
func ArxivURL(set, from, token string) string {
	q := url.Values{"verb": {"ListRecords"}}
	if token != "" {
		q.Set("resumptionToken", token)
	} else {
		q.Set("metadataPrefix", "arXiv")
		q.Set("set", set)
		if from != "" {
			q.Set("from", from)
		}
	}
	return ArxivOAI + "?" + q.Encode()
}

type oaiResponse struct {
	Error *struct {
		Code string `xml:"code,attr"`
		Text string `xml:",chardata"`
	} `xml:"error"`
	Records []struct {
		Header struct {
			Status string `xml:"status,attr"`
			Date   string `xml:"datestamp"`
		} `xml:"header"`
		Meta struct {
			ID      string `xml:"id"`
			Created string `xml:"created"`
			Updated string `xml:"updated"`
			Authors []struct {
				Key   string `xml:"keyname"`
				Fore  string `xml:"forenames"`
				Affil string `xml:"affiliation"`
			} `xml:"authors>author"`
			Title      string `xml:"title"`
			Categories string `xml:"categories"`
			Journal    string `xml:"journal-ref"`
			DOI        string `xml:"doi"`
			Licence    string `xml:"license"`
		} `xml:"metadata>arXiv"`
	} `xml:"ListRecords>record"`
	Token string `xml:"ListRecords>resumptionToken"`
}

// ArxivLicence maps the licence URL of a paper to an SPDX id and a class of 19.1. The metadata is CC0 whatever the
// paper's licence; the class says what a node may take from the paper itself.
func ArxivLicence(u string) (id, class string) {
	u = strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(u, "http://"), "https://"), "/")
	switch {
	case u == "":
		return "arxiv-assumed", "D"
	case strings.Contains(u, "publicdomain/zero/1.0"):
		return "CC0-1.0", "A"
	case strings.Contains(u, "arxiv.org/licenses/nonexclusive-distrib/1.0"):
		return "arxiv-nonexclusive-1.0", "D"
	case strings.Contains(u, "arxiv.org/licenses/assumed-1991-2003"):
		return "arxiv-assumed", "D"
	case strings.Contains(u, "creativecommons.org/licenses/"):
		parts := strings.Split(strings.TrimPrefix(u[strings.Index(u, "licenses/"):], "licenses/"), "/")
		if len(parts) < 2 {
			break
		}
		id = "CC-" + strings.ToUpper(parts[0]) + "-" + parts[1]
		switch {
		case strings.Contains(parts[0], "nc") || strings.Contains(parts[0], "nd"):
			return id, "D"
		case strings.Contains(parts[0], "sa"):
			return id, "B"
		default:
			return id, "C"
		}
	}
	return "unknown", "D"
}

// ArxivRecords reads one page of ListRecords. It returns the rows, the resumption token ("" on the last page) and
// how many deleted records it skipped. A page with noRecordsMatch is an empty page.
func ArxivRecords(b []byte, snap string) (rows []Row, token string, deleted int, err error) {
	var r oaiResponse
	d := xml.NewDecoder(bytes.NewReader(b))
	d.Strict = false
	if err := d.Decode(&r); err != nil {
		return nil, "", 0, &ErrSchema{"arxiv", "the OAI page is not XML: " + err.Error()}
	}
	if r.Error != nil {
		if r.Error.Code == "noRecordsMatch" {
			return nil, "", 0, nil
		}
		return nil, "", 0, &ErrSchema{"arxiv", "OAI error " + r.Error.Code + ": " + clean(r.Error.Text)}
	}
	for _, rec := range r.Records {
		m := rec.Meta
		if rec.Header.Status == "deleted" || m.ID == "" {
			deleted++
			continue
		}
		row := Row{ID: m.ID, Name: clean(m.Title), Kind: "work", URL: "https://arxiv.org/abs/" + m.ID,
			SameAs: map[string]string{"arxiv": m.ID}, Snapshot: snap}
		if doi := strings.ToLower(clean(m.DOI)); doi != "" {
			if i := strings.IndexAny(doi, " ;,"); i > 0 {
				doi = doi[:i]
			}
			row.SameAs["doi"] = doi
		}
		var authors []string
		for _, a := range m.Authors {
			if n := clean(strings.TrimSpace(a.Fore + " " + a.Key)); n != "" {
				authors = append(authors, n)
			}
		}
		lic, class := ArxivLicence(clean(m.Licence))
		row.Fields = map[string]any{"type": "paper", "standing": "preprint", "licence": lic, "licence_class": class}
		if len(authors) > 0 {
			row.Fields["authors"] = authors
		}
		if j := clean(m.Journal); j != "" {
			row.Fields["journal_ref"] = j
		}
		if y := date(m.Created); y != "" {
			row.Fields["year"] = y[:4]
		}
		row.Labels = strings.Fields(m.Categories)
		setFact(&row, "submitted", date(m.Created), "created")
		setFact(&row, "updated", date(m.Updated), "updated")
		rows = append(rows, row)
	}
	return rows, strings.TrimSpace(r.Token), deleted, nil
}

// harvestArxiv lists each category from a date: -since, or a week before the run, since arXiv's whole history is a
// full run that needs -since 1991-01-01 on purpose. With a limit, each category gives its share of the limit, so a
// sample covers every category.
func harvestArxiv(ctx context.Context, r *Run, o Options) ([]Row, error) {
	cats, err := arxivCategories(r.Root)
	if err != nil {
		return nil, err
	}
	from := o.Since
	if from == "" {
		d, err := time.Parse("2006-01-02", r.Date)
		if err != nil {
			return nil, err
		}
		from = d.AddDate(0, 0, -7).Format("2006-01-02")
	}
	share := 0
	if o.Limit > 0 {
		share = (o.Limit + len(cats) - 1) / len(cats)
	}
	seen := map[string]bool{}
	var out []Row
	deleted := 0
	for _, cat := range cats {
		got, token := 0, ""
		for {
			b, snap, err := r.Get(ctx, ArxivURL(ArxivSet(cat), from, token), 64<<20)
			if err != nil {
				return nil, err
			}
			rows, next, del, err := ArxivRecords(b, snap)
			if err != nil {
				return nil, err
			}
			deleted += del
			for _, row := range rows {
				if seen[row.ID] || share > 0 && got >= share {
					continue
				}
				seen[row.ID] = true
				out = append(out, row)
				got++
			}
			if next == "" || share > 0 && got >= share {
				break
			}
			token = next
		}
	}
	r.Skip(deleted, "records that arXiv marks deleted")
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return keep(out, o.Limit), nil
}
