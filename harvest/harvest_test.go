// SPDX-License-Identifier: Apache-2.0

package harvest

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tamnd/ai-agent/cite"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// noProse fails when a row keeps text of a fixture that the adapters must drop: descriptions and abstracts.
func noProse(t *testing.T, rows []Row) {
	t.Helper()
	b, _ := json.Marshal(rows)
	if bytes.Contains(b, []byte("must not keep")) || bytes.Contains(b, []byte("Another description")) {
		t.Errorf("a row keeps a description or an abstract: %s", b)
	}
}

func byID(rows []Row) map[string]Row {
	m := map[string]Row{}
	for _, r := range rows {
		m[r.ID] = r
	}
	return m
}

func TestEpochModels(t *testing.T) {
	rows, err := EpochModels(fixture(t, "epoch-models.csv"), "b3:x", Options{})
	if err != nil {
		t.Fatal(err)
	}
	noProse(t, rows)
	if len(rows) != 3 {
		t.Fatalf("got %d rows, want 3 (the row with no name is dropped)", len(rows))
	}
	m := byID(rows)
	l := m["Llama 3.1-405B"]
	if l.Kind != "model" || l.SameAs["epoch"] != "Llama 3.1-405B" || l.Snapshot != "b3:x" {
		t.Errorf("Llama row: %+v", l)
	}
	for path, want := range map[string]any{
		"released":                   "2024-07-23",
		"params.total":               int64(405000000000),
		"training.compute_flop":      3.8e25,
		"training.hardware_quantity": int64(16384),
		"organization":               []string{"Meta AI"},
		"accessibility":              "Open weights (restricted use)",
	} {
		if got := l.Facts[path].V; !reflect.DeepEqual(got, want) {
			t.Errorf("fact %s = %#v, want %#v", path, got, want)
		}
	}
	if l.Facts["params.total"].Loc != "Parameters" {
		t.Errorf("a fact keeps its column as loc, got %q", l.Facts["params.total"].Loc)
	}
	if l.Signals["frontier"] != true || l.Signals["notable"] != true || !reflect.DeepEqual(l.Labels, []string{"SOTA improvement", "Training cost"}) {
		t.Errorf("signals %v, labels %v", l.Signals, l.Labels)
	}
	if a := m["AlexNet"]; a.URL != "https://papers.nips.cc/paper/4824" || a.Signals["frontier"] != nil {
		t.Errorf("AlexNet: url %q, signals %v", a.URL, a.Signals)
	}
	if f := m["Tiny fine-tune"]; !reflect.DeepEqual(f.Links["base_model"], []string{"Llama 3.1-405B"}) || f.Signals["notable"] != nil {
		t.Errorf("fine-tune links %v, signals %v", f.Links, f.Signals)
	}
	since, _ := EpochModels(fixture(t, "epoch-models.csv"), "", Options{Since: "2024-01-01"})
	limit, _ := EpochModels(fixture(t, "epoch-models.csv"), "", Options{Limit: 1})
	if len(since) != 2 || len(limit) != 1 || limit[0].ID != "Llama 3.1-405B" {
		t.Errorf("since gave %d rows, limit gave %v", len(since), limit)
	}
	if _, err := EpochModels([]byte("Name,Other\nx,y\n"), "", Options{}); !errors.As(err, new(*ErrSchema)) {
		t.Errorf("a file with other columns gives %v, want a schema error", err)
	}
}

func TestEpochHardware(t *testing.T) {
	rows, err := EpochHardware(fixture(t, "epoch-hardware.csv"), "", Options{})
	if err != nil {
		t.Fatal(err)
	}
	m := byID(rows)
	h := m["hardware/NVIDIA H100 SXM5 80GB"]
	if h.Kind != "hardware" || h.SameAs["epoch"] != "hardware/NVIDIA H100 SXM5 80GB" || h.URL == "" {
		t.Errorf("H100 row: %+v", h)
	}
	if h.Facts["specs.flops_fp16_tensor"].V != int64(989000000000000) || h.Facts["vendor"].V != "NVIDIA" || h.Facts["specs.tdp_w"].V != int64(700) {
		t.Errorf("H100 facts: %v", h.Facts)
	}
	if tpu := m["hardware/Google TPU v4"]; tpu.URL != "" || tpu.Facts["type"].V != "TPU" {
		t.Errorf("TPU row: %+v", tpu)
	}
}

func gz(b []byte) []byte {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	_, _ = w.Write(b)
	_ = w.Close()
	return buf.Bytes()
}

