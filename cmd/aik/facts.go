// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/tamnd/ai-agent/computed"
	"github.com/tamnd/ai-agent/facts"
)

// computedFile is what aik facts compute writes: the results, and the definitions that gave none.
type computedFile struct {
	Facts  []*computed.Result `json:"facts"`
	Errors map[string]string  `json:"errors,omitempty"`
}

// cmdFacts computes the facts of topics/computed.yaml (04.9, rule D08): the first, the largest or the most of a
// population of cards and nodes, with the population and the date.
func cmdFacts(args []string) error {
	if len(args) == 0 || args[0] != "compute" {
		return usageError{"use aik facts compute"}
	}
	f := newFlags("facts compute", "[flags]")
	root := f.String("root", ".", "the root of the content repository")
	out := f.String("out", "build/facts/computed.json", "the file of the results")
	asJSON := f.Bool("json", false, "print the results as JSON")
	if err := parse(f, args[1:]); err != nil {
		return err
	}
	if f.NArg() > 0 {
		return usageError{"aik facts compute takes no arguments"}
	}
	c, err := load(*root)
	if err != nil {
		return err
	}
	for _, e := range c.Errors {
		if strings.HasPrefix(e.Error(), computed.File) {
			return e
		}
	}
	res := computedFile{Facts: []*computed.Result{}}
	for _, d := range c.ComputedDefs {
		if r := c.Computed[d.ID]; r != nil {
			res.Facts = append(res.Facts, r)
		}
	}
	if len(c.ComputedErrs) > 0 {
		res.Errors = map[string]string{}
		for id, err := range c.ComputedErrs {
			res.Errors[id] = err.Error()
		}
	}
	b, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return err
	}
	p := under(*root, *out)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(p, append(b, '\n'), 0o644); err != nil {
		return err
	}
	if *asJSON {
		if err := printJSON(res); err != nil {
			return err
		}
	} else {
		for _, r := range res.Facts {
			fmt.Printf("%s: %s (%s; %d of the population give the measure, %d do not)\n", r.ID, r.Name, facts.Format(r.Value, r.Unit), r.Count, r.Missing)
			fmt.Printf("  %s\n", r.Sentence)
		}
		ids := make([]string, 0, len(res.Errors))
		for id := range res.Errors {
			ids = append(ids, id)
		}
		slices.Sort(ids)
		for _, id := range ids {
			fmt.Printf("%s: no result: %s\n", id, res.Errors[id])
		}
		fmt.Printf("wrote %d computed facts to %s\n", len(res.Facts), p)
	}
	if len(res.Errors) > 0 {
		return errFindings
	}
	return nil
}
