// SPDX-License-Identifier: Apache-2.0

package harvest

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func helmFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestHELMRows(t *testing.T) {
	ps, err := HELMReleases(helmFixture(t, "helm-projects.json"))
	if err != nil {
		t.Fatal(err)
	}
	p := ps["finance"]
	if p.Release != "v1.0.0" || p.Title == "" || ps["lite"].Release != "v1.13.0" {
		t.Fatalf("projects %+v", ps)
	}
	if u := helmBase(p); u != "https://storage.googleapis.com/crfm-helm-public/finance/benchmark_output/releases/v1.0.0/" {
		t.Errorf("base %s", u)
	}
	day, err := HELMSummary(helmFixture(t, "helm-finance-summary.json"), p.Release)
	if err != nil || day != "2024-11-01" {
		t.Fatalf("summary %q %v", day, err)
	}
	if _, err := HELMSummary(helmFixture(t, "helm-finance-summary.json"), "v2.0.0"); err == nil {
		t.Error("the summary of another release is not an error")
	}
	models, err := HELMModels(helmFixture(t, "helm-finance-schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	if m := models["Llama 3 Instruct (70B)"]; m.Name != "meta/llama-3-70b-chat" || m.Developer != "Meta" {
		t.Errorf("model %+v", m)
	}
	g, err := HELMMainGroup(helmFixture(t, "helm-finance-groups.json"))
	if err != nil || g != "financial_scenarios" {
		t.Fatalf("group %q %v", g, err)
	}
	rows, empty, err := HELMRows(helmFixture(t, "helm-finance-main.json"), p, g, day, models, "b3:x")
	if err != nil {
		t.Fatal(err)
	}
	// 3 models by 3 scenarios; the mean win rate is of the table and gives no row.
	if len(rows) != 9 || empty != 0 {
		t.Fatalf("%d rows, %d empty", len(rows), empty)
	}
	r := rows[0]
	id := "finance/v1.0.0/financial_scenarios/FinQA - Program Accuracy/Llama 3 Instruct (70B)"
	if r.ID != id || r.Kind != "result" || r.SameAs["helm"] != id || r.Snapshot != "b3:x" {
		t.Errorf("row %+v", r)
	}
	f := r.Fields
	want := map[string]any{
		"model":          "Llama 3 Instruct (70B)",
		"benchmark_name": "HELM " + p.Title + ": FinQA",
		"metric_name":    "Program Accuracy",
		"score":          map[string]any{"v": 0.418, "unit": "fraction"},
		"developer":      "Meta",
		"setting":        map[string]any{"harness": "HELM", "notes": "HELM " + p.Title + " v1.0.0; model meta/llama-3-70b-chat"},
	}
	for k, v := range want {
		if !reflect.DeepEqual(f[k], v) {
			t.Errorf("%s is %v, want %v", k, f[k], v)
		}
	}
	src, _ := f["source"].(map[string]any)
	if src["src"] != "S:helm" || src["kind"] != "leaderboard" || src["date"] != "2024-11-01" ||
		src["loc"] != "finance v1.0.0, groups/financial_scenarios.json, table Accuracy, row Llama 3 Instruct (70B), column FinQA - Program Accuracy" {
		t.Errorf("source %v", src)
	}

	// A cell with no score is counted; a model that the schema does not name keeps its display name and no developer.
	b := []byte(`[{"title":"Accuracy","header":[{"value":"Model/adapter"},{"value":"Mean win rate"},
		{"value":"X - EM","metadata":{"metric":"EM","run_group":"X"}},{"value":"Y - BLEU-4","metadata":{"metric":"BLEU-4","run_group":"Y"}}],
		"rows":[[{"value":"New model (1B)"},{"value":0.5},{"description":"No matching runs"},{"value":12.5}],
		[{"value":"a/b"},{"value":0.5},{"value":0.1},{"value":0.2}]]}]`)
	rows, empty, err = HELMRows(b, p, "g", day, models, "s")
	if err != nil || len(rows) != 1 || empty != 4 {
		t.Fatalf("%+v %d %v", rows, empty, err)
	}
	if s := rows[0].Fields["score"]; !reflect.DeepEqual(s, map[string]any{"v": 12.5, "unit": "number"}) || rows[0].Fields["developer"] != nil {
		t.Errorf("row %+v", rows[0])
	}
}

func TestHELMUnit(t *testing.T) {
	for m, want := range map[string]string{"EM": "fraction", "F1": "fraction", "COT correct": "fraction",
		"Equivalent (CoT)": "fraction", "IFEval Strict Acc": "fraction", "EHRSQLExeAcc": "fraction", "MIMICBillingF1": "fraction",
		"Refusal Rate": "fraction", "RULER String Match": "fraction", "BBQ accuracy": "fraction",
		"WB Score": "number", "Jury Score": "number", "BLEU-4": "number", "ROUGE-L": "number", "NDCG@10": "number",
		"Helpfulness": "number", "LM Evaluated Safety score": "number"} {
		if got := HELMUnit(m); got != want {
			t.Errorf("%s: %s, want %s", m, got, want)
		}
	}
}

func TestHELMSchema(t *testing.T) {
	for name, err := range map[string]error{
		"projects not a list": func() error { _, err := HELMReleases([]byte(`{"id":"lite"}`)); return err }(),
		"projects no release": func() error { _, err := HELMReleases([]byte(`[{"id":"lite","releases":[]}]`)); return err }(),
		"schema html":         func() error { _, err := HELMModels([]byte(`<html></html>`)); return err }(),
		"schema no models":    func() error { _, err := HELMModels([]byte(`{"models":[]}`)); return err }(),
		"groups no href":      func() error { _, err := HELMMainGroup([]byte(`[{"rows":[[{"value":"All"}]]}]`)); return err }(),
		"groups bad id":       func() error { _, err := HELMMainGroup([]byte(`[{"rows":[[{"href":"?group=../x"}]]}]`)); return err }(),
		"table no model": func() error {
			_, _, err := HELMRows([]byte(`[{"header":[{"value":"Name"}]}]`), HELMProject{}, "g", "", nil, "")
			return err
		}(),
		"table not json": func() error { _, _, err := HELMRows([]byte(`<?xml?>`), HELMProject{}, "g", "", nil, ""); return err }(),
	} {
		if err == nil {
			t.Errorf("%s is not an error", name)
		}
	}
}

func TestHELMProjects(t *testing.T) {
	root := t.TempDir()
	ps, err := helmProjectList(root)
	if err != nil || !reflect.DeepEqual(ps, HELMDefault) {
		t.Errorf("default projects %v %v", ps, err)
	}
	if err := os.MkdirAll(filepath.Join(root, "crosswalk"), 0o755); err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(root, HELMProjectsFile)
	for body, want := range map[string][]string{
		"projects: [lite, long-context]\n": {"lite", "long-context"},
		"projects: []\n":                   nil,
		"projects: [../x]\n":               nil,
		"- lite\n":                         nil,
	} {
		if err := os.WriteFile(f, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		ps, err := helmProjectList(root)
		if (err != nil) != (want == nil) || !reflect.DeepEqual(ps, want) {
			t.Errorf("%q: %v %v", body, ps, err)
		}
	}
}
