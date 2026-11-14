// SPDX-License-Identifier: Apache-2.0

// Package audit is the daily audit of the content repository (spec 20.5, audit.yml): the findings of every rule, the
// state of the citation records, the size alarm of 20.3, and, weekly, the check of the external links (14.9). Its
// report, reports/audit.json, has no date in it, so that it changes only when the state does.
package audit

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/tamnd/ai-agent/check"
	"github.com/tamnd/ai-agent/cite"
	"github.com/tamnd/ai-agent/harvest"
	"github.com/tamnd/ai-agent/node"
)

// The size alarm of 20.3: the repository and one directory.
const (
	MaxRepo = 6 << 30
	MaxDir  = 50 << 20
)

// File is where the report goes, under the root of the content repository.
const File = "reports/audit.json"

// Report is reports/audit.json.
type Report struct {
	Check CheckSummary `json:"check"`
	Cite  CiteSummary  `json:"cite"`
	Size  Size         `json:"size"`
	Links *Links       `json:"links,omitempty"`
}

// CheckSummary counts the findings of aik check by severity and by rule, and lists the findings that block.
type CheckSummary struct {
	Findings int             `json:"findings"`
	Block    int             `json:"block"`
	Level    int             `json:"level"`
	Warn     int             `json:"warn"`
	ByRule   map[string]int  `json:"by_rule"`
	Blocking []check.Finding `json:"blocking"`
}

// CiteSummary counts the citation records of prov/cite (13.6, 15.4) and lists those that fail: a source that the
// checks of 13.6 do not find at its locator, or a verdict other than supported.
type CiteSummary struct {
	Records  int            `json:"records"`
	Pass     int            `json:"pass"`
	Verdicts map[string]int `json:"verdicts"`
	Failing  []CiteFail     `json:"failing"`
}

// CiteFail is a record that fails.
type CiteFail struct {
	Tag     string `json:"tag"`
	Claim   string `json:"claim"`
	Work    string `json:"work"`
	Locator string `json:"locator"`
	Why     string `json:"why"`
}

// Size is the size alarm: the bytes of the repository, without .git and the build outputs, and each directory whose own
// files pass the limit.
type Size struct {
	Bytes int64 `json:"bytes"`
	Alarm bool  `json:"alarm"`
	Big   []Dir `json:"big_dirs"`
}

// Dir is a directory and the bytes of its own files.
type Dir struct {
	Path  string `json:"path"`
	Bytes int64  `json:"bytes"`
}

// Links is the check of the external links of the nodes.
type Links struct {
	Checked int    `json:"checked"`
	Dead    []Link `json:"dead"`
	// Unsure are the links that answered with an error that is not a sure sign of a dead page, such as 403 from a
	// host that turns away clients, or that robots.txt does not let aik read.
	Unsure []Link `json:"unsure"`
}

// Link is an external URL, the first file that has it, and what the check found.
type Link struct {
	URL    string `json:"url"`
	File   string `json:"file"`
	Status int    `json:"status,omitempty"`
	Error  string `json:"error,omitempty"`
}

