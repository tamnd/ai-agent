// SPDX-License-Identifier: Apache-2.0

package harvest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// HELMProjects lists the leaderboards of HELM (Stanford CRFM), each with its releases, the newest first. The front
// end of crfm.stanford.edu/helm reads it.
const HELMProjects = "https://crfm.stanford.edu/helm/project_metadata.json"

// HELMBucket is where the outputs of each release of each project are, as JSON.
const HELMBucket = "https://storage.googleapis.com/crfm-helm-public/"

// HELMProjectsFile lists the projects to read; without it the adapter reads HELMDefault.
const HELMProjectsFile = "crosswalk/helm.yaml"

// HELMDefault are the projects of text models whose leaderboards hold scores of benchmarks: the general ones (Lite,
// Classic, Capabilities, MMLU), the safety ones, and the domain ones (medicine, finance, long context), and Instruct,
// whose scores are ratings. The image, audio and vision-language projects are not read.
var HELMDefault = []string{"capabilities", "lite", "classic", "mmlu", "safety", "air-bench", "medhelm", "finance",
	"long-context", "instruct"}

var (
	helmProjectRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,39}$`)
	helmReleaseRE = regexp.MustCompile(`^v\d+\.\d+\.\d+$`)
	helmGroupRE   = regexp.MustCompile(`^[A-Za-z0-9_-]{1,80}$`)
	// helmFraction are the metrics whose scores are proportions: exact match, F1, accuracy, correct answers, rates and
	// string match. The others (ratings such as Jury Score, WB Score, BLEU, ROUGE, ranking scores) are numbers.
	helmFraction = regexp.MustCompile(`(?i)(^|[ (])(EM|F1|acc|accuracy|correct|equivalent|rate|string match)\b|(Acc|F1)$`)
)

// HELMUnit is the unit of the scores of a metric of HELM.
func HELMUnit(metric string) string {
	if helmFraction.MatchString(metric) {
		return "fraction"
	}
	return "number"
}

// helmProjectList reads the projects file, or gives the default projects.
func helmProjectList(root string) ([]string, error) {
	b, err := os.ReadFile(filepath.Join(root, HELMProjectsFile))
	switch {
	case os.IsNotExist(err):
		return HELMDefault, nil
	case err != nil:
		return nil, err
	}
	var doc struct {
		Projects []string `yaml:"projects"`
	}
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return nil, &ErrSchema{"helm", HELMProjectsFile + " is not a map with projects: " + err.Error()}
	}
	for _, p := range doc.Projects {
		if !helmProjectRE.MatchString(p) {
			return nil, &ErrSchema{"helm", HELMProjectsFile + ": " + strconv.Quote(p) + " is not a project id"}
		}
	}
	if len(doc.Projects) == 0 {
		return nil, &ErrSchema{"helm", HELMProjectsFile + " names no project"}
	}
	return doc.Projects, nil
}

// HELMProject is a project of project_metadata.json with its newest release.
type HELMProject struct {
	ID      string
	Title   string
	Release string
}

// HELMReleases reads project_metadata.json: the newest release of each project, by id.
func HELMReleases(b []byte) (map[string]HELMProject, error) {
	var ps []struct {
		ID       string   `json:"id"`
		Title    string   `json:"title"`
		Releases []string `json:"releases"`
	}
	if err := json.Unmarshal(b, &ps); err != nil {
		return nil, &ErrSchema{"helm", "project_metadata.json is not a list of projects: " + err.Error()}
	}
	out := map[string]HELMProject{}
	for _, p := range ps {
		if helmProjectRE.MatchString(p.ID) && len(p.Releases) > 0 && helmReleaseRE.MatchString(p.Releases[0]) {
			out[p.ID] = HELMProject{p.ID, clean(p.Title), p.Releases[0]}
		}
	}
	if len(out) == 0 {
		return nil, &ErrSchema{"helm", "project_metadata.json has no project with a release"}
	}
	return out, nil
}

// helmBase is the folder of the outputs of a release of a project.
func helmBase(p HELMProject) string {
	return HELMBucket + p.ID + "/benchmark_output/releases/" + p.Release + "/"
}

// HELMModel is a model of the schema of a release.
type HELMModel struct {
	Name      string `json:"name"`
	Display   string `json:"display_name"`
	Short     string `json:"short_display_name"`
	Developer string `json:"creator_organization"`
}

// HELMModels reads schema.json: the models of a release by the names that the tables print.
func HELMModels(b []byte) (map[string]HELMModel, error) {
	var s struct {
		Models []HELMModel `json:"models"`
	}
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, &ErrSchema{"helm", "schema.json is not JSON: " + err.Error()}
	}
	if len(s.Models) == 0 {
		return nil, &ErrSchema{"helm", "schema.json has no models"}
	}
	out := map[string]HELMModel{}
	for _, m := range s.Models {
		for _, n := range []string{m.Short, m.Display} {
			if n = clean(n); n != "" {
				out[n] = m
			}
		}
	}
	return out, nil
}

// HELMSummary reads the date of a release from summary.json.
func HELMSummary(b []byte, release string) (string, error) {
	var s struct {
		Release string `json:"release"`
		Date    string `json:"date"`
	}
	if err := json.Unmarshal(b, &s); err != nil {
		return "", &ErrSchema{"helm", "summary.json is not JSON: " + err.Error()}
	}
	if s.Release != release || date(s.Date) == "" {
		return "", &ErrSchema{"helm", "summary.json is of release " + s.Release + " dated " + strconv.Quote(s.Date) + ", not of " + release}
	}
	return date(s.Date), nil
}

// helmCell is a cell of a table of HELM: a string or a number, with the runs it comes from.
type helmCell struct {
	Value any `json:"value"`
	Href  string
}

func (c *helmCell) UnmarshalJSON(b []byte) error {
	var x struct {
		Value json.RawMessage `json:"value"`
		Href  string          `json:"href"`
	}
	if err := json.Unmarshal(b, &x); err != nil {
		return err
	}
	c.Href = x.Href
	var f float64
	var s string
	switch {
	case len(x.Value) == 0 || string(x.Value) == "null":
	case json.Unmarshal(x.Value, &f) == nil:
		c.Value = f
	case json.Unmarshal(x.Value, &s) == nil:
		c.Value = s
	}
	return nil
}

type helmTable struct {
	Title  string `json:"title"`
	Header []struct {
		Value    string `json:"value"`
		Metadata struct {
			Metric   string `json:"metric"`
			RunGroup string `json:"run_group"`
		} `json:"metadata"`
	} `json:"header"`
	Rows [][]helmCell `json:"rows"`
}

// HELMMainGroup reads groups.json: the first group of its first table, which is the leaderboard of the project
// ("core_scenarios" in Lite).
func HELMMainGroup(b []byte) (string, error) {
	var ts []helmTable
	if err := json.Unmarshal(b, &ts); err != nil {
		return "", &ErrSchema{"helm", "groups.json is not a list of tables: " + err.Error()}
	}
	if len(ts) == 0 || len(ts[0].Rows) == 0 || len(ts[0].Rows[0]) == 0 {
		return "", &ErrSchema{"helm", "groups.json has no group"}
	}
	_, g, ok := strings.Cut(ts[0].Rows[0][0].Href, "group=")
	if !ok || !helmGroupRE.MatchString(g) {
		return "", &ErrSchema{"helm", "the first group of groups.json has no id: " + strconv.Quote(ts[0].Rows[0][0].Href)}
	}
	return g, nil
}

// HELMRows are the result rows of the first table of a group of a release: one for each score of a column that
// names a metric and a scenario. It gives the number of cells with no score.
func HELMRows(b []byte, p HELMProject, group, day string, models map[string]HELMModel, snap string) ([]Row, int, error) {
	var ts []helmTable
	if err := json.Unmarshal(b, &ts); err != nil {
		return nil, 0, &ErrSchema{"helm", "groups/" + group + ".json is not a list of tables: " + err.Error()}
	}
	if len(ts) == 0 || len(ts[0].Header) == 0 || !strings.HasPrefix(ts[0].Header[0].Value, "Model") {
		return nil, 0, &ErrSchema{"helm", "the first table of groups/" + group + ".json has no column Model"}
	}
	t := ts[0]
	file := "groups/" + group + ".json"
	var out []Row
	empty := 0
	for _, cells := range t.Rows {
		if len(cells) == 0 {
			continue
		}
		display, _ := cells[0].Value.(string)
		display = clean(display)
		if display == "" || strings.Contains(display, "/") {
			empty += len(cells) - 1
			continue
		}
		m := models[display]
		for i := 1; i < len(cells) && i < len(t.Header); i++ {
			h := t.Header[i]
			metric, scen := clean(h.Metadata.Metric), clean(h.Metadata.RunGroup)
			if metric == "" || scen == "" {
				continue // the mean win rate and the mean score are of the table, not of a benchmark
			}
			v, ok := cells[i].Value.(float64)
			if !ok || math.IsNaN(v) || math.IsInf(v, 0) {
				empty++
				continue
			}
			col := clean(h.Value)
			id := p.ID + "/" + p.Release + "/" + group + "/" + col + "/" + display
			notes := "HELM " + p.Title + " " + p.Release
			if m.Name != "" {
				notes += "; model " + m.Name
			}
			row := Row{ID: id, Name: display + " on HELM " + p.Title + " " + col, Kind: "result",
				URL:    "https://crfm.stanford.edu/helm/" + p.ID + "/" + p.Release + "/#/leaderboard/" + group,
				SameAs: map[string]string{"helm": id}, Snapshot: snap,
				Fields: map[string]any{
					"model":          display,
					"benchmark_name": "HELM " + p.Title + ": " + scen,
					"metric_name":    metric,
					"score":          map[string]any{"v": v, "unit": HELMUnit(metric)},
					"setting":        map[string]any{"harness": "HELM", "notes": notes},
					"source": map[string]any{"src": "S:helm", "kind": "leaderboard", "date": day,
						"loc": p.ID + " " + p.Release + ", " + file + ", table " + clean(t.Title) + ", row " + display + ", column " + col},
				}}
			if m.Developer != "" {
				row.Fields["developer"] = clean(m.Developer)
			}
			out = append(out, row)
		}
	}
	return out, empty, nil
}

// harvestHELM reads the leaderboard of the newest release of each project of crosswalk/helm.yaml: a result row for
// each score of each model on each scenario of the project's main table. A project whose files are not in the bucket
// is skipped and the run is partial. -since skips the releases dated before it; -limit keeps that many rows.
func harvestHELM(ctx context.Context, r *Run, o Options) ([]Row, error) {
	want, err := helmProjectList(r.Root)
	if err != nil {
		return nil, err
	}
	b, _, err := r.Get(ctx, HELMProjects, 4<<20)
	if err != nil {
		return nil, fmt.Errorf("helm projects: %w", err)
	}
	projects, err := HELMReleases(b)
	if err != nil {
		return nil, err
	}
	var out []Row
	empty := 0
	for _, id := range want {
		p, ok := projects[id]
		if !ok {
			r.Partial = true
			r.Notes = append(r.Notes, "project_metadata.json has no project "+id)
			continue
		}
		rows, n, err := helmProject(ctx, r, p, o)
		var se *StatusError
		if errors.As(err, &se) && se.Code == 404 {
			r.Partial = true
			r.Notes = append(r.Notes, fmt.Sprintf("the bucket has no file of %s %s (%s); its rows are kept from the last run", id, p.Release, se.URL))
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("helm %s %s: %w", id, p.Release, err)
		}
		out, empty = append(out, rows...), empty+n
		if o.Limit > 0 && len(out) >= o.Limit {
			break
		}
	}
	r.Skip(empty, "cells with no score")
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return keep(out, o.Limit), nil
}

func helmProject(ctx context.Context, r *Run, p HELMProject, o Options) ([]Row, int, error) {
	base := helmBase(p)
	b, _, err := r.Get(ctx, base+"summary.json", 1<<20)
	if err != nil {
		return nil, 0, err
	}
	day, err := HELMSummary(b, p.Release)
	if err != nil {
		return nil, 0, err
	}
	if o.Since != "" && day < o.Since {
		r.Notes = append(r.Notes, p.ID+" "+p.Release+" is dated "+day+", before -since")
		return nil, 0, nil
	}
	if b, _, err = r.Get(ctx, base+"schema.json", 64<<20); err != nil {
		return nil, 0, err
	}
	models, err := HELMModels(b)
	if err != nil {
		return nil, 0, err
	}
	if b, _, err = r.Get(ctx, base+"groups.json", 16<<20); err != nil {
		return nil, 0, err
	}
	group, err := HELMMainGroup(b)
	if err != nil {
		return nil, 0, err
	}
	b, snap, err := r.Get(ctx, base+"groups/"+group+".json", 128<<20)
	if err != nil {
		return nil, 0, err
	}
	return HELMRows(b, p, group, day, models, snap)
}
