// SPDX-License-Identifier: Apache-2.0

// Package mutate makes the mutants of a source file (spec 09.6). It is the generic mutator aik mutate: it reads the text of the file with the comment and string syntax of the language, and it uses the same six operators for every language, so that the scores of two languages can be compared.
package mutate

import (
	"fmt"
	"hash/fnv"
	"math/rand/v2"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// The operators of spec 09.6.
const (
	Relational = "relational"
	Arithmetic = "arithmetic"
	Constant   = "constant"
	Negate     = "negate"
	Delete     = "delete"
	Swap       = "swap"
)

// Operators are the operators, in the order of the report.
var Operators = []string{Relational, Arithmetic, Constant, Negate, Delete, Swap}

// Mutant is one small change of a file. The bytes from Start to End become To.
type Mutant struct {
	File  string `json:"file"`
	Line  int    `json:"line"`
	Op    string `json:"op"`
	From  string `json:"from"`
	To    string `json:"to"`
	Start int    `json:"-"`
	End   int    `json:"-"`
}

// ID names the mutant in a report and in mutants.ignore: the file, the line, the operator and the change.
func (m Mutant) ID() string {
	return fmt.Sprintf("%s:%d: %s %s -> %s", m.File, m.Line, m.Op, m.From, m.To)
}

// Apply gives the source with the mutant.
func (m Mutant) Apply(src string) string { return src[:m.Start] + m.To + src[m.End:] }

// Syntax is what the mutator must know of a language.
type Syntax struct {
	// Exts are the extensions of the source files.
	Exts []string
	// Hash tells if # starts a comment, as in Python. Otherwise // and /* */ start a comment, and # at the start of a line is a preprocessor line.
	Hash bool
	// Empty is the statement that takes the place of a deleted statement, so that a block or a branch does not become empty.
	Empty string
	// Parens tells if the condition of if and while is in parentheses, as in C. Otherwise it ends at the { (Go, Rust) or the : (Python) at the end of the line.
	Parens bool
	// Lifetimes tells if a single quote can start a lifetime, as in Rust, and not only a character.
	Lifetimes bool
	// Floor tells if // is an operator, as in Python.
	Floor bool
}

// Languages are the languages that the mutator reads.
var Languages = map[string]Syntax{
	"python":     {Exts: []string{".py"}, Hash: true, Empty: "pass", Floor: true},
	"go":         {Exts: []string{".go"}},
	"rust":       {Exts: []string{".rs"}, Lifetimes: true},
	"c":          {Exts: []string{".c", ".h"}, Empty: ";", Parens: true},
	"cpp":        {Exts: []string{".cpp", ".cc", ".hpp", ".h"}, Empty: ";", Parens: true},
	"javascript": {Exts: []string{".js", ".mjs"}, Empty: ";", Parens: true},
	"typescript": {Exts: []string{".ts"}, Empty: ";", Parens: true},
	"java":       {Exts: []string{".java"}, Empty: ";", Parens: true},
	"kotlin":     {Exts: []string{".kt"}, Empty: "Unit", Parens: true},
	"csharp":     {Exts: []string{".cs"}, Empty: ";", Parens: true},
}

// The framework versions of spec 08.2 are Python code, so the mutator reads them as Python (08.8).
func init() {
	for _, fw := range []string{"numpy", "pytorch", "jax"} {
		Languages[fw] = Languages["python"]
	}
}

// Source tells if the file is a source file of the language that the mutator reads.
func Source(lang, name string) bool {
	sy, ok := Languages[lang]
	if !ok {
		return false
	}
	ext := path.Ext(name)
	for _, e := range sy.Exts {
		if e == ext {
			return true
		}
	}
	return false
}

// MaxPerFile is the largest number of mutants of one file in a pull request run (spec 09.6).
const MaxPerFile = 150

// Seed is the fixed seed of the sample, so that the score stays the same from run to run.
const Seed = 9

// Sample gives at most max of the mutants, chosen with the fixed seed and the file name, in the order of the file. A max of 0 or less gives all.
func Sample(ms []Mutant, max int) []Mutant {
	if max <= 0 || len(ms) <= max {
		return ms
	}
	h := fnv.New64a()
	if len(ms) > 0 {
		_, _ = h.Write([]byte(ms[0].File))
	}
	rng := rand.New(rand.NewPCG(Seed, h.Sum64())) //nolint:gosec // The sample must be the same on each machine, so the source is seeded on purpose.
	out := append([]Mutant{}, ms...)
	rng.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	out = out[:max]
	sort.SliceStable(out, func(i, j int) bool { return out[i].Start < out[j].Start })
	return out
}

// Mutants gives each mutant of the source file name of the language, in the order of the file.
func Mutants(lang, name, src string) ([]Mutant, error) {
	sy, ok := Languages[lang]
	if !ok {
		return nil, fmt.Errorf("the mutator does not read %s", lang)
	}
	f := &file{name: name, src: src, sy: sy, code: codeMask(src, sy)}
	f.operators()
	f.constants()
	f.conditions()
	f.statements()
	f.swaps()
	sort.SliceStable(f.out, func(i, j int) bool {
		if f.out[i].Start != f.out[j].Start {
			return f.out[i].Start < f.out[j].Start
		}
		return f.out[i].End < f.out[j].End
	})
	return f.out, nil
}

type file struct {
	name string
	src  string
	sy   Syntax
	code []bool
	out  []Mutant
}

func (f *file) add(op string, start, end int, to string) {
	from := f.src[start:end]
	if from == to {
		return
	}
	f.out = append(f.out, Mutant{File: f.name, Line: 1 + strings.Count(f.src[:start], "\n"), Op: op, From: from, To: to, Start: start, End: end})
}

// isCode tells if each byte from i to j is code, not a comment or a string.
func (f *file) isCode(i, j int) bool {
	for k := i; k < j; k++ {
		if !f.code[k] {
			return false
		}
	}
	return true
}

// codeMask marks each byte of src that is code, and not in a comment or a string.
func codeMask(src string, sy Syntax) []bool {
	code := make([]bool, len(src))
	lineStart := true
	for i := 0; i < len(src); {
		c := src[i]
		rest := src[i:]
		switch {
		case sy.Hash && c == '#', !sy.Hash && strings.HasPrefix(rest, "//"), !sy.Hash && c == '#' && lineStart:
			for i < len(src) && src[i] != '\n' {
				i++
			}
			continue
		case !sy.Hash && strings.HasPrefix(rest, "/*"):
			end := strings.Index(rest[2:], "*/")
			if end < 0 {
				return code
			}
			i += end + 4
			continue
		case sy.Hash && (strings.HasPrefix(rest, `"""`) || strings.HasPrefix(rest, "'''")):
			end := strings.Index(rest[3:], rest[:3])
			if end < 0 {
				return code
			}
			i += end + 6
			continue
		case c == '\'' && sy.Lifetimes:
			// A character is 'x' or an escape such as '\n'; anything else is a lifetime, which is code.
			if j := strings.IndexByte(rest[1:], '\''); j >= 1 && (j == 1 || rest[1] == '\\') && j <= 10 {
				i += j + 2
				continue
			}
		case c == '"' || c == '\'' || c == '`':
			i = endString(src, i) + 1
			lineStart = false
			continue
		}
		code[i] = true
		if c == '\n' {
			lineStart = true
		} else if c != ' ' && c != '\t' {
			lineStart = false
		}
		i++
	}
	return code
}

// endString gives the index of the quote that ends the string that starts at i, or the last index of the line when the string does not end there. A template literal can span lines.
func endString(s string, i int) int {
	q := s[i]
	for j := i + 1; j < len(s); j++ {
		switch s[j] {
		case '\\':
			j++
		case q:
			return j
		case '\n':
			if q != '`' {
				return j - 1
			}
		}
	}
	return len(s) - 1
}

const opChars = "<>=!+-*/%&|^"

var relational = map[string]string{"<": "<=", "<=": "<", ">": ">=", ">=": ">", "==": "!=", "!=": "==", "===": "!==", "!==": "==="}

var arithmetic = map[string]string{"+": "-", "-": "+", "*": "/", "/": "*", "%": "*", "+=": "-=", "-=": "+=", "*=": "/=", "/=": "*=", "++": "--", "--": "++"}

func isOperand(c byte) bool {
	return c == '_' || c == ')' || c == ']' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

func isIdent(c byte) bool {
	return c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// operators adds the relational and the arithmetic changes. A relational operator must have a space on each side, so that the < of a generic type is not changed. An arithmetic operator can also be between two operands with no space, as in hi-lo.
func (f *file) operators() {
	s := f.src
	for i := 0; i < len(s); i++ {
		if !strings.ContainsRune(opChars, rune(s[i])) || !f.code[i] {
			continue
		}
		j := i
		for j < len(s) && strings.ContainsRune(opChars, rune(s[j])) && f.code[j] {
			j++
		}
		run := s[i:j]
		if f.sy.Floor && run == "//" {
			run = "/"
		}
		var left, right byte = ' ', ' '
		if i > 0 {
			left = s[i-1]
		}
		if j < len(s) {
			right = s[j]
		}
		spaced := (left == ' ' || left == '\t') && (right == ' ' || right == '\t' || right == '\n')
		tight := isOperand(left) && (isOperand(right) || right == '(') && right != ')' && right != ']'
		switch {
		case relational[run] != "" && spaced:
			f.add(Relational, i, j, relational[run])
		case run == "++" || run == "--":
			if isIdent(left) || left == ')' || left == ']' || isIdent(right) {
				f.add(Arithmetic, i, j, arithmetic[run])
			}
		case arithmetic[run] != "" && (spaced || tight && !strings.HasSuffix(run, "=")):
			to := arithmetic[run]
			if f.sy.Floor && s[i:j] == "//" {
				to = "*"
			}
			f.add(Arithmetic, i, j, to)
		}
		i = j - 1
	}
}

var intSuffix = regexp.MustCompile(`^(?:[uU]?[lL]{0,2}|[lL]{1,2}[uU]?|[iu](?:8|16|32|64|128|size)|n)$`)

// constants adds the change of each integer constant by +1 and -1.
func (f *file) constants() {
	s := f.src
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' || !f.code[i] || i > 0 && (isIdent(s[i-1]) || s[i-1] == '.' && (i <= 1 || s[i-2] != '.')) {
			continue
		}
		j := i
		for j < len(s) && s[j] >= '0' && s[j] <= '9' {
			j++
		}
		k := j
		for k < len(s) && isIdent(s[k]) {
			k++
		}
		float := k+1 < len(s) && s[k] == '.' && s[k+1] >= '0' && s[k+1] <= '9'
		ok := intSuffix.MatchString(s[j:k]) && !float
		if ok && len(s[i:j]) > 1 && s[i] == '0' {
			ok = false
		}
		if ok {
			n, err := strconv.ParseInt(s[i:j], 10, 64)
			if err == nil && n < 1<<62 {
				f.add(Constant, i, j, strconv.FormatInt(n+1, 10))
				if n > 0 {
					f.add(Constant, i, j, strconv.FormatInt(n-1, 10))
				}
			}
		}
		i = k - 1
	}
}

var condKW = regexp.MustCompile(`\b(if|elif|while|for)\b`)

// conditions adds the negation of the condition of each if, else if and while, and of the test of each for loop that has one.
func (f *file) conditions() {
	for _, line := range f.lines() {
		text := f.src[line[0]:line[1]]
		for _, m := range condKW.FindAllStringIndex(text, -1) {
			kw := text[m[0]:m[1]]
			at := line[0] + m[1]
			if !f.isCode(line[0]+m[0], at) {
				continue
			}
			start, end, ok := f.condition(kw, at, line[1])
			if !ok {
				continue
			}
			cond := f.src[start:end]
			if f.sy.Hash {
				f.add(Negate, start, end, "not ("+cond+")")
			} else {
				f.add(Negate, start, end, "!("+cond+")")
			}
		}
	}
}

// condition finds the condition of the keyword kw that ends at at, on the line that ends at eol.
func (f *file) condition(kw string, at, eol int) (int, int, bool) {
	s := f.src
	i := at
	for i < eol && s[i] == ' ' {
		i++
	}
	if f.sy.Hash {
		if kw == "for" || i == at {
			return 0, 0, false
		}
		end := f.lastCode(i, eol, ':')
		if end < 0 {
			return 0, 0, false
		}
		return trim(s, i, end)
	}
	if f.sy.Parens || i < eol && s[i] == '(' && kw != "for" {
		if i >= eol || s[i] != '(' {
			return 0, 0, false
		}
		end := f.closing(i, eol)
		if end < 0 {
			return 0, 0, false
		}
		if kw == "for" {
			parts := f.splitTop(i+1, end, ';')
			if len(parts) != 3 {
				return 0, 0, false
			}
			a, b, ok := trim(s, parts[1][0], parts[1][1])
			return a, b, ok && b > a
		}
		return trim(s, i+1, end)
	}
	// Go and Rust: the condition ends at the { at the end of the line. A Go for loop counts only when it has a test and no clauses.
	end := f.lastCode(i, eol, '{')
	if end < 0 || i == at {
		return 0, 0, false
	}
	a, b, ok := trim(s, i, end)
	if !ok {
		return 0, 0, false
	}
	cond := s[a:b]
	if kw == "for" && (strings.Contains(cond, ";") || strings.Contains(cond, ":=") || strings.Contains(cond, " in ") || strings.HasPrefix(cond, "range ") || strings.Contains(cond, "range ")) {
		return 0, 0, false
	}
	if strings.HasPrefix(cond, "let ") {
		return 0, 0, false
	}
	return a, b, true
}

func trim(s string, a, b int) (int, int, bool) {
	for a < b && (s[a] == ' ' || s[a] == '\t') {
		a++
	}
	for b > a && (s[b-1] == ' ' || s[b-1] == '\t') {
		b--
	}
	return a, b, b > a
}

// lastCode gives the index of the last code byte of the line from i to eol, when it is c.
func (f *file) lastCode(i, eol int, c byte) int {
	for k := eol - 1; k >= i; k-- {
		if !f.code[k] || f.src[k] == ' ' || f.src[k] == '\t' || f.src[k] == '\r' {
			continue
		}
		if f.src[k] == c {
			return k
		}
		return -1
	}
	return -1
}

// closing gives the index of the parenthesis that closes the one at open, on the same line, or -1.
func (f *file) closing(open, eol int) int {
	depth := 0
	for k := open; k < eol; k++ {
		if !f.code[k] {
			continue
		}
		switch f.src[k] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
			if depth == 0 {
				if f.src[k] != ')' {
					return -1
				}
				return k
			}
		}
	}
	return -1
}

