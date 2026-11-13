// SPDX-License-Identifier: Apache-2.0

package card

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"sort"
	"strings"
)

// Line is one line of a shard: the card, its bytes as they are in the file, and whether a writer changed it. A line
// that no writer changed is written back byte for byte.
type Line struct {
	Card    *Card
	Raw     []byte
	Changed bool
}

// ReadShard reads the lines of one shard. A missing file is an empty shard.
func ReadShard(path string) ([]*Line, error) {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []*Line
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for n := 1; sc.Scan(); n++ {
		b := bytes.TrimSpace(sc.Bytes())
		if len(b) == 0 {
			continue
		}
		c, err := Parse(b)
		if err != nil {
			return nil, &Error{File: path, Line: n, Err: err}
		}
		out = append(out, &Line{Card: c, Raw: append([]byte(nil), b...)})
	}
	return out, sc.Err()
}

// first and last are the keys that a card line puts at its start and its end; the other keys come between them in
// alphabetical order, as in the example of 04.6.
var (
	first = []string{"tag", "kind", "slug", "owner", "name", "role", "aliases"}
	last  = []string{"same_as", "summary", "harvest", "rules"}
	// factFirst is the order of the keys of a fact object (04.9).
	factFirst = []string{"v", "src", "loc", "as_of"}
)

// Encode writes the fields of a card as one canonical JSON line.
func Encode(fields map[string]json.RawMessage) ([]byte, error) {
	var buf bytes.Buffer
	if err := encodeObject(&buf, fields, first, last); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func encodeObject(buf *bytes.Buffer, m map[string]json.RawMessage, head, tail []string) error {
	keys := make([]string, 0, len(m))
	rank := map[string]int{}
	for i, k := range head {
		rank[k] = i - len(head) - 1000
	}
	for i, k := range tail {
		rank[k] = 1000 + i
	}
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := rank[keys[i]], rank[keys[j]]
		if a != b {
			return a < b
		}
		return keys[i] < keys[j]
	})
	buf.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		kb, _ := marshal(k)
		buf.Write(kb)
		buf.WriteByte(':')
		if err := encodeValue(buf, m[k]); err != nil {
			return fmt.Errorf("%s: %w", k, err)
		}
	}
	buf.WriteByte('}')
	return nil
}

func encodeValue(buf *bytes.Buffer, raw json.RawMessage) error {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return errors.New("an empty value")
	}
	switch raw[0] {
	case '{':
		var m map[string]json.RawMessage
		if err := json.Unmarshal(raw, &m); err != nil {
			return err
		}
		var head []string
		if _, ok := m["v"]; ok {
			head = factFirst
		}
		return encodeObject(buf, m, head, nil)
	case '[':
		var l []json.RawMessage
		if err := json.Unmarshal(raw, &l); err != nil {
			return err
		}
		buf.WriteByte('[')
		for i, v := range l {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := encodeValue(buf, v); err != nil {
				return err
			}
		}
		buf.WriteByte(']')
		return nil
	case '"':
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return err
		}
		b, _ := marshal(s)
		buf.Write(b)
		return nil
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return err
	}
	buf.Write(raw)
	return nil
}

// marshal writes JSON without escaping <, > and &, which a card has in names such as "R&D".
func marshal(v any) ([]byte, error) {
	var buf bytes.Buffer
	e := json.NewEncoder(&buf)
	e.SetEscapeHTML(false)
	if err := e.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// Set puts a value in the fields of the card and marks nothing; the caller marks the line changed.
func (c *Card) Set(key string, v any) error {
	b, err := marshal(v)
	if err != nil {
		return err
	}
	if c.Fields == nil {
		c.Fields = map[string]json.RawMessage{}
	}
	c.Fields[key] = b
	return nil
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify makes a kebab-case slug of at most 60 characters from a name (rule T03): "Llama 3.1-405B" is
// "llama-3-1-405b".
func Slugify(name string) string {
	s := strings.ToLower(name)
	s = strings.NewReplacer("+", " plus ", "#", " sharp ", "&", " and ", "'", "", "’", "").Replace(s)
	s = strings.Trim(nonSlug.ReplaceAllString(s, "-"), "-")
	for len(s) > 60 {
		i := strings.LastIndexByte(s[:60], '-')
		if i <= 0 {
			s = s[:60]
			break
		}
		s = s[:i]
	}
	return strings.Trim(s, "-")
}
