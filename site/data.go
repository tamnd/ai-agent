// SPDX-License-Identifier: Apache-2.0

package site

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/tamnd/ai-agent/card"
	"github.com/tamnd/ai-agent/claims"
	"github.com/tamnd/ai-agent/results"
)

// The forms of the datasets of 16.10 that the build does not write yet; the manifest lists them under not_built.
var notBuilt = []string{}

// Dataset is one dataset of the manifest.
type Dataset struct {
	Name    string     `json:"name"`
	Licence string     `json:"licence"`
	Files   []DataFile `json:"files"`
}

// DataFile is one file of a dataset.
type DataFile struct {
	Path   string `json:"path"`
	Rows   int    `json:"rows"`
	Bytes  int    `json:"bytes"`
	SHA256 string `json:"sha256"`
}

// Manifest is /data/manifest.json: the datasets with their files, rows and sums.
type Manifest struct {
	AsOf     string    `json:"as_of"`
	Commit   string    `json:"commit,omitempty"`
	Version  string    `json:"aik_version,omitempty"`
	Schema   string    `json:"record_schema"`
	Datasets []Dataset `json:"datasets"`
	NotBuilt []string  `json:"not_built"`
}

// ClaimLine is one line of claims.jsonl: a claim with the item it is on.
type ClaimLine struct {
	Item string `json:"item"`
	URL  string `json:"url"`
	claims.Claim
}

// ResultLine is one line of results.jsonl: a result card that the citation check found, with its comparable group.
type ResultLine struct {
	URL string `json:"url"`
	*results.Result
	Group string `json:"group"`
}

// jsonl collects the lines of a file.
type jsonl struct {
	buf  bytes.Buffer
	rows int
}

func (j *jsonl) add(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	j.buf.Write(b)
	j.buf.WriteByte('\n')
	j.rows++
	return nil
}

// dataFile is a file of a dataset before it is written; rows counts the records in it (the lines of a JSON Lines file,
// the rows of a CSV file without its header or of a Parquet file, the edges of a GraphML file, the files of a tarball).
type dataFile struct {
	path string
	data []byte
	rows int
}

