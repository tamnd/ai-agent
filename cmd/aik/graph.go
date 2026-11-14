// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"strings"

	"github.com/tamnd/ai-agent/graph"
)

// cmdGraph writes the graph of 03.4 and reads it: the edges of an item both ways, and the reading path to a node.
func cmdGraph(args []string) error {
	if len(args) == 0 {
		return usageError{"use aik graph build | path <slug> | show <slug>"}
	}
	switch args[0] {
	case "build":
		return graphBuild(args[1:])
	case "path":
		return graphPath(args[1:])
	case "show":
		return graphShow(args[1:])
	}
	return usageError{fmt.Sprintf("aik graph has no subcommand %q; use build, path or show", args[0])}
}

func graphBuild(args []string) error {
	f := newFlags("graph build", "[flags]")
	root := f.String("root", ".", "the root of the content repository")
	out := f.String("o", "build/graph", "the directory of edges.jsonl, reverse.jsonl and summary.json")
	asJSON := f.Bool("json", false, "print the summary as JSON")
	if err := parse(f, args); err != nil {
		return err
	}
	if f.NArg() > 0 {
		return usageError{"aik graph build takes no arguments"}
	}
	c, err := load(*root)
	if err != nil {
		return err
	}
	s, err := graph.Build(c).Write(under(*root, *out))
	if err != nil {
		return err
	}
	if *asJSON {
		return printJSON(s)
	}
	fmt.Printf("%d edges, %d reverse edges, %d to neighbours, %d to unknown items, %d cycles\n", s.Edges, s.Reverse, s.Neighbor, s.Unknown, len(s.Cycles))
	for _, cy := range s.Cycles {
		fmt.Printf("  cycle: %s\n", strings.Join(cy, " -> "))
	}
	return nil
}

func graphPath(args []string) error {
	f := newFlags("graph path", "[flags] <slug>")
	root := f.String("root", ".", "the root of the content repository")
	asJSON := f.Bool("json", false, "print the path as JSON")
	if err := parse(f, args); err != nil {
		return err
	}
	if f.NArg() != 1 {
		return usageError{"name one node: aik graph path <slug>"}
	}
	c, err := load(*root)
	if err != nil {
		return err
	}
	slug := f.Arg(0)
	if c.BySlug[slug] == nil {
		return usageError{fmt.Sprintf("no node has the slug %q", slug)}
	}
	p := graph.Build(c).Path(slug)
	if *asJSON {
		return printJSON(p)
	}
	for i, k := range p {
		name := k
		if n := c.BySlug[k]; n != nil {
			name = n.Name + " (" + k + ")"
		} else if b := c.Bridges.Find(k); b != nil {
			name = b.Name + " (" + k + ")"
		}
		fmt.Printf("%3d  %s\n", i+1, name)
	}
	return nil
}

func graphShow(args []string) error {
	f := newFlags("graph show", "[flags] <slug>")
	root := f.String("root", ".", "the root of the content repository")
	asJSON := f.Bool("json", false, "print the edges as JSON")
	if err := parse(f, args); err != nil {
		return err
	}
	if f.NArg() != 1 {
		return usageError{"name one item: aik graph show <slug>"}
	}
	c, err := load(*root)
	if err != nil {
		return err
	}
	g := graph.Build(c)
	key := f.Arg(0)
	if _, ok := g.Ref(key); !ok {
		return usageError{fmt.Sprintf("no node or card has the key %q", key)}
	}
	out, in := g.Out(key), g.In(key)
	if *asJSON {
		return printJSON(map[string][]graph.Edge{"out": out, "in": in})
	}
	for _, e := range out {
		mark := ""
		if !e.Known {
			mark = " (unknown)"
		}
		fmt.Printf("%-18s -> %s%s\n", e.Type, e.To, mark)
	}
	for _, e := range in {
		typ := e.Type
		if r, ok := graph.Reverse[e.Type]; ok {
			typ = r
		}
		fmt.Printf("%-18s <- %s\n", typ, e.From)
	}
	return nil
}
