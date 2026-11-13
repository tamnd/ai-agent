// SPDX-License-Identifier: Apache-2.0

package card

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// MaxShard is the size at which a shard splits by one more character of the tag (spec 20.3).
const MaxShard = 5 << 20

// ReadKind reads every shard of the directory of one kind.
func ReadKind(dir string) ([]*Line, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	var out []*Line
	for _, p := range paths {
		lines, err := ReadShard(p)
		if err != nil {
			return nil, err
		}
		out = append(out, lines...)
	}
	return out, nil
}

// WriteKind writes the lines of one kind into its shards. Each line goes to the shard of the longest prefix of its
// tag; a shard of more than max bytes splits by one more character, and a shard left with no lines is removed. A kind
// with one shard keeps it in _.jsonl.
func WriteKind(dir string, lines []*Line, max int) error {
	enc := make(map[*Line][]byte, len(lines))
	for _, l := range lines {
		b := l.Raw
		if l.Changed || b == nil {
			var err error
			if b, err = Encode(l.Card.Fields); err != nil {
				return fmt.Errorf("%s: the card %s: %w", dir, l.Card.Slug, err)
			}
			l.Raw, l.Changed = b, false
		}
		enc[l] = b
	}
	old, err := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	if err != nil {
		return err
	}
	var prefixes []string
	for _, p := range old {
		prefixes = append(prefixes, Shard(p))
	}
	shards := place(lines, prefixes)
	for {
		split := ""
		for p, ls := range shards {
			if size(ls, enc) > max && len(ls) > 1 && (split == "" || p < split) {
				split = p
			}
		}
		if split == "" {
			break
		}
		ls := shards[split]
		delete(shards, split)
		depth := len(split) + 1
		if split == "_" {
			depth = 1
		}
		for _, l := range ls {
			k := prefix(l.Card.Tag, depth)
			shards[k] = append(shards[k], l)
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, p := range old {
		if _, ok := shards[Shard(p)]; !ok {
			if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return err
			}
		}
	}
	for p, ls := range shards {
		sort.SliceStable(ls, func(i, j int) bool { return ls[i].Card.Tag < ls[j].Card.Tag })
		var buf bytes.Buffer
		for _, l := range ls {
			buf.Write(enc[l])
			buf.WriteByte('\n')
		}
		path := filepath.Join(dir, p+".jsonl")
		if cur, err := os.ReadFile(path); err == nil && bytes.Equal(cur, buf.Bytes()) {
			continue
		}
		if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// place puts each line in the shard of the longest prefix of its tag. A tag that no prefix shard holds goes to _, or,
// when the kind is split and has no _, to the shard of its prefix at the depth of the shortest shard. A provisional
// tag NEW-... falls in the shard N, NE or NEW of a split kind.
func place(lines []*Line, prefixes []string) map[string][]*Line {
	depth := 0
	for _, p := range prefixes {
		if p != "_" && (depth == 0 || len(p) < depth) {
			depth = len(p)
		}
	}
	under := false
	for _, p := range prefixes {
		under = under || p == "_"
	}
	out := map[string][]*Line{}
	for _, l := range lines {
		best := ""
		for _, p := range prefixes {
			if p != "_" && strings.HasPrefix(l.Card.Tag, p) && len(p) > len(best) {
				best = p
			}
		}
		switch {
		case best != "":
		case under || depth == 0:
			best = "_"
		default:
			best = prefix(l.Card.Tag, depth)
		}
		out[best] = append(out[best], l)
	}
	return out
}

func prefix(tag string, n int) string {
	if len(tag) < n {
		return tag
	}
	return tag[:n]
}

func size(ls []*Line, enc map[*Line][]byte) int {
	n := 0
	for _, l := range ls {
		n += len(enc[l]) + 1
	}
	return n
}
