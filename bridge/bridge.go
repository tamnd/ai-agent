// SPDX-License-Identifier: Apache-2.0

// Package bridge keeps the bridge cards of spec 5.9. A bridge card is a card whose item lives on a neighbour site: algorithm.tamnd.com (al), math2.tamnd.com (m2) or programming.tamnd.com (pg). It holds the tag, the slug, the name, the summary and the URL of the item, copied from the index of the neighbour. The cards are in cards/bridge/<site>/<prefix>.jsonl, one card on each line, sorted by tag.
package bridge

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Dir is the directory of the bridge cards, relative to the content root.
const Dir = "cards/bridge" // card.BridgeDir

// MaxShard is the size at which a shard file is split by one more character of the tag (spec 20.3).
var MaxShard = 5 << 20

// Site is one neighbour.
type Site struct {
	ID    string // al, m2 or pg
	Name  string // the long name, as crosswalk/<name>.yaml uses it
	Host  string
	Index string // the path of the index of all items on the host
	Scope Scope  // the items that get a bridge card when crosswalk/<name>.yaml does not exist
}

// Sites are the three neighbours, by their id.
var Sites = map[string]*Site{
	"al": {ID: "al", Name: "algorithm", Host: "algorithm.tamnd.com", Index: "/api/nodes.json", Scope: Scope{
		// The machine learning, search and logic nodes of algorithm (spec 5.9).
		Areas: []string{"machine-learning", "games-and-search", "logic-and-constraints"},
	}},
	"m2": {ID: "m2", Name: "math2", Host: "math2.tamnd.com", Index: "/api/nodes.json", Scope: Scope{
		// Linear algebra, probability, statistics, artificial intelligence, mathematical programming and information theory, by the primary MSC class.
		MSC:   []string{"15A", "60", "62", "68T", "90C", "94A"},
		Kinds: []string{"concept", "definition", "statement", "object"},
	}},
	"pg": {ID: "pg", Name: "programming", Host: "programming.tamnd.com", Index: "/api/nodes.json", Scope: Scope{All: true}},
}

// IDs gives the ids of the sites in order.
func IDs() []string { return []string{"al", "m2", "pg"} }

// SiteOf gives the site of an id, or of its long name.
func SiteOf(id string) *Site {
	if s := Sites[id]; s != nil {
		return s
	}
	for _, s := range Sites {
		if s.Name == id || s.Host == id {
			return s
		}
	}
	return nil
}

// URL is the URL of the index of the site.
func (s *Site) URL() string { return "https://" + s.Host + s.Index }

// Scope selects the items of a neighbour that get a bridge card.
type Scope struct {
	All   bool     `yaml:"all,omitempty"`
	Areas []string `yaml:"areas,omitempty"` // area prefixes, such as machine-learning
	MSC   []string `yaml:"msc,omitempty"`   // prefixes of the primary MSC class, such as 62
	Kinds []string `yaml:"kinds,omitempty"` // when not empty, only these kinds of the neighbour
	Tags  []string `yaml:"tags,omitempty"`  // more items, by their tag
}

// LoadScope reads the bridge scope of crosswalk/<name>.yaml under root. It gives the default scope of the site when the file does not exist or has no bridge key.
func LoadScope(root string, s *Site) (Scope, error) {
	b, err := os.ReadFile(filepath.Join(root, "crosswalk", s.Name+".yaml"))
	if errors.Is(err, fs.ErrNotExist) {
		return s.Scope, nil
	}
	if err != nil {
		return Scope{}, err
	}
	var f struct {
		Bridge *Scope `yaml:"bridge"`
	}
	if err := yaml.Unmarshal(b, &f); err != nil {
		return Scope{}, fmt.Errorf("crosswalk/%s.yaml: %w", s.Name, err)
	}
	if f.Bridge == nil {
		return s.Scope, nil
	}
	return *f.Bridge, nil
}

// Has tells if the item is in the scope.
func (sc Scope) Has(it Item) bool {
	if slices.Contains(sc.Tags, it.Tag) {
		return true
	}
	if len(sc.Kinds) > 0 && !slices.Contains(sc.Kinds, it.Kind) {
		return false
	}
	if sc.All {
		return true
	}
	for _, a := range sc.Areas {
		if it.Area == a || strings.HasPrefix(it.Area, a+"/") {
			return true
		}
	}
	if len(it.MSC) > 0 {
		for _, m := range sc.MSC {
			if strings.HasPrefix(it.MSC[0], m) {
				return true
			}
		}
	}
	return false
}

// Item is one item of the index of a neighbour.
type Item struct {
	Tag     string   `json:"tag"`
	Kind    string   `json:"kind"`
	Slug    string   `json:"slug"`
	Name    string   `json:"name"`
	Aliases []string `json:"aliases"`
	Area    string   `json:"area"`
	MSC     []string `json:"msc"`
	Summary string   `json:"summary"`
	URL     string   `json:"url"`
}

// Index is the index of all items of a neighbour, as its /api/nodes.json gives it.
type Index struct {
	Commit string `json:"commit"`
	Nodes  []Item `json:"nodes"`
}

