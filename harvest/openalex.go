// SPDX-License-Identifier: Apache-2.0

package harvest

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// OpenAlexWorks is the works endpoint of the OpenAlex API. The adapter sends no API key and no address: without a
// key, OpenAlex gives a small free daily budget, which the top works of S-W1 fit in.
const OpenAlexWorks = "https://api.openalex.org/works"

// OpenAlexCrosswalk lists the parts of OpenAlex that are AI (5.8): subfields, such as 1702, and topics of other
// subfields, such as T10036. Without it the adapter reads subfield 1702, Artificial Intelligence.
const OpenAlexCrosswalk = "crosswalk/openalex.yaml"

// OpenAlexTop is the share of the works of each year and part that S-W1 makes cards: the top 2% by citations (18.5,
// Q-07). Only those are harvested; the other works stay out of the inventory, and search finds them by id (17).
const OpenAlexTop = 0.02

// openAlexSelect are the fields that the adapter reads; select keeps the pages small.
const openAlexSelect = "id,doi,display_name,publication_year,publication_date,cited_by_count,ids,type,primary_topic,authorships"

var (
	subfieldRE = regexp.MustCompile(`^(?:subfields/)?([0-9]{4})$`)
	topicRE    = regexp.MustCompile(`^T[0-9]+$`)
)

// OpenAlexPart is a part of OpenAlex that the adapter reads, as a filter of the API.
type OpenAlexPart struct {
	ID     string // "1702" or "T10036"
	Filter string // "primary_topic.subfield.id:1702"
}

