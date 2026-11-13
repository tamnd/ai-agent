// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/tamnd/ai-agent/contract"
)

func TestOraclesAgree(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("no python3")
	}
	dir := t.TempDir()
	files := map[string]string{
		"contract.yaml":         "problem: sorting-problem\nentry: sort\nparams:\n  - {name: xs, type: \"list<T>\", mode: inout}\nreturns: {type: unit}\ngeneric: {T: ordered}\nbounds:\n  len(xs): [0, 100]\nchecker: exact\n",
		"cases/basic.jsonl":     `{"name":"two","args":{"xs":[2,1]},"expect":[1,2]}` + "\n",
		"cases/generated.yaml":  "size: len(xs)\nclasses:\n  tiny: {range: [0, 3], count: 0, exhaustive: true}\n  small: {range: [4, 20], count: 10}\ngenerators:\n  - {name: uniform, file: generators/uniform.py}\nseeds:\n  pr: [1]\n",
		"generators/uniform.py": "import itertools\ndef generate(rng, n, T='int'):\n    return {'xs': [rng.choice('abc') if T == 'string' else rng.randint(0, 3) for _ in range(n)]}\ndef exhaustive(n, T='int'):\n    return [{'xs': list(p)} for p in itertools.product([0, 1], repeat=n)] if T == 'int' else []\n",
		"reference.py":          "def sort(xs):\n    xs.sort()\n",
		"brute.py":              "def sort(xs):\n    out = []\n    while xs:\n        m = min(xs)\n        xs.remove(m)\n        out.append(m)\n    xs.extend(out)\n",
	}
	for name, body := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	k, _, err := contract.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if bad, err := oraclesAgree(k, nil, "sorting-problem"); err != nil || bad != 0 {
		t.Fatalf("two right oracles: %d disagreements, %v", bad, err)
	}
	// A brute force that drops repeated items disagrees with the reference.
	if err := os.WriteFile(filepath.Join(dir, "brute.py"), []byte("def sort(xs):\n    xs[:] = sorted(set(xs))\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if bad, err := oraclesAgree(k, nil, "sorting-problem"); err != nil || bad == 0 {
		t.Fatalf("a wrong brute force: %d disagreements, %v", bad, err)
	}
}
