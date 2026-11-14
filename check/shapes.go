// SPDX-License-Identifier: Apache-2.0

package check

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/tamnd/ai-agent/node"
	"github.com/tamnd/ai-agent/shapes"
)

// shapes checks the shape contract of an architecture (rule X09) and the references of the prose to it (rule X20).
// The tests of the implementations check the code against fixtures/shapes.json; here the build checks the file itself and that the fixture is current.
func (r *run) shapes(n *node.Node) {
	f := n.Shapes
	if f == nil {
		if n.Kind == "architecture" {
			r.add("X09", Level, n.Rel+"/node.md", 0, "the architecture has no %s with its shapes, parameter count and FLOP formulas (08.6)", shapes.FileName)
		}
		r.shapeRefs(n)
		r.factRefs(n)
		return
	}
	ps := f.Check()
	for _, p := range ps {
		r.add("X09", Block, f.Path, p.Line, "%s", p.Msg)
	}
	for _, b := range f.Blocks {
		for _, id := range b.Eq {
			if !strings.Contains(n.Body, "{#eq-"+id+"}") {
				r.add("X09", Level, f.Path, b.Line, "the block %s names the equation eq-%s, but node.md has no {#eq-%s} anchor", b.ID, id, id)
			}
		}
	}
	r.shapeRefs(n)
	r.factRefs(n)
	if len(ps) > 0 {
		return
	}
	path := n.Rel + "/" + shapes.FixtureFile
	fx, err := f.MakeFixture(shapes.Seed(n.Rel))
	if err != nil {
		r.add("X09", Block, f.Path, 0, "%v", err)
		return
	}
	want, err := fx.Encode()
	if err != nil {
		r.add("X09", Block, f.Path, 0, "%v", err)
		return
	}
	got, err := os.ReadFile(filepath.Join(n.Dir, filepath.FromSlash(shapes.FixtureFile)))
	switch {
	case errors.Is(err, os.ErrNotExist):
		if len(n.Impls) > 0 {
			r.add("X09", Level, path, 0, "the node has code but no %s for its shape tests; run aik shapes sample -w %s", shapes.FixtureFile, n.Slug)
		}
	case err != nil:
		r.add("X09", Block, path, 0, "%v", err)
	case !bytes.Equal(bytes.ReplaceAll(got, []byte("\r\n"), []byte("\n")), want):
		r.add("X09", Level, path, 0, "the fixture is not what %s gives; run aik shapes sample -w %s", shapes.FileName, n.Slug)
	}
}

// shapeRefs checks that each {shape:...} of the prose names a formula of shapes.yaml and, with @config, a config (rule X20).
func (r *run) shapeRefs(n *node.Node) {
	for i, line := range strings.Split(n.Body, "\n") {
		for _, m := range shapes.RefRE.FindAllStringSubmatch(line, -1) {
			if n.Shapes == nil {
				r.add("X20", Block, n.Rel+"/node.md", n.BodyLine+i, "%s names the shape contract, but the node has no %s", m[0], shapes.FileName)
				continue
			}
			if _, _, err := n.Shapes.Ref(m[1], m[2]); err != nil {
				r.add("X20", Block, n.Rel+"/node.md", n.BodyLine+i, "%s: %v", m[0], err)
			}
		}
	}
}

// factRefs checks that each {fact:<item>/<field>} of the prose names a fact that the item holds (rule D01), and that
// a computed fact comes from aik facts compute (rule D08).
func (r *run) factRefs(n *node.Node) {
	for i, line := range strings.Split(n.Body, "\n") {
		for _, m := range node.FactRefRE.FindAllStringSubmatch(line, -1) {
			_, err := r.c.Fact(m[1], m[2])
			switch {
			case errors.Is(err, node.ErrComputed):
				r.add("D08", Block, n.Rel+"/node.md", n.BodyLine+i, "%s: %v", m[0], err)
			case err != nil:
				r.add("D01", Block, n.Rel+"/node.md", n.BodyLine+i, "%s: %v", m[0], err)
			}
		}
	}
}
