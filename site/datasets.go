// SPDX-License-Identifier: Apache-2.0

package site

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/tamnd/ai-agent/exp"
	"github.com/tamnd/ai-agent/node"
	"github.com/tamnd/ai-agent/results"
	"github.com/tamnd/ai-agent/status"
)

// cell writes a value of a record as one CSV cell: a string as it is, a list of strings joined with "; ", and any other
// value as JSON.
func cell(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(t)
	case []any:
		parts := make([]string, 0, len(t))
		for _, x := range t {
			s, ok := x.(string)
			if !ok {
				b, _ := json.Marshal(t)
				return string(b)
			}
			parts = append(parts, s)
		}
		return strings.Join(parts, "; ")
	}
	b, _ := json.Marshal(v)
	return string(b)
}

func writeCSV(rows [][]string) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err := w.WriteAll(rows); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// modelsCSV is models.csv (16.10): one row per model node or card with each fact as a value and its source.
func modelsCSV(recs []Record) ([]byte, int, error) {
	ids := map[string]bool{}
	var models []Record
	for _, r := range recs {
		if r.Kind != "model" {
			continue
		}
		models = append(models, r)
		for _, f := range r.Facts {
			ids[f.ID] = true
		}
	}
	cols := make([]string, 0, len(ids))
	for id := range ids {
		cols = append(cols, id)
	}
	sort.Strings(cols)
	head := []string{"key", "tag", "name", "url", "role", "owner"}
	for _, id := range cols {
		head = append(head, id, id+".src")
	}
	rows := [][]string{head}
	for _, r := range models {
		row := []string{r.Key, r.Tag, r.Name, r.URL, cell(field(r, "role")), cell(field(r, "owner"))}
		byID := map[string]RecordFact{}
		for _, f := range r.Facts {
			byID[f.ID] = f
		}
		for _, id := range cols {
			f, ok := byID[id]
			src := ""
			if ok && len(f.Sources) > 0 {
				srcs := make([]string, 0, len(f.Sources))
				for _, s := range f.Sources {
					srcs = append(srcs, s.Src)
				}
				src = strings.Join(srcs, "; ")
			}
			row = append(row, cell(f.Value), src)
		}
		rows = append(rows, row)
	}
	b, err := writeCSV(rows)
	return b, len(models), err
}

// resultsCSV is results.csv (16.10): one row per shown result card, with its setting as JSON.
func resultsCSV(g *Graph, rs []*results.Result) ([]byte, error) {
	rows := [][]string{{"tag", "url", "model", "model_variant", "benchmark", "version", "metric", "score", "unit",
		"interval_low", "interval_high", "votes", "setting", "source", "locator", "source_kind", "date", "self_reported",
		"evidence", "group"}}
	for _, r := range rs {
		score, lo, hi, votes := "", "", "", ""
		if r.Score.V != nil {
			score = strconv.FormatFloat(*r.Score.V, 'f', -1, 64)
		}
		if len(r.Score.Interval) == 2 {
			lo = strconv.FormatFloat(r.Score.Interval[0], 'f', -1, 64)
			hi = strconv.FormatFloat(r.Score.Interval[1], 'f', -1, 64)
		}
		if r.Score.Votes > 0 {
			votes = strconv.Itoa(r.Score.Votes)
		}
		setting, err := json.Marshal(r.Setting)
		if err != nil {
			return nil, err
		}
		rows = append(rows, []string{r.Tag, g.URL(Thing{Card: r.Card}), r.Model, r.ModelVariant, r.Benchmark, r.Version,
			r.Metric, score, r.Score.Unit, lo, hi, votes, string(setting), r.Source.Src, r.Source.Loc, r.Source.Kind,
			r.Source.Date, strconv.FormatBool(r.SelfReported), r.Evidence, r.Group})
	}
	return writeCSV(rows)
}

// graphML is graph.graphml (16.10): every item as a node and every edge, with the ends that are not items marked unknown.
func graphML(recs []Record, edges []Edge) []byte {
	var b bytes.Buffer
	esc := func(s string) string {
		var x bytes.Buffer
		_ = xml.EscapeText(&x, []byte(s))
		return x.String()
	}
	b.WriteString(xml.Header)
	b.WriteString(`<graphml xmlns="http://graphml.graphdrawing.org/xmlns">` + "\n")
	for _, k := range [][3]string{{"kind", "node", "string"}, {"name", "node", "string"}, {"url", "node", "string"},
		{"tag", "node", "string"}, {"known", "node", "boolean"}, {"type", "edge", "string"}} {
		fmt.Fprintf(&b, `  <key id="%s" for="%s" attr.name="%s" attr.type="%s"/>`+"\n", k[0], k[1], k[0], k[2])
	}
	b.WriteString(`  <graph id="ai" edgedefault="directed">` + "\n")
	seen := map[string]bool{}
	for _, r := range recs {
		if seen[r.Key] {
			continue
		}
		seen[r.Key] = true
		fmt.Fprintf(&b, `    <node id="%s"><data key="kind">%s</data><data key="name">%s</data><data key="url">%s</data><data key="tag">%s</data><data key="known">true</data></node>`+"\n",
			esc(r.Key), esc(r.Kind), esc(r.Name), esc(r.URL), esc(r.Tag))
	}
	var unknown []string
	for _, e := range edges {
		if !seen[e.To] {
			seen[e.To] = true
			unknown = append(unknown, e.To)
		}
	}
	sort.Strings(unknown)
	for _, k := range unknown {
		fmt.Fprintf(&b, `    <node id="%s"><data key="known">false</data></node>`+"\n", esc(k))
	}
	for i, e := range edges {
		fmt.Fprintf(&b, `    <edge id="e%d" source="%s" target="%s"><data key="type">%s</data></edge>`+"\n", i, esc(e.From), esc(e.To), esc(e.Type))
	}
	b.WriteString("  </graph>\n</graphml>\n")
	return b.Bytes()
}

