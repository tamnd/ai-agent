// SPDX-License-Identifier: Apache-2.0

package contract

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
)

// Type is a type of the neutral type language (spec 08.3.3), such as list<int> or option<index>.
type Type struct {
	Name string
	Args []Type
}

// arity is the number of type arguments of each type.
var arity = map[string]int{
	"int": 0, "index": 0, "bool": 0, "float": 0, "string": 0, "unit": 0, "graph": 0, "wgraph": 0,
	"list": 1, "option": 1, "set": 1, "matrix": 1, "map": 2, "tuple": -1,
}

// Parse reads a type. A single capital letter, such as T, is a type variable of a generic contract.
func Parse(s string) (Type, error) {
	p := &typeParser{s: strings.ReplaceAll(s, " ", "")}
	t, err := p.parse()
	if err != nil {
		return Type{}, err
	}
	if p.i != len(p.s) {
		return Type{}, fmt.Errorf("the type %q has extra text at %q", s, p.s[p.i:])
	}
	return t, nil
}

type typeParser struct {
	s string
	i int
}

func (p *typeParser) parse() (Type, error) {
	start := p.i
	for p.i < len(p.s) && (p.s[p.i] >= 'a' && p.s[p.i] <= 'z' || p.s[p.i] >= 'A' && p.s[p.i] <= 'Z') {
		p.i++
	}
	t := Type{Name: p.s[start:p.i]}
	if t.Name == "" {
		return t, fmt.Errorf("no type name at %q", p.s[start:])
	}
	if p.i < len(p.s) && p.s[p.i] == '<' {
		p.i++
		for {
			a, err := p.parse()
			if err != nil {
				return t, err
			}
			t.Args = append(t.Args, a)
			if p.i < len(p.s) && p.s[p.i] == ',' {
				p.i++
				continue
			}
			if p.i < len(p.s) && p.s[p.i] == '>' {
				p.i++
				break
			}
			return t, fmt.Errorf("the type arguments of %s are not closed", t.Name)
		}
	}
	if t.IsVar() {
		if len(t.Args) > 0 {
			return t, fmt.Errorf("the type variable %s has arguments", t.Name)
		}
		return t, nil
	}
	n, ok := arity[t.Name]
	if !ok {
		return t, fmt.Errorf("%q is not a type of the type language (08.3.3)", t.Name)
	}
	if n >= 0 && len(t.Args) != n || n < 0 && len(t.Args) == 0 {
		return t, fmt.Errorf("%s has %d type arguments", t.Name, len(t.Args))
	}
	return t, nil
}

// IsVar tells if the type is a type variable, such as T.
func (t Type) IsVar() bool { return len(t.Name) == 1 && t.Name[0] >= 'A' && t.Name[0] <= 'Z' }

// HasVar tells if the type has a type variable in it.
func (t Type) HasVar() bool {
	if t.IsVar() {
		return true
	}
	for _, a := range t.Args {
		if a.HasVar() {
			return true
		}
	}
	return false
}

// Subst replaces the type variable v with the type named by.
func (t Type) Subst(v, by string) Type {
	if t.Name == v {
		return Type{Name: by}
	}
	out := Type{Name: t.Name}
	for _, a := range t.Args {
		out.Args = append(out.Args, a.Subst(v, by))
	}
	return out
}

func (t Type) String() string {
	if len(t.Args) == 0 {
		return t.Name
	}
	parts := make([]string, len(t.Args))
	for i, a := range t.Args {
		parts[i] = a.String()
	}
	return t.Name + "<" + strings.Join(parts, ",") + ">"
}

var (
	minInt = big.NewInt(0).Lsh(big.NewInt(-1), 63)
	maxInt = big.NewInt(0).Sub(big.NewInt(0).Lsh(big.NewInt(1), 63), big.NewInt(1))
)

// integer reads an integer value. Integers outside the safe range of JSON readers are strings (spec 09.2).
func integer(v any) (*big.Int, bool) {
	var s string
	switch x := v.(type) {
	case json.Number:
		s = x.String()
	case string:
		s = x
	default:
		return nil, false
	}
	n, ok := new(big.Int).SetString(s, 10)
	return n, ok
}

// Check tells if the decoded JSON value v has the type t. It also checks the kind of each value, so a float where the type is int fails.
func (t Type) Check(v any) error {
	switch t.Name {
	case "int", "index":
		n, ok := integer(v)
		if !ok {
			return fmt.Errorf("%s is not an integer", show(v))
		}
		if n.Cmp(minInt) < 0 || n.Cmp(maxInt) > 0 || t.Name == "index" && n.Sign() < 0 {
			return fmt.Errorf("%s is out of the range of %s", show(v), t.Name)
		}
	case "bool":
		if _, ok := v.(bool); !ok {
			return fmt.Errorf("%s is not a bool", show(v))
		}
	case "float":
		if _, ok := v.(json.Number); !ok {
			return fmt.Errorf("%s is not a number", show(v))
		}
	case "string":
		if _, ok := v.(string); !ok {
			return fmt.Errorf("%s is not a string", show(v))
		}
	case "unit":
		if v != nil {
			return fmt.Errorf("%s is not null", show(v))
		}
	case "option":
		if v == nil {
			return nil
		}
		return t.Args[0].Check(v)
	case "list", "set", "matrix":
		xs, ok := v.([]any)
		if !ok {
			return fmt.Errorf("%s is not a list", show(v))
		}
		elem := t.Args[0]
		if t.Name == "matrix" {
			elem = Type{Name: "list", Args: t.Args}
		}
		for i, x := range xs {
			if err := elem.Check(x); err != nil {
				return fmt.Errorf("item %d: %w", i, err)
			}
		}
	case "tuple":
		xs, ok := v.([]any)
		if !ok || len(xs) != len(t.Args) {
			return fmt.Errorf("%s is not a tuple of %d items", show(v), len(t.Args))
		}
		for i, x := range xs {
			if err := t.Args[i].Check(x); err != nil {
				return fmt.Errorf("item %d: %w", i, err)
			}
		}
	case "map":
		m, ok := v.(map[string]any)
		if !ok {
			return fmt.Errorf("%s is not a map", show(v))
		}
		for k, x := range m {
			if err := t.Args[1].Check(x); err != nil {
				return fmt.Errorf("key %s: %w", k, err)
			}
		}
	case "graph", "wgraph":
		if _, ok := v.([]any); !ok {
			return fmt.Errorf("%s is not an adjacency list", show(v))
		}
	default:
		return fmt.Errorf("the type %s has no value check", t)
	}
	return nil
}

// show returns v as short JSON text for a message.
func show(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return Short(string(b))
}

// Short cuts a long JSON text for a report.
func Short(s string) string {
	if len(s) > 120 {
		return s[:100] + "…" + s[len(s)-12:]
	}
	return s
}
