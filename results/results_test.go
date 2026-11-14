// SPDX-License-Identifier: Apache-2.0

package results

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/ai-agent/node"
)

// Cards is a small results register: a benchmark with two versions, a metric, three models, and their results.
const Cards = `{"tag":"9MMLU0","kind":"benchmark","slug":"mmlu","name":"MMLU","versions":[{"id":"original-test","name":"MMLU, test set","metric":"accuracy"},{"id":"pro","name":"MMLU-Pro"}]}
`

const metricCards = `{"tag":"9ACC00","kind":"metric","slug":"accuracy","name":"Accuracy"}
`

const modelCards = `{"tag":"9MA000","kind":"model","slug":"model-a","name":"Model A"}
{"tag":"9MB000","kind":"model","slug":"model-b","name":"Model B"}
{"tag":"9MC000","kind":"model","slug":"model-c","name":"Model C"}
`

const resultCards = `{"tag":"9R0001","kind":"result","model":"model-a","model_variant":"instruct","benchmark":"mmlu","version":"original-test","metric":"accuracy","score":{"v":0.886,"unit":"fraction"},"setting":{"shots":5,"prompt":"custom","cot":"none","decoding":"greedy","split":"test"},"source":{"src":"S:paper-a","loc":"Table 2","kind":"primary","date":"2024-07-23"},"self_reported":true,"checks":{"cite":"found"}}
{"tag":"9R0002","kind":"result","model":"model-a","model_variant":"instruct","benchmark":"mmlu","version":"original-test","metric":"accuracy","score":{"v":0.881,"unit":"fraction"},"setting":{"shots":5,"prompt":"custom","cot":"none","decoding":"greedy","split":"test"},"source":{"src":"S:epoch","loc":"row 12","kind":"secondary","date":"2024-09-01"},"checks":{"cite":"found"}}
{"tag":"9R0003","kind":"result","model":"model-b","benchmark":"mmlu","version":"original-test","metric":"accuracy","score":{"v":0.79,"unit":"fraction"},"setting":{"shots":5,"prompt":"custom","cot":"none","decoding":"greedy","split":"test"},"source":{"src":"S:paper-b","loc":"Table 1","kind":"primary","date":"2024-01-02"},"checks":{"cite":"found"}}
{"tag":"9R0004","kind":"result","model":"model-b","benchmark":"mmlu","version":"original-test","metric":"accuracy","score":{"v":0.70,"unit":"fraction"},"setting":{"shots":5,"prompt":"custom","cot":"none","decoding":"greedy","split":"test"},"source":{"src":"S:other","loc":"Table 4","kind":"secondary","date":"2024-03-02"},"checks":{"cite":"found"}}
{"tag":"9R0005","kind":"result","model":"model-c","benchmark":"mmlu","version":"original-test","metric":"accuracy","score":{"v":0.65,"unit":"fraction"},"setting":{"shots":0,"prompt":"unknown","cot":"none","split":"test"},"source":{"src":"S:paper-c","loc":"Table 3","kind":"primary","date":"2023-05-02"},"checks":{"cite":"found"}}
{"tag":"9R0006","kind":"result","model":"model-c","benchmark":"mmlu","version":"original-test","metric":"accuracy","score":{"v":0.99,"unit":"fraction"},"setting":{"shots":5,"prompt":"custom","cot":"none","decoding":"greedy","split":"test"},"source":{"src":"S:blog","loc":"Figure 1","kind":"secondary","date":"2023-05-02"},"checks":{"cite":"not-found"}}
{"tag":"9R0007","kind":"result","model":"model-c","benchmark":"mmlu","version":"original-test","metric":"accuracy","score":{"v":0.61,"unit":"fraction"},"setting":{"shots":5,"prompt":"custom","cot":"none","decoding":"greedy","split":"test"},"source":{"src":"exp:mmlu/harness-c","loc":"results.json","kind":"harness","date":"2026-11-13"},"checks":{"cite":"found"}}
`