func TestPwCMethods(t *testing.T) {
	for _, in := range [][]byte{fixture(t, "pwc-methods.json"), gz(fixture(t, "pwc-methods.json"))} {
		rows, spam, err := PwCMethods(in, "b3:m", Options{})
		if err != nil {
			t.Fatal(err)
		}
		noProse(t, rows)
		if spam != 2 || len(rows) != 3 {
			t.Fatalf("got %d rows and %d spam, want 3 and 2", len(rows), spam)
		}
		if rows[0].ID != "method/dropout" || rows[1].ID != "method/1x1-convolution" {
			t.Errorf("rows are not the most used first: %s, %s", rows[0].ID, rows[1].ID)
		}
		d := rows[0]
		if d.Aliases != nil || d.SameAs["pwc"] != "method/dropout" || d.Facts["introduced_year"].V != 2014 {
			t.Errorf("dropout: %+v", d)
		}
		if !reflect.DeepEqual(d.Labels, []string{"General: Regularization"}) || d.Links["introduced_by"][0] != "paper/dropout-a-simple-way-to-prevent-neural" {
			t.Errorf("dropout labels %v, links %v", d.Labels, d.Links)
		}
		if ax := rows[1].Links["introduced_by_arxiv"]; !reflect.DeepEqual(ax, []string{"1312.4400"}) {
			t.Errorf("1x1 convolution arXiv link %v", ax)
		}
		if o := rows[2]; o.Facts["introduced_year"].V != nil || !reflect.DeepEqual(o.Aliases, []string{"An Old Thing"}) {
			t.Errorf("the year 2000 means unknown and is dropped: %+v", o)
		}
	}
}

func TestPwCDatasets(t *testing.T) {
	ds, tasks, err := PwCDatasets(fixture(t, "pwc-datasets.json"), "b3:d", Options{})
	if err != nil {
		t.Fatal(err)
	}
	noProse(t, ds)
	if len(ds) != 2 || len(tasks) != 2 {
		t.Fatalf("got %d datasets and %d tasks", len(ds), len(tasks))
	}
	if ds[0].ID != "dataset/imagenet" || ds[0].Facts["licence_name"].V != nil || ds[0].Fields["homepage"] != "https://image-net.org/" {
		t.Errorf("imagenet: %+v", ds[0])
	}
	if ds[1].Facts["licence_name"].V != "CC BY 4.0" || !reflect.DeepEqual(ds[1].Aliases, []string{"Microsoft Common Objects in Context"}) {
		t.Errorf("coco: %+v", ds[1])
	}
	if tasks[0].ID != "task/object-detection" || tasks[0].Signals["pwc_datasets"] != 2 || tasks[0].Kind != "task" {
		t.Errorf("tasks are not the most used first: %+v", tasks)
	}
	since, _, _ := PwCDatasets(fixture(t, "pwc-datasets.json"), "", Options{Since: "2010-01-01"})
	if len(since) != 1 || since[0].ID != "dataset/coco" {
		t.Errorf("since gave %v", since)
	}
}

func TestSpam(t *testing.T) {
	for name, spam := range map[string]bool{
		"Dropout":                   false,
		"Q-Learning":                false,
		"Mixture of Experts":        false,
		"1x1 Convolution":           false,
		"Call +1 888 555 0100 now":  true,
		"How to get a refund":       true,
		"[[FAQs]] Delta Airlines":   true,
		"Customer Service Helpline": true,
	} {
		if got := spamRE.MatchString(name); got != spam {
			t.Errorf("spam(%q) = %v, want %v", name, got, spam)
		}
	}
}

func TestAIIDIncidents(t *testing.T) {
	rows, err := AIIDIncidents(fixture(t, "aiid-incidents.csv"), "b3:a", Options{})
	if err != nil {
		t.Fatal(err)
	}
	noProse(t, rows)
	if len(rows) != 2 || rows[0].ID != "1" || rows[1].ID != "2" {
		t.Fatalf("rows %v, want incidents 1 and 2 in order", rows)
	}
	w := rows[1]
	if w.Kind != "incident" || w.URL != "https://incidentdatabase.ai/cite/2/" || w.SameAs["aiid"] != "2" || w.Signals["reports"] != 2 {
		t.Errorf("incident 2: %+v", w)
	}
	if !reflect.DeepEqual(w.Facts["entities.harmed"].V, []string{"warehouse-workers"}) || w.Facts["date"].V != "2018-12-05" {
		t.Errorf("incident 2 facts: %v", w.Facts)
	}
}

