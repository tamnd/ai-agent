// SPDX-License-Identifier: Apache-2.0

// Package cite is the citation check of spec 7.5 and 13.6: it resolves the source of each claim to a stored snapshot
// of the cited version, finds the place that the locator names, and checks that the numbers and the key terms of
// the claim are there. Its records say which claims are grounded (G2), and carry the verifier's verdict (G3).
package cite

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/tamnd/ai-agent/claims"
)

// CheckerVersion is the version of the rules of Check. A change of it changes every key, so every claim is checked
// again.
const CheckerVersion = "aik-cite-1"

// MinTerms is the share of the key terms of a claim that its span or the section of its span must hold (Default,
// spec 7.5). The verifier of G3 reads the meaning; this step only finds a passage that is about the claim.
const MinTerms = 0.75

// Work is how a work card says what to fetch: the cited version, its URL and its format (spec 13.6).
type Work struct {
	Slug    string `json:"slug"`
	Version string `json:"version"`
	URL     string `json:"url"`
	Format  string `json:"format"` // pdf, html or none
	Licence string `json:"licence_class,omitempty"`
	// Authors are the names of the authors, which a claim may name without the source holding them.
	Authors []string `json:"authors,omitempty"`
}

// Snapshot is a line of manifests/works.jsonl: the stored copy of the cited version of a work.
type Snapshot struct {
	Work      string `json:"work"`
	Version   string `json:"version"`
	URL       string `json:"url"`
	Format    string `json:"format"`
	Hash      string `json:"hash"`
	Size      int    `json:"size"`
	Fetched   string `json:"fetched"`
	Extractor string `json:"extractor"`
}

// Record is the result of the check of one source of one claim: a line of prov/cite/<tag>.jsonl. It holds offsets
// into the snapshot and never text of the source (C01).
type Record struct {
	Claim    string `json:"claim"`
	Work     string `json:"work"`
	Locator  string `json:"locator"`
	Key      string `json:"key"`
	Snapshot string `json:"snapshot"`
	Checked  string `json:"checked"`
	Spans    []Span `json:"spans,omitempty"`
	// Steps of 13.6: the locator, the numbers, the key terms.
	Located      bool     `json:"located"`
	NotLocated   []string `json:"not_located,omitempty"`
	Numbers      int      `json:"numbers"`
	NumbersMiss  []string `json:"numbers_missing,omitempty"`
	Terms        int      `json:"terms"`
	TermsFound   int      `json:"terms_found"`
	TermsMissing []string `json:"terms_missing,omitempty"`
	Pass         bool     `json:"pass"`
	// The hashes of the claim as it is judged and of the passage and section that the verifier reads (aik cite
	// passages), so that a verdict outlives a new extractor or checker that leaves both as they were (Carry).
	Judged  string `json:"judged,omitempty"`
	Passage string `json:"passage,omitempty"`
	// The verifier of 15.4 reads the claim and the span and gives supported, unsupported, contradicted or unreadable.
	Verdict  string `json:"verdict,omitempty"`
	Verifier string `json:"verifier,omitempty"`
	Run      string `json:"run,omitempty"`
}

// The verdicts of the verifier (spec 15.4).
const (
	Supported    = "supported"
	Unsupported  = "unsupported"
	Contradicted = "contradicted"
	Unreadable   = "unreadable"
)

// Judged is what the verifier of 15.4 judges of a claim, and so what its record is keyed on: the text, and the type and
// the scope when the claim states them. A change of the scope makes the verdict stale, as a change of the text does.
func Judged(c *claims.Claim) string {
	if c.Scope.Empty() {
		return c.Text
	}
	b, _ := json.Marshal(c.Scope)
	return c.Text + "\x00" + c.Type + "\x00" + string(b)
}

// Key is the cache key of a check (spec 7.5): the claim as it is judged, the snapshot, the locator, and the versions of
// the extractor and of the checker.
func Key(claim, snapshot, locator, extractor string) string {
	return Hash([]byte(strings.Join([]string{claim, snapshot, locator, extractor, CheckerVersion}, "\x00")))[:32]
}

