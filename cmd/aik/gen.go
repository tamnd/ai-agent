// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tamnd/ai-agent/lang"
	"github.com/tamnd/ai-agent/node"
)

// cmdGen writes the build tree of each language (spec 08.5). Each implementation gets a directory build/lang/<lang>/<node path>/ with a copy of its files, the build files, aik-test.sh, and the adapter of each problem that it solves. build/lang/<lang>/SOURCES maps each copied file back to the content tree.
func cmdGen(args []string) error {
	f := newFlags("gen", "[flags] [slug ...]")
	root := f.String("root", ".", "the root of the content repository")
	only := f.String("lang", "", "a comma list of languages; the default is all")
	out := f.String("out", "build/lang", "the directory of the trees")
	if err := parse(f, args); err != nil {
		return err
	}
	c, err := load(*root)
	if err != nil {
		return err
	}
	nodes, err := selectNodes(c, f.Args())
	if err != nil {
		return err
	}
	want, err := langSet(*only)
	if err != nil {
		return err
	}
	problems := contractsOf(c)
	r := &lang.Runner{Log: func(format string, a ...any) { fmt.Fprintf(os.Stderr, format+"\n", a...) }}
	base := under(*root, *out)
	whole := f.NArg() == 0
	sources := map[string]map[string]string{} // lang, then path in the tree, then path in the content tree
	count := map[string]int{}
	skipped := map[string]bool{}
	for _, n := range nodes {
		for _, im := range n.Impls {
			if len(want) > 0 && !want[im.Lang] || skipped[im.Lang] {
				continue
			}
			rc, ok := lang.Recipes[im.Lang]
			if !ok {
				continue
			}
			if err := rc.Available(); err != nil {
				var skip lang.Skip
				if errors.As(err, &skip) {
					fmt.Fprintf(os.Stderr, "skip %s: %v\n", im.Lang, err)
					skipped[im.Lang] = true
					continue
				}
				return err
			}
			top := filepath.Join(base, im.Lang)
			if sources[im.Lang] == nil {
				sources[im.Lang] = map[string]string{}
				if whole {
					if err := os.RemoveAll(top); err != nil {
						return err
					}
				} else if err := readSources(filepath.Join(top, "SOURCES"), sources[im.Lang]); err != nil {
					return err
				}
			}
			rel := strings.TrimPrefix(filepath.ToSlash(n.Rel), "nodes/")
			dir := genNodeDir(base, im.Lang, n)
			if err := os.RemoveAll(dir); err != nil {
				return err
			}
			for g := range sources[im.Lang] {
				if strings.HasPrefix(g, rel+"/") {
					delete(sources[im.Lang], g)
				}
			}
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
			tree, err := r.Gen(dir, n, im)
			if err != nil {
				return fmt.Errorf("%s/%s: %w", n.Slug, im.Lang, err)
			}
			if err := lang.WriteScript(filepath.Join(dir, "aik-test.sh"), tree.Test); err != nil {
				return err
			}
			for g, p := range tree.Sources {
				sources[im.Lang][rel+"/"+g] = p
			}
			for _, p := range problems(n) {
				if p.err != nil {
					return fmt.Errorf("the contract of a problem that %s solves: %w", n.Slug, p.err)
				}
				ad := filepath.Join(dir, "adapter", p.k.Problem)
				if err := os.MkdirAll(ad, 0o755); err != nil {
					return err
				}
				a, err := r.GenAdapter(ad, n, im, p.k)
				if err != nil {
					fmt.Fprintf(os.Stderr, "no adapter for %s/%s and %s: %v\n", n.Slug, im.Lang, p.k.Problem, err)
					continue
				}
				if err := lang.WriteScript(filepath.Join(ad, "build.sh"), a.Build); err != nil {
					return err
				}
				run := append([]string{"exec"}, a.Run...)
				if err := lang.WriteScript(filepath.Join(ad, "run.sh"), [][]string{run}); err != nil {
					return err
				}
			}
			count[im.Lang]++
		}
	}
	langs := make([]string, 0, len(sources))
	for l := range sources {
		langs = append(langs, l)
	}
	sort.Strings(langs)
	for _, l := range langs {
		if err := writeSources(filepath.Join(base, l, "SOURCES"), sources[l]); err != nil {
			return err
		}
		fmt.Printf("wrote %s: %d implementations\n", filepath.ToSlash(filepath.Join(*out, l)), count[l])
	}
	return nil
}

// readSources reads a SOURCES file into m. A missing file gives nothing.
func readSources(path string, m map[string]string) error {
	fh, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer fh.Close()
	s := bufio.NewScanner(fh)
	for s.Scan() {
		if g, p, ok := strings.Cut(s.Text(), "\t"); ok {
			m[g] = p
		}
	}
	return s.Err()
}

// writeSources writes one line for each file: the path in the tree, a tab, and the path in the content tree.
func writeSources(path string, m map[string]string) error {
	keys := make([]string, 0, len(m))
	for g := range m {
		keys = append(keys, g)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, g := range keys {
		b.WriteString(g + "\t" + m[g] + "\n")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// genNodeDir is the directory of an implementation in the tree of its language.
func genNodeDir(out, lang string, n *node.Node) string {
	return filepath.Join(out, lang, filepath.FromSlash(strings.TrimPrefix(filepath.ToSlash(n.Rel), "nodes/")))
}
