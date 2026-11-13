// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/tamnd/ai-agent/card"
	"github.com/tamnd/ai-agent/node"
	"github.com/tamnd/ai-agent/universe"
)

// crockford is the alphabet of tags (spec 04.2). It has no I, L, O or U.
const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// tagSource gives random numbers for new tags. Tests replace it to get the same tags each run.
var (
	cryptoSource io.Reader = rand.Reader
	tagSource              = cryptoSource
)

var (
	provisionalRE = regexp.MustCompile(`^NEW-[a-z0-9-]+$`)
	tagLineRE     = regexp.MustCompile(`(?m)^tag:[ \t]*\S+[ \t]*$`)
)

func newTag(used map[string]bool) (string, error) {
	max := big.NewInt(int64(len(crockford)))
	for range 1000 {
		var b strings.Builder
		for range 6 {
			i, err := rand.Int(tagSource, max)
			if err != nil {
				return "", err
			}
			b.WriteByte(crockford[i.Int64()])
		}
		if t := b.String(); !used[t] {
			used[t] = true
			return t, nil
		}
	}
	return "", fmt.Errorf("no free tag after 1000 tries")
}

// readTagFile returns the tags of a register file. Each line that is not a comment starts with a tag.
func readTagFile(path string, into map[string]bool) error {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		into[strings.Fields(line)[0]] = true
	}
	return s.Err()
}

type minted struct {
	n   *node.Node
	c   *card.Card
	tag string
}

func (m minted) old() string {
	if m.c != nil {
		return m.c.Tag
	}
	return m.n.Tag
}

func (m minted) key() string {
	if m.c != nil {
		return m.c.Key()
	}
	return m.n.Slug
}

func (m minted) rel() string {
	if m.c != nil {
		return m.c.File
	}
	return m.n.Rel
}

