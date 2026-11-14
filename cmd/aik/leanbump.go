// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/tamnd/ai-agent/formal"
	"github.com/tamnd/ai-agent/status"
)

// lsRemote lists the tags of a git repository. Tests replace it.
var lsRemote = func(ctx context.Context, url string) (string, error) {
	b, err := exec.CommandContext(ctx, "git", "ls-remote", "--tags", url).Output()
	return string(b), err
}

// bumpNode is the formal level of a node before and after the bump.
type bumpNode struct {
	Slug   string
	Module string
	Before int
	After  int    // -1 when the package does not build, so the level is not known
	Broken bool   // the module of the node does not build
	Why    string // the next step of the level after the bump
}

// cmdLeanBump moves the toolchain and the Mathlib tag of the Lean package to a new release, builds each module, and
// reports the modules that break, the declarations that changed and the nodes whose formal level drops (11.11).
func cmdLeanBump(args []string) error {
	f := newFlags("lean bump", "[flags]")
	root := f.String("root", ".", "the root of the content repository")
	out := f.String("out", "build/lean", "the directory of decls.jsonl and of the report bump.md")
	to := f.String("to", "", "the release to move to, such as v4.36.0; the default is the latest release tag of Mathlib")
	dry := f.Bool("n", false, "print the release that the bump would move to, and change nothing")
	pr := f.Bool("pr", false, "commit the pins on a new branch lean-bump-<tag>, push it and open a pull request with the report")
	sandbox := f.String("sandbox", "none", "run lake in a container of the Lean image: docker, podman or none")
	images := f.String("images", "manifests/images.yaml", "the image manifest, for -sandbox")
	verbose := f.Bool("v", false, "show the output of lake")
	if err := parse(f, args); err != nil {
		return err
	}
	if f.NArg() > 0 {
		return usageError{"aik lean bump takes no arguments"}
	}
	ctx := context.Background()
	dir := filepath.Join(*root, "lean")
	from, err := formal.ReadPins(dir)
	if err != nil {
		return err
	}
	target := *to
	if target == "" {
		tags, err := lsRemote(ctx, formal.MathlibGit)
		if err != nil {
			return fmt.Errorf("git ls-remote %s: %w", formal.MathlibGit, err)
		}
		if target = formal.LatestTag(tags); target == "" {
			return fmt.Errorf("%s has no release tag", formal.MathlibGit)
		}
	}
	if target == from.Mathlib && target == from.Tag() {
		fmt.Printf("the package is at %s already\n", target)
		return nil
	}
	fmt.Printf("bump %s and Mathlib %s to %s\n", from.Toolchain, from.Mathlib, target)
	if *dry {
		return nil
	}
	c, err := load(*root)
	if err != nil {
		return err
	}
	o := formal.Options{Dir: dir, Out: under(*root, *out)}
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
	fail := func(err error) error {
		if !*verbose {
			fmt.Fprint(os.Stderr, log.String())
		}
		return err
	}

	// The levels before the bump need the declarations at the old pins.
	declsPath := filepath.Join(o.Out, "decls.jsonl")
	old, err := status.LoadDecls(declsPath)
	if err != nil {
		return err
	}
	if old == nil {
		fmt.Fprintln(os.Stderr, "building the package at the old pins first")
		p, err := formal.Build(ctx, o)
		if err != nil {
			return fail(err)
		}
		if old, err = formal.Load(p); err != nil {
			return err
		}
	}
	var nodes []bumpNode
	for _, n := range c.Nodes {
		if n.Lean["module"] == "" {
			continue
		}
		nodes = append(nodes, bumpNode{Slug: n.Slug, Module: n.Lean["module"], Before: status.Of(n, status.Env{Corpus: c, Decls: old}).L.Level, After: -1})
	}

	if err := formal.WritePins(dir, target); err != nil {
		return err
	}
	if err := formal.Update(ctx, o); err != nil {
		return fail(err)
	}
	mods, err := formal.Modules(dir)
	if err != nil {
		return err
	}
	fails, err := formal.BuildModules(ctx, o, mods)
	if err != nil {
		return fail(err)
	}
	var changes []formal.DeclChange
	if len(fails) == 0 {
		nb := o
		nb.NoBuild = true
		p, err := formal.Build(ctx, nb)
		if err != nil {
			return fail(err)
		}
		cur, err := formal.Load(p)
		if err != nil {
			return err
		}
		changes = formal.CompareDecls(old, cur)
		for i := range nodes {
			l := status.Of(c.BySlug[nodes[i].Slug], status.Env{Corpus: c, Decls: cur}).L
			nodes[i].After = l.Level
			if len(l.Next) > 0 {
				nodes[i].Why = l.Next[0]
			}
		}
	}
	broken := map[string]bool{}
	for _, x := range fails {
		broken[x.Module] = true
	}
	for i := range nodes {
		nodes[i].Broken = broken[nodes[i].Module]
	}
	to2 := formal.Pins{Toolchain: "leanprover/lean4:" + target, Mathlib: target}
	report, drops := bumpReport(from, to2, len(mods), fails, changes, nodes)
	if err := os.MkdirAll(o.Out, 0o755); err != nil {
		return err
	}
	rp := filepath.Join(o.Out, "bump.md")
	if err := os.WriteFile(rp, []byte(report), 0o644); err != nil {
		return err
	}
	fmt.Print(report)
	if *pr {
		if err := bumpPR(*root, target, rp); err != nil {
			return err
		}
	}
	if drops > 0 || len(fails) > 0 {
		return errFindings
	}
	return nil
}

