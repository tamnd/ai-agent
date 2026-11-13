// SPDX-License-Identifier: Apache-2.0

package shapes

import (
	"fmt"
	"math/big"
	"regexp"
	"strings"
)

// Symbol is the TeX of a name in the formulas of the page.
// A config variable shows its symbol, else the letter of its axis, else its name; a bound axis shows the symbol of its variable when it has one; a block's params is P with the block id; its FLOPs is C with the block id.
func (f *File) Symbol(name string) string {
	f.bind()
	if id, part, ok := strings.Cut(name, "."); ok {
		letter := "P"
		if part == "flops_forward" {
			letter = "C"
		}
		return letter + `_{\mathrm{` + texText(id) + `}}`
	}
	if v := f.Var(name); v != nil {
		if v.Symbol != "" {
			return v.Symbol
		}
		for _, a := range f.Axes {
			if a.Var == name {
				return a.Name
			}
		}
		return texName(name)
	}
	if a := f.Axis(name); a != nil && a.Var != "" {
		if v := f.Var(a.Var); v != nil && v.Symbol != "" {
			return v.Symbol
		}
	}
	if m := f.Formula(name); m != nil && m.Symbol != "" {
		return m.Symbol
	}
	return texName(name)
}

func texText(s string) string { return strings.ReplaceAll(s, "_", `\_`) }

func texName(s string) string {
	if len(s) == 1 {
		return s
	}
	return `\mathrm{` + texText(s) + `}`
}

// TeX renders a name's formula as TeX: the formula of a config variable, of a block's params or flops_forward, or of a model total.
func (f *File) TeX(name string) (string, error) {
	f.bind()
	var e Expr
	if id, part, ok := strings.Cut(name, "."); ok {
		b := f.Block(id)
		if b == nil {
			return "", fmt.Errorf("%s names no block", name)
		}
		switch part {
		case "params":
			e = b.params
		case "flops_forward":
			e = b.flops
		}
		if e == nil && len(b.Of) > 0 {
			var parts []string
			for _, c := range b.Of {
				parts = append(parts, f.Symbol(c+"."+part))
			}
			return strings.Join(parts, " + "), nil
		}
	} else if v := f.Var(name); v != nil {
		e = v.expr
	} else if m := f.Formula(name); m != nil {
		e = m.expr
	}
	if e == nil {
		return "", fmt.Errorf("%s has no formula", name)
	}
	return f.tex(e), nil
}

// tex renders an expression. Products show their numbers first and repeated names as powers, as in 4 M^{2} for 4 * M * M.
func (f *File) tex(e Expr) string {
	switch x := e.(type) {
	case num:
		return x.v.String()
	case ref:
		return f.Symbol(x.name)
	case neg:
		return "-" + f.wrap(x.x, isSum)
	case bin:
		switch x.op {
		case '+':
			return f.tex(x.l) + " + " + f.tex(x.r)
		case '-':
			return f.tex(x.l) + " - " + f.wrap(x.r, isSum)
		case '/':
			return `\frac{` + f.tex(x.l) + "}{" + f.tex(x.r) + "}"
		case '^':
			return f.wrap(x.l, notAtom) + "^{" + f.tex(x.r) + "}"
		case '*':
			return f.product(x)
		}
	}
	return ""
}

func isSum(e Expr) bool {
	switch x := e.(type) {
	case bin:
		return x.op == '+' || x.op == '-'
	case neg:
		return true
	}
	return false
}

func notAtom(e Expr) bool {
	switch e.(type) {
	case num, ref:
		return false
	}
	return true
}

func (f *File) wrap(e Expr, need func(Expr) bool) string {
	if need(e) {
		return `\left(` + f.tex(e) + `\right)`
	}
	return f.tex(e)
}

func factors(e Expr, out []Expr) []Expr {
	if b, ok := e.(bin); ok && b.op == '*' {
		return factors(b.r, factors(b.l, out))
	}
	return append(out, e)
}

func (f *File) product(e bin) string {
	coef := big.NewInt(1)
	var order []string
	count := map[string]int{}
	var rest []string
	for _, x := range factors(e, nil) {
		switch y := x.(type) {
		case num:
			coef.Mul(coef, y.v)
		case ref:
			if count[y.name] == 0 {
				order = append(order, y.name)
			}
			count[y.name]++
		default:
			rest = append(rest, f.wrap(x, isSum))
		}
	}
	var parts []string
	if coef.Cmp(big.NewInt(1)) != 0 || (len(order) == 0 && len(rest) == 0) {
		parts = append(parts, coef.String())
	}
	for _, n := range order {
		s := f.Symbol(n)
		if count[n] > 1 {
			if strings.Contains(s, "_") || strings.HasPrefix(s, `\mathrm`) {
				s = "{" + s + "}"
			}
			s += fmt.Sprintf("^{%d}", count[n])
		}
		parts = append(parts, s)
	}
	parts = append(parts, rest...)
	out := parts[0]
	for i := 1; i < len(parts); i++ {
		if long(parts[i-1]) || long(parts[i]) || isNumber(parts[i]) {
			out += ` \cdot ` + parts[i]
		} else {
			out += " " + parts[i]
		}
	}
	return out
}

// long tells whether a factor is a word, which needs a dot to be read apart from its neighbour.
func long(s string) bool {
	return strings.Contains(s, `\mathrm{`) && !strings.HasPrefix(s, "P_") && !strings.HasPrefix(s, "C_")
}

func isNumber(s string) bool { return s != "" && s[0] >= '0' && s[0] <= '9' }

// RefRE finds a reference of the prose to the shape contract: {shape:params} prints the formula of params, and {shape:params@base} prints its value at the config base (rule X20).
var RefRE = regexp.MustCompile(`\{shape:([A-Za-z_][A-Za-z0-9_.]*)(?:@([a-z0-9][a-z0-9-]*))?\}`)

// Ref resolves a reference: the TeX of the formula of a name when config is empty, else the value of the name at the config, which is "example" or a named config.
func (f *File) Ref(name, config string) (tex string, value *big.Int, err error) {
	if !f.known(name) {
		return "", nil, fmt.Errorf("%s is not a name of %s", name, FileName)
	}
	if config == "" {
		tex, err = f.TeX(name)
		return tex, nil, err
	}
	in := f.Example()
	if config != "example" {
		var ok bool
		if in, ok = f.NamedConfig(config); !ok {
			return "", nil, fmt.Errorf("%s has no config %q; its configs are example%s", FileName, config, f.configNames())
		}
	}
	value, err = f.At(in).Int(name)
	return "", value, err
}

func (f *File) configNames() string {
	var b strings.Builder
	for _, c := range f.Configs {
		b.WriteString(", " + c.Name)
	}
	return b.String()
}

// Group writes a whole number with commas between groups of three digits, as the site prints counts: 63,082,496.
func Group(n *big.Int) string {
	s := n.String()
	sign := ""
	if strings.HasPrefix(s, "-") {
		sign, s = "-", s[1:]
	}
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	return sign + b.String()
}

// ShapeTeX renders a shape such as "(B, S, M)" in the symbols of the formulas, as (B, S, d_{\text{model}}) when M is bound to a variable with that symbol.
func (f *File) ShapeTeX(shape string) (string, error) {
	f.bind()
	dims, err := ParseShape(shape)
	if err != nil {
		return "", err
	}
	parts := make([]string, len(dims))
	for i, d := range dims {
		parts[i] = f.tex(d)
	}
	return "(" + strings.Join(parts, ", ") + ")", nil
}
