// SPDX-License-Identifier: Apache-2.0

package harvest

import (
	"context"
	"fmt"
	"regexp"
	"sort"

	"go.yaml.in/yaml/v3"
)

// ATLASURL is the data file of MITRE ATLAS: the tactics, techniques, mitigations and case studies of attacks on AI
// systems, in one YAML file of the data repository (Apache-2.0).
const ATLASURL = "https://raw.githubusercontent.com/mitre-atlas/atlas-data/main/dist/ATLAS.yaml"

type atlasRef struct {
	ID  string `yaml:"id"`
	URL string `yaml:"url"`
}

type atlasObject struct {
	ID          string    `yaml:"id"`
	Name        string    `yaml:"name"`
	Type        string    `yaml:"object-type"`
	Tactics     []string  `yaml:"tactics"`
	Specializes string    `yaml:"specializes"`
	Maturity    string    `yaml:"maturity"`
	Created     string    `yaml:"created_date"`
	Modified    string    `yaml:"modified_date"`
	ATTACK      *atlasRef `yaml:"ATT&CK-reference"`
	Category    []string  `yaml:"category"`
	Lifecycle   []string  `yaml:"ml-lifecycle"`
	Techniques  []struct {
		ID string `yaml:"id"`
	} `yaml:"techniques"`
}

type atlasCase struct {
	ID          string `yaml:"id"`
	Name        string `yaml:"name"`
	Type        string `yaml:"case-study-type"`
	Date        string `yaml:"incident-date"`
	Granularity string `yaml:"incident-date-granularity"`
	Target      string `yaml:"target"`
	Procedure   []struct {
		Tactic    string `yaml:"tactic"`
		Technique string `yaml:"technique"`
	} `yaml:"procedure"`
}

type atlasFile struct {
	ID       string `yaml:"id"`
	Version  string `yaml:"version"`
	Matrices []struct {
		ID          string        `yaml:"id"`
		Tactics     []atlasObject `yaml:"tactics"`
		Techniques  []atlasObject `yaml:"techniques"`
		Mitigations []atlasObject `yaml:"mitigations"`
	} `yaml:"matrices"`
	Cases []atlasCase `yaml:"case-studies"`
}

var atlasIDRE = regexp.MustCompile(`^AML\.(TA|T|M|CS)\d{4}(\.\d{3})?$`)

// atlasPage is the page of an object on the ATLAS site: /tactics/AML.TA0002, /techniques/AML.T0000.000.
func atlasPage(kind, id string) string { return "https://atlas.mitre.org/" + kind + "/" + id }

