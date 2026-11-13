// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tamnd/ai-agent/node"
)

const aliasesHeader = "# Each line is an old slug and the slug it moved to (spec 04.2). aik mv writes it. The site redirects each old URL, so a line is never removed.\n"

// cmdMv renames a node, moves it to another area, or both. The tag never changes.
func cmdMv(args []string) error {
	f := newFlags("mv", "<slug> <new-slug | top/sub> [top/sub]")
	root := f.String("root", ".", "the root of the content repository")
	dry := f.Bool("dry-run", false, "print the changes and change no file")
	if err := parse(f, args); err != nil {
		return err
	}
	if f.NArg() < 2 || f.NArg() > 3 {
		return usageError{"give a slug, then a new slug, a new area, or both"}
	}
	c, err := load(*root)
	if err != nil {
		return err
	}
	old := f.Arg(0)
	n := c.BySlug[old]
	if n == nil {
		return usageError{fmt.Sprintf("no node has the slug %q", old)}
	}
	slug, area := old, n.Area
	for _, a := range f.Args()[1:] {
		if strings.Contains(a, "/") {
			area = a
		} else {
			slug = a
		}
	}
	if slug == old && area == n.Area {
		return usageError{fmt.Sprintf("%s is already at %s", old, n.Rel)}
	}
	if err := checkTarget(c, old, slug, area); err != nil {
		return err
	}
	dst := filepath.Join(*root, "nodes", filepath.FromSlash(area), slug)
	if exists(dst) {
		return fmt.Errorf("%s exists", rel(*root, dst))
	}

	// Collect the new text of each file first, so a failure changes nothing.
	type change struct {
		path string
		text []byte
	}
	var changes []change
	for _, m := range c.Nodes {
		files := []string{filepath.Join(m.Dir, "node.md")}
		proofs, err := filepath.Glob(filepath.Join(m.Dir, "proofs", "*.md"))
		if err != nil {
			return err
		}
		for _, p := range append(files, proofs...) {
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			out := b
			if slug != old {
				out = node.RenameRefs(out, old, slug)
			}
			if m == n && filepath.Base(p) == "node.md" {
				out = node.SetSlugArea(out, slug, area)
			}
			if !bytes.Equal(b, out) {
				changes = append(changes, change{p, out})
			}
		}
	}
	reg := filepath.Join(*root, "registers", "tags.reg")
	var regText []byte
	if slug != old {
		if regText, err = renameInTags(reg, old, slug); err != nil {
			return err
		}
	}

	fmt.Printf("move %s to %s\n", n.Rel, rel(*root, dst))
	for _, ch := range changes {
		fmt.Printf("edit %s\n", rel(*root, ch.path))
	}
	if slug != old {
		fmt.Printf("edit registers/tags.reg\nadd  registers/aliases.reg: %s %s\n", old, slug)
	}
	if *dry {
		return nil
	}
	for _, ch := range changes {
		if err := os.WriteFile(ch.path, ch.text, 0o644); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := os.Rename(n.Dir, dst); err != nil {
		return err
	}
	removeEmptyParents(filepath.Dir(n.Dir), filepath.Join(*root, "nodes"))
	if slug == old {
		return nil
	}
	if err := os.WriteFile(reg, regText, 0o644); err != nil {
		return err
	}
	return addAlias(filepath.Join(*root, "registers", "aliases.reg"), old, slug)
}

func checkTarget(c *node.Corpus, old, slug, area string) error {
	if slug != old {
		switch {
		case !slugRE.MatchString(slug) || len(slug) > 60:
			return usageError{fmt.Sprintf("%q is not lowercase kebab-case of at most 60 characters", slug)}
		case c.BySlug[slug] != nil:
			return usageError{fmt.Sprintf("the slug %q is in use at %s", slug, c.BySlug[slug].Rel)}
		case c.Reserved[slug]:
			return usageError{fmt.Sprintf("the slug %q is a reserved word of the site", slug)}
		case c.OldSlugs[slug] != "":
			return usageError{fmt.Sprintf("the slug %q is an old slug that redirects to %q; an old URL never changes its target", slug, c.OldSlugs[slug])}
		}
	}
	if a := c.AreaByID[area]; a == nil || a.Parent == "" {
		return usageError{fmt.Sprintf("%q is not a sub-area in topics/areas.yaml", area)}
	}
	return nil
}

// renameInTags returns tags.reg with the slug of the line of old changed to slug.
func renameInTags(path, old, slug string) ([]byte, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.SplitAfter(string(b), "\n")
	for i, l := range lines {
		fs := strings.Fields(l)
		if len(fs) >= 2 && !strings.HasPrefix(fs[0], "#") && fs[1] == old {
			lines[i] = strings.Replace(l, old, slug, 1)
		}
	}
	return []byte(strings.Join(lines, "")), nil
}

// addAlias appends "old slug" to aliases.reg. An older line that points to old now points to slug, so a redirect is never a chain.
func addAlias(path, old, slug string) error {
	b, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if len(b) == 0 {
		b = []byte(aliasesHeader)
	}
	lines := strings.SplitAfter(string(b), "\n")
	for i, l := range lines {
		fs := strings.Fields(l)
		if len(fs) == 2 && !strings.HasPrefix(fs[0], "#") && fs[1] == old {
			lines[i] = fs[0] + " " + slug + "\n"
		}
	}
	text := strings.Join(lines, "")
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	text += old + " " + slug + "\n"
	return os.WriteFile(path, []byte(text), 0o644)
}

// removeEmptyParents removes empty directories from dir up to stop, so a move out of the last node of an area leaves no empty directory.
func removeEmptyParents(dir, stop string) {
	for dir != stop && strings.HasPrefix(dir, stop) {
		if os.Remove(dir) != nil {
			return
		}
		dir = filepath.Dir(dir)
	}
}
