// SPDX-License-Identifier: Apache-2.0

// Package course reads the curated courses of topics/courses (spec 05.7): an ordered list of items with section
// headings, which the site shows as a reading path (16.12).
package course

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Dir is the directory of the courses, under the root of the content repository.
const Dir = "topics/courses"

// Course is one file of topics/courses.
type Course struct {
	ID       string    `yaml:"id" json:"id"`
	Title    string    `yaml:"title" json:"title"`
	Basis    string    `yaml:"basis" json:"basis,omitempty"`
	Sections []Section `yaml:"sections" json:"sections"`
}

// Section is a heading of a course and its items, in order.
type Section struct {
	Heading string `yaml:"heading" json:"heading"`
	Items   []Item `yaml:"items" json:"items"`
}

// Item is one step of a course. It names its node by slug when the node exists, and by its name and kind before.
type Item struct {
	Name string `yaml:"name" json:"name"`
	Kind string `yaml:"kind,omitempty" json:"kind,omitempty"`
	Node string `yaml:"node,omitempty" json:"node,omitempty"`
}

// Len is the number of items of the course.
func (c Course) Len() int {
	n := 0
	for _, s := range c.Sections {
		n += len(s.Items)
	}
	return n
}

// Load reads the courses of root, sorted by id. It gives nil and no error when there is none. A course with no id
// takes the name of its file, and an item with no kind is a concept.
func Load(root string) ([]Course, error) {
	files, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(Dir), "*.yaml"))
	if err != nil {
		return nil, err
	}
	var out []Course
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		var c Course
		if err := yaml.Unmarshal(b, &c); err != nil {
			return nil, fmt.Errorf("%s/%s: %w", Dir, filepath.Base(f), err)
		}
		if c.ID == "" {
			c.ID = strings.TrimSuffix(filepath.Base(f), ".yaml")
		}
		if c.Title == "" {
			c.Title = c.ID
		}
		for i := range c.Sections {
			for j := range c.Sections[i].Items {
				if c.Sections[i].Items[j].Kind == "" {
					c.Sections[i].Items[j].Kind = "concept"
				}
			}
		}
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}