// ParseIndex reads an index. An item with no slug takes the last part of the path of its URL, as math2 gives it.
func ParseIndex(b []byte) (*Index, error) {
	var idx Index
	if err := json.Unmarshal(b, &idx); err != nil {
		return nil, fmt.Errorf("the index is not valid JSON: %w", err)
	}
	if len(idx.Nodes) == 0 {
		return nil, errors.New("the index has no items")
	}
	for i := range idx.Nodes {
		it := &idx.Nodes[i]
		if it.Slug == "" && it.URL != "" {
			if u, err := url.Parse(it.URL); err == nil {
				it.Slug = path.Base(strings.TrimSuffix(u.Path, "/"))
			}
		}
	}
	return &idx, nil
}

// Card is one bridge card.
type Card struct {
	Tag      string   `json:"tag"`
	Site     string   `json:"site"`
	Kind     string   `json:"kind"` // the kind on the neighbour, such as algorithm or definition
	Slug     string   `json:"slug"`
	Name     string   `json:"name"`
	Aliases  []string `json:"aliases,omitempty"`
	Area     string   `json:"area,omitempty"`
	MSC      []string `json:"msc,omitempty"`
	Summary  string   `json:"summary,omitempty"`
	URL      string   `json:"url"`
	OldSlugs []string `json:"old_slugs,omitempty"` // the slugs that the neighbour used before; a link with an old slug still resolves
	Retired  string   `json:"retired,omitempty"`   // the date of the sync that did not find the tag; the card is then a broken-bridge gap (spec 18.7)
	Commit   string   `json:"commit,omitempty"`    // the commit of the neighbour at the sync

	File string `json:"-"`
	Line int    `json:"-"`
}

// Key is the key of the card in an edge field: the slug and the site, such as k-means@al.
func (c *Card) Key() string { return c.Slug + "@" + c.Site }

// Ref is the form of the card in a wiki link, such as al:k-means.
func (c *Card) Ref() string { return c.Site + ":" + c.Slug }

// Mark is the note that a link to the card carries, such as "on algorithm.tamnd.com".
func (c *Card) Mark() string {
	if s := Sites[c.Site]; s != nil {
		return "on " + s.Host
	}
	return "on " + c.Site
}

// Error is a line that is not a bridge card.
type Error struct {
	File string
	Line int
	Err  error
}

func (e *Error) Error() string { return fmt.Sprintf("%s:%d: %v", e.File, e.Line, e.Err) }