// splitTop splits the code from a to b at each sep that is not inside brackets.
func (f *file) splitTop(a, b int, sep byte) [][2]int {
	var parts [][2]int
	depth, start := 0, a
	for k := a; k < b; k++ {
		if !f.code[k] {
			continue
		}
		switch f.src[k] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case sep:
			if depth == 0 {
				parts = append(parts, [2]int{start, k})
				start = k + 1
			}
		}
	}
	return append(parts, [2]int{start, b})
}

// lines gives the start and the end of each line, without the newline.
func (f *file) lines() [][2]int {
	var out [][2]int
	start := 0
	for i := 0; i <= len(f.src); i++ {
		if i == len(f.src) || f.src[i] == '\n' {
			out = append(out, [2]int{start, i})
			start = i + 1
		}
	}
	return out
}

// codeEnd gives the end of the code of the line, before a comment and the spaces before it.
func (f *file) codeEnd(line [2]int) int {
	end := line[0]
	inString := false
	for k := line[0]; k < line[1]; k++ {
		switch {
		case f.code[k]:
			if f.src[k] != ' ' && f.src[k] != '\t' && f.src[k] != '\r' {
				end = k + 1
			}
			inString = false
		case f.src[k] == '"' || f.src[k] == '\'' || f.src[k] == '`' || inString:
			// A string is part of the code of the line.
			end = k + 1
			inString = true
		default:
			return end
		}
	}
	return end
}

