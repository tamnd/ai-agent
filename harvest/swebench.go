// SPDX-License-Identifier: Apache-2.0

package harvest

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// SWEBenchData is the data of the leaderboards of swebench.com, in the repository of the site: one file with every
// leaderboard and its entries.
const SWEBenchData = "https://raw.githubusercontent.com/SWE-bench/swe-bench.github.io/master/data/leaderboards.json"

// SWEBenchVersions maps a leaderboard of the site to the version of the benchmark swe-bench (10.3); a leaderboard
// that is not here is read under its name in lowercase.
var SWEBenchVersions = map[string]string{"Test": "full", "Verified": "verified", "Lite": "lite",
	"Multimodal": "multimodal", "Multilingual": "multilingual"}

// SWEBenchMetric is the metric of every leaderboard: the percentage of the instances whose tests pass after the
// system's patch.
const SWEBenchMetric = "percent-resolved"

// SWEBenchEntry is an entry of a leaderboard: a system, the models that it uses, and its score.
type SWEBenchEntry struct {
	Name      string   `json:"name"`
	Folder    string   `json:"folder"`
	Resolved  *float64 `json:"resolved"`
	Date      string   `json:"date"`
	Model     string   `json:"model_display"`
	ModelOrg  string   `json:"model_org"`
	Agent     string   `json:"agent"`
	AgentOrg  string   `json:"agent_org"`
	Checked   any      `json:"checked"` // true, false, null, or a string that says false
	Effort    any      `json:"reasoning_effort"`
	Tags      []string `json:"tags"`
	Warning   any      `json:"warning"`
	MiniAgent string   `json:"mini-swe-agent_version"`
}

// SWEBenchBoard is a leaderboard of the site.
type SWEBenchBoard struct {
	Name    string          `json:"name"`
	Results []SWEBenchEntry `json:"results"`
}

// SWEBenchBoards reads the data file.
func SWEBenchBoards(b []byte) ([]SWEBenchBoard, error) {
	var d struct {
		Boards []SWEBenchBoard `json:"leaderboards"`
	}
	if err := json.Unmarshal(b, &d); err != nil {
		return nil, &ErrSchema{"swe-bench", "the data is not JSON: " + err.Error()}
	}
	if len(d.Boards) == 0 {
		return nil, &ErrSchema{"swe-bench", "the data has no leaderboards"}
	}
	for _, lb := range d.Boards {
		if lb.Name == "" {
			return nil, &ErrSchema{"swe-bench", "a leaderboard has no name"}
		}
		for _, e := range lb.Results {
			if e.Folder == "" || e.Name == "" {
				return nil, &ErrSchema{"swe-bench", "an entry of " + lb.Name + " has no folder or no name"}
			}
		}
	}
	return d.Boards, nil
}

// tagValues are the values of the tags "<key>: <value>" of an entry.
func tagValues(tags []string, key string) []string {
	var out []string
	for _, t := range tags {
		if k, v, ok := strings.Cut(t, ":"); ok && strings.TrimSpace(k) == key {
			if v = strings.TrimSpace(v); v != "" {
				out = append(out, v)
			}
		}
	}
	return out
}

// sweNoModel are the names that the leaderboard shows for a system that does not name one model.
var sweNoModel = map[string]bool{"undisclosed": true, "multiple": true}

