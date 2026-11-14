// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/tamnd/ai-agent/cite"
	"github.com/tamnd/ai-agent/claims"
	"github.com/tamnd/ai-agent/node"
)

// cmdCite is the citation check of spec 7.5: aik cite <slug>... | -all, and aik cite show <work> [<locator>].
func cmdCite(args []string) error {
	if len(args) > 0 {
		switch args[0] {
		case "show":
			return citeShow(args[1:])
		case "passages":
			return citePassages(args[1:])
		case "verdict":
			return citeVerdict(args[1:])
		}
	}
	f := newFlags("cite", "[flags] <slug>... | -all | show <work> [<locator>] | passages <slug>... | verdict -file F")
	root := f.String("root", ".", "the root of the content repository")
	all := f.Bool("all", false, "check every node that has claims")
	offline := f.Bool("offline", false, "do not fetch; use only the snapshots in the store")
	store := f.String("store", "", "the raw store; the default is $AIK_RAW or the user cache")
	asJSON := f.Bool("json", false, "print the records as JSON")
	if err := parse(f, args); err != nil {
		return err
	}
	if !*all && f.NArg() == 0 {
		return usageError{"give a slug or -all"}
	}
	c, err := load(*root)
	if err != nil {
		return err
	}
	var nodes []*node.Node
	if *all {
		for _, n := range c.Nodes {
			if len(n.Claims) > 0 {
				nodes = append(nodes, n)
			}
		}
	} else {
		for _, s := range f.Args() {
			n := c.BySlug[s]
			if n == nil {
				return usageError{fmt.Sprintf("no node has the slug %q", s)}
			}
			nodes = append(nodes, n)
		}
	}
	ix, err := cite.Load(*root)
	if err != nil {
		return err
	}
	ck := &citer{c: c, ix: ix, store: cite.DefaultStore(), offline: *offline, docs: map[string]*cite.Doc{}, prev: map[string]*cite.Doc{}, failed: map[string]string{}}
	if *store != "" {
		ck.store = cite.Store{Dir: *store}
	}
	ctx := context.Background()
	bad := 0
	var out []cite.Record
	for _, n := range nodes {
		rs, ungrounded := ck.node(ctx, n)
		out = append(out, rs...)
		if len(rs) > 0 {
			if err := cite.WriteRecords(*root, n.Tag, rs); err != nil {
				return err
			}
		}
		bad += ungrounded
	}
	if err := cite.WriteManifest(*root, ix.Snapshots); err != nil {
		return err
	}
	if *asJSON {
		return printJSON(out)
	}
	if bad > 0 {
		return errFindings
	}
	return nil
}

type citer struct {
	c       *node.Corpus
	ix      *cite.Index
	store   cite.Store
	offline bool
	docs    map[string]*cite.Doc
	prev    map[string]*cite.Doc // the texts that the index's records were made on, where a new extractor replaced them
	failed  map[string]string    // works that could not be read, with why
	last    time.Time            // the last request to arXiv
	carried int                  // the verdicts of the node that a new key kept, as what was judged is the same
}

// carry tells if the verdict of the old record o of a source holds for its new record r (cite.Carry).
func (ck *citer) carry(o, r cite.Record, cl *claims.Claim) bool {
	snap, ok := ck.ix.Snapshots[r.Work]
	d := ck.docs[snap.Hash]
	return ok && d != nil && cite.Carry(o, r, cl, d, ck.prev[snap.Hash], snap)
}