var (
	assignRE = regexp.MustCompile(`^(?:[A-Za-z_][\w.]*(?:\[[^\]]*\])*(?:\s*,\s*[A-Za-z_][\w.]*(?:\[[^\]]*\])*)*\s*(?:[-+*/%]?=[^=]|\+\+|--)|(?:\+\+|--)[A-Za-z_])`)
	callRE   = regexp.MustCompile(`^[A-Za-z_][\w.:]*\(.*\);?$`)
	keywords = map[string]bool{"if": true, "elif": true, "else": true, "while": true, "for": true, "switch": true, "match": true, "return": true, "def": true, "func": true, "fn": true, "fun": true, "function": true, "let": true, "var": true, "val": true, "const": true, "assert": true, "panic": true, "throw": true, "raise": true, "yield": true, "await": true, "import": true, "package": true, "using": true, "catch": true, "print": true, "println": true, "super": true, "this": true, "sizeof": true, "typeof": true, "not": true, "and": true, "or": true, "in": true, "is": true, "new": true, "delete": true, "do": true, "case": true}
)

// statements adds the deletion of each statement on one line that assigns a variable or calls a function.
func (f *file) statements() {
	for _, line := range f.lines() {
		end := f.codeEnd(line)
		a, b, ok := trim(f.src, line[0], end)
		if !ok || !f.code[a] {
			continue
		}
		text := f.src[a:b]
		word := text
		if k := strings.IndexFunc(text, notIdent); k >= 0 {
			word = text[:k]
		}
		// A statement at the top level, such as T = TypeVar("T"), defines a name; its deletion only breaks the build.
		if a == line[0] || keywords[word] || !balanced(text) {
			continue
		}
		if !assignRE.MatchString(text) && !callRE.MatchString(text) {
			continue
		}
		if !f.sy.Hash && f.sy.Empty == ";" && !strings.HasSuffix(text, ";") {
			continue
		}
		f.add(Delete, a, b, f.sy.Empty)
	}
}

