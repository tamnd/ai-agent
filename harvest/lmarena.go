// SPDX-License-Identifier: Apache-2.0

package harvest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// LMArenaDataset is the dataset of the published leaderboards of LMArena (now arena.ai) on Hugging Face: a config for
// each arena, each with the split latest (the leaderboard published last) and full (every one published).
const LMArenaDataset = "lmarena-ai/leaderboard-dataset"

// LMArenaSplits lists the configs of the dataset on the dataset server of Hugging Face.
const LMArenaSplits = "https://datasets-server.huggingface.co/splits?dataset=" + LMArenaDataset

// LMArenaCategory is the category of a leaderboard that the adapter reads: overall. The other categories (coding,
// math, a language and others) rank the same models on a subset of the battles.
const LMArenaCategory = "overall"

// LMArenaPage is the number of rows of a page of the dataset server.
const LMArenaPage = 100

// lmarenaRating are the columns of a config with a Bradley-Terry rating. The Agent Arena configs (agent and
// agent_*) give an IPS score with other columns instead, which is not a rating (10.10).
var lmarenaRating = []string{"model_name", "rating", "rating_lower", "rating_upper", "vote_count", "category", "leaderboard_publish_date"}

// LMArenaRows is the URL of a page of the rows of the split latest of a config. The adapter keeps the rows of the
// category overall; the filter endpoint of the dataset server would select them, but it failed for this dataset when
// the adapter was written (500 on 2026-11-14), while the rows endpoint answered.
func LMArenaRows(config string, offset int) string {
	q := url.Values{}
	q.Set("dataset", LMArenaDataset)
	q.Set("config", config)
	q.Set("split", "latest")
	q.Set("offset", strconv.Itoa(offset))
	q.Set("length", strconv.Itoa(LMArenaPage))
	return "https://datasets-server.huggingface.co/rows?" + q.Encode()
}

// LMArenaConfigs reads the answer of the splits endpoint: the configs that have the split latest, sorted.
func LMArenaConfigs(b []byte) ([]string, error) {
	var d struct {
		Splits []struct {
			Config string `json:"config"`
			Split  string `json:"split"`
		} `json:"splits"`
	}
	if err := json.Unmarshal(b, &d); err != nil {
		return nil, &ErrSchema{"lmarena", "the list of splits is not JSON: " + err.Error()}
	}
	var out []string
	for _, s := range d.Splits {
		if s.Split == "latest" && s.Config != "" {
			out = append(out, s.Config)
		}
	}
	if len(out) == 0 {
		return nil, &ErrSchema{"lmarena", "no config has the split latest"}
	}
	sort.Strings(out)
	return out, nil
}

// lmarenaAgent tells if a config is of the Agent Arena, whose score is not a rating.
func lmarenaAgent(config string) bool {
	return config == "agent" || strings.HasPrefix(config, "agent_")
}

// LMArenaRow makes the result row of a row of a leaderboard, or says why it has none. The result is a rating (10.10):
// the metric rating, the score with its 95% interval and its number of battles, the arena (the config) and the date of
// the snapshot. aik ingest resolves the model by its name and the arena by the crosswalk.
func LMArenaRow(config string, row map[string]any, snap string) (Row, string) {
	str := func(k string) string {
		s, _ := row[k].(string)
		return clean(s)
	}
	num := func(k string) (float64, bool) {
		f, ok := row[k].(float64)
		return f, ok && !math.IsNaN(f) && !math.IsInf(f, 0)
	}
	model, day := str("model_name"), date(str("leaderboard_publish_date"))
	if model == "" || day == "" {
		return Row{}, "rows with no model or no date"
	}
	if str("category") != LMArenaCategory {
		return Row{}, "rows of another category than overall"
	}
	rating, ok1 := num("rating")
	lo, ok2 := num("rating_lower")
	hi, ok3 := num("rating_upper")
	votes, ok4 := num("vote_count")
	if !ok1 || !ok2 || !ok3 || !ok4 || votes < 1 {
		return Row{}, "rows with no rating, interval or votes"
	}
	notes := []string{"category overall"}
	if r, ok := num("rank"); ok {
		notes = append(notes, "rank "+strconv.FormatFloat(r, 'f', -1, 64))
	}
	if strings.HasSuffix(config, "_style_control") {
		notes = append(notes, "style control")
	}
	id := config + "/" + model
	out := Row{ID: id, Name: model + " on LMArena " + config, Kind: "result",
		URL:    "https://huggingface.co/datasets/" + LMArenaDataset,
		SameAs: map[string]string{"lmarena": id}, Snapshot: snap,
		Fields: map[string]any{
			"model":          model,
			"benchmark_name": "LMArena " + config,
			"metric":         "rating",
			"score":          map[string]any{"v": rating, "unit": "rating", "interval": []float64{lo, hi}, "votes": int(votes)},
			"arena":          "LMArena " + config,
			"snapshot":       day,
			"setting":        map[string]any{"harness": "LMArena", "notes": strings.Join(notes, "; ")},
			"source": map[string]any{"src": "S:lmarena", "kind": "leaderboard", "date": day,
				"loc": LMArenaDataset + ", config " + config + ", split latest, row " + model + ", column rating"},
		}}
	if org := str("organization"); org != "" {
		out.Fields["developer"] = org
	}
	return out, ""
}

