// SPDX-License-Identifier: Apache-2.0

package node

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

// KeyOrder is the order of the front matter keys (spec 04.4). Keys that are not in the list go after it, in the order of the file.
var KeyOrder = []string{
	"tag", "kind", "slug", "name", "aliases", "area", "areas", "level", "summary",
	"solves", "implements", "techniques", "uses", "variant_of", "generalizes",
	"model", "complexity", "complexity_in", "properties", "constraints", "history",
	"prereqs", "see_also", "same_as", "lean", "sources", "exercises", "people",
}

var topKeyRE = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)\s*:`)

// FormatNode returns node.md in canonical form: LF line ends, no trailing spaces, the front matter keys in the order of KeyOrder, at most one blank line in a row outside code blocks, and one newline at the end.
// It returns an error when the new front matter does not parse to the same value as the old one, so it never changes what a node says.
func FormatNode(b []byte) ([]byte, error) {
	front, body, _, err := SplitFront(b)
	if err != nil {
		return nil, err
	}
	newFront, err := formatFront(front)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	out.WriteString("---\n")
	out.Write(newFront)
	out.WriteString("---\n")
	if text := formatMarkdown(body); len(text) > 0 {
		out.WriteString("\n")
		out.Write(text)
	}
	return out.Bytes(), nil
}

func formatFront(front []byte) ([]byte, error) {
	lines := strings.Split(strings.TrimRight(trimLines(string(front)), "\n"), "\n")
	type block struct {
		key   string
		lines []string
	}
	var blocks []block
	var pending []string // comments and blank lines that belong to the next key
	for _, l := range lines {
		if m := topKeyRE.FindStringSubmatch(l); m != nil {
			blocks = append(blocks, block{m[1], append(pending, l)})
			pending = nil
			continue
		}
		if len(blocks) == 0 || strings.TrimSpace(l) == "" || strings.HasPrefix(l, "#") {
			pending = append(pending, l)
			continue
		}
		// An indented line continues the last key. Blank lines before it are part of that value.
		last := &blocks[len(blocks)-1]
		last.lines = append(append(last.lines, pending...), l)
		pending = nil
	}
	rank := func(k string) int {
		if i := slices.Index(KeyOrder, k); i >= 0 {
			return i
		}
		return len(KeyOrder)
	}
	slices.SortStableFunc(blocks, func(a, b block) int { return rank(a.key) - rank(b.key) })
	var out strings.Builder
	for _, bl := range blocks {
		ls := bl.lines
		for len(ls) > 0 && strings.TrimSpace(ls[0]) == "" {
			ls = ls[1:]
		}
		for _, l := range ls {
			out.WriteString(l)
			out.WriteByte('\n')
		}
	}
	for _, l := range pending {
		if strings.TrimSpace(l) != "" {
			out.WriteString(l)
			out.WriteByte('\n')
		}
	}
	var before, after any
	if err := yaml.Unmarshal(front, &before); err != nil {
		return nil, fmt.Errorf("the front matter does not parse: %w", err)
	}
	if err := yaml.Unmarshal([]byte(out.String()), &after); err != nil || !reflect.DeepEqual(before, after) {
		return nil, errors.New("the front matter has a form that aik fmt cannot reorder safely; put each top-level key at the start of a line")
	}
	return []byte(out.String()), nil
}

// formatMarkdown removes trailing spaces, keeps at most one blank line in a row outside code blocks, and ends the text with one newline.
func formatMarkdown(b []byte) []byte {
	lines := strings.Split(trimLines(string(b)), "\n")
	var out []string
	fence := ""
	blank := 0
	for _, l := range lines {
		t := strings.TrimSpace(l)
		if fence == "" && (strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~")) {
			fence = t[:3]
		} else if fence != "" && strings.HasPrefix(t, fence) {
			fence = ""
		}
		if t == "" && fence == "" {
			blank++
			if blank > 1 || len(out) == 0 {
				continue
			}
		} else {
			blank = 0
		}
		out = append(out, l)
	}
	for len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
		out = out[:len(out)-1]
	}
	if len(out) == 0 {
		return nil
	}
	return []byte(strings.Join(out, "\n") + "\n")
}

// FormatText returns a text file with LF line ends, no trailing spaces and one newline at the end. aik fmt uses it for YAML, registers and proofs.
func FormatText(b []byte) []byte {
	s := strings.TrimRight(trimLines(string(b)), "\n")
	if s == "" {
		return nil
	}
	return []byte(s + "\n")
}

func trimLines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " \t")
	}
	return strings.Join(lines, "\n")
}
