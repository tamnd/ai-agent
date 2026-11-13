// SPDX-License-Identifier: Apache-2.0

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tamnd/ai-agent/contract"
	"github.com/tamnd/ai-agent/node"
	"github.com/tamnd/ai-agent/status"
)

// changedFiles lists the files that changed between base and HEAD, the changes in the work tree and the new files, relative to the root.
func changedFiles(root, base string) ([]string, error) {
	var files []string
	for _, a := range [][]string{{"diff", "--name-only", base + "...HEAD"}, {"diff", "--name-only", "HEAD"}, {"ls-files", "--others", "--exclude-standard"}} {
		cmd := exec.Command("git", a...)
		cmd.Dir = root
		out, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("git %s: %w", strings.Join(a, " "), err)
		}
		for _, p := range strings.Split(string(out), "\n") {
			if p = strings.TrimSpace(p); p != "" {
				files = append(files, filepath.ToSlash(p))
			}
		}
	}
	return files, nil
}

// affected is the set of implementations that a change touches (spec 09.8).
type affected struct {
	// All is true when a file that every test reads changed: a register, the CI workflow or the toolchain.
	All bool
	// Why tells the reason of All.
	Why string
	// Langs holds the languages to test for each slug. A nil set means all the languages of the node.
	Langs map[string]map[string]bool
}

// Has tells if the implementation in lang of the node n is in the set.
func (a affected) Has(n *node.Node, lang string) bool {
	if a.All {
		return true
	}
	ls, ok := a.Langs[n.Slug]
	return ok && (ls == nil || ls[lang])
}

// globalFile tells if a change to the file can change the result of every test.
func globalFile(p string) bool {
	return strings.HasPrefix(p, "registers/") || strings.HasPrefix(p, ".github/workflows/")
}

// findAffected maps each changed file to the implementations that it touches.
//
//   - A file in impl/<lang>/ of a node touches that implementation only.
//   - Another file of a node, such as node.md, touches all its implementations, because it can change the problems that the node solves.
//   - A file of a problem node, such as contract.yaml or a case file, touches every implementation of every node that solves the problem.
//   - A register or a CI workflow touches everything.
func findAffected(c *node.Corpus, files []string) affected {
	a := affected{Langs: map[string]map[string]bool{}}
	all := func(slug string) { a.Langs[slug] = nil }
	add := func(slug, lang string) {
		ls, ok := a.Langs[slug]
		if ok && ls == nil {
			return
		}
		if ls == nil {
			ls = map[string]bool{}
			a.Langs[slug] = ls
		}
		ls[lang] = true
	}
	solvers := map[string][]string{}
	for _, n := range c.Nodes {
		for _, s := range n.Solves {
			solvers[s.Problem] = append(solvers[s.Problem], n.Slug)
		}
	}
	for _, p := range files {
		if globalFile(p) {
			a.All, a.Why = true, p+" changed"
			return a
		}
		for _, n := range c.Nodes {
			prefix := n.Rel + "/"
			if !strings.HasPrefix(p, prefix) {
				continue
			}
			rest := strings.TrimPrefix(p, prefix)
			if l, ok := strings.CutPrefix(rest, "impl/"); ok {
				if lang, _, ok := strings.Cut(l, "/"); ok {
					add(n.Slug, lang)
					continue
				}
			}
			all(n.Slug)
			for _, s := range solvers[n.Slug] {
				all(s)
			}
		}
	}
	return a
}

// testCache keeps the results of the tests by the hash of everything that they read, so that an unchanged implementation does not run again (spec 09.8).
type testCache struct {
	dir  string
	salt string
	used map[string]bool
}

// newTestCache opens the cache in dir. The salt is the version of aik. A build without a version uses the hash of its own binary, so that a changed aik never reads old results.
func newTestCache(dir, root string) (*testCache, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	salt := ver()
	if salt == "dev" {
		exe, err := os.Executable()
		if err != nil {
			return nil, err
		}
		h, err := hashFile(exe)
		if err != nil {
			return nil, err
		}
		salt = "dev " + h
	}
	if h, err := hashFile(filepath.Join(root, "registers", "languages.yaml")); err == nil {
		salt += " languages " + h
	}
	return &testCache{dir: dir, salt: salt, used: map[string]bool{}}, nil
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// key is the hash of the inputs of the tests of one implementation: aik, the toolchain, the files of the implementation, and the contract and cases of each problem that it solves.
func (tc *testCache) key(tool string, n *node.Node, im node.Impl, ks []*contract.Contract) string {
	h := sha256.New()
	fmt.Fprintf(h, "aik %s\nlang %s\ntool %s\nnode %s\n", tc.salt, im.Lang, tool, n.Slug)
	for _, f := range im.Files {
		fmt.Fprintf(h, "file %s %d\n%s\n", f.Name, len(f.Content), f.Content)
	}
	for _, k := range ks {
		fmt.Fprintf(h, "problem %s\n", k.Problem)
		var paths []string
		_ = filepath.WalkDir(k.Dir, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() && p != k.Dir && d.Name() != "cases" {
				return filepath.SkipDir
			}
			if !d.IsDir() {
				paths = append(paths, p)
			}
			return nil
		})
		sort.Strings(paths)
		for _, p := range paths {
			b, err := os.ReadFile(p)
			if err != nil {
				continue
			}
			rel, _ := filepath.Rel(k.Dir, p)
			fmt.Fprintf(h, "file %s %d\n", filepath.ToSlash(rel), len(b))
			h.Write(b)
		}
	}
	return hex.EncodeToString(h.Sum(nil))[:32]
}

func (tc *testCache) path(lang, key string) string {
	return filepath.Join(tc.dir, lang, key+".json")
}

// Get returns the cached result of the key.
func (tc *testCache) Get(lang, key string) (status.Test, bool) {
	var t status.Test
	b, err := os.ReadFile(tc.path(lang, key))
	if err != nil || json.Unmarshal(b, &t) != nil {
		return t, false
	}
	tc.used[lang+"/"+key] = true
	return t, true
}

// Put keeps a result. Only a result that passes is kept, so that a flaky failure runs again next time.
func (tc *testCache) Put(lang, key string, t status.Test) error {
	tc.used[lang+"/"+key] = true
	if !t.OK() {
		return nil
	}
	b, err := json.Marshal(t)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(tc.dir, lang), 0o755); err != nil {
		return err
	}
	return os.WriteFile(tc.path(lang, key), b, 0o644)
}

// Prune removes the entries of the languages that this run did not use. Call it only after a run of all nodes.
func (tc *testCache) Prune(langs []string) {
	for _, l := range langs {
		ents, err := os.ReadDir(filepath.Join(tc.dir, l))
		if err != nil {
			continue
		}
		for _, e := range ents {
			if k := strings.TrimSuffix(e.Name(), ".json"); !tc.used[l+"/"+k] {
				_ = os.Remove(filepath.Join(tc.dir, l, e.Name()))
			}
		}
	}
}
