// SPDX-License-Identifier: Apache-2.0

package harvest

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/tamnd/ai-agent/card"
)

// EpochBenchURL is the data of Epoch AI's Benchmarking Hub: one zip with a CSV for each benchmark, the table of the
// benchmarks (benchmark_metadata.csv) and the table of the model versions (model_metadata.csv).
const EpochBenchURL = "https://epoch.ai/data/benchmark_data.zip"

// EpochHub is the register source of the hub, the src of its results.
const EpochHub = "S:epoch-benchmarking-hub"

// epochEfforts are the reasoning efforts that end a model version of the hub ("claude-opus-4-8_max").
var epochEfforts = map[string]bool{"none": true, "minimal": true, "low": true, "medium": true, "high": true,
	"xhigh": true, "max": true}

// epochBudgetRE is a thinking budget that ends a model version ("_32K").
var epochBudgetRE = regexp.MustCompile(`^\d+K$`)

// epochShotsRE is the number of shots as the hub writes it: "5", "0-shot", "25-shot".
var epochShotsRE = regexp.MustCompile(`^(\d+)(?:-shot)?$`)

// EpochBench is a benchmark of the hub: its file, the column of its score and the scale of the column (1 for a
// fraction, 0.01 for a percentage).
type EpochBench struct {
	Name, File, Column string
	Scale              float64
}

// epochUnit is the unit of a scale of the hub, or "" for a scale that is not a fraction or a percentage.
func epochUnit(scale float64) string {
	switch scale {
	case 1:
		return "fraction"
	case 0.01:
		return "percent"
	}
	return ""
}

