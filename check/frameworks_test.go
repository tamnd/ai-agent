// SPDX-License-Identifier: Apache-2.0

package check

import (
	"slices"
	"strings"
	"testing"
)

const frameworksReg = `- {id: numpy, name: NumPy, tier: 1, language: python, ext: [py], file_case: snake, packages: numpy-ref}
- {id: pytorch, name: PyTorch, tier: 1, language: python, ext: [py], file_case: snake, packages: pytorch-ref}
- {id: jax, name: JAX, tier: 1, language: python, ext: [py], file_case: snake, packages: jax-ref}
`

const pyHeader = "# SPDX-License-Identifier: MIT\n# Part of https://ai.tamnd.com/toy-model/ (ai://7TQYMD). The authors are listed in prov/7TQYMD.jsonl.\n"

func TestFrameworkRules(t *testing.T) {
	long := pyHeader + strings.Repeat("x = 1\n", 210)
	c := fixture(t, map[string]string{
		"registers/frameworks.yaml":            frameworksReg,
		modelDir + "node.md":                   modelNode,
		modelDir + "impl/pytorch/toy.py":       pyHeader + "import torch\nimport numpy as np\nimport os, sys\nfrom . import util\nfrom optax import adam\nimport sklearn.linear_model\n",
		modelDir + "impl/pytorch/test_toy.py":  pyHeader + "import pytest\nimport hypothesis\nfrom toy import forward\nimport pandas\n",
		modelDir + "impl/pytorch/Toy-Bad.py":   pyHeader,
		modelDir + "impl/pytorch/notes.txt":    pyHeader,
		modelDir + "impl/jax/toy.py":           "# SPDX-License-Identifier: MIT\nimport jax\n",
		modelDir + "impl/jax/big.py":           long,
		modelDir + "impl/jax/big_efficient.py": long,
		modelDir + "impl/jax/plain.py":         "import jax\n",
		modelDir + "impl/tensorflow/toy.py":    pyHeader,
	})
	if !c.Frameworks || c.LangFile != "registers/frameworks.yaml" {
		t.Fatalf("the corpus reads %s, not the framework register", c.LangFile)
	}
	fs := Run(c, Options{})
	for rule, want := range map[string][]string{
		"X01": {"impl/pytorch: level: the node has PyTorch code but no NumPy reference", "impl/jax: level: the node has JAX code but no NumPy reference"},
		"X07": {
			"impl/pytorch/toy.py: block: the code imports optax, which is not in the package set pytorch-ref of PyTorch",
			"impl/pytorch/toy.py: block: the code imports sklearn",
			"impl/pytorch/test_toy.py: block: the tests import pandas",
		},
		"X12": {
			`impl/pytorch/Toy-Bad.py: level: the file name "Toy-Bad.py" is not snake_case`,
			`impl/pytorch/notes.txt: level: the file has the extension "txt"`,
			`impl/tensorflow: level: the framework "tensorflow" is not in registers/frameworks.yaml`,
		},
		"X14": {"impl/jax/big.py: warn: the reference version has 212 lines"},
		"X19": {
			"impl/jax/toy.py: level: the header does not name the page and the tag of the node; write `Part of https://ai.tamnd.com/toy-model/ (ai://7TQYMD).`",
			"impl/jax/plain.py: level: the file does not start with the licence header",
		},
	} {
		got := fired(fs, rule)
		for _, w := range want {
			if !slices.ContainsFunc(got, func(s string) bool { return strings.Contains(s, w) }) {
				t.Errorf("%s does not report %q; it reports:\n%s", rule, w, strings.Join(got, "\n"))
			}
		}
	}
	for _, f := range fs {
		bad := (f.Rule == "X07" && (strings.Contains(f.Msg, " torch,") || strings.Contains(f.Msg, " numpy,") || strings.Contains(f.Msg, " os,") || strings.Contains(f.Msg, "pytest") || strings.Contains(f.Msg, "hypothesis") || strings.Contains(f.Msg, " toy,"))) ||
			(f.Rule == "X14" && strings.Contains(f.Path, "big_efficient"))
		if bad {
			t.Errorf("a good file gives %s %s: %s", f.Rule, f.Path, f.Msg)
		}
	}
}

func TestFrameworkNumPyFirst(t *testing.T) {
	c := fixture(t, map[string]string{
		"registers/frameworks.yaml":           frameworksReg,
		modelDir + "node.md":                  modelNode,
		modelDir + "impl/numpy/toy.py":        pyHeader + "import numpy as np\n",
		modelDir + "impl/pytorch/toy.py":      pyHeader + "import torch\n",
		modelDir + "impl/pytorch/test_toy.py": pyHeader + "import torch\nimport toy\nimport scipy.special\n",
	})
	for _, f := range Run(c, Options{}) {
		if strings.HasPrefix(f.Path, modelDir) && f.Rule[0] == 'X' {
			t.Errorf("a good node gives %s %s: %s", f.Rule, f.Path, f.Msg)
		}
	}
}

func TestPyImports(t *testing.T) {
	src := "import a.b as c, d\nfrom e.f import g\nfrom . import h\nfrom ..i import j\n  import k  # note\nx = 'import no'\n"
	want := []string{"a.b", "d", "k", "e.f", "", ""}
	if got := pyImports(src); !slices.Equal(got, want) {
		t.Errorf("pyImports gives %q, not %q", got, want)
	}
}
