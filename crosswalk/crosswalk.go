// SPDX-License-Identifier: Apache-2.0

// Package crosswalk reads the crosswalks of 5.8: the registers crosswalk/<name>.yaml that map the ids of an outside
// classification (ACM CCS 2012, the CS2023 knowledge units, the arXiv categories) to our items. A crosswalk of this
// kind has a system and rows; the other files of crosswalk/ (the bridge scopes of 5.9, the names of crosswalk/results.yaml,
// the venue lists of the harvest) have neither and are not read here.
//
// A row is one outside id in the outside system's order, with its name, its parent, and the items it maps to, each
// with the kind of match. A row that maps to no item is a gap of 18.4. A crosswalk can name paths (16.12): the rows
// under one id, read in order as the sections of a curriculum path.
package crosswalk

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/tamnd/ai-agent/node"
	"go.yaml.in/yaml/v3"
)

// Dir is the directory of the crosswalks under the root of the tree.
const Dir = "crosswalk"

// Matches are the kinds of match of a row to an item, as in programming 5.7: the item is the outside id (exact), is
// wider than it (broader), is a part of it (narrower), or is only near it (related).
var Matches = []string{"exact", "broader", "narrower", "related"}

// Crosswalk is one file of crosswalk/ with rows.
type Crosswalk struct {
	Name    string `yaml:"-" json:"name"` // the file name with no .yaml
	System  string `yaml:"system" json:"system"`
	Version string `yaml:"version,omitempty" json:"version,omitempty"`
	Source  string `yaml:"source,omitempty" json:"source,omitempty"`
	Licence string `yaml:"licence,omitempty" json:"licence,omitempty"`
	Checked string `yaml:"checked,omitempty" json:"checked,omitempty"` // the date the rows were checked against the source
	// Units is the plural noun of a row of the outside system for the site, such as "knowledge units" or "concepts".
	// The default is "units".
	Units string `yaml:"units,omitempty" json:"units,omitempty"`
	Paths []Path `yaml:"paths,omitempty" json:"paths,omitempty"`
	Rows  []Row  `yaml:"rows" json:"rows"`
}

// Path is a curriculum path of 16.12: the rows under one row (all rows when Under is empty), each a section.
type Path struct {
	ID    string `yaml:"id" json:"id"`
	Title string `yaml:"title" json:"title"`
	Under string `yaml:"under,omitempty" json:"under,omitempty"`
	// Depth is how many levels of rows under Under are sections; deeper rows join the section of their ancestor.
	// Zero is every level.
	Depth int `yaml:"depth,omitempty" json:"depth,omitempty"`
}

// Row is one outside id.
type Row struct {
	ID     string `yaml:"id" json:"id"`
	Name   string `yaml:"name" json:"name"`
	Parent string `yaml:"parent,omitempty" json:"parent,omitempty"`
	Maps   []Map  `yaml:"maps,omitempty" json:"maps,omitempty"`
	// Note is why a row maps to nothing on purpose: out of scope, or owned by a neighbour.
	Note string `yaml:"note,omitempty" json:"note,omitempty"`
}

// Map is one item of a row: a tag, a slug, the key of a card, or an area as area:<path>.
type Map struct {
	To    string `yaml:"to" json:"to"`
	Match string `yaml:"match" json:"match"`
}

// Load reads the crosswalks with rows under root, sorted by name. A tree with no crosswalk/ has none.
func Load(root string) ([]Crosswalk, error) {
	files, err := filepath.Glob(filepath.Join(root, Dir, "*.yaml"))
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	var out []Crosswalk
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		var probe struct {
			System string    `yaml:"system"`
			Rows   yaml.Node `yaml:"rows"`
		}
		if err := yaml.Unmarshal(b, &probe); err != nil {
			return nil, fmt.Errorf("%s/%s: %w", Dir, filepath.Base(f), err)
		}
		if probe.System == "" && probe.Rows.Kind == 0 {
			continue // a file of another kind
		}
		var cw Crosswalk
		dec := yaml.NewDecoder(strings.NewReader(string(b)))
		dec.KnownFields(true)
		if err := dec.Decode(&cw); err != nil {
			return nil, fmt.Errorf("%s/%s: %w", Dir, filepath.Base(f), err)
		}
		cw.Name = strings.TrimSuffix(filepath.Base(f), ".yaml")
		out = append(out, cw)
	}
	return out, nil
}

