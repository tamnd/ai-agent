// SPDX-License-Identifier: Apache-2.0

package contract

import (
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/tamnd/ai-agent/internal/jsonl"
)

// Generated is cases/generated.yaml of a problem (spec 09.4). It registers the generators and the size classes of the random inputs.
type Generated struct {
	// Size is the size parameter that the classes measure, such as len(xs).
	Size       string               `yaml:"size"`
	Classes    map[string]SizeClass `yaml:"classes"`
	Generators []GenSpec            `yaml:"generators"`
	// Pre is the file with the function pre(args), which gives None when the arguments meet the preconditions of the contract and a reason when they do not. The shrinker uses it, so that it keeps only inputs that the problem allows. It is optional.
	Pre   string `yaml:"pre,omitempty"`
	Seeds struct {
		PR      []int  `yaml:"pr"`
		Nightly string `yaml:"nightly,omitempty"`
	} `yaml:"seeds"`
}

// SizeClass is one size class: tiny, small, medium or large.
type SizeClass struct {
	Range      [2]int `yaml:"range"`
	Count      int    `yaml:"count"`
	Exhaustive bool   `yaml:"exhaustive,omitempty"`
}

// GenSpec is one generator of generated.yaml. Params maps each parameter of the generator to the list of its values; the generator runs with each combination.
type GenSpec struct {
	Name        string           `yaml:"name"`
	File        string           `yaml:"file"`
	Params      map[string][]any `yaml:"params,omitempty"`
	Adversarial bool             `yaml:"adversarial,omitempty"`
	Note        string           `yaml:"note,omitempty"`
}

// ClassOrder is the order of the size classes, from the smallest.
var ClassOrder = []string{"tiny", "small", "medium", "large"}

