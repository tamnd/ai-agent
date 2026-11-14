// SPDX-License-Identifier: Apache-2.0

package harvest

import (
	"context"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// CVFBase is the site of the CVF open access papers: the main papers of CVPR, ICCV and WACV, with no robots.txt.
const CVFBase = "https://openaccess.thecvf.com/"

var (
	// cvfConfRE is a main conference of the menu: not its workshops, findings or demos.
	cvfConfRE  = regexp.MustCompile(`href="/?((CVPR|ICCV|WACV)(\d{4}))(?:\.py)?"`)
	cvfDayRE   = regexp.MustCompile(`href="/?((?:CVPR|ICCV|WACV)\d{4})(?:\.py)?\?day=([0-9a-z-]+)"`)
	cvfEntryRE = regexp.MustCompile(`<dt class="ptitle">(?:<br>)?\s*<a href="([^"]+)">([^<]+)</a>\s*</dt>`)
	cvfAuthRE  = regexp.MustCompile(`<input type="hidden" name="query(?:_author)?" value="([^"]*)">`)
	cvfArxivRE = regexp.MustCompile(`href="https?://arxiv\.org/abs/([0-9a-z./-]+?)(?:v\d+)?"`)
)

// CVFConf is a main conference of the site, such as CVPR2024.
type CVFConf struct {
	ID    string
	Venue string
	Year  int
}

// CVFMenu reads the conferences of the menu, the newest first.
func CVFMenu(b []byte) ([]CVFConf, error) {
	seen := map[string]bool{}
	var out []CVFConf
	for _, m := range cvfConfRE.FindAllStringSubmatch(string(b), -1) {
		if seen[m[1]] {
			continue
		}
		seen[m[1]] = true
		y, _ := strconv.Atoi(m[3])
		out = append(out, CVFConf{m[1], m[2], y})
	}
	if len(out) == 0 {
		return nil, &ErrSchema{"cvf", "the menu lists no CVPR, ICCV or WACV"}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Year != out[j].Year {
			return out[i].Year > out[j].Year
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

// CVFDays gives the URLs of the pages of the days of a conference page that lists none of its papers: its page of all
// days if it has one, else a page for each day.
func CVFDays(b []byte, conf string) []string {
	var all string
	var days []string
	for _, m := range cvfDayRE.FindAllStringSubmatch(string(b), -1) {
		if m[1] != conf {
			continue
		}
		u := CVFBase + conf + "?" + url.Values{"day": {m[2]}}.Encode()
		if m[2] == "all" {
			all = u
		} else if !strings.Contains(strings.Join(days, " "), u) {
			days = append(days, u)
		}
	}
	if all != "" {
		return []string{all}
	}
	return days
}

// CVFPapers reads the papers of a page of a conference: a row for each, with its authors and its arXiv id.
func CVFPapers(b []byte, snap string, c CVFConf) []Row {
	s := string(b)
	idx := cvfEntryRE.FindAllStringSubmatchIndex(s, -1)
	var out []Row
	for k, ix := range idx {
		end := len(s)
		if k+1 < len(idx) {
			end = idx[k+1][0]
		}
		if e := strings.Index(s[ix[1]:end], "</dl>"); e >= 0 {
			end = ix[1] + e
		}
		href, name := s[ix[2]:ix[3]], clean(html.UnescapeString(s[ix[4]:ix[5]]))
		file := href[strings.LastIndex(href, "/")+1:]
		file = strings.TrimSuffix(file, ".html")
		if name == "" || file == "" {
			continue
		}
		u, err := url.Parse(CVFBase)
		if err != nil {
			continue
		}
		ref, err := u.Parse(href)
		if err != nil {
			continue
		}
		id := c.ID + "/" + file
		row := Row{ID: id, Name: name, Kind: "work", URL: ref.String(), Snapshot: snap,
			SameAs: map[string]string{"cvf": id}, Fields: map[string]any{"venue": c.Venue, "year": c.Year}}
		body := s[ix[1]:end]
		var authors []string
		for _, a := range cvfAuthRE.FindAllStringSubmatch(body, -1) {
			if n := clean(html.UnescapeString(a[1])); n != "" {
				authors = append(authors, n)
			}
		}
		if len(authors) > 0 {
			row.Fields["authors"] = authors
		}
		if m := cvfArxivRE.FindStringSubmatch(body); m != nil {
			row.SameAs["arxiv"] = m[1]
		}
		out = append(out, row)
	}
	return out
}

// harvestCVF reads the main papers of the conferences of the CVF open access site, the newest first: the menu, then the
// page of each conference, or the pages of its days. -since keeps the conferences of that year and later; -limit keeps
// at most that many papers.
func harvestCVF(ctx context.Context, r *Run, o Options) ([]Row, error) {
	b, _, err := r.Get(ctx, CVFBase+"menu", 8<<20)
	if err != nil {
		return nil, fmt.Errorf("cvf menu: %w", err)
	}
	confs, err := CVFMenu(b)
	if err != nil {
		return nil, err
	}
	since := 0
	if len(o.Since) >= 4 {
		since, _ = strconv.Atoi(o.Since[:4])
	}
	seen := map[string]bool{}
	var out []Row
	empty := 0
	for _, c := range confs {
		if c.Year < since || o.Limit > 0 && len(out) >= o.Limit {
			continue
		}
		b, snap, err := r.Get(ctx, CVFBase+c.ID, 64<<20)
		if err != nil {
			return nil, fmt.Errorf("cvf %s: %w", c.ID, err)
		}
		rows := CVFPapers(b, snap, c)
		if len(rows) == 0 {
			for _, u := range CVFDays(b, c.ID) {
				db, dsnap, err := r.Get(ctx, u, 64<<20)
				if err != nil {
					return nil, fmt.Errorf("cvf %s: %w", c.ID, err)
				}
				rows = append(rows, CVFPapers(db, dsnap, c)...)
			}
		}
		if len(rows) == 0 {
			empty++
			continue
		}
		for _, row := range rows {
			if !seen[row.ID] {
				seen[row.ID] = true
				out = append(out, row)
			}
		}
	}
	if empty > 0 {
		r.Partial = true
		r.Skip(empty, "conferences of the menu whose pages list no paper")
	}
	return keep(out, o.Limit), nil
}