// node checks every source of every claim of n. It prints a line for each claim and returns the records and the
// number of labelled claims with no source that passes (G05).
func (ck *citer) node(ctx context.Context, n *node.Node) ([]cite.Record, int) {
	old := ck.ix.Records[n.Tag]
	ck.carried = 0
	var rs []cite.Record
	pass, ungrounded := 0, 0
	fmt.Printf("%s (%s)\n", n.Slug, n.Tag)
	for i := range n.Claims {
		cl := &n.Claims[i]
		ok := false
		var lines []string
		for _, e := range cl.Evidence {
			r, why := ck.one(ctx, cl, e)
			if why != "" {
				lines = append(lines, fmt.Sprintf("      %s %s: %s", e.Work, e.Locator, why))
				continue
			}
			// The verdict of the verifier stays while the key is the same, and while what it judged is the same: the
			// claim, the work, the locator, the snapshot and the passage with its section (spec 7.5, 15.4).
			if o, has := old[cite.RecordID(r.Claim, r.Work, r.Locator)]; has && (o.Key == r.Key || ck.carry(o, r, cl)) {
				if o.Key != r.Key && o.Verdict != "" {
					ck.carried++
				}
				r.Verdict, r.Verifier, r.Run = o.Verdict, o.Verifier, o.Run
			}
			rs = append(rs, r)
			if r.Pass {
				ok = true
				lines = append(lines, fmt.Sprintf("      %s %s: found (%d of %d terms)", e.Work, e.Locator, r.TermsFound, r.Terms))
			} else {
				lines = append(lines, fmt.Sprintf("      %s %s: %s", e.Work, e.Locator, r.Why()))
			}
		}
		state := "found"
		switch {
		case len(cl.Evidence) == 0:
			state = "no source"
		case !ok:
			state = "NOT FOUND"
		}
		if ok || len(cl.Evidence) == 0 && !claims.Labelled(cl.Type) {
			pass++
		} else if claims.Labelled(cl.Type) {
			ungrounded++
		}
		fmt.Printf("  %-9s %s\n", state, cl.ID)
		for _, l := range lines {
			fmt.Println(l)
		}
	}
	fmt.Printf("  %d of %d claims found in their sources\n", pass, len(n.Claims))
	if ck.carried > 0 {
		fmt.Printf("  %d verdicts carried over: the passage the verifier judged is the same\n", ck.carried)
	}
	fmt.Println()
	return rs, ungrounded
}

// one checks one source of a claim. why says why the check could not run.
func (ck *citer) one(ctx context.Context, cl *claims.Claim, e claims.Evidence) (cite.Record, string) {
	w, why := ck.work(e.Work)
	if why != "" {
		return cite.Record{}, why
	}
	snap, d, why := ck.doc(ctx, w)
	if why != "" {
		return cite.Record{}, why
	}
	r := cite.Check(cl, e, d, snap, w.Authors...)
	r.Checked = time.Now().UTC().Format("2006-01-02")
	return r, ""
}

// work reads the cited version of a work from its work card.
func (ck *citer) work(slug string) (cite.Work, string) {
	cd := ck.c.CardByKey[slug]
	if cd == nil || cd.Kind != "work" {
		if n := ck.c.BySlug[slug]; n != nil && n.Kind == "work" {
			return cite.Work{}, "the work is a node whose front matter does not make a card"
		}
		return cite.Work{}, "no work card has this slug (G01)"
	}
	w := cite.Work{Slug: slug}
	var cited struct {
		Version string `json:"version"`
		URL     string `json:"url"`
		Format  string `json:"format"`
	}
	raw, ok := cd.Fields["cited"]
	if !ok || json.Unmarshal(raw, &cited) != nil {
		return w, "the work card has no cited version (cited: {version, url, format})"
	}
	w.Version, w.URL, w.Format = cited.Version, cited.URL, cited.Format
	if raw, ok := cd.Fields["licence_class"]; ok {
		_ = json.Unmarshal(raw, &w.Licence)
	}
	if raw, ok := cd.Fields["authors"]; ok {
		_ = json.Unmarshal(raw, &w.Authors)
	}
	if w.URL == "" || w.Format == "none" {
		return w, "the work has no open full text, so the claim needs a reviewer's attestation or another source (7.5)"
	}
	return w, ""
}

