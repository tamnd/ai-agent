// SPDX-License-Identifier: Apache-2.0

package site

import (
	"fmt"
	"strings"

	"github.com/tamnd/ai-agent/facts"
	"github.com/tamnd/ai-agent/node"
)

// factRefs replaces each {fact:<item>/<field>} of the prose, outside code, with the value and unit of the fact, a
// footnote to its source and, for a time-bound fact, the date it holds at (spec 06.2 and 06.10). A disputed fact
// prints each value with its source (D05). A reference that does not resolve is left as written; rule D01 blocks it.
func factRefs(c *node.Corpus, body string) string {
	if !strings.Contains(body, "{fact:") {
		return body
	}
	var notes []string
	label := map[string]string{} // the footnote of a source and locator
	note := func(s facts.Source) string {
		text := s.Src
		if s.Loc != "" {
			text += ", " + s.Loc
		}
		if s.AsOf != "" {
			text += " (" + s.AsOf + ")"
		}
		if l, ok := label[text]; ok {
			return "[^" + l + "]"
		}
		l := fmt.Sprintf("fact-%d", len(notes)+1)
		label[text] = l
		notes = append(notes, "[^"+l+"]: "+text)
		return "[^" + l + "]"
	}
	body = outsideMarkdownCode(body, func(s string) string {
		return node.FactRefRE.ReplaceAllStringFunc(s, func(m string) string {
			sm := node.FactRefRE.FindStringSubmatch(m)
			f, err := c.Fact(sm[1], sm[2])
			if err != nil || f.Value == nil || len(f.Sources) == 0 {
				return m
			}
			var out string
			if f.Status == facts.Disputed {
				var parts []string
				for _, s := range f.Sources {
					if s.Value != nil {
						parts = append(parts, facts.Format(s.Value, f.Unit)+note(s))
					}
				}
				out = strings.Join(parts, " or ") + " (the sources differ)"
			} else {
				best := facts.Best(&f)
				if best == nil {
					best = &f.Sources[0]
				}
				out = facts.Format(f.Value, f.Unit) + note(*best)
			}
			if d := facts.AsOf(f); d != "" && facts.TimeBound(f.ID) {
				out += " (as of " + d + ")"
			}
			return out
		})
	})
	if len(notes) == 0 {
		return body
	}
	return strings.TrimRight(body, "\n") + "\n\n" + strings.Join(notes, "\n") + "\n"
}

// outsideMarkdownCode applies fn to the prose of a Markdown body: the text outside fenced blocks and code spans.
func outsideMarkdownCode(body string, fn func(string) string) string {
	lines := strings.Split(body, "\n")
	fence := ""
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
			if fence == "" {
				fence = t[:3]
			} else if strings.HasPrefix(t, fence) {
				fence = ""
			}
			continue
		}
		if fence != "" {
			continue
		}
		parts := strings.Split(l, "`")
		for j := 0; j < len(parts); j += 2 { // the even parts are outside code spans
			parts[j] = fn(parts[j])
		}
		lines[i] = strings.Join(parts, "`")
	}
	return strings.Join(lines, "\n")
}
