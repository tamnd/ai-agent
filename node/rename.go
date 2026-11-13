// SPDX-License-Identifier: Apache-2.0

package node

import (
	"regexp"
	"strings"
)

// EdgeKeys are the front matter keys whose values name other nodes by slug (spec 04.6).
var EdgeKeys = []string{
	"solves", "implements", "techniques", "uses", "variant_of", "generalizes",
	"model", "complexity_in", "prereqs", "see_also", "exercises", "related_to", "applies_to",
}

var (
	slugLineRE = regexp.MustCompile(`(?m)^slug:[ \t]*\S+[ \t]*$`)
	areaLineRE = regexp.MustCompile(`(?m)^area:[ \t]*\S+[ \t]*$`)
)

// RenameRefs changes each reference to the slug old into new: the values of the edge keys in the front matter, and the links "](/old/" and "](/old#" in the text. It changes nothing else, so a word in a summary or a URL in a source stays as it is.
func RenameRefs(b []byte, old, new string) []byte {
	text := string(b)
	front, body, ok := cut(text)
	if !ok {
		return []byte(renameLinks(text, old, new))
	}
	return []byte(renameFront(front, old, new) + renameLinks(body, old, new))
}

// SetSlugArea sets the slug line and, when area is not empty, the area line of the front matter.
func SetSlugArea(b []byte, slug, area string) []byte {
	text := string(b)
	front, body, ok := cut(text)
	if !ok {
		return b
	}
	front = slugLineRE.ReplaceAllLiteralString(front, "slug: "+slug)
	if area != "" {
		front = areaLineRE.ReplaceAllLiteralString(front, "area: "+area)
	}
	return []byte(front + body)
}

// cut splits node.md after the line that closes the front matter.
func cut(text string) (front, body string, ok bool) {
	if !strings.HasPrefix(text, "---") {
		return "", text, false
	}
	end := strings.Index(text[3:], "\n---")
	if end < 0 {
		return "", text, false
	}
	end += 3 + len("\n---")
	if nl := strings.IndexByte(text[end:], '\n'); nl >= 0 {
		end += nl + 1
	} else {
		end = len(text)
	}
	return text[:end], text[end:], true
}

func renameFront(front, old, new string) string {
	lines := strings.SplitAfter(front, "\n")
	edge := false
	for i, l := range lines {
		if m := topKeyRE.FindStringSubmatch(l); m != nil {
			edge = isEdgeKey(m[1])
			if edge {
				// Keep the key itself; only the value names nodes.
				k := len(m[0])
				lines[i] = l[:k] + replaceWord(l[k:], old, new)
			}
			continue
		}
		if edge && (strings.HasPrefix(l, " ") || strings.HasPrefix(l, "\t")) {
			lines[i] = replaceWord(l, old, new)
		}
	}
	return strings.Join(lines, "")
}

func isEdgeKey(k string) bool {
	for _, e := range EdgeKeys {
		if e == k {
			return true
		}
	}
	return false
}

// replaceWord replaces old with new where old is a whole slug: the bytes on each side are not letters, digits or hyphens.
func replaceWord(s, old, new string) string {
	var out strings.Builder
	for {
		i := strings.Index(s, old)
		if i < 0 {
			out.WriteString(s)
			return out.String()
		}
		j := i + len(old)
		if (i == 0 || !slugByte(s[i-1])) && (j == len(s) || !slugByte(s[j])) {
			out.WriteString(s[:i])
			out.WriteString(new)
		} else {
			out.WriteString(s[:j])
		}
		s = s[j:]
	}
}

func slugByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-'
}

func renameLinks(s, old, new string) string {
	for _, end := range []string{"/", "#", ")"} {
		s = strings.ReplaceAll(s, "](/"+old+end, "](/"+new+end)
	}
	return s
}