// doc gives the snapshot and the text of the cited version of a work, fetching it when the manifest has no copy of
// this version.
func (ck *citer) doc(ctx context.Context, w cite.Work) (cite.Snapshot, *cite.Doc, string) {
	if why, ok := ck.failed[w.Slug]; ok {
		return cite.Snapshot{}, nil, why
	}
	fail := func(why string) (cite.Snapshot, *cite.Doc, string) {
		ck.failed[w.Slug] = why
		return cite.Snapshot{}, nil, why
	}
	s, have := ck.ix.Snapshots[w.Slug]
	if have && (s.Version != w.Version || s.URL != w.URL) {
		have = false // the card now cites another version
	}
	var b []byte
	if have && ck.store.Has(s.Hash) {
		if d, ok := ck.docs[s.Hash]; ok {
			return s, d, ""
		}
		if d := ck.store.Doc(s.Hash); d != nil && d.Extractor == s.Extractor {
			ck.docs[s.Hash] = d
			return s, d, ""
		}
		if d := ck.store.AnyDoc(s.Hash); d != nil && d.Extractor == s.Extractor && ck.prev != nil {
			ck.prev[s.Hash] = d // the text that the records were made on, before the new extractor replaces it
		}
		var err error
		if b, err = ck.store.Get(s.Hash); err != nil {
			return fail(err.Error())
		}
	} else {
		if ck.offline {
			return fail("the store has no snapshot of " + w.Version + "; run without -offline")
		}
		var err error
		var mt string
		b, mt, err = ck.fetch(ctx, w.URL)
		if err != nil {
			return fail(err.Error())
		}
		now := time.Now().UTC()
		h, err := ck.store.Put(b, cite.Meta{URL: w.URL, Fetched: now.Format(time.RFC3339), ContentType: mt, Licence: w.Licence, Fetcher: "aik/" + ver()})
		if err != nil {
			return fail(err.Error())
		}
		if have && h != s.Hash {
			fmt.Fprintf(os.Stderr, "aik cite: %s: the bytes at %s changed since %s; the new snapshot replaces the old one\n", w.Slug, w.URL, s.Fetched)
		}
		s = cite.Snapshot{Work: w.Slug, Version: w.Version, URL: w.URL, Format: w.Format, Hash: h, Size: len(b), Fetched: now.Format("2006-01-02")}
	}
	d, err := cite.Extract(ctx, b, w.Format)
	if errors.Is(err, cite.ErrNoExtractor) {
		return fail(err.Error())
	}
	if err != nil {
		return fail(fmt.Sprintf("%s: %v", w.URL, err))
	}
	if err := ck.store.PutDoc(s.Hash, d); err != nil {
		return fail(err.Error())
	}
	s.Extractor = d.Extractor
	ck.ix.Snapshots[w.Slug] = s
	ck.docs[s.Hash] = d
	return s, d, ""
}

// fetch gets a URL, at most one request every three seconds to arXiv, as its terms ask.
func (ck *citer) fetch(ctx context.Context, raw string) ([]byte, string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, "", err
	}
	if strings.HasSuffix(u.Hostname(), "arxiv.org") {
		if wait := 3*time.Second - time.Since(ck.last); wait > 0 {
			time.Sleep(wait)
		}
		defer func() { ck.last = time.Now() }()
	}
	return cite.Fetch(ctx, raw, "aik/"+strings.TrimPrefix(ver(), "v")+" (+https://github.com/tamnd/ai-agent)")
}

