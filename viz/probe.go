// SPDX-License-Identifier: Apache-2.0

package viz

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/evanw/esbuild/pkg/api"
)

// markerRE matches a line marker of TypeScript, such as "// @line L4" (spec 08.3).
var markerRE = regexp.MustCompile(`//\s*@line\s+([A-Za-z][A-Za-z0-9_]*)\s*$`)

// condRE matches the start of a line whose marker names a condition: an if, an else if or a while.
var condRE = regexp.MustCompile(`^(\}\s*else\s+)?(if|while)\s*\(`)

// elseRE matches the start of a line whose marker names an else block.
var elseRE = regexp.MustCompile(`^\}?\s*else\s*\{`)

// Probe turns each "// @line" marker of a TypeScript reference into a probe call, and compiles the result to a CommonJS script (rule V06). The probe is the global function __probe(id), which returns true.
//
// The probe goes where the line runs:
//
//   - On an if, an else if or a while, the probe is the first part of the condition, so it runs each time the condition is tested, also the last time.
//   - On a for loop with three clauses, the probe is the first part of the test, so it also runs each time the test runs. On a for of loop, the probe is the first statement of the body.
//   - On an else, the probe is the first statement of the block.
//   - On any other line, the probe is a statement before the code of the line.
//
// A line with code that continues the line before it, such as the second line of a long call, cannot carry a marker.
func Probe(src []byte, file string) ([]byte, error) {
	lines := strings.Split(string(src), "\n")
	for i, line := range lines {
		m := markerRE.FindStringSubmatchIndex(line)
		if m == nil {
			continue
		}
		id := line[m[2]:m[3]]
		code := line[:m[0]]
		out, err := probeLine(code, id)
		if err != nil {
			return nil, fmt.Errorf("%s:%d: %w", file, i+1, err)
		}
		lines[i] = out + line[m[0]:]
	}
	res := api.Transform(strings.Join(lines, "\n"), api.TransformOptions{
		Loader:     api.LoaderTS,
		Format:     api.FormatCommonJS,
		Target:     api.ES2020,
		Sourcefile: file,
	})
	if len(res.Errors) > 0 {
		var msgs []string
		for _, m := range res.Errors {
			loc := ""
			if m.Location != nil {
				loc = fmt.Sprintf("%s:%d: ", file, m.Location.Line)
			}
			msgs = append(msgs, loc+m.Text)
		}
		return nil, errors.New(strings.Join(msgs, "; "))
	}
	return res.Code, nil
}

// probeLine puts the probe of id into one line of code, without its marker.
func probeLine(code, id string) (string, error) {
	body := strings.TrimLeft(code, " \t")
	indent := code[:len(code)-len(body)]
	call := fmt.Sprintf("__probe(%q)", id)
	if strings.TrimSpace(body) == "" {
		return "", errors.New("the marker " + id + " is on a line with no code")
	}
	if m := condRE.FindStringIndex(body); m != nil {
		open := m[1] - 1
		end, err := closeParen(body, open)
		if err != nil {
			return "", err
		}
		return indent + body[:open+1] + call + " && (" + body[open+1:end] + ")" + body[end:], nil
	}
	if strings.HasPrefix(body, "for") && strings.HasPrefix(strings.TrimSpace(body[3:]), "(") {
		open := strings.Index(body, "(")
		end, err := closeParen(body, open)
		if err != nil {
			return "", err
		}
		parts := splitTop(body[open+1:end], ';')
		if len(parts) == 3 {
			test := strings.TrimSpace(parts[1])
			if test == "" {
				test = "true"
			}
			return indent + body[:open+1] + parts[0] + "; " + call + " && (" + test + ");" + parts[2] + body[end:], nil
		}
		return intoBlock(indent, body, end, call)
	}
	if elseRE.MatchString(body) {
		return intoBlock(indent, body, 0, call)
	}
	return indent + call + "; " + body, nil
}

// intoBlock puts the probe as the first statement of the block that opens after the index from.
func intoBlock(indent, body string, from int, call string) (string, error) {
	k := strings.Index(body[from:], "{")
	if k < 0 {
		return "", errors.New("the marker is on a loop or an else with no { on the same line")
	}
	k += from
	return indent + body[:k+1] + " " + call + ";" + body[k+1:], nil
}

// closeParen gives the index of the parenthesis that closes the one at open. It skips strings and template literals.
func closeParen(s string, open int) (int, error) {
	depth := 0
	for i := open; i < len(s); i++ {
		switch c := s[i]; c {
		case '"', '\'', '`':
			j := skipString(s, i)
			if j < 0 {
				return 0, errors.New("a string does not end on the line of the marker")
			}
			i = j
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i, nil
			}
		}
	}
	return 0, errors.New("the condition of the marker does not end on its line")
}

// skipString gives the index of the quote that ends the string that starts at i, or -1.
func skipString(s string, i int) int {
	q := s[i]
	for j := i + 1; j < len(s); j++ {
		switch s[j] {
		case '\\':
			j++
		case q:
			return j
		}
	}
	return -1
}

// splitTop splits s at each sep that is not inside parentheses, brackets, braces or strings.
func splitTop(s string, sep byte) []string {
	var parts []string
	depth, start := 0, 0
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '"', '\'', '`':
			if j := skipString(s, i); j >= 0 {
				i = j
			}
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		default:
			if c == sep && depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	return append(parts, s[start:])
}
