// SPDX-License-Identifier: Apache-2.0

package harvest

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// PwCRevision is the revision of the Hugging Face dataset pwc-archive/files that the adapter reads. The site closed
// in July 2025 and its last dump does not change, so the revision is pinned and a new one is a reviewed change.
const PwCRevision = "1af3a546770f9b98665523ccba4b22b4f3e18c1f" //nolint:gosec // G101: a git revision, not a credential

// PwCURL is the URL of a file of the dump at the pinned revision.
func PwCURL(file string) string {
	return "https://huggingface.co/datasets/pwc-archive/files/resolve/" + PwCRevision + "/" + file
}

func harvestPwC(ctx context.Context, r *Run, o Options) ([]Row, error) {
	b, snap, err := r.Get(ctx, PwCURL("jul-28-methods.json.gz"), 64<<20)
	if err != nil {
		return nil, err
	}
	methods, spam, err := PwCMethods(b, snap, o)
	if err != nil {
		return nil, err
	}
	r.Skip(spam, "methods of the dump whose names are advertising, not methods")
	b, snap, err = r.Get(ctx, PwCURL("jul-28-datasets.json.gz"), 64<<20)
	if err != nil {
		return nil, err
	}
	datasets, tasks, err := PwCDatasets(b, snap, o)
	if err != nil {
		return nil, err
	}
	return append(append(methods, datasets...), tasks...), nil
}

type pwcPaper struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

type pwcMethod struct {
	URL         string    `json:"url"`
	Name        string    `json:"name"`
	FullName    string    `json:"full_name"`
	Paper       *pwcPaper `json:"paper"`
	Year        any       `json:"introduced_year"`
	SourceURL   string    `json:"source_url"`
	NumPapers   int       `json:"num_papers"`
	Collections []struct {
		Collection string `json:"collection"`
		Area       string `json:"area"`
	} `json:"collections"`
}

type pwcDataset struct {
	URL        string    `json:"url"`
	Name       string    `json:"name"`
	FullName   string    `json:"full_name"`
	Homepage   string    `json:"homepage"`
	Paper      *pwcPaper `json:"paper"`
	Introduced string    `json:"introduced_date"`
	Licence    string    `json:"license_name"`
	Modalities []string  `json:"modalities"`
	Languages  []string  `json:"languages"`
	Variants   []string  `json:"variants"`
	NumPapers  int       `json:"num_papers"`
	Tasks      []struct {
		Task string `json:"task"`
		URL  string `json:"url"`
	} `json:"tasks"`
}

func gunzipJSON(source string, b []byte, v any) error {
	if len(b) > 2 && b[0] == 0x1f && b[1] == 0x8b {
		zr, err := gzip.NewReader(bytes.NewReader(b))
		if err != nil {
			return err
		}
		if b, err = io.ReadAll(io.LimitReader(zr, 1<<30)); err != nil {
			return err
		}
	}
	if err := json.Unmarshal(b, v); err != nil {
		return &ErrSchema{source, err.Error()}
	}
	return nil
}

// spamRE finds the advertising that fills about three quarters of the methods of the final dump: customer service
// phone numbers and travel questions, with the brackets and symbols that they use.
var spamRE = regexp.MustCompile(`(?i)[?¿{}\[\]|@®【】~➡™⟷]|\d{3}[ .-]\d{3}|\b(contact|refunds?|customer|booking|airlines?|cruises?|phone|expedia|faqs?|cancel\w*|reservations?|flights?|tickets?)\b`)

// pwcID is the path of a Papers with Code URL: "method/dropout".
func pwcID(u string) string {
	u = strings.TrimSuffix(strings.TrimPrefix(u, "https://paperswithcode.com/"), "/")
	if strings.Contains(u, "://") {
		return ""
	}
	return u
}

// PwCMethods reads the methods of the dump, the most used first. It returns the rows and how many methods it
// skipped as advertising.
func PwCMethods(b []byte, snap string, o Options) ([]Row, int, error) {
	var ms []pwcMethod
	if err := gunzipJSON("pwc-dump", b, &ms); err != nil {
		return nil, 0, err
	}
	spam := 0
	var out []Row
	for _, m := range ms {
		id, name := pwcID(m.URL), clean(m.Name)
		if !strings.HasPrefix(id, "method/") || name == "" {
			continue
		}
		if spamRE.MatchString(name) || spamRE.MatchString(m.FullName) || len(name) > 80 {
			spam++
			continue
		}
		row := Row{ID: id, Name: name, Kind: "method", URL: m.URL, SameAs: map[string]string{"pwc": id}, Snapshot: snap}
		if f := clean(m.FullName); f != "" && !strings.EqualFold(f, name) {
			row.Aliases = []string{f}
		}
		for _, c := range m.Collections {
			if c.Collection != "" {
				row.Labels = append(row.Labels, c.Area+": "+c.Collection)
			}
		}
		if m.Paper != nil {
			if p := pwcID(m.Paper.URL); p != "" {
				row.Links = map[string][]string{"introduced_by": {p}}
			}
		}
		if ax := arxivFromURL(m.SourceURL); ax != "" {
			if row.Links == nil {
				row.Links = map[string][]string{}
			}
			row.Links["introduced_by_arxiv"] = []string{ax}
		}
		if y, ok := year(m.Year); ok && y > 1900 && y != 2000 {
			// The dump writes 2000 for an unknown year.
			setFact(&row, "introduced_year", y, "introduced_year")
		}
		setSignal(&row, "pwc_papers", m.NumPapers)
		out = append(out, row)
	}
	sortBy(out, "pwc_papers")
	return keep(out, o.Limit), spam, nil
}

