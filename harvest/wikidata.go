// SPDX-License-Identifier: Apache-2.0

package harvest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// Wikidata's robots.txt closes query.wikidata.org/sparql and the API to robots, so the adapter asks QLever, a
// public SPARQL engine over a weekly copy of Wikidata whose robots.txt leaves it open. The data is the same CC0
// statements; the run record names the engine.
const WikidataSPARQL = "https://qlever.dev/api/wikidata"

// WikidataClass is a class of AI items: the items of the class (Path "P31/P279*") or its subclasses ("P279+"), and
// the kind that the rows get.
type WikidataClass struct {
	QID  string
	Name string
	Path string
	Kind string
}

// WikidataClasses limit the adapter to AI items (21.2): no query walks all of Wikidata.
var WikidataClasses = []WikidataClass{
	{"Q115305900", "large language model", "P31/P279*", "model"},
	{"Q115215420", "machine learning model", "P31/P279*", "model"},
	{"Q192776", "artificial neural network", "P279+", "architecture"},
	{"Q111862379", "machine learning method", "P31/P279*", "method"},
	{"Q110688486", "machine learning algorithm", "P31/P279*", "method"},
	{"Q24895600", "AI accelerator", "P31/P279*", "hardware"},
	{"Q870780", "chatbot", "P31", "system"},
	{"Q2539", "machine learning", "P279", "concept"},
	{"Q11660", "artificial intelligence", "P279", "concept"},
}

// WikidataQuery is the SPARQL query of a class: one row per item with an English label, the most linked first.
func WikidataQuery(c WikidataClass, limit int) string {
	path := strings.NewReplacer("P31", "wdt:P31", "P279", "wdt:P279").Replace(c.Path)
	q := `PREFIX wd: <http://www.wikidata.org/entity/>
PREFIX wdt: <http://www.wikidata.org/prop/direct/>
PREFIX rdfs: <http://www.w3.org/2000/01/rdf-schema#>
PREFIX skos: <http://www.w3.org/2004/02/skos/core#>
PREFIX wikibase: <http://wikiba.se/ontology#>
PREFIX schema: <http://schema.org/>
SELECT ?item (SAMPLE(?l) AS ?label) (GROUP_CONCAT(DISTINCT ?alt; SEPARATOR="|") AS ?aliases)
  (SAMPLE(?n) AS ?sitelinks) (SAMPLE(?inc) AS ?inception) (SAMPLE(?ax) AS ?arxiv) (SAMPLE(?d) AS ?doi)
  (SAMPLE(?p) AS ?pwc) (SAMPLE(?wp) AS ?enwiki) (GROUP_CONCAT(DISTINCT ?devid; SEPARATOR="|") AS ?developers)
WHERE {
  ?item ` + path + ` wd:` + c.QID + ` .
  ?item rdfs:label ?l . FILTER(LANG(?l) = "en")
  OPTIONAL { ?item skos:altLabel ?alt . FILTER(LANG(?alt) = "en") }
  OPTIONAL { ?item wikibase:sitelinks ?n }
  OPTIONAL { ?item wdt:P571 ?inc }
  OPTIONAL { ?item wdt:P818 ?ax }
  OPTIONAL { ?item wdt:P356 ?d }
  OPTIONAL { ?item wdt:P12490 ?p }
  OPTIONAL { ?item wdt:P178 ?dev . BIND(STR(?dev) AS ?devid) }
  OPTIONAL { ?wp schema:about ?item ; schema:isPartOf <https://en.wikipedia.org/> }
}
GROUP BY ?item
ORDER BY DESC(?sitelinks) ?item`
	if limit > 0 {
		q += "\nLIMIT " + strconv.Itoa(limit)
	}
	return q
}

// WikidataURL is the GET URL of a query.
func WikidataURL(query string) string {
	return WikidataSPARQL + "?" + url.Values{"query": {query}}.Encode()
}

type sparqlResult struct {
	Results struct {
		Bindings []map[string]struct {
			Value string `json:"value"`
		} `json:"bindings"`
	} `json:"results"`
}

func qid(u string) string {
	if i := strings.LastIndex(u, "/entity/"); i >= 0 {
		return u[i+len("/entity/"):]
	}
	return ""
}

// WikidataRows reads the SPARQL JSON of a class.
func WikidataRows(b []byte, snap string, c WikidataClass) ([]Row, error) {
	var r sparqlResult
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, &ErrSchema{"wikidata", "the SPARQL result is not JSON: " + err.Error()}
	}
	var out []Row
	for _, b := range r.Results.Bindings {
		id, name := qid(b["item"].Value), clean(b["label"].Value)
		if !strings.HasPrefix(id, "Q") || name == "" || name == id {
			continue
		}
		row := Row{ID: id, Name: name, Kind: c.Kind, URL: "https://www.wikidata.org/wiki/" + id,
			SameAs: map[string]string{"wikidata": id}, Snapshot: snap, Labels: []string{c.QID + " " + c.Name}}
		for a := range strings.SplitSeq(b["aliases"].Value, "|") {
			if a = clean(a); a != "" && !strings.EqualFold(a, name) {
				row.Aliases = append(row.Aliases, a)
			}
		}
		sort.Strings(row.Aliases)
		if v := b["arxiv"].Value; v != "" {
			row.SameAs["arxiv"] = v
		}
		if v := b["doi"].Value; v != "" {
			row.SameAs["doi"] = strings.ToLower(v)
		}
		if v := b["pwc"].Value; v != "" {
			// P12490 holds the path after paperswithcode.com/, as the pwc ids of the dump do.
			row.SameAs["pwc"] = strings.Trim(pwcID(v), "/")
		}
		if v := b["enwiki"].Value; strings.HasPrefix(v, "https://en.wikipedia.org/wiki/") {
			t, err := url.PathUnescape(strings.TrimPrefix(v, "https://en.wikipedia.org/wiki/"))
			if err == nil {
				row.SameAs["wikipedia"] = t
			}
		}
		setFact(&row, "inception", date(b["inception"].Value), "P571")
		var devs []string
		for d := range strings.SplitSeq(b["developers"].Value, "|") {
			if q := qid(d); q != "" {
				devs = append(devs, q)
			}
		}
		if len(devs) > 0 {
			sort.Strings(devs)
			row.Links = map[string][]string{"developer": devs}
		}
		if n, err := strconv.Atoi(b["sitelinks"].Value); err == nil {
			setSignal(&row, "sitelinks", n)
		}
		out = append(out, row)
	}
	return out, nil
}

// harvestWikidata runs one query per class. An item in two classes keeps the kind of the first class, and its row
// lists both classes.
func harvestWikidata(ctx context.Context, r *Run, o Options) ([]Row, error) {
	share := 0
	if o.Limit > 0 {
		share = (o.Limit + len(WikidataClasses) - 1) / len(WikidataClasses)
	}
	byID := map[string]int{}
	var out []Row
	for _, c := range WikidataClasses {
		b, snap, err := r.Get(ctx, WikidataURL(WikidataQuery(c, share)), 256<<20)
		if err != nil {
			return nil, fmt.Errorf("wikidata class %s: %w", c.QID, err)
		}
		rows, err := WikidataRows(b, snap, c)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			if i, ok := byID[row.ID]; ok {
				out[i].Labels = append(out[i].Labels, row.Labels...)
				continue
			}
			byID[row.ID] = len(out)
			out = append(out, row)
		}
	}
	sortBy(out, "sitelinks")
	return keep(out, o.Limit), nil
}
