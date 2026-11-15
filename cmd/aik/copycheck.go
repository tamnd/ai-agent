// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/tamnd/ai-agent/cite"
	"github.com/tamnd/ai-agent/copycheck"
	"github.com/tamnd/ai-agent/node"
)

// refText is a line of a file of -ref: a reference text, such as the Wikipedia article of an item or a canon work that
// the project holds for checking only. A text with items is compared with the pages of those slugs only.
type refText struct {
	Name  string   `json:"name"`
	URI   string   `json:"uri,omitempty"`
	Items []string `json:"items,omitempty"`
	Text  string   `json:"text"`
}

// copyResult is the copy check of one node.
type copyResult struct {
	Node     string              `json:"node"`
	Sections int                 `json:"sections"`
	Sources  []string            `json:"sources"`
	Missing  []string            `json:"missing,omitempty"` // the cited works with no stored text
	Files    int                 `json:"files"`
	Findings []copycheck.Finding `json:"findings"`
}

// codeExt are the files of the code corpora that the code test reads.
var codeExt = []string{".py", ".jl", ".go", ".js", ".ts", ".rs", ".c", ".cc", ".cpp", ".h", ".java"}

// cmdCopycheck is the copy check of spec 13.7, rule C01: the prose of each node against the stored texts of the works
// that its claims cite and the texts of -ref, and its code against the code corpora of -code.
func cmdCopycheck(args []string) error {
	f := newFlags("copycheck", "[flags] [<slug> | <path>...]")
	root := f.String("root", ".", "the root of the content repository")
	store := f.String("store", "", "the raw store; the default is $AIK_RAW or the user cache")
	refs := f.String("ref", "", "a directory of more reference texts: *.jsonl, a {name, uri, items, text} on each line")
	codeDir := f.String("code", "", "a directory of the code corpora: every code file under it is a reference")
	asJSON := f.Bool("json", false, "print the results as JSON")
	if err := parse(f, args); err != nil {
		return err
	}
	c, err := load(*root)
	if err != nil {
		return err
	}
	nodes, err := copyNodes(c, *root, f.Args())
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
	extra, err := readRefs(*refs)
	if err != nil {
		return err
	}
	corpus, err := readCode(*codeDir)
	if err != nil {
		return err
	}
	docs := map[string]string{}
	var out []copyResult
	bad := 0
	for _, n := range nodes {
		r := copyNode(*root, n, ix, st, docs, extra, corpus)
		bad += len(r.Findings)
		out = append(out, r)
	}
	if *asJSON {
		if err := printJSON(out); err != nil {
			return err
		}
	} else {
		for _, r := range out {
			printCopy(r, len(corpus) > 0)
		}
	}
	if bad > 0 {
		return errFindings
	}
	return nil
}

// copyNodes gives the nodes of the arguments, slugs or paths in node directories, or every node.
func copyNodes(c *node.Corpus, root string, args []string) ([]*node.Node, error) {
	if len(args) == 0 {
		return c.Nodes, nil
	}
	var out []*node.Node
	for _, a := range args {
		if n := c.BySlug[a]; n != nil {
			out = append(out, n)
			continue
		}
		rel, err := filepath.Rel(root, a)
		if err != nil || strings.HasPrefix(rel, "..") {
			rel = a
		}
		rel = filepath.ToSlash(filepath.Clean(rel))
		i := slices.IndexFunc(c.Nodes, func(n *node.Node) bool { return rel == n.Rel || strings.HasPrefix(rel, n.Rel+"/") })
		if i < 0 {
			return nil, usageError{fmt.Sprintf("%s is not a slug or a path in a node", a)}
		}
		out = append(out, c.Nodes[i])
	}
	slices.SortFunc(out, func(a, b *node.Node) int { return strings.Compare(a.Rel, b.Rel) })
	return slices.Compact(out), nil
}

