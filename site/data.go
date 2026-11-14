// SPDX-License-Identifier: Apache-2.0

package site

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"

	"github.com/tamnd/ai-agent/card"
	"github.com/tamnd/ai-agent/claims"
	"github.com/tamnd/ai-agent/results"
)

// The datasets of 16.10 that the build writes, as JSON Lines. The Parquet, CSV and GraphML forms, the code, the
// experiments and the pages are not written yet; the manifest lists them under not_built.
var notBuilt = []string{
	"items, models, results and works as Parquet; models and results as CSV; graph as GraphML",
	"models, works, incidents and policy as their own datasets (their cards are in items)",
	"code, experiments and pages",
}

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

// datasets writes the datasets of 16.10 to dir: items/<kind>.jsonl with the record of every node and card, claims.jsonl,
// graph.jsonl and results.jsonl, and manifest.json. nodes holds the records of the nodes with their ladders. Like the
// site, the datasets go to a new directory that replaces dir at the end, so a failed build leaves the old ones.
func (b *builder) datasets(dir string, g *Graph, nodes []Record) error {
	items := map[string]*jsonl{}
	var cl, gr, rs jsonl
	add := func(r Record) error {
		if items[r.Kind] == nil {
			items[r.Kind] = &jsonl{}
		}
		if err := items[r.Kind].add(r); err != nil {
			return err
		}
		for _, e := range r.Out {
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
	for _, r := range g.Results().Results {
		if r.Card == nil || shown[r.Card] == nil {
			continue
		}
		if err := rs.add(ResultLine{URL: g.URL(Thing{Card: r.Card}), Result: r, Group: r.Group}); err != nil {
			return err
		}
	}

	next := filepath.Clean(dir) + ".next"
	_ = os.RemoveAll(next)
	m := Manifest{AsOf: b.meta.Date, Commit: b.opt.Commit, Version: b.opt.Version, Schema: "record.v1", NotBuilt: notBuilt}
	write := func(name string, j *jsonl) (DataFile, error) {
		full := filepath.Join(next, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return DataFile{}, err
		}
		sum := sha256.Sum256(j.buf.Bytes())
		return DataFile{Path: name, Rows: j.rows, Bytes: j.buf.Len(), SHA256: hex.EncodeToString(sum[:])}, os.WriteFile(full, j.buf.Bytes(), 0o644)
	}
	kinds := make([]string, 0, len(items))
	for k := range items {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	ds := Dataset{Name: "items", Licence: "CC0-1.0 where the sources allow it, otherwise per field (19.2)"}
	for _, k := range kinds {
		f, err := write("items/"+k+".jsonl", items[k])
		if err != nil {
			return err
		}
		ds.Files = append(ds.Files, f)
	}
	m.Datasets = append(m.Datasets, ds)
	for _, d := range []struct {
		name, licence string
		j             *jsonl
	}{
		{"claims", "CC0-1.0; the text of a claim quotes the prose, CC-BY-SA-4.0", &cl},
		{"graph", "CC0-1.0", &gr},
		{"results", "CC0-1.0 where the sources allow it, otherwise per field (19.2)", &rs},
	} {
		f, err := write(d.name+".jsonl", d.j)
		if err != nil {
			return err
		}
		m.Datasets = append(m.Datasets, Dataset{Name: d.name, Licence: d.licence, Files: []DataFile{f}})
	}
	mb, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
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