// lmarenaPage is a page of the rows endpoint.
type lmarenaPage struct {
	Rows []struct {
		Row map[string]any `json:"row"`
	} `json:"rows"`
	Features []struct {
		Name string `json:"name"`
	} `json:"features"`
	Total *int `json:"num_rows_total"`
}

// harvestLMArena reads the category overall of the latest leaderboard of each arena with a rating. -since keeps the
// leaderboards published on that date or later; -limit keeps at most that many rows.
func harvestLMArena(ctx context.Context, r *Run, o Options) ([]Row, error) {
	b, _, err := r.Get(ctx, LMArenaSplits, 1<<20)
	if err != nil {
		return nil, fmt.Errorf("lmarena splits: %w", err)
	}
	configs, err := LMArenaConfigs(b)
	if err != nil {
		return nil, err
	}
	var out []Row
	skipped := map[string]int{}
	for _, cfg := range configs {
		if lmarenaAgent(cfg) {
			skipped["Agent Arena leaderboards, whose IPS score is not a rating"]++
			continue
		}
		total := -1
		for off := 0; total < 0 || off < total; off += LMArenaPage {
			b, snap, err := r.Get(ctx, LMArenaRows(cfg, off), 16<<20)
			var se *StatusError
			if errors.As(err, &se) && se.Code >= 500 {
				// The dataset server could not serve the config (it answered 501 for text on 2026-11-14, a job
				// "terminated due to a temporary spike in resource usage"). The run reads the other configs and is
				// partial, so it removes no row of the config from the inventory.
				r.Partial = true
				r.Notes = append(r.Notes, fmt.Sprintf("the dataset server could not serve the config %s (%s); its rows are kept from the last run", cfg, se.Status))
				break
			}
			if err != nil {
				return nil, fmt.Errorf("lmarena %s, offset %d: %w", cfg, off, err)
			}
			var p lmarenaPage
			if err := json.Unmarshal(b, &p); err != nil {
				return nil, &ErrSchema{"lmarena", cfg + ": a page is not JSON: " + err.Error()}
			}
			if p.Total == nil {
				return nil, &ErrSchema{"lmarena", cfg + ": a page has no num_rows_total"}
			}
			if off == 0 {
				have := map[string]bool{}
				for _, f := range p.Features {
					have[f.Name] = true
				}
				for _, c := range lmarenaRating {
					if !have[c] {
						return nil, &ErrSchema{"lmarena", cfg + " has no column " + c}
					}
				}
			}
			total = *p.Total
			for _, pr := range p.Rows {
				row, why := LMArenaRow(cfg, pr.Row, snap)
				if why == "" && o.Since != "" && row.Fields["snapshot"].(string) < o.Since {
					why = "leaderboards published before -since"
				}
				if why != "" {
					skipped[why]++
					continue
				}
				out = append(out, row)
			}
			if len(p.Rows) == 0 {
				break
			}
		}
		if o.Limit > 0 && len(out) >= o.Limit {
			break
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
