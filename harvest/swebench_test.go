// SPDX-License-Identifier: Apache-2.0

package harvest

import (
	"os"
	"reflect"
	"testing"
)

func TestSWEBench(t *testing.T) {
	b, err := os.ReadFile("testdata/swebench.json")
	if err != nil {
		t.Fatal(err)
	}
	boards, err := SWEBenchBoards(b)
	if err != nil || len(boards) != 1 || len(boards[0].Results) != 4 {
		t.Fatalf("%v %v", boards, err)
	}
	var rows []Row
	var whys []string
	for _, e := range boards[0].Results {
		row, why := SWEBenchRow(boards[0].Name, e, "s")
		if why != "" {
			whys = append(whys, why)
			continue
		}
		rows = append(rows, row)
	}
	if len(rows) != 3 || len(whys) != 1 {
		t.Fatalf("%d rows and %v, want 3 rows and the undisclosed model skipped", len(rows), whys)
	}
	trae, mini, sonar := rows[0], rows[1], rows[2]
	if trae.ID != "verified/20250928_trae_doubao_seed_code" || trae.Kind != "result" || trae.Fields["model"] != "Doubao-Seed-Code" ||
		trae.Fields["benchmark"] != "swe-bench" || trae.Fields["version"] != "verified" || trae.Fields["metric"] != SWEBenchMetric {
		t.Errorf("the row of TRAE %+v", trae)
	}
	if sc := trae.Fields["score"].(map[string]any); sc["v"] != 78.8 || sc["unit"] != "percent" {
		t.Errorf("the score %v", sc)
	}
	set := trae.Fields["setting"].(map[string]any)
	if set["scaffold"] != "TRAE" || set["compute"] != "attempts: 2+" || set["split"] != "test" || set["date_run"] != "2025-09-28" ||
		set["notes"] != "the system uses 2 models: Doubao-Seed-Code, Doubao-Seed-1.6" {
		t.Errorf("the setting of TRAE %v", set)
	}
	if trae.Fields["self_reported"] != true || trae.Fields["submitted_by"] != "ByteDance" {
		t.Errorf("ByteDance reports its own model: %v", trae.Fields)
	}
	if !reflect.DeepEqual(trae.Links["model"], []string{"Doubao-Seed-Code", "Doubao-Seed-1.6"}) {
		t.Errorf("the models %v", trae.Links)
	}
	ms := mini.Fields["setting"].(map[string]any)
	if ms["scaffold"] != "mini-SWE-agent@1.16.0" || ms["cot"] != "reasoning-model (effort medium)" ||
		mini.Fields["submitted_by"] != "SWE-agent" || mini.Fields["self_reported"] != nil || mini.Fields["checked"] != true {
		t.Errorf("a run of mini-SWE-agent by the maintainers is not self-reported: %v %v", ms, mini.Fields)
	}
	if sonar.Fields["self_reported"] != nil || sonar.Fields["submitted_by"] != "Sonar" {
		t.Errorf("Sonar does not develop Claude: %v", sonar.Fields)
	}
	src := sonar.Fields["source"].(map[string]any)
	if src["src"] != "S:swe-bench-leaderboard" || src["date"] != "2025-12-05" ||
		src["loc"] != "Verified leaderboard, entry 20251205_sonar-foundation-agent_claude-opus-4-5, field resolved" {
		t.Errorf("the source %v", src)
	}
	e := boards[0].Results[2]
	e.Checked = "false (See README.md for info on how to get your results verified)"
	if r, _ := SWEBenchRow("Verified", e, "s"); r.Fields["checked"] != false {
		t.Errorf("a string that says false is not checked: %v", r.Fields["checked"])
	}
	if _, err := SWEBenchBoards([]byte(`{"leaderboards":[]}`)); err == nil {
		t.Error("no leaderboards is not an error")
	}
}
