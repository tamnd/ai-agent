// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/tamnd/ai-agent/lang"
	"github.com/tamnd/ai-agent/node"
	"github.com/tamnd/ai-agent/shapes"
)

// cmdFlopcount counts the multiply-adds of the NumPy forward pass of each architecture with an instrumented array type
// and checks the counts against the FLOP formula of shapes.yaml (spec 08.6, check 4).
func cmdFlopcount(args []string) error {
	f := newFlags("flopcount", "[flags] [slug ...]")
	root := f.String("root", ".", "the root of the content repository")
	total := f.String("total", "flops_forward", "the model formula of shapes.yaml that the count must equal")
	asJSON := f.Bool("json", false, "print the counts as JSON")
	unsafe := f.Bool("unsafe-local", false, "run the code on this machine, not in the sandbox; CI sets CI=true instead")
	sandbox := f.String("sandbox", "auto", "the container engine of the sandbox: docker, podman, or none; auto uses docker when the image manifest is there (08.7)")
	images := f.String("images", "manifests/images.yaml", "the manifest of the runner images, pinned by digest")
	keep := f.Bool("keep", false, "keep the temporary directories")
	if err := parse(f, args); err != nil {
		return err
	}
	c, err := load(*root)
	if err != nil {
		return err
	}
	var nodes []*node.Node
	if f.NArg() == 0 {
		for _, n := range c.Nodes {
			if n.Shapes != nil && n.Lang(lang.FlopLang) != nil {
				nodes = append(nodes, n)
			}
		}
	} else if nodes, err = selectNodes(c, f.Args()); err != nil {
		return err
	}
	if len(nodes) == 0 {
		fmt.Fprintf(os.Stderr, "no architecture has %s and a NumPy reference\n", shapes.FileName)
		return nil
	}
	r, err := codeRunner(c.Root, *sandbox, *images, *unsafe, *keep)
	if err != nil {
		return err
	}
	if err := r.Available(lang.Recipes[lang.FlopLang]); err != nil {
		var skip lang.Skip
		if errors.As(err, &skip) {
			return fmt.Errorf("the count needs %s: %w", skip.Tool, err)
		}
		return err
	}
	var reps []lang.FlopReport
	bad := 0
	for _, n := range nodes {
		rep := r.FlopCount(n, *total)
		if !rep.OK() {
			bad++
		}
		reps = append(reps, rep)
	}
	if *asJSON {
		if err := printJSON(reps); err != nil {
			return err
		}
	} else {
		for _, rep := range reps {
			printFlops(rep)
		}
		fmt.Printf("%d %s, %d with a count that is not the formula\n", len(reps), plural(len(reps), "architecture", "architectures"), bad)
	}
	if bad > 0 {
		return errFindings
	}
	return nil
}

func printFlops(rep lang.FlopReport) {
	if rep.Error != "" {
		fmt.Printf("%s: %s\n", rep.Slug, rep.Error)
		return
	}
	for _, c := range rep.Configs {
		switch {
		case c.Error != "":
			fmt.Printf("%s %s: forward fails: %s\n", rep.Slug, c.Config, lastLine(c.Error))
		case c.OK():
			fmt.Printf("%s %s: %s FLOPs, as %s gives\n", rep.Slug, c.Config, shapes.Group(c.Counted), rep.Total)
		default:
			why := ""
			if c.Counted.Sign() == 0 {
				why = "; nothing was counted, so the forward may turn its inputs into plain arrays with np.asarray"
			}
			fmt.Printf("%s %s: counted %s FLOPs, and %s is %s%s\n", rep.Slug, c.Config, shapes.Group(c.Counted), rep.Total, shapes.Group(c.Formula), why)
		}
		if len(c.Uncounted) > 0 {
			var names []string
			for k, v := range c.Uncounted {
				names = append(names, fmt.Sprintf("%s (%d)", k, v))
			}
			sort.Strings(names)
			fmt.Printf("  not counted: %s\n", strings.Join(names, ", "))
		}
	}
}

// cmdMutate is the mutation test of 08.8, the same as aik test -kind mutate.
func cmdMutate(args []string) error {
	for _, a := range args {
		if a == "-kind" || strings.HasPrefix(a, "-kind=") || a == "--kind" || strings.HasPrefix(a, "--kind=") {
			return usageError{"aik mutate is aik test -kind mutate; it takes the other flags of aik test"}
		}
	}
	return cmdTest(append([]string{"-kind", "mutate"}, args...))
}