// PwCDatasets reads the datasets of the dump and the tasks that they name, the most used first.
func PwCDatasets(b []byte, snap string, o Options) (datasets, tasks []Row, err error) {
	var ds []pwcDataset
	if err := gunzipJSON("pwc-dump", b, &ds); err != nil {
		return nil, nil, err
	}
	taskRows := map[string]*Row{}
	for _, d := range ds {
		id, name := pwcID(d.URL), clean(d.Name)
		if !strings.HasPrefix(id, "dataset/") || name == "" || spamRE.MatchString(name) {
			continue
		}
		if o.Since != "" && d.Introduced != "" && d.Introduced < o.Since {
			continue
		}
		row := Row{ID: id, Name: name, Kind: "dataset", URL: d.URL, SameAs: map[string]string{"pwc": id}, Snapshot: snap}
		if f := clean(d.FullName); f != "" && !strings.EqualFold(f, name) {
			row.Aliases = []string{f}
		}
		row.Labels = append(row.Labels, d.Modalities...)
		setFact(&row, "introduced", date(d.Introduced), "introduced_date")
		if d.Licence != "" && d.Licence != "Unknown" {
			setFact(&row, "licence_name", d.Licence, "license_name")
		}
		if len(d.Languages) > 0 {
			setFact(&row, "languages", d.Languages, "languages")
		}
		if strings.HasPrefix(d.Homepage, "http") {
			row.Fields = map[string]any{"homepage": d.Homepage}
		}
		links := map[string][]string{}
		if d.Paper != nil {
			if p := pwcID(d.Paper.URL); p != "" {
				links["introduced_by"] = []string{p}
			}
		}
		for _, t := range d.Tasks {
			tid, tname := pwcID(t.URL), clean(t.Task)
			if !strings.HasPrefix(tid, "task/") || tname == "" {
				continue
			}
			links["tasks"] = append(links["tasks"], tid)
			tr := taskRows[tid]
			if tr == nil {
				tr = &Row{ID: tid, Name: tname, Kind: "task", URL: t.URL, SameAs: map[string]string{"pwc": tid}, Snapshot: snap,
					Signals: map[string]any{"pwc_datasets": 0}}
				taskRows[tid] = tr
			}
			tr.Signals["pwc_datasets"] = tr.Signals["pwc_datasets"].(int) + 1
		}
		if len(links) > 0 {
			row.Links = links
		}
		setSignal(&row, "pwc_papers", d.NumPapers)
		datasets = append(datasets, row)
	}
	for _, t := range taskRows {
		tasks = append(tasks, *t)
	}
	sortBy(datasets, "pwc_papers")
	sortBy(tasks, "pwc_datasets")
	return keep(datasets, o.Limit), keep(tasks, o.Limit), nil
}

// sortBy puts the rows with the largest signal first, then by id.
func sortBy(rows []Row, signal string) {
	n := func(r Row) float64 {
		switch v := r.Signals[signal].(type) {
		case int:
			return float64(v)
		case float64:
			return v
		}
		return 0
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if a, b := n(rows[i]), n(rows[j]); a != b {
			return a > b
		}
		return rows[i].ID < rows[j].ID
	})
}

func year(v any) (int, bool) {
	switch y := v.(type) {
	case float64:
		return int(y), true
	case string:
		n, err := strconv.Atoi(y)
		return n, err == nil
	}
	return 0, false
}

var arxivURLRE = regexp.MustCompile(`^(?:https?://)?(?:www\.|export\.)?arxiv\.org/(?:abs|pdf)/(\d{4}\.\d{4,5}|[a-z-]+(?:\.[A-Z]{2})?/\d{7})`)

// arxivFromURL gives the arXiv id of an abs or pdf URL, without its version.
func arxivFromURL(u string) string {
	if m := arxivURLRE.FindStringSubmatch(u); m != nil {
		return m[1]
	}
	return ""
}
