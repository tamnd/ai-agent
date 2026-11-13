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
