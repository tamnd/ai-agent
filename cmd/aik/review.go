// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/tamnd/ai-agent/review"
)

// cmdReview writes the template of a review record (spec 15.5), or with -check tells whether the record counts.
func cmdReview(args []string) error {
	f := newFlags("review", "[flags] <slug> <part>")
	root := f.String("root", ".", "the root of the content repository")
	reviewer := f.String("reviewer", "", "the reviewer, github:<login>, as in "+review.Register)
	checklist := f.String("checklist", "", "the checklist version, as text/method/v1; the default is the newest of the part and the kind")
	write := f.Bool("w", false, "write the template to reviews/<tag>/<part>.yaml rather than print it")
	check := f.Bool("check", false, "tell whether the record of the part counts, and why not")
	if err := parse(f, args); err != nil {
		return err
	}
	if f.NArg() != 2 {
		return usageError{"give a slug and a part: text, positions, facts, contract, impl-<framework>, exp/<slug>, scene, lean-spec, lean-statement or proof-<id>"}
	}
	c, err := load(*root)
	if err != nil {
		return err
	}
	n := c.BySlug[f.Arg(0)]
	if n == nil {
		return usageError{fmt.Sprintf("no node has the slug %q", f.Arg(0))}
	}
	if n.Tag == "" {
		return fmt.Errorf("%s has no tag yet: run aik mint", n.Slug)
	}
	part := f.Arg(1)
	files, err := review.Files(*root, n, part)
	if err != nil {
		return err
	}
	if *check {
		// Every file of the part must be hashed and current; the ladders check the main file.
		for _, file := range files {
			if ok, why := review.Check(*root, n, part, file); !ok {
				fmt.Printf("FAIL %s %s: %s\n", n.Slug, part, why)
				return errFindings
			}
		}
		fmt.Printf("pass %s %s: the review counts\n", n.Slug, part)
		return nil
	}
	if *reviewer == "" {
		return usageError{"give the reviewer with -reviewer github:<login>"}
	}
	cl := *checklist
	if cl == "" {
		if cl = review.Latest(*root, n, part); cl == "" {
			return fmt.Errorf("no checklist for %s under checklists/%s: write one (15.7), or name it with -checklist", part, review.ChecklistDir(part))
		}
	}
	t, err := review.Template(*root, n, part, *reviewer, cl, time.Now().Format(time.DateOnly))
	if err != nil {
		return err
	}
	if !*write {
		fmt.Print(t)
		return nil
	}
	path := review.File(*root, n.Tag, part)
	if _, err := os.Stat(path); err == nil {
		fmt.Fprintf(os.Stderr, "note: %s replaces the record there; the old one stays in the history\n", path)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(t), 0o644); err != nil {
		return err
	}
	fmt.Println(path)
	return nil
}
