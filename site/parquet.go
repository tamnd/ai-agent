// SPDX-License-Identifier: Apache-2.0

package site

import (
	"bytes"
	"encoding/json"
	"math"
	"strings"

	"github.com/parquet-go/parquet-go"

	"github.com/tamnd/ai-agent/results"
)

// The rows of the Parquet forms of 16.10. A part of a record that has a fixed shape gets its own columns; a part whose
// shape changes from kind to kind (the fields, the claims, the ladders) is one column of JSON, which holds the JSON
// null when the item has none. (A column that is both optional and JSON makes parquet-go v0.32 panic.)

// PqItem is a row of items/<kind>.parquet.
type PqItem struct {
	URL      string   `parquet:"url"`
	Tag      string   `parquet:"tag,optional"`
	Kind     string   `parquet:"kind,dict"`
	Key      string   `parquet:"key"`
	Name     string   `parquet:"name"`
	AsOf     string   `parquet:"as_of,dict"`
	Summary  string   `parquet:"summary,optional"`
	Markdown string   `parquet:"markdown_url,optional"`
	Fields   string   `parquet:"fields,json"`
	Facts    []PqFact `parquet:"facts,list"`
	Claims   string   `parquet:"claims,json"`
	Status   string   `parquet:"status,json"`
	Out      []PqEdge `parquet:"edges_out,list"`
	In       []PqEdge `parquet:"edges_in,list"`
	Results  []string `parquet:"results,list"`
}

// PqFact is a fact of a row, with its value as JSON.
type PqFact struct {
	ID      string     `parquet:"id,dict"`
	Label   string     `parquet:"label,dict"`
	Value   string     `parquet:"value,json"`
	Unit    string     `parquet:"unit,optional,dict"`
	Text    string     `parquet:"text"`
	Status  string     `parquet:"status,optional,dict"`
	AsOf    string     `parquet:"as_of,optional"`
	Note    string     `parquet:"note,optional"`
	Sources []PqSource `parquet:"sources,list"`
}

// PqSource is a source of a fact.
type PqSource struct {
	Src  string `parquet:"src"`
	Kind string `parquet:"kind,optional,dict"`
	Loc  string `parquet:"loc,optional"`
	AsOf string `parquet:"as_of,optional"`
}

// PqEdge is an edge of a row.
type PqEdge struct {
	Type  string `parquet:"type,dict"`
	From  string `parquet:"from"`
	To    string `parquet:"to"`
	Known bool   `parquet:"known"`
}

// PqModel is a row of models.parquet.
type PqModel struct {
	Key   string   `parquet:"key"`
	Tag   string   `parquet:"tag,optional"`
	Name  string   `parquet:"name"`
	URL   string   `parquet:"url"`
	Role  string   `parquet:"role,optional,dict"`
	Owner string   `parquet:"owner,optional"`
	Facts []PqFact `parquet:"facts,list"`
}

// PqResult is a row of results.parquet.
type PqResult struct {
	Tag          string   `parquet:"tag"`
	URL          string   `parquet:"url"`
	Model        string   `parquet:"model"`
	ModelVariant string   `parquet:"model_variant,optional"`
	Benchmark    string   `parquet:"benchmark,dict"`
	Version      string   `parquet:"version,dict"`
	Metric       string   `parquet:"metric,dict"`
	Score        *float64 `parquet:"score,optional"`
	Unit         string   `parquet:"unit,optional,dict"`
	IntervalLow  *float64 `parquet:"interval_low,optional"`
	IntervalHigh *float64 `parquet:"interval_high,optional"`
	Votes        *int64   `parquet:"votes,optional"`
	Setting      string   `parquet:"setting,json"`
	Source       string   `parquet:"source"`
	Locator      string   `parquet:"locator,optional"`
	SourceKind   string   `parquet:"source_kind,optional,dict"`
	Date         string   `parquet:"date,optional"`
	SelfReported bool     `parquet:"self_reported"`
	Evidence     string   `parquet:"evidence,optional,dict"`
	Group        string   `parquet:"group,optional"`
}

// PqWork is a row of works.parquet: a work card with its ids.
type PqWork struct {
	Key          string   `parquet:"key"`
	Tag          string   `parquet:"tag,optional"`
	Name         string   `parquet:"name"`
	URL          string   `parquet:"url"`
	Type         string   `parquet:"type,optional,dict"`
	Year         *int64   `parquet:"year,optional"`
	Venue        string   `parquet:"venue,optional,dict"`
	Authors      []string `parquet:"authors,list"`
	DOI          string   `parquet:"doi,optional"`
	ArXiv        string   `parquet:"arxiv,optional"`
	OpenAlex     string   `parquet:"openalex,optional"`
	SemanticSch  string   `parquet:"semantic_scholar,optional"`
	Wikidata     string   `parquet:"wikidata,optional"`
	SameAs       string   `parquet:"same_as,json"`
	Standing     string   `parquet:"standing,optional,dict"`
	LicenceClass string   `parquet:"licence_class,optional,dict"`
	Cited        string   `parquet:"cited,json"`
}

// blobs are the columns of long text or JSON. Their bounds in the footer would copy whole values and mean nothing, so
// the writer skips them.
var blobs = []string{"fields", "claims", "status", "summary", "setting", "same_as", "cited", "markdown_url"}

