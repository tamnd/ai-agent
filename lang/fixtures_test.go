// SPDX-License-Identifier: Apache-2.0

package lang

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/ai-agent/node"
)

// A make.py in the form of 08.7.2: it loads the reference, writes an .npz file and a manifest with its hash. SCALE
// changes the arrays, and NOISE makes two runs differ.
const toyMake = `import hashlib
import os
import pathlib

import numpy as np

HERE = pathlib.Path(__file__).resolve().parent
SCALE = 2.0
NOISE = False


def main():
    rng = np.random.default_rng(7)
    x = rng.normal(size=(3, 4))
    y = SCALE * x + (np.frombuffer(os.urandom(8), dtype=np.uint64)[0] % 1000 if NOISE else 0)
    with open(HERE / "expected-small.npz", "wb") as f:
        np.savez(f, x=x, y=y, n=np.arange(3))
    h = hashlib.sha256((HERE / "expected-small.npz").read_bytes()).hexdigest()
    (HERE / "manifest.yaml").write_text(f"seed: 7\nfiles:\n  expected-small.npz: {{sha256: {h}}}\n")


main()
`

func TestMakeFixtures(t *testing.T) {
	if exec.Command("python3", "-c", "import numpy").Run() != nil {
		t.Skip("python3 has no numpy")
	}
	n := &node.Node{Dir: t.TempDir()}
	n.Slug = "toy"
	put := func(script string) {
		p := filepath.Join(n.Dir, "fixtures", "make.py")
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(script), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	r := &Runner{}
	run := func() *FixtureRun {
		t.Helper()
		fr := r.MakeFixtures(n)
		t.Cleanup(func() { fr.Close(false) })
		if fr.Error != "" {
			t.Fatal(fr.Error)
		}
		return fr
	}
	put(toyMake)
	fr := run()
	if len(fr.Changed) != 2 || fr.Changed[0].File != "expected-small.npz" || fr.Changed[0].Arrays[0].Change != "new file" || fr.Changed[1].File != "manifest.yaml" || len(fr.Unstable) != 0 {
		t.Fatalf("the first run: %+v", fr)
	}
	if wrote, err := fr.Write(n); err != nil || strings.Join(wrote, " ") != "expected-small.npz manifest.yaml" {
		t.Fatalf("wrote %v, %v", wrote, err)
	}
	// np.savez puts the time in the zip, so the bytes change on each run; the arrays and the manifest do not.
	if fr := run(); !fr.OK() {
		t.Errorf("a run on the files it wrote: %+v", fr)
	}
	put(strings.Replace(toyMake, "SCALE = 2.0", "SCALE = 2.5", 1))
	fr = run()
	if len(fr.Changed) != 2 || len(fr.Changed[0].Arrays) != 1 {
		t.Fatalf("a changed reference: %+v", fr)
	}
	if a := fr.Changed[0].Arrays[0]; a.Name != "y" || a.Change != "values" || a.Count != 12 || a.Size != 12 || a.MaxAbs == nil || *a.MaxAbs <= 0 {
		t.Errorf("the change of y: %+v", a)
	}
	put(strings.Replace(toyMake, "NOISE = False", "NOISE = True", 1))
	if fr := run(); len(fr.Unstable) != 2 || fr.Unstable[0].Arrays[0].Name != "y" {
		t.Errorf("a make.py that is not deterministic: %+v", fr.Unstable)
	}
	if fr := r.MakeFixtures(&node.Node{Dir: t.TempDir()}); !strings.Contains(fr.Error, "no fixtures/make.py") {
		t.Errorf("no make.py: %+v", fr)
	}
}
