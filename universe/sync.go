// SPDX-License-Identifier: Apache-2.0

package universe

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tamnd/ai-agent/card"
	"github.com/tamnd/ai-agent/harvest"
	"github.com/tamnd/ai-agent/node"
)

// Curated is an item of the curated catalog with the outcome card, as work/stub.py writes it.
type Curated struct {
	Slug      string            `json:"slug"`
	Kind      string            `json:"kind"`
	Name      string            `json:"name"`
	Area      string            `json:"area,omitempty"`
	Aliases   []string          `json:"aliases,omitempty"`
	Note      string            `json:"note,omitempty"`
	Sources   []string          `json:"sources,omitempty"`
	SameAs    map[string]string `json:"same_as,omitempty"`
	Universe  string            `json:"universe,omitempty"`
	Merged    []string          `json:"merged,omitempty"`
	Outcome   string            `json:"outcome,omitempty"`
	Target    string            `json:"target,omitempty"`
	DecidedBy string            `json:"decided_by,omitempty"`
}

// ReadCurated reads a catalog file of curated items, one JSON object per line.
func ReadCurated(path string) ([]Curated, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []Curated
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for n := 1; sc.Scan(); n++ {
		b := bytes.TrimSpace(sc.Bytes())
		if len(b) == 0 {
			continue
		}
		var c Curated
		if err := json.Unmarshal(b, &c); err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, n, err)
		}
		out = append(out, c)
	}
	return out, sc.Err()
}

// SyncReport is the record of a card sync.
type SyncReport struct {
	Items     int      `json:"items"`
	Created   int      `json:"created"`
	Updated   int      `json:"updated"`
	Unchanged int      `json:"unchanged"`
	Added     int      `json:"facts_added"`
	Changed   int      `json:"facts_changed"`
	Disagree  int      `json:"facts_disagree"`
	Merged    int      `json:"merged"`
	Notes     []string `json:"notes,omitempty"`
}

type shards struct {
	root  string
	kinds map[string][]*card.Line // the lines of each kind, from all its shards
	dirty map[string]bool
	byTag map[string]*card.Line
	byKey map[string]*card.Line
	bySA  map[string]*card.Line
	used  map[string]bool // slugs of nodes, cards and reserved words
	// claimed holds the job that wrote each card in this run, so two items never write one card.
	claimed map[*card.Line]*job
}

func readShards(root string, c *node.Corpus) (*shards, error) {
	s := &shards{root: root, kinds: map[string][]*card.Line{}, dirty: map[string]bool{}, byTag: map[string]*card.Line{},
		byKey: map[string]*card.Line{}, bySA: map[string]*card.Line{}, used: map[string]bool{},
		claimed: map[*card.Line]*job{}}
	dirs, err := os.ReadDir(filepath.Join(root, card.Dir))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	for _, d := range dirs {
		if !d.IsDir() || card.Dir+"/"+d.Name() == card.BridgeDir {
			continue
		}
		lines, err := card.ReadKind(filepath.Join(root, card.Dir, d.Name()))
		if err != nil {
			return nil, err
		}
		s.kinds[d.Name()] = lines
		for _, l := range lines {
			s.index(l)
		}
	}
	if c != nil {
		for slug := range c.BySlug {
			s.used[slug] = true
		}
		for w := range c.Reserved {
			s.used[w] = true
		}
	}
	return s, nil
}

func (s *shards) index(l *card.Line) {
	c := l.Card
	s.byTag[c.Tag] = l
	s.byKey[c.Kind+"/"+c.Key()] = l
	s.used[c.Slug] = true
	var same map[string]string
	_ = json.Unmarshal(c.Fields["same_as"], &same)
	for k, v := range same {
		if v != "" {
			s.bySA[c.Kind+"/"+k+"="+NormID(k, v)] = l
		}
	}
}

// slug finds a free slug: the wanted one, then with the kind, then with a number.
func (s *shards) slug(want, kind string) string {
	if !s.used[want] {
		return want
	}
	base := card.Slugify(want + " " + kind)
	if !s.used[base] {
		return base
	}
	for n := 2; ; n++ {
		if x := card.Slugify(fmt.Sprintf("%s %d", base, n)); !s.used[x] {
			return x
		}
	}
}

func (s *shards) create(kind, slug, name string) *card.Line {
	c := &card.Card{Tag: "NEW-" + slug, Kind: kind, Slug: slug, Name: name}
	for k, v := range map[string]string{"tag": c.Tag, "kind": kind, "slug": slug, "name": name} {
		_ = c.Set(k, v)
	}
	l := &card.Line{Card: c, Changed: true}
	s.kinds[kind] = append(s.kinds[kind], l)
	s.dirty[kind] = true
	s.index(l)
	return l
}

// write writes the shards of each kind that changed, and splits a shard that passes 5 MB (20.3).
func (s *shards) write() error {
	for kind, lines := range s.kinds {
		changed := s.dirty[kind]
		for _, l := range lines {
			changed = changed || l.Changed
		}
		if !changed {
			continue
		}
		if err := card.WriteKind(filepath.Join(s.root, card.Dir, kind), lines, card.MaxShard); err != nil {
			return err
		}
	}
	return nil
}

type job struct {
	item *Item
	cur  *Curated
}

// SyncOptions limit a sync to the items of one kind or with a row of one source; the zero value syncs every item.
type SyncOptions struct {
	Kind   string
	Source string
}