// Read reads the bridge cards under root/cards/bridge. It returns the cards in the order of the files and their lines.
func Read(root string) ([]*Card, []error) {
	var out []*Card
	var bad []error
	err := filepath.WalkDir(filepath.Join(root, filepath.FromSlash(Dir)), func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(p, ".jsonl") {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		site := path.Base(path.Dir(rel))
		b, err := os.ReadFile(p)
		if err != nil {
			bad = append(bad, err)
			return nil
		}
		sc := bufio.NewScanner(bytes.NewReader(b))
		sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
		for line := 1; sc.Scan(); line++ {
			t := bytes.TrimSpace(sc.Bytes())
			if len(t) == 0 {
				continue
			}
			c := &Card{}
			if err := json.Unmarshal(t, c); err != nil {
				bad = append(bad, &Error{File: rel, Line: line, Err: err})
				continue
			}
			if c.Site != site {
				bad = append(bad, &Error{File: rel, Line: line, Err: fmt.Errorf("the card %s has the site %q, but it is in %s/%s/", c.Tag, c.Site, Dir, site)})
			}
			if c.Tag == "" || c.Slug == "" || c.URL == "" {
				bad = append(bad, &Error{File: rel, Line: line, Err: errors.New("a bridge card needs a tag, a slug and a URL")})
				continue
			}
			c.File, c.Line = rel, line
			out = append(out, c)
		}
		if err := sc.Err(); err != nil {
			bad = append(bad, &Error{File: rel, Err: err})
		}
		return nil
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		bad = append(bad, err)
	}
	return out, bad
}

// Set holds the bridge cards of all sites, with the indexes that the build and the checks use.
type Set struct {
	Cards []*Card
	// ByRef maps site:slug, and site:old-slug, to the card.
	ByRef map[string]*Card
}

// NewSet indexes the cards.
func NewSet(cs []*Card) *Set {
	s := &Set{Cards: cs, ByRef: map[string]*Card{}}
	for _, c := range cs {
		if _, dup := s.ByRef[c.Ref()]; !dup {
			s.ByRef[c.Ref()] = c
		}
	}
	for _, c := range cs {
		for _, o := range c.OldSlugs {
			if _, dup := s.ByRef[c.Site+":"+o]; !dup {
				s.ByRef[c.Site+":"+o] = c
			}
		}
	}
	return s
}

// Split cuts a reference into its site and slug. It takes the edge form slug@al and the link form al:slug. ok is false when the reference does not name a neighbour.
func Split(ref string) (site, slug string, ok bool) {
	if s, site, found := strings.Cut(ref, "@"); found && Sites[site] != nil && s != "" {
		return site, s, true
	}
	if site, s, found := strings.Cut(ref, ":"); found && Sites[site] != nil && s != "" {
		return site, s, true
	}
	return "", "", false
}

// Find gives the card of a reference, in the form slug@al or al:slug.
func (s *Set) Find(ref string) *Card {
	if s == nil {
		return nil
	}
	site, slug, ok := Split(ref)
	if !ok {
		return nil
	}
	return s.ByRef[site+":"+slug]
}

// Report says what a sync changed.
type Report struct {
	Site    string
	Commit  string
	Cards   int
	Added   []string
	Renamed []string // old -> new
	Changed []string
	Retired []string
	Back    []string // tags that were retired and are in the index again
	Files   []string
}

// Sync makes the bridge cards of one site from its index, and writes them to root/cards/bridge/<site>/. It follows the tag: a card whose tag is in the index takes the new slug and name, and keeps the old slug; a card whose tag is not in the index stays, with the date of the sync in retired. The items of the scope get a card when they do not have one.
func Sync(root string, s *Site, idx *Index, scope Scope, today string) (*Report, error) {
	all, _ := Read(root)
	old := map[string]*Card{}
	for _, c := range all {
		if c.Site == s.ID {
			old[c.Tag] = c
		}
	}
	rep := &Report{Site: s.ID, Commit: idx.Commit}
	seen := map[string]bool{}
	var out []*Card
	for _, it := range idx.Nodes {
		if it.Tag == "" || it.Slug == "" || seen[it.Tag] {
			continue
		}
		prev := old[it.Tag]
		if prev == nil && !scope.Has(it) {
			continue
		}
		seen[it.Tag] = true
		c := &Card{Tag: it.Tag, Site: s.ID, Kind: it.Kind, Slug: it.Slug, Name: it.Name, Aliases: it.Aliases, Area: it.Area, MSC: it.MSC, Summary: it.Summary, URL: it.URL, Commit: idx.Commit}
		if c.URL == "" {
			c.URL = "https://" + s.Host + "/" + it.Slug + "/"
		}
		if prev == nil {
			rep.Added = append(rep.Added, c.Slug)
		} else {
			c.OldSlugs = prev.OldSlugs
			if prev.Slug != c.Slug {
				if !slices.Contains(c.OldSlugs, prev.Slug) {
					c.OldSlugs = append(slices.Clone(c.OldSlugs), prev.Slug)
				}
				c.OldSlugs = slices.DeleteFunc(slices.Clone(c.OldSlugs), func(o string) bool { return o == c.Slug })
				rep.Renamed = append(rep.Renamed, prev.Slug+" -> "+c.Slug)
			} else if prev.Name != c.Name || prev.Summary != c.Summary || prev.URL != c.URL || prev.Kind != c.Kind {
				rep.Changed = append(rep.Changed, c.Slug)
			}
			if prev.Retired != "" {
				rep.Back = append(rep.Back, c.Slug)
			}
		}
		out = append(out, c)
	}
	for tag, c := range old {
		if seen[tag] {
			continue
		}
		if c.Retired == "" {
			c.Retired = today
			rep.Retired = append(rep.Retired, c.Slug)
		}
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Tag < out[j].Tag })
	for _, l := range [][]string{rep.Added, rep.Renamed, rep.Changed, rep.Retired, rep.Back} {
		sort.Strings(l)
	}
	rep.Cards = len(out)
	files, err := write(root, s.ID, out)
	rep.Files = files
	return rep, err
}

// write writes the cards of a site to their shards. It uses the shortest prefix of the tag at which no shard passes MaxShard, and removes the shards of the site that it does not write.
func write(root, site string, cs []*Card) ([]string, error) {
	lines := make([][]byte, len(cs))
	for i, c := range cs {
		b, err := json.Marshal(c)
		if err != nil {
			return nil, err
		}
		lines[i] = append(b, '\n')
	}
	var shards map[string][]byte
	for k := 0; ; k++ {
		shards = map[string][]byte{}
		big := false
		for i, c := range cs {
			name := "_"
			if k > 0 {
				name = c.Tag[:min(k, len(c.Tag))]
			}
			shards[name] = append(shards[name], lines[i]...)
			big = big || len(shards[name]) > MaxShard
		}
		if !big || k >= 6 {
			break
		}
	}
	dir := filepath.Join(root, filepath.FromSlash(Dir), site)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, e := range ents {
		name := strings.TrimSuffix(e.Name(), ".jsonl")
		if _, keep := shards[name]; !keep && strings.HasSuffix(e.Name(), ".jsonl") {
			if err := os.Remove(filepath.Join(dir, e.Name())); err != nil {
				return nil, err
			}
		}
	}
	var files []string
	for name, b := range shards {
		p := filepath.Join(dir, name+".jsonl")
		if err := os.WriteFile(p, b, 0o644); err != nil {
			return nil, err
		}
		files = append(files, Dir+"/"+site+"/"+name+".jsonl")
	}
	sort.Strings(files)
	return files, nil
}
