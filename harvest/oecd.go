// SPDX-License-Identifier: Apache-2.0

package harvest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// OECDAPI is the public list of the policy initiatives of the OECD.AI policy observatory. It needs no key.
const OECDAPI = "https://api.oecd.ai/policy-initiatives/public"

// OECDPageSize is the number of initiatives of a page.
const OECDPageSize = 100

// OECDInstruments are the initiative types of OECD.AI that are instruments of S-L1 (18.5): laws, amendments,
// regulations, guidance, frameworks, national AI strategies, and the treaties, directives, decisions, recommendations
// and declarations of intergovernmental bodies. Programmes, bodies, events, standards and projects are not.
var OECDInstruments = map[int]string{
	49:  "Treaty",
	50:  "Regulation",
	51:  "Directive",
	52:  "Decision",
	53:  "Recommendation",
	55:  "Declaration, opinion, or outcome document",
	59:  "Guidance/guidelines",
	62:  "National AI strategy",
	102: "Law/legislation/act (by legislative body)",
	103: "Regulation (by government authority)",
	104: "Amendment to/repeal of existing legislation",
	105: "Guidance document (instructions on how to implement a law, regulation, policy or other rule)",
	106: "Principles/guidelines/frameworks for trustworthy AI",
	121: "Trade agreement that include AI",
}

// OECDURL is page n (from 1) of the initiatives.
func OECDURL(page int) string {
	return OECDAPI + "?" + url.Values{"page": {strconv.Itoa(page)}, "perPage": {strconv.Itoa(OECDPageSize)}}.Encode()
}

type oecdNamed struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// OECDInitiative is an initiative of a page: only the fields that the adapter keeps. The API also gives the
// descriptions, which are not facts, and the names and emails of the editors, which the adapter never reads.
type OECDInitiative struct {
	ID       int    `json:"id"`
	Name     string `json:"englishName"`
	Original string `json:"originalName"`
	Slug     string `json:"slug"`
	Website  string `json:"website"`
	Status   string `json:"status"`
	Category string `json:"category"`
	Binding  string `json:"extentBinding"`
	Start    *int   `json:"startYear"`
	End      *int   `json:"endYear"`
	Country  *struct {
		Name     string `json:"name"`
		Code     string `json:"code"`
		Category string `json:"category"`
	} `json:"gaiinCountry"`
	Org *struct {
		Name  string `json:"name"`
		Short string `json:"shortName"`
		Slug  string `json:"slug"`
	} `json:"intergovernmentalOrganisation"`
	Type    *oecdNamed  `json:"initiativeType"`
	Tags    []oecdNamed `json:"tags"`
	Sectors []oecdNamed `json:"targetSectors"`
	Edition string      `json:"editorialStatus"`
}

// OECDPage reads a page: the initiatives and the number of pages.
func OECDPage(b []byte) ([]OECDInitiative, int, error) {
	var p struct {
		Data     []OECDInitiative `json:"data"`
		LastPage *int             `json:"lastPage"`
		Message  string           `json:"message"`
	}
	if err := json.Unmarshal(b, &p); err != nil {
		return nil, 0, &ErrSchema{"oecd-ai", "the page is not JSON: " + err.Error()}
	}
	if p.LastPage == nil {
		return nil, 0, &ErrSchema{"oecd-ai", "the page has no lastPage: " + p.Message}
	}
	for _, it := range p.Data {
		if it.ID == 0 || clean(it.Name) == "" {
			return nil, 0, &ErrSchema{"oecd-ai", "an initiative of the page has no id or no englishName"}
		}
	}
	return p.Data, *p.LastPage, nil
}

func oecdNames(l []oecdNamed) []string {
	var out []string
	for _, x := range l {
		if n := clean(x.Name); n != "" {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
}

// OECDRow is the row of an initiative: a policy with its type, its jurisdiction (an ISO 3166 alpha-3 code, or the
// intergovernmental organisation) and its years.
func OECDRow(it OECDInitiative, snap string) Row {
	id := strconv.Itoa(it.ID)
	row := Row{ID: id, Name: clean(it.Name), Kind: "policy", SameAs: map[string]string{"oecd_ai": id},
		Fields: map[string]any{}, Snapshot: snap}
	if it.Slug != "" {
		row.URL = "https://oecd.ai/en/dashboards/policy-initiatives/" + it.Slug
	}
	if o := clean(it.Original); o != "" && o != row.Name {
		row.Aliases = []string{o}
	}
	if it.Type != nil {
		row.Fields["type"] = clean(it.Type.Name)
		if _, ok := OECDInstruments[it.Type.ID]; ok {
			setSignal(&row, "instrument", 1)
		}
	}
	switch {
	case it.Country != nil && it.Country.Code != "":
		row.Fields["jurisdiction"] = it.Country.Code
		row.Links = map[string][]string{"country": {it.Country.Code}}
	case it.Org != nil:
		org := clean(it.Org.Short)
		if org == "" {
			org = clean(it.Org.Name)
		}
		if org != "" {
			row.Fields["jurisdiction"] = org
			row.Links = map[string][]string{"organisation": {org}}
		}
	}
	for k, v := range map[string]string{"category": it.Category, "binding": it.Binding, "status": it.Status,
		"website": it.Website} {
		if v = clean(v); v != "" {
			row.Fields[k] = v
		}
	}
	if t := oecdNames(it.Tags); len(t) > 0 {
		row.Labels = t
	}
	if s := oecdNames(it.Sectors); len(s) > 0 {
		row.Fields["sectors"] = s
	}
	if it.Start != nil && *it.Start > 0 {
		setFact(&row, "started", strconv.Itoa(*it.Start), "startYear")
	}
	if it.End != nil && *it.End > 0 {
		setFact(&row, "ended", strconv.Itoa(*it.End), "endYear")
	}
	return row
}

// harvestOECD reads the published initiatives of the policy database of OECD.AI. -since keeps the initiatives that
// started in that year or later; -limit keeps at most that many rows.
func harvestOECD(ctx context.Context, r *Run, o Options) ([]Row, error) {
	since := 0
	if len(o.Since) >= 4 {
		since, _ = strconv.Atoi(o.Since[:4])
	}
	var out []Row
	unpublished := 0
	for page := 1; ; page++ {
		b, snap, err := r.Get(ctx, OECDURL(page), 32<<20)
		if err != nil {
			return nil, fmt.Errorf("oecd.ai page %d: %w", page, err)
		}
		items, last, err := OECDPage(b)
		if err != nil {
			return nil, err
		}
		for _, it := range items {
			if it.Edition != "" && !strings.EqualFold(it.Edition, "Published") {
				unpublished++
				continue
			}
			if since > 0 && (it.Start == nil || *it.Start < since) {
				continue
			}
			out = append(out, OECDRow(it, snap))
		}
		if page >= last || len(items) == 0 || o.Limit > 0 && len(out) >= o.Limit {
			break
		}
	}
	if unpublished > 0 {
		r.Skip(unpublished, "initiatives that are not published")
	}
	out = keep(out, o.Limit)
	sort.Slice(out, func(i, j int) bool {
		a, _ := strconv.Atoi(out[i].ID)
		b, _ := strconv.Atoi(out[j].ID)
		return a < b
	})
	return out, nil
}
