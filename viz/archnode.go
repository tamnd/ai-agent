// SPDX-License-Identifier: Apache-2.0

package viz

import (
	"os"
	"path/filepath"

	"github.com/tamnd/ai-agent/node"
	"github.com/tamnd/ai-agent/shapes"
)

// ArchSamples is the number of sampled configs on which aik viz check runs an architecture scene, besides its presets (V14).
const ArchSamples = 20

// CorpusLinks resolves a link of a scene to the page of a node of the corpus or, for slug@al, to its bridge card.
func CorpusLinks(c *node.Corpus) Links {
	return func(slug string) (string, bool) {
		if n := c.BySlug[slug]; n != nil {
			return "/" + n.Slug + "/", true
		}
		if b := c.Bridges.Find(slug); b != nil && b.Retired == "" {
			return b.URL, true
		}
		return "", false
	}
}

// NodeArch reads, compiles and checks the architecture scene of a node. The data is nil when the scene cannot be
// shown; the report holds every failure, of the scene file and of the check.
func NodeArch(c *node.Corpus, n *node.Node) (*ArchData, Report) {
	r := Report{Slug: n.Slug}
	b, err := os.ReadFile(filepath.Join(c.Root, filepath.FromSlash(n.ArchScene)))
	if err != nil {
		r.Fails = []string{err.Error()}
		return nil, r
	}
	if n.Shapes == nil {
		r.Fails = []string{"the node has an architecture scene but no " + shapes.FileName + " (12.6)"}
		return nil, r
	}
	s, err := ParseArch(b)
	if err != nil {
		r.Fails = []string{n.ArchScene + ": " + err.Error()}
		return nil, r
	}
	d, fails := CompileArch(n.Slug, n.Shapes, s, CorpusLinks(c))
	if d == nil {
		r.Fails = fails
		return nil, r
	}
	samples, err := n.Shapes.Sample(shapes.Seed(n.Rel), ArchSamples)
	if err != nil {
		fails = append(fails, err.Error())
	}
	r = CheckArch(n.Shapes, d, samples)
	r.Fails = append(fails, r.Fails...)
	return d, r
}
