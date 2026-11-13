// SPDX-License-Identifier: Apache-2.0

package metric

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const contract = `entry: score
inputs:
  predictions: {type: "list[str]", meaning: one hypothesis per segment}
  references: {type: "list[list[str]]", meaning: the references of each segment}
options:
  lowercase: {type: bool, default: false, meaning: fold the case first}
output:
  score: {type: float, meaning: the score}
tolerance: {rtol: 1.0e-9, atol: 1.0e-9}
`

func TestParse(t *testing.T) {
	k, err := Parse([]byte(contract))
	if err != nil {
		t.Fatal(err)
	}
	if k.Entry != "score" || k.Tolerance.Rtol != 1e-9 || k.Options["lowercase"] == nil {
		t.Errorf("got %+v", k)
	}
	for _, c := range []struct{ in, want string }{
		{strings.Replace(contract, "  predictions:", "  hyps:", 1), "the inputs have no predictions"},
		{strings.Replace(contract, "rtol: 1.0e-9, atol: 1.0e-9", "rtol: 0, atol: 0", 1), "the tolerance"},
		{strings.Replace(contract, "entry: score", "entry: 2score", 1), "is not a Python name"},
		{strings.Replace(contract, ", meaning: the score", "", 1), "score needs a type and a meaning"},
		{contract + "extra: 1\n", "field extra not found"},
	} {
		if _, err := Parse([]byte(c.in)); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("got %v, want %q", err, c.want)
		}
	}
}

func TestReadVectors(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, VectorsDir), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, s string) {
		if err := os.WriteFile(filepath.Join(dir, VectorsDir, name), []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("official.jsonl", `{"id": "a", "predictions": ["x"], "references": [["x"]], "want": {"score": 100}, "by": "sacrebleu 2.5.1"}`+"\n\n")
	write("edge.jsonl", `{"id": "b", "predictions": [""], "references": [["x"]], "want": {"score": 0}, "by": "hand", "note": "the official one rejects it"}`+"\n")
	k, _ := Parse([]byte(contract))
	vs, err := ReadVectors(dir, k)
	if err != nil || len(vs) != 2 || vs[0].File != "vectors/edge.jsonl" || vs[1].Line != 1 {
		t.Fatalf("got %+v, %v", vs, err)
	}
	write("edge.jsonl", `{"id": "a", "predictions": [""], "references": [["x"]], "options": {"smooth": "exp"}, "want": {"bleu": 0}, "by": "hand"}`+"\n")
	_, err = ReadVectors(dir, k)
	for _, want := range []string{"the id a is also at", "needs a note", "the option smooth is not in", "the output bleu is not in"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("got %v, want %q", err, want)
		}
	}
	write("edge.jsonl", `{"id": "c", "predictions": [], "references": [], "want": 1, "by": "hand", "note": "n", "extra": 1}`+"\n")
	if _, err := ReadVectors(dir, k); err == nil || !strings.Contains(err.Error(), "edge.jsonl: line 1") {
		t.Errorf("an unknown field: got %v", err)
	}
}
