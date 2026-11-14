// SPDX-License-Identifier: Apache-2.0

package site

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tamnd/ai-agent/audit"
	"github.com/tamnd/ai-agent/claims"
	"github.com/tamnd/ai-agent/coverage"
	"github.com/tamnd/ai-agent/quality"
)

// dashboards writes the dashboards under /about/: the quality metrics of 15.9 and the evidence labels by area of 16.15.
func (b *builder) dashboards() error {
	if err := b.qualityPage(); err != nil {
		return err
	}
	return b.evidencePage()
}

// qualityReport is the report that aik report quality wrote to opt.Quality, or, without one, the metrics that the build
// can compute from the tree: all but the audit first-pass rate and the findings, which need the writer state and the
// issues.
func (b *builder) qualityReport() (quality.Report, error) {
	if b.opt.Quality != "" {
		data, err := os.ReadFile(b.opt.Quality)
		if err == nil {
			var r quality.Report
			if err := json.Unmarshal(data, &r); err != nil {
				return r, fmt.Errorf("%s: %w", b.opt.Quality, err)
			}
			return r, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return quality.Report{}, err
		}
	}
	rows, err := coverage.LoadUniverse(b.c.Root)
	if err != nil {
		return quality.Report{}, err
	}
	in := quality.Input{Corpus: b.c, Status: b.status, Cite: b.cite, Core: coverage.CoreIDs(rows), Now: b.opt.Now}
	if in.Links, err = quality.LoadLinks(filepath.Join(b.c.Root, filepath.FromSlash(audit.File))); err != nil {
		return quality.Report{}, err
	}
	if in.Waivers, err = quality.LoadWaivers(b.c.Root); err != nil {
		return quality.Report{}, err
	}
	return quality.Compute(in), nil
}

func (b *builder) qualityPage() error {
	r, err := b.qualityReport()
	if err != nil {
		return err
	}
	esc := template.HTMLEscapeString
	var h strings.Builder
	fmt.Fprintf(&h, `<p>Computed on %s over %d pages and %d core items. A value that misses its target is marked <span class="miss">below target</span>. The targets are also exit criteria of the milestones; a metric off target for two weeks opens an issue for the editors.</p>`, esc(r.Date), r.Nodes, r.Core)
	h.WriteString(`<table class="dash"><caption>The metrics of spec 15.9 and their targets</caption><thead><tr><th scope="col">Metric</th><th scope="col" class="num">Value</th><th scope="col" class="num">Count</th><th scope="col" class="num">Target at M3</th><th scope="col" class="num">Target at M5</th></tr></thead><tbody>`)
	cellOf := func(m quality.Metric, t *quality.Target, met *bool) string {
		s := esc(m.TargetText(t))
		if met != nil && !*met {
			return `<td class="num miss">` + s + ` <span class="sr">missed</span></td>`
		}
		return `<td class="num">` + s + `</td>`
	}
	for _, m := range r.Metrics {
		count := ""
		if m.Value != nil {
			count = fmt.Sprintf("%d of %d", m.Num, m.Den)
		}
		name := esc(m.Name)
		switch {
		case m.NotComputed != "":
			name += `<br><span class="muted">not computed: ` + esc(m.NotComputed) + `</span>`
		case m.Note != "":
			name += `<br><span class="muted">` + esc(m.Note) + `</span>`
		}
		fmt.Fprintf(&h, `<tr id="%s"><th scope="row">%s</th><td class="num">%s</td><td class="num">%s</td>%s%s</tr>`,
			esc(m.ID), name, esc(m.Text()), count, cellOf(m, m.M3, m.MetM3), cellOf(m, m.M5, m.MetM5))
	}
	h.WriteString(`</tbody></table>`)
	h.WriteString(`<p>A metric whose records the project does not keep yet is listed with the reason and no value; the dashboard never shows a guess. The same numbers are in <a href="/about/quality.json">quality.json</a>.</p>`)
	desc := "The numbers that show whether the checks, the audit and the review work, against the targets of each milestone."
	p := DocPage{Base: b.base("Quality", desc, "/about/quality/"), Heading: "Quality", Lead: desc, Body: template.HTML(h.String())}
	p.Section = "about"
	if err := b.page("doc", p.Path, p); err != nil {
		return err
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return b.write("about/quality.json", append(data, '\n'))
}

// EvidenceRow is a row of /about/evidence/: the claims of the pages of a top area by headline label.
type EvidenceRow struct {
	Area    string         `json:"area"`
	Name    string         `json:"name"`
	Claims  int            `json:"claims"`
	ByLabel map[string]int `json:"by_label"`
}

// evidenceLabels are the columns of /about/evidence/: the headline labels in their order, and the claims with none.
var evidenceLabels = append(append([]string{}, claims.HeadlineOrder...), claims.NormativeReported, "none")

func (b *builder) evidenceRows() []EvidenceRow {
	by := map[string]*EvidenceRow{}
	total := &EvidenceRow{Area: "", Name: "All areas", ByLabel: map[string]int{}}
	for _, n := range b.c.Nodes {
		if len(n.Claims) == 0 {
			continue
		}
		top := topArea(n.Area)
		r := by[top]
		if r == nil {
			r = &EvidenceRow{Area: top, Name: top, ByLabel: map[string]int{}}
			if a, ok := b.c.AreaByID[top]; ok && a.Name != "" {
				r.Name = a.Name
			}
			by[top] = r
		}
		for _, c := range n.Claims {
			l := c.Headline
			if l == "" {
				l = "none"
			}
			r.Claims++
			r.ByLabel[l]++
			total.Claims++
			total.ByLabel[l]++
		}
	}
	rows := make([]EvidenceRow, 0, len(by)+1)
	for _, r := range by {
		rows = append(rows, *r)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })
	return append(rows, *total)
}

