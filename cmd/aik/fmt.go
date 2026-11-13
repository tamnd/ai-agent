// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/tamnd/ai-agent/node"
)

// fmtFiles returns the files that aik fmt owns: node.md, the proofs, the presets, the registers and the topics. It never touches code.
func fmtFiles(root string, c *node.Corpus, only map[string]bool) ([]string, error) {
	var out []string
	for _, n := range c.Nodes {
		if len(only) > 0 && !only[n.Slug] {
			continue
		}
		out = append(out, filepath.Join(n.Dir, "node.md"))
		proofs, err := filepath.Glob(filepath.Join(n.Dir, "proofs", "*.md"))
		if err != nil {
			return nil, err
		}
		out = append(out, proofs...)
		if p := filepath.Join(n.Dir, "viz", "presets.yaml"); exists(p) {
			out = append(out, p)
		}
	}
	if len(only) > 0 {
		return out, nil
	}
	for _, dir := range []string{"registers", "topics"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(p string, d fs.DirEntry, err error) error {
			if os.IsNotExist(err) {
				return filepath.SkipDir
			}
			if err != nil {
				return err
			}
			switch filepath.Ext(p) {
			case ".yaml", ".yml", ".reg", ".jsonl":
				if !d.IsDir() {
					out = append(out, p)
				}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func cmdFmt(args []string) error {
	f := newFlags("fmt", "[slug ...]")
	root := f.String("root", ".", "the root of the content repository")
	checkOnly := f.Bool("check", false, "change no file; exit with 1 when a file is not in canonical form")
	if err := parse(f, args); err != nil {
		return err
	}
	c, err := load(*root)
	if err != nil {
		return err
	}
	only := map[string]bool{}
	for _, s := range f.Args() {
		if c.BySlug[s] == nil {
			return fmt.Errorf("no node has the slug %q", s)
		}
		only[s] = true
	}
	files, err := fmtFiles(*root, c, only)
	if err != nil {
		return err
	}
	bad := 0
	for _, p := range files {
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		var out []byte
		if filepath.Base(p) == "node.md" {
			out, err = node.FormatNode(b)
			if err != nil {
				return fmt.Errorf("%s: %w", rel(*root, p), err)
			}
		} else {
			out = node.FormatText(b)
		}
		if bytes.Equal(b, out) {
			continue
		}
		bad++
		fmt.Println(rel(*root, p))
		if *checkOnly {
			continue
		}
		if err := os.WriteFile(p, out, 0o644); err != nil {
			return err
		}
	}
	if *checkOnly && bad > 0 {
		fmt.Fprintf(os.Stderr, "%d %s not in canonical form; run aik fmt\n", bad, plural(bad, "file is", "files are"))
		return errFindings
	}
	return nil
}

func rel(root, p string) string {
	if r, err := filepath.Rel(root, p); err == nil && !strings.HasPrefix(r, "..") {
		return filepath.ToSlash(r)
	}
	return p
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
