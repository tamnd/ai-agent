// SPDX-License-Identifier: Apache-2.0

package lang

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tamnd/ai-agent/node"
)

//go:embed aik_fixtures.py
var fixturesHarness string

// FixturesDir is the directory of the shared fixtures of a node, and FixturesMake the script that writes them (08.7.2).
const (
	FixturesDir  = "fixtures"
	FixturesMake = "make.py"
	manifestFile = "manifest.yaml"
)

// ArrayChange is one array of an .npz file that a run of make.py writes differently.
type ArrayChange struct {
	Name   string   `json:"name"`
	Change string   `json:"change"` // added, removed, values, "dtype a -> b", "shape [..] -> [..]", or "new file"
	Shape  []int    `json:"shape,omitempty"`
	Count  int      `json:"changed,omitempty"` // the number of values that changed
	Size   int      `json:"size,omitempty"`
	MaxAbs *float64 `json:"max_abs,omitempty"`
}

// FixtureChange is one file of fixtures/ that a run of make.py writes differently: an .npz file with its arrays, or
// another file, whose bytes differ.
type FixtureChange struct {
	File   string        `json:"file"`
	Arrays []ArrayChange `json:"arrays,omitempty"`
}

// FixtureRun is the result of running make.py of a node twice (rule X15).
type FixtureRun struct {
	Slug  string `json:"slug"`
	NumPy string `json:"numpy,omitempty"`
	// Changed are the files of the first run that are not the committed ones; Unstable are those of the second run
	// that are not the first's, so make.py is not deterministic.
	Changed  []FixtureChange `json:"changed,omitempty"`
	Unstable []FixtureChange `json:"unstable,omitempty"`
	Error    string          `json:"error,omitempty"`

	made string // the fixtures/ of the first run
}

// OK tells if make.py wrote the committed files, twice.
func (fr *FixtureRun) OK() bool {
	return fr.Error == "" && len(fr.Changed) == 0 && len(fr.Unstable) == 0
}

// MakeFixtures runs fixtures/make.py of a node twice on copies of the node directory, and compares what it writes with
// the committed fixtures and with each other: the .npz files by their arrays, the other files by their bytes, and
// manifest.yaml with the hash of each .npz file whose arrays did not change taken as it was. Call Write to copy the
// changed files to the node, and Close to remove the work tree.
func (r *Runner) MakeFixtures(n *node.Node) *FixtureRun {
	fr := &FixtureRun{Slug: n.Slug}
	src := filepath.Join(n.Dir, FixturesDir)
	if _, err := os.Stat(filepath.Join(src, FixturesMake)); err != nil {
		fr.Error = "the node has no " + FixturesDir + "/" + FixturesMake
		return fr
	}
	dir, err := os.MkdirTemp(r.TempDir, "aik-fixtures-")
	if err != nil {
		fr.Error = err.Error()
		return fr
	}
	fr.made = filepath.Join(dir, "a", FixturesDir)
	defer r.reap()
	for _, sub := range []string{"a", "b"} {
		if err := copyNode(n.Dir, filepath.Join(dir, sub)); err != nil {
			fr.Error = err.Error()
			return fr
		}
	}
	if err := copyNode(src, filepath.Join(dir, "committed")); err != nil {
		fr.Error = err.Error()
		return fr
	}
	if err := write(dir, "aik_fixtures.py", fixturesHarness); err != nil {
		fr.Error = err.Error()
		return fr
	}
	ctx, cancel := context.WithTimeout(context.Background(), Timeout)
	defer cancel()
	py := func(args ...string) (string, error) {
		var out, errOut bytes.Buffer
		cmd := r.command(ctx, FlopLang, dir, append([]string{"python3", "-B"}, args...))
		cmd.Stdout, cmd.Stderr = &out, &errOut
		if err := cmd.Run(); err != nil {
			return "", errors.New(tail(errOut.String()+"\n"+err.Error(), 1500))
		}
		return out.String(), nil
	}
	for _, sub := range []string{"a", "b"} {
		if _, err := py(sub + "/" + FixturesDir + "/" + FixturesMake); err != nil {
			fr.Error = "make.py fails: " + err.Error()
			return fr
		}
	}
	out, err := py("aik_fixtures.py", "committed", "a/"+FixturesDir, "b/"+FixturesDir)
	if err != nil {
		fr.Error = "the comparison fails: " + err.Error()
		return fr
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	var res struct {
		NumPy    string                   `json:"numpy"`
		Changed  map[string][]ArrayChange `json:"changed"`
		Unstable map[string][]ArrayChange `json:"unstable"`
	}
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &res); err != nil {
		fr.Error = "the comparison wrote no result: " + tail(out, 1000)
		return fr
	}
	fr.NumPy = res.NumPy
	// The second run first: the comparison with the committed files sets the hashes of the first run's manifest back.
	if fr.Unstable, err = changes(fr.made, filepath.Join(dir, "b", FixturesDir), res.Unstable); err != nil {
		fr.Error = err.Error()
		return fr
	}
	if fr.Changed, err = changes(filepath.Join(dir, "committed"), fr.made, res.Changed); err != nil {
		fr.Error = err.Error()
	}
	return fr
}

