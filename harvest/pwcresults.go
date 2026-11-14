// SPDX-License-Identifier: Apache-2.0

package harvest

import (
	"context"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/tamnd/ai-agent/card"
)

// PwCEvalFile is the file of the evaluation tables of the dump: the leaderboards of Papers with Code, by task and
// dataset, with a row for each model of each paper and its metrics.
const PwCEvalFile = "jul-28-evaluation-tables.json.gz"

// PwCUnverified is the flag of a result of the dump until the citation check finds its number in the paper (Q-35).
const PwCUnverified = "pwc-unverified"

// PwCEvalTask is a task of the evaluation tables, with its datasets and its subtasks.
type PwCEvalTask struct {
	Task     string        `json:"task"`
	Datasets []PwCEvalSet  `json:"datasets"`
	Subtasks []PwCEvalTask `json:"subtasks"`
}

// PwCEvalSet is the leaderboard of a dataset of a task.
type PwCEvalSet struct {
	Dataset string `json:"dataset"`
	SOTA    struct {
		Rows []PwCEvalRow `json:"rows"`
	} `json:"sota"`
}

// PwCEvalRow is a row of a leaderboard: a model of a paper and its metrics, as strings.
type PwCEvalRow struct {
	Model      string            `json:"model_name"`
	Metrics    map[string]string `json:"metrics"`
	PaperTitle string            `json:"paper_title"`
	PaperURL   string            `json:"paper_url"`
	PaperDate  string            `json:"paper_date"`
	Extra      bool              `json:"uses_additional_data"`
}

// pwcNumberRE is a score as the dump writes it: a number, then a percent sign, an interval or other text.
var pwcNumberRE = regexp.MustCompile(`^\s*([+-]?(?:\d+\.?\d*|\.\d+)(?:[eE][+-]?\d+)?)\s*(%?)`)

// PwCScore reads a value of the dump: the number and its unit, percent when the value has a percent sign and
// number (the number as the paper prints it, on a scale that the metric gives) when not.
func PwCScore(s string) (float64, string, bool) {
	m := pwcNumberRE.FindStringSubmatch(strings.ReplaceAll(s, ",", ""))
	if m == nil {
		return 0, "", false
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, "", false
	}
	if m[2] == "%" {
		return v, "percent", true
	}
	return v, "number", true
}

// pwcPaperSrc is the source of a paper of the dump: arxiv:<id> for an arXiv paper, else its URL.
func pwcPaperSrc(u string) string {
	if ax := arxivFromURL(u); ax != "" {
		return "arxiv:" + ax
	}
	return strings.TrimSpace(u)
}

// PwCResultRows reads the evaluation tables: a result row for each metric of each row of each leaderboard. A row
// that a leaderboard of two tasks repeats is one row with both tasks as labels. It returns the rows and the counts
// of the values it skipped, by reason.
func PwCResultRows(b []byte, snap string, o Options) ([]Row, map[string]int, error) {
	var tasks []PwCEvalTask
	if err := gunzipJSON("pwc-results", b, &tasks); err != nil {
		return nil, nil, err
	}
	if len(tasks) == 0 {
		return nil, nil, &ErrSchema{"pwc-results", "the evaluation tables have no task"}
	}
	byID := map[string]*Row{}
	skipped := map[string]int{}
	var walk func([]PwCEvalTask)
	walk = func(ts []PwCEvalTask) {
		for _, t := range ts {
			task := clean(t.Task)
			for _, ds := range t.Datasets {
				dataset := clean(ds.Dataset)
				for _, er := range ds.SOTA.Rows {
					model := clean(er.Model)
					if dataset == "" || model == "" {
						skipped["rows with no dataset or no model name"] += len(er.Metrics)
						continue
					}
					if o.Since != "" && date(er.PaperDate) < o.Since {
						continue
					}
					paper := pwcPaperSrc(er.PaperURL)
					if paper == "" {
						skipped["values of rows that name no paper"] += len(er.Metrics)
						continue
					}
					for metric, val := range er.Metrics {
						metric = clean(metric)
						v, unit, ok := PwCScore(val)
						if metric == "" || !ok {
							skipped["values that are not numbers"]++
							continue
						}
						pid := strings.TrimPrefix(paper, "arxiv:")
						if strings.Contains(pid, "://") {
							pid = card.Slugify(er.PaperTitle)
						}
						id := card.Slugify(dataset) + "/" + pid + "/" + card.Slugify(model) + "/" + card.Slugify(metric)
						// Two numbers for one model, paper and metric are two runs, such as two sizes of one name:
						// the second gets a number. The same number again is the row of another task.
						base, dup := id, false
						for n := 2; byID[id] != nil; n++ {
							if prev := byID[id]; prev.Fields["score"].(map[string]any)["v"] == v {
								if task != "" && !contains(prev.Labels, task) {
									prev.Labels = append(prev.Labels, task)
								}
								dup = true
								break
							}
							id = base + "~" + strconv.Itoa(n)
						}
						if dup {
							continue
						}
						notes := "Papers with Code: no extra training data"
						if er.Extra {
							notes = "Papers with Code: uses extra training data"
						}
						row := &Row{ID: id, Name: model + " on " + dataset + " (" + metric + ")", Kind: "result",
							URL: er.PaperURL, SameAs: map[string]string{"pwc_result": id}, Snapshot: snap,
							Fields: map[string]any{
								"model":          model,
								"benchmark_name": dataset,
								"metric_name":    metric,
								"score":          map[string]any{"v": v, "unit": unit},
								"setting":        map[string]any{"notes": notes},
								"source": map[string]any{"src": paper, "kind": "primary", "date": date(er.PaperDate),
									"loc": "not located; the dump lists it under " + task + ", " + dataset + ", " + metric},
								"secondary": map[string]any{"src": "S:pwc-dump",
									"loc": PwCEvalFile + ": " + task + " / " + dataset + " / " + model + " / " + metric},
								"flags": []string{PwCUnverified},
							}}
						if t := clean(er.PaperTitle); t != "" {
							row.Fields["paper_title"] = t
						}
						if task != "" {
							row.Labels = []string{task}
						}
						byID[id] = row
					}
				}
			}
			walk(t.Subtasks)
		}
	}
	walk(tasks)
	out := make([]Row, 0, len(byID))
	for _, r := range byID {
		sort.Strings(r.Labels)
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return keep(out, o.Limit), skipped, nil
}

func contains(l []string, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}

// harvestPwCResults reads the evaluation tables of the dump. -since keeps the results of papers of that date and
// later; -limit keeps at most that many rows.
func harvestPwCResults(ctx context.Context, r *Run, o Options) ([]Row, error) {
	b, snap, err := r.Get(ctx, PwCURL(PwCEvalFile), 64<<20)
	if err != nil {
		return nil, err
	}
	rows, skipped, err := PwCResultRows(b, snap, o)
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
