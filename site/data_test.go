// SPDX-License-Identifier: Apache-2.0

package site

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/parquet-go/parquet-go"

	"github.com/tamnd/ai-agent/exp"
	"github.com/tamnd/ai-agent/node"
	"github.com/tamnd/ai-agent/prov"
)

// TestDatasets checks the datasets of 16.10 that a build writes with Options.Data.
func TestDatasets(t *testing.T) {
	c, err := node.Load("../testdata/corpus")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	data := filepath.Join(dir, "data")
	if err := os.MkdirAll(filepath.Join(data, "stale"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(c, Options{Out: filepath.Join(dir, "site"), Data: data, Commit: "0123456789abcdef", Version: "test", Now: time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(data, "stale")); err == nil {
		t.Error("the build did not replace the old datasets")
	}
	b, err := os.ReadFile(filepath.Join(data, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if m.AsOf != "2026-11-03" || m.Commit != "0123456789abcdef" || len(m.NotBuilt) != 0 {
		t.Errorf("manifest = %+v", m)
	}
	files := map[string]DataFile{}
	for _, d := range m.Datasets {
		if d.Licence == "" {
			t.Errorf("%s has no licence", d.Name)
		}
		for _, f := range d.Files {
			files[f.Path] = f
			raw, err := os.ReadFile(filepath.Join(data, filepath.FromSlash(f.Path)))
			if err != nil {
				t.Fatal(err)
			}
			sum := sha256.Sum256(raw)
			if hex.EncodeToString(sum[:]) != f.SHA256 || len(raw) != f.Bytes || strings.HasSuffix(f.Path, ".jsonl") && bytes.Count(raw, []byte("\n")) != f.Rows {
				t.Errorf("%s: the manifest does not match the file", f.Path)
			}
		}
	}
	n, ok := c.BySlug["binary-search"]
	if !ok {
		t.Fatal("no binary-search in the test corpus")
	}
	items := "items/" + n.Kind + ".jsonl"
	if files[items].Rows == 0 {
		t.Fatalf("%s is missing or empty: %v", items, files)
	}
	found := false
	each(t, filepath.Join(data, items), func(line []byte) {
		var r Record
		if err := json.Unmarshal(line, &r); err != nil {
			t.Fatal(err)
		}
		if r.Key == "binary-search" {
			found = true
			if r.Status == nil || r.URL != "https://ai.tamnd.com/binary-search/" {
				t.Errorf("the record of binary-search: %+v", r.Head)
			}
		}
	})
	if !found {
		t.Error("binary-search is not in the items")
	}
	solves := false
	each(t, filepath.Join(data, "graph.jsonl"), func(line []byte) {
		var e Edge
		if err := json.Unmarshal(line, &e); err != nil {
			t.Fatal(err)
		}
		solves = solves || e.From == "binary-search" && e.Type == "solves" && e.To == "search-in-sorted-array"
	})
	if !solves {
		t.Error("graph.jsonl does not have the solves edge of binary-search")
	}
	if want := len(n.Claims); want > 0 && files["claims.jsonl"].Rows < want {
		t.Errorf("claims.jsonl has %d rows, binary-search alone has %d claims", files["claims.jsonl"].Rows, want)
	}
}

func readParquet[T any](t *testing.T, b []byte) []T {
	t.Helper()
	rows, err := parquet.Read[T](bytes.NewReader(b), int64(len(b)))
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

func each(t *testing.T, file string, fn func([]byte)) {
	t.Helper()
	fh, err := os.Open(file)
	if err != nil {
		t.Fatal(err)
	}
	defer fh.Close()
	sc := bufio.NewScanner(fh)
	sc.Buffer(make([]byte, 1<<20), 16<<20)
	for sc.Scan() {
		fn(sc.Bytes())
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
}

// TestDatasetForms checks the forms of 16.10 besides the items: the CSV of the models and the results, the GraphML of
// the graph, the incidents, the policy, the experiments without the ones the quarantine hides, the code with the
// tarball of a framework, and the pages.
func TestDatasetForms(t *testing.T) {
	root := filepath.Join(t.TempDir(), "corpus")
	if err := os.CopyFS(root, os.DirFS("../testdata/corpus")); err != nil {
		t.Fatal(err)
	}
	toy := filepath.Join("nodes", "foundations", "models-of-computation", "toy-model")
	bs := filepath.Join("nodes", "searching", "sequential-and-binary-search", "binary-search")
	design := "id: %s\nclaims: [c-fits]\nquestion: Does the toy fit?\narms:\n  - {id: none, config: {p: 0.0}}\n  - {id: dropout, config: {p: 0.5}}\ndata:\n  - {generator: moons.py}\nseeds: {n: 5, base: 1}\nbudget: cpu-small\nmetrics: [test_loss]\n"
	files := map[string]string{
		"cards/model/_.jsonl": `{"tag":"9MA000","kind":"model","slug":"model-a","name":"Model A","role":"checkpoint","developer":{"v":"lab-a","src":"S:paper-a"},"params":{"total":{"v":7000000000,"src":"S:paper-a","loc":"Table 1"}}}` + "\n" +
			`{"tag":"9MB000","kind":"model","slug":"model-b","name":"Model B, \"large\"","fine_tuned_from":"model-a"}` + "\n",
		"cards/work/_.jsonl":              `{"tag":"9WK000","kind":"work","slug":"attention-is-all-you-need","name":"Attention Is All You Need","type":"paper","year":2017,"venue":"NeurIPS","authors":["ashish-vaswani","noam-shazeer"],"same_as":{"arxiv":"1706.03762","doi":"10.48550/arXiv.1706.03762"}}` + "\n",
		"cards/incident/_.jsonl":          `{"tag":"9IN000","kind":"incident","slug":"incident-a","name":"Incident A"}` + "\n",
		"cards/policy/_.jsonl":            `{"tag":"9PO000","kind":"policy","slug":"policy-a","name":"Policy A"}` + "\n",
		"cards/provision/_.jsonl":         `{"tag":"9PV000","kind":"provision","slug":"provision-a","name":"Provision A","policy":"policy-a"}` + "\n",
		"cards/result/_.jsonl":            `{"tag":"9R0001","kind":"result","model":"model-a","benchmark":"mmlu","version":"original-test","metric":"accuracy","score":{"v":0.886,"unit":"fraction","interval":[0.88,0.89]},"setting":{"shots":5},"source":{"src":"S:paper-a","loc":"Table 2","date":"2024-07-23"},"checks":{"cite":"found"}}` + "\n",
		filepath.Join(toy, "node.md"):     "---\ntag: 7TQYMD\nkind: model\nslug: toy-model\nname: Toy model\narea: foundations/models-of-computation\nlevel: intro\nsummary: A small model.\n---\n\n## Overview\n\nThe toy model fits the data {#c-fits}.\n",
		filepath.Join(toy, "claims.yaml"): "- id: c-fits\n  type: empirical\n  text: The toy model fits the data.\n  small_scale: yes\n  evidence:\n    - {work: paper, role: original, stance: for}\n",
		filepath.Join("nodes", "foundations", "models-of-computation", "turing-1936", "node.md"): "---\ntag: 7TUR36\nkind: work\nslug: turing-1936\nname: On Computable Numbers\narea: foundations/models-of-computation\nlevel: core\nsummary: The paper that defined the Turing machine.\nsame_as: {wikidata: Q1129575, doi: [\"10.1112/plms/s2-42.1.230\"]}\ntype: paper\nauthors: [Alan Turing]\nyear: 1936\nvenue: Proceedings of the London Mathematical Society\n---\n\nThe paper defines the machine.\n",
		filepath.Join(bs, "impl", "numpy", "binary_search.py"):                                   "import numpy as np\n\n\ndef binary_search(a, x):\n    i = int(np.searchsorted(a, x))\n    return i if i < len(a) and a[i] == x else -1\n",
	}
	for _, id := range []string{"toy-fit", "hidden"} {
		ed := filepath.Join(toy, exp.Dir, id)
		files[filepath.Join(ed, exp.DesignFile)] = fmt.Sprintf(design, id)
		files[filepath.Join(ed, exp.Script)] = "\n"
		files[filepath.Join(ed, "moons.py")] = "\n"
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
	if err := prov.Append(root, "7TQYMD", []prov.Line{{Date: "2026-11-15", File: "exp/hidden/experiment.yaml", Op: prov.Quarantine, Author: prov.Author{Login: "tamnd"}}}); err != nil {
		t.Fatal(err)
	}
	c, err := node.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	data := filepath.Join(t.TempDir(), "data")
	if _, err := Build(c, Options{Out: filepath.Join(t.TempDir(), "site"), Data: data}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(data, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	files2 := map[string]DataFile{}
	for _, d := range m.Datasets {
		for _, f := range d.Files {
			files2[f.Path] = f
		}
	}
	read := func(name string) []byte {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(data, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	table := func(name string) []map[string]string {
		t.Helper()
		rows, err := csv.NewReader(bytes.NewReader(read(name))).ReadAll()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(rows)-1 != files2[name].Rows {
			t.Errorf("%s has %d rows, the manifest says %d", name, len(rows)-1, files2[name].Rows)
		}
		var out []map[string]string
		for _, r := range rows[1:] {
			row := map[string]string{}
			for i, h := range rows[0] {
				row[h] = r[i]
			}
			out = append(out, row)
		}
		return out
	}

	models := table("models.csv")
	byKey := map[string]map[string]string{}
	for _, r := range models {
		byKey[r["key"]] = r
	}
	if a := byKey["model-a"]; a["params.total"] != "7000000000" || a["params.total.src"] != "S:paper-a" || a["developer"] != "lab-a" || a["role"] != "checkpoint" {
		t.Errorf("the row of model-a: %v", a)
	}
	if bm := byKey["model-b"]; bm["name"] != `Model B, "large"` || bm["params.total"] != "" {
		t.Errorf("the row of model-b: %v", bm)
	}
	if byKey["toy-model"] == nil {
		t.Error("models.csv does not have the model node toy-model")
	}

	res := table("results.csv")
	if len(res) != 1 || res[0]["score"] != "0.886" || res[0]["interval_low"] != "0.88" || res[0]["setting"] != `{"shots":5}` || res[0]["source"] != "S:paper-a" {
		t.Errorf("results.csv = %v", res)
	}

	var gml struct {
		Graph struct {
			Nodes []struct {
				ID   string `xml:"id,attr"`
				Data []struct {
					Key string `xml:"key,attr"`
					V   string `xml:",chardata"`
				} `xml:"data"`
			} `xml:"node"`
			Edges []struct {
				Source string `xml:"source,attr"`
				Target string `xml:"target,attr"`
				Data   string `xml:"data"`
			} `xml:"edge"`
		} `xml:"graph"`
	}
	if err := xml.Unmarshal(read("graph.graphml"), &gml); err != nil {
		t.Fatal(err)
	}
	if len(gml.Graph.Edges) != files2["graph.jsonl"].Rows || len(gml.Graph.Edges) != files2["graph.graphml"].Rows {
		t.Errorf("graph.graphml has %d edges, graph.jsonl %d", len(gml.Graph.Edges), files2["graph.jsonl"].Rows)
	}
	ids := map[string]bool{}
	for _, n := range gml.Graph.Nodes {
		ids[n.ID] = true
	}
	tuned := false
	for _, e := range gml.Graph.Edges {
		if !ids[e.Source] || !ids[e.Target] {
			t.Errorf("the edge %s -> %s has an end that is not a node", e.Source, e.Target)
		}
		tuned = tuned || e.Source == "model-b" && e.Target == "model-a" && e.Data == "fine_tuned_from"
	}
	if !tuned || !ids["binary-search"] {
		t.Error("graph.graphml misses the edge model-b fine_tuned_from model-a or the node binary-search")
	}

	kinds := func(name string) map[string]int {
		out := map[string]int{}
		each(t, filepath.Join(data, name), func(line []byte) {
			var r Record
			if err := json.Unmarshal(line, &r); err != nil {
				t.Fatal(err)
			}
			out[r.Kind]++
		})
		return out
	}
	if k := kinds("incidents.jsonl"); len(k) != 1 || k["incident"] != 1 {
		t.Errorf("incidents.jsonl has %v", k)
	}
	if k := kinds("policy.jsonl"); len(k) != 2 || k["policy"] != 1 || k["provision"] != 1 {
		t.Errorf("policy.jsonl has %v", k)
	}

	var exps []ExperimentLine
	each(t, filepath.Join(data, "experiments.jsonl"), func(line []byte) {
		var l ExperimentLine
		if err := json.Unmarshal(line, &l); err != nil {
			t.Fatal(err)
		}
		exps = append(exps, l)
	})
	if len(exps) != 1 || exps[0].ID != "toy-fit" || exps[0].Item != "toy-model" {
		t.Fatalf("experiments.jsonl = %+v", exps)
	}
	if d, ok := exps[0].Design.(map[string]any); !ok || d["id"] != "toy-fit" || d["budget"] != "cpu-small" {
		t.Errorf("the design of toy-fit is %v", exps[0].Design)
	}

	ladders := map[string]string{}
	each(t, filepath.Join(data, "code.jsonl"), func(line []byte) {
		var l CodeLine
		if err := json.Unmarshal(line, &l); err != nil {
			t.Fatal(err)
		}
		if l.Item == "binary-search" {
			ladders[l.Lang] = l.Ladder
			if len(l.Files) == 0 || l.Files[0].Content == "" {
				t.Errorf("the %s code of binary-search has no content", l.Lang)
			}
		}
	})
	if ladders["numpy"] != "X" || ladders["go"] != "I" {
		t.Errorf("the ladders of the code of binary-search: %v", ladders)
	}
	gz, err := gzip.NewReader(bytes.NewReader(read("code/numpy.tar.gz")))
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(gz)
	var names []string
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, h.Name)
	}
	if len(names) != 1 || names[0] != "numpy/binary-search/binary_search.py" || files2["code/numpy.tar.gz"].Rows != 1 {
		t.Errorf("code/numpy.tar.gz has %v", names)
	}

	pmodels := readParquet[PqModel](t, read("models.parquet"))
	if len(pmodels) != files2["models.parquet"].Rows || len(pmodels) != len(models) {
		t.Errorf("models.parquet has %d rows, models.csv %d", len(pmodels), len(models))
	}
	for _, r := range pmodels {
		if r.Key != "model-a" {
			continue
		}
		var total *PqFact
		for i := range r.Facts {
			if r.Facts[i].ID == "params.total" {
				total = &r.Facts[i]
			}
		}
		if r.Role != "checkpoint" || total == nil || total.Value != "7000000000" || len(total.Sources) != 1 || total.Sources[0].Loc != "Table 1" {
			t.Errorf("the Parquet row of model-a: %+v", r)
		}
	}
	presults := readParquet[PqResult](t, read("results.parquet"))
	if len(presults) != 1 || presults[0].Score == nil || *presults[0].Score != 0.886 || presults[0].IntervalHigh == nil || *presults[0].IntervalHigh != 0.89 || presults[0].Votes != nil || presults[0].Setting != `{"shots":5}` {
		t.Errorf("results.parquet = %+v", presults)
	}
	works := readParquet[PqWork](t, read("works.parquet"))
	byWork := map[string]PqWork{}
	for _, w := range works {
		byWork[w.Key] = w
	}
	if w := byWork["attention-is-all-you-need"]; w.ArXiv != "1706.03762" || w.DOI != "10.48550/arxiv.1706.03762" || w.Year == nil || *w.Year != 2017 || len(w.Authors) != 2 || w.Cited != "null" {
		t.Errorf("the work card in works.parquet: %+v", w)
	}
	// A work that is a node keeps the fields of its kind under "fields" in its record.
	if w := byWork["turing-1936"]; w.Type != "paper" || w.Year == nil || *w.Year != 1936 || len(w.Authors) != 1 || w.Wikidata != "Q1129575" || w.DOI != "10.1112/plms/s2-42.1.230" {
		t.Errorf("the work node in works.parquet: %+v", w)
	}
	for _, k := range []string{"concept", "incident", "model"} {
		rows := readParquet[PqItem](t, read("items/"+k+".parquet"))
		if len(rows) == 0 || len(rows) != files2["items/"+k+".jsonl"].Rows {
			t.Errorf("items/%s.parquet has %d rows, the JSON Lines %d", k, len(rows), files2["items/"+k+".jsonl"].Rows)
		}
		for _, r := range rows {
			if r.Kind != k || !json.Valid([]byte(r.Fields)) || !json.Valid([]byte(r.Status)) || !json.Valid([]byte(r.Claims)) {
				t.Errorf("a row of items/%s.parquet: %+v", k, r)
			}
		}
	}

	page := false
	for p := range files2 {
		if !strings.HasPrefix(p, "pages/") {
			continue
		}
		each(t, filepath.Join(data, p), func(line []byte) {
			var l PageLine
			if err := json.Unmarshal(line, &l); err != nil {
				t.Fatal(err)
			}
			if l.Item == "binary-search" {
				page = p == "pages/searching.jsonl" && strings.HasPrefix(l.Markdown, "---\n") && l.Licence == "CC-BY-SA-4.0"
			}
		})
	}
	if !page {
		t.Errorf("pages/searching.jsonl does not have the page of binary-search: %v", files2)
	}
}