func TestAIIDPage(t *testing.T) {
	row, ok, err := AIIDPageRow(fixture(t, "aiid-page.json"), "b3:p")
	if err != nil || !ok {
		t.Fatal(ok, err)
	}
	noProse(t, []Row{row})
	csv, _ := AIIDIncidents(fixture(t, "aiid-incidents.csv"), "b3:p", Options{})
	want := csv[0]
	want.Signals["reports"] = 2
	for k, f := range want.Facts {
		f.Loc = "page-data"
		want.Facts[k] = f
	}
	if !reflect.DeepEqual(row, want) {
		t.Errorf("the page and the export disagree:\n%+v\n%+v", row, want)
	}
	if _, ok, err := AIIDPageRow(fixture(t, "aiid-missing.json"), ""); ok || err != nil {
		t.Errorf("a page with no incident gives %v, %v", ok, err)
	}
	if _, _, err := AIIDPageRow([]byte("<html>"), ""); err == nil {
		t.Error("a page that is not JSON gives no error")
	}
}

func TestArxivRecords(t *testing.T) {
	rows, token, deleted, err := ArxivRecords(fixture(t, "arxiv-oai.xml"), "b3:o")
	if err != nil {
		t.Fatal(err)
	}
	noProse(t, rows)
	if len(rows) != 2 || token != "7654321|1001" || deleted != 1 {
		t.Fatalf("got %d rows, token %q, %d deleted", len(rows), token, deleted)
	}
	a := rows[0]
	if a.Name != "Attention Is All You Need" || a.Kind != "work" || a.SameAs["arxiv"] != "1706.03762" || a.URL != "https://arxiv.org/abs/1706.03762" {
		t.Errorf("attention row: %+v", a)
	}
	if !reflect.DeepEqual(a.Fields["authors"], []string{"Ashish Vaswani", "Noam Shazeer"}) || a.Fields["licence_class"] != "D" || a.Fields["year"] != "2017" {
		t.Errorf("attention fields: %v", a.Fields)
	}
	if !reflect.DeepEqual(a.Labels, []string{"cs.CL", "cs.LG"}) || a.Facts["submitted"].V != "2017-06-12" || a.Facts["updated"].V != "2023-08-02" {
		t.Errorf("attention labels %v, facts %v", a.Labels, a.Facts)
	}
	b := rows[1]
	if b.SameAs["doi"] != "10.1000/xyz.1" || b.Fields["licence"] != "CC-BY-4.0" || b.Fields["licence_class"] != "C" || b.Fields["journal_ref"] == nil {
		t.Errorf("cc-by row: %+v", b)
	}
	rows, token, _, err = ArxivRecords(fixture(t, "arxiv-empty.xml"), "")
	if err != nil || rows != nil || token != "" {
		t.Errorf("noRecordsMatch gives %v, %q, %v", rows, token, err)
	}
	if _, _, _, err := ArxivRecords([]byte(`<OAI-PMH><error code="badArgument">bad</error></OAI-PMH>`), ""); err == nil {
		t.Error("an OAI error gives no error")
	}
}

func TestArxivLicence(t *testing.T) {
	for u, want := range map[string][2]string{
		"": {"arxiv-assumed", "D"},
		"http://arxiv.org/licenses/nonexclusive-distrib/1.0/": {"arxiv-nonexclusive-1.0", "D"},
		"http://arxiv.org/licenses/assumed-1991-2003/":        {"arxiv-assumed", "D"},
		"http://creativecommons.org/publicdomain/zero/1.0/":   {"CC0-1.0", "A"},
		"http://creativecommons.org/licenses/by/4.0/":         {"CC-BY-4.0", "C"},
		"https://creativecommons.org/licenses/by-sa/4.0/":     {"CC-BY-SA-4.0", "B"},
		"http://creativecommons.org/licenses/by-nc-sa/4.0/":   {"CC-BY-NC-SA-4.0", "D"},
		"http://creativecommons.org/licenses/by-nc-nd/4.0/":   {"CC-BY-NC-ND-4.0", "D"},
		"http://example.org/mine":                             {"unknown", "D"},
	} {
		if id, class := ArxivLicence(u); id != want[0] || class != want[1] {
			t.Errorf("ArxivLicence(%q) = %s %s, want %v", u, id, class, want)
		}
	}
}