// SWEBenchRow is the result row of an entry, or the reason it has none: no score, or no one model (rule M01). The
// model is the name that the
// leaderboard shows; aik ingest resolves it to a model card. The setting has the keys that the leaderboard states
// (10.5): the scaffold, the attempts and the split; the other keys are fixed by the protocol of the benchmark.
func SWEBenchRow(board string, e SWEBenchEntry, snap string) (Row, string) {
	if e.Resolved == nil {
		return Row{}, "entries with no score"
	}
	if m := clean(e.Model); m == "" || sweNoModel[strings.ToLower(m)] {
		return Row{}, "entries of systems that name no one model (M01)"
	}
	version, ok := SWEBenchVersions[board]
	if !ok {
		version = strings.ToLower(board)
	}
	id := version + "/" + e.Folder
	model := clean(e.Model)
	setting := map[string]any{"split": "test", "harness": "swe-bench"}
	if a := clean(e.Agent); a != "" {
		setting["scaffold"] = a
		if e.MiniAgent != "" {
			setting["scaffold"] = a + "@" + e.MiniAgent
		}
	} else if sys, _, ok := strings.Cut(e.Name, " + "); ok && clean(sys) != "" {
		setting["scaffold"] = clean(sys)
	}
	if at := tagValues(e.Tags, "System"); len(at) > 0 {
		for _, a := range at {
			if n, ok := strings.CutPrefix(a, "Attempts - "); ok {
				setting["compute"] = "attempts: " + n
			}
		}
	}
	if eff, ok := e.Effort.(string); ok && eff != "" {
		setting["cot"] = "reasoning-model (effort " + eff + ")"
	}
	if d := date(e.Date); d != "" {
		setting["date_run"] = d
	}
	models := tagValues(e.Tags, "Model")
	if len(models) > 1 {
		setting["notes"] = "the system uses " + strconv.Itoa(len(models)) + " models: " + strings.Join(models, ", ")
	}
	// The tag Org names the submitter, except on the runs of mini-SWE-agent, which the maintainers of the benchmark
	// make for each model and tag with the model's developer.
	submitter := ""
	if e.MiniAgent != "" {
		submitter = clean(e.AgentOrg)
	} else if orgs := tagValues(e.Tags, "Org"); len(orgs) > 0 {
		submitter = orgs[0]
	}
	row := Row{ID: id, Name: model + " on SWE-bench " + board, Kind: "result",
		URL:    "https://www.swebench.com/",
		SameAs: map[string]string{"swe_bench": id}, Snapshot: snap,
		Fields: map[string]any{
			"model":     model,
			"benchmark": "swe-bench",
			"version":   version,
			"metric":    SWEBenchMetric,
			"score":     map[string]any{"v": *e.Resolved, "unit": "percent"},
			"setting":   setting,
			"source": map[string]any{"src": "S:swe-bench-leaderboard", "kind": "leaderboard", "date": date(e.Date),
				"loc": board + " leaderboard, entry " + e.Folder + ", field resolved"},
			"checked": e.Checked == true,
		}}
	if submitter != "" {
		row.Fields["submitted_by"] = submitter
		// The developer of the model reports its own result when it submits it (10.6.1).
		if mo := clean(e.ModelOrg); mo != "" && strings.EqualFold(mo, submitter) {
			row.Fields["self_reported"] = true
		}
	}
	if len(models) > 0 {
		row.Links = map[string][]string{"model": models}
	}
	if e.Warning != nil {
		if w, ok := e.Warning.(string); ok && clean(w) != "" {
			row.Fields["warning"] = clean(w)
		}
	}
	return row, ""
}

// harvestSWEBench reads the leaderboards of SWE-bench: a result row for each entry with a score. -since keeps the
// entries of that date and later; -limit keeps at most that many rows.
func harvestSWEBench(ctx context.Context, r *Run, o Options) ([]Row, error) {
	b, snap, err := r.Get(ctx, SWEBenchData, 64<<20)
	if err != nil {
		return nil, fmt.Errorf("swe-bench leaderboards: %w", err)
	}
	boards, err := SWEBenchBoards(b)
	if err != nil {
		return nil, err
	}
	var out []Row
	skipped := map[string]int{}
	for _, lb := range boards {
		for _, e := range lb.Results {
			if o.Since != "" && date(e.Date) < o.Since {
				continue
			}
			row, why := SWEBenchRow(lb.Name, e, snap)
			if why != "" {
				skipped[why]++
				continue
			}
			out = append(out, row)
		}
	}
	whys := make([]string, 0, len(skipped))
	for w := range skipped {
		whys = append(whys, w)
	}
	sort.Strings(whys)
	for _, w := range whys {
		r.Skip(skipped[w], w)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return keep(out, o.Limit), nil
}