// datasets writes the datasets of 16.10 to dir and their manifest.json: items/<kind> with the record of every node and
// card as JSON Lines and Parquet, models as Parquet and CSV, results as JSON Lines, Parquet and CSV, works.parquet,
// claims.jsonl, the graph as JSON Lines and GraphML, incidents.jsonl, policy.jsonl, experiments.jsonl, code.jsonl with a
// tarball of each framework, and pages/<top>.jsonl.
// nodes holds the records of the nodes with their ladders. Like the site, the datasets go to a new directory that
// replaces dir at the end, so a failed build leaves the old ones.
func (b *builder) datasets(dir string, g *Graph, nodes []Record) error {
	items := map[string]*jsonl{}
	pqItems := map[string][]PqItem{}
	var pqModels []PqModel
	var pqWorks []PqWork
	var cl, gr, rs, inc, pol jsonl
	var all []Record
	var edges []Edge
	add := func(r Record) error {
		all = append(all, r)
		if items[r.Kind] == nil {
			items[r.Kind] = &jsonl{}
		}
		if err := items[r.Kind].add(r); err != nil {
			return err
		}
		pqItems[r.Kind] = append(pqItems[r.Kind], pqItem(r))
		switch r.Kind {
		case "model":
			pqModels = append(pqModels, pqModel(r))
		case "work":
			pqWorks = append(pqWorks, pqWork(r))
		case "incident":
			if err := inc.add(r); err != nil {
				return err
			}
		case "policy", "provision":
			if err := pol.add(r); err != nil {
				return err
			}
		}
		for _, e := range r.Out {
			edges = append(edges, e)
			if err := gr.add(e); err != nil {
				return err
			}
		}
		return nil
	}
	for _, r := range nodes {
		if err := add(r); err != nil {
			return err
		}
		for _, c := range r.Claims {
			if err := cl.add(ClaimLine{Item: r.Key, URL: r.URL, Claim: c}); err != nil {
				return err
			}
		}
	}
	shown := map[*card.Card]*results.Result{}
	for _, r := range g.Results().Results {
		if r.Card != nil && r.Shown() {
			shown[r.Card] = r
		}
	}
	for _, cd := range b.c.Cards {
		if cd.Kind == results.Kind && shown[cd] == nil {
			continue // a result card that the citation check did not find is not shown (10.6.2)
		}
		if err := add(g.Record(Thing{Card: cd}, b.meta.Date)); err != nil {
			return err
		}
	}
	var shownResults []*results.Result
	var pqResults []PqResult
	for _, r := range g.Results().Results {
		if r.Card == nil || shown[r.Card] == nil {
			continue
		}
		shownResults = append(shownResults, r)
		pqResults = append(pqResults, pqResult(g, r))
		if err := rs.add(ResultLine{URL: g.URL(Thing{Card: r.Card}), Result: r, Group: r.Group}); err != nil {
			return err
		}
	}

	var ex, code jsonl
	tars := map[string]*tarball{}
	pages := map[string]*jsonl{}
	for _, n := range b.c.Nodes {
		url := b.opt.BaseURL + "/" + n.Slug + "/"
		for _, e := range n.Exps {
			if b.hiddenExp(n, e) {
				continue
			}
			l := ExperimentLine{Item: n.Slug, URL: url, ID: e.Slug, Path: e.Rel, Results: e.Results}
			var err error
			if e.Design != nil {
				if l.Design, err = asJSON(e.Design); err != nil {
					return err
				}
			}
			if e.Expect != nil {
				if l.Expect, err = asJSON(e.Expect); err != nil {
					return err
				}
			}
			if err := ex.add(l); err != nil {
				return err
			}
		}
		for _, im := range n.Impls {
			l := b.codeLine(n, im)
			if err := code.add(l); err != nil {
				return err
			}
			if l.Ladder != "X" {
				continue
			}
			if tars[im.Lang] == nil {
				tars[im.Lang] = newTarball()
			}
			for _, f := range l.Files {
				if err := tars[im.Lang].add(im.Lang+"/"+n.Slug+"/"+f.Name, f.Content); err != nil {
					return err
				}
			}
		}
		if !HasTwin(n) {
			continue
		}
		md, err := os.ReadFile(filepath.Join(n.Dir, "node.md"))
		if err != nil {
			return err
		}
		top := topArea(n.Area)
		if pages[top] == nil {
			pages[top] = &jsonl{}
		}
		if err := pages[top].add(PageLine{Item: n.Slug, Tag: n.Tag, URL: url, Area: n.Area, Licence: "CC-BY-SA-4.0", Markdown: string(md)}); err != nil {
			return err
		}
	}

	m := Manifest{AsOf: b.meta.Date, Commit: b.opt.Commit, Version: b.opt.Version, Schema: "record.v1", NotBuilt: notBuilt}
	var out []dataFile
	set := func(name, licence string, fs ...dataFile) {
		d := Dataset{Name: name, Licence: licence}
		for _, f := range fs {
			sum := sha256.Sum256(f.data)
			d.Files = append(d.Files, DataFile{Path: f.path, Rows: f.rows, Bytes: len(f.data), SHA256: hex.EncodeToString(sum[:])})
			out = append(out, f)
		}
		m.Datasets = append(m.Datasets, d)
	}
	lines := func(p string, j *jsonl) dataFile { return dataFile{p, j.buf.Bytes(), j.rows} }
	const open = "CC0-1.0 where the sources allow it, otherwise per field (19.2)"

	var fs []dataFile
	for _, k := range sortedKeys(items) {
		pq, err := writeParquet(pqItems[k])
		if err != nil {
			return err
		}
		fs = append(fs, lines("items/"+k+".jsonl", items[k]), dataFile{"items/" + k + ".parquet", pq, len(pqItems[k])})
	}
	set("items", open, fs...)
	models, nm, err := modelsCSV(all)
	if err != nil {
		return err
	}
	mpq, err := writeParquet(pqModels)
	if err != nil {
		return err
	}
	set("models", open+"; the fields derived from Epoch AI keep CC-BY-4.0 with attribution", dataFile{"models.parquet", mpq, len(pqModels)}, dataFile{"models.csv", models, nm})
	rcsv, err := resultsCSV(g, shownResults)
	if err != nil {
		return err
	}
	rpq, err := writeParquet(pqResults)
	if err != nil {
		return err
	}
	set("results", open, lines("results.jsonl", &rs), dataFile{"results.parquet", rpq, len(pqResults)}, dataFile{"results.csv", rcsv, rs.rows})
	set("claims", "CC0-1.0; the text of a claim quotes the prose, CC-BY-SA-4.0", lines("claims.jsonl", &cl))
	set("graph", "CC0-1.0", lines("graph.jsonl", &gr), dataFile{"graph.graphml", graphML(all, edges), gr.rows})
	wpq, err := writeParquet(pqWorks)
	if err != nil {
		return err
	}
	set("works", "CC0-1.0", dataFile{"works.parquet", wpq, len(pqWorks)})
	set("incidents", "per source (19.2)", lines("incidents.jsonl", &inc))
	set("policy", "per source (19.2)", lines("policy.jsonl", &pol))
	set("experiments", "MIT; CC0-1.0 for the numbers", lines("experiments.jsonl", &ex))
	fs = []dataFile{lines("code.jsonl", &code)}
	for _, fw := range sortedKeys(tars) {
		data, err := tars[fw].close()
		if err != nil {
			return err
		}
		fs = append(fs, dataFile{"code/" + fw + ".tar.gz", data, tars[fw].files})
	}
	set("code", "MIT", fs...)
	fs = nil
	for _, top := range sortedKeys(pages) {
		fs = append(fs, lines("pages/"+top+".jsonl", pages[top]))
	}
	set("pages", "CC-BY-SA-4.0 for the prose, MIT for the code", fs...)

	next := filepath.Clean(dir) + ".next"
	_ = os.RemoveAll(next)
	for _, f := range out {
		full := filepath.Join(next, filepath.FromSlash(f.path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(full, f.data, 0o644); err != nil {
			return err
		}
	}
	mb, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(next, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(next, "manifest.json"), append(mb, '\n'), 0o644); err != nil {
		return err
	}
	old := filepath.Clean(dir) + ".old"
	if err := os.RemoveAll(old); err != nil {
		return err
	}
	if _, err := os.Stat(dir); err == nil {
		if err := os.Rename(dir, old); err != nil {
			return err
		}
	}
	if err := os.Rename(next, dir); err != nil {
		return err
	}
	return os.RemoveAll(old)
}
