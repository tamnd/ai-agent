package markup

import (
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// mathInline is $...$ in the text.
type mathInline struct {
	ast.BaseInline
	value   string
	display bool // $$...$$ in the middle of a line
}

var kindMathInline = ast.NewNodeKind("MathInline")

func (n *mathInline) Kind() ast.NodeKind { return kindMathInline }

func (n *mathInline) Dump(src []byte, level int) {
	ast.DumpHelper(n, src, level, map[string]string{"Value": n.value}, nil)
}

type mathParser struct{}

func (mathParser) Trigger() []byte { return []byte{'$'} }

// Parse reads $...$. The opening $ must not be followed by a space, the closing $ must not follow a space, and the closing $ must not be followed by a digit, so that "$5 and $6" stays text.
func (mathParser) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	line, _ := block.PeekLine()
	if len(line) >= 5 && line[0] == '$' && line[1] == '$' {
		if end := strings.Index(string(line[2:]), "$$"); end > 0 {
			tex := strings.TrimSpace(string(line[2 : 2+end]))
			if tex != "" {
				block.Advance(end + 4)
				return &mathInline{value: tex, display: true}
			}
		}
		return nil
	}
	if len(line) < 3 || line[0] != '$' || line[1] == ' ' || line[1] == '$' {
		return nil
	}
	for i := 1; i < len(line); i++ {
		switch line[i] {
		case '\\':
			i++
		case '$':
			if line[i-1] == ' ' {
				return nil
			}
			if i+1 < len(line) && line[i+1] >= '0' && line[i+1] <= '9' {
				return nil
			}
			block.Advance(i + 1)
			return &mathInline{value: string(line[1:i])}
		case '\n':
			return nil
		}
	}
	return nil
}

func (r *codeRenderer) renderMathInline(w util.BufWriter, src []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		m := n.(*mathInline)
		if m.display {
			r.formula(w, `\displaystyle `+m.value, false)
		} else {
			r.formula(w, m.value, false)
		}
	}
	return ast.WalkSkipChildren, nil
}

type mathExt struct{}

func (mathExt) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(parser.WithInlineParsers(util.Prioritized(mathParser{}, 150)))
}

// displayMath changes display math written as $$...$$ to a ```math block, outside of fenced code (spec 06, display math).
// It accepts the formula on one line, "$$ x $$", and on several lines, from a line that starts with $$ to a line that ends with $$.
// A fenced block can interrupt a paragraph, so the text before and after the formula needs no blank line.
// A $$ with no closing $$ is reported in bad, and the text is left as it is.
func displayMath(src string) (out string, bad []string) {
	lines := strings.Split(src, "\n")
	res := make([]string, 0, len(lines))
	fence := ""
	for i := 0; i < len(lines); i++ {
		l := lines[i]
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
			if fence == "" {
				fence = t[:3]
			} else if strings.HasPrefix(t, fence) {
				fence = ""
			}
			res = append(res, l)
			continue
		}
		if fence != "" || !strings.HasPrefix(t, "$$") {
			res = append(res, l)
			continue
		}
		indent := l[:len(l)-len(strings.TrimLeft(l, " \t"))]
		body := strings.TrimPrefix(t, "$$")
		j := i
		var tex []string
		closed := false
		for {
			if k := strings.Index(body, "$$"); k >= 0 {
				if strings.TrimSpace(body[k+2:]) != "" {
					break // text after the closing $$: leave it to the inline parser
				}
				tex = append(tex, body[:k])
				closed = true
				break
			}
			tex = append(tex, body)
			j++
			if j >= len(lines) || strings.TrimSpace(lines[j]) == "" {
				break
			}
			body = strings.TrimSpace(lines[j])
		}
		if !closed {
			if j == i && strings.Contains(body, "$$") {
				res = append(res, l) // "$$x$$ text": inline display math
				continue
			}
			bad = append(bad, "unclosed $$: "+t)
			res = append(res, l)
			continue
		}
		res = append(res, indent+"```math", indent+strings.TrimSpace(strings.Join(tex, "\n")), indent+"```")
		i = j
	}
	return strings.Join(res, "\n"), bad
}
