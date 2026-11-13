// SPDX-License-Identifier: Apache-2.0

package contract

import (
	"encoding/json"
	"math/big"
	"sort"
)

// Shrink makes a failing case smaller (spec 09.11). It tries to remove parts of each list, then items one at a time, then to make each integer nearer to 0 and each string shorter, and keeps each change for which fails still gives true. It stops after budget calls of fails. It gives the smallest case that it found and the number of calls.
//
// fails must give false for an input that does not meet the contract, so the shrinker never keeps an input that the problem does not allow.
func Shrink(c Case, fails func(Case) (bool, error), budget int) (Case, int, error) {
	args, err := decodeArgs(c.Args)
	if err != nil {
		return c, 0, err
	}
	names := make([]string, 0, len(args))
	for n := range args {
		names = append(names, n)
	}
	sort.Strings(names)
	calls := 0
	try := func(name string, v any) (bool, error) {
		if calls >= budget {
			return false, nil
		}
		calls++
		next := make(map[string]any, len(args))
		for k, x := range args {
			next[k] = x
		}
		next[name] = v
		cand := c
		cand.Args, err = encodeArgs(next)
		if err != nil {
			return false, err
		}
		cand.Expect, cand.ExpectAny = nil, nil
		ok, err := fails(cand)
		if err != nil || !ok {
			return false, err
		}
		args = next
		return true, nil
	}
	for progress := true; progress && calls < budget; {
		progress = false
		for _, name := range names {
			for _, v := range smaller(args[name]) {
				ok, err := try(name, v)
				if err != nil {
					return c, calls, err
				}
				if ok {
					progress = true
					break
				}
			}
		}
	}
	out := c
	if out.Args, err = encodeArgs(args); err != nil {
		return c, calls, err
	}
	out.Expect, out.ExpectAny = nil, nil
	return out, calls, nil
}

// smaller gives the candidates that are smaller than v, the larger steps first: for a list, the list without each half, quarter and so on down to one item, then the list with one item made smaller; for an integer, 0, then half of it, then one step nearer to 0; for a string, the string without each half down to one character.
func smaller(v any) []any {
	switch x := v.(type) {
	case []any:
		var out []any
		for size := len(x) / 2; size >= 1; size /= 2 {
			for at := 0; at+size <= len(x); at += size {
				out = append(out, append(append([]any{}, x[:at]...), x[at+size:]...))
			}
		}
		if len(x) == 1 {
			out = append(out, []any{})
		}
		for i, item := range x {
			for _, s := range smaller(item) {
				y := append([]any{}, x...)
				y[i] = s
				out = append(out, y)
			}
		}
		return out
	case json.Number, string:
		if n, ok := integer(x); ok {
			if n.Sign() == 0 {
				return nil
			}
			half := new(big.Int).Quo(n, big.NewInt(2))
			step := new(big.Int).Sub(n, big.NewInt(int64(n.Sign())))
			out := []any{json.Number("0")}
			for _, m := range []*big.Int{half, step} {
				if m.Sign() != 0 && m.Cmp(n) != 0 {
					out = append(out, intValue(m))
				}
			}
			return out
		}
		s, ok := x.(string)
		if !ok || s == "" {
			return nil
		}
		r := []rune(s)
		var out []any
		for size := len(r) / 2; size >= 1; size /= 2 {
			for at := 0; at+size <= len(r); at += size {
				out = append(out, string(append(append([]rune{}, r[:at]...), r[at+size:]...)))
			}
		}
		if len(r) == 1 {
			out = append(out, "")
		}
		return out
	}
	return nil
}

// intValue writes an integer as a JSON number when it is in the safe range of JSON readers, and as a string when it is not (spec 09.2).
func intValue(n *big.Int) any {
	if n.IsInt64() && n.Int64() >= -(1<<53) && n.Int64() <= 1<<53 {
		return json.Number(n.String())
	}
	return n.String()
}

func decodeArgs(args map[string]json.RawMessage) (map[string]any, error) {
	out := make(map[string]any, len(args))
	for k, raw := range args {
		v, err := decode(raw)
		if err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, nil
}

func encodeArgs(args map[string]any) (map[string]json.RawMessage, error) {
	out := make(map[string]json.RawMessage, len(args))
	for k, v := range args {
		b, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		out[k] = b
	}
	return out, nil
}

// SafeInts writes each integer of the value outside the safe range of JSON readers as a string, as a case file does (spec 09.2). t is the type of the value.
func SafeInts(t Type, raw json.RawMessage) (json.RawMessage, error) {
	v, err := decode(raw)
	if err != nil {
		return nil, err
	}
	return json.Marshal(safeInts(t, v))
}

func safeInts(t Type, v any) any {
	switch t.Name {
	case "int", "index":
		if n, ok := integer(v); ok {
			return intValue(n)
		}
	case "option":
		if v != nil {
			return safeInts(t.Args[0], v)
		}
	case "list", "set", "matrix", "tuple":
		xs, ok := v.([]any)
		if !ok {
			return v
		}
		out := make([]any, len(xs))
		for i, x := range xs {
			switch {
			case t.Name == "matrix":
				out[i] = safeInts(Type{Name: "list", Args: t.Args}, x)
			case t.Name == "tuple" && i < len(t.Args):
				out[i] = safeInts(t.Args[i], x)
			case t.Name == "tuple":
				out[i] = x
			default:
				out[i] = safeInts(t.Args[0], x)
			}
		}
		return out
	}
	return v
}

// SafeCase writes the integers of the arguments and of expect of the case as a case file does.
func (k *Contract) SafeCase(c Case) (Case, error) {
	args := make(map[string]json.RawMessage, len(c.Args))
	for _, p := range k.Params {
		raw, ok := c.Args[p.Name]
		if !ok {
			continue
		}
		ty, err := Parse(p.Type)
		if err != nil {
			return c, err
		}
		if args[p.Name], err = SafeInts(ty.Subst("T", c.Type()), raw); err != nil {
			return c, err
		}
	}
	c.Args = args
	if len(c.Expect) > 0 {
		e, err := SafeInts(k.OutputType(c.Type()), c.Expect)
		if err != nil {
			return c, err
		}
		c.Expect = e
	}
	return c, nil
}
