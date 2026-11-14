// SPDX-License-Identifier: Apache-2.0

package audit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tamnd/ai-agent/check"
	"github.com/tamnd/ai-agent/cite"
	"github.com/tamnd/ai-agent/harvest"
	"github.com/tamnd/ai-agent/node"
)

func write(t *testing.T, path string, n int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Repeat("x", n)), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestChecks(t *testing.T) {
	s := Checks([]check.Finding{
		{Rule: "T02", Severity: check.Block, Path: "b.md"},
		{Rule: "G03", Severity: check.Level, Path: "a.md"},
		{Rule: "T02", Severity: check.Block, Path: "a.md", Line: 2},
		{Rule: "V01", Severity: check.Warn, Path: "a.md"},
	})
	if s.Findings != 4 || s.Block != 2 || s.Level != 1 || s.Warn != 1 || s.ByRule["T02"] != 2 {
		t.Errorf("counts: %+v", s)
	}
	if len(s.Blocking) != 2 || s.Blocking[0].Path != "a.md" || s.Blocking[1].Path != "b.md" {
		t.Errorf("blocking, sorted by path: %+v", s.Blocking)
	}
	if s := Checks(nil); s.Blocking == nil || s.ByRule == nil {
		t.Error("an empty summary has empty lists, not null")
	}
}

func TestCites(t *testing.T) {
	ix := &cite.Index{Records: map[string]map[string]cite.Record{
		"BBBBBB": {"1": {Claim: "c-a", Work: "w", Pass: true, Verdict: cite.Supported}},
		"AAAAAA": {
			"1": {Claim: "c-b", Work: "w", Locator: "p. 3", Pass: false},
			"2": {Claim: "c-c", Work: "w", Pass: true, Verdict: cite.Contradicted},
			"3": {Claim: "c-d", Work: "w", Pass: true},
		},
	}}
	s := Cites(ix)
	if s.Records != 4 || s.Pass != 3 || s.Verdicts["supported"] != 1 || s.Verdicts["none"] != 2 || s.Verdicts["contradicted"] != 1 {
		t.Errorf("counts: %+v", s)
	}
	if len(s.Failing) != 2 || s.Failing[0].Claim != "c-b" || !strings.Contains(s.Failing[1].Why, "contradicted") {
		t.Errorf("failing: %+v", s.Failing)
	}
}

func TestSizes(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "nodes", "a", "node.md"), 100)
	write(t, filepath.Join(root, "nodes", "a", "big.bin"), 2000)
	write(t, filepath.Join(root, "nodes", "b", "node.md"), 100)
	write(t, filepath.Join(root, ".git", "pack"), 1<<20)
	write(t, filepath.Join(root, "build", "site.tar"), 1<<20)
	s, err := Sizes(root, 1<<20, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if s.Bytes != 2200 || !s.Alarm || len(s.Big) != 1 || s.Big[0].Path != "nodes/a" || s.Big[0].Bytes != 2100 {
		t.Errorf("Sizes = %+v", s)
	}
	if s, _ := Sizes(root, 2000, 1<<20); !s.Alarm || len(s.Big) != 0 {
		t.Errorf("a repository over its limit: %+v", s)
	}
	if s, _ := Sizes(root, 1<<20, 1<<20); s.Alarm {
		t.Errorf("no alarm under both limits: %+v", s)
	}
}

func TestLinks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok":
		case "/gone":
			w.WriteHeader(http.StatusNotFound)
		case "/robots.txt":
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusForbidden)
		}
	}))
	defer srv.Close()
	c := &node.Corpus{Root: t.TempDir()}
	dir := filepath.Join(c.Root, "nodes", "t")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	md := "See [the page](" + srv.URL + "/ok), " + srv.URL + "/gone. And <" + srv.URL + "/blocked>.\n"
	if err := os.WriteFile(filepath.Join(dir, "node.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "impl.py"), []byte(srv.URL+"/not-read\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c.Nodes = []*node.Node{{Dir: dir}}
	urls, err := URLs(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(urls) != 3 || urls[srv.URL+"/gone"] != "nodes/t/node.md" {
		t.Fatalf("URLs = %v", urls)
	}
	cl := harvest.NewClient("v0", 0)
	cl.Sleep = func(time.Duration) {}
	l := CheckLinks(context.Background(), cl, urls, 2)
	if l.Checked != 3 || len(l.Dead) != 1 || l.Dead[0].URL != srv.URL+"/gone" || l.Dead[0].Status != 404 {
		t.Errorf("dead: %+v", l)
	}
	if len(l.Unsure) != 1 || l.Unsure[0].Status != 403 {
		t.Errorf("unsure: %+v", l.Unsure)
	}
}
