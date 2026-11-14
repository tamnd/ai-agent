// SPDX-License-Identifier: Apache-2.0

package harvest

import (
	"errors"
	"net/url"
	"reflect"
	"testing"
)

func TestLMArenaConfigs(t *testing.T) {
	got, err := LMArenaConfigs([]byte(`{"splits":[{"dataset":"d","config":"webdev","split":"full"},{"config":"webdev","split":"latest"},
		{"config":"text","split":"latest"},{"config":"agent","split":"latest"}]}`))
	if err != nil || !reflect.DeepEqual(got, []string{"agent", "text", "webdev"}) {
		t.Errorf("%v %v", got, err)
	}
	var se *ErrSchema
	if _, err := LMArenaConfigs([]byte(`{"splits":[{"config":"text","split":"full"}]}`)); !errors.As(err, &se) {
		t.Errorf("no split latest: %v", err)
	}
	if !lmarenaAgent("agent") || !lmarenaAgent("agent_steerability") || lmarenaAgent("text") {
		t.Error("the Agent Arena configs are agent and agent_*")
	}
}

func TestLMArenaRows(t *testing.T) {
	u, err := url.Parse(LMArenaRows("text_style_control", 200))
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if u.Host != "datasets-server.huggingface.co" || u.Path != "/rows" || q.Get("dataset") != LMArenaDataset ||
		q.Get("config") != "text_style_control" || q.Get("split") != "latest" ||
		q.Get("offset") != "200" || q.Get("length") != "100" {
		t.Errorf("%s", u)
	}
}

func TestLMArenaRow(t *testing.T) {
	row := map[string]any{"model_name": "Model A (High)", "organization": "acme", "license": "Proprietary", "rating": 1501.25,
		"rating_lower": 1495.5, "rating_upper": 1507.0, "variance": 9.1, "vote_count": 4273.0, "rank": 3.0,
		"category": "overall", "leaderboard_publish_date": "2026-11-11"}
	r, why := LMArenaRow("text_style_control", row, "b3:1")
	if why != "" {
		t.Fatal(why)
	}
	if r.ID != "text_style_control/Model A (High)" || r.Kind != "result" || r.SameAs["lmarena"] != r.ID || r.Snapshot != "b3:1" {
		t.Errorf("the row %+v", r)
	}
	f := r.Fields
	if f["model"] != "Model A (High)" || f["benchmark_name"] != "LMArena text_style_control" || f["metric"] != "rating" ||
		f["arena"] != "LMArena text_style_control" || f["snapshot"] != "2026-11-11" || f["developer"] != "acme" {
		t.Errorf("the fields %v", f)
	}
	want := map[string]any{"v": 1501.25, "unit": "rating", "interval": []float64{1495.5, 1507.0}, "votes": 4273}
	if !reflect.DeepEqual(f["score"], want) {
		t.Errorf("the score %v", f["score"])
	}
	if s := f["setting"].(map[string]any); s["harness"] != "LMArena" || s["notes"] != "category overall; rank 3; style control" {
		t.Errorf("the setting %v", s)
	}
	src := f["source"].(map[string]any)
	if src["src"] != "S:lmarena" || src["kind"] != "leaderboard" || src["date"] != "2026-11-11" ||
		src["loc"] != "lmarena-ai/leaderboard-dataset, config text_style_control, split latest, row Model A (High), column rating" {
		t.Errorf("the source %v", src)
	}
	for _, tc := range []struct {
		key  string
		v    any
		want string
	}{
		{"category", "coding", "rows of another category than overall"},
		{"rating_upper", nil, "rows with no rating, interval or votes"},
		{"vote_count", 0.0, "rows with no rating, interval or votes"},
		{"model_name", " ", "rows with no model or no date"},
		{"leaderboard_publish_date", "", "rows with no model or no date"},
	} {
		bad := map[string]any{}
		for k, v := range row {
			bad[k] = v
		}
		bad[tc.key] = tc.v
		if _, why := LMArenaRow("text", bad, "b3:1"); why != tc.want {
			t.Errorf("%s = %v: %q, want %q", tc.key, tc.v, why, tc.want)
		}
	}
}

// A run that could not read a part of its source is partial: it removes no row of the inventory.
func TestRunPartial(t *testing.T) {
	root := t.TempDir()
	row := func(id string) Row {
		return Row{ID: id, Name: id, Kind: "result", SameAs: map[string]string{"lmarena": id}, Snapshot: "b3:1"}
	}
	if _, err := testRun(t, root, "2026-11-01").Finish([]Row{row("text/a"), row("vision/b")}, Options{}); err != nil {
		t.Fatal(err)
	}
	r := testRun(t, root, "2026-11-08")
	r.Partial = true
	rep, err := r.Finish([]Row{row("vision/b")}, Options{})
	if err != nil || !rep.Partial || rep.Removed != 0 || rep.Rows != 2 {
		t.Errorf("%+v %v", rep, err)
	}
}
