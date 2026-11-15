// SPDX-License-Identifier: Apache-2.0

package lang

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/tamnd/ai-agent/node"
	"github.com/tamnd/ai-agent/shapes"
)

const toyShapes = `config:
  d_in: {meaning: input width, example: 3, sample: [2, 5]}
  d_hid: {meaning: hidden width, example: 4, sample: [2, 5]}
  d_out: {meaning: output width, example: 2, sample: [2, 3]}
axes:
  B: {meaning: batch, example: 2, sample: [1, 3]}
  I: d_in
  H: d_hid
  O: d_out
blocks:
  - id: up
    inputs: {x: "(B, I)"}
    outputs: {h: "(B, H)"}
    params: "I * H + H"
    flops_forward: "2 * B * I * H"
  - id: down
    inputs: {h: "(B, H)"}
    outputs: {y: "(B, O)"}
    params: "H * O"
    flops_forward: "2 * B * H * O"
model:
  params: "up.params + down.params"
  flops_forward: "up.flops_forward + down.flops_forward + 2 * B * O * O"
  flops_all: "up.flops_forward + down.flops_forward + 4 * B * O * O"
`

// The forward pass has a product of each kind that the count follows, an activation and a softmax that it does not, and
// a call to np.convolve that it reports. The product y @ eye is not in the formula flops_forward, so its count is
// more; flops_all has it.
const toyNumpy = `import numpy as np


def init(rng, d_in, d_hid, d_out):
    return {"up": {"W": rng.normal(size=(d_in, d_hid)), "b": np.zeros(d_hid)}, "down": [rng.normal(size=(d_hid, d_out))]}


def forward(p, x, convolve=False):
    h = np.maximum(np.einsum("bi,ih->bh", x, p["up"]["W"]) + p["up"]["b"], 0.0)
    y = h.dot(p["down"][0])
    e = np.exp(y - y.max(axis=-1, keepdims=True))
    y = e / e.sum(axis=-1, keepdims=True)
    gram = np.tensordot(y, y, axes=((0,), (0,)))  # (O, O) over the batch: 2 * B * O * O
    if convolve:
        np.convolve(y[0], y[0])
    return y @ np.eye(y.shape[-1]) * 0 + y, gram
`

func toyFlopNode(t *testing.T, test string) *node.Node {
	t.Helper()
	f, err := shapes.ParseFile([]byte(toyShapes))
	if err != nil {
		t.Fatal(err)
	}
	n := &node.Node{Dir: t.TempDir(), Rel: "nodes/x/toy", Shapes: f}
	n.Slug, n.Kind = "toy", "architecture"
	n.Impls = []node.Impl{{Lang: "numpy", Files: []node.File{{Name: "toy.py", Content: toyNumpy}, {Name: "test_toy.py", Content: test, Test: true}}}}
	return n
}

func TestFlopCount(t *testing.T) {
	if exec.Command("python3", "-c", "import numpy").Run() != nil {
		t.Skip("python3 has no numpy")
	}
	test := `import numpy as np
from toy import forward, init


def flop_case(c):
    k = c["config"]
    rng = np.random.default_rng(0)
    return forward, (init(rng, k["d_in"], k["d_hid"], k["d_out"]), rng.normal(size=(k["B"], k["I"]))), {"convolve": c["kind"] == "sample"}
`
	r := &Runner{}
	rep := r.FlopCount(toyFlopNode(t, test), "flops_forward")
	if rep.Error != "" || len(rep.Configs) != 3 {
		t.Fatalf("%+v", rep)
	}
	ex := rep.Configs[0]
	// 2*2*3*4 + 2*2*4*2 + 2*2*2*2 for the gram + 2*2*2*2 for y @ eye = 48 + 32 + 16 + 16.
	if ex.Config != "example" || ex.Counted.Int64() != 112 || ex.Formula.Int64() != 96 || ex.OK() || rep.OK() {
		t.Errorf("the example: counted %v, formula %v", ex.Counted, ex.Formula)
	}
	if all := r.FlopCount(toyFlopNode(t, test), "flops_all"); !all.OK() {
		t.Errorf("the total with every product: %+v", all.Configs)
	}
	if ex.Uncounted != nil || rep.Configs[1].Uncounted["np.convolve"] != 1 {
		t.Errorf("uncounted: %v, %v", ex.Uncounted, rep.Configs[1].Uncounted)
	}
	if rep := r.FlopCount(toyFlopNode(t, test), "nothing"); !strings.Contains(rep.Error, "no total nothing") {
		t.Errorf("an unknown total: %+v", rep)
	}
	if rep := r.FlopCount(toyFlopNode(t, "import toy\n"), "flops_forward"); !strings.Contains(rep.Error, "no flop_case") {
		t.Errorf("no flop_case: %+v", rep)
	}
	bad := strings.Replace(test, `return forward,`, `return (lambda p, x, convolve: 1 / 0),`, 1)
	if rep := r.FlopCount(toyFlopNode(t, bad), "flops_forward"); rep.Error != "" || !strings.Contains(rep.Configs[0].Error, "ZeroDivisionError") {
		t.Errorf("a forward that fails: %+v", rep)
	}
}
