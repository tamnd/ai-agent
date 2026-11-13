// SPDX-License-Identifier: Apache-2.0

package shapes

import (
	"math/big"
	"strings"
	"testing"
)

const block = `
config:
  d_model: {meaning: "width of the residual stream", example: 512}
  n_heads: {meaning: "number of attention heads", example: 8}
  d_head:  {meaning: "width of each head", formula: "d_model / n_heads"}
  d_ff:    {meaning: "width of the MLP hidden layer", example: 2048}
  n_layers: {meaning: "number of blocks", symbol: "L", example: 6}
  vocab:   {example: 32000, sample: [5, 20]}
axes:
  B: {meaning: batch, example: 2}
  T: {meaning: sequence length, example: 5}
  H: n_heads
  D: d_head
  M: d_model
  F: d_ff
  V: vocab
blocks:
  - id: attn
    eq: [sdpa]
    inputs:  {x: "(B, T, M)"}
    outputs: {y: "(B, T, M)"}
    internals: {q: "(B, H, T, D)", scores: "(B, H, T, T)"}
    params: "4 * M * M + 4 * M"
    flops_forward: "8 * B * T * M * M + 4 * B * T * T * M"
  - id: mlp
    inputs: {x: "(B, T, M)"}
    outputs: {y: "(B, T, M)"}
    params: "2 * M * F + F + M"
    flops_forward: "4 * B * T * M * F"
  - id: ln1
    params: "2 * M"
  - id: ln2
    params: "2 * M"
  - id: block
    of: [attn, mlp, ln1, ln2]
    params: "4 * M * M + 2 * M * F + 9 * M + F"
model:
  params_non_embedding: {formula: "n_layers * block.params", symbol: "N"}
  params: "params_non_embedding + V * M"
  flops_per_token_forward: "2 * params_non_embedding + 2 * n_layers * T * M"
  sources:
    - {work: vaswani-2017-attention, locator: "Section 3.3 and Table 1"}
configs:
  base: {d_model: 512, n_heads: 8, d_ff: 2048, n_layers: 6, vocab: 37000, source: {work: vaswani-2017-attention, locator: "Table 3"}}
`