func TestArxivURL(t *testing.T) {
	if s := ArxivSet("stat.ML"); s != "stat:stat:ML" {
		t.Errorf("set %q", s)
	}
	if u := ArxivURL("cs:cs:LG", "2026-11-01", ""); u != ArxivOAI+"?from=2026-11-01&metadataPrefix=arXiv&set=cs%3Acs%3ALG&verb=ListRecords" {
		t.Errorf("first page %q", u)
	}
	if u := ArxivURL("cs:cs:LG", "2026-11-01", "1|2"); u != ArxivOAI+"?resumptionToken=1%7C2&verb=ListRecords" {
		t.Errorf("next page %q", u)
	}
	root := t.TempDir()
	if cats, _ := arxivCategories(root); len(cats) != len(ArxivCategories) {
		t.Errorf("a tree with no crosswalk gives %v", cats)
	}
	_ = os.MkdirAll(filepath.Join(root, "crosswalk"), 0o755)
	_ = os.WriteFile(filepath.Join(root, ArxivCrosswalk), []byte("- {id: cs.LG, tags: [ABC123], match: exact}\n- {id: not a category}\n"), 0o644)
	if cats, err := arxivCategories(root); err != nil || !reflect.DeepEqual(cats, []string{"cs.LG"}) {
		t.Errorf("the crosswalk gives %v, %v", cats, err)
	}
}

func TestWikidataRows(t *testing.T) {
	c := WikidataClasses[0]
	rows, err := WikidataRows(fixture(t, "wikidata-llm.json"), "b3:w", c)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2 (an item with no label is dropped)", len(rows))
	}
	g := rows[0]
	if g.ID != "Q115564437" || g.Kind != "model" || g.SameAs["wikidata"] != "Q115564437" || g.SameAs["wikipedia"] != "ChatGPT" {
		t.Errorf("chatgpt: %+v", g)
	}
	if !reflect.DeepEqual(g.Aliases, []string{"Chat GPT", "Chat Generative Pre-trained Transformer"}) || !reflect.DeepEqual(g.Links["developer"], []string{"Q21708200"}) {
		t.Errorf("chatgpt aliases %v, links %v", g.Aliases, g.Links)
	}
	cl := rows[1]
	if cl.SameAs["arxiv"] != "2212.08073" || cl.SameAs["doi"] != "10.48550/arxiv.2212.08073" || cl.SameAs["pwc"] != "method/claude" || cl.SameAs["wikipedia"] != "Claude_(AI)" {
		t.Errorf("claude same_as %v", cl.SameAs)
	}
	if cl.Facts["inception"].V != "2023-03-14" || cl.Signals["sitelinks"] != 60 || cl.Aliases != nil {
		t.Errorf("claude: %+v", cl)
	}
	q := WikidataQuery(WikidataClasses[2], 10)
	if !strings.Contains(q, "?item wdt:P279+ wd:Q192776 .") || !strings.HasSuffix(q, "LIMIT 10") {
		t.Errorf("query:\n%s", q)
	}
}

func TestRobots(t *testing.T) {
	rb := parseRobots([]byte(`# comment
User-agent: Googlebot
Disallow: /

User-agent: *
Disallow: /sparql
Disallow: /w/
Allow: /w/index.php$
Disallow: /*.json$
Crawl-delay: 5
`), "aik")
	if rb.delay != 5*time.Second {
		t.Errorf("crawl delay %v", rb.delay)
	}
	for path, want := range map[string]bool{
		"/":                   true,
		"/sparql":             false,
		"/sparql?query=x":     false,
		"/w/api.php":          false,
		"/w/index.php":        true,
		"/w/index.php?x=1":    false,
		"/data/all.json":      false,
		"/data/all.json.gz":   true,
		"/wiki/Q42":           true,
		"/page-data/cite/1/x": true,
	} {
		if got := rb.allows(path); got != want {
			t.Errorf("allows(%q) = %v, want %v", path, got, want)
		}
	}
	own := parseRobots([]byte("User-agent: *\nDisallow: /\n\nUser-agent: aik\nAllow: /\n"), "aik")
	if !own.allows("/x") {
		t.Error("the group of aik does not win over *")
	}
	if !parseRobots(nil, "aik").allows("/anything") {
		t.Error("an empty robots.txt disallows")
	}
}

type fakeHost struct {
	mu      sync.Mutex
	robots  int // the status of robots.txt; 200 serves body
	body    string
	agents  []string
	paths   []string
	limited int // how many 429s to send first
}