// EpochBenchRows reads the zip of the hub: a result row for each score of each benchmark that benchmark_metadata.csv
// maps to a file. A run of the hub (a file with no suffix _external) has the hub as its source; a score that the hub
// copies from another leaderboard or paper (_external) has the hub as its source, which is what the snapshot holds,
// and the leaderboard or paper as its original. It returns the rows and the counts of what it skipped, by reason.
func EpochBenchRows(b []byte, snap, today string, o Options) ([]Row, map[string]int, error) {
	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		return nil, nil, &ErrSchema{"epoch-benchmarks", "the data is not a zip: " + err.Error()}
	}
	files := map[string][]byte{}
	for _, f := range zr.File {
		if f.FileInfo().IsDir() || !strings.HasSuffix(f.Name, ".csv") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, nil, err
		}
		data, err := io.ReadAll(io.LimitReader(rc, 64<<20))
		_ = rc.Close()
		if err != nil {
			return nil, nil, err
		}
		files[path.Base(f.Name)] = data
	}
	if files["benchmark_metadata.csv"] == nil || files["model_metadata.csv"] == nil {
		return nil, nil, &ErrSchema{"epoch-benchmarks", "the zip has no benchmark_metadata.csv or no model_metadata.csv"}
	}
	bt, err := readTable("epoch-benchmarks", files["benchmark_metadata.csv"], "benchmark", "source_file", "score_column", "scale")
	if err != nil {
		return nil, nil, err
	}
	mt, err := readTable("epoch-benchmarks", files["model_metadata.csv"], "model_version", "model_group")
	if err != nil {
		return nil, nil, err
	}
	groups := map[string]string{}
	for _, rec := range mt.rows {
		if v, g := mt.get(rec, "model_version"), clean(mt.get(rec, "model_group")); v != "" && g != "" {
			groups[v] = g
		}
	}
	skipped := map[string]int{}
	mapped := map[string]bool{"benchmark_metadata.csv": true, "model_metadata.csv": true}
	var out []Row
	for _, rec := range bt.rows {
		file := bt.get(rec, "source_file")
		if file == "" {
			continue
		}
		mapped[file] = true
		scale, _ := number(bt.get(rec, "scale"))
		bench := EpochBench{Name: clean(bt.get(rec, "benchmark")), File: file, Column: bt.get(rec, "score_column"), Scale: scale}
		data := files[file]
		if data == nil {
			skipped["benchmarks whose file is not in the zip"]++
			continue
		}
		if epochUnit(scale) == "" {
			skipped["benchmarks whose scale is not a fraction or a percentage"]++
			continue
		}
		rows, err := EpochBenchFile(bench, data, groups, snap, today, o, skipped)
		if err != nil {
			return nil, nil, err
		}
		out = append(out, rows...)
	}
	for f := range files {
		if !mapped[f] {
			skipped["files that benchmark_metadata.csv does not map to a benchmark"]++
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return keep(out, o.Limit), skipped, nil
}

// EpochBenchFile reads the CSV of one benchmark of the hub.
func EpochBenchFile(bench EpochBench, data []byte, groups map[string]string, snap, today string, o Options,
	skipped map[string]int) ([]Row, error) {
	t, err := readTable("epoch-benchmarks", data, "Model version", bench.Column)
	if err != nil {
		return nil, err
	}
	stem := strings.TrimSuffix(bench.File, ".csv")
	external := strings.HasSuffix(stem, "_external")
	stem = strings.TrimSuffix(stem, "_external")
	unit := epochUnit(bench.Scale)
	var out []Row
	seen := map[string]bool{}
	for _, rec := range t.rows {
		version := t.get(rec, "Model version")
		v, ok := number(t.get(rec, bench.Column))
		if !ok {
			skipped["rows with no score"]++
			continue
		}
		model := groups[version]
		if model == "" {
			model = clean(t.get(rec, "Name"))
		}
		if model == "" {
			skipped["rows whose model version has no model in model_metadata.csv"]++
			continue
		}
		run := date(t.get(rec, "Started at"))
		if o.Since != "" && run != "" && run < o.Since {
			continue
		}
		key := t.get(rec, "id")
		if key == "" {
			key = card.Slugify(version)
			if key == "" {
				key = card.Slugify(model)
			}
		}
		id := stem + "/" + key
		for n := 2; seen[id]; n++ {
			id = stem + "/" + key + "~" + strconv.Itoa(n)
		}
		seen[id] = true
		setting := map[string]any{}
		if i := strings.LastIndexByte(version, '_'); i > 0 {
			switch suf := version[i+1:]; {
			case epochEfforts[suf]:
				setting["cot"] = "reasoning-model (effort " + suf + ")"
			case epochBudgetRE.MatchString(suf):
				setting["cot"] = "reasoning-model (budget " + suf + " tokens)"
			}
		}
		if m := epochShotsRE.FindStringSubmatch(t.get(rec, "Shots")); m != nil {
			n, _ := strconv.Atoi(m[1])
			setting["shots"] = n
		}
		var notes []string
		if version != "" {
			notes = append(notes, "Epoch AI model version "+version)
		}
		if se := t.get(rec, "stderr"); se != "" {
			notes = append(notes, "standard error "+se)
		}
		if n := clean(t.get(rec, "Notes")); n != "" {
			notes = append(notes, n)
		}
		if len(notes) > 0 {
			setting["notes"] = strings.Join(notes, "; ")
		}
		loc := bench.File + ", row " + key + ", column " + bench.Column
		src := map[string]any{"src": EpochHub, "kind": "leaderboard", "loc": loc}
		row := Row{ID: id, Name: model + " on " + bench.Name, Kind: "result", URL: "https://epoch.ai/benchmarks",
			SameAs: map[string]string{"epoch_result": id}, Snapshot: snap,
			Fields: map[string]any{
				"model":          model,
				"benchmark_name": bench.Name,
				"metric_name":    bench.Name + ": " + bench.Column,
				"score":          map[string]any{"v": v, "unit": unit},
				"setting":        setting,
				"source":         src,
			}}
		if external {
			// The hub copies the score; the source is the hub's table as of the snapshot, and the original is named.
			src["date"] = today
			orig := map[string]any{}
			name, link := clean(t.get(rec, "Source")), clean(t.get(rec, "Source link"))
			if link == "" && strings.HasPrefix(name, "http") {
				name, link = "", name
			}
			if name != "" {
				orig["name"] = name
			}
			if link != "" {
				orig["src"] = pwcPaperSrc(link)
			}
			if len(orig) > 0 {
				row.Fields["original"] = orig
			}
		} else {
			// A run of the hub: Epoch AI runs the model on its own protocol and publishes the log.
			src["date"] = run
			if run == "" {
				src["date"] = today
			}
			setting["harness"] = "Epoch AI Benchmarking Hub"
			if run != "" {
				setting["date_run"] = run
			}
			if l := t.get(rec, "Log viewer"); strings.HasPrefix(l, "https://") {
				row.URL = l
			}
		}
		out = append(out, row)
	}
	return out, nil
}

// harvestEpochBench reads the zip of the hub in one request. -since keeps the runs of that date and later (and every
// copied score, which has no date of its own); -limit keeps at most that many rows.
func harvestEpochBench(ctx context.Context, r *Run, o Options) ([]Row, error) {
	b, snap, err := r.Get(ctx, EpochBenchURL, 64<<20)
	if err != nil {
		return nil, err
	}
	rows, skipped, err := EpochBenchRows(b, snap, r.Date, o)
	if err != nil {
		return nil, err
	}
	whys := make([]string, 0, len(skipped))
	for w := range skipped {
		whys = append(whys, w)
	}
	sort.Strings(whys)
	for _, w := range whys {
		r.Skip(skipped[w], w)
	}
	return rows, nil
}
