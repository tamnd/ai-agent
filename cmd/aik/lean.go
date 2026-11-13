// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/tamnd/ai-agent/formal"
	"github.com/tamnd/ai-agent/lang"
	"github.com/tamnd/ai-agent/status"
)

// cmdLean runs the subcommands of aik lean.
func cmdLean(args []string) error {
	if len(args) == 0 || args[0] != "build" {
		return usageError{"use aik lean build"}
	}
	return cmdLeanBuild(args[1:])
}

// cmdLeanBuild builds the Lean package, extracts its declarations, and checks the axiom policy and the lean fields (spec 07.5, 07.12).
func cmdLeanBuild(args []string) error {
	f := newFlags("lean build", "[flags]")
	root := f.String("root", ".", "the root of the content repository")
	out := f.String("out", "build/lean", "the directory for decls.jsonl")
	noBuild := f.Bool("no-build", false, "skip lake build, and only run AlgoExtract and the checks")
	sandbox := f.String("sandbox", "none", "run lake in a container of the Lean image: docker, podman or none")
	images := f.String("images", "manifests/images.yaml", "the image manifest, for -sandbox")
	verbose := f.Bool("v", false, "show the output of lake")
	timed := f.Bool("time", false, "after the build, elaborate each module again and report a slow module (L10)")
	if err := parse(f, args); err != nil {
		return err
	}
	c, err := load(*root)
	if err != nil {
		return err
	}
	o := formal.Options{Dir: filepath.Join(*root, "lean"), Out: under(*root, *out), NoBuild: *noBuild}
	var log strings.Builder
	o.Log = &log
	if *verbose {
		o.Log = os.Stderr
	}
	if *sandbox != "none" {
		wrap, err := leanSandbox(*sandbox, under(*root, *images), o)
		if err != nil {
			return err
		}
		o.Wrap = wrap
	}
	path, err := formal.Build(context.Background(), o)
	if err != nil {
		if !*verbose {
			fmt.Fprint(os.Stderr, log.String())
		}
		return err
	}
	ds, err := formal.Load(path)
	if err != nil {
		return err
	}
	thm := 0
	for _, d := range ds {
		if d.Kind == "theorem" {
			thm++
		}
	}
	fmt.Printf("%d declarations, %d theorems, in %s\n", len(ds), thm, path)
	for _, n := range c.Nodes {
		if n.Lean["module"] == "" {
			continue
		}
		l := status.Of(n, status.Env{Corpus: c, Decls: ds}).L
		fmt.Printf("  %-24s %s  %s\n", n.Slug, l.Label, l.Name)
	}
	fs := formal.Check(c, ds)
	if *timed {
		ts, err := formal.Time(context.Background(), o)
		if err != nil {
			if !*verbose {
				fmt.Fprint(os.Stderr, log.String())
			}
			return err
		}
		for _, t := range ts {
			fmt.Printf("  %-40s %6.1f s (%.1f s with the imports)\n", t.Module, t.Took.Seconds(), t.Wall.Seconds())
		}
		fs = append(fs, formal.CheckTimes(ts)...)
	}
	blocks := 0
	for _, x := range fs {
		fmt.Println(x)
		if x.Sev() != "warn" {
			blocks++
		}
	}
	if blocks > 0 {
		return errFindings
	}
	fmt.Printf("Each declaration uses only the axioms %s.\n", strings.Join(formal.Allowed, ", "))
	return nil
}

// leanSandbox returns a Wrap that runs lake in the Lean image. The package and the output directory are the writable mounts. A package of .lake/packages that is a link to another directory, as in a local clone that shares Mathlib, is mounted read-only at its own path.
func leanSandbox(engine, manifest string, o formal.Options) (func([]string) []string, error) {
	imgs, err := lang.LoadImages(manifest)
	if err != nil {
		return nil, err
	}
	box := lang.NewSandbox(engine, imgs)
	if err := box.Ready("lean"); err != nil {
		return nil, err
	}
	dir, err := filepath.Abs(o.Dir)
	if err != nil {
		return nil, err
	}
	outDir, err := filepath.Abs(o.Out)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, err
	}
	mounts := []lang.Mount{{Dir: dir}, {Dir: outDir}}
	pkgs, _ := os.ReadDir(filepath.Join(dir, ".lake", "packages"))
	for _, p := range pkgs {
		if p.Type()&os.ModeSymlink == 0 {
			continue
		}
		if t, err := filepath.EvalSymlinks(filepath.Join(dir, ".lake", "packages", p.Name())); err == nil && !slices.ContainsFunc(mounts, func(m lang.Mount) bool { return m.Dir == t }) {
			mounts = append(mounts, lang.Mount{Dir: t, ReadOnly: true})
		}
	}
	return func(argv []string) []string { return box.Argv("lean", false, mounts, argv) }, nil
}
