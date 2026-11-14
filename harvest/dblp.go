// SPDX-License-Identifier: Apache-2.0

package harvest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// DBLPSPARQL is dblp's SPARQL endpoint, a QLever engine over the dblp knowledge graph. The robots.txt of dblp.org
// disallows /search, the host of the search API, and its other pages answer robots with a challenge; the robots.txt
// of sparql.dblp.org allows /sparql with a Crawl-delay of 10 s.
const DBLPSPARQL = "https://sparql.dblp.org/sparql"

// DBLPStreamsFile lists the venue streams of dblp to read; without it the adapter reads DBLPStreams.
const DBLPStreamsFile = "crosswalk/dblp.yaml"

// DBLPStreams are the main venues of machine learning and AI by their dblp stream ids: the machine learning
// conferences and journals, the general AI conferences and journals, vision, language, data mining, robot learning,
// and fairness and AI ethics.
var DBLPStreams = []string{
	"conf/nips", "conf/icml", "conf/iclr", "journals/jmlr", "journals/tmlr", "conf/colt", "conf/aistats", "conf/uai",
	"conf/aaai", "conf/ijcai", "journals/ai", "journals/jair",
	"conf/cvpr", "conf/iccv", "conf/eccv", "journals/pami",
	"conf/acl", "conf/emnlp", "conf/naacl",
	"conf/kdd", "conf/corl", "conf/fat", "conf/aies",
}

// DBLPPage is the number of papers of a page of a query.
const DBLPPage = 10000

var (
	dblpStreamRE  = regexp.MustCompile(`^(conf|journals)/[a-z0-9][a-z0-9-]{0,39}$`)
	dblpHomonymRE = regexp.MustCompile(` \d{4}$`)
)

// dblpStreams reads the streams file, or gives the default streams.
func dblpStreams(root string) ([]string, error) {
	b, err := os.ReadFile(filepath.Join(root, DBLPStreamsFile))
	switch {
	case os.IsNotExist(err):
		return DBLPStreams, nil
	case err != nil:
		return nil, err
	}
	var doc struct {
		Streams []string `yaml:"streams"`
	}
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return nil, &ErrSchema{"dblp", DBLPStreamsFile + " is not a map with streams: " + err.Error()}
	}
	for _, s := range doc.Streams {
		if !dblpStreamRE.MatchString(s) {
			return nil, &ErrSchema{"dblp", DBLPStreamsFile + ": " + strconv.Quote(s) + " is not a stream id such as conf/nips"}
		}
	}
	if len(doc.Streams) == 0 {
		return nil, &ErrSchema{"dblp", DBLPStreamsFile + " names no stream"}
	}
	return doc.Streams, nil
}

// dblpWhere is the pattern of the papers of a stream: the papers of a conference are Inproceedings, those of a
// journal Articles, so the proceedings volumes (Editorship) are not read.
func dblpWhere(stream, since string) string {
	typ := "dblp:Inproceedings"
	if strings.HasPrefix(stream, "journals/") {
		typ = "dblp:Article"
	}
	w := "  ?p dblp:publishedInStream <https://dblp.org/streams/" + stream + "> ; a " + typ + " ; dblp:yearOfPublication ?year .\n"
	if len(since) >= 4 {
		// The years are typed xsd:gYear, which the engine does not compare with a number; four digits compare as text.
		w += `  FILTER(STR(?year) >= "` + since[:4] + `")` + "\n"
	}
	return w
}

// DBLPPapersQuery is the query of a page of the papers of a stream: the key, the title, the year, the DOI and the
// Wikidata item of each.
func DBLPPapersQuery(stream, since string, size, offset int) string {
	return `PREFIX dblp: <https://dblp.org/rdf/schema#>
SELECT ?p (SAMPLE(?t) AS ?title) (SAMPLE(?year) AS ?y) (SAMPLE(?d) AS ?doi) (SAMPLE(?w) AS ?wikidata) WHERE {
` + dblpWhere(stream, since) + `  ?p dblp:title ?t .
  OPTIONAL { ?p dblp:doi ?d }
  OPTIONAL { ?p dblp:wikidata ?w }
}
GROUP BY ?p
ORDER BY ?p
LIMIT ` + strconv.Itoa(size) + ` OFFSET ` + strconv.Itoa(offset)
}