// openAlexParts reads the crosswalk, or gives subfield 1702.
func openAlexParts(root string) ([]OpenAlexPart, error) {
	ids := []string{"1702"}
	b, err := os.ReadFile(filepath.Join(root, OpenAlexCrosswalk))
	switch {
	case os.IsNotExist(err):
	case err != nil:
		return nil, err
	default:
		var doc struct {
			Rows []struct {
				ID string `yaml:"id"`
			} `yaml:"rows"`
		}
		if err := yaml.Unmarshal(b, &doc); err != nil {
			return nil, &ErrSchema{"openalex", OpenAlexCrosswalk + " is not a map with rows: " + err.Error()}
		}
		ids = nil
		for _, r := range doc.Rows {
			ids = append(ids, r.ID)
		}
	}
	var out []OpenAlexPart
	seen := map[string]bool{}
	for _, id := range ids {
		var p OpenAlexPart
		if m := subfieldRE.FindStringSubmatch(id); m != nil {
			p = OpenAlexPart{ID: m[1], Filter: "primary_topic.subfield.id:" + m[1]}
		} else if topicRE.MatchString(id) {
			p = OpenAlexPart{ID: id, Filter: "primary_topic.id:" + id}
		} else {
			continue
		}
		if !seen[p.ID] {
			seen[p.ID] = true
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil, &ErrSchema{"openalex", OpenAlexCrosswalk + " names no subfield or topic"}
	}
	return out, nil
}

// OpenAlexYearsURL asks for the number of works of the part in each year, in one of two ranges of years, since the
// API gives at most 200 groups.
func OpenAlexYearsURL(p OpenAlexPart, old bool) string {
	years := "publication_year:>1999"
	if old {
		years = "publication_year:<2000"
	}
	return OpenAlexWorks + "?" + url.Values{"filter": {p.Filter + "," + years}, "group_by": {"publication_year"}}.Encode()
}

// OpenAlexPerPage is the most works of a page that the API supports.
const OpenAlexPerPage = 100

// OpenAlexDepth is how deep basic paging reads: page × per_page must not pass it. Cursor paging goes deeper, but past
// the first page it needs a key, which the adapter does not have.
const OpenAlexDepth = 10000

// OpenAlexPageURL is page n (from 1) of the works of the part in one year, the most cited first.
func OpenAlexPageURL(p OpenAlexPart, year, page, perPage int) string {
	return OpenAlexWorks + "?" + url.Values{
		"filter":   {p.Filter + ",publication_year:" + strconv.Itoa(year)},
		"sort":     {"cited_by_count:desc"},
		"per_page": {strconv.Itoa(perPage)},
		"page":     {strconv.Itoa(page)},
		"select":   {openAlexSelect},
	}.Encode()
}

// OpenAlexYears reads the group_by answer: the number of works in each year.
func OpenAlexYears(b []byte) (map[int]int, error) {
	var r struct {
		GroupBy *[]struct {
			Key   string `json:"key"`
			Count int    `json:"count"`
		} `json:"group_by"`
	}
	if err := json.Unmarshal(b, &r); err != nil || r.GroupBy == nil {
		return nil, &ErrSchema{"openalex", "the group_by answer has no group_by list"}
	}
	out := map[int]int{}
	for _, g := range *r.GroupBy {
		if y, err := strconv.Atoi(g.Key); err == nil {
			out[y] = g.Count
		}
	}
	return out, nil
}

type openAlexWork struct {
	ID    string         `json:"id"`
	DOI   string         `json:"doi"`
	Name  string         `json:"display_name"`
	Year  int            `json:"publication_year"`
	Date  string         `json:"publication_date"`
	Cited *int           `json:"cited_by_count"`
	Type  string         `json:"type"`
	IDs   map[string]any `json:"ids"`
	Topic *struct {
		ID       string `json:"id"`
		Name     string `json:"display_name"`
		Subfield struct {
			ID string `json:"id"`
		} `json:"subfield"`
	} `json:"primary_topic"`
	Authorships []struct {
		Author struct {
			ID    string `json:"id"`
			Name  string `json:"display_name"`
			ORCID string `json:"orcid"`
		} `json:"author"`
		Institutions []struct {
			ID string `json:"id"`
		} `json:"institutions"`
	} `json:"authorships"`
}

// oaID is the short id of an OpenAlex URL: https://openalex.org/W2626778328 → W2626778328.
func oaID(u string) string {
	return u[strings.LastIndex(u, "/")+1:]
}

var arxivDOIRE = regexp.MustCompile(`^10\.48550/arxiv\.(.+)$`)

// OpenAlexPage reads a page of works.
func OpenAlexPage(b []byte, snap string) ([]Row, error) {
	var r struct {
		Meta    *struct{}       `json:"meta"`
		Results *[]openAlexWork `json:"results"`
	}
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, &ErrSchema{"openalex", "the page is not JSON: " + err.Error()}
	}
	if r.Meta == nil || r.Results == nil {
		return nil, &ErrSchema{"openalex", "the page has no meta or no results"}
	}
	var rows []Row
	for _, w := range *r.Results {
		id := oaID(w.ID)
		name := clean(w.Name)
		if !strings.HasPrefix(id, "W") || name == "" {
			continue
		}
		row := Row{ID: id, Name: name, Kind: "work", URL: "https://openalex.org/" + id,
			SameAs: map[string]string{"openalex": id}, Snapshot: snap}
		if d := strings.ToLower(strings.TrimPrefix(w.DOI, "https://doi.org/")); d != "" {
			row.SameAs["doi"] = d
			if m := arxivDOIRE.FindStringSubmatch(d); m != nil {
				row.SameAs["arxiv"] = m[1]
			}
		}
		if mag, ok := w.IDs["mag"].(string); ok && mag != "" {
			row.SameAs["mag"] = mag
		}
		row.Fields = map[string]any{"type": w.Type}
		if w.Year > 0 {
			row.Fields["year"] = strconv.Itoa(w.Year)
		}
		var authors, aids, insts []string
		seenInst := map[string]bool{}
		for _, a := range w.Authorships {
			if n := clean(a.Author.Name); n != "" {
				authors = append(authors, n)
			}
			if a.Author.ID != "" {
				aids = append(aids, oaID(a.Author.ID))
			}
			for _, in := range a.Institutions {
				if i := oaID(in.ID); i != "" && !seenInst[i] {
					seenInst[i] = true
					insts = append(insts, i)
				}
			}
		}
		if len(authors) > 0 {
			row.Fields["authors"] = authors
		}
		if len(aids) > 0 || len(insts) > 0 {
			row.Links = map[string][]string{}
			if len(aids) > 0 {
				row.Links["author"] = aids
			}
			if len(insts) > 0 {
				sort.Strings(insts)
				row.Links["institution"] = insts
			}
		}
		if w.Topic != nil && w.Topic.ID != "" {
			row.Labels = []string{oaID(w.Topic.ID) + " " + clean(w.Topic.Name)}
		}
		setFact(&row, "published", date(w.Date), "publication_date")
		if w.Cited != nil {
			setSignal(&row, "cited_by", *w.Cited)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// OpenAlexNeed is the number of works of a year that S-W1 keeps: the top share, at least one when the year has any,
// and no deeper than basic paging reads.
func OpenAlexNeed(count int) int {
	if count <= 0 {
		return 0
	}
	return min(int(math.Ceil(float64(count)*OpenAlexTop)), OpenAlexDepth)
}

// Percentile is the citation percentile of the work of the rank among n works of its year, which S-W1 compares
// with 98: 100 for the first, and at least 98 for each work of the top 2%. It has two decimals.
func Percentile(rank, n int) float64 {
	if n <= 0 || rank < 1 {
		return 0
	}
	return math.Floor(10000-10000*float64(rank-1)/float64(n)) / 100
}

// harvestOpenAlex reads, for each part of the crosswalk and each year, the most cited works up to the top share of
// S-W1. -since keeps the years from its year on; -limit keeps at most that many works of each year and part. A work
// in two parts keeps the row of the first. Years after the year of the run are left out.
func harvestOpenAlex(ctx context.Context, r *Run, o Options) ([]Row, error) {
	parts, err := openAlexParts(r.Root)
	if err != nil {
		return nil, err
	}
	from := 0
	if o.Since != "" {
		if from, err = strconv.Atoi(o.Since[:min(4, len(o.Since))]); err != nil {
			return nil, fmt.Errorf("-since %s does not start with a year", o.Since)
		}
	}
	seen := map[string]bool{}
	var out []Row
	for _, p := range parts {
		counts := map[int]int{}
		for _, old := range []bool{true, false} {
			b, _, err := r.Get(ctx, OpenAlexYearsURL(p, old), 16<<20)
			if err != nil {
				return nil, fmt.Errorf("openalex %s: %w", p.ID, err)
			}
			c, err := OpenAlexYears(b)
			if err != nil {
				return nil, err
			}
			for y, n := range c {
				counts[y] += n
			}
		}
		// OpenAlex dates some works after the year of the run; they are errors of the source.
		last, _ := strconv.Atoi(r.Date[:min(4, len(r.Date))])
		years := make([]int, 0, len(counts))
		for y := range counts {
			if y >= from && (last == 0 || y <= last) {
				years = append(years, y)
			}
		}
		sort.Sort(sort.Reverse(sort.IntSlice(years)))
		skipped := 0
		for _, y := range years {
			need := OpenAlexNeed(counts[y])
			if o.Limit > 0 {
				need = min(need, o.Limit)
			}
			skipped += counts[y] - OpenAlexNeed(counts[y])
			per := min(OpenAlexPerPage, need)
			got := 0
			for page := 1; got < need && page*per <= OpenAlexDepth; page++ {
				b, snap, err := r.Get(ctx, OpenAlexPageURL(p, y, page, per), 64<<20)
				if err != nil {
					return nil, fmt.Errorf("openalex %s %d: %w", p.ID, y, err)
				}
				rows, err := OpenAlexPage(b, snap)
				if err != nil {
					return nil, err
				}
				if len(rows) == 0 {
					break
				}
				for _, row := range rows {
					if got >= need {
						break
					}
					got++
					if seen[row.ID] {
						continue
					}
					seen[row.ID] = true
					setSignal(&row, "rank_in_year", got)
					setSignal(&row, "works_in_year", counts[y])
					setSignal(&row, "citation_percentile", Percentile(got, counts[y]))
					out = append(out, row)
				}
			}
		}
		r.Skip(skipped, fmt.Sprintf("works of %s outside the top %g%% of their year by citations (S-W1)", p.ID, OpenAlexTop*100))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}