// writeParquet encodes rows as a Parquet file with zstd pages, without page statistics, which would copy the long
// values of each page into its header.
func writeParquet[T any](rows []T) ([]byte, error) {
	var buf bytes.Buffer
	opts := []parquet.WriterOption{parquet.Compression(&parquet.Zstd), parquet.CreatedBy("aik", "", ""), parquet.DataPageStatistics(false)}
	for _, b := range blobs {
		opts = append(opts, parquet.SkipPageBounds(b))
	}
	w := parquet.NewGenericWriter[T](&buf, opts...)
	if _, err := w.Write(rows); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// jsonText is v as JSON; nil, and a value that has no JSON, give null.
func jsonText(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "null"
	}
	return string(b)
}

func pqFacts(fs []RecordFact) []PqFact {
	out := make([]PqFact, 0, len(fs))
	for _, f := range fs {
		p := PqFact{ID: f.ID, Label: f.Label, Value: jsonText(f.Value), Unit: f.Unit, Text: f.Text, Status: f.Status, AsOf: f.AsOf, Note: f.Note}
		for _, s := range f.Sources {
			p.Sources = append(p.Sources, PqSource{Src: s.Src, Kind: s.Kind, Loc: s.Loc, AsOf: s.AsOf})
		}
		out = append(out, p)
	}
	return out
}

func pqEdges(es []Edge) []PqEdge {
	out := make([]PqEdge, 0, len(es))
	for _, e := range es {
		out = append(out, PqEdge(e))
	}
	return out
}

func pqItem(r Record) PqItem {
	p := PqItem{URL: r.URL, Tag: r.Tag, Kind: r.Kind, Key: r.Key, Name: r.Name, AsOf: r.AsOf, Summary: r.Summary,
		Markdown: r.Markdown, Fields: jsonText(r.Fields), Facts: pqFacts(r.Facts), Out: pqEdges(r.Out), In: pqEdges(r.In),
		Results: r.Results, Claims: "null", Status: "null"}
	if r.Fields == nil {
		p.Fields = "{}"
	}
	if len(r.Claims) > 0 {
		p.Claims = jsonText(r.Claims)
	}
	if r.Status != nil {
		p.Status = jsonText(r.Status)
	}
	return p
}

func pqModel(r Record) PqModel {
	return PqModel{Key: r.Key, Tag: r.Tag, Name: r.Name, URL: r.URL, Role: cell(field(r, "role")),
		Owner: cell(field(r, "owner")), Facts: pqFacts(r.Facts)}
}

func pqResult(g *Graph, r *results.Result) PqResult {
	p := PqResult{Tag: r.Tag, URL: g.URL(Thing{Card: r.Card}), Model: r.Model, ModelVariant: r.ModelVariant,
		Benchmark: r.Benchmark, Version: r.Version, Metric: r.Metric, Score: r.Score.V, Unit: r.Score.Unit,
		Setting: jsonText(r.Setting), Source: r.Source.Src, Locator: r.Source.Loc, SourceKind: r.Source.Kind,
		Date: r.Source.Date, SelfReported: r.SelfReported, Evidence: r.Evidence, Group: r.Group}
	if len(r.Score.Interval) == 2 {
		lo, hi := r.Score.Interval[0], r.Score.Interval[1]
		p.IntervalLow, p.IntervalHigh = &lo, &hi
	}
	if r.Score.Votes > 0 {
		v := int64(r.Score.Votes)
		p.Votes = &v
	}
	return p
}

// field is the value of a field of a record: a field of a card, a field of the front matter of a node, or a field of
// the kind of a node, which the front matter keeps under "fields". A card fact ({"v": ..., "src": ...}) gives its value.
func field(r Record, k string) any {
	if v, ok := r.Fields[k]; ok {
		return value(v)
	}
	if kf, ok := r.Fields["fields"].(map[string]any); ok {
		return value(kf[k])
	}
	return nil
}

// value unwraps a card fact ({"v": ..., "src": ...}) to its value.
func value(v any) any {
	if m, ok := v.(map[string]any); ok {
		if x, ok := m["v"]; ok {
			return x
		}
	}
	return v
}

func pqWork(r Record) PqWork {
	f := func(k string) any { return field(r, k) }
	p := PqWork{Key: r.Key, Tag: r.Tag, Name: r.Name, URL: r.URL, Type: cell(f("type")), Venue: cell(f("venue")),
		Standing: cell(f("standing")), LicenceClass: cell(f("licence_class")), DOI: cell(f("doi")), Cited: jsonText(f("cited")),
		SameAs: "null"}
	if y, ok := f("year").(float64); ok && y == math.Trunc(y) {
		v := int64(y)
		p.Year = &v
	}
	switch a := f("authors").(type) {
	case []any:
		for _, x := range a {
			if s := cell(value(x)); s != "" {
				p.Authors = append(p.Authors, s)
			}
		}
	case string:
		p.Authors = []string{a}
	}
	if sa, ok := r.Fields["same_as"].(map[string]any); ok {
		p.SameAs = jsonText(sa)
		id := func(k string) string {
			switch v := sa[k].(type) {
			case string:
				return v
			case []any:
				if len(v) > 0 {
					s, _ := v[0].(string)
					return s
				}
			}
			return ""
		}
		p.ArXiv, p.OpenAlex, p.SemanticSch, p.Wikidata = id("arxiv"), id("openalex"), id("semantic_scholar"), id("wikidata")
		if p.DOI == "" {
			p.DOI = id("doi")
		}
	}
	p.DOI = strings.TrimPrefix(strings.ToLower(p.DOI), "https://doi.org/")
	return p
}