// LoadGenerated reads cases/generated.yaml of the problem. A problem without the file gives nil.
func (k *Contract) LoadGenerated() (*Generated, error) {
	b, err := os.ReadFile(filepath.Join(k.Dir, "cases", "generated.yaml"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var g Generated
	dec := yaml.NewDecoder(strings.NewReader(string(b)))
	dec.KnownFields(true)
	if err := dec.Decode(&g); err != nil {
		return nil, fmt.Errorf("cases/generated.yaml: %w", err)
	}
	if err := g.validate(k); err != nil {
		return nil, fmt.Errorf("cases/generated.yaml: %w", err)
	}
	return &g, nil
}

func (g *Generated) validate(k *Contract) error {
	if g.Size == "" {
		return errors.New("size is missing")
	}
	if _, ok := k.Bounds[g.Size]; !ok {
		return fmt.Errorf("the size %s is not a bound of the contract", g.Size)
	}
	for name, c := range g.Classes {
		if !slices.Contains(ClassOrder, name) {
			return fmt.Errorf("the class %q is not one of %s", name, strings.Join(ClassOrder, ", "))
		}
		if c.Range[0] < 0 || c.Range[1] < c.Range[0] {
			return fmt.Errorf("the class %s has the range %v", name, c.Range)
		}
	}
	if len(g.Generators) == 0 {
		return errors.New("there is no generator")
	}
	seen := map[string]bool{}
	for _, s := range g.Generators {
		if s.Name == "" || s.File == "" {
			return errors.New("a generator needs a name and a file")
		}
		if seen[s.Name] {
			return fmt.Errorf("two generators have the name %s", s.Name)
		}
		seen[s.Name] = true
		if _, err := os.Stat(filepath.Join(k.Dir, s.File)); err != nil {
			return fmt.Errorf("the generator %s: %w", s.Name, err)
		}
	}
	if g.Pre != "" {
		if _, err := os.Stat(filepath.Join(k.Dir, g.Pre)); err != nil {
			return fmt.Errorf("pre: %w", err)
		}
	}
	if len(g.Seeds.PR) == 0 {
		return errors.New("seeds.pr is empty")
	}
	return nil
}

// GenRequest is one call of a generator: the generator with one combination of its parameters, a size, a seed, an index and the type of T.
type GenRequest struct {
	Gen    string         `json:"gen"`
	File   string         `json:"file"`
	Params map[string]any `json:"params,omitempty"`
	Class  string         `json:"class"`
	N      int            `json:"n"`
	Seed   int            `json:"seed"`
	Index  int            `json:"index"`
	T      string         `json:"T"`
	// All asks for every input of the size, from the function exhaustive of the generator, for an exhaustive class.
	All bool `json:"all,omitempty"`
}

// combos gives each combination of the parameters, in a fixed order.
func (s GenSpec) combos() []map[string]any {
	names := make([]string, 0, len(s.Params))
	for n := range s.Params {
		names = append(names, n)
	}
	sort.Strings(names)
	out := []map[string]any{{}}
	for _, n := range names {
		var next []map[string]any
		for _, m := range out {
			for _, v := range s.Params[n] {
				c := make(map[string]any, len(m)+1)
				for k, x := range m {
					c[k] = x
				}
				c[n] = v
				next = append(next, c)
			}
		}
		out = next
	}
	return out
}

// Plan gives the generator calls of a run for the classes, with the seeds. Each class splits its count over the generators and their parameter combinations in turn, and draws each size from its range with a random source made from the seed, so the plan is the same on every machine. An exhaustive class asks each generator for every input of each size of the range. The types of T alternate over the types of the contract. An adversarial generator runs only at the small size, as spec 09.5 says for the PR run.
func (g *Generated) Plan(k *Contract, classes []string, seeds []int) []GenRequest {
	type gc struct {
		s GenSpec
		p map[string]any
	}
	var all, plain []gc
	for _, s := range g.Generators {
		for _, p := range s.combos() {
			all = append(all, gc{s, p})
			if !s.Adversarial {
				plain = append(plain, gc{s, p})
			}
		}
	}
	types := k.Types()
	var out []GenRequest
	for _, name := range classes {
		c, ok := g.Classes[name]
		if !ok {
			continue
		}
		pool := plain
		if name == "small" {
			pool = all
		}
		if len(pool) == 0 {
			continue
		}
		for _, seed := range seeds {
			if c.Exhaustive {
				for n := c.Range[0]; n <= c.Range[1]; n++ {
					for i, x := range pool {
						out = append(out, GenRequest{Gen: x.s.Name, File: x.s.File, Params: x.p, Class: name, N: n, Seed: seed, Index: i, T: types[(n+i)%len(types)], All: true})
					}
				}
			}
			h := fnv.New64a()
			fmt.Fprintf(h, "%s/%s/%d", k.Problem, name, seed)
			rng := rand.New(rand.NewPCG(h.Sum64(), uint64(seed))) //nolint:gosec // The plan must be the same on each machine, so the source is seeded on purpose.
			for i := 0; i < c.Count; i++ {
				x := pool[i%len(pool)]
				n := c.Range[0] + rng.IntN(c.Range[1]-c.Range[0]+1)
				out = append(out, GenRequest{Gen: x.s.Name, File: x.s.File, Params: x.p, Class: name, N: n, Seed: seed, Index: i, T: types[i%len(types)]})
			}
		}
	}
	return out
}

// Generator runs the generators of a problem in one Python process.
type Generator struct {
	p *jsonl.Proc
	k *Contract
}

// genHarness loads each generator file once and calls generate(rng, n, T=T, **params) with a random.Random made from a text seed, which gives the same numbers on every machine. With all, it calls exhaustive(n, T=T, **params), or gives no input when the generator has no such function. It runs with python3 -I, so no file of the current directory is imported.
const genHarness = `import importlib.util, json, os, random, sys
mods = {}
def load(f):
    if f not in mods:
        spec = importlib.util.spec_from_file_location("gen_" + str(len(mods)), os.path.join(sys.argv[1], f))
        m = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(m)
        mods[f] = m
    return mods[f]
for line in sys.stdin:
    q = json.loads(line)
    if q.get("op") == "bye":
        break
    try:
        if q.get("op") == "pre":
            r = load(q["file"]).pre(q["args"])
            sys.stdout.write(json.dumps({"reason": r if r else None}) + "\n")
            sys.stdout.flush()
            continue
        m = load(q["file"])
        params = q.get("params") or {}
        if q.get("all"):
            fn = getattr(m, "exhaustive", None)
            ins = list(fn(q["n"], T=q["T"], **params)) if fn else []
        else:
            seed = "%s/%s/%s/%d/%d/%d" % (q["gen"], json.dumps(params, sort_keys=True), q["T"], q["n"], q["seed"], q["index"])
            ins = [m.generate(random.Random(seed), q["n"], T=q["T"], **params)]
        sys.stdout.write(json.dumps({"inputs": ins}) + "\n")
    except Exception as e:
        sys.stdout.write(json.dumps({"error": type(e).__name__ + ": " + str(e)}) + "\n")
    sys.stdout.flush()
`

// StartGenerator starts the generator process of the problem. A non-nil wrap changes the command, so that the generators can run in a sandbox.
func (k *Contract) StartGenerator(wrap func(argv []string) []string) (*Generator, error) {
	dir, err := filepath.Abs(k.Dir)
	if err != nil {
		return nil, err
	}
	argv := []string{"python3", "-I", "-B", "-c", genHarness, dir}
	if wrap != nil {
		argv = wrap(argv)
	}
	p, err := jsonl.Start(k.Dir, argv, nil)
	if err != nil {
		return nil, fmt.Errorf("start the generators: %w", err)
	}
	return &Generator{p: p, k: k}, nil
}

// Make runs one request and gives the cases that it makes, without expect. Each case is checked against the contract, so a bad input is a generator bug (rule M04).
func (g *Generator) Make(q GenRequest) ([]Case, error) {
	line, err := g.p.Call(q, 60*time.Second)
	if err != nil {
		return nil, fmt.Errorf("the generator %s: %w", q.Gen, err)
	}
	var r struct {
		Inputs []map[string]json.RawMessage `json:"inputs"`
		Error  string                       `json:"error"`
	}
	if err := json.Unmarshal(line, &r); err != nil {
		return nil, jsonl.Errorf(line, "the generator %s wrote a line that is not a list of inputs", q.Gen)
	}
	if r.Error != "" {
		return nil, fmt.Errorf("the generator %s (n = %d) raised %s (M04)", q.Gen, q.N, r.Error)
	}
	var out []Case
	for i, in := range r.Inputs {
		c := Case{Name: q.CaseName(i), T: q.T, Args: in, File: "generated"}
		if c.T == "int" {
			c.T = ""
		}
		if err := g.k.CheckArgs(c); err != nil {
			return nil, fmt.Errorf("the generator %s (n = %d) gives an input that does not meet the contract (M04): %w", q.Gen, q.N, err)
		}
		out = append(out, c)
	}
	return out, nil
}

// CaseName is the name of the i-th case of the request, such as small-uniform-k=3-n42-s1-7.
func (q GenRequest) CaseName(i int) string {
	var b strings.Builder
	b.WriteString(q.Class + "-" + q.Gen)
	keys := make([]string, 0, len(q.Params))
	for k := range q.Params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&b, "-%s=%v", k, q.Params[k])
	}
	if q.T != "int" {
		b.WriteString("-" + q.T)
	}
	fmt.Fprintf(&b, "-n%d-s%d-%d", q.N, q.Seed, q.Index)
	if q.All {
		fmt.Fprintf(&b, "-all%d", i)
	}
	return b.String()
}

// Pre checks the arguments with the function pre of the file, and gives the reason why they do not meet the preconditions, or "" when they do.
func (g *Generator) Pre(file string, args map[string]json.RawMessage) (string, error) {
	line, err := g.p.Call(map[string]any{"op": "pre", "file": file, "args": args}, 30*time.Second)
	if err != nil {
		return "", fmt.Errorf("pre: %w", err)
	}
	var r struct {
		Reason any    `json:"reason"`
		Error  string `json:"error"`
	}
	if err := json.Unmarshal(line, &r); err != nil {
		return "", jsonl.Errorf(line, "pre wrote a line that is not a verdict")
	}
	if r.Error != "" {
		return "", fmt.Errorf("pre raised %s", r.Error)
	}
	if r.Reason == nil {
		return "", nil
	}
	return fmt.Sprint(r.Reason), nil
}

// Close stops the generator process.
func (g *Generator) Close() { g.p.Close() }
