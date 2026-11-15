// SPDX-License-Identifier: Apache-2.0

package lang

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/ai-agent/node"
)

const toyContract = `entry: score
inputs:
  predictions: {type: "list[int]", meaning: the predicted labels}
  references: {type: "list[int]", meaning: the true labels}
options:
  weight: {type: float, default: 1.0, meaning: a factor}
output:
  acc: {type: float, meaning: the share of right labels}
  n: {type: int, meaning: the number of labels}
tolerance: {rtol: 1.0e-9, atol: 1.0e-12}
`

// A make.py in the form of 08.7.5, with the standard library as the official implementation. SCALE changes a score,
// NOISE makes two runs differ, and EXTRA adds an option that the contract does not have.
const toyVectorsMake = `import json
import os
import pathlib

HERE = pathlib.Path(__file__).resolve().parent
SCALE = 1.0
NOISE = False
EXTRA = {}
BY = "toy 1.0"
assert not (HERE.parent / "impl").exists(), "make.py sees impl/"

ROWS = [("half", [1, 2], [1, 3]), ("all", [4, 5, 6], [4, 5, 6])]

rows = []
for rid, p, r in ROWS:
    acc = SCALE * sum(a == b for a, b in zip(p, r)) / len(p)
    if NOISE:
        acc += int.from_bytes(os.urandom(2), "big") / 1e9
    rows.append(json.dumps({"id": rid, "predictions": p, "references": r, "options": EXTRA, "want": {"acc": acc, "n": len(p)}, "by": BY}))
(HERE / "official.jsonl").write_text("\n".join(rows) + "\n")
`

func TestMakeVectors(t *testing.T) {
	if Recipes["python"].Available() != nil {
		t.Skip("no python3")
	}
	n := &node.Node{Dir: t.TempDir()}
	n.Slug = "toy-metric"
	put := func(name, s string) {
		p := filepath.Join(n.Dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	put("contract.yaml", toyContract)
	put("vectors/edge.jsonl", `{"id": "empty", "predictions": [], "references": [], "want": {"acc": 0.0, "n": 0}, "by": "hand", "note": "no labels: 0 by the convention of the page"}`+"\n")
	put("impl/numpy/secret.py", "raise SystemExit('make.py must not see impl/')\n")
	r := &Runner{}
	run := func(script string) *VectorRun {
		t.Helper()
		put("vectors/make.py", script)
		vr := r.MakeVectors(n)
		t.Cleanup(func() { vr.Close(false) })
		if vr.Error != "" {
			t.Fatal(vr.Error)
		}
		return vr
	}
	vr := run(toyVectorsMake)
	if len(vr.Changed) != 1 || !vr.Changed[0].New || len(vr.Changed[0].Rows) != 2 || vr.Invalid != "" || strings.Join(vr.Kept, " ") != "edge.jsonl" {
		t.Fatalf("the first run: %+v", vr)
	}
	if wrote, err := vr.Write(n); err != nil || strings.Join(wrote, " ") != "official.jsonl" {
		t.Fatalf("wrote %v, %v", wrote, err)
	}
	if vr := run(toyVectorsMake); !vr.OK() {
		t.Errorf("a run on the files it wrote: %+v", vr)
	}
	vr = run(strings.Replace(toyVectorsMake, "SCALE = 1.0", "SCALE = 0.5", 1))
	if len(vr.Changed) != 1 || len(vr.Changed[0].Rows) != 2 {
		t.Fatalf("a changed score: %+v", vr.Changed)
	}
	if row := vr.Changed[0].Rows[0]; row.ID != "half" || row.Change != "changed" || len(row.Fields) != 1 || row.Fields[0].Path != "want.acc" || row.Fields[0].Within == nil || *row.Fields[0].Within {
		t.Errorf("the change of half: %+v", row)
	}
	vr = run(strings.Replace(toyVectorsMake, `ROWS = [("half", [1, 2], [1, 3]), `, `ROWS = [("third", [1], [2]), `, 1))
	var got []string
	for _, row := range vr.Changed[0].Rows {
		got = append(got, row.ID+" "+row.Change)
	}
	if strings.Join(got, ", ") != "third added, half removed" {
		t.Errorf("the rows: %v", got)
	}
	if vr := run(strings.Replace(toyVectorsMake, "NOISE = False", "NOISE = True", 1)); strings.Join(vr.Unstable, " ") != "official.jsonl" {
		t.Errorf("a make.py that is not deterministic: %+v", vr)
	}
	if vr := run(strings.Replace(toyVectorsMake, "EXTRA = {}", `EXTRA = {"beam": 4}`, 1)); !strings.Contains(vr.Invalid, "the option beam is not in contract.yaml") {
		t.Errorf("an option of no contract: %+v", vr)
	}
	if vr := run(strings.NewReplacer(`BY = "toy 1.0"`, `BY = "hand"`, `"by": BY}`, `"by": BY, "note": "by hand"}`).Replace(toyVectorsMake)); !strings.Contains(vr.Invalid, "official.jsonl:1: the vector half is computed by hand") {
		t.Errorf("a hand vector in official.jsonl: %q", vr.Invalid)
	}
	put("vectors/make.py", "import sacrebleu_that_is_not_there\n")
	if vr := r.MakeVectors(n); !strings.Contains(vr.Error, "make.py fails") || !strings.Contains(vr.Error, "ModuleNotFoundError") {
		t.Errorf("a missing package: %+v", vr)
	}
}