// bumpReport writes the report of a bump in Markdown, and counts the nodes at F2 or above that it breaks.
func bumpReport(from, to formal.Pins, mods int, fails []formal.Failure, changes []formal.DeclChange, nodes []bumpNode) (string, int) {
	var b strings.Builder
	fmt.Fprintf(&b, "# Lean bump to %s\n\n", to.Mathlib)
	fmt.Fprintf(&b, "The toolchain moves from `%s` to `%s`, and Mathlib from `%s` to `%s` (spec 11.11). The package has %d modules.\n\n", from.Toolchain, to.Toolchain, from.Mathlib, to.Mathlib, mods)
	bySlugs := func(mod string) string {
		var s []string
		for _, n := range nodes {
			if n.Module == mod {
				s = append(s, n.Slug)
			}
		}
		if len(s) == 0 {
			return ""
		}
		return " (" + strings.Join(s, ", ") + ")"
	}
	if len(fails) > 0 {
		fmt.Fprintf(&b, "## Modules that do not build (%d)\n\n", len(fails))
		for _, x := range fails {
			fmt.Fprintf(&b, "- `%s`%s\n", x.Module, bySlugs(x.Module))
			for _, e := range x.Errors {
				fmt.Fprintf(&b, "  - `%s`\n", strings.ReplaceAll(e, "`", "'"))
			}
		}
		b.WriteString("\nThe declarations were not extracted, because the library does not build; the levels after the bump are not known.\n\n")
	}
	if len(changes) > 0 {
		fmt.Fprintf(&b, "## Declarations (%d)\n\n| Declaration | Module | Change |\n|---|---|---|\n", len(changes))
		what := map[string]string{"lost": "lost", "sorry": "now depends on sorry", "axioms": "now uses an axiom outside the policy", "type": "type changed: review the fidelity (11.7)"}
		for _, d := range changes {
			fmt.Fprintf(&b, "| `%s` | `%s` | %s |\n", d.Decl, d.Module, what[d.What])
		}
		b.WriteString("\n")
	}
	drops := 0
	var rows []string
	for _, n := range nodes {
		after := fmt.Sprintf("F%d", n.After)
		switch {
		case n.Broken:
			after = "does not build"
		case n.After < 0:
			after = "not known"
		}
		dropped := n.Before >= 2 && (n.Broken || (n.After >= 0 && n.After < 2))
		if dropped {
			drops++
		}
		if n.After == n.Before && !n.Broken {
			continue
		}
		why := n.Why
		if dropped {
			why = "the proof breaks: fix it, or the node drops to F1 until it is fixed"
		}
		rows = append(rows, fmt.Sprintf("| %s | F%d | %s | %s |", n.Slug, n.Before, after, why))
	}
	if len(rows) > 0 {
		fmt.Fprintf(&b, "## Nodes whose level changes (%d)\n\n| Node | Before | After | Next |\n|---|---|---|---|\n%s\n\n", len(rows), strings.Join(rows, "\n"))
	}
	switch {
	case drops > 0:
		what := fmt.Sprintf("%d nodes at F2 or above break", drops)
		if drops == 1 {
			what = "1 node at F2 or above breaks"
		}
		fmt.Fprintf(&b, "**%s.** The bump merges only when every F2 theorem builds (11.11).\n", what)
	case len(fails) > 0:
		b.WriteString("**The library does not build.** The bump merges only when every F2 theorem builds (11.11).\n")
	default:
		b.WriteString("Every module builds and every F2 theorem still builds, so the bump can merge.\n")
	}
	return b.String(), drops
}

// bumpPR commits the pins on a new branch, pushes it and opens a pull request whose body is the report.
func bumpPR(root, tag, report string) error {
	branch := "lean-bump-" + tag
	steps := [][]string{
		{"git", "switch", "-c", branch},
		{"git", "add", "lean/lean-toolchain", "lean/lakefile.toml", "lean/lake-manifest.json"},
		{"git", "commit", "-m", "lean: bump the toolchain and Mathlib to " + tag + " (11.11)"},
		{"git", "push", "-u", "origin", branch},
		{"gh", "pr", "create", "--title", "lean: bump the toolchain and Mathlib to " + tag, "--body-file", report},
	}
	for _, s := range steps {
		cmd := exec.Command(s[0], s[1:]...)
		cmd.Dir = root
		cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%s: %w", strings.Join(s[:min(3, len(s))], " "), err)
		}
	}
	return nil
}