func (o SyncOptions) wants(j *job) bool {
	if o.Kind != "" && j.kind() != o.Kind {
		return false
	}
	if o.Source == "" {
		return true
	}
	for _, s := range j.sources() {
		if src, _, _ := strings.Cut(s, ":"); src == o.Source {
			return true
		}
	}
	return false
}

// Sync writes the card of each universe item with the outcome card and of each curated card item: it makes the
// cards that are missing, with provisional tags for aik mint, and updates the facts, the ids, the aliases and the
// harvest records of the cards that exist. A fact that a person or another source wrote is never replaced; when
// the harvested value differs, the report counts a disagreement for the fact model of 4.9.
func Sync(root string, c *node.Corpus, curated []Curated, date string, o SyncOptions) (*SyncReport, error) {
	items, err := Read(root)
	if err != nil {
		return nil, err
	}
	byID := map[string]*Item{}
	for _, it := range items {
		byID[it.ID] = it
	}
	inv, err := harvest.ReadInventories(root)
	if err != nil {
		return nil, err
	}
	rows := map[string]*harvest.Row{}
	for s, rs := range inv {
		for i := range rs {
			rows[s+":"+rs[i].ID] = &rs[i]
		}
	}
	sh, err := readShards(root, c)
	if err != nil {
		return nil, err
	}
	rep := &SyncReport{}
	var jobs []*job
	seen := map[string]*job{}
	for _, it := range items {
		if it.Outcome == "card" {
			j := &job{item: it}
			jobs = append(jobs, j)
			seen[it.ID] = j
		}
	}
	for i := range curated {
		cu := &curated[i]
		if cu.Outcome != "" && cu.Outcome != "card" {
			continue
		}
		if cu.Kind == "" || cu.Name == "" || !node.IsKind(cu.Kind) {
			rep.Notes = append(rep.Notes, fmt.Sprintf("the curated item %q has no kind of 04.2 or no name", cu.Slug))
			continue
		}
		if j := seen[cu.Universe]; j != nil && cu.Universe != "" {
			j.cur = cu
			continue
		}
		j := &job{item: byID[cu.Universe], cur: cu}
		jobs = append(jobs, j)
	}
	regDirty := false
	for _, j := range jobs {
		if !o.wants(j) {
			continue
		}
		rep.Items++
		l, state, err := sh.sync(j, rows, rep)
		if err != nil {
			return nil, err
		}
		switch state {
		case "created":
			rep.Created++
		case "updated":
			rep.Updated++
		default:
			rep.Unchanged++
		}
		it := j.item
		if it == nil {
			continue
		}
		if uri := URI(l.Card.Tag); it.Target != uri || it.Outcome != "card" {
			it.Target, it.Outcome = uri, "card"
			regDirty = true
		}
		if j.cur != nil && j.cur.DecidedBy != "" && it.DecidedBy != j.cur.DecidedBy {
			it.DecidedBy, it.Decided = j.cur.DecidedBy, date
			regDirty = true
		}
		if j.cur != nil {
			for _, id := range j.cur.Merged {
				o := byID[id]
				if o == nil || o == it || o.Outcome == "merged" {
					continue
				}
				if o.Outcome == "node" || o.Outcome == "card" {
					rep.Notes = append(rep.Notes, fmt.Sprintf("%s is a %s; a person merges it into %s (18.3)", id, o.Outcome, it.ID))
					continue
				}
				o.Outcome, o.Target, o.DecidedBy, o.Decided = "merged", it.ID, j.cur.DecidedBy, date
				rep.Merged++
				regDirty = true
			}
		}
	}
	if err := sh.write(); err != nil {
		return nil, err
	}
	if regDirty {
		if err := Write(root, items); err != nil {
			return nil, err
		}
	}
	return rep, nil
}

func (s *shards) find(j *job) *card.Line {
	kind := j.kind()
	for _, t := range []string{TagOf(j.target()), TagOf(j.curTarget())} {
		if l := s.byTag[t]; t != "" && l != nil {
			return l
		}
	}
	if j.cur != nil && j.cur.Slug != "" {
		if l := s.byKey[kind+"/"+j.cur.Slug]; l != nil {
			return l
		}
	}
	keys := make([]string, 0)
	for k, v := range j.sameAs() {
		keys = append(keys, kind+"/"+k+"="+NormID(k, v))
	}
	sort.Strings(keys)
	for _, k := range keys {
		if l := s.bySA[k]; l != nil {
			return l
		}
	}
	return nil
}

func (j *job) id() string {
	if j.item != nil {
		return j.item.ID
	}
	return j.cur.Slug
}

func (j *job) kind() string {
	if j.cur != nil {
		return j.cur.Kind
	}
	return j.item.Kind
}

func (j *job) target() string {
	if j.item != nil {
		return j.item.Target
	}
	return ""
}

func (j *job) curTarget() string {
	if j.cur != nil {
		return j.cur.Target
	}
	return ""
}

func (j *job) sameAs() map[string]string {
	m := map[string]string{}
	if j.item != nil {
		for k, v := range j.item.SameAs {
			m[k] = v
		}
	}
	if j.cur != nil {
		for k, v := range j.cur.SameAs {
			if v != "" {
				m[k] = v
			}
		}
	}
	return m
}

func (j *job) sources() []string {
	var out []string
	if j.item != nil {
		out = append(out, j.item.Sources...)
	}
	if j.cur != nil {
		out = append(out, j.cur.Sources...)
	}
	return out
}

func (j *job) aliases() []string {
	var out []string
	if j.item != nil {
		out = append(out, j.item.Aliases...)
	}
	if j.cur != nil {
		out = append(out, j.cur.Aliases...)
	}
	return out
}