// CodeLine is one line of code.jsonl: the implementation of a node in one framework or language, with its files and its
// level (X for a framework of 08.2, I for another language).
type CodeLine struct {
	Item   string      `json:"item"`
	URL    string      `json:"url"`
	Lang   string      `json:"lang"`
	Ladder string      `json:"ladder"`
	Level  int         `json:"level"`
	Files  []CodeFile  `json:"files"`
	Rung   status.Rung `json:"rung"`
}

// CodeFile is one file of an implementation.
type CodeFile struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Test    bool   `json:"test"`
	Content string `json:"content"`
}

// tarball packs files into a gzipped tar with a fixed time, so that the same files give the same bytes.
type tarball struct {
	buf   bytes.Buffer
	gz    *gzip.Writer
	tw    *tar.Writer
	files int
}

func newTarball() *tarball {
	t := &tarball{}
	t.gz, _ = gzip.NewWriterLevel(&t.buf, gzip.BestCompression)
	t.tw = tar.NewWriter(t.gz)
	return t
}

func (t *tarball) add(name, content string) error {
	h := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(content)), ModTime: time.Unix(0, 0).UTC(), Format: tar.FormatPAX}
	if err := t.tw.WriteHeader(h); err != nil {
		return err
	}
	_, err := t.tw.Write([]byte(content))
	t.files++
	return err
}

func (t *tarball) close() ([]byte, error) {
	if err := t.tw.Close(); err != nil {
		return nil, err
	}
	if err := t.gz.Close(); err != nil {
		return nil, err
	}
	return t.buf.Bytes(), nil
}

// codeLine gives the line of code.jsonl of an implementation.
func (b *builder) codeLine(n *node.Node, im node.Impl) CodeLine {
	l := CodeLine{Item: n.Slug, URL: b.opt.BaseURL + "/" + n.Slug + "/", Lang: im.Lang, Ladder: "I"}
	if status.Frameworks[im.Lang] {
		l.Ladder = "X"
	}
	if r, ok := b.status[n.Slug].I[im.Lang]; ok {
		l.Rung, l.Level = r, r.Level
	}
	for _, f := range im.Files {
		l.Files = append(l.Files, CodeFile{Name: f.Name, Path: f.Path, Test: f.Test, Content: f.Content})
	}
	return l
}

// ExperimentLine is one line of experiments.jsonl: the design, the test and the results of an experiment, as the files
// have them.
type ExperimentLine struct {
	Item    string       `json:"item"`
	URL     string       `json:"url"`
	ID      string       `json:"id"`
	Path    string       `json:"path"`
	Design  any          `json:"design"`
	Expect  any          `json:"expect"`
	Results *exp.Results `json:"results"`
}

// asJSON gives a value read from YAML with the names of its YAML keys, for a JSON line.
func asJSON(v any) (any, error) {
	if v == nil {
		return nil, nil
	}
	b, err := yaml.Marshal(v)
	if err != nil {
		return nil, err
	}
	var out any
	return out, yaml.Unmarshal(b, &out)
}

// hiddenExp tells if the quarantine hides a file of the experiment (15.7).
func (b *builder) hiddenExp(n *node.Node, e *exp.Experiment) bool {
	prefix := exp.Dir + "/" + e.Slug + "/"
	for _, f := range b.withdrawn[n.Slug].Files {
		if strings.HasPrefix(f, prefix) || f == exp.Dir+"/"+e.Slug {
			return true
		}
	}
	return false
}

// PageLine is one line of pages/<top>.jsonl: the Markdown twin of a node.
type PageLine struct {
	Item     string `json:"item"`
	Tag      string `json:"tag"`
	URL      string `json:"url"`
	Area     string `json:"area"`
	Licence  string `json:"licence"`
	Markdown string `json:"markdown"`
}

// topArea is the first part of an area path.
func topArea(area string) string {
	top, _, _ := strings.Cut(area, "/")
	if top == "" {
		return "_"
	}
	return top
}
