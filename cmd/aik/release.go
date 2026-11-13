// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"time"

	"github.com/tamnd/ai-agent/release"
)

// cmdRelease cuts the monthly release of spec 20.6.
func cmdRelease(args []string) error {
	if len(args) == 0 || args[0] != "cut" {
		return usageError{"use aik release cut <YYYY.MM>, or <YYYY.MM.N> for a patch release"}
	}
	f := newFlags("release cut", "[flags] <YYYY.MM>")
	lf := addLadderFlags(f)
	noTag := f.Bool("no-tag", false, "write the manifest and the notes, and do not tag the commit")
	dirty := f.Bool("allow-dirty", false, "cut from a work tree with changes that are not committed; the manifest then names a commit that is not what was built")
	out := f.String("out", release.Dir, "the directory of the manifest and the notes")
	if err := parse(f, args[1:]); err != nil {
		return err
	}
	if f.NArg() != 1 {
		return usageError{"give one release version, for example 2027.03"}
	}
	v := f.Arg(0)
	if !release.Valid(v) {
		return usageError{fmt.Sprintf("%q is not a release version: give YYYY.MM, or YYYY.MM.N for a patch release", v)}
	}
	c, st, err := lf.ladders()
	if err != nil {
		return err
	}
	now := time.Now()
	cov, err := coverageReport(*lf.root, c, st, now)
	if err != nil {
		return err
	}
	r, err := release.Cut(release.Options{Root: *lf.root, Version: v, Corpus: c, Coverage: cov, Aik: ver(), Now: now,
		Dirty: *dirty, Tag: !*noTag, Out: under(*lf.root, *out)})
	if err != nil {
		return err
	}
	m := r.Manifest
	fmt.Printf("release %s of commit %s, aik %s\n", m.Version, m.Commit, m.Aik)
	lean := "none"
	if m.Lean != nil {
		lean = m.Lean.Toolchain
		if m.Lean.Mathlib != "" {
			lean += ", Mathlib " + m.Lean.Mathlib
		}
	}
	fmt.Printf("  %d images, Lean %s, %d data pins, models %s\n", len(m.Images), lean, len(m.Data), orNone(m.Models))
	if r.Previous != nil {
		fmt.Printf("  the notes compare with %s\n", r.Previous.Version)
	}
	for _, p := range r.Files {
		fmt.Printf("  wrote %s\n", rel(*lf.root, p))
	}
	for _, s := range r.Missing {
		fmt.Printf("  not pinned: %s\n", s)
	}
	if r.Tagged {
		fmt.Printf("  tagged %s; push it with git push origin %s\n", m.Version, m.Version)
	}
	return nil
}

func orNone(s string) string {
	if s == "" {
		return "none"
	}
	return s
}
