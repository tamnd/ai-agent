// SPDX-License-Identifier: Apache-2.0

package site

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/ai-agent/node"
)

func TestShapesSection(t *testing.T) {
	root := filepath.Join(t.TempDir(), "corpus")
	if err := os.CopyFS(root, os.DirFS("../testdata/corpus")); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "nodes", "foundations", "models-of-computation", "toy-net")
	md := "---\ntag: 7TQYNT\nkind: architecture\nslug: toy-net\nname: Toy net\narea: foundations/models-of-computation\nlevel: intro\nsummary: A toy architecture.\n---\n\n## Overview\n\nThe toy net has {shape:params@wide} parameters at the wide config, by the formula {shape:params}. In code: `{shape:params}`.\n\n## Computation and cost\n\nIt is one matrix product.\n"
	sy := "config:\n  d_in: {meaning: input width, example: 4}\n  d_out: {meaning: output width, example: 3}\naxes:\n  B: {meaning: batch, example: 2}\n  I: d_in\n  O: d_out\nblocks:\n  - id: linear\n    inputs: {x: \"(B, I)\"}\n    outputs: {y: \"(B, O)\"}\n    params: \"I * O + O\"\n    flops_forward: \"2 * B * I * O\"\nmodel:\n  params: {formula: \"linear.params\", meaning: all parameters}\nconfigs:\n  wide: {d_in: 1000, d_out: 1000}\n"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, s := range map[string]string{"node.md": md, "shapes.yaml": sy} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	c, err := node.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "site")
	if _, err := Build(c, Options{Out: out}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(out, "toy-net", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	page := string(b)
	for _, want := range []string{
		"has 1,001,000 parameters at the wide config",
		`<h2 id="shapes-and-parameters">Shapes and parameters</h2>`,
		"<caption>Blocks</caption>",
		`<td class="num">15</td>`,
		`<td class="num">1,001,000</td>`,
		"<code>{shape:params}</code>",
		`<span class="shapes-role">in</span> <code>x</code> <span class="katex">`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the page does not contain %q", want)
		}
	}
	if strings.Index(page, `id="shapes-and-parameters"`) > strings.Index(page, `id="computation-and-cost"`) {
		t.Error("the shapes section is not before Computation and cost")
	}
	if strings.Contains(page, "by the formula {shape:") {
		t.Error("the formula reference was not rendered")
	}
}

func TestArchScene(t *testing.T) {
	root := filepath.Join(t.TempDir(), "corpus")
	if err := os.CopyFS(root, os.DirFS("../testdata/corpus")); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "nodes", "foundations", "models-of-computation", "toy-net")
	md := "---\ntag: 7TQYNT\nkind: architecture\nslug: toy-net\nname: Toy net\narea: foundations/models-of-computation\nlevel: intro\nsummary: A toy architecture.\n---\n\n## Overview\n\nThe toy net is two linear layers.\n\n## Computation and cost\n\nIt is two matrix products.\n"
	sy := "config:\n  d_in: {meaning: input width, example: 4}\n  d_out: {meaning: output width, example: 3}\n  n_layers: {meaning: hidden layers, example: 2}\naxes:\n  B: {meaning: batch, example: 2}\n  I: d_in\n  O: d_out\nblocks:\n  - id: linear\n    meaning: one affine map\n    inputs: {x: \"(B, I)\"}\n    outputs: {y: \"(B, O)\"}\n    params: \"I * O + O\"\n    flops_forward: \"2 * B * I * O\"\n  - id: net\n    of: [linear, linear]\nmodel:\n  params: {formula: \"n_layers * net.params\", meaning: all parameters}\nconfigs:\n  wide: {d_in: 1000, d_out: 1000, n_layers: 4}\n  narrow: {d_in: 8, d_out: 2, n_layers: 1}\n"
	scene := "title: Toy net\ncontrols:\n  - {name: d_in, choices: [4, 8, 1000]}\n  - {name: d_out, choices: [2, 3, 1000]}\n  - {name: n_layers, choices: [1, 2, 4]}\n  - {name: B, choices: [2]}\npresets: [example, wide, narrow]\nstack: [net]\nrepeat: {net: n_layers}\nlinks: {linear: binary-search}\n"
	if err := os.MkdirAll(filepath.Join(dir, "viz"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, s := range map[string]string{"node.md": md, "shapes.yaml": sy, "viz/shapes.scene.yaml": scene} {
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(name)), []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	c, err := node.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if c.BySlug["binary-search"] == nil {
		t.Skip("the test corpus has no binary-search node to link to")
	}
	out := filepath.Join(t.TempDir(), "site")
	if _, err := Build(c, Options{Out: out}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(out, "toy-net", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	page := string(b)
	for _, want := range []string{
		`<figure class="arch" aria-labelledby="arch-title">`,
		`<option value="wide">wide</option>`,
		`data-size="d_in" value="4"`,
		`<a href="/binary-search/"><code>linear</code></a> <span class="arch-count">&times; 2</span>`,
		`<span data-v="sizes:n_layers">2</span>`,
		`<span class="num" data-t="linear.in.x">(2, 4)</span>`,
		`<span class="num" data-v="params:net">30</span>`,
		`<dd class="num" data-v="model:params">60</dd>`,
		`<script type="application/json">{"slug":"toy-net"`,
		`<script src="/assets/arch.js"`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the page does not contain %q", want)
		}
	}
	if _, err := os.Stat(filepath.Join(out, "assets", "arch.js")); err != nil {
		t.Errorf("arch.js is not written: %v", err)
	}
}