// Item is what a map resolves to: a node, a card, or the nodes of an area.
type Item struct {
	Node  *node.Node
	Key   string // the key of a card
	Area  string // the path of an area
	Match string
}

// Resolve finds the item of a map in the corpus. It is false when the target names nothing.
func Resolve(c *node.Corpus, m Map) (Item, bool) {
	it := Item{Match: m.Match}
	to := strings.TrimSpace(m.To)
	if a, ok := strings.CutPrefix(to, "area:"); ok {
		if c.AreaByID[a] == nil {
			return it, false
		}
		it.Area = a
		return it, true
	}
	if n := c.ByTag[to]; n != nil {
		it.Node = n
		return it, true
	}
	if moved, ok := c.OldSlugs[to]; ok && c.BySlug[to] == nil {
		to = moved
	}
	if n := c.BySlug[to]; n != nil {
		it.Node = n
		return it, true
	}
	if c.CardByKey[to] != nil {
		it.Key = to
		return it, true
	}
	return it, false
}

// Covered tells if an item is a page: a node, a card, or an area with at least one node in it.
func Covered(c *node.Corpus, it Item) bool {
	if it.Area != "" {
		return len(c.InArea(it.Area)) > 0
	}
	return it.Node != nil || it.Key != ""
}

// Check gives the problems of the crosswalks against the corpus: a row with no id or name, an id twice, a parent that
// is not a row before it, a map with an unknown match or a target that names nothing, and a path with no id or whose
// under is not a row.
func Check(c *node.Corpus, cws []Crosswalk) []string {
	var out []string
	paths := map[string]string{}
	for _, cw := range cws {
		at := Dir + "/" + cw.Name + ".yaml"
		if cw.System == "" {
			out = append(out, at+": no system")
		}
		ids := map[string]bool{}
		for i, r := range cw.Rows {
			where := fmt.Sprintf("%s: row %d", at, i+1)
			if r.ID != "" {
				where += " (" + r.ID + ")"
			}
			switch {
			case r.ID == "" || r.Name == "":
				out = append(out, where+": a row needs an id and a name")
			case ids[r.ID]:
				out = append(out, where+": the id is twice")
			}
			if r.Parent != "" && !ids[r.Parent] {
				out = append(out, where+": the parent "+r.Parent+" is not a row before it")
			}
			ids[r.ID] = true
			for _, m := range r.Maps {
				if !slices.Contains(Matches, m.Match) {
					out = append(out, fmt.Sprintf("%s: the match %q of %s is not one of %s", where, m.Match, m.To, strings.Join(Matches, ", ")))
				}
				if _, ok := Resolve(c, m); !ok {
					out = append(out, where+": "+m.To+" is not a tag, a slug, a card or area:<path>")
				}
			}
		}
		for _, p := range cw.Paths {
			switch {
			case p.ID == "" || p.Title == "":
				out = append(out, at+": a path needs an id and a title")
			case paths[p.ID] != "":
				out = append(out, fmt.Sprintf("%s: the path %s is also in %s", at, p.ID, paths[p.ID]))
			case strings.Contains(p.ID, "/"):
				out = append(out, at+": the path id "+p.ID+" has a /")
			}
			paths[p.ID] = at
			if p.Under != "" && !ids[p.Under] {
				out = append(out, at+": the path "+p.ID+" is under "+p.Under+", which is not a row")
			}
		}
	}
	return out
}

