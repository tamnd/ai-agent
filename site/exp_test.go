// SPDX-License-Identifier: Apache-2.0

package site

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/ai-agent/exp"
	"github.com/tamnd/ai-agent/node"
)

func TestExperimentBox(t *testing.T) {
	root := filepath.Join(t.TempDir(), "corpus")
	if err := os.CopyFS(root, os.DirFS("../testdata/corpus")); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "nodes", "foundations", "models-of-computation", "toy-model")
	md := "---\ntag: 7TQYMD\nkind: model\nslug: toy-model\nname: Toy model\narea: foundations/models-of-computation\nlevel: intro\nsummary: A small model.\n---\n\n## Overview\n\nThe toy model fits the data {#c-fits}.\n"
	cy := "- id: c-fits\n  type: empirical\n  text: The toy model fits the data.\n  small_scale: yes\n  evidence:\n    - {work: paper, role: original, stance: for}\n"
	design := "id: toy-fit\nclaims: [c-fits]\nquestion: Does the toy fit?\nscale_note: {here: \"Two moons.\", source: \"The paper uses MNIST.\"}\narms:\n  - {id: none, config: {p: 0.0}}\n  - {id: dropout, config: {p: 0.5}}\ndata:\n  - {generator: moons.py}\nseeds: {n: 5, base: 1}\nbudget: cpu-small\nmetrics: [test_loss]\n"
	expect := "claim: c-fits\nprimary: {metric: test_loss, compare: {treatment: dropout, control: none}, direction: lower, test: welch-t, alpha: 0.01, min_effect: {kind: relative, value: 0.1}}\n"
	ed := filepath.Join(dir, exp.Dir, "toy-fit")
	files := map[string]string{
		filepath.Join(dir, "node.md"): md, filepath.Join(dir, "claims.yaml"): cy,
		filepath.Join(ed, exp.DesignFile): design, filepath.Join(ed, exp.ExpectFile): expect, filepath.Join(ed, exp.Script): "\n", filepath.Join(ed, "moons.py"): "\n",
		filepath.Join(ed, exp.Figures, "loss.svg"): "<svg xmlns=\"http://www.w3.org/2000/svg\"/>",
	}
	for p, s := range files {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	es, err := exp.Read(dir, "nodes/foundations/models-of-computation/toy-model")
	if err != nil || len(es) != 1 {
		t.Fatalf("exp.Read gives %d experiments and %v", len(es), err)
	}
	fake := func(_ context.Context, j exp.Job, out string) error {
		loss := 0.15 + float64(j.Seed%3)*0.001
		if j.Config["p"] == 0.5 {
			loss -= 0.05
		}
		b, _ := json.Marshal(exp.Metrics{Final: map[string]float64{"test_loss": loss}})
		return os.WriteFile(filepath.Join(out, exp.MetricsFile), b, 0o644)
	}
	r, err := exp.Run(context.Background(), es[0], exp.Options{Work: t.TempDir(), Exec: fake, Node: "7TQYMD", Image: "local"})
	if err != nil {
		t.Fatal(err)
	}
	if err := exp.Write(es[0], r); err != nil {
		t.Fatal(err)
	}
	c, err := node.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "site")
	if _, err := Build(c, Options{Out: out}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(out, "toy-model", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	page := string(b)
	for _, want := range []string{
		`<h2 id="experiments">Experiments</h2>`,
		`<section class="exp-box" id="exp-toy-fit"`,
		`<strong>Here:</strong> Two moons. <strong>Source:</strong> The paper uses MNIST.`,
		`<p class="exp-verdict verdict-holds">The claim holds at this scale.</p>`,
		"This is a small-scale check, not a reproduction of the paper",
		"<code>aik exp run toy-model/toy-fit</code>",
		`<img src="/toy-model/exp/toy-fit/figures/loss.svg"`,
		"label-measured-here",
		`<th scope="row">Reproduction</th>`,
		"R2</span> holds at small scale",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the page does not contain %q", want)
		}
	}
	if _, err := os.Stat(filepath.Join(out, "toy-model", "exp", "toy-fit", "figures", "loss.svg")); err != nil {
		t.Errorf("the build does not copy the figure: %v", err)
	}
}