// ATLASRows gives a row for each tactic (a concept), technique (a risk: spec 04 names ATLAS as a risk taxonomy),
// mitigation (a method) and case study (an incident when ATLAS calls it one, a finding when it is an exercise, a
// demonstration in research or a red team) of the ATLAS data file. A row holds the ids, the name, the
// structure (the tactics of a technique, its parent, the techniques a mitigation or a case study names), the maturity
// and the dates; never the descriptions, and never the actor of a case study, which can name a person (19, P family).
func ATLASRows(b []byte, snap string) (rows []Row, version string, skipped int, err error) {
	var f atlasFile
	if err := yaml.Unmarshal(b, &f); err != nil {
		return nil, "", 0, &ErrSchema{"mitre-atlas", "ATLAS.yaml is not the data file: " + err.Error()}
	}
	if f.ID != "ATLAS" || len(f.Matrices) == 0 {
		return nil, "", 0, &ErrSchema{"mitre-atlas", "ATLAS.yaml has no ATLAS matrix"}
	}
	m := f.Matrices[0]
	if len(m.Techniques) == 0 || len(m.Tactics) == 0 {
		return nil, "", 0, &ErrSchema{"mitre-atlas", "the ATLAS matrix has no tactics or no techniques"}
	}
	base := func(o atlasObject, kind, page string) (Row, bool) {
		if !atlasIDRE.MatchString(o.ID) || clean(o.Name) == "" {
			return Row{}, false
		}
		r := Row{ID: o.ID, Name: clean(o.Name), Kind: kind, URL: atlasPage(page, o.ID), Snapshot: snap,
			SameAs: map[string]string{"atlas": o.ID}, Labels: []string{"ATLAS " + o.Type},
			Fields: map[string]any{"atlas_type": o.Type}}
		setFact(&r, "created", o.Created, o.ID+" created_date")
		setFact(&r, "modified", o.Modified, o.ID+" modified_date")
		if o.ATTACK != nil && o.ATTACK.ID != "" {
			// Not a same_as id: several ATLAS techniques adapt one ATT&CK technique (AML.T0011 and AML.T0011.003 are
			// both T1204), so the ATT&CK id would join different items.
			r.Fields["attack"] = o.ATTACK.ID
		}
		return r, true
	}
	for _, o := range m.Tactics {
		r, ok := base(o, "concept", "tactics")
		if !ok {
			skipped++
			continue
		}
		rows = append(rows, r)
	}
	for _, o := range m.Techniques {
		r, ok := base(o, "risk", "techniques")
		if !ok {
			skipped++
			continue
		}
		r.Links = map[string][]string{}
		if len(o.Tactics) > 0 {
			r.Links["tactic"] = o.Tactics
		}
		if o.Specializes != "" {
			r.Links["specializes"] = []string{o.Specializes}
		}
		if len(r.Links) == 0 {
			r.Links = nil
		}
		setFact(&r, "maturity", o.Maturity, o.ID+" maturity")
		rows = append(rows, r)
	}
	for _, o := range m.Mitigations {
		r, ok := base(o, "method", "mitigations")
		if !ok {
			skipped++
			continue
		}
		var ts []string
		for _, t := range o.Techniques {
			ts = append(ts, t.ID)
		}
		if len(ts) > 0 {
			r.Links = map[string][]string{"mitigates": ts}
		}
		setFact(&r, "category", o.Category, o.ID+" category")
		setFact(&r, "ml_lifecycle", o.Lifecycle, o.ID+" ml-lifecycle")
		rows = append(rows, r)
	}
	for _, c := range f.Cases {
		if !atlasIDRE.MatchString(c.ID) || clean(c.Name) == "" {
			skipped++
			continue
		}
		kind := "finding"
		if c.Type == "incident" {
			kind = "incident"
		}
		r := Row{ID: c.ID, Name: clean(c.Name), Kind: kind, URL: atlasPage("studies", c.ID), Snapshot: snap,
			SameAs: map[string]string{"atlas": c.ID}, Labels: []string{"ATLAS case study"},
			Fields: map[string]any{"atlas_type": "case-study", "case_study_type": c.Type}}
		seen := map[string]bool{}
		var ts []string
		for _, p := range c.Procedure {
			if p.Technique != "" && !seen[p.Technique] {
				seen[p.Technique] = true
				ts = append(ts, p.Technique)
			}
		}
		if len(ts) > 0 {
			r.Links = map[string][]string{"technique": ts}
		}
		setFact(&r, "date", c.Date, c.ID+" incident-date")
		setFact(&r, "date_granularity", c.Granularity, c.ID+" incident-date-granularity")
		setFact(&r, "target", clean(c.Target), c.ID+" target")
		rows = append(rows, r)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	return rows, f.Version, skipped, nil
}

func harvestATLAS(ctx context.Context, r *Run, o Options) ([]Row, error) {
	b, snap, err := r.Get(ctx, ATLASURL, 16<<20)
	if err != nil {
		return nil, fmt.Errorf("mitre-atlas: %w", err)
	}
	rows, version, skipped, err := ATLASRows(b, snap)
	if err != nil {
		return nil, err
	}
	r.Skip(skipped, "objects with no ATLAS id or no name")
	if version != "" {
		r.Notes = append(r.Notes, "ATLAS version "+version)
	}
	return keep(rows, o.Limit), nil
}