func (h *fakeHost) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.agents = append(h.agents, r.Header.Get("User-Agent"))
	h.paths = append(h.paths, r.URL.RequestURI())
	if r.URL.Path == "/robots.txt" {
		w.WriteHeader(h.robots)
		_, _ = w.Write([]byte(h.body))
		return
	}
	if h.limited > 0 {
		h.limited--
		w.Header().Set("Retry-After", "7")
		w.WriteHeader(http.StatusTooManyRequests)
		return
	}
	_, _ = w.Write([]byte("ok " + r.URL.Path))
}

func testClient(gap time.Duration) (*Client, *[]time.Duration) {
	c := NewClient("v1.2.3", gap)
	var slept []time.Duration
	now := time.Unix(0, 0)
	c.Now = func() time.Time { return now }
	c.Sleep = func(d time.Duration) { slept = append(slept, d); now = now.Add(d) }
	return c, &slept
}

func TestClient(t *testing.T) {
	h := &fakeHost{robots: 200, body: "User-agent: *\nDisallow: /private\n"}
	srv := httptest.NewServer(h)
	defer srv.Close()
	c, slept := testClient(2 * time.Second)
	ctx := context.Background()
	for _, p := range []string{"/a", "/b"} {
		b, err := c.Get(ctx, srv.URL+p, 1<<20)
		if err != nil || string(b) != "ok "+p {
			t.Fatalf("GET %s: %q, %v", p, b, err)
		}
	}
	if _, err := c.Get(ctx, srv.URL+"/private/x", 1<<20); !errors.Is(err, ErrRobots) {
		t.Errorf("a disallowed path gives %v", err)
	}
	if _, err := c.Get(ctx, srv.URL+"/a", 2); err == nil {
		t.Error("a body over the limit gives no error")
	}
	for _, ua := range h.agents {
		if ua != "aik/1.2.3 (+https://github.com/tamnd/ai-agent)" || strings.Contains(ua, "@") {
			t.Errorf("the User-Agent is %q", ua)
		}
	}
	if !reflect.DeepEqual(h.paths, []string{"/robots.txt", "/a", "/b", "/a"}) {
		t.Errorf("the requests were %v: robots.txt is read once, and a disallowed URL is never fetched", h.paths)
	}
	if !reflect.DeepEqual(*slept, []time.Duration{2 * time.Second, 2 * time.Second, 2 * time.Second}) || c.Requests != 4 {
		t.Errorf("the client slept %v and sent %d requests", *slept, c.Requests)
	}
}

func TestClientBackoff(t *testing.T) {
	h := &fakeHost{robots: 404, limited: 2}
	srv := httptest.NewServer(h)
	defer srv.Close()
	c, slept := testClient(time.Second)
	b, err := c.Get(context.Background(), srv.URL+"/x", 1<<20)
	if err != nil || string(b) != "ok /x" {
		t.Fatalf("got %q, %v", b, err)
	}
	var back int
	for _, d := range *slept {
		if d == 7*time.Second {
			back++
		}
	}
	if back != 2 {
		t.Errorf("the client slept %v; it should wait the Retry-After of 7 s twice", *slept)
	}
	h2 := &fakeHost{robots: 503}
	srv2 := httptest.NewServer(h2)
	defer srv2.Close()
	if _, err := c.Get(context.Background(), srv2.URL+"/x", 1<<20); err == nil {
		t.Error("a robots.txt that answers 503 lets the client fetch")
	}
	if len(h2.paths) != 1 {
		t.Errorf("the client fetched %v from a host whose robots.txt failed", h2.paths)
	}
}

func testRun(t *testing.T, root, day string) *Run {
	t.Helper()
	return &Run{Root: root, Source: *SourceByID("epoch-ai"), Store: cite.Store{Dir: filepath.Join(root, ".raw")}, Date: day, Version: "v0.0.1"}
}