// Corpus writes the register into a copy of the test corpus and loads it.
func Corpus(t *testing.T, extra map[string]string) *node.Corpus {
	t.Helper()
	root := filepath.Join(t.TempDir(), "corpus")
	if err := os.CopyFS(root, os.DirFS("../testdata/corpus")); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{"cards/benchmark/_.jsonl": Cards, "cards/metric/_.jsonl": metricCards, "cards/model/_.jsonl": modelCards, "cards/result/_.jsonl": resultCards}
	for k, v := range extra {
		files[k] = v
	}
	for p, s := range files {
		full := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	c, err := node.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestLoad(t *testing.T) {
	reg := Load(Corpus(t, nil))
	if len(reg.Results) != 7 || len(reg.Versions["mmlu"]) != 2 || reg.Versions["mmlu"]["pro"].Name != "MMLU-Pro" {
		t.Fatalf("the register has %d results and the versions %v", len(reg.Results), reg.Versions)
	}
	by := map[string]*Result{}
	for _, r := range reg.Results {
		if r.Err != nil {
			t.Errorf("%s: %v", r.Tag, r.Err)
		}
		by[r.Tag] = r
	}
	if c := by["9R0001"].Card; c.Owner != "mmlu" || c.Slug != "9r0001" || c.Name != "model-a on mmlu original-test" || len(c.Facts) != 0 {
		t.Errorf("the card of a result is %+v", c)
	}
	if g := by["9R0001"].Group; g != "mmlu@original-test/accuracy/shots=5,prompt=custom,cot=none,decoding=greedy,split=test,model_variant=instruct" {
		t.Errorf("the group is %q", g)
	}
	if g, u := by["9R0005"].Group, by["9R0005"].Unknown; !strings.HasSuffix(g, "/unknown:9R0005") || len(u) != 1 || u[0] != "prompt" {
		t.Errorf("a result with an unknown hard key has the group %q and %v", g, u)
	}
	for tag, want := range map[string]string{"9R0001": Replicated, "9R0002": Replicated, "9R0003": Contested, "9R0004": Contested, "9R0005": Reported, "9R0007": MeasuredHere} {
		if got := by[tag].Evidence; got != want {
			t.Errorf("%s has the evidence %s, want %s", tag, got, want)
		}
	}
}

func TestTable(t *testing.T) {
	reg := Load(Corpus(t, nil))
	tb := reg.TableOf("mmlu", "original-test", nil)
	if tb.Count != 6 || tb.Hidden != 1 || len(tb.Other) != 1 || len(tb.Groups) != 2 {
		t.Fatalf("the table is %+v", tb)
	}
	var tags []string
	for _, g := range tb.Groups {
		if g.Setting == "" || g.Metric != "accuracy" {
			t.Errorf("the group %+v has no setting or no metric", g)
		}
		for _, r := range g.Rows {
			tags = append(tags, r.Tag)
		}
	}
	if got := strings.Join(tags, " "); got != "9R0003 9R0004 9R0007 9R0001 9R0002" {
		t.Errorf("the rows are %s", got)
	}
	if low := reg.TableOf("mmlu", "original-test", map[string]bool{"accuracy": true}); low.Groups[0].Rows[0].Tag != "9R0007" {
		t.Errorf("a metric where lower is better puts %s first", low.Groups[0].Rows[0].Tag)
	}
}

func TestReleasesFile(t *testing.T) {
	md := "---\ntag: 9SQD00\nkind: benchmark\nslug: squad\nname: SQuAD\narea: foundations/models-of-computation\nlevel: intro\nsummary: A benchmark.\n---\n\n## Overview\n\nA benchmark.\n"
	dir := "nodes/foundations/models-of-computation/squad/"
	reg := Load(Corpus(t, map[string]string{dir + "node.md": md, dir + ReleasesFile: "- {id: v1-1-dev, name: \"SQuAD 1.1, dev set\", metric: f1}\n"}))
	if v := reg.Versions["squad"]["v1-1-dev"]; v == nil || v.Metric != "f1" {
		t.Errorf("the versions of a benchmark node are %v", reg.Versions["squad"])
	}
	bad := Load(Corpus(t, map[string]string{dir + "node.md": md, dir + ReleasesFile: "id: not-a-list\n"}))
	if len(bad.VersionErrs) != 1 {
		t.Errorf("a releases.yaml that is not a list gives %v", bad.VersionErrs)
	}
}

func TestTolerance(t *testing.T) {
	for _, c := range []struct {
		unit string
		v    float64
		want float64
	}{{"fraction", 0.5, 0.01}, {"percent", 50, 1}, {"points", 30, 1}, {"rating", 1200, 12}} {
		if got := Tolerance(c.unit, c.v); got != c.want {
			t.Errorf("Tolerance(%s, %g) = %g, want %g", c.unit, c.v, got, c.want)
		}
	}
}

// A rating is comparable only inside one snapshot of one arena (10.10).
func TestGroupOfRating(t *testing.T) {
	r := func(arena, snap string) *Result {
		return &Result{Tag: "T", Benchmark: "lmarena", Version: "text", Metric: "rating", Arena: arena, Snapshot: snap,
			Setting: map[string]any{"harness": "LMArena"}}
	}
	a, _ := GroupOf(r("LMArena text", "2026-11-11"))
	b, _ := GroupOf(r("LMArena text", "2026-11-04"))
	c, _ := GroupOf(r("LMArena vision", "2026-11-11"))
	same, _ := GroupOf(r("LMArena text", "2026-11-11"))
	if a == b || a == c || a != same {
		t.Errorf("groups %q %q %q %q", a, b, c, same)
	}
	acc, _ := GroupOf(&Result{Benchmark: "mmlu", Version: "test", Metric: "accuracy", Arena: "x", Snapshot: "2026-11-11"})
	if acc != "mmlu@test/accuracy/" {
		t.Errorf("a result that is not a rating ignores arena and snapshot: %q", acc)
	}
}
