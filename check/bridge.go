// SPDX-License-Identifier: Apache-2.0

package check

import (
	"errors"
	"regexp"
	"strings"

	"github.com/tamnd/ai-agent/bridge"
	"github.com/tamnd/ai-agent/node"
)

// bridges reports the lines of cards/bridge that are not bridge cards.
func (r *run) bridges() {
	for _, err := range r.c.BridgeErrors {
		var e *bridge.Error
		if errors.As(err, &e) {
			r.add("T01", Block, e.File, e.Line, "the line is not a bridge card: %v", e.Err)
			continue
		}
		r.add("T01", Block, bridge.Dir, 0, "%v", err)
	}
}

// bridgeTarget checks an edge target of the form slug@al (E01 and L02). It tells if the target names a neighbour.
func (r *run) bridgeTarget(n *node.Node, field, ref string) bool {
	site, slug, ok := bridge.Split(ref)
	if !ok {
		return false
	}
	p := n.Rel + "/node.md"
	c := r.c.Bridges.Find(ref)
	switch {
	case c == nil:
		r.add("E01", Block, p, 0, "%s: no bridge card has the slug %q on %s; run aik bridge sync %s", field, slug, bridge.Sites[site].Host, site)
	case c.Retired != "":
		r.add("L02", Level, p, 0, "%s: %s is not on %s since the sync of %s (broken-bridge)", field, ref, bridge.Sites[site].Host, c.Retired)
	case c.Slug != slug:
		r.add("L02", Level, p, 0, "%s: %s is now %s on %s; write %s", field, ref, c.Slug, bridge.Sites[site].Host, c.Key())
	}
	return true
}

// neighbourLink finds a wiki link to a neighbour, such as [[al:k-means]] or [[m2:kl-divergence|KL divergence]].
var neighbourLink = regexp.MustCompile(`\[\[((?:al|m2|pg):[a-z0-9][a-z0-9-]*)(?:\|[^\]]+)?\]\]`)

// neighbourLinks is rule L02: each link to a neighbour resolves through a bridge card (spec 5.9).
func (r *run) neighbourLinks(n *node.Node) {
	files := []struct {
		path, body string
		line       int
	}{{n.Rel + "/node.md", n.Body, n.BodyLine}}
	for _, pr := range n.Proofs {
		files = append(files, struct {
			path, body string
			line       int
		}{pr.Path, pr.Body, 0})
	}
	for _, f := range files {
		for _, m := range neighbourLink.FindAllStringSubmatchIndex(f.body, -1) {
			ref := f.body[m[2]:m[3]]
			line := 0
			if f.line > 0 {
				line = f.line + strings.Count(f.body[:m[0]], "\n")
			}
			site, slug, _ := bridge.Split(ref)
			host := bridge.Sites[site].Host
			c := r.c.Bridges.Find(ref)
			switch {
			case c == nil:
				r.add("L02", Level, f.path, line, "the link [[%s]] has no bridge card on %s; run aik bridge sync %s", ref, host, site)
			case c.Retired != "":
				r.add("L02", Level, f.path, line, "the link [[%s]] names an item that is not on %s since the sync of %s (broken-bridge)", ref, host, c.Retired)
			case c.Slug != slug:
				r.add("L02", Level, f.path, line, "the link [[%s]] uses an old slug; write [[%s]]", ref, c.Ref())
			}
		}
	}
}