func TestFinish(t *testing.T) {
	root := t.TempDir()
	rows := func(names ...string) []Row {
		var out []Row
		for _, n := range names {
			r := Row{ID: n, Name: n, Kind: "model", SameAs: map[string]string{"epoch": n}, Snapshot: "b3:1"}
			setFact(&r, "released", "2024-01-01", "Publication date")
			out = append(out, r)
		}
		return out
	}
	r := testRun(t, root, "2026-11-01")
	rep, err := r.Finish(append(rows("b", "a"), Row{ID: "a", Name: "dup"}, Row{ID: "", Name: "x"}), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Rows != 2 || rep.New != 2 || rep.Partial || len(rep.Notes) != 1 {
		t.Errorf("first run: %+v", rep)
	}
	inv, _ := ReadInventory(root, "epoch-ai")
	if len(inv) != 2 || inv[0].ID != "a" || inv[0].Source != "epoch-ai" || inv[0].Date != "2026-11-01" || inv[0].SameAs["epoch"] != "a" {
		t.Errorf("inventory %+v", inv)
	}
	// A second run: a is unchanged with a new snapshot, b changes, c is new.
	next := rows("a", "b", "c")
	next[0].Snapshot = "b3:2"
	next[1].Facts["released"] = Fact{V: "2025-01-01"}
	r = testRun(t, root, "2026-11-08")
	rep, _ = r.Finish(next, Options{})
	if rep.New != 1 || rep.Changed != 1 || rep.Unchanged != 1 || rep.Removed != 0 {
		t.Errorf("second run: %+v", rep)
	}
	inv, _ = ReadInventory(root, "epoch-ai")
	if inv[0].Date != "2026-11-01" || inv[0].Snapshot != "b3:1" || inv[1].Date != "2026-11-08" {
		t.Errorf("an unchanged row keeps its old date and snapshot: %+v", inv[:2])
	}
	// A partial run removes nothing; a full run removes what the source dropped.
	rep, _ = testRun(t, root, "2026-11-09").Finish(rows("a"), Options{Limit: 1})
	if rep.Rows != 3 || rep.Removed != 0 || !rep.Partial {
		t.Errorf("partial run: %+v", rep)
	}
	rep, _ = testRun(t, root, "2026-11-10").Finish(rows("a"), Options{})
	if rep.Rows != 1 || rep.Removed != 2 {
		t.Errorf("full run: %+v", rep)
	}
	var rec Report
	b, _ := os.ReadFile(filepath.Join(root, ReportDir, "epoch-ai", "2026-11-10.json"))
	if err := json.Unmarshal(b, &rec); err != nil || rec.Date != "2026-11-10" || rec.Licence != "CC-BY-4.0" || rec.Class != "C" {
		t.Errorf("run record %s: %v", b, err)
	}
	srcs, _ := os.ReadFile(filepath.Join(root, SourcesFile))
	if strings.Count(string(srcs), "- id: epoch-ai") != 1 || !strings.Contains(string(srcs), "credit: Data from Epoch AI") {
		t.Errorf("sources.yaml has the row of the source once:\n%s", srcs)
	}
}

func TestOffline(t *testing.T) {
	h := &fakeHost{robots: 404}
	srv := httptest.NewServer(h)
	defer srv.Close()
	root := t.TempDir()
	r := testRun(t, root, "2026-11-01")
	r.Client, _ = testClient(0)
	b, snap, err := r.Get(context.Background(), srv.URL+"/data.csv", 1<<20)
	if err != nil || !strings.HasPrefix(snap, "b3:") {
		t.Fatal(snap, err)
	}
	if _, err := r.Finish(nil, Options{Limit: 1}); err != nil {
		t.Fatal(err)
	}
	m, _ := ReadManifest(root, "epoch-ai")
	if len(m) != 1 || m[0].URL != srv.URL+"/data.csv" || m[0].Hash != snap || m[0].Class != "C" {
		t.Errorf("manifest %+v", m)
	}
	off := testRun(t, root, "2026-11-02")
	off.Offline = true
	again, snap2, err := off.Get(context.Background(), srv.URL+"/data.csv", 1<<20)
	if err != nil || snap2 != snap || !bytes.Equal(again, b) {
		t.Errorf("offline read %q %s %v", again, snap2, err)
	}
	if _, _, err := off.Get(context.Background(), srv.URL+"/other", 1<<20); err == nil {
		t.Error("offline, a URL with no snapshot gives no error")
	}
	if n := len(h.paths); n != 2 {
		t.Errorf("the offline run fetched: %v", h.paths)
	}
}

func TestSources(t *testing.T) {
	for _, s := range Sources {
		if _, ok := Adapters[s.ID]; !ok {
			t.Errorf("source %s has no adapter", s.ID)
		}
		if s.Licence == "" || s.Class == "" || s.Checked == "" || s.Credit == "" || s.Access == "" {
			t.Errorf("source %s lacks a field of 13.2: %+v", s.ID, s)
		}
	}
	if len(Names()) != len(Sources) {
		t.Errorf("adapters %v, sources %d", Names(), len(Sources))
	}
}

func TestOpenAlexPage(t *testing.T) {
	rows, err := OpenAlexPage(fixture(t, "openalex-page.json"), "b3:o")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2 (a work with no title is dropped)", len(rows))
	}
	a := rows[0]
	if a.ID != "W2626778328" || a.Kind != "work" || a.Name != "Attention Is All You Need" || a.URL != "https://openalex.org/W2626778328" || a.Snapshot != "b3:o" {
		t.Errorf("attention: %+v", a)
	}
	want := map[string]string{"openalex": "W2626778328", "doi": "10.48550/arxiv.1706.03762", "arxiv": "1706.03762", "mag": "2626778328"}
	if !reflect.DeepEqual(a.SameAs, want) {
		t.Errorf("same_as %v", a.SameAs)
	}
	if !reflect.DeepEqual(a.Fields["authors"], []string{"Ashish Vaswani", "Noam Shazeer"}) || a.Fields["year"] != "2017" || a.Fields["type"] != "preprint" {
		t.Errorf("fields %v", a.Fields)
	}
	if !reflect.DeepEqual(a.Links["author"], []string{"A1", "A2"}) || !reflect.DeepEqual(a.Links["institution"], []string{"I1", "I2"}) {
		t.Errorf("links %v", a.Links)
	}
	if !reflect.DeepEqual(a.Labels, []string{"T10181 Natural Language Processing Techniques"}) {
		t.Errorf("labels %v", a.Labels)
	}
	if a.Facts["published"].V != "2017-06-12" || a.Signals["cited_by"] != 60000 {
		t.Errorf("facts %v, signals %v", a.Facts, a.Signals)
	}
	d := rows[1]
	if d.SameAs["doi"] != "10.1109/cvpr.2017.243" || d.SameAs["arxiv"] != "" || d.SameAs["mag"] != "" || d.Links != nil || d.Labels != nil {
		t.Errorf("densenet: %+v", d)
	}
	if _, err := OpenAlexPage([]byte(`{"error":"x"}`), ""); err == nil {
		t.Error("a page with no results gives no error")
	}
}

