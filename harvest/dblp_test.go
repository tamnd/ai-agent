// SPDX-License-Identifier: Apache-2.0

package harvest

import (
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDBLPQueries(t *testing.T) {
	q := DBLPPapersQuery("journals/jmlr", "2021-05-01", 10, 20)
	for _, want := range []string{"<https://dblp.org/streams/journals/jmlr>", "a dblp:Article", `FILTER(STR(?year) >= "2021")`,
		"LIMIT 10 OFFSET 20", "ORDER BY ?p"} {
		if !strings.Contains(q, want) {
			t.Errorf("the papers query has no %q:\n%s", want, q)
		}
	}
	q = DBLPAuthorsQuery("conf/nips", "", DBLPPage, 0)
	if !strings.Contains(q, "a dblp:Inproceedings") || strings.Contains(q, "FILTER") || !strings.Contains(q, "LIMIT 10000 OFFSET 0") {
		t.Errorf("the authors query:\n%s", q)
	}
	u, err := url.Parse(DBLPURL(q))
	if err != nil || u.Host != "sparql.dblp.org" || u.Path != "/sparql" || u.Query().Get("query") != q {
		t.Errorf("url %v %v", u, err)
	}
}

func TestDBLPRows(t *testing.T) {
	b, err := os.ReadFile("testdata/dblp-papers.json")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := DBLPPapers(b, "conf/nips", "s")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("%d rows, want 3", len(rows))
	}
	r := rows[0]
	if r.ID != "conf/nips/000100BAGMF22" || r.Name != "Masked Autoencoders that Listen" || r.Kind != "work" ||
		r.URL != "https://dblp.org/rec/conf/nips/000100BAGMF22" || r.SameAs["dblp"] != r.ID || len(r.SameAs) != 1 {
		t.Errorf("row %+v", r)
	}
	if r.Fields["venue"] != "conf/nips" || r.Fields["year"] != "2022" {
		t.Errorf("fields %v", r.Fields)
	}
	b, err = os.ReadFile("testdata/dblp-authors.json")
	if err != nil {
		t.Fatal(err)
	}
	as, err := DBLPAuthors(b)
	if err != nil {
		t.Fatal(err)
	}
	dblpSetAuthors(&r, as[r.ID])
	want := []string{"Po-Yao Huang", "Hu Xu", "Juncheng Li", "Alexei Baevski", "Michael Auli", "Wojciech Galuba",
		"Florian Metze", "Christoph Feichtenhofer"}
	if !reflect.DeepEqual(r.Fields["authors"], want) {
		t.Errorf("authors %v, want %v", r.Fields["authors"], want)
	}
	if l := r.Links["author"]; len(l) != 8 || l[0] != "154/3943-1" || l[1] != "11/6234-1" {
		t.Errorf("author ids %v", l)
	}

	b, err = os.ReadFile("testdata/dblp-papers-naacl.json")
	if err != nil {
		t.Fatal(err)
	}
	rows, err = DBLPPapers(b, "conf/naacl", "s")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 || rows[0].SameAs["doi"] != "10.18653/v1/2022.findings-naacl.39" {
		t.Errorf("rows %+v", rows)
	}
	rows, err = DBLPPapers([]byte(`{"head":{"vars":["p","title","y","doi","wikidata"]},"results":{"bindings":[
		{"p":{"value":"https://dblp.org/rec/conf/naacl/DevlinCLT19"},"title":{"value":"BERT: Pre-training of Deep Bidirectional Transformers for Language Understanding."},
		 "wikidata":{"value":"http://www.wikidata.org/entity/Q57267388"}},
		{"p":{"value":"https://dblp.org/rec/conf/x/Q19"},"title":{"value":"Is it?"}},
		{"p":{"value":"https://example.org/not-a-record"},"title":{"value":"Skipped."}}]}}`), "conf/naacl", "s")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].SameAs["wikidata"] != "Q57267388" || rows[1].Name != "Is it?" || rows[0].Fields["year"] != nil {
		t.Errorf("rows %+v", rows)
	}
}

func TestDBLPSchema(t *testing.T) {
	if _, err := DBLPPapers([]byte(`<html>Making sure you're not a bot!</html>`), "conf/nips", "s"); err == nil {
		t.Error("a page that is not JSON is not an error")
	}
	if _, err := DBLPPapers([]byte(`{"head":{"vars":["p","name"]},"results":{"bindings":[]}}`), "conf/nips", "s"); err == nil {
		t.Error("a result with no title variable is not an error")
	}
	if _, err := DBLPAuthors([]byte(`{"head":{"vars":["p"]},"results":{"bindings":[]}}`)); err == nil {
		t.Error("a result with no authors variable is not an error")
	}
}

func TestDBLPStreams(t *testing.T) {
	root := t.TempDir()
	s, err := dblpStreams(root)
	if err != nil || !reflect.DeepEqual(s, DBLPStreams) {
		t.Errorf("default streams %v %v", s, err)
	}
	for _, x := range DBLPStreams {
		if !dblpStreamRE.MatchString(x) {
			t.Errorf("default stream %q is not a stream id", x)
		}
	}
	if err := os.MkdirAll(filepath.Join(root, "crosswalk"), 0o755); err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(root, DBLPStreamsFile)
	for _, c := range []struct {
		body string
		want []string
		bad  bool
	}{
		{"streams: [conf/nips, journals/tmlr]\n", []string{"conf/nips", "journals/tmlr"}, false},
		{"streams: []\n", nil, true},
		{"streams: [nips]\n", nil, true},
		{"streams: [conf/nips/2017]\n", nil, true},
		{"- conf/nips\n", nil, true},
	} {
		if err := os.WriteFile(f, []byte(c.body), 0o644); err != nil {
			t.Fatal(err)
		}
		s, err := dblpStreams(root)
		if (err != nil) != c.bad || (!c.bad && !reflect.DeepEqual(s, c.want)) {
			t.Errorf("%q: %v %v", c.body, s, err)
		}
	}
}