// citeShow prints the headings and marks that the extraction found in a work, or the text that a locator names, so
// that a writer can fix a locator. The text stays on the writer's screen; it is never written to the repository.
func citeShow(args []string) error {
	f := newFlags("cite show", "[flags] <work> [<locator>]")
	root := f.String("root", ".", "the root of the content repository")
	store := f.String("store", "", "the raw store; the default is $AIK_RAW or the user cache")
	if err := parse(f, args); err != nil {
		return err
	}
	if f.NArg() < 1 || f.NArg() > 2 {
		return usageError{"give a work and, if you like, a locator"}
	}
	c, err := load(*root)
	if err != nil {
		return err
	}
	ix, err := cite.Load(*root)
	if err != nil {
		return err
	}
	ck := &citer{c: c, ix: ix, store: cite.DefaultStore(), docs: map[string]*cite.Doc{}, prev: map[string]*cite.Doc{}, failed: map[string]string{}}
	if *store != "" {
		ck.store = cite.Store{Dir: *store}
	}
	w, why := ck.work(f.Arg(0))
	if why != "" {
		return errors.New(why)
	}
	snap, d, why := ck.doc(context.Background(), w)
	if why != "" {
		return errors.New(why)
	}
	if err := cite.WriteManifest(*root, ix.Snapshots); err != nil {
		return err
	}
	if f.NArg() == 2 {
		refs, err := cite.ParseLocator(f.Arg(1))
		if err != nil {
			return err
		}
		spans, fails := cite.Resolve(d, refs)
		for _, s := range cite.Merge(spans) {
			fmt.Printf("--- [%d, %d), page %d\n%s\n", s.Start, s.End, d.Page(s.Start)+1, strings.TrimSpace(d.Text[s.Start:s.End]))
		}
		for _, m := range fails {
			fmt.Println("not found:", m)
		}
		if len(fails) > 0 {
			return errFindings
		}
		return nil
	}
	fmt.Printf("%s, %s, snapshot %s, %d pages, %s\n", w.Slug, w.Version, snap.Hash[:12], len(d.Pages), d.Extractor)
	if d.Abstract != nil {
		fmt.Println("  the abstract")
	}
	if d.Title != nil {
		fmt.Printf("  the title page (%d bytes)\n", d.Title.End-d.Title.Start)
	}
	provs := map[string][]string{}
	for _, p := range d.Provisions {
		provs[p.Kind] = append(provs[p.Kind], p.Num)
	}
	for _, k := range []string{"recital", "article", "annex"} {
		if len(provs[k]) > 0 {
			fmt.Printf("  %s %s\n", k, strings.Join(provs[k], ", "))
		}
	}
	for _, h := range d.Heads {
		fmt.Printf("  %s%s %s (page %d)\n", strings.Repeat("  ", strings.Count(h.Num, ".")), h.Num, h.Title, d.Page(h.Start)+1)
	}
	kinds := map[string][]string{}
	for _, m := range d.Marks {
		if !slices.Contains(kinds[m.Kind], m.Num) {
			kinds[m.Kind] = append(kinds[m.Kind], m.Num)
		}
	}
	for _, k := range []string{"table", "figure", "algorithm", "equation", "footnote"} {
		if len(kinds[k]) > 0 {
			fmt.Printf("  %s %s\n", k, strings.Join(kinds[k], ", "))
		}
	}
	return nil
}

// passage is what the verifier of 15.4 reads for one source of a claim: the claim, its type and scope, and the
// located span with its section. It does not see the page.
type passage struct {
	Tag     string        `json:"tag"`
	Slug    string        `json:"slug"`
	Claim   string        `json:"claim"`
	Type    string        `json:"type"`
	Scope   *claims.Scope `json:"scope,omitempty"`
	Text    string        `json:"text"`
	Work    string        `json:"work"`
	Version string        `json:"version"`
	Locator string        `json:"locator"`
	Key     string        `json:"key"`
	Spans   []cite.Span   `json:"spans"`
	Passage string        `json:"passage"`
	Section string        `json:"section,omitempty"`
	Verdict string        `json:"verdict,omitempty"`
}

