// SPDX-License-Identifier: Apache-2.0

package shapes

import (
	"errors"
	"fmt"
	"math/big"
	"strings"
	"unicode"
)

// Expr is a parsed formula of shapes.yaml: integers, names, + - * / ^ and parentheses.
// A name is a config variable, an axis, a model formula, or block.params and block.flops_forward.
type Expr interface {
	eval(env func(string) (*big.Rat, error)) (*big.Rat, error)
	names(add func(string))
}

type num struct{ v *big.Int }
type ref struct{ name string }
type neg struct{ x Expr }
type bin struct {
	op   byte
	l, r Expr
}

func (n num) eval(func(string) (*big.Rat, error)) (*big.Rat, error) {
	return new(big.Rat).SetInt(n.v), nil
}
func (n num) names(func(string)) {}

func (r ref) eval(env func(string) (*big.Rat, error)) (*big.Rat, error) { return env(r.name) }
func (r ref) names(add func(string))                                    { add(r.name) }

func (n neg) eval(env func(string) (*big.Rat, error)) (*big.Rat, error) {
	x, err := n.x.eval(env)
	if err != nil {
		return nil, err
	}
	return new(big.Rat).Neg(x), nil
}
func (n neg) names(add func(string)) { n.x.names(add) }

func (b bin) eval(env func(string) (*big.Rat, error)) (*big.Rat, error) {
	l, err := b.l.eval(env)
	if err != nil {
		return nil, err
	}
	r, err := b.r.eval(env)
	if err != nil {
		return nil, err
	}
	z := new(big.Rat)
	switch b.op {
	case '+':
		return z.Add(l, r), nil
	case '-':
		return z.Sub(l, r), nil
	case '*':
		return z.Mul(l, r), nil
	case '/':
		if r.Sign() == 0 {
			return nil, errors.New("division by zero")
		}
		return z.Quo(l, r), nil
	case '^':
		if !r.IsInt() || r.Sign() < 0 || r.Num().BitLen() > 16 {
			return nil, fmt.Errorf("the exponent %s is not a small whole number", r.RatString())
		}
		k := r.Num().Int64()
		z.SetInt64(1)
		for range k {
			z.Mul(z, l)
		}
		return z, nil
	}
	return nil, fmt.Errorf("unknown operator %c", b.op)
}
func (b bin) names(add func(string)) { b.l.names(add); b.r.names(add) }

// Names lists the names that a formula uses, each once, in the order of first use.
func Names(e Expr) []string {
	var out []string
	seen := map[string]bool{}
	e.names(func(s string) {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	})
	return out
}

// Eval evaluates a formula exactly. env gives the value of each name.
func Eval(e Expr, env func(string) (*big.Rat, error)) (*big.Rat, error) { return e.eval(env) }

// Parse parses a formula.
func Parse(s string) (Expr, error) {
	p := &parser{src: s}
	p.next()
	if p.tok == "" && p.err == nil {
		return nil, errors.New("the formula is empty")
	}
	e := p.sum()
	if p.err == nil && p.tok != "" {
		p.fail("unexpected %q", p.tok)
	}
	if p.err != nil {
		return nil, fmt.Errorf("%q: %w", s, p.err)
	}
	return e, nil
}

type parser struct {
	src string
	pos int
	tok string // the current token; "" at the end
	err error
}

func (p *parser) fail(format string, args ...any) {
	if p.err == nil {
		p.err = fmt.Errorf("at %d: "+format, append([]any{p.pos}, args...)...)
	}
	p.tok = ""
}

func isName(r rune, first bool) bool {
	return r == '_' || unicode.IsLetter(r) || (!first && (unicode.IsDigit(r) || r == '.'))
}

func (p *parser) next() {
	for p.pos < len(p.src) && p.src[p.pos] == ' ' {
		p.pos++
	}
	if p.pos >= len(p.src) {
		p.tok = ""
		return
	}
	start := p.pos
	c := rune(p.src[p.pos])
	switch {
	case strings.ContainsRune("+-*/^()", c):
		p.pos++
	case unicode.IsDigit(c):
		for p.pos < len(p.src) && (unicode.IsDigit(rune(p.src[p.pos])) || p.src[p.pos] == '_') {
			p.pos++
		}
	case isName(c, true):
		for p.pos < len(p.src) && isName(rune(p.src[p.pos]), false) {
			p.pos++
		}
	default:
		p.fail("unexpected %q", string(c))
		return
	}
	p.tok = p.src[start:p.pos]
}

func (p *parser) sum() Expr {
	e := p.product()
	for p.tok == "+" || p.tok == "-" {
		op := p.tok[0]
		p.next()
		e = bin{op, e, p.product()}
	}
	return e
}

func (p *parser) product() Expr {
	e := p.unary()
	for p.tok == "*" || p.tok == "/" {
		op := p.tok[0]
		p.next()
		e = bin{op, e, p.unary()}
	}
	return e
}

func (p *parser) unary() Expr {
	if p.tok == "-" {
		p.next()
		return neg{p.unary()}
	}
	e := p.atom()
	if p.tok == "^" {
		p.next()
		return bin{'^', e, p.unary()}
	}
	return e
}

func (p *parser) atom() Expr {
	t := p.tok
	switch {
	case t == "":
		p.fail("the formula ends too early")
		return num{big.NewInt(0)}
	case t == "(":
		p.next()
		e := p.sum()
		if p.tok != ")" {
			p.fail("a ( has no )")
			return e
		}
		p.next()
		return e
	case unicode.IsDigit(rune(t[0])):
		v, ok := new(big.Int).SetString(strings.ReplaceAll(t, "_", ""), 10)
		if !ok {
			p.fail("bad number %q", t)
			return num{big.NewInt(0)}
		}
		p.next()
		return num{v}
	case isName(rune(t[0]), true):
		if strings.HasSuffix(t, ".") || strings.Contains(t, "..") {
			p.fail("bad name %q", t)
		}
		p.next()
		return ref{t}
	}
	p.fail("unexpected %q", t)
	return num{big.NewInt(0)}
}

// ParseShape parses a tensor shape such as "(B, T, M)" or "(B, H, T, M / H)" into one formula per axis.
func ParseShape(s string) ([]Expr, error) {
	t := strings.TrimSpace(s)
	if !strings.HasPrefix(t, "(") || !strings.HasSuffix(t, ")") {
		return nil, fmt.Errorf("the shape %q is not in parentheses, as in (B, T, M)", s)
	}
	t = strings.TrimSpace(t[1 : len(t)-1])
	if t == "" {
		return nil, nil // a scalar
	}
	var out []Expr
	depth, start := 0, 0
	for i := 0; i <= len(t); i++ {
		if i < len(t) {
			switch t[i] {
			case '(':
				depth++
			case ')':
				depth--
			}
			if t[i] != ',' || depth != 0 {
				continue
			}
		}
		e, err := Parse(strings.TrimSpace(t[start:i]))
		if err != nil {
			return nil, fmt.Errorf("the shape %q: %w", s, err)
		}
		out = append(out, e)
		start = i + 1
	}
	return out, nil
}
