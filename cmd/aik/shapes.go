// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tamnd/ai-agent/node"
	"github.com/tamnd/ai-agent/shapes"
)

// cmdShapes works with the shape contract of an architecture (spec 08.6): sample writes fixtures/shapes.json, and show prints the formulas and their values.
func cmdShapes(args []string) error {
	if len(args) == 0 || (args[0] != "sample" && args[0] != "show") {
		return usageError{"use aik shapes sample [-w] slug... or aik shapes show slug..."}
	}
	sub := args[0]
	f := newFlags("shapes "+sub, "[flags] slug...")
	root := f.String("root", ".", "the root of the content repository")
	write := f.Bool("w", false, "write fixtures/shapes.json in each node; without it, print the fixture")
	if err := parse(f, args[1:]); err != nil {
		return err
	}
	if f.NArg() == 0 {
		return usageError{"name at least one architecture node"}
	}
	c, err := load(*root)
	if err != nil {
		return err
	}
	for _, slug := range f.Args() {
		n := c.BySlug[slug]
		if n == nil {
			return usageError{fmt.Sprintf("no node has the slug %q", slug)}
		}
		if n.Shapes == nil {
			return fmt.Errorf("%s has no %s", n.Rel, shapes.FileName)
		}
		if sub == "show" {
			if err := showShapes(n); err != nil {
				return err
			}
			continue
		}
		fx, err := n.Shapes.MakeFixture(shapes.Seed(n.Rel))
		if err != nil {
			return err
		}
		b, err := fx.Encode()
		if err != nil {
			return err
		}
		if !*write {
			if _, err := os.Stdout.Write(b); err != nil {
				return err
			}
			continue
		}
		p := filepath.Join(n.Dir, filepath.FromSlash(shapes.FixtureFile))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(p, b, 0o644); err != nil {
			return err
		}
		fmt.Printf("wrote %s/%s with %d configs\n", n.Rel, shapes.FixtureFile, len(fx.Configs))
	}
	return nil
}

func showShapes(n *node.Node) error {
	f := n.Shapes
	if ps := f.Check(); len(ps) > 0 {
		for _, p := range ps {
			fmt.Fprintf(os.Stderr, "%s: %v\n", f.Path, p)
		}
		return fmt.Errorf("%s has %d problems", f.Path, len(ps))
	}
	configs := []string{"example"}
	for _, c := range f.Configs {
		configs = append(configs, c.Name)
	}
	fmt.Printf("%s\n\n%-28s %s\n", n.Name, "", strings.Join(configs, "  "))
	row := func(name string) error {
		var cells []string
		for _, c := range configs {
			_, v, err := f.Ref(name, c)
			if err != nil {
				return err
			}
			cells = append(cells, shapes.Group(v))
		}
		fmt.Printf("%-28s %s\n", name, strings.Join(cells, "  "))
		return nil
	}
	for _, b := range f.Blocks {
		for _, part := range []string{"params", "flops_forward"} {
			if err := row(b.ID + "." + part); err != nil {
				return err
			}
		}
	}
	for _, m := range f.Model {
		if err := row(m.Name); err != nil {
			return err
		}
	}
	return nil
}