// citePassages prints, as JSON, the passages of the sources that passed the citation check, for the verifier
// (work/verify.py). The text goes to the verifier and to no file of the repository (C01).
func citePassages(args []string) error {
	f := newFlags("cite passages", "[flags] <slug>...")
	root := f.String("root", ".", "the root of the content repository")
	store := f.String("store", "", "the raw store; the default is $AIK_RAW or the user cache")
	redo := f.Bool("redo", false, "give the passages that have a verdict too")
	if err := parse(f, args); err != nil {
		return err
	}
	if f.NArg() == 0 {
		return usageError{"give a slug"}
	}
	c, err := load(*root)
	if err != nil {
		return err
	}
	ix, err := cite.Load(*root)
	if err != nil {
		return err
	}
	st := cite.DefaultStore()
	if *store != "" {
		st = cite.Store{Dir: *store}
	}
	out := []passage{}
	for _, slug := range f.Args() {
		n := c.BySlug[slug]
		if n == nil {
			return usageError{fmt.Sprintf("no node has the slug %q", slug)}
		}
		for i := range n.Claims {
			cl := &n.Claims[i]
			for _, e := range cl.Evidence {
				r, ok := ix.Current(n.Tag, cl, e)
				if !ok || !r.Pass || (r.Verdict != "" && !*redo) {
					continue
				}
				d := st.Doc(r.Snapshot)
				if d == nil {
					return fmt.Errorf("the store %s has no current extraction of the snapshot %s of %s; run aik cite %s", st.Dir, r.Snapshot[:12], e.Work, slug)
				}
				p, sec := cite.Passage(d, r.Spans)
				out = append(out, passage{Tag: n.Tag, Slug: slug, Claim: cl.ID, Type: cl.Type, Scope: cl.Scope, Text: cl.Text,
					Work: e.Work, Version: ix.Snapshots[e.Work].Version, Locator: e.Locator, Key: r.Key, Spans: r.Spans,
					Passage: p, Section: sec, Verdict: r.Verdict})
			}
		}
	}
	return printJSON(out)
}

// citeVerdict writes the verdicts of the verifier into the records: a JSON list of {tag, key, verdict}. A verdict for
// a key that no record has any more is skipped, because the claim, the locator or the snapshot changed since.
func citeVerdict(args []string) error {
	f := newFlags("cite verdict", "[flags] -file F -verifier backend:model -run RUN")
	root := f.String("root", ".", "the root of the content repository")
	file := f.String("file", "", "the verdicts, as a JSON list of {tag, key, verdict}; - for stdin")
	verifier := f.String("verifier", "", "the model that gave the verdicts, as backend:model")
	run := f.String("run", "", "the id of the run record (14.7)")
	if err := parse(f, args); err != nil {
		return err
	}
	if *file == "" || *verifier == "" || *run == "" {
		return usageError{"give -file, -verifier and -run"}
	}
	var b []byte
	var err error
	if *file == "-" {
		b, err = io.ReadAll(os.Stdin)
	} else {
		b, err = os.ReadFile(*file)
	}
	if err != nil {
		return err
	}
	var vs []struct{ Tag, Key, Verdict string }
	if err := json.Unmarshal(b, &vs); err != nil {
		return fmt.Errorf("%s: %w", *file, err)
	}
	ix, err := cite.Load(*root)
	if err != nil {
		return err
	}
	changed := map[string]bool{}
	set, stale := 0, 0
	for _, v := range vs {
		if !slices.Contains(cite.Verdicts, v.Verdict) {
			return usageError{fmt.Sprintf("the verdict %q is not one of %s", v.Verdict, strings.Join(cite.Verdicts, ", "))}
		}
		found := false
		for id, r := range ix.Records[v.Tag] {
			if r.Key == v.Key {
				r.Verdict, r.Verifier, r.Run = v.Verdict, *verifier, *run
				ix.Records[v.Tag][id] = r
				changed[v.Tag], found = true, true
			}
		}
		if found {
			set++
		} else {
			stale++
		}
	}
	for tag := range changed {
		rs := make([]cite.Record, 0, len(ix.Records[tag]))
		for _, r := range ix.Records[tag] {
			rs = append(rs, r)
		}
		if err := cite.WriteRecords(*root, tag, rs); err != nil {
			return err
		}
	}
	fmt.Printf("%d verdicts written", set)
	if stale > 0 {
		fmt.Printf(", %d skipped because no record has their key", stale)
	}
	fmt.Println()
	return nil
}
