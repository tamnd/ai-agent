// SPDX-License-Identifier: Apache-2.0

package contract

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/tamnd/ai-agent/internal/jsonl"
)

// The Python oracles of spec 09.5, in order of preference. The Lean oracle comes first when it exists; it is built by aik lean build, not here.
var pyOracles = []string{"reference", "brute"}

// OracleFile gives the Python oracle of the problem: the file that the field oracle.reference or oracle.brute names, or else reference.py or brute.py when it exists. It gives "" when the problem has no Python oracle.
func (k *Contract) OracleFile() string {
	if fs := k.OracleFiles(); len(fs) > 0 {
		return fs[0]
	}
	return ""
}

// Oracle runs the Python oracle of a problem in one process (spec 09.5).
type Oracle struct {
	// File is the oracle file, relative to the problem directory.
	File string
	p    *jsonl.Proc
}

// oracleHarness loads the oracle by its path and calls the entry for each line, with the arguments by name. When out is a parameter name, the output is the value of that argument after the call, for the in-place form of a contract. It runs with python3 -I, so no file of the current directory is imported.
const oracleHarness = `import importlib.util, json, sys
spec = importlib.util.spec_from_file_location("oracle", sys.argv[1])
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)
fn = getattr(m, sys.argv[2], None)
for line in sys.stdin:
    q = json.loads(line)
    if q.get("op") == "bye":
        break
    try:
        if fn is None:
            raise AttributeError("the oracle has no function " + sys.argv[2])
        args = q["args"]
        r = fn(**args)
        out = args[q["out"]] if q.get("out") else r
        sys.stdout.write(json.dumps({"output": out}) + "\n")
    except Exception as e:
        sys.stdout.write(json.dumps({"error": type(e).__name__ + ": " + str(e)}) + "\n")
    sys.stdout.flush()
`

// ErrNoOracle is the error of StartOracle for a problem with no Python oracle.
var ErrNoOracle = errors.New("the problem has no reference.py or brute.py")

// OracleFiles gives each Python oracle of the problem that exists, in order of preference: the reference, then the brute force.
func (k *Contract) OracleFiles() []string {
	var out []string
	for _, kind := range pyOracles {
		name, _ := k.Oracle[kind].(string)
		if name == "" {
			name = kind + ".py"
		}
		if _, err := os.Stat(filepath.Join(k.Dir, name)); err == nil {
			out = append(out, name)
		}
	}
	return out
}

// StartOracle starts the Python oracle of the problem. A non-nil wrap changes the command, so that the oracle can run in a sandbox.
func (k *Contract) StartOracle(wrap func(argv []string) []string) (*Oracle, error) {
	file := k.OracleFile()
	if file == "" {
		return nil, ErrNoOracle
	}
	return k.StartOracleFile(file, wrap)
}

// StartOracleFile starts one Python oracle of the problem, from its file relative to the problem directory.
func (k *Contract) StartOracleFile(file string, wrap func(argv []string) []string) (*Oracle, error) {
	path, err := filepath.Abs(filepath.Join(k.Dir, file))
	if err != nil {
		return nil, err
	}
	argv := []string{"python3", "-I", "-B", "-c", oracleHarness, path, k.Entry}
	if wrap != nil {
		argv = wrap(argv)
	}
	p, err := jsonl.Start(k.Dir, argv, nil)
	if err != nil {
		return nil, fmt.Errorf("start the oracle: %w", err)
	}
	return &Oracle{File: file, p: p}, nil
}

// Run gives the output of the oracle for the arguments, as JSON. out is the name of the inout parameter whose value is the output, or "" when the return value is the output.
func (o *Oracle) Run(args map[string]json.RawMessage, out string) (json.RawMessage, error) {
	line, err := o.p.Call(map[string]any{"args": args, "out": out}, 30*time.Second)
	if err != nil {
		return nil, fmt.Errorf("the oracle %s: %w", o.File, err)
	}
	var r struct {
		Output json.RawMessage `json:"output"`
		Error  string          `json:"error"`
	}
	if err := json.Unmarshal(line, &r); err != nil {
		return nil, jsonl.Errorf(line, "the oracle wrote a line that is not an output")
	}
	if r.Error != "" {
		return nil, fmt.Errorf("the oracle %s raised %s", o.File, r.Error)
	}
	if len(r.Output) == 0 {
		r.Output = json.RawMessage("null")
	}
	return r.Output, nil
}

// Close stops the oracle.
func (o *Oracle) Close() { o.p.Close() }

// OutName is the name of the inout parameter whose value is the output, or "" when the return value is the output.
func (k *Contract) OutName() string {
	if i := k.Output(); i >= 0 {
		return k.Params[i].Name
	}
	return ""
}

// OracleArgs gives the arguments of a case in the form that the oracle reads. Python reads integers of any size, so an integer that the case writes as a string (spec 09.2) becomes a JSON number. t is the type of T.
func (k *Contract) OracleArgs(args map[string]json.RawMessage, t string) (map[string]json.RawMessage, error) {
	out := make(map[string]json.RawMessage, len(args))
	for name, raw := range args {
		out[name] = raw
		for _, p := range k.Params {
			if p.Name != name {
				continue
			}
			ty, err := Parse(p.Type)
			if err != nil {
				return nil, err
			}
			v, err := decode(raw)
			if err != nil {
				return nil, fmt.Errorf("the argument %s: %w", name, err)
			}
			b, err := json.Marshal(plainInts(ty.Subst("T", t), v))
			if err != nil {
				return nil, err
			}
			out[name] = b
		}
	}
	return out, nil
}

// plainInts turns each integer that is a string into a JSON number.
func plainInts(t Type, v any) any {
	switch t.Name {
	case "int", "index":
		if s, ok := v.(string); ok {
			if n, ok := integer(s); ok {
				return json.Number(n.String())
			}
		}
	case "option":
		if v != nil {
			return plainInts(t.Args[0], v)
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
				out[i] = plainInts(Type{Name: "list", Args: t.Args}, x)
			case t.Name == "tuple" && i < len(t.Args):
				out[i] = plainInts(t.Args[i], x)
			case t.Name == "tuple":
				out[i] = x
			default:
				out[i] = plainInts(t.Args[0], x)
			}
		}
		return out
	}
	return v
}

// Agree checks the oracle on one case (rule T01). It runs the oracle on the arguments, and gives the verdict of the checker on the oracle output: against expect when the case has it, and for a custom checker with the oracle output as expected. The reason is empty when the oracle agrees.
func (k *Contract) Agree(o *Oracle, c Case, custom *Custom) (reason string, output json.RawMessage, err error) {
	args, err := k.OracleArgs(c.Args, c.Type())
	if err != nil {
		return "", nil, err
	}
	output, err = o.Run(args, k.OutName())
	if err != nil {
		return "", nil, err
	}
	if len(c.Expect) == 0 && len(c.ExpectAny) == 0 {
		if k.Checker.Kind != "custom" {
			return "", output, nil
		}
		c.Expect = output
	}
	reason, err = k.Verdict(c, output, custom)
	return reason, output, err
}