func TestOpenAlexYears(t *testing.T) {
	c, err := OpenAlexYears(fixture(t, "openalex-years.json"))
	if err != nil || !reflect.DeepEqual(c, map[int]int{2017: 78526, 2016: 49}) {
		t.Errorf("years %v %v", c, err)
	}
	if _, err := OpenAlexYears([]byte(`{"results":[]}`)); err == nil {
		t.Error("an answer with no group_by gives no error")
	}
	for n, want := range map[int]int{0: 0, 1: 1, 49: 1, 50: 1, 51: 2, 78526: 1571, 1 << 30: OpenAlexDepth} {
		if got := OpenAlexNeed(n); got != want {
			t.Errorf("OpenAlexNeed(%d) = %d, want %d", n, got, want)
		}
	}
}

func TestOpenAlexParts(t *testing.T) {
	root := t.TempDir()
	ps, err := openAlexParts(root)
	if err != nil || !reflect.DeepEqual(ps, []OpenAlexPart{{ID: "1702", Filter: "primary_topic.subfield.id:1702"}}) {
		t.Errorf("default parts %v %v", ps, err)
	}
	if err := os.MkdirAll(filepath.Join(root, "crosswalk"), 0o755); err != nil {
		t.Fatal(err)
	}
	cw := "rows:\n  - id: subfields/1702\n  - id: T10036\n  - id: 1702\n  - id: concept\n"
	if err := os.WriteFile(filepath.Join(root, OpenAlexCrosswalk), []byte(cw), 0o644); err != nil {
		t.Fatal(err)
	}
	ps, err = openAlexParts(root)
	want := []OpenAlexPart{{ID: "1702", Filter: "primary_topic.subfield.id:1702"}, {ID: "T10036", Filter: "primary_topic.id:T10036"}}
	if err != nil || !reflect.DeepEqual(ps, want) {
		t.Errorf("parts %v %v", ps, err)
	}
	u := OpenAlexPageURL(want[1], 2017, 3, 100)
	for _, s := range []string{"filter=primary_topic.id%3AT10036%2Cpublication_year%3A2017", "page=3", "per_page=100", "sort=cited_by_count%3Adesc", "select="} {
		if !strings.Contains(u, s) {
			t.Errorf("page URL %s lacks %s", u, s)
		}
	}
	if u := OpenAlexYearsURL(want[0], true); !strings.Contains(u, "publication_year%3A%3C2000") || !strings.Contains(u, "group_by=publication_year") {
		t.Errorf("years URL %s", u)
	}
	if strings.Contains(u, "mailto") || strings.Contains(u, "api_key") {
		t.Errorf("the URL sends an address or a key: %s", u)
	}
}