// Check runs the steps of 13.6 for one source of a claim against the text of its snapshot. The words of exempt, as
// the names of the authors of the work, are not key terms.
func Check(c *claims.Claim, e claims.Evidence, d *Doc, snap Snapshot, exempt ...string) Record {
	r := Record{Claim: c.ID, Work: e.Work, Locator: e.Locator, Snapshot: snap.Hash, Key: Key(Judged(c), snap.Hash, e.Locator, snap.Extractor)}
	refs, err := ParseLocator(e.Locator)
	if err != nil {
		r.NotLocated = []string{err.Error()}
		return r
	}
	spans, fails := Resolve(d, refs)
	r.Spans, r.NotLocated = Merge(spans), fails
	r.Located = len(fails) == 0
	if !r.Located {
		return r
	}
	var span strings.Builder
	for _, s := range r.Spans {
		span.WriteString(d.Text[s.Start:s.End])
		span.WriteByte('\n')
	}
	in := Numbers(span.String())
	for _, n := range Numbers(ownIDs(c.Text, d)) {
		r.Numbers++
		if !Found(n, in) {
			r.NumbersMiss = append(r.NumbersMiss, n.Text)
		}
	}
	// The key terms may be in the span or in the section that holds it (spec 7.5).
	ctx := span.String()
	for _, s := range r.Spans {
		if h := d.section(s.Start); h != nil {
			ctx += "\n" + d.Text[h.Start:h.End]
		}
	}
	v := NewVocab(ctx)
	terms := Terms(c.Text, exempt...)
	r.Terms = len(terms)
	for _, t := range terms {
		if v.Has(t) {
			r.TermsFound++
		} else {
			r.TermsMissing = append(r.TermsMissing, t)
		}
	}
	r.Pass = len(r.NumbersMiss) == 0 && (r.Terms == 0 || float64(r.TermsFound) >= MinTerms*float64(r.Terms))
	r.Judged = Hash([]byte(Judged(c)))[:32]
	if r.Pass {
		r.Passage = PassageHash(d, r.Spans)
	}
	return r
}

// PassageHash is the hash of what the verifier reads of the spans: the passage and its section, as Passage gives them.
func PassageHash(d *Doc, spans []Span) string {
	p, sec := Passage(d, spans)
	return Hash([]byte(p + "\x00" + sec))[:32]
}

// HashlessExtractorVersion is the last extractor whose records have no hashes of the judged text and passage; Carry
// reads them from the key and the spans of such a record instead.
const HashlessExtractorVersion = "aik-doc-3"

// Carry tells if the verdict of the old record of a source holds for the new one, r, whose key differs: the claim as
// it is judged, the work, the locator and the snapshot are the same, and so is the passage and section that the
// verifier read, so that only the extractor or the checker changed and not what was judged. d is the text of the
// snapshot now and prev the text that the old record was made on, if the store still had it; nil means d. When the
// hashes differ, the two passages are read and compared with no regard to white space, as an extractor that only
// joins runs of spaces leaves the words that the verifier read as they were. A record written before the hashes were
// stored is matched by its key under HashlessExtractorVersion, and its passage is read from its spans.
func Carry(old, r Record, c *claims.Claim, d, prev *Doc, snap Snapshot) bool {
	if old.Verdict == "" || !r.Pass || r.Passage == "" || old.Work != r.Work || old.Locator != r.Locator || old.Snapshot != r.Snapshot {
		return false
	}
	if prev == nil {
		prev = d
	}
	judged := old.Judged
	if judged == "" {
		v := snap.Extractor
		if k := strings.LastIndexByte(v, '+'); k >= 0 {
			v = v[:k+1] + HashlessExtractorVersion
		}
		if v == snap.Extractor || old.Key != Key(Judged(c), snap.Hash, old.Locator, v) {
			return false
		}
		judged = r.Judged
	}
	if judged != r.Judged {
		return false
	}
	if old.Passage == r.Passage {
		return true
	}
	spans := old.Spans
	if old.Passage == "" {
		// The extractor before the hashes put a heading at the top of a page on the form feed before it: a span from
		// there starts at the heading, and a span up to there ends at it.
		spans = slices.Clone(old.Spans)
		for i := range spans {
			sp := &spans[i]
			for sp.Start < sp.End && sp.Start < len(prev.Text) && blank(prev.Text[sp.Start]) {
				sp.Start++
			}
			for sp.End > sp.Start && sp.End < len(prev.Text) && blank(prev.Text[sp.End]) {
				sp.End++
			}
		}
		if prev == d && PassageHash(d, spans) == r.Passage {
			return true
		}
	} else if PassageHash(prev, spans) != old.Passage {
		return false // the old text is not the one that the record was made on
	}
	p0, s0 := Passage(prev, spans)
	p1, s1 := Passage(d, r.Spans)
	return squeeze(p0) == squeeze(p1) && squeeze(s0) == squeeze(s1)
}