// changes lists the files of new that are not those of old. The .npz files come from the arrays of the comparison;
// manifest.yaml is compared with the hash of each .npz file whose arrays are the same set back to old's, and is
// rewritten so in new.
func changes(old, new string, npz map[string][]ArrayChange) ([]FixtureChange, error) {
	var out []FixtureChange
	var manifest bool
	err := filepath.WalkDir(new, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(new, p)
		rel = filepath.ToSlash(rel)
		switch {
		case strings.HasSuffix(rel, ".npz"):
			if a, ok := npz[rel]; ok {
				out = append(out, FixtureChange{File: rel, Arrays: a})
			}
			return nil
		case rel == manifestFile:
			manifest = true
			return nil
		}
		if !sameBytes(filepath.Join(old, filepath.FromSlash(rel)), p) {
			out = append(out, FixtureChange{File: rel})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if manifest {
		m := filepath.Join(new, manifestFile)
		b, err := os.ReadFile(m)
		if err != nil {
			return nil, err
		}
		text := string(b)
		_ = filepath.WalkDir(new, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(p, ".npz") {
				return err
			}
			rel, _ := filepath.Rel(new, p)
			if _, changed := npz[filepath.ToSlash(rel)]; changed {
				return nil
			}
			if h, h0 := hashOf(p), hashOf(filepath.Join(old, rel)); h != "" && h0 != "" {
				text = strings.ReplaceAll(text, h, h0)
			}
			return nil
		})
		if err := os.WriteFile(m, []byte(text), 0o644); err != nil {
			return nil, err
		}
		if !sameBytes(filepath.Join(old, manifestFile), m) {
			out = append(out, FixtureChange{File: manifestFile})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].File < out[j].File })
	return out, nil
}

// Write copies the changed files of the first run to the fixtures/ of the node and gives their names. The .npz files
// whose arrays did not change keep their bytes.
func (fr *FixtureRun) Write(n *node.Node) ([]string, error) {
	var out []string
	for _, c := range fr.Changed {
		b, err := os.ReadFile(filepath.Join(fr.made, filepath.FromSlash(c.File)))
		if err != nil {
			return out, err
		}
		dst := filepath.Join(n.Dir, FixturesDir, filepath.FromSlash(c.File))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return out, err
		}
		if err := os.WriteFile(dst, b, 0o644); err != nil {
			return out, err
		}
		out = append(out, c.File)
	}
	return out, nil
}

// Close removes the work tree of the run.
func (fr *FixtureRun) Close(keep bool) {
	if fr.made != "" && !keep {
		_ = os.RemoveAll(filepath.Dir(filepath.Dir(fr.made)))
	}
}

func sameBytes(a, b string) bool {
	x, err1 := os.ReadFile(a)
	y, err2 := os.ReadFile(b)
	return err1 == nil && err2 == nil && bytes.Equal(x, y)
}

func hashOf(p string) string {
	b, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// copyNode copies a node directory, without hidden files, __pycache__ and build/.
func copyNode(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if rel != "." && (strings.HasPrefix(d.Name(), ".") || d.Name() == "__pycache__" || (d.IsDir() && rel == "build")) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		to := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(to, 0o755)
		}
		if !d.Type().IsRegular() {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(to, b, 0o644)
	})
}