func TestHubCursor(t *testing.T) {
	// The cursor of the Link header of the live API, after jinaai/jina-embeddings-v2-base-en.
	want := "eyIkb3IiOlt7ImRvd25sb2FkcyI6MzU2MDQxLCJfaWQiOnsiJGd0IjoiNjUxNDYwODA2YTM4ZmI2NGQ1OTlhM2M2In19LHsiZG93bmxvYWRzIjp7IiRsdCI6MzU2MDQxfX0seyJkb3dubG9hZHMiOm51bGx9XX0="
	if got := HubCursor("downloads", 356041, "651460806a38fb64d599a3c6"); got != want {
		t.Errorf("cursor %s", got)
	}
	u := HubURL(HubParts[1], "likes", "abc=")
	for _, s := range []string{"https://huggingface.co/api/datasets?", "sort=likes", "direction=-1", "limit=1000", "cursor=abc%3D", "expand%5B%5D=paperswithcode_id"} {
		if !strings.Contains(u, s) {
			t.Errorf("URL %s lacks %s", u, s)
		}
	}
	if strings.Contains(HubURL(HubParts[0], "downloads", ""), "cursor") {
		t.Error("the first page has a cursor")
	}
}

func TestHubSelect(t *testing.T) {
	page, err := HubPage(fixture(t, "hub-models.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := HubPage([]byte(`{"error":"x"}`)); err == nil {
		t.Error("an error answer gives no error")
	}
	repos := map[string]HubRepo{}
	for _, r := range page[:6] {
		repos[r.ID] = r
	}
	cards, derived := HubSelect(repos)
	var ids []string
	for _, c := range cards {
		ids = append(ids, c.ID)
	}
	// the developer's fine-tune stays; a fine-tune with more downloads than its base stays; a quantization of a
	// base that is not over the thresholds stays.
	if want := []string{"acme/base", "acme/base-instruct", "other/base-ft", "x/orphan-q"}; !reflect.DeepEqual(ids, want) {
		t.Errorf("cards %v, want %v", ids, want)
	}
	want := map[string]map[string]int{"acme/base": {"quantized": 1}, "other/base-ft": {"quantized": 1}}
	if !reflect.DeepEqual(derived, want) {
		t.Errorf("derived %v", derived)
	}
	if b := repos["other/base-gguf"].Bases(); !reflect.DeepEqual(b, []HubBase{{Relation: "quantized", ID: "acme/base"}}) {
		t.Errorf("bases %v", b)
	}
	row := HubRow(HubParts[0], repos["acme/base"], "b3:h")
	if row.ID != "acme/base" || row.Kind != "model" || row.URL != "https://huggingface.co/acme/base" || row.SameAs["hf"] != "acme/base" {
		t.Errorf("row %+v", row)
	}
	if row.Fields["licence"] != "apache-2.0" || row.Fields["gated"] != "auto" || row.Fields["commit"] != "aaa" || row.Fields["library"] != "transformers" {
		t.Errorf("fields %v", row.Fields)
	}
	if !reflect.DeepEqual(row.Links, map[string][]string{"author": {"acme"}, "dataset": {"acme/corpus"}, "paper": {"arxiv:2401.00001", "doi:10.57967/hf/1"}}) {
		t.Errorf("links %v", row.Links)
	}
	if row.Facts["created"].V != "2024-01-15" || row.Signals["downloads_30d"] != 100000 || row.Signals["likes"] != 900 || !reflect.DeepEqual(row.Labels, []string{"text-generation"}) {
		t.Errorf("row %+v", row)
	}
	ds := HubRow(HubParts[1], HubRepo{OID: "1", ID: "HuggingFaceFW/fineweb", PwC: "fineweb"}, "")
	if ds.ID != "datasets/HuggingFaceFW/fineweb" || ds.URL != "https://huggingface.co/datasets/HuggingFaceFW/fineweb" || ds.SameAs["hf_dataset"] != "HuggingFaceFW/fineweb" || ds.SameAs["pwc"] != "dataset/fineweb" {
		t.Errorf("dataset row %+v", ds)
	}
}
