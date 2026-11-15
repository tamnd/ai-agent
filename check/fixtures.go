// SPDX-License-Identifier: Apache-2.0

package check

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tamnd/ai-agent/node"
	"go.yaml.in/yaml/v3"
)

// MaxFixtures is the largest size of the fixtures/ of a node (rule X15).
const MaxFixtures = 1 << 20

// fixtures checks the shared fixtures of a node (spec 08.7.2, rule X15): at most 1 MB, and a manifest.yaml whose sha256
// of each file is the hash of the file. That make.py writes the same files again, so that they are deterministic and
// current, is the run of aik fixtures make -check, which needs the code to run.
func (r *run) fixtures(n *node.Node) {
	dir := filepath.Join(n.Dir, "fixtures")
	rel := n.Rel + "/fixtures"
	var size int64
	data := map[string]bool{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != dir && (strings.HasPrefix(d.Name(), ".") || d.Name() == "__pycache__") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		size += info.Size()
		if k, _ := filepath.Rel(dir, p); filepath.Ext(k) == ".npz" {
			data[filepath.ToSlash(k)] = true
		}
		return nil
	})
	if errors.Is(err, fs.ErrNotExist) {
		return
	}
	if err != nil {
		r.add("X15", Block, rel, 0, "%v", err)
		return
	}
	if size > MaxFixtures {
		r.add("X15", Level, rel, 0, "the fixtures have %d KB, and at most 1024 KB is allowed (08.7.2); make the configs or the inputs smaller", (size+1023)/1024)
	}
	b, err := os.ReadFile(filepath.Join(dir, "manifest.yaml"))
	if errors.Is(err, fs.ErrNotExist) {
		if _, err := os.Stat(filepath.Join(dir, "make.py")); err == nil {
			r.add("X15", Level, rel+"/make.py", 0, "make.py writes no manifest.yaml with the seeds, the configs, the NumPy version and the hash of each file")
		}
		return
	}
	if err != nil {
		r.add("X15", Block, rel+"/manifest.yaml", 0, "%v", err)
		return
	}
	var m struct {
		Files map[string]struct {
			SHA256 string `yaml:"sha256"`
		} `yaml:"files"`
	}
	if err := yaml.Unmarshal(b, &m); err != nil {
		r.add("X15", Block, rel+"/manifest.yaml", 0, "%v", err)
		return
	}
	var names []string
	for name := range m.Files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		want := m.Files[name].SHA256
		got, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name)))
		switch {
		case err != nil:
			r.add("X15", Level, rel+"/manifest.yaml", 0, "the manifest names %s, which is not in fixtures/", name)
		case want == "":
			r.add("X15", Level, rel+"/manifest.yaml", 0, "the manifest has no sha256 of %s", name)
		default:
			h := sha256.Sum256(got)
			if hex.EncodeToString(h[:]) != strings.ToLower(want) {
				r.add("X15", Level, rel+"/"+name, 0, "the file is not the one of the manifest: its sha256 is not %s; run aik fixtures make %s", want, n.Slug)
			}
		}
		delete(data, name)
	}
	var left []string
	for name := range data {
		left = append(left, name)
	}
	sort.Strings(left)
	for _, name := range left {
		r.add("X15", Level, rel+"/"+name, 0, "the manifest has no hash of the file")
	}
}
