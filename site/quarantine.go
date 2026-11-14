// SPDX-License-Identifier: Apache-2.0

package site

import (
	"maps"
	"path"
	"slices"
	"sort"
	"strings"

	"github.com/tamnd/ai-agent/node"
	"github.com/tamnd/ai-agent/prov"
)

// withdrawn is what the quarantine hid of a node (15.7): the whole node, or some of its files.
type withdrawn struct {
	Whole bool
	Since string   // the date of the oldest quarantine line
	Files []string // the files hidden, when not the whole node
}

// Notice is the sentence that the page shows. It never gives the reason, which may itself be what was hidden.
func (w withdrawn) Notice() string {
	if w.Whole {
		return "This page is withdrawn while a person reviews a reported problem (since " + w.Since + "). It comes back when the review ends."
	}
	return "Part of this page is withdrawn while a person reviews a reported problem (since " + w.Since + "): " + strings.Join(w.Files, ", ") + "."
}

// withdraw gives a copy of the corpus without what the quarantine hides: a node quarantined as a whole, or by its
// node.md, keeps only its name, kind, area and links, as a stub with no copies; a quarantined file of a node leaves
// the page (an implementation file, a proof, a derivation, the facts, the claims, a scene). The corpus that it is
// given does not change.
func withdraw(c *node.Corpus) (*node.Corpus, map[string]withdrawn, error) {
	out := map[string]withdrawn{}
	var cc *node.Corpus
	for i, n := range c.Nodes {
		if n.Tag == "" || strings.HasPrefix(n.Tag, "NEW-") {
			continue
		}
		ls, err := prov.Read(c.Root, n.Tag)
		if err != nil {
			return nil, nil, err
		}
		q := prov.Quarantined(ls)
		if len(q) == 0 {
			continue
		}
		if cc == nil {
			copied := *c
			cc = &copied
			cc.Nodes = slices.Clone(c.Nodes)
			cc.BySlug, cc.ByTag = maps.Clone(c.BySlug), maps.Clone(c.ByTag)
		}
		m := *n
		w := withdrawn{}
		for f, l := range q {
			if w.Since == "" || l.Date < w.Since {
				w.Since = l.Date
			}
			if f == prov.Whole || f == "node.md" {
				w.Whole = true
			}
			w.Files = append(w.Files, f)
		}
		sort.Strings(w.Files)
		if w.Whole {
			m.Body, m.Summary = "", "Withdrawn for review."
			m.Impls, m.Proofs, m.Derivations, m.Facts, m.Claims, m.Exps, m.Presets = nil, nil, nil, nil, nil, nil, nil
			m.Scene, m.ArchScene, m.Shapes = "", "", nil
			w.Files = nil
		} else {
			hidden := func(p string) bool {
				_, ok := q[strings.TrimPrefix(path.Clean(strings.ReplaceAll(p, "\\", "/")), strings.ReplaceAll(n.Rel, "\\", "/")+"/")]
				return ok
			}
			var impls []node.Impl
			for _, im := range n.Impls {
				im.Files = slices.DeleteFunc(slices.Clone(im.Files), func(f node.File) bool { return hidden(f.Path) })
				if im.Main() != nil {
					impls = append(impls, im)
				}
			}
			m.Impls = impls
			m.Proofs = slices.DeleteFunc(slices.Clone(n.Proofs), func(p node.Proof) bool { return hidden(p.Path) })
			m.Derivations = slices.DeleteFunc(slices.Clone(n.Derivations), func(d node.Derivation) bool { return hidden(d.Path) })
			if _, ok := q["facts.yaml"]; ok {
				m.Facts = nil
			}
			if _, ok := q["claims.yaml"]; ok {
				m.Claims = nil
			}
			if _, ok := q["viz/scene.ts"]; ok {
				m.Scene, m.Presets = "", nil
			}
			if _, ok := q["shapes.yaml"]; ok {
				m.Shapes = nil
			}
			if _, ok := q["viz/shapes.scene.yaml"]; ok {
				m.ArchScene = ""
			}
		}
		cc.Nodes[i] = &m
		cc.BySlug[m.Slug] = &m
		cc.ByTag[m.Tag] = &m
		out[n.Slug] = w
	}
	if cc == nil {
		return c, out, nil
	}
	return cc, out, nil
}
