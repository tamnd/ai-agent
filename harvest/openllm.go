// SPDX-License-Identifier: Apache-2.0

package harvest

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// OpenLLMRows is the page of rows of the archive of the Open LLM Leaderboard (v2, closed in March 2025) on the
// dataset server of Hugging Face: 100 rows a request.
const OpenLLMRows = "https://datasets-server.huggingface.co/rows?dataset=open-llm-leaderboard/contents&config=default&split=train"

// OpenLLMPage is the number of rows of a page of the dataset server.
const OpenLLMPage = 100

// OpenLLMBench is a benchmark of the leaderboard: the column of its raw score and the number of shots of the
// leaderboard's protocol.
type OpenLLMBench struct {
	Name, Column string
	Shots        int
}

// OpenLLMBenches are the six benchmarks of the leaderboard v2, with the shots of its protocol (the about page of the
// leaderboard: IFEval 0, BBH 3, MATH Level 5 4, GPQA 0, MuSR 0, MMLU-PRO 5).
var OpenLLMBenches = []OpenLLMBench{
	{"IFEval", "IFEval", 0}, {"BBH", "BBH", 3}, {"MATH Lvl 5", "MATH Lvl 5", 4},
	{"GPQA", "GPQA", 0}, {"MuSR", "MUSR", 0}, {"MMLU-PRO", "MMLU-PRO", 5},
}

// openLLMUnquantized are the precisions of the leaderboard that are not a quantization.
var openLLMUnquantized = map[string]bool{"float16": true, "bfloat16": true, "float32": true}

// OpenLLMPageData is a page of the dataset server.
type OpenLLMPageData struct {
	Rows []struct {
		Row map[string]any `json:"row"`
	} `json:"rows"`
	Total int `json:"num_rows_total"`
}

// openLLMString is a string cell of a row, or "".
func openLLMString(row map[string]any, k string) string {
	if s, ok := row[k].(string); ok {
		return clean(s)
	}
	return ""
}

// OpenLLMResultRows makes the result rows of one row of the leaderboard: one for each benchmark with a score. The
// score is the raw score of the harness (a fraction); the leaderboard's normalized score is in the notes.
func OpenLLMResultRows(row map[string]any, snap string) ([]Row, string) {
	eval, model := openLLMString(row, "eval_name"), openLLMString(row, "fullname")
	if eval == "" || model == "" {
		return nil, "rows with no eval_name or no model"
	}
	precision := openLLMString(row, "Precision")
	submitted := date(openLLMString(row, "Submission Date"))
	var notes []string
	if t := openLLMString(row, "Type"); t != "" {
		notes = append(notes, "type: "+strings.TrimLeftFunc(t, func(c rune) bool { return !unicode.IsLetter(c) }))
	}
	if sha := openLLMString(row, "Model sha"); sha != "" {
		notes = append(notes, "revision "+sha)
	}
	if b, _ := row["Merged"].(bool); b {
		notes = append(notes, "a merge of models")
	}
	if b, _ := row["Chat Template"].(bool); b {
		notes = append(notes, "chat template")
	}
	var out []Row
	for _, b := range OpenLLMBenches {
		v, ok := row[b.Column+" Raw"].(float64)
		if !ok {
			continue
		}
		setting := map[string]any{"shots": b.Shots, "harness": "lm-evaluation-harness, Open LLM Leaderboard v2"}
		if precision != "" {
			if openLLMUnquantized[precision] {
				setting["quantization"] = "none (" + precision + ")"
			} else {
				setting["quantization"] = precision
			}
		}
		n := notes
		if norm, ok := row[b.Column].(float64); ok {
			n = append([]string{"normalized score " + strconv.FormatFloat(norm, 'f', 2, 64)}, notes...)
		}
		setting["notes"] = strings.Join(n, "; ")
		id := eval + "/" + strings.ToLower(strings.ReplaceAll(b.Name, " ", "-"))
		r := Row{ID: id, Name: model + " on " + b.Name + " (Open LLM Leaderboard)", Kind: "result",
			URL:    "https://huggingface.co/datasets/open-llm-leaderboard/contents",
			SameAs: map[string]string{"open_llm_leaderboard": id}, Snapshot: snap,
			Fields: map[string]any{
				"model":          model,
				"benchmark_name": "Open LLM Leaderboard v2: " + b.Name,
				"metric_name":    "Open LLM Leaderboard v2: " + b.Name + " raw score",
				"score":          map[string]any{"v": v, "unit": "fraction"},
				"setting":        setting,
				"source": map[string]any{"src": "S:open-llm-leaderboard", "kind": "leaderboard", "date": submitted,
					"loc": "open-llm-leaderboard/contents, row " + eval + ", column " + b.Column + " Raw"},
			}}
		if f, _ := row["Flagged"].(bool); f {
			r.Fields["warning"] = "flagged on the leaderboard"
		}
		if base := openLLMString(row, "Base Model"); base != "" && base != model {
			r.Links = map[string][]string{"base_model": {base}}
		}
		out = append(out, r)
	}
	if len(out) == 0 {
		return nil, "rows with no score"
	}
	return out, ""
}

// harvestOpenLLM reads the archive page by page. -since keeps the rows submitted on that date or later; -limit keeps
// at most that many result rows.
func harvestOpenLLM(ctx context.Context, r *Run, o Options) ([]Row, error) {
	var out []Row
	skipped := map[string]int{}
	total := -1
	for off := 0; total < 0 || off < total; off += OpenLLMPage {
		b, snap, err := r.Get(ctx, OpenLLMRows+"&offset="+strconv.Itoa(off)+"&length="+strconv.Itoa(OpenLLMPage), 16<<20)
		if err != nil {
			return nil, fmt.Errorf("open-llm-leaderboard, offset %d: %w", off, err)
		}
		var p OpenLLMPageData
		if err := json.Unmarshal(b, &p); err != nil {
			return nil, &ErrSchema{"open-llm-leaderboard", "a page is not JSON: " + err.Error()}
		}
		if p.Total <= 0 || len(p.Rows) == 0 {
			return nil, &ErrSchema{"open-llm-leaderboard", "the page at offset " + strconv.Itoa(off) + " has no rows"}
		}
		total = p.Total
		for _, pr := range p.Rows {
			if o.Since != "" && date(openLLMString(pr.Row, "Submission Date")) < o.Since {
				continue
			}
			rows, why := OpenLLMResultRows(pr.Row, snap)
			if why != "" {
				skipped[why]++
				continue
			}
			out = append(out, rows...)
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
