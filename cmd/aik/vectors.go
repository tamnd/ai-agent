// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tamnd/ai-agent/lang"
	"github.com/tamnd/ai-agent/metric"
	"github.com/tamnd/ai-agent/node"
)

// cmdVectors runs vectors/make.py of the metric nodes, which computes the test vectors with the official
// implementation in the aik-vectors image (spec 08.7.5): make writes the files whose rows changed, with the change of
// each row; make -check only reports them, and that make.py is deterministic and its rows fit the contract.
func cmdVectors(args []string) error {
	if len(args) == 0 || args[0] != "make" {
		return usageError{"use aik vectors make [-check] [slug...]"}
	}
	f := newFlags("vectors make", "[flags] [slug ...]")
	root := f.String("root", ".", "the root of the content repository")
	check := f.Bool("check", false, "write nothing; exit 1 when make.py writes other vectors than the committed ones")
	asJSON := f.Bool("json", false, "print the runs as JSON")
	unsafe := f.Bool("unsafe-local", false, "run make.py on this machine, with its Python and packages, not in the sandbox; CI sets CI=true instead")
	sandbox := f.String("sandbox", "auto", "the container engine of the sandbox: docker, podman, or none; auto uses docker when the image manifest is there (08.7)")
	images := f.String("images", "manifests/images.yaml", "the manifest of the images; make.py runs in the image of "+lang.VectorsImage)
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
			if _, err := os.Stat(filepath.Join(n.Dir, metric.VectorsDir, lang.VectorsMake)); err == nil {
				nodes = append(nodes, n)
			}
		}
	} else if nodes, err = selectNodes(c, f.Args()); err != nil {
		return err
	}
	if len(nodes) == 0 {
		fmt.Fprintln(os.Stderr, "no metric node has vectors/make.py")
		return nil
	}
	r, err := codeRunner(c.Root, *sandbox, *images, *unsafe, *keep)
	if err != nil {
		return err
	}
	if err := r.VectorsReady(); err != nil {
		var skip lang.Skip
		if errors.As(err, &skip) && skip.Tool == "image" {
			return fmt.Errorf("%s has no image %s, the image of the official metric implementations (08.9.1); give -unsafe-local to run make.py with the packages of this machine", *images, lang.VectorsImage)
		}
		return err
	}
	var runs []*lang.VectorRun
	bad := 0
	for _, n := range nodes {
		vr := r.MakeVectors(n)
		runs = append(runs, vr)
		if !*asJSON {
			printVectorRun(vr, *check)
		}
		switch {
		case vr.Error != "" || vr.Invalid != "" || len(vr.Unstable) > 0:
			bad++
		case len(vr.Changed) > 0 && *check:
			bad++
		case len(vr.Changed) > 0:
			wrote, err := vr.Write(n)
			if err != nil {
				vr.Close(*keep)
				return err
			}
			if !*asJSON {
				fmt.Printf("%s: wrote %s\n", n.Slug, strings.Join(wrote, ", "))
			}
		}
		vr.Close(*keep)
	}
	if *asJSON {
		if err := printJSON(runs); err != nil {
			return err
		}
	} else {
		fmt.Printf("%d %s, %d with a problem\n", len(runs), plural(len(runs), "metric", "metrics"), bad)
	}
	if bad > 0 {
		return errFindings
	}
	return nil
}

func printVectorRun(vr *lang.VectorRun, check bool) {
	if vr.Error != "" {
		fmt.Printf("%s: %s\n", vr.Slug, vr.Error)
		return
	}
	if vr.OK() {
		kept := ""
		if len(vr.Kept) > 0 {
			kept = "; it does not write " + strings.Join(vr.Kept, ", ")
		}
		fmt.Printf("%s: make.py writes the committed vectors%s\n", vr.Slug, kept)
		return
	}
	if vr.Invalid != "" {
		fmt.Printf("%s: the vectors do not fit %s: %s\n", vr.Slug, metric.ContractFile, vr.Invalid)
	}
	if len(vr.Unstable) > 0 {
		fmt.Printf("%s: make.py is not deterministic: a second run writes other bytes in %s\n", vr.Slug, strings.Join(vr.Unstable, ", "))
	}
	if len(vr.Changed) == 0 {
		return
	}
	head := "make.py writes other vectors than the committed ones"
	if check {
		head += "; run aik vectors make " + vr.Slug
	}
	fmt.Printf("%s: %s\n", vr.Slug, head)
	for _, fc := range vr.Changed {
		switch {
		case fc.New:
			fmt.Printf("  %s: a new file of %d %s\n", fc.File, len(fc.Rows), plural(len(fc.Rows), "row", "rows"))
			continue
		case len(fc.Rows) == 0:
			fmt.Printf("  %s: the same rows, with other bytes\n", fc.File)
			continue
		}
		for _, row := range fc.Rows {
			fmt.Printf("  %s %s: %s\n", fc.File, row.ID, row.Change)
			for _, fd := range row.Fields {
				within := ""
				if fd.Within != nil {
					within = " (within the tolerance)"
					if !*fd.Within {
						within = " (outside the tolerance)"
					}
				}
				fmt.Printf("    %s: %v -> %v%s\n", fd.Path, brief(fd.Old), brief(fd.New), within)
			}
			if row.More > 0 {
				fmt.Printf("    and %d more\n", row.More)
			}
		}
	}
}

func brief(v any) string {
	if v == nil {
		return "none"
	}
	s := fmt.Sprint(v)
	if len(s) > 60 {
		s = s[:57] + "..."
	}
	return s
}