// Checks summarizes the findings of check.Run.
func Checks(fs []check.Finding) CheckSummary {
	s := CheckSummary{Findings: len(fs), ByRule: map[string]int{}, Blocking: []check.Finding{}}
	for _, f := range fs {
		s.ByRule[f.Rule]++
		switch f.Severity {
		case check.Block:
			s.Block++
			s.Blocking = append(s.Blocking, f)
		case check.Level:
			s.Level++
		default:
			s.Warn++
		}
	}
	sort.SliceStable(s.Blocking, func(i, j int) bool {
		a, b := s.Blocking[i], s.Blocking[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Rule < b.Rule
	})
	return s
}

// Cites summarizes the citation records of the index.
func Cites(ix *cite.Index) CiteSummary {
	s := CiteSummary{Verdicts: map[string]int{}, Failing: []CiteFail{}}
	tags := make([]string, 0, len(ix.Records))
	for t := range ix.Records {
		tags = append(tags, t)
	}
	sort.Strings(tags)
	for _, t := range tags {
		ids := make([]string, 0, len(ix.Records[t]))
		for id := range ix.Records[t] {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			r := ix.Records[t][id]
			s.Records++
			if r.Pass {
				s.Pass++
			}
			v := r.Verdict
			if v == "" {
				v = "none"
			}
			s.Verdicts[v]++
			why := ""
			switch {
			case !r.Pass:
				why = "the checks of 13.6 do not find the claim at the locator"
			case r.Verdict == cite.Unsupported || r.Verdict == cite.Contradicted || r.Verdict == cite.Unreadable:
				why = "the verifier says " + r.Verdict
			}
			if why != "" {
				s.Failing = append(s.Failing, CiteFail{Tag: t, Claim: r.Claim, Work: r.Work, Locator: r.Locator, Why: why})
			}
		}
	}
	return s
}

// skipDir tells if a directory is outside the size of the repository: git, the build outputs, and caches.
func skipDir(name string) bool {
	switch name {
	case "build", "node_modules", "__pycache__":
		return true
	}
	return strings.HasPrefix(name, ".") && name != "."
}

// Sizes walks the repository and returns its size and each directory whose own files pass maxDir; the alarm is on when
// the repository passes maxRepo or a directory passes maxDir.
func Sizes(root string, maxRepo, maxDir int64) (Size, error) {
	own := map[string]int64{}
	var s Size
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != root && skipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		s.Bytes += info.Size()
		rel, _ := filepath.Rel(root, filepath.Dir(p))
		own[filepath.ToSlash(rel)] += info.Size()
		return nil
	})
	if err != nil {
		return s, err
	}
	s.Big = []Dir{}
	for d, b := range own {
		if b > maxDir {
			s.Big = append(s.Big, Dir{Path: d, Bytes: b})
		}
	}
	sort.Slice(s.Big, func(i, j int) bool { return s.Big[i].Path < s.Big[j].Path })
	s.Alarm = s.Bytes > maxRepo || len(s.Big) > 0
	return s, nil
}

var urlRE = regexp.MustCompile(`https?://[^\s<>"'` + "`" + `\])}]+`)

// URLs are the external URLs of the markdown and YAML files of the nodes, each with the first file that has it, by
// path from root.
func URLs(c *node.Corpus) (map[string]string, error) {
	out := map[string]string{}
	for _, n := range c.Nodes {
		err := filepath.WalkDir(n.Dir, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if p != n.Dir && skipDir(d.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			if ext := filepath.Ext(p); ext != ".md" && ext != ".yaml" {
				return nil
			}
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(c.Root, p)
			for _, u := range urlRE.FindAllString(string(b), -1) {
				u = strings.TrimRight(u, ".,;:!?*_")
				if f, ok := out[u]; !ok || filepath.ToSlash(rel) < f {
					out[u] = filepath.ToSlash(rel)
				}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// CheckLinks asks for each URL with the client, with workers requests at a time; the client keeps the gap between two
// requests to one host. A 404 or a 410, or a host that does not resolve, is dead; any other error is unsure.
func CheckLinks(ctx context.Context, cl *harvest.Client, urls map[string]string, workers int) *Links {
	list := make([]string, 0, len(urls))
	for u := range urls {
		list = append(list, u)
	}
	sort.Strings(list)
	l := &Links{Checked: len(list), Dead: []Link{}, Unsure: []Link{}}
	var mu sync.Mutex
	jobs := make(chan string)
	var wg sync.WaitGroup
	for range max(workers, 1) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for u := range jobs {
				code, err := cl.Status(ctx, u)
				k := Link{URL: u, File: urls[u], Status: code}
				var dns *net.DNSError
				dead := code == http.StatusNotFound || code == http.StatusGone || errors.As(err, &dns) && dns.IsNotFound
				if err != nil {
					k.Error = err.Error()
				}
				mu.Lock()
				switch {
				case dead:
					l.Dead = append(l.Dead, k)
				case err != nil || code >= 400:
					l.Unsure = append(l.Unsure, k)
				}
				mu.Unlock()
			}
		}()
	}
	for _, u := range list {
		jobs <- u
	}
	close(jobs)
	wg.Wait()
	sort.Slice(l.Dead, func(i, j int) bool { return l.Dead[i].URL < l.Dead[j].URL })
	sort.Slice(l.Unsure, func(i, j int) bool { return l.Unsure[i].URL < l.Unsure[j].URL })
	return l
}

// Write writes the report as indented JSON, ending with a newline.
func Write(path string, r *Report) error {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}
