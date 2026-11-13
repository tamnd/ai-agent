// SPDX-License-Identifier: Apache-2.0

package check

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/ai-agent/node"
	"github.com/tamnd/ai-agent/shapes"
)

const archDir = "nodes/searching/sequential-and-binary-search/toy-net/"

const archNode = `---
tag: TOYNET
kind: architecture
slug: toy-net
name: Toy net
area: searching/sequential-and-binary-search
level: intro
summary: A toy architecture for the tests.
---

## Overview

One linear layer: $y = W x + b$. {#eq-linear}

It has {shape:params@example} parameters, and the formula is {shape:params}.
`

const archShapes = `config:
  d_in: {meaning: "input width", example: 4}
  d_out: {meaning: "output width", example: 3}
axes:
  B: {meaning: batch, example: 2}
  I: d_in
  O: d_out
blocks:
  - id: linear
    eq: [linear]
    inputs: {x: "(B, I)"}
    outputs: {y: "(B, O)"}
    params: "I * O + O"
    flops_forward: "2 * B * I * O"
model:
  params: "linear.params"
`

func TestShapesRules(t *testing.T) {
	// No shapes.yaml: the architecture stays below X2.
	c := fixture(t, map[string]string{archDir + "node.md": strings.ReplaceAll(archNode, "{shape:", "{nothing:")})
	got := fired(Run(c, Options{}), "X09")
	if len(got) != 1 || !strings.Contains(got[0], "level: the architecture has no shapes.yaml") {
		t.Errorf("X09 without shapes.yaml: %q", got)
	}

	// A good file and good references: nothing fires.
	c = fixture(t, map[string]string{archDir + "node.md": archNode, archDir + "shapes.yaml": archShapes})
	fs := Run(c, Options{})
	if got := append(fired(fs, "X09"), fired(fs, "X20")...); len(got) != 0 {
		t.Errorf("a good contract has findings: %q", got)
	}

	// Broken formulas, a missing anchor, and bad references.
	bad := strings.Replace(archShapes, `"I * O + O"`, `"I * O + Z"`, 1)
	bad = strings.Replace(bad, "eq: [linear]", "eq: [linear, bias]", 1)
	page := archNode + "\nAt the big config: {shape:params@big}. Unknown: {shape:nope}.\n"
	c = fixture(t, map[string]string{archDir + "node.md": page, archDir + "shapes.yaml": bad})
	fs = Run(c, Options{})
	x09 := strings.Join(fired(fs, "X09"), "\n")
	for _, want := range []string{"uses Z, which is not", "eq-bias, but node.md has no {#eq-bias} anchor"} {
		if !strings.Contains(x09, want) {
			t.Errorf("X09 has no %q in:\n%s", want, x09)
		}
	}
	x20 := strings.Join(fired(fs, "X20"), "\n")
	for _, want := range []string{`has no config "big"`, "nope is not a name"} {
		if !strings.Contains(x20, want) {
			t.Errorf("X20 has no %q in:\n%s", want, x20)
		}
	}
}

func TestShapesFixture(t *testing.T) {
	files := map[string]string{
		archDir + "node.md":                    archNode,
		archDir + "shapes.yaml":                archShapes,
		archDir + "impl/numpy/toy_net.py":      "# SPDX-License-Identifier: MIT\n",
		archDir + "impl/numpy/test_toy_net.py": "# SPDX-License-Identifier: MIT\n",
	}
	c := fixture(t, files)
	want := "the node has code but no fixtures/shapes.json"
	if got := strings.Join(fired(Run(c, Options{}), "X09"), "\n"); !strings.Contains(got, want) {
		t.Fatalf("X09 has no %q in %q", want, got)
	}
	n := c.BySlug["toy-net"]
	fx, err := n.Shapes.MakeFixture(shapes.Seed(n.Rel))
	if err != nil {
		t.Fatal(err)
	}
	b, _ := fx.Encode()
	p := filepath.Join(n.Dir, "fixtures", "shapes.json")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
	c, _ = node.Load(c.Root)
	if got := fired(Run(c, Options{}), "X09"); len(got) != 0 {
		t.Errorf("a current fixture has findings: %q", got)
	}
	if err := os.WriteFile(p, []byte(strings.Replace(string(b), `"params": 15`, `"params": 16`, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	c, _ = node.Load(c.Root)
	if got := strings.Join(fired(Run(c, Options{}), "X09"), "\n"); !strings.Contains(got, "the fixture is not what shapes.yaml gives") {
		t.Errorf("a stale fixture: %q", got)
	}
}