// DBLPAuthorsQuery is the query of a page of the authors of the papers of a stream: for each paper, one line per
// author with the author's place, the name that dblp prints and the person's dblp id. One query that joins the
// authors to the papers runs past the engine's time limit, so they are two.
func DBLPAuthorsQuery(stream, since string, size, offset int) string {
	return `PREFIX dblp: <https://dblp.org/rdf/schema#>
SELECT ?p (GROUP_CONCAT(?sig; SEPARATOR="\n") AS ?authors) WHERE {
` + dblpWhere(stream, since) + `  ?s dblp:signaturePublication ?p ; dblp:signatureOrdinal ?o ; dblp:signatureDblpName ?name .
  OPTIONAL { ?s dblp:signatureCreator ?person }
  BIND(CONCAT(STR(?o), "\t", ?name, "\t", COALESCE(STR(?person), "")) AS ?sig)
}
GROUP BY ?p
ORDER BY ?p
LIMIT ` + strconv.Itoa(size) + ` OFFSET ` + strconv.Itoa(offset)
}

// DBLPURL is the GET URL of a query.
func DBLPURL(query string) string {
	return DBLPSPARQL + "?" + url.Values{"query": {query}}.Encode()
}

func dblpBindings(b []byte, need string) ([]map[string]struct {
	Value string `json:"value"`
}, error) {
	var r struct {
		Head struct {
			Vars []string `json:"vars"`
		} `json:"head"`
		sparqlResult
	}
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, &ErrSchema{"dblp", "the SPARQL result is not JSON: " + err.Error()}
	}
	for _, v := range r.Head.Vars {
		if v == need {
			return r.Results.Bindings, nil
		}
	}
	return nil, &ErrSchema{"dblp", "the SPARQL result has no variable " + need}
}

// dblpKey is the dblp key of a record IRI: "conf/nips/VaswaniSPUJGKP17".
func dblpKey(iri string) string {
	if k, ok := strings.CutPrefix(iri, "https://dblp.org/rec/"); ok {
		return k
	}
	return ""
}

// DBLPPapers reads a page of papers into rows of a stream, without authors.
func DBLPPapers(b []byte, stream, snap string) ([]Row, error) {
	bs, err := dblpBindings(b, "title")
	if err != nil {
		return nil, err
	}
	var out []Row
	for _, x := range bs {
		key := dblpKey(x["p"].Value)
		// dblp ends each title with a full stop of its own; a title that ends in ? or ! has none.
		name := clean(strings.TrimSuffix(clean(x["title"].Value), "."))
		if key == "" || name == "" {
			continue
		}
		row := Row{ID: key, Name: name, Kind: "work", URL: "https://dblp.org/rec/" + key,
			SameAs: map[string]string{"dblp": key}, Snapshot: snap, Fields: map[string]any{"venue": stream}}
		if y := x["y"].Value; y != "" {
			row.Fields["year"] = y
		}
		if d, ok := strings.CutPrefix(x["doi"].Value, "https://doi.org/"); ok && d != "" {
			row.SameAs["doi"] = strings.ToLower(d)
		}
		if q := qid(x["wikidata"].Value); strings.HasPrefix(q, "Q") {
			row.SameAs["wikidata"] = q
		}
		out = append(out, row)
	}
	return out, nil
}

// DBLPAuthor is an author of a paper: the name without dblp's homonym number ("Hu Xu 0001" is "Hu Xu") and the dblp
// person id ("26/9012"), which keeps the number.
type DBLPAuthor struct {
	Place int
	Name  string
	PID   string
}

