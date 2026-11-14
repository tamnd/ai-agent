// SPDX-License-Identifier: Apache-2.0

package harvest

import (
	"encoding/json"
	"testing"
)

const openLLMPage = `{"rows":[
 {"row_idx":0,"row":{"eval_name":"0-hero_Matter-0.2-7B-DPO_bfloat16","Precision":"bfloat16","Type":"💬 chat models (RLHF, DPO, IFT, ...)",
  "fullname":"0-hero/Matter-0.2-7B-DPO","Model sha":"26a66f0d","Merged":false,"Flagged":false,"Chat Template":true,
  "IFEval Raw":0.3302792147058693,"IFEval":33.02792147058693,"BBH Raw":0.3596254301656297,"BBH":10.055525080241035,
  "MATH Lvl 5 Raw":0.014350453172205438,"MATH Lvl 5":1.4350453172205437,"GPQA Raw":0.25922818791946306,"GPQA":1.230425055928408,
  "MUSR Raw":0.381375,"MUSR":5.871874999999999,"MMLU-PRO Raw":0.1163563829787234,"MMLU-PRO":1.8173758865248217,
  "Submission Date":"2024-08-05","Base Model":"0-hero/Matter-0.2-7B-DPO"}},
 {"row_idx":1,"row":{"eval_name":"x_merge_4bit","Precision":"4bit","Type":"🤝 base merges and moerges","fullname":"x/merge",
  "Merged":true,"Flagged":true,"Chat Template":false,"GPQA Raw":0.3,"GPQA":6.7,"Submission Date":"2024-09-01","Base Model":"y/base"}},
 {"row_idx":2,"row":{"eval_name":"","fullname":"z/none"}},
 {"row_idx":3,"row":{"eval_name":"z_none_float16","fullname":"z/none","Precision":"float16"}}],
 "num_rows_total":4,"num_rows_per_page":100,"partial":false}`

func TestOpenLLMResultRows(t *testing.T) {
	var p OpenLLMPageData
	if err := json.Unmarshal([]byte(openLLMPage), &p); err != nil {
		t.Fatal(err)
	}
	if p.Total != 4 || len(p.Rows) != 4 {
		t.Fatalf("page %+v", p)
	}
	rows, why := OpenLLMResultRows(p.Rows[0].Row, "b3:snap")
	if why != "" || len(rows) != 6 {
		t.Fatalf("%d rows (%s), want 6", len(rows), why)
	}
	by := map[string]Row{}
	for _, r := range rows {
		by[r.ID] = r
	}
	g := by["0-hero_Matter-0.2-7B-DPO_bfloat16/gpqa"]
	f, set, src := g.Fields, g.Fields["setting"].(map[string]any), g.Fields["source"].(map[string]any)
	if f["model"] != "0-hero/Matter-0.2-7B-DPO" || f["benchmark_name"] != "Open LLM Leaderboard v2: GPQA" ||
		f["metric_name"] != "Open LLM Leaderboard v2: GPQA raw score" {
		t.Errorf("fields %v", f)
	}
	if sc := f["score"].(map[string]any); sc["v"] != 0.25922818791946306 || sc["unit"] != "fraction" {
		t.Errorf("score %v", sc)
	}
	if set["shots"] != 0 || set["quantization"] != "none (bfloat16)" ||
		set["notes"] != "normalized score 1.23; type: chat models (RLHF, DPO, IFT, ...); revision 26a66f0d; chat template" {
		t.Errorf("setting %v", set)
	}
	if src["src"] != "S:open-llm-leaderboard" || src["date"] != "2024-08-05" ||
		src["loc"] != "open-llm-leaderboard/contents, row 0-hero_Matter-0.2-7B-DPO_bfloat16, column GPQA Raw" {
		t.Errorf("source %v", src)
	}
	if g.Links != nil || g.Fields["warning"] != nil {
		t.Errorf("links %v, warning %v: the base model is the model itself and the row is not flagged", g.Links, g.Fields["warning"])
	}
	if s := by["0-hero_Matter-0.2-7B-DPO_bfloat16/bbh"].Fields["setting"].(map[string]any)["shots"]; s != 3 {
		t.Errorf("bbh shots %v", s)
	}
	if _, ok := by["0-hero_Matter-0.2-7B-DPO_bfloat16/math-lvl-5"]; !ok {
		t.Error("no row for MATH Lvl 5")
	}
	m, _ := OpenLLMResultRows(p.Rows[1].Row, "b3:snap")
	if len(m) != 1 || m[0].Fields["warning"] != "flagged on the leaderboard" || m[0].Links["base_model"][0] != "y/base" ||
		m[0].Fields["setting"].(map[string]any)["quantization"] != "4bit" {
		t.Errorf("merge rows %+v", m)
	}
	if _, why := OpenLLMResultRows(p.Rows[2].Row, "b3:snap"); why != "rows with no eval_name or no model" {
		t.Errorf("row with no eval_name: %q", why)
	}
	if _, why := OpenLLMResultRows(p.Rows[3].Row, "b3:snap"); why != "rows with no score" {
		t.Errorf("row with no score: %q", why)
	}
}
