// SPDX-License-Identifier: Apache-2.0

// Package card reads the cards of the content repository (spec 04.6). A card is one JSON object on one line of cards/<kind>/<shard>.jsonl: the front-matter fields of its kind, its facts with their sources, a summary of at most 30 words, and the harvest records that say where it came from.
package card

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
)

// Dir is the directory of the cards in the content repository.
const Dir = "cards"

// BridgeDir is the directory of the bridge cards, which Read skips.
const BridgeDir = Dir + "/bridge"

// MaxSummary is the most words that the summary of a card can have.
const MaxSummary = 30

// Fact is one fact field of a card, such as params.total: {"v": 4.05e11, "src": "S:meta-llama31-paper", "loc": "Table 3"}.
type Fact struct {
	ID  string `json:"id"`
	V   any    `json:"v"`
	Src string `json:"src,omitempty"`
	Loc string `json:"loc,omitempty"`
}

// Harvest says where the row of a card came from.
type Harvest struct {
	Source   string `json:"source"`
	Snapshot string `json:"snapshot,omitempty"`
	Row      string `json:"row,omitempty"`
	Date     string `json:"date,omitempty"`
}

// Card is one card.
type Card struct {
	Tag     string
	Kind    string
	Slug    string
	Owner   string // the owner of an owned slug, such as the family of a model checkpoint
	Name    string
	Role    string
	Summary string
	Harvest []Harvest
	Facts   []Fact
	// Fields holds each field of the line as it is in the file.
	Fields map[string]json.RawMessage

	File string // the path of the shard, relative to the content root
	Line int
	// Bad holds the problems of the fields that the reader found, such as a fact with no value. Package check reports them.
	Bad []string
}

// Key is the key of the card in its namespace: the slug, or the owner and the slug.
func (c *Card) Key() string {
	if c.Owner != "" {
		return c.Owner + "/" + c.Slug
	}
	return c.Slug
}

// Error is a line that is not a card.
type Error struct {
	File string
	Line int
	Err  error
}

func (e *Error) Error() string { return fmt.Sprintf("%s:%d: %v", e.File, e.Line, e.Err) }

// notFacts are the fields with an object value that are not facts.
var notFacts = map[string]bool{"same_as": true, "harvest": true, "rules": true, "aliases": true}

// resultFields are the fields of a result card that are records, not facts (spec 10.6.1).
var resultFields = map[string]bool{"score": true, "setting": true, "source": true, "checks": true, "versions": true}

// Read reads every shard under root/cards. It returns the cards in the order of the files and their lines, and the lines that are not cards.
func Read(root string) ([]*Card, []error) {
	var out []*Card
	var bad []error
	dir := filepath.Join(root, Dir)
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if d.IsDir() && rel == BridgeDir {
			// Package bridge reads the bridge cards (spec 5.9). They are the items of a neighbour, so the rules of our cards do not apply to them.
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(p, ".jsonl") {
			return nil
		}
		cs, errs := readFile(p, rel)
		out = append(out, cs...)
		bad = append(bad, errs...)
		return nil
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		bad = append(bad, err)
	}
	return out, bad
}

func readFile(path, rel string) ([]*Card, []error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, []error{err}
	}
	defer f.Close()
	var out []*Card
	var bad []error
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for line := 1; sc.Scan(); line++ {
		b := bytes.TrimSpace(sc.Bytes())
		if len(b) == 0 {
			continue
		}
		c, err := Parse(b)
		if err != nil {
			bad = append(bad, &Error{File: rel, Line: line, Err: err})
			continue
		}
		c.File, c.Line = rel, line
		out = append(out, c)
	}
	if err := sc.Err(); err != nil {
		bad = append(bad, &Error{File: rel, Err: err})
	}
	return out, bad
}

// Parse reads one card from its JSON line.
func Parse(b []byte) (*Card, error) {
	c := &Card{}
	if err := json.Unmarshal(b, &c.Fields); err != nil {
		return nil, err
	}
	if c.Fields == nil {
		return nil, errors.New("the line is not a JSON object")
	}
	for k, dst := range map[string]*string{"tag": &c.Tag, "kind": &c.Kind, "slug": &c.Slug, "owner": &c.Owner, "name": &c.Name, "role": &c.Role, "summary": &c.Summary} {
		raw, ok := c.Fields[k]
		if !ok {
			continue
		}
		if err := json.Unmarshal(raw, dst); err != nil {
			return nil, fmt.Errorf("the field %s is not a string", k)
		}
	}
	if raw, ok := c.Fields["harvest"]; ok {
		if err := json.Unmarshal(raw, &c.Harvest); err != nil {
			var h Harvest
			if err := json.Unmarshal(raw, &h); err != nil {
				return nil, errors.New("the field harvest is not a harvest record or a list of them")
			}
			c.Harvest = []Harvest{h}
		}
	}
	keys := make([]string, 0, len(c.Fields))
	for k := range c.Fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if !notFacts[k] && (!resultFields[k] || c.Kind != "result" && k != "versions") {
			c.facts(k, c.Fields[k])
		}
	}
	if c.Kind == "result" {
		c.result()
	}
	return c, nil
}

// result gives a result card the names that it does not write. A result belongs to its benchmark, so the owner is the benchmark and the slug is the tag in lowercase (spec 04.4). The name says the model and the benchmark.
func (c *Card) result() {
	str := func(k string) string {
		var s string
		_ = json.Unmarshal(c.Fields[k], &s)
		return s
	}
	if c.Owner == "" {
		c.Owner = str("benchmark")
	}
	if c.Slug == "" {
		c.Slug = strings.ToLower(c.Tag)
	}
	if c.Name == "" && str("model") != "" {
		c.Name = str("model") + " on " + str("benchmark")
		if v := str("version"); v != "" {
			c.Name += " " + v
		}
	}
}

// facts reads the fact fields under id. An object with the key v is a fact. Another object holds more fact fields, as training holds training.tokens.
func (c *Card) facts(id string, raw json.RawMessage) {
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil || m == nil {
		return
	}
	if v, ok := m["v"]; ok {
		f := Fact{ID: id}
		_ = json.Unmarshal(v, &f.V)
		if s, ok := m["src"]; ok && json.Unmarshal(s, &f.Src) != nil {
			c.Bad = append(c.Bad, fmt.Sprintf("the src of the fact %s is not a string", id))
		}
		if l, ok := m["loc"]; ok && json.Unmarshal(l, &f.Loc) != nil {
			c.Bad = append(c.Bad, fmt.Sprintf("the loc of the fact %s is not a string", id))
		}
		c.Facts = append(c.Facts, f)
		return
	}
	if _, ok := m["src"]; ok {
		c.Bad = append(c.Bad, fmt.Sprintf("the fact %s has a src but no v", id))
		return
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		c.facts(id+"."+k, m[k])
	}
}

// Words counts the words of a summary.
func Words(s string) int { return len(strings.Fields(s)) }

// Shard is the prefix of the tag that a shard file holds: "_" holds every tag, and "5T" holds the tags that start with 5T (spec 20.3).
func Shard(file string) string { return strings.TrimSuffix(filepath.Base(file), ".jsonl") }

// InShard tells if the tag belongs in the shard.
func InShard(tag, shard string) bool { return shard == "_" || strings.HasPrefix(tag, shard) }