func (b *builder) evidencePage() error {
	rows := b.evidenceRows()
	esc := template.HTMLEscapeString
	var h strings.Builder
	h.WriteString(`<p>The share of the claims of each area by the label on its chip, the strongest evidence that the build found for it. A claim that is <b>proved</b> has a checked proof; <b>contested</b> has a source that disputes it; <b>measured-here</b> was run by an experiment of this site; <b>replicated</b> has an independent source that confirms it; <b>reported</b> rests on one source. The <a href="/badges/">badges</a> page says how each label is computed.</p>`)
	h.WriteString(`<table class="dash"><caption>Claims by area and headline label</caption><thead><tr><th scope="col">Area</th><th scope="col" class="num">Claims</th>`)
	for _, l := range evidenceLabels {
		fmt.Fprintf(&h, `<th scope="col" class="num">%s</th>`, esc(l))
	}
	h.WriteString(`</tr></thead><tbody>`)
	for _, r := range rows {
		name := esc(r.Name)
		if r.Area != "" {
			if _, ok := b.c.AreaByID[r.Area]; ok {
				name = `<a href="/` + esc(r.Area) + `/">` + name + `</a>`
			}
		}
		fmt.Fprintf(&h, `<tr><th scope="row">%s</th><td class="num">%d</td>`, name, r.Claims)
		for _, l := range evidenceLabels {
			k := r.ByLabel[l]
			if k == 0 {
				h.WriteString(`<td class="num muted">0</td>`)
				continue
			}
			fmt.Fprintf(&h, `<td class="num" title="%d of %d">%.0f%%</td>`, k, r.Claims, 100*float64(k)/float64(r.Claims))
		}
		h.WriteString(`</tr>`)
	}
	h.WriteString(`</tbody></table>`)
	desc := "The share of claims by evidence label in each area: proved, measured here, replicated, reported or contested."
	p := DocPage{Base: b.base("Evidence", desc, "/about/evidence/"), Heading: "Evidence", Lead: desc, Body: template.HTML(h.String())}
	p.Section = "about"
	return b.page("doc", p.Path, p)
}
