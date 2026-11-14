// SPDX-License-Identifier: Apache-2.0

package graph

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/tamnd/ai-agent/node"
)

// corpus is the test corpus with a node that has a map edge and a bridge prerequisite, models with a lineage cycle,
// a benchmark and a result card.
func corpus(t *testing.T) *node.Corpus {
	t.Helper()
	root := filepath.Join(t.TempDir(), "corpus")
	if err := os.CopyFS(root, os.DirFS("../testdata/corpus")); err != nil {
		t.Fatal(err)
	}
	md := "---\ntag: 7TQYMD\nkind: method\nslug: toy-search\nname: Toy search\narea: searching/sequential-and-binary-search\nlevel: core\nsummary: A toy.\n" +
		"prereqs: [binary-search, matrix-multiplication@m2, no-such-node]\nrelated_to: [loop-invariant]\n---\n\n## Overview\n\nA toy.\n"
	files := map[string]string{
		"nodes/searching/sequential-and-binary-search/toy-search/node.md": md,
		"cards/bridge/m2/_.jsonl": `{"tag":"M2MM01","site":"m2","kind":"theorem","slug":"matrix-multiplication","name":"Matrix multiplication","url":"https://math2.tamnd.com/matrix-multiplication/"}` + "\n",
		"cards/benchmark/_.jsonl": `{"tag":"9MMLU0","kind":"benchmark","slug":"mmlu","name":"MMLU","versions":[{"id":"original-test","name":"MMLU, test set"}]}` + "\n",
		"cards/model/_.jsonl": `{"tag":"9MA000","kind":"model","slug":"model-a","name":"Model A","fine_tuned_from":"model-c"}` + "\n" +
			`{"tag":"9MB000","kind":"model","slug":"model-b","name":"Model B","fine_tuned_from":"model-a"}` + "\n" +
			`{"tag":"9MC000","kind":"model","slug":"model-c","name":"Model C","fine_tuned_from":"model-b","distilled_from":"secret-model"}` + "\n",
		"cards/result/_.jsonl": `{"tag":"9R0001","kind":"result","model":"model-a","benchmark":"mmlu","version":"original-test","metric":"accuracy","score":{"v":0.886,"unit":"fraction"},"source":{"src":"S:paper-a","loc":"Table 2","date":"2024-07-23"}}` + "\n" +
			`{"tag":"9R0002","kind":"result","model":"model-a","benchmark":"mmlu","version":"original-test","metric":"accuracy","score":{"v":0.9,"unit":"fraction"},"source":{"src":"S:paper-b","loc":"Table 1","date":"2024-08-01"},"evidence_for":[{"slug":"comparison-model","claim":"c-fast","role":"replication"}],"used_by":["binary-search"]}` + "\n",
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

func find(es []Edge, typ, to string) (Edge, bool) {
	for _, e := range es {
		if e.Type == typ && e.To == to {
			return e, true
		}
	}
	return Edge{}, false
}

func TestBuild(t *testing.T) {
	g := Build(corpus(t))
	out := g.Out("toy-search")
	for _, w := range []struct {
		typ, to string
		known   bool
		site    string
	}{
		{"prereqs", "binary-search", true, ""},
		{"prereqs", "matrix-multiplication@m2", true, "m2"},
		{"prereqs", "no-such-node", false, ""},
		{"related_to", "loop-invariant", true, ""},
	} {
		e, ok := find(out, w.typ, w.to)
		if !ok {
			t.Errorf("no %s edge to %s in %+v", w.typ, w.to, out)
			continue
		}
		if e.Known != w.known || e.Site != w.site {
			t.Errorf("%s %s: known %v site %q", w.typ, w.to, e.Known, e.Site)
		}
	}
	if e, _ := find(g.Out("mmlu/9r0002"), "evidence_for", "comparison-model"); e.Claim != "c-fast" || e.Role != "replication" || !e.Known {
		t.Errorf("evidence_for keeps no claim and role: %+v", e)
	}
	if _, ok := find(g.Out("mmlu/9r0002"), "used_by", "binary-search"); ok {
		t.Error("a reverse field written by hand became an edge")
	}
	if e, ok := find(g.Out("model-a"), "evaluated_on", "mmlu@original-test"); !ok || !e.Derived || !e.Known {
		t.Errorf("evaluated_on of model-a: %+v %v", e, ok)
	} else if n := len(g.In("mmlu@original-test")); n != 1 {
		t.Errorf("two results of one model on one version give %d edges", n)
	}
	if e, ok := find(g.Out("model-c"), "distilled_from", "secret-model"); !ok || e.Known {
		t.Errorf("an unknown lineage end: %+v %v", e, ok)
	}
	if in := g.In("binary-search"); len(in) == 0 || in[0].From != "toy-search" {
		t.Errorf("in edges of binary-search: %+v", in)
	}
	var rev []Edge
	for _, e := range g.Reversed() {
		if e.From == "binary-search" {
			rev = append(rev, e)
		}
	}
	if len(rev) != 1 || rev[0].Type != "needed_by" || rev[0].To != "toy-search" {
		t.Errorf("reverse edges of binary-search: %+v", rev)
	}
}

func TestPathClosureCycles(t *testing.T) {
	g := Build(corpus(t))
	want := []string{"loop-invariant", "asymptotic-notation", "binary-search", "matrix-multiplication@m2", "toy-search"}
	if p := g.Path("toy-search"); !slices.Equal(p, want) {
		t.Errorf("path %v, want %v", p, want)
	}
	if cl := g.Closure("toy-search", false, "prereqs"); !slices.Contains(cl, "loop-invariant") || slices.Contains(cl, "toy-search") {
		t.Errorf("closure %v", cl)
	}
	if cl := g.Closure("loop-invariant", true, "prereqs"); !slices.Equal(cl, []string{"binary-search", "toy-search"}) {
		t.Errorf("closure back %v", cl)
	}
	cy := g.Cycles("fine_tuned_from")
	if len(cy) != 1 || !slices.Equal(cy[0], []string{"model-a", "model-c", "model-b", "model-a"}) {
		t.Errorf("cycles %v", cy)
	}
	if cy := g.Cycles("prereqs"); len(cy) != 0 {
		t.Errorf("prereq cycles %v", cy)
	}
}

func TestWrite(t *testing.T) {
	g := Build(corpus(t))
	dir := t.TempDir()
	s, err := g.Write(dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.Edges != g.Len() || s.Neighbor != 1 || s.ByType["evaluated_on"] != 1 || len(s.Cycles) != 1 {
		t.Errorf("summary %+v", s)
	}
	f, err := os.Open(filepath.Join(dir, "edges.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	n := 0
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var e Edge
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			t.Fatal(err)
		}
		n++
	}
	if n != g.Len() {
		t.Errorf("edges.jsonl has %d lines, want %d", n, g.Len())
	}
	if _, err := os.Stat(filepath.Join(dir, "reverse.jsonl")); err != nil {
		t.Error(err)
	}
}