// Gap is a row that maps to no page and has no note: an outside id with no page, for 18.4. A row that maps only to
// areas with no node in them is a gap too.
type Gap struct {
	Crosswalk string `json:"crosswalk"`
	ID        string `json:"id"`
	Name      string `json:"name"`
}

// Gaps are the rows of the crosswalks with no note and no covered item, in order. A row with a covered item in a row
// under it is not a gap: the outside id is a heading whose parts have pages.
func Gaps(c *node.Corpus, cws []Crosswalk) []Gap {
	var out []Gap
	for _, cw := range cws {
		covered := map[string]bool{}
		for _, r := range cw.Rows {
			for _, m := range r.Maps {
				if it, ok := Resolve(c, m); ok && Covered(c, it) {
					covered[r.ID] = true
					break
				}
			}
		}
		// The rows come after their parents, so the walk from the last row up carries each row to its ancestors.
		for i := len(cw.Rows) - 1; i >= 0; i-- {
			if r := cw.Rows[i]; covered[r.ID] && r.Parent != "" {
				covered[r.Parent] = true
			}
		}
		for _, r := range cw.Rows {
			if r.Note == "" && !covered[r.ID] {
				out = append(out, Gap{Crosswalk: cw.Name, ID: r.ID, Name: r.Name})
			}
		}
	}
	return out
}

// Section is one row of a path, with the items of it and of the rows under it that are deeper than the path's depth.
type Section struct {
	Row   Row
	Items []Item
	// Rows are the rows of the section, the row first.
	Rows []Row
}

// ErrNoRow is the error of a path whose under is not a row.
var ErrNoRow = errors.New("not a row")

// Sections are the sections of a path in the order of the rows: each row under the path down to its depth, with the
// maps of the deeper rows joined to the section of their ancestor. The row that under names is a section only when it
// has maps of its own.
func (cw Crosswalk) Sections(c *node.Corpus, p Path) ([]Section, error) {
	parent := map[string]string{}
	for _, r := range cw.Rows {
		parent[r.ID] = r.Parent
	}
	if _, ok := parent[p.Under]; p.Under != "" && !ok {
		return nil, fmt.Errorf("%s: path %s: under %s: %w", cw.Name, p.ID, p.Under, ErrNoRow)
	}
	// depth is the number of steps from the row up to under (a top row is 1 when under is empty), or -1 when the row
	// is not under it. A parent that is not a row, which Check reports, ends the walk.
	depth := func(id string) int {
		d := 0
		for k, seen := id, map[string]bool{}; !seen[k]; k = parent[k] {
			if p.Under != "" && k == p.Under {
				return d
			}
			seen[k] = true
			d++
			if k == "" || parent[k] == "" {
				if p.Under == "" {
					return d
				}
				return -1
			}
		}
		return -1
	}
	var out []Section
	at := map[string]int{} // the index of the section of each row
	for _, r := range cw.Rows {
		d := depth(r.ID)
		if d < 0 || (d == 0 && len(r.Maps) == 0) {
			continue
		}
		sec, joined := at[r.Parent]
		if !joined || p.Depth == 0 || d <= p.Depth {
			sec = len(out)
			out = append(out, Section{Row: r})
		}
		at[r.ID] = sec
		out[sec].Rows = append(out[sec].Rows, r)
		for _, m := range r.Maps {
			if it, ok := Resolve(c, m); ok {
				out[sec].Items = append(out[sec].Items, it)
			}
		}
	}
	// A row with no maps of its own whose child rows are sections is a heading, not a section.
	parentOf := map[int]bool{}
	for i, sec := range out {
		if i > 0 && sec.Row.Parent != "" {
			for j := i - 1; j >= 0; j-- {
				if out[j].Row.ID == sec.Row.Parent {
					parentOf[j] = true
					break
				}
			}
		}
	}
	kept := out[:0]
	for i, sec := range out {
		if parentOf[i] && len(sec.Items) == 0 && len(sec.Rows) == 1 && len(sec.Row.Maps) == 0 {
			continue
		}
		kept = append(kept, sec)
	}
	return kept, nil
}
