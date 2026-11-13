// SPDX-License-Identifier: Apache-2.0

package site

import (
	"html/template"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/tamnd/ai-agent/formal"
	"github.com/tamnd/ai-agent/markup"
	"github.com/tamnd/ai-agent/node"
	"github.com/tamnd/ai-agent/status"
)

// LeanView is the Lean part of a node: its module, its declarations and the source (spec 07.16, 14.7).
type LeanView struct {
	Module string
	File   string // the path in the content repository, such as lean/AI/Sorting/MergeSort.lean
	GitHub string
	URL    string // the Lean page, /<slug>/lean/
	Lines  int
	HTML   template.HTML
	Rung   status.Rung
	// Built is false when the build has no decls.jsonl. Then the page shows the source only.
	Built bool
	// Std is true when each declaration of the module uses only the standard axioms.
	Std bool
	// Key holds the declarations that the lean field names, and the class lemmas, in the order of the ladder.
	Key []DeclView
	// All holds each declaration of the module, in the order of the source.
	All []DeclView
}

// DeclView is one declaration of a Lean module.
type DeclView struct {
	Role   string
	Name   string
	Short  string // the name without the module namespace
	Kind   string
	Type   string
	Doc    string
	Axioms []string
	Std    bool
	Line   int
	Source string // the lines on GitHub
	Proof  *Link  // the human proof of the same claim, from the lean field of the proof
}

var leanRoles = []struct{ key, name string }{
	{"spec", "Spec"},
	{"model", "Model"},
	{"correct", "Correctness"},
	{"cost", "Cost bound"},
	{"space", "Space bound"},
}

func (b *builder) leanView(n *node.Node) *LeanView {
	mod := n.Lean["module"]
	if mod == "" {
		return nil
	}
	rel := "lean/" + strings.ReplaceAll(mod, ".", "/") + ".lean"
	src, err := os.ReadFile(filepath.Join(b.c.Root, filepath.FromSlash(rel)))
	if err != nil {
		b.opt.Log("%s: the Lean module %s: %v", n.Slug, mod, err)
		return nil
	}
	v := &LeanView{
		Module: mod,
		File:   rel,
		GitHub: b.opt.Repo + "/blob/main/" + rel,
		URL:    "/" + n.Slug + "/lean/",
		Lines:  strings.Count(string(src), "\n"),
		HTML:   template.HTML(markup.Highlight(string(src), "lean")),
		Rung:   b.status[n.Slug].L,
		Built:  b.decls != nil,
		Std:    b.decls != nil,
	}
	proofs := map[string]*Link{}
	for _, p := range n.Proofs {
		if p.Front.Lean != "" {
			t := p.Front.Title
			if t == "" {
				t = humanize(p.ID)
			}
			proofs[p.Front.Lean] = &Link{Name: t, URL: "/" + n.Slug + "/#proof-" + p.ID}
		}
	}
	view := func(d formal.Decl, role string) DeclView {
		dv := DeclView{Role: role, Name: d.Decl, Short: strings.TrimPrefix(d.Decl, mod+"."), Kind: d.Kind, Type: d.Type, Doc: d.Doc, Axioms: d.Axioms, Std: len(d.Disallowed()) == 0, Proof: proofs[d.Decl]}
		if d.Pos != nil {
			dv.Line = d.Pos.Line
			dv.Source = v.GitHub + "#L" + strconv.Itoa(d.Pos.Line) + "-L" + strconv.Itoa(d.Pos.EndLine)
		}
		return dv
	}
	for _, d := range b.decls {
		if d.Module != mod {
			continue
		}
		if len(d.Disallowed()) > 0 {
			v.Std = false
		}
		v.All = append(v.All, view(d, ""))
	}
	slices.SortFunc(v.All, func(x, y DeclView) int { return x.Line - y.Line })
	for _, r := range leanRoles {
		if d, ok := b.decls[n.Lean[r.key]]; ok {
			v.Key = append(v.Key, view(d, r.name))
		}
	}
	for _, d := range v.All {
		if d.Kind == "theorem" && formal.IsClassLemma(d.Type) {
			d.Role = "Class"
			v.Key = append(v.Key, d)
		}
	}
	return v
}