// squeeze drops the white space of a text.
func squeeze(s string) string {
	return strings.Join(strings.Fields(s), "")
}

// blank tells if the byte is a space, a tab, a new line or a form feed.
func blank(b byte) bool { return strings.IndexByte(" \t\n\f", b) >= 0 }

// section is the deepest heading whose text holds the offset: in a legal text, the recital, the article or the annex
// that holds it, so that a paragraph of an article is read with its article.
func (d *Doc) section(off int) *Head {
	if p := d.provisionAt(off); p != nil {
		return &Head{Num: provisionTitle(*p), Start: p.Start, End: p.End}
	}
	var best *Head
	for i := range d.Heads {
		h := &d.Heads[i]
		if h.Start <= off && off < h.End && (best == nil || h.Start >= best.Start) {
			best = h
		}
	}
	return best
}

// MaxSection is how much of the section around a span the verifier reads, in bytes.
const MaxSection = 12000

// Passage gives the text of the spans of a record and of the sections that hold them, which is what the verifier
// reads (15.4). A long section is cut around the span.
func Passage(d *Doc, spans []Span) (passage, section string) {
	var p, s strings.Builder
	seen := map[int]bool{}
	for _, sp := range spans {
		if sp.Start < 0 || sp.End > len(d.Text) || sp.Start > sp.End {
			continue
		}
		p.WriteString(strings.TrimSpace(d.Text[sp.Start:sp.End]))
		p.WriteString("\n\n")
		h := d.section(sp.Start)
		if h == nil || seen[h.Start] || (h.Start >= sp.Start && h.End <= sp.End) {
			continue
		}
		seen[h.Start] = true
		a, b := h.Start, h.End
		if b-a > MaxSection {
			mid := (sp.Start + sp.End) / 2
			a, b = max(h.Start, mid-MaxSection/2), min(h.End, mid+MaxSection/2)
			for a > h.Start && !utf8.RuneStart(d.Text[a]) {
				a--
			}
			for b < h.End && !utf8.RuneStart(d.Text[b]) {
				b++
			}
		}
		s.WriteString(strings.TrimSpace(d.Text[a:b]))
		s.WriteString("\n\n")
	}
	return strings.TrimSpace(p.String()), strings.TrimSpace(s.String())
}

// Verdicts are the verdicts that the verifier may give.
var Verdicts = []string{Supported, Unsupported, Contradicted, Unreadable}

// Why says in one line why a record does not pass.
func (r Record) Why() string {
	switch {
	case r.Pass:
		return ""
	case !r.Located:
		return "the locator does not resolve: " + strings.Join(r.NotLocated, "; ")
	case len(r.NumbersMiss) > 0:
		return "the numbers " + strings.Join(r.NumbersMiss, ", ") + " are not at the located place"
	}
	return fmt.Sprintf("%d of %d key terms are at the located place; missing: %s", r.TermsFound, r.Terms, strings.Join(r.TermsMissing, ", "))
}

// The files of the content repository that hold the snapshots and the records.
const (
	ManifestFile = "manifests/works.jsonl"
	RecordDir    = "prov/cite"
)

// Index is what the build knows about the citation check: the snapshots of the works and the records of the claims.
type Index struct {
	Snapshots map[string]Snapshot          // by work
	Records   map[string]map[string]Record // by node tag, then by RecordID
}

// RecordID names the record of a source of a claim.
func RecordID(claim, work, locator string) string { return claim + "\x00" + work + "\x00" + locator }

