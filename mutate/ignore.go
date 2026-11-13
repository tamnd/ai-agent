// SPDX-License-Identifier: Apache-2.0

package mutate

import (
	"fmt"
	"strings"

	"github.com/tamnd/ai-agent/node"
)

// IgnoreFile is the name of the list of equivalent mutants in impl/<lang>/ (spec 09.6).
const IgnoreFile = "mutants.ignore"

// MaxIgnored is the largest part of the mutants of an implementation that mutants.ignore can list (rule S08).
const MaxIgnored = 0.05

// Ignore is one entry of mutants.ignore: the id of an equivalent mutant and the reason why it cannot change the output.
type Ignore struct {
	ID     string
	Reason string
	Line   int
}

// ParseIgnore reads mutants.ignore. Each line is the id of a mutant, as the mutation run writes it, then " # " and the reason. A line that starts with # is a comment.
func ParseIgnore(text string) ([]Ignore, error) {
	var out []Ignore
	for i, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		id, reason, ok := strings.Cut(line, " # ")
		id, reason = strings.TrimSpace(id), strings.TrimSpace(reason)
		if !ok || reason == "" {
			return nil, fmt.Errorf("line %d: an entry is the id of the mutant, then # and the reason why it cannot change the output", i+1)
		}
		out = append(out, Ignore{ID: id, Reason: reason, Line: i + 1})
	}
	return out, nil
}

// Set is the sample of mutants of one implementation, with the entries of its mutants.ignore.
type Set struct {
	Mutants []Mutant
	Ignore  map[string]Ignore
	// Stale are the entries of mutants.ignore that name no mutant of the sample.
	Stale []Ignore
}

// Of makes the sample of mutants of each source file of the implementation (spec 09.6), and reads its mutants.ignore. max is the largest number of mutants of one file; 0 gives all.
func Of(im node.Impl, max int) (Set, error) {
	s := Set{Ignore: map[string]Ignore{}}
	for _, f := range im.Files {
		if f.Test || !Source(im.Lang, f.Name) {
			continue
		}
		ms, err := Mutants(im.Lang, f.Name, f.Content)
		if err != nil {
			return s, err
		}
		s.Mutants = append(s.Mutants, Sample(ms, max)...)
	}
	ids := map[string]bool{}
	for _, m := range s.Mutants {
		ids[m.ID()] = true
	}
	for _, f := range im.Files {
		if f.Name != IgnoreFile {
			continue
		}
		list, err := ParseIgnore(f.Content)
		if err != nil {
			return s, fmt.Errorf("%s: %w", f.Path, err)
		}
		for _, e := range list {
			s.Ignore[e.ID] = e
			if !ids[e.ID] {
				s.Stale = append(s.Stale, e)
			}
		}
	}
	return s, nil
}

// Ignored counts the mutants of the sample that mutants.ignore lists.
func (s Set) Ignored() int {
	k := 0
	for _, m := range s.Mutants {
		if _, ok := s.Ignore[m.ID()]; ok {
			k++
		}
	}
	return k
}
