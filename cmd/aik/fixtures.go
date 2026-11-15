// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tamnd/ai-agent/lang"
	"github.com/tamnd/ai-agent/node"
)

// cmdFixtures runs the make.py of the shared fixtures of the nodes (spec 08.7.2, rule X15): make writes the files
// that changed, with the diff of each array; make -check only reports them, and that make.py is deterministic.
func cmdFixtures(args []string) error {
	if len(args) > 0 && args[0] == "sample" {
		fmt.Fprintln(os.Stderr, "aik fixtures sample, the random inputs of the differential tests of 8.7.3, is planned for milestone M2 and is not built yet.")
		return errPlanned
	}
	if len(args) == 0 || args[0] != "make" {
		return usageError{"use aik fixtures make [-check] [slug...]"}
	}
	f := newFlags("fixtures make", "[flags] [slug ...]")
	root := f.String("root", ".", "the root of the content repository")
	check := f.Bool("check", false, "write nothing; exit 1 when make.py writes other arrays than the committed ones, or other ones on a second run")
	asJSON := f.Bool("json", false, "print the runs as JSON")
	unsafe := f.Bool("unsafe-local", false, "run make.py on this machine, not in the sandbox; CI sets CI=true instead")
	sandbox := f.String("sandbox", "auto", "the container engine of the sandbox: docker, podman, or none; auto uses docker when the image manifest is there (08.7)")
	images := f.String("images", "manifests/images.yaml", "the manifest of the runner images, pinned by digest")
	keep := f.Bool("keep", false, "keep the temporary directories")
	if err := parse(f, args[1:]); err != nil {
		return err
	}
	c, err := load(*root)
	if err != nil {
		return err
	}
	var nodes []*node.Node
	if f.NArg() == 0 {
		for _, n := range c.Nodes {
			if _, err := os.Stat(filepath.Join(n.Dir, lang.FixturesDir, lang.FixturesMake)); err == nil {
				nodes = append(nodes, n)
			}
		}
	} else if nodes, err = selectNodes(c, f.Args()); err != nil {
		return err
	}
	if len(nodes) == 0 {
		fmt.Fprintln(os.Stderr, "no node has fixtures/make.py")
		return nil
	}
	r, err := codeRunner(c.Root, *sandbox, *images, *unsafe, *keep)
	if err != nil {
		return err
	}
	if err := r.Available(lang.Recipes[lang.FlopLang]); err != nil {
		return err
	}
	var runs []*lang.FixtureRun
	bad := 0
	for _, n := range nodes {
		fr := r.MakeFixtures(n)
		runs = append(runs, fr)
		if !*asJSON {
			printFixtureRun(fr, *check)
		}
		switch {
		case fr.Error != "" || len(fr.Unstable) > 0:
			bad++
		case len(fr.Changed) > 0 && *check:
			bad++
		case len(fr.Changed) > 0:
			wrote, err := fr.Write(n)
			if err != nil {
				fr.Close(*keep)
				return err
			}
			if !*asJSON {
				fmt.Printf("%s: wrote %s\n", n.Slug, strings.Join(wrote, ", "))
			}
		}
		fr.Close(*keep)
	}
	if *asJSON {
		if err := printJSON(runs); err != nil {
			return err
		}
	} else {
		fmt.Printf("%d %s, %d with a problem\n", len(runs), plural(len(runs), "node", "nodes"), bad)
	}
	if bad > 0 {
		return errFindings
	}
	return nil
}

func printFixtureRun(fr *lang.FixtureRun, check bool) {
	if fr.Error != "" {
		fmt.Printf("%s: %s\n", fr.Slug, fr.Error)
		return
	}
	if fr.OK() {
		fmt.Printf("%s: make.py writes the committed fixtures (NumPy %s)\n", fr.Slug, fr.NumPy)
		return
	}
	show := func(head string, cs []lang.FixtureChange) {
		if len(cs) == 0 {
			return
		}
		fmt.Printf("%s: %s\n", fr.Slug, head)
		for _, c := range cs {
			if len(c.Arrays) == 0 {
				fmt.Printf("  %s\n", c.File)
			}
			for _, a := range c.Arrays {
				line := fmt.Sprintf("  %s %s: %s", c.File, a.Name, a.Change)
				if a.Change == "values" {
					line += fmt.Sprintf(", %d of %d", a.Count, a.Size)
					if a.MaxAbs != nil {
						line += fmt.Sprintf(", at most %.3g", *a.MaxAbs)
					}
				}
				fmt.Println(line)
			}
		}
	}
	head := "make.py writes other files than the committed ones"
	if check {
		head += "; run aik fixtures make " + fr.Slug
	}
	show(head, fr.Changed)
	show("make.py is not deterministic: a second run writes other arrays", fr.Unstable)
}