// copyNode runs the check on one node. docs keeps the text of each work across nodes.
func copyNode(root string, n *node.Node, ix *cite.Index, st cite.Store, docs map[string]string, extra []refText, corpus []copycheck.Source) copyResult {
	r := copyResult{Node: n.Slug, Sources: []string{}, Findings: []copycheck.Finding{}}
	secs := copycheck.Sections(n.Rel+"/node.md", n.Body, n.BodyLine)
	for _, d := range n.Derivations {
		line := 1
		if b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(d.Path))); err == nil {
			if _, _, l, err := node.SplitFront(b); err == nil {
				line = l
			}
		}
		for _, s := range copycheck.Sections(d.Path, d.Body, line) {
			s.Name = "derivations/" + d.ID + ": " + s.Name
			secs = append(secs, s)
		}
	}
	r.Sections = len(secs)
	var works []string
	for _, cl := range n.Claims {
		for _, e := range cl.Evidence {
			works = append(works, e.Work)
		}
	}
	slices.Sort(works)
	var srcs []copycheck.Source
	byHash := map[string]int{} // works in one snapshot, as the commentaries of an issue, are one source
	for _, w := range slices.Compact(works) {
		text, ok := docs[w]
		if !ok {
			if s, have := ix.Snapshots[w]; have {
				if d := st.AnyDoc(s.Hash); d != nil {
					text = d.Text
				}
			}
			docs[w] = text
		}
		if text == "" {
			r.Missing = append(r.Missing, w)
			continue
		}
		h := ix.Snapshots[w].Hash
		if i, ok := byHash[h]; ok {
			srcs[i].Name += " + " + w
			continue
		}
		byHash[h] = len(srcs)
		srcs = append(srcs, copycheck.Source{Name: w, URI: ix.Snapshots[w].URL, Text: text})
	}
	for _, t := range extra {
		if len(t.Items) == 0 || slices.Contains(t.Items, n.Slug) {
			srcs = append(srcs, copycheck.Source{Name: t.Name, URI: t.URI, Text: t.Text})
		}
	}
	for _, s := range srcs {
		r.Sources = append(r.Sources, s.Name)
	}
	r.Findings = append(r.Findings, copycheck.Prose(secs, srcs)...)
	if len(corpus) > 0 {
		var files []copycheck.File
		for _, im := range n.Impls {
			for _, fl := range im.Files {
				if !fl.Test {
					files = append(files, copycheck.File{Path: fl.Path, Text: fl.Content})
				}
			}
		}
		r.Files = len(files)
		r.Findings = append(r.Findings, copycheck.Code(files, corpus)...)
	}
	copycheck.Sort(r.Findings)
	return r
}

func printCopy(r copyResult, code bool) {
	what := fmt.Sprintf("%d sections against %d sources", r.Sections, len(r.Sources))
	if code {
		what += fmt.Sprintf(", %d code files", r.Files)
	}
	if len(r.Missing) > 0 {
		what += fmt.Sprintf("; %d cited works have no stored text: %s", len(r.Missing), strings.Join(r.Missing, ", "))
	}
	state := "ok"
	if len(r.Findings) > 0 {
		state = fmt.Sprintf("%d findings", len(r.Findings))
	}
	fmt.Printf("%s: %s: %s\n", r.Node, state, what)
	for _, f := range r.Findings {
		switch f.Test {
		case "shingles":
			fmt.Printf("  %s:%d: C01: the section %q shares %.1f%% of its %d-word shingles (%d of %d) with %s; the rule allows %.0f%%\n", f.File, f.Line, f.Section, 100*f.Share, copycheck.Shingle, f.Shared, f.Of, f.Source, 100*copycheck.SectionShare)
			for _, r := range f.Runs {
				fmt.Printf("    line %d: %s\n", r.Line, r.Text)
			}
		case "aligned":
			fmt.Printf("  %s:%d-%d: C01: %d consecutive sentences of the section %q follow sentences of %s in the same order, each with %.0f%% or more of its content words; write them afresh\n", f.File, f.Line, f.End, f.Shared, f.Section, f.Source, 100*f.Share)
		case "code":
			fmt.Printf("  %s: C01: %.0f%% of the fingerprints of the file (%d of %d) are in %s; the rule allows less than %.0f%%\n", f.File, 100*f.Share, f.Shared, f.Of, f.Source, 100*copycheck.CodeShare)
		}
	}
}

// readRefs reads the reference texts of the *.jsonl files of a directory, in the order of the files and the lines.
func readRefs(dir string) ([]refText, error) {
	if dir == "" {
		return nil, nil
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	if err != nil {
		return nil, err
	}
	var out []refText
	for _, p := range files {
		fh, err := os.Open(p)
		if err != nil {
			return nil, err
		}
		sc := bufio.NewScanner(fh)
		sc.Buffer(make([]byte, 1<<20), 1<<28)
		for i := 1; sc.Scan(); i++ {
			var t refText
			if err := json.Unmarshal(sc.Bytes(), &t); err != nil {
				_ = fh.Close()
				return nil, fmt.Errorf("%s:%d: %w", p, i, err)
			}
			out = append(out, t)
		}
		err = sc.Err()
		_ = fh.Close()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
	}
	return out, nil
}

// readCode reads the code files under a directory, named by their path in it.
func readCode(dir string) ([]copycheck.Source, error) {
	if dir == "" {
		return nil, nil
	}
	var out []copycheck.Source
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != dir && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !slices.Contains(codeExt, filepath.Ext(p)) {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		out = append(out, copycheck.Source{Name: filepath.ToSlash(rel), Text: string(b)})
		return nil
	})
	return out, err
}
