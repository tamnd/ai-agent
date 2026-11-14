// SPDX-License-Identifier: Apache-2.0

package viz

import (
	"encoding/json"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"github.com/tamnd/ai-agent/shapes"
)

func archFile(t *testing.T) *shapes.File {
	t.Helper()
	f, err := shapes.Read("testdata/arch", "testdata/arch")
	if err != nil || f == nil {
		t.Fatalf("read: %v", err)
	}
	if ps := f.Check(); len(ps) > 0 {
		t.Fatalf("shapes.yaml: %v", ps)
	}
	return f
}

func links(slug string) (string, bool) {
	switch slug {
	case "layer-normalization@al":
		return "https://algorithm.tamnd.com/layer-normalization/", true
	case "dropout":
		return "/dropout/", true
	}
	return "", false
}

func TestGenArch(t *testing.T) {
	f := archFile(t)
	s := GenArch(f, "Transformer")
	if want := []string{"embed", "enc_layer", "dec_layer", "logits"}; !slices.Equal(s.Stack, want) {
		t.Errorf("stack: got %v, want %v", s.Stack, want)
	}
	if want := []string{"example", "base", "big"}; !slices.Equal(s.Presets, want) {
		t.Errorf("presets: got %v, want %v", s.Presets, want)
	}
	var names []string
	for _, c := range s.Controls {
		names = append(names, c.Name)
	}
	if want := []string{"d_model", "n_heads", "d_ff", "n_layers", "vocab", "B", "S", "T"}; !slices.Equal(names, want) {
		t.Errorf("controls: got %v, want %v", names, want)
	}
	if got := s.Controls[0].Choices; !slices.Equal(got, []int64{16, 512, 1024}) {
		t.Errorf("d_model choices: got %v", got)
	}
	if got := s.Controls[4].Choices; !slices.Equal(got, []int64{5, 20, 50, 37000}) {
		t.Errorf("vocab choices, with the sample range: got %v", got)
	}
	b, err := s.Encode()
	if err != nil {
		t.Fatal(err)
	}
	back, err := ParseArch(b)
	if err != nil {
		t.Fatalf("the written scene does not parse: %v\n%s", err, b)
	}
	if !slices.Equal(back.Stack, s.Stack) || len(back.Controls) != len(s.Controls) {
		t.Errorf("round trip: got %+v", back)
	}
}

func TestParseArchRejectsUnknownKeys(t *testing.T) {
	if _, err := ParseArch([]byte("title: x\nblocks: [a]\n")); err == nil {
		t.Error("an unknown key parses")
	}
}

func TestCompileArchFails(t *testing.T) {
	f := archFile(t)
	s := GenArch(f, "Transformer")
	s.Presets = []string{"example", "huge"}
	s.Controls = s.Controls[1:]
	s.Stack = append(s.Stack, "nope")
	s.Links = map[string]string{"norm": "no-such-node"}
	s.Repeat = map[string]string{"enc_layer": "B"}
	_, fails := CompileArch("transformer", f, s, links)
	for _, want := range []string{"the preset huge", "2 of the 3 presets", "the size d_model has no control", "the stack names nope", "no-such-node", "repeats B times"} {
		if !slices.ContainsFunc(fails, func(m string) bool { return strings.Contains(m, want) }) {
			t.Errorf("no failure says %q; got %q", want, fails)
		}
	}
}

func TestCheckArch(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is not installed")
	}
	f := archFile(t)
	s := GenArch(f, "Transformer")
	s.Repeat = map[string]string{"enc_layer": "n_layers", "dec_layer": "n_layers"}
	s.Links = map[string]string{"norm": "layer-normalization@al"}
	d, fails := CompileArch("transformer", f, s, links)
	if len(fails) > 0 {
		t.Fatalf("compile: %v", fails)
	}
	samples, err := f.Sample(1, 20)
	if err != nil {
		t.Fatal(err)
	}
	r := CheckArch(f, d, samples)
	if r.Skipped != "" || len(r.Fails) > 0 {
		t.Fatalf("check: skipped %q, fails %v", r.Skipped, r.Fails)
	}
	if len(r.Steps) != 23 {
		t.Errorf("runs: got %d, want 3 presets and 20 samples", len(r.Steps))
	}

	// With a large batch and long sequences, the FLOPs of the big model pass 2^53, so an evaluator on floats would
	// lose digits; the check compares every digit.
	huge, _ := f.NamedConfig("big")
	huge["B"], huge["S"], huge["T"] = 1_000_003, 100_003, 100_019
	want, err := Expected(f, d, huge)
	if err != nil || len(want.Model["flops_forward"]) <= 16 {
		t.Fatalf("expected: %v %v", want.Model, err)
	}
	if r := CheckArch(f, d, []map[string]int64{huge}); len(r.Fails) > 0 {
		t.Errorf("the scene loses digits past 2^53: %v", r.Fails)
	}

	// A formula of the scene that differs from shapes.yaml is caught, and so is a shape.
	for i := range d.Blocks {
		switch d.Blocks[i].ID {
		case "enc_ffn":
			d.Blocks[i].Params = map[string]any{"o": "*", "a": "2", "b": map[string]any{"r": "M"}}
		case "logits":
			d.Blocks[i].Tensors[1].Dims[2] = map[string]any{"r": "M"}
		}
	}
	r = CheckArch(f, d, nil)
	for _, want := range []string{"the parameters of enc_ffn", "(V14)", "logits.out.logits", "(V07)"} {
		if !slices.ContainsFunc(r.Fails, func(m string) bool { return strings.Contains(m, want) }) {
			t.Errorf("no failure says %q; got %q", want, r.Fails)
		}
	}
}

// A config that makes a size a fraction, as d_model 10 with 4 heads, is an error in both evaluators.
func TestCheckArchFraction(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is not installed")
	}
	f := archFile(t)
	d, _ := CompileArch("transformer", f, GenArch(f, "Transformer"), links)
	in := f.Example()
	in["d_model"] = 10
	r := CheckArch(f, d, []map[string]int64{in})
	if len(r.Fails) > 0 {
		t.Errorf("a config that both reject fails the check: %v", r.Fails)
	}
}

// A contract with no derived config variable, or no bound axis, gives empty lists, not null, so that the scene runs.
func TestCheckArchNoDerived(t *testing.T) {
	f, err := shapes.Read("testdata/arch-plain", "testdata/arch-plain")
	if err != nil || f == nil {
		t.Fatalf("read: %v", err)
	}
	if ps := f.Check(); len(ps) > 0 {
		t.Fatalf("shapes.yaml: %v", ps)
	}
	d, fails := CompileArch("linear", f, GenArch(f, "Linear"), links)
	if len(fails) > 0 {
		t.Fatalf("compile: %v", fails)
	}
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "null") {
		t.Errorf("the scene data has a null: %s", b)
	}
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is not installed")
	}
	samples, err := f.Sample(1, 5)
	if err != nil {
		t.Fatal(err)
	}
	if r := CheckArch(f, d, samples); r.Skipped != "" || len(r.Fails) > 0 {
		t.Errorf("check: skipped %q, fails %v", r.Skipped, r.Fails)
	}
}