func parse(t *testing.T, src string) *File {
	t.Helper()
	f, err := ParseFile([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestTransformerBlock(t *testing.T) {
	f := parse(t, block)
	if ps := f.Check(); len(ps) > 0 {
		t.Fatalf("problems: %v", ps)
	}
	v := f.At(f.Example())
	for name, want := range map[string]int64{
		"d_head":                  64,
		"D":                       64,
		"block.params":            3152384,
		"params":                  6*3152384 + 32000*512,
		"attn.flops_forward":      8*2*5*512*512 + 4*2*5*5*512,
		"block.flops_forward":     8*2*5*512*512 + 4*2*5*5*512 + 4*2*5*512*2048,
		"flops_per_token_forward": 2*6*3152384 + 2*6*5*512,
	} {
		got, err := v.Int(name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got.Cmp(big.NewInt(want)) != 0 {
			t.Errorf("%s = %s, want %d", name, got, want)
		}
	}
	base, ok := f.NamedConfig("base")
	if !ok {
		t.Fatal("no base config")
	}
	got, _ := f.At(base).Int("params")
	if want := int64(6*3152384 + 37000*512); got.Int64() != want {
		t.Errorf("base params = %s, want %d", got, want)
	}
	d, err := v.Dims(f.Block("attn").Internals[1])
	if err != nil || len(d) != 4 || d[0] != 2 || d[1] != 8 || d[2] != 5 || d[3] != 5 {
		t.Errorf("scores dims = %v, %v", d, err)
	}
}

func TestCheckFindsProblems(t *testing.T) {
	cases := map[string]struct{ old, new, want string }{
		"unknown name":   {`"2 * M * F + F + M"`, `"2 * M * F + F + Q"`, "uses Q"},
		"parts disagree": {`"4 * M * M + 2 * M * F + 9 * M + F"`, `"4 * M * M + 2 * M * F + 8 * M + F"`, "its parts attn, mlp, ln1, ln2 add up to"},
		"not whole":      {`n_heads: {meaning: "number of attention heads", example: 8}`, `n_heads: {meaning: "number of attention heads", example: 7}`, "not a whole number"},
		"free axis":      {`B: {meaning: batch, example: 2}`, `B: batch`, "the free axis B (batch) needs an example"},
		"unknown key":    {`eq: [sdpa]`, `equations: [sdpa]`, `unknown key "equations"`},
		"cycle":          {`"params_non_embedding + V * M"`, `"params + V * M"`, "depends on itself"},
		"bad part":       {`of: [attn, mlp, ln1, ln2]`, `of: [attn, mlp, ln1, ln3]`, "ln3, which is not a block"},
		"parse":          {`"2 * M"`, `"2 * * M"`, "unexpected"},
		"named key":      {`d_ff: 2048, n_layers: 6, vocab: 37000`, `d_ff: 2048, d_head: 64, vocab: 37000`, "sets d_head, which is not"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if !strings.Contains(block, c.old) {
				t.Fatalf("the fixture has no %q", c.old)
			}
			f := parse(t, strings.Replace(block, c.old, c.new, 1))
			ps := f.Check()
			var all []string
			for _, p := range ps {
				all = append(all, p.Msg)
			}
			if !strings.Contains(strings.Join(all, "\n"), c.want) {
				t.Errorf("want a problem with %q, got %q", c.want, all)
			}
		})
	}
}

func TestSample(t *testing.T) {
	f := parse(t, block)
	a, err := f.Sample(Seed("nodes/x/transformer"), Samples)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := f.Sample(Seed("nodes/x/transformer"), Samples)
	if len(a) != Samples {
		t.Fatalf("got %d samples", len(a))
	}
	for i := range a {
		if a[i]["n_heads"] == 0 || a[i]["d_model"]%a[i]["n_heads"] != 0 {
			t.Errorf("sample %d: d_model %d is not a multiple of n_heads %d", i, a[i]["d_model"], a[i]["n_heads"])
		}
		if a[i]["vocab"] < 5 || a[i]["vocab"] > 20 || a[i]["d_model"] > 8 {
			t.Errorf("sample %d out of range: %v", i, a[i])
		}
		for k := range a[i] {
			if a[i][k] != b[i][k] {
				t.Errorf("the sample is not deterministic: %v and %v", a[i], b[i])
			}
		}
	}
	fx, err := f.MakeFixture(Seed("nodes/x/transformer"))
	if err != nil {
		t.Fatal(err)
	}
	if len(fx.Configs) != 1+Samples+1 || fx.Configs[0].Kind != "example" || fx.Configs[3].Name != "base" {
		t.Fatalf("configs: %+v", fx.Configs)
	}
	if got := fx.Configs[0].Blocks["attn"].Internals["q"]; len(got) != 4 || got[3] != 64 {
		t.Errorf("q shape = %v", got)
	}
	out, err := fx.Encode()
	if err != nil || !strings.Contains(string(out), `"params": 35298304`) {
		t.Errorf("encode: %v\n%s", err, out)
	}
}

func TestTeX(t *testing.T) {
	g := parse(t, strings.Replace(block, `d_ff:    {meaning: "width of the MLP hidden layer", example: 2048}`, `d_ff:    {meaning: "width of the MLP hidden layer", symbol: "d_{\\text{ff}}", example: 2048}`, 1))
	if got, _ := g.TeX("mlp.params"); got != `2 M d_{\text{ff}} + d_{\text{ff}} + M` {
		t.Errorf("a bound axis with a symbol: %q", got)
	}
	f := parse(t, block)
	for name, want := range map[string]string{
		"attn.params":             `4 M^{2} + 4 M`,
		"attn.flops_forward":      `8 B T M^{2} + 4 B T^{2} M`,
		"d_head":                  `\frac{M}{H}`,
		"block.flops_forward":     `C_{\mathrm{attn}} + C_{\mathrm{mlp}} + C_{\mathrm{ln1}} + C_{\mathrm{ln2}}`,
		"params_non_embedding":    `L P_{\mathrm{block}}`,
		"flops_per_token_forward": `2 N + 2 L T M`,
	} {
		got, err := f.TeX(name)
		if err != nil || got != want {
			t.Errorf("TeX(%s) = %q, %v; want %q", name, got, err, want)
		}
	}
}

func TestParse(t *testing.T) {
	env := func(s string) (*big.Rat, error) { return big.NewRat(3, 1), nil }
	for src, want := range map[string]string{
		"1 + 2 * 3":   "7",
		"(1 + 2) * 3": "9",
		"2 ^ 3 ^ 2":   "512",
		"-x + 10":     "7",
		"x / 2":       "3/2",
		"1_000 * a.b": "3000",
		"x - x - x":   "-3",
		"12 / 4 / 3":  "1",
	} {
		e, err := Parse(src)
		if err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		got, err := Eval(e, env)
		if err != nil || got.RatString() != want {
			t.Errorf("%s = %v, %v; want %s", src, got, err, want)
		}
	}
	for _, bad := range []string{"", "1 +", "(1", "1 $ 2", "a..b", "a."} {
		if _, err := Parse(bad); err == nil {
			t.Errorf("Parse(%q) has no error", bad)
		}
	}
	d, err := ParseShape("(B, H, T, M / (H))")
	if err != nil || len(d) != 4 {
		t.Errorf("ParseShape: %v %v", d, err)
	}
	if d, err := ParseShape("()"); err != nil || len(d) != 0 {
		t.Errorf("scalar shape: %v %v", d, err)
	}
}
