// SPDX-License-Identifier: Apache-2.0

package contract

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"slices"
	"strconv"
	"time"

	"github.com/tamnd/ai-agent/internal/jsonl"
)

// Verdict tells if the output of an adapter is right for a case. It returns an empty reason when the output is right.
// The checker of the contract decides (spec 09.3). Before it, the output must have the output type, so an int result that is a float fails.
func (k *Contract) Verdict(c Case, raw json.RawMessage, custom *Custom) (string, error) {
	t := k.OutputType(c.Type())
	v, err := decode(raw)
	if err != nil {
		return "the output is not JSON: " + err.Error(), nil
	}
	v = normalize(t, v)
	if err := t.Check(v); err != nil {
		return "the output does not have the type " + t.String() + ": " + err.Error(), nil
	}
	got := canon(t, v)
	var want any
	if len(c.Expect) > 0 {
		w, err := decode(c.Expect)
		if err != nil {
			return "", err
		}
		want = canon(t, w)
	}
	switch k.Checker.Kind {
	case "exact":
		if !equal(got, want) {
			return "the output is not the expected output", nil
		}
	case "unordered", "multiset":
		g, okG := got.([]any)
		w, okW := want.([]any)
		if !okG || !okW {
			return "", fmt.Errorf("the checker %s needs a list type, not %s", k.Checker.Kind, t)
		}
		if !sameItems(g, w, k.Checker.Kind == "multiset") {
			return "the output does not have the expected items", nil
		}
	case "float":
		if r := closeEnough(got, want, k.Checker.Abs, k.Checker.Rel); r != "" {
			return r, nil
		}
	case "any_of":
		for _, e := range c.ExpectAny {
			w, err := decode(e)
			if err != nil {
				return "", err
			}
			if equal(got, canon(t, w)) {
				return "", nil
			}
		}
		return "the output is none of the outputs of expect_any", nil
	case "custom":
		if custom == nil {
			return "", errors.New("the checker is custom, but no checker process runs")
		}
		return custom.Check(c, got, want)
	}
	return "", nil
}

// normalize makes a negative integer where the type is option<index> into none. It is the convention of the entries that return -1, such as the C, Java and Go entries of binary search.
func normalize(t Type, v any) any {
	if t.Name == "option" && (t.Args[0].Name == "index") {
		if n, ok := integer(v); ok && n.Sign() < 0 {
			return nil
		}
	}
	return v
}

// canon turns a value into a form that compares by value: integers become int64 or a decimal string, and the rest stays.
func canon(t Type, v any) any {
	switch t.Name {
	case "int", "index":
		if n, ok := integer(v); ok {
			if n.IsInt64() {
				return n.Int64()
			}
			return n.String()
		}
	case "float":
		if x, ok := v.(json.Number); ok {
			f, _ := strconv.ParseFloat(x.String(), 64)
			return f
		}
	case "option":
		if v == nil {
			return nil
		}
		return canon(t.Args[0], v)
	case "list", "set", "matrix":
		xs, ok := v.([]any)
		if !ok {
			return v
		}
		elem := t.Args[0]
		if t.Name == "matrix" {
			elem = Type{Name: "list", Args: t.Args}
		}
		out := make([]any, len(xs))
		for i, x := range xs {
			out[i] = canon(elem, x)
		}
		return out
	case "tuple":
		xs, ok := v.([]any)
		if !ok {
			return v
		}
		out := make([]any, len(xs))
		for i, x := range xs {
			if i < len(t.Args) {
				out[i] = canon(t.Args[i], x)
			}
		}
		return out
	}
	return v
}

func key(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func equal(a, b any) bool { return key(a) == key(b) }

func sameItems(got, want []any, multi bool) bool {
	g := make([]string, len(got))
	for i, x := range got {
		g[i] = key(x)
	}
	w := make([]string, len(want))
	for i, x := range want {
		w[i] = key(x)
	}
	slices.Sort(g)
	slices.Sort(w)
	if !multi {
		g, w = slices.Compact(g), slices.Compact(w)
	}
	return slices.Equal(g, w)
}

func closeEnough(got, want any, abs, rel float64) string {
	switch w := want.(type) {
	case float64:
		g, ok := got.(float64)
		if !ok {
			return "the output is not a number"
		}
		if d := math.Abs(g - w); d > abs && d > rel*math.Abs(w) {
			return fmt.Sprintf("the output %g differs from %g by %g", g, w, d)
		}
	case []any:
		g, ok := got.([]any)
		if !ok || len(g) != len(w) {
			return "the output does not have the expected length"
		}
		for i := range w {
			if r := closeEnough(g[i], w[i], abs, rel); r != "" {
				return fmt.Sprintf("item %d: %s", i, r)
			}
		}
	default:
		if !equal(got, want) {
			return "the output is not the expected output"
		}
	}
	return ""
}

// Custom runs checker.py of a problem in one Python process (spec 09.3).
type Custom struct {
	p *jsonl.Proc
}

// harness loads checker.py by its path and calls check for each line. It runs with python3 -I, so no file of the current directory is imported.
const harness = `import importlib.util, json, sys
spec = importlib.util.spec_from_file_location("checker", sys.argv[1])
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)
for line in sys.stdin:
    q = json.loads(line)
    if q.get("op") == "bye":
        break
    try:
        r = m.check(q["args"], q["output"], q.get("expected"))
        if r is not None and (not isinstance(r, str) or r == ""):
            r = "the checker gave a reason that is not a text: " + repr(r)
    except Exception as e:
        r = "the checker raised " + type(e).__name__ + ": " + str(e)
    sys.stdout.write(json.dumps({"reason": r}) + "\n")
    sys.stdout.flush()
`

// StartCustom starts the checker of the contract. A non-nil wrap changes the command, so that the checker can run in a sandbox.
func (k *Contract) StartCustom(wrap func(argv []string) []string) (*Custom, error) {
	path, err := filepath.Abs(filepath.Join(k.Dir, "checker.py"))
	if err != nil {
		return nil, err
	}
	argv := []string{"python3", "-I", "-B", "-c", harness, path}
	if wrap != nil {
		argv = wrap(argv)
	}
	p, err := jsonl.Start(k.Dir, argv, nil)
	if err != nil {
		return nil, fmt.Errorf("start the checker: %w", err)
	}
	return &Custom{p: p}, nil
}

// Check returns the reason of the checker, or an empty reason when the checker accepts the output.
func (c *Custom) Check(cs Case, output, expected any) (string, error) {
	line, err := c.p.Call(map[string]any{"args": cs.Args, "output": output, "expected": expected}, 30*time.Second)
	if err != nil {
		return "", fmt.Errorf("the checker: %w", err)
	}
	var r struct {
		Reason *string `json:"reason"`
	}
	if err := json.Unmarshal(line, &r); err != nil {
		return "", jsonl.Errorf(line, "the checker wrote a line that is not a verdict")
	}
	if r.Reason == nil {
		return "", nil
	}
	return *r.Reason, nil
}

// Close stops the checker.
func (c *Custom) Close() { c.p.Close() }
