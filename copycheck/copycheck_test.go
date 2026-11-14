// SPDX-License-Identifier: Apache-2.0

package copycheck

import (
	"strings"
	"testing"
)

const source = `Dropout is a technique for addressing this problem. The key idea is to randomly drop units, along with their
connections, from the neural network during training. This prevents units from co-adapting too much. During
training, dropout samples from an exponential number of different thinned networks. At test time, it is easy to
approximate the effect of averaging the predictions of all these thinned networks by simply using a single unthinned
network that has smaller weights. This significantly reduces overfitting and gives major improvements over other
regularization methods.`

func TestSections(t *testing.T) {
	body := "Lead sentence.\n\n## Method\n\nOne line.\n```python\nx = 1\n```\nTwo line.\n\n## Empty\n\n## Last\n\nEnd.\n"
	ss := Sections("n/node.md", body, 10)
	if len(ss) != 3 || ss[0].Name != "lead" || ss[1].Name != "Method" || ss[2].Name != "Last" {
		t.Fatalf("sections %+v", ss)
	}
	if ss[1].Line != 13 || strings.Contains(ss[1].Text, "x = 1") {
		t.Errorf("section Method at line %d: %q", ss[1].Line, ss[1].Text)
	}
	if got := strings.Split(ss[1].Text, "\n")[1]; got != "One line." {
		t.Errorf("the second line of Method is %q, so the lines do not keep their numbers", got)
	}
}

func TestShingles(t *testing.T) {
	src := []Source{{Name: "srivastava-2014", Text: source}}
	copied := Section{Name: "Copied", File: "f", Line: 1, Text: "The key idea is to randomly drop units, along with their connections, from the neural network during training.\nIt helps.\n"}
	fresh := Section{Name: "Fresh", File: "f", Line: 5, Text: "While the network trains, switch off a random part of its units at each step.\nThe network cannot count on any one unit.\n"}
	quoted := Section{Name: "Quoted", File: "f", Line: 9, Text: `Srivastava et al. write that "the key idea is to randomly drop units, along with their connections, from the neural network during training" {#c-key}.` + "\n"}
	formula := Section{Name: "Formula", File: "f", Line: 12, Text: "The key idea is to randomly $p$ drop units, along with $q$ their connections, from the $r$ neural network during training.\n"}
	fs := Prose([]Section{copied, fresh, quoted, formula}, src)
	if len(fs) != 1 || fs[0].Section != "Copied" || fs[0].Test != "shingles" || fs[0].Source != "srivastava-2014" {
		t.Fatalf("findings %+v", fs)
	}
	if f := fs[0]; f.Shared != f.Of || f.Share != 1 {
		t.Errorf("a copied sentence shares %d of %d shingles", f.Shared, f.Of)
	}
}

func TestAligned(t *testing.T) {
	src := []Source{{Name: "srivastava-2014", Text: source}}
	para := []string{
		"The central idea: units are dropped at random, connections included, from the neural network while training.",
		"Such dropping keeps units from co-adapting excessively.",
		"In training, dropout draws samples from exponentially many distinct thinned networks.",
		"At test time, approximating the averaged predictions of the thinned networks is easy with one unthinned network of smaller weights.",
	}
	in := Section{Name: "Para", File: "f", Line: 20, Text: strings.Join(para, "\n") + "\n"}
	fs := Prose([]Section{in}, src)
	var al []Finding
	for _, f := range fs {
		if f.Test == "aligned" {
			al = append(al, f)
		}
	}
	if len(al) != 1 || al[0].Line < 20 || al[0].End != 23 || al[0].Shared < AlignRun {
		t.Fatalf("aligned findings %+v", fs)
	}
	rev := []string{para[3], para[2], para[0]}
	if fs := Prose([]Section{{Name: "Rev", File: "f", Line: 1, Text: strings.Join(rev, "\n")}}, src); len(fs) != 0 {
		t.Errorf("sentences in another order: %+v", fs)
	}
}

func TestCode(t *testing.T) {
	ref := `import numpy as np

def dropout_forward(x, p, rng):
    mask = (rng.random(x.shape) >= p) / (1.0 - p)
    out = x * mask
    cache = (mask,)
    return out, cache

def dropout_backward(dout, cache):
    (mask,) = cache
    dx = dout * mask
    return dx

def relu(x):
    return np.maximum(0, x) + 0 * x - 0 * x
`
	renamed := strings.NewReplacer("dropout_forward", "fwd", "dropout_backward", "bwd", "mask", "m", "cache", "c", "dout", "g", "relu", "rect").Replace(ref)
	fresh := `def train(model, data, epochs):
    for epoch in range(epochs):
        for batch in data:
            loss = model.loss(batch)
            if loss > 10:
                raise ValueError("diverged")
    return model

def accuracy(pred, gold):
    hits = sum(1 for a, b in zip(pred, gold) if a == b)
    return hits / len(gold)
`
	corpus := []Source{{Name: "cs231n/dropout.py", Text: ref}}
	fs := Code([]File{{Path: "a.py", Text: renamed}, {Path: "b.py", Text: fresh}}, corpus)
	if len(fs) != 1 || fs[0].File != "a.py" || fs[0].Share < CodeShare {
		t.Fatalf("findings %+v", fs)
	}
	if got := tokens("import torch\noptimizer.zero_grad()\nx = foo(1, 'a') # note\n"); strings.Join(got, " ") != "v = v ( n , s )" {
		t.Errorf("tokens %q", got)
	}
}