// Load reads the manifest and the records of a content root. Missing files give an empty index.
func Load(root string) (*Index, error) {
	ix := &Index{Snapshots: map[string]Snapshot{}, Records: map[string]map[string]Record{}}
	if err := readLines(filepath.Join(root, ManifestFile), func(b []byte) error {
		var s Snapshot
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		ix.Snapshots[s.Work] = s
		return nil
	}); err != nil {
		return nil, err
	}
	files, _ := filepath.Glob(filepath.Join(root, RecordDir, "*.jsonl"))
	for _, f := range files {
		tag := strings.TrimSuffix(filepath.Base(f), ".jsonl")
		m := map[string]Record{}
		if err := readLines(f, func(b []byte) error {
			var r Record
			if err := json.Unmarshal(b, &r); err != nil {
				return err
			}
			m[RecordID(r.Claim, r.Work, r.Locator)] = r
			return nil
		}); err != nil {
			return nil, err
		}
		ix.Records[tag] = m
	}
	return ix, nil
}

func readLines(path string, f func([]byte) error) error {
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	sc := bufio.NewScanner(bytes.NewReader(b))
	sc.Buffer(make([]byte, 0, 64*1024), 8<<20)
	for i := 1; sc.Scan(); i++ {
		if len(bytes.TrimSpace(sc.Bytes())) == 0 {
			continue
		}
		if err := f(sc.Bytes()); err != nil {
			return fmt.Errorf("%s:%d: %w", path, i, err)
		}
	}
	return sc.Err()
}

// Current gives the record of a source of a claim when its key matches the claim, the locator and the snapshot of
// the work as they are now. A record of an older text, locator or snapshot is stale and counts for nothing.
func (ix *Index) Current(tag string, c *claims.Claim, e claims.Evidence) (Record, bool) {
	if ix == nil {
		return Record{}, false
	}
	r, ok := ix.Records[tag][RecordID(c.ID, e.Work, e.Locator)]
	if !ok {
		return Record{}, false
	}
	s, ok := ix.Snapshots[e.Work]
	if !ok || r.Key != Key(Judged(c), s.Hash, e.Locator, s.Extractor) {
		return Record{}, false
	}
	return r, true
}

// Grounded tells if a source of a claim passes the citation check (G2).
func (ix *Index) Grounded(tag string, c *claims.Claim, e claims.Evidence) bool {
	r, ok := ix.Current(tag, c, e)
	return ok && r.Pass
}

// WriteRecords writes the records of a node, sorted, one per line.
func WriteRecords(root, tag string, rs []Record) error {
	slices.SortFunc(rs, func(a, b Record) int {
		return strings.Compare(RecordID(a.Claim, a.Work, a.Locator), RecordID(b.Claim, b.Work, b.Locator))
	})
	var b bytes.Buffer
	for _, r := range rs {
		line, err := json.Marshal(r)
		if err != nil {
			return err
		}
		b.Write(line)
		b.WriteByte('\n')
	}
	p := filepath.Join(root, RecordDir, tag+".jsonl")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, b.Bytes(), 0o644)
}

// WriteManifest writes the snapshots, sorted by work.
func WriteManifest(root string, snaps map[string]Snapshot) error {
	works := make([]string, 0, len(snaps))
	for w := range snaps {
		works = append(works, w)
	}
	slices.Sort(works)
	var b bytes.Buffer
	for _, w := range works {
		line, err := json.Marshal(snaps[w])
		if err != nil {
			return err
		}
		b.Write(line)
		b.WriteByte('\n')
	}
	p := filepath.Join(root, ManifestFile)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, b.Bytes(), 0o644)
}

// actIDRE is the number of an act or a document: "2024/1689", "2021/0106(COD)".
var actIDRE = regexp.MustCompile(`\b[0-9]{2,4}/[0-9]{1,4}\b`)

// ownIDs takes out of a claim the numbers that name the document itself, as "Regulation (EU) 2026/1744 states that…"
// cited to a recital of that regulation: they are on its title page, not at the located place. A number of the
// claim that is not on the title page, as a date, is still checked.
func ownIDs(claim string, d *Doc) string {
	ids := actIDRE.FindAllString(claim, -1)
	if len(ids) == 0 {
		return claim
	}
	refs, err := ParseLocator("the title page")
	if err != nil {
		return claim
	}
	spans, _ := Resolve(d, refs)
	var title strings.Builder
	for _, s := range spans {
		title.WriteString(d.Text[s.Start:s.End])
	}
	for _, id := range ids {
		if strings.Contains(title.String(), id) {
			claim = strings.ReplaceAll(claim, id, " ")
		}
	}
	return claim
}
