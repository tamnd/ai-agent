package check

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode"
)

// ParseClaim checks one complexity string in the small grammar of spec 04.4.
//
// A claim is a bound, Θ(e), O(e), Ω(e), o(e) or ω(e), or an exact expression e. Words such as "amortized", "expected" and "w.h.p." may follow. An expression uses numbers, variables, the functions log, lg, ln, sqrt, min, max, the operators + − - * · × / ^, brackets ( ) ⌊ ⌋ ⌈ ⌉ and |x|.
func ParseClaim(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return errors.New("the claim is empty")
	}
	for _, w := range []string{"amortized", "expected", "w.h.p.", "worst case"} {
		s = strings.TrimSpace(strings.TrimSuffix(s, w))
	}
	for _, b := range []string{"Θ", "O", "Ω", "o", "ω"} {
		if strings.HasPrefix(s, b+"(") {
			if !strings.HasSuffix(s, ")") {
				return fmt.Errorf("the bound %s( has no closing bracket at the end", b)
			}
			return expr(s[len(b)+1 : len(s)-1])
		}
	}
	return expr(s)
}

var funcs = []string{"log", "lg", "ln", "sqrt", "min", "max", "exp", "poly", "polylog", "α", "log*", "lg*"}

func expr(s string) error {
	if strings.TrimSpace(s) == "" {
		return errors.New("the expression is empty")
	}
	var stack []rune
	pair := map[rune]rune{')': '(', '⌋': '⌊', '⌉': '⌈'}
	bars := 0
	for _, w := range words(s) {
		if !slices.Contains(funcs, w) && !isVar(w) {
			return fmt.Errorf("the word %q is not a variable or a known function", w)
		}
	}
	for _, c := range s {
		switch c {
		case '(', '⌊', '⌈':
			stack = append(stack, c)
		case ')', '⌋', '⌉':
			if len(stack) == 0 || stack[len(stack)-1] != pair[c] {
				return fmt.Errorf("the bracket %c does not match", c)
			}
			stack = stack[:len(stack)-1]
		case '|':
			bars++
		default:
			if unicode.IsLetter(c) || unicode.IsDigit(c) || unicode.IsSpace(c) || strings.ContainsRune("+−-*·×/^,.!'_²³√∞αε", c) {
				continue
			}
			return fmt.Errorf("the character %q is not in the grammar", c)
		}
	}
	if len(stack) > 0 {
		return fmt.Errorf("the bracket %c is not closed", stack[len(stack)-1])
	}
	if bars%2 != 0 {
		return errors.New("a | has no partner")
	}
	return nil
}

// words returns the runs of ASCII letters of s.
func words(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool { return r > unicode.MaxASCII || !unicode.IsLetter(r) })
}

// isVar tells if w is a variable: one letter, or one letter and digits.
func isVar(w string) bool {
	return len(w) == 1
}

// Vars returns the variables of a claim, once each, in order.
func Vars(s string) []string {
	var out []string
	for _, w := range words(s) {
		if isVar(w) && !slices.Contains(out, w) && w != "O" && w != "o" {
			out = append(out, w)
		}
	}
	return out
}