// cmdMint gives a permanent tag to each node and card that has a provisional one (NEW-...), writes it into the
// node or the card line, adds the line of tags.reg and points the universe items at the new tag.
func cmdMint(args []string) error {
	f := newFlags("mint", "[slug ...]")
	root := f.String("root", ".", "the root of the content repository")
	dry := f.Bool("dry-run", false, "print the new tags and change no file")
	author := f.String("author", "", "the author for a new provenance file; the default is $GITHUB_ACTOR or the git user name")
	if err := parse(f, args); err != nil {
		return err
	}
	c, err := load(*root)
	if err != nil {
		return err
	}
	only := map[string]bool{}
	for _, s := range f.Args() {
		if c.BySlug[s] == nil && c.CardByKey[s] == nil {
			return fmt.Errorf("no node or card has the slug %q", s)
		}
		only[s] = true
	}
	used := map[string]bool{}
	for _, t := range c.Tags {
		used[t.Tag] = true
	}
	for _, n := range c.Nodes {
		used[n.Tag] = true
	}
	for _, cd := range c.Cards {
		used[cd.Tag] = true
	}
	if err := readTagFile(filepath.Join(*root, "registers", "retired.reg"), used); err != nil {
		return err
	}
	var todo []minted
	for _, n := range c.Nodes {
		if provisionalRE.MatchString(n.Tag) && (len(only) == 0 || only[n.Slug]) {
			todo = append(todo, minted{n: n})
		}
	}
	for _, cd := range c.Cards {
		if provisionalRE.MatchString(cd.Tag) && (len(only) == 0 || only[cd.Key()]) {
			todo = append(todo, minted{c: cd})
		}
	}
	sort.SliceStable(todo, func(i, j int) bool { return todo[i].key() < todo[j].key() })
	for i := range todo {
		if todo[i].tag, err = newTag(used); err != nil {
			return err
		}
	}
	if len(todo) == 0 {
		fmt.Println("no provisional ids")
		return nil
	}
	for _, m := range todo {
		fmt.Printf("%s %s %s\n", m.old(), m.tag, m.rel())
	}
	if *dry {
		return nil
	}
	if *author == "" {
		*author = defaultAuthor(*root)
	}
	reg := filepath.Join(*root, "registers", "tags.reg")
	var lines strings.Builder
	cards := map[string]map[string]string{} // the kind, then the old tag and the new one
	moved := map[string]string{}
	for _, m := range todo {
		if m.c != nil {
			if cards[m.c.Kind] == nil {
				cards[m.c.Kind] = map[string]string{}
			}
			cards[m.c.Kind][m.c.Tag] = m.tag
		} else if err := mintNode(*root, m, *author); err != nil {
			return err
		}
		moved[m.old()] = m.tag
		fmt.Fprintf(&lines, "%s %s\n", m.tag, m.key())
	}
	kinds := make([]string, 0, len(cards))
	for kind := range cards {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	for _, kind := range kinds {
		if err := mintCards(filepath.Join(*root, card.Dir, kind), cards[kind]); err != nil {
			return err
		}
	}
	if err := appendLines(reg, lines.String()); err != nil {
		return err
	}
	return universe.Retarget(*root, moved)
}

// mintCards writes the new tags into the card lines of one kind, and moves each line to the shard of its new tag
// (20.3). Only the tag of a line changes; the other bytes stay as they are.
func mintCards(dir string, tags map[string]string) error {
	lines, err := card.ReadKind(dir)
	if err != nil {
		return err
	}
	for _, l := range lines {
		t, ok := tags[l.Card.Tag]
		if !ok {
			continue
		}
		re := regexp.MustCompile(`"tag"(\s*):(\s*)"` + regexp.QuoteMeta(l.Card.Tag) + `"`)
		done := false
		raw := re.ReplaceAllFunc(l.Raw, func(m []byte) []byte {
			if done {
				return m
			}
			done = true
			return re.ReplaceAll(m, []byte(`"tag"${1}:${2}"`+t+`"`))
		})
		if err := l.Card.Set("tag", t); err != nil {
			return err
		}
		l.Changed = l.Changed || bytes.Equal(raw, l.Raw)
		l.Raw, l.Card.Tag = raw, t
	}
	return card.WriteKind(dir, lines, card.MaxShard)
}

func mintNode(root string, m minted, author string) error {
	p := filepath.Join(m.n.Dir, "node.md")
	b, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	text := string(b)
	end := strings.Index(text[3:], "\n---")
	if !strings.HasPrefix(text, "---") || end < 0 {
		return fmt.Errorf("%s: no front matter", p)
	}
	front := text[:end+3]
	loc := tagLineRE.FindStringIndex(front)
	if loc == nil {
		return fmt.Errorf("%s: no tag line in the front matter", p)
	}
	text = front[:loc[0]] + "tag: " + m.tag + front[loc[1]:] + text[end+3:]
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		return err
	}
	dir := filepath.Join(root, "prov")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	old := filepath.Join(dir, m.n.Tag+".jsonl")
	dst := filepath.Join(dir, m.tag+".jsonl")
	if _, err := os.Stat(old); err == nil {
		return os.Rename(old, dst)
	}
	rec, err := json.Marshal(map[string]any{"date": time.Now().UTC().Format("2006-01-02"), "file": "*", "op": "create", "author": author, "sources": []string{}})
	if err != nil {
		return err
	}
	return appendLines(dst, string(rec)+"\n")
}

func appendLines(path, s string) error {
	b, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if len(b) > 0 && b[len(b)-1] != '\n' {
		s = "\n" + s
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644) //nolint:gosec // the registers and the provenance are public files of the content repo
	if err != nil {
		return err
	}
	if _, err := f.WriteString(s); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func defaultAuthor(root string) string {
	if a := os.Getenv("GITHUB_ACTOR"); a != "" {
		return a
	}
	cmd := exec.Command("git", "-C", root, "config", "user.name")
	if out, err := cmd.Output(); err == nil && strings.TrimSpace(string(out)) != "" {
		return strings.TrimSpace(string(out))
	}
	return "unknown"
}