// DBLPAuthors reads a page of authors: the authors of each paper by its key, in their order on the paper.
func DBLPAuthors(b []byte) (map[string][]DBLPAuthor, error) {
	bs, err := dblpBindings(b, "authors")
	if err != nil {
		return nil, err
	}
	out := map[string][]DBLPAuthor{}
	for _, x := range bs {
		key := dblpKey(x["p"].Value)
		if key == "" {
			continue
		}
		var as []DBLPAuthor
		for line := range strings.SplitSeq(x["authors"].Value, "\n") {
			f := strings.Split(line, "\t")
			if len(f) != 3 {
				continue
			}
			n, err := strconv.Atoi(f[0])
			name := clean(dblpHomonymRE.ReplaceAllString(clean(f[1]), ""))
			if err != nil || name == "" {
				continue
			}
			pid, _ := strings.CutPrefix(f[2], "https://dblp.org/pid/")
			as = append(as, DBLPAuthor{n, name, pid})
		}
		sort.SliceStable(as, func(i, j int) bool { return as[i].Place < as[j].Place })
		out[key] = as
	}
	return out, nil
}

// dblpSetAuthors puts the authors of a paper on its row.
func dblpSetAuthors(row *Row, as []DBLPAuthor) {
	var names, pids []string
	for _, a := range as {
		names = append(names, a.Name)
		if a.PID != "" {
			pids = append(pids, a.PID)
		}
	}
	if len(names) > 0 {
		row.Fields["authors"] = names
	}
	if len(pids) > 0 {
		row.Links = map[string][]string{"author": pids}
	}
}

// dblpPages runs a query page by page until a page is short, and gives the pages. With a limit below a page it reads
// one page of that size.
func dblpPages(ctx context.Context, r *Run, stream string, query func(size, offset int) string, need string, limit int) ([][]byte, []string, error) {
	size := DBLPPage
	if limit > 0 && limit < size {
		size = limit
	}
	var pages [][]byte
	var snaps []string
	for off := 0; ; off += size {
		b, snap, err := r.Get(ctx, DBLPURL(query(size, off)), 256<<20)
		if err != nil {
			return nil, nil, fmt.Errorf("dblp %s at %d: %w", stream, off, err)
		}
		bs, err := dblpBindings(b, need)
		if err != nil {
			return nil, nil, fmt.Errorf("dblp %s at %d: %w", stream, off, err)
		}
		pages, snaps = append(pages, b), append(snaps, snap)
		if len(bs) < size || size < DBLPPage {
			return pages, snaps, nil
		}
	}
}

// harvestDBLP reads the papers of each venue stream of crosswalk/dblp.yaml from dblp's SPARQL endpoint: a row for
// each paper with its dblp key, title, year, DOI, Wikidata item and authors. A paper in two streams keeps the venue
// of the first. -since keeps the papers of that year and later; -limit keeps at most that many papers of each stream.
func harvestDBLP(ctx context.Context, r *Run, o Options) ([]Row, error) {
	streams, err := dblpStreams(r.Root)
	if err != nil {
		return nil, err
	}
	byID := map[string]bool{}
	var out []Row
	for _, st := range streams {
		pages, snaps, err := dblpPages(ctx, r, st, func(n, off int) string { return DBLPPapersQuery(st, o.Since, n, off) }, "title", o.Limit)
		if err != nil {
			return nil, err
		}
		var rows []Row
		for i, b := range pages {
			rs, err := DBLPPapers(b, st, snaps[i])
			if err != nil {
				return nil, err
			}
			rows = append(rows, rs...)
		}
		if len(rows) == 0 {
			r.Notes = append(r.Notes, "the stream "+st+" has no papers")
			continue
		}
		pages, _, err = dblpPages(ctx, r, st, func(n, off int) string { return DBLPAuthorsQuery(st, o.Since, n, off) }, "authors", o.Limit)
		if err != nil {
			return nil, err
		}
		authors := map[string][]DBLPAuthor{}
		for _, b := range pages {
			as, err := DBLPAuthors(b)
			if err != nil {
				return nil, err
			}
			for k, v := range as {
				authors[k] = v
			}
		}
		dup := 0
		for _, row := range keep(rows, o.Limit) {
			if byID[row.ID] {
				dup++
				continue
			}
			byID[row.ID] = true
			dblpSetAuthors(&row, authors[row.ID])
			out = append(out, row)
		}
		r.Skip(dup, "papers of "+st+" that an earlier stream has")
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}