// balanced tells if each bracket of the text closes on the text.
func balanced(text string) bool {
	depth := 0
	for _, c := range text {
		switch c {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
			if depth < 0 {
				return false
			}
		}
	}
	return depth == 0
}

var callName = regexp.MustCompile(`([A-Za-z_][\w.:]*)\s*\($`)

// commutative are the functions whose arguments can change places with no change of the result.
var commutative = map[string]bool{"min": true, "max": true, "gcd": true, "hypot": true}

// skipSwap are the functions whose arguments cannot change places: a swap gives the same result, or the arguments are a type and a size.
var skipSwap = map[string]bool{"swap": true, "Swap": true, "make": true, "new": true}

// swaps adds the swap of the two arguments of each call with exactly two arguments.
func (f *file) swaps() {
	for _, line := range f.lines() {
		for k := line[0]; k < line[1]; k++ {
			if f.src[k] != '(' || !f.code[k] {
				continue
			}
			m := callName.FindStringSubmatch(f.src[line[0] : k+1])
			if m == nil {
				continue
			}
			name := m[1]
			last := name[strings.LastIndexAny(name, ".:")+1:]
			if keywords[name] || commutative[strings.ToLower(last)] || skipSwap[last] {
				continue
			}
			// A definition is not a call.
			before := strings.TrimSpace(f.src[line[0] : k-len(m[0])+1])
			if w := lastWord(before); keywords[w] || before != "" && isIdent(before[len(before)-1]) && !strings.HasSuffix(before, "return") || strings.HasSuffix(before, ">") {
				continue
			}
			end := f.closing(k, line[1])
			if end < 0 {
				continue
			}
			parts := f.splitTop(k+1, end, ',')
			if len(parts) != 2 {
				continue
			}
			a1, b1, ok1 := trim(f.src, parts[0][0], parts[0][1])
			a2, b2, ok2 := trim(f.src, parts[1][0], parts[1][1])
			if !ok1 || !ok2 {
				continue
			}
			x, y := f.src[a1:b1], f.src[a2:b2]
			if x == y || !f.isCode(a1, b2) || strings.ContainsAny(x+y, "=\"'`") || twoWords(x) || twoWords(y) || strings.Contains(x+y, ": ") {
				continue
			}
			f.add(Swap, a1, b2, y+f.src[b1:a2]+x)
		}
	}
}

// notIdent tells if r cannot be in an identifier.
func notIdent(r rune) bool { return r > 127 || !isIdent(byte(r)) } //nolint:gosec // r is at most 127 here.

func lastWord(s string) string {
	k := strings.LastIndexFunc(s, notIdent)
	return s[k+1:]
}

var twoWordsRE = regexp.MustCompile(`[\w\]>]\s+[\w*&]`)

// twoWords tells if an argument is a declaration such as int lo, not an expression.
func twoWords(s string) bool {
	return twoWordsRE.MatchString(s) && !strings.ContainsAny(s, "+-*/%<>")
}
