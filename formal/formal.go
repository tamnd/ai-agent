// SPDX-License-Identifier: Apache-2.0

// Package formal builds the Lean package of the content repository and reads what it proves (spec 11).
//
// Build runs lake build in the lean directory, then runs AIExtract (Extract.lean) with lake env lean. AIExtract writes one JSON line for each declaration in the namespace AI to decls.jsonl. Check reads that file and reports each declaration that uses an axiom outside the policy of 11.5, and each name of a lean field that does not exist.
package formal

import (
	"bufio"
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/tamnd/ai-agent/node"
)

//go:embed Extract.lean
var extractSrc []byte

// Allowed are the axioms that a proof can use (spec 11.5). Each other axiom, such as sorryAx or Lean.ofReduceBool, fails the build.
var Allowed = []string{"propext", "Classical.choice", "Quot.sound"}

// Decl is one line of decls.jsonl.
type Decl struct {
	Decl       string   `json:"decl"`
	Kind       string   `json:"kind"`
	Module     string   `json:"module"`
	Type       string   `json:"type"`
	TypeHash   string   `json:"type_hash"`
	Axioms     []string `json:"axioms"`
	Deps       []string `json:"deps"`
	Sorry      bool     `json:"sorry"`
	Computable bool     `json:"computable"`
	Doc        string   `json:"doc,omitempty"`
	Pos        *Pos     `json:"pos,omitempty"`
}

// Pos is the line range of a declaration in its module.
type Pos struct {
	Line    int `json:"line"`
	EndLine int `json:"end_line"`
}

// Disallowed returns the axioms of d that the policy does not permit.
func (d Decl) Disallowed() []string {
	var out []string
	for _, a := range d.Axioms {
		if !slices.Contains(Allowed, a) {
			out = append(out, a)
		}
	}
	return out
}

// Decls maps each full name to its declaration.
type Decls map[string]Decl

// Load reads a decls.jsonl file.
func Load(path string) (Decls, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	out := Decls{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<16), 1<<24)
	for i := 1; sc.Scan(); i++ {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var d Decl
		if err := json.Unmarshal(line, &d); err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, i, err)
		}
		out[d.Decl] = d
	}
	return out, sc.Err()
}

// Options tell Build where the package is and how to run lake.
type Options struct {
	// Dir is the Lean package, the lean directory of the content repository.
	Dir string
	// Out is the directory for decls.jsonl and the copy of Extract.lean, such as build/lean.
	Out string
	// NoBuild skips lake build, when the package is built already.
	NoBuild bool
	// Wrap changes a command before it runs, for example to run it in a container. Nil runs it on the host.
	Wrap func(argv []string) []string
	// Log gets the output of lake.
	Log io.Writer
}

// Build runs lake build and AIExtract, and returns the path of decls.jsonl.
func Build(ctx context.Context, o Options) (string, error) {
	dir, err := filepath.Abs(o.Dir)
	if err != nil {
		return "", err
	}
	out, err := filepath.Abs(o.Out)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(filepath.Join(dir, "lakefile.toml")); err != nil {
		return "", fmt.Errorf("%s is not a Lake package: %w", dir, err)
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return "", err
	}
	if !o.NoBuild {
		if err := o.run(ctx, dir, nil, "lake", "build"); err != nil {
			return "", fmt.Errorf("lake build: %w", err)
		}
	}
	src := filepath.Join(out, "Extract.lean")
	if err := os.WriteFile(src, extractSrc, 0o644); err != nil {
		return "", err
	}
	decls := filepath.Join(out, "decls.jsonl")
	_ = os.Remove(decls)
	if err := o.run(ctx, dir, []string{"AI_DECLS=" + decls}, "lake", "env", "lean", src); err != nil {
		return "", fmt.Errorf("AIExtract: %w", err)
	}
	return decls, nil
}

func (o Options) run(ctx context.Context, dir string, env []string, argv ...string) error {
	if o.Wrap != nil {
		// The container gets the variables as arguments of env, because the engine does not pass the environment of the client.
		if len(env) > 0 {
			argv = append(append([]string{"env"}, env...), argv...)
			env = nil
		}
		argv = o.Wrap(argv)
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	log := o.Log
	if log == nil {
		log = io.Discard
	}
	cmd.Stdout, cmd.Stderr = log, log
	return cmd.Run()
}

// Finding is one problem that Check finds.
type Finding struct {
	// Rule is the rule of 11.13, such as L01 or L03.
	Rule string
	// Where is the declaration, or the node and its field.
	Where string
	Msg   string
	// Node is the slug of the node of the finding, and Decl the declaration, when there is one.
	Node string
	Decl string
	// Severity, when it is not empty, overrides the severity of the rule in Severities. L10 uses it, because a slow module is a warning and a very slow module blocks.
	Severity string
}

// Severities are the severities of the L rules in 13.3: a block rule stops a merge, and a level rule keeps the node below the level that needs it.
var Severities = map[string]string{"L01": "block", "L02": "level", "L03": "block", "L05": "level", "L07": "level", "L09": "block", "L10": "warn"}

func (f Finding) String() string { return f.Rule + " " + f.Where + ": " + f.Msg }

// Sev returns the severity of the finding: block, level or warn.
func (f Finding) Sev() string {
	if f.Severity != "" {
		return f.Severity
	}
	return Severities[f.Rule]
}

// Kinds are the kinds that each key of a lean field must name.
var Kinds = map[string][]string{
	"model":   {"def"},
	"spec":    {"def", "inductive", "structure"},
	"correct": {"theorem"},
	"cost":    {"theorem"},
	"space":   {"theorem"},
}

// Check reports each declaration that uses an axiom outside the policy (rule L03), and each name of a lean field that is not in ds or has the wrong kind (rule L01). The findings are in a stable order.
func Check(c *node.Corpus, ds Decls) []Finding {
	var out []Finding
	names := make([]string, 0, len(ds))
	for n := range ds {
		names = append(names, n)
	}
	slices.Sort(names)
	for _, n := range names {
		d := ds[n]
		if bad := d.Disallowed(); len(bad) > 0 {
			msg := "uses the axiom " + strings.Join(bad, ", ")
			if d.Sorry {
				msg += " (a sorry)"
			}
			out = append(out, Finding{Rule: "L03", Where: d.Decl, Msg: msg, Decl: d.Decl})
		}
	}
	if c == nil {
		return out
	}
	for _, nd := range c.Nodes {
		out = append(out, CheckNode(nd, ds)...)
		out = append(out, CheckClaims(nd, ds)...)
	}
	return out
}

// CheckNode checks the lean field of one node: each name exists with the right kind (L01), the module mirrors the node path (L09), the correctness theorem names the spec and the model (L02), and the model is total and computable (L07).
func CheckNode(n *node.Node, ds Decls) []Finding {
	var out []Finding
	mod := n.Lean["module"]
	for _, k := range slices.Sorted(maps.Keys(n.Lean)) {
		v := n.Lean[k]
		if k == "module" || k == "cslib" || v == "" {
			continue
		}
		where := n.Slug + " lean." + k
		d, ok := ds[v]
		if !ok {
			out = append(out, Finding{Rule: "L01", Where: where, Msg: v + " is not a declaration of the AI package", Node: n.Slug})
			continue
		}
		if want := Kinds[k]; want != nil && !slices.Contains(want, d.Kind) {
			out = append(out, Finding{Rule: "L01", Where: where, Msg: fmt.Sprintf("%s is of the kind %s, want %s", v, d.Kind, strings.Join(want, " or ")), Node: n.Slug})
		}
		if mod != "" && k != "spec" && d.Module != mod {
			out = append(out, Finding{Rule: "L01", Where: where, Msg: fmt.Sprintf("%s is in the module %s, not in %s", v, d.Module, mod), Node: n.Slug})
		}
	}
	if mod == "" {
		return out
	}
	if !Mirrors(mod, n.Rel) {
		out = append(out, Finding{Rule: "L09", Where: n.Slug + " lean.module", Msg: fmt.Sprintf("%s does not mirror the node path; use a module such as %s", mod, MirrorOf(n.Rel)), Node: n.Slug})
	}
	if c, ok := ds[n.Lean["correct"]]; ok {
		for _, k := range []string{"spec", "model"} {
			if name := n.Lean[k]; name != "" && !mentions(c.Type, name) {
				out = append(out, Finding{Rule: "L02", Where: n.Slug + " lean.correct", Msg: fmt.Sprintf("the statement of %s does not mention the %s %s", c.Decl, k, name), Node: n.Slug})
			}
		}
	}
	if m, ok := ds[n.Lean["model"]]; ok && (m.Kind == "def" || m.Kind == "opaque") {
		switch {
		case m.Kind == "opaque":
			out = append(out, Finding{Rule: "L07", Where: n.Slug + " lean.model", Msg: m.Decl + " is opaque, which a partial def gives; the model must be total", Node: n.Slug})
		case !m.Computable:
			out = append(out, Finding{Rule: "L07", Where: n.Slug + " lean.model", Msg: m.Decl + " is not computable, so the oracle cannot run it", Node: n.Slug})
		}
	}
	return out
}

// mentions tells if a statement names a declaration as a whole name, not as a part of a longer one.
func mentions(typ, name string) bool {
	for i := 0; i < len(typ); {
		j := strings.Index(typ[i:], name)
		if j < 0 {
			return false
		}
		j += i
		end := j + len(name)
		if (j == 0 || !isNameChar(typ[j-1])) && (end == len(typ) || !isNameChar(typ[end])) {
			return true
		}
		i = j + 1
	}
	return false
}

func isNameChar(b byte) bool {
	return b == '.' || b == '_' || b == '\'' || b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= 0x80
}

// camel turns a kebab-case path segment into CamelCase: comparison-sorts gives ComparisonSorts.
func camel(s string) string {
	var b strings.Builder
	for _, w := range strings.Split(s, "-") {
		if w != "" {
			b.WriteString(strings.ToUpper(w[:1]) + w[1:])
		}
	}
	return b.String()
}

// MirrorOf returns the module that mirrors the node path in full (spec 11.2): nodes/graphs/shortest-paths/dijkstra gives AI.Graphs.ShortestPaths.Dijkstra.
func MirrorOf(rel string) string {
	parts := strings.Split(strings.TrimPrefix(filepath.ToSlash(rel), "nodes/"), "/")
	for i, p := range parts {
		parts[i] = camel(p)
	}
	return "AI." + strings.Join(parts, ".")
}

// Mirrors tells if a module mirrors the node path (rule L09). The last part is the slug in CamelCase, the first part after AI is the top area, and the parts between are some of the sub-areas, in order. So both AI.Sorting.ComparisonSorts.MergeSort and AI.Sorting.MergeSort mirror nodes/sorting/comparison-sorts/merge-sort.
func Mirrors(mod, rel string) bool {
	want := strings.Split(MirrorOf(rel), ".")
	got := strings.Split(mod, ".")
	if len(got) < 3 || len(want) < 3 || got[0] != "AI" || got[1] != want[1] || got[len(got)-1] != want[len(want)-1] {
		return false
	}
	i := 2
	for _, g := range got[2 : len(got)-1] {
		for i < len(want)-1 && want[i] != g {
			i++
		}
		if i == len(want)-1 {
			return false
		}
		i++
	}
	return true
}

// classOps are the relations of Mathlib's asymptotics, as the pretty printer shows them.
var classOps = []string{"=O[", "=Θ[", "=o["}

// IsClassLemma tells if a type states an asymptotic class, such as `f =O[Filter.atTop] g`.
func IsClassLemma(typ string) bool {
	for _, op := range classOps {
		if strings.Contains(typ, op) {
			return true
		}
	}
	return false
}

var (
	funRE   = regexp.MustCompile(`^fun \(?(\w+)(?: : [^)]*)?\)? => `)
	claimRE = regexp.MustCompile(`^[ΘOΩo]\((.*)\)$`)
	ascRE   = regexp.MustCompile(`\((\w+) : [^()]*\)`)
	dropRE  = regexp.MustCompile(`↑|Real\.|Nat\.|\\|[\s*·()]`)
)

// ClassOf returns the class of a class lemma in a normal form, such as "nlogn" for `f =O[Filter.atTop] fun n => ↑n * Real.log ↑n`. It returns "" when typ is not a class lemma.
func ClassOf(typ string) string {
	for _, op := range classOps {
		i := strings.Index(typ, op)
		if i < 0 {
			continue
		}
		rest := typ[i+len(op):]
		j := strings.Index(rest, "] ")
		if j < 0 {
			return ""
		}
		g := strings.TrimSpace(rest[j+2:])
		g = funRE.ReplaceAllString(g, "")
		return normClass(g)
	}
	return ""
}

// ClaimClass returns the class of a front matter bound in the same normal form, such as "nlogn" for "Θ(n log n)". It returns "" when the bound is not of the form Θ(...), O(...) or Ω(...).
func ClaimClass(bound string) string {
	m := claimRE.FindStringSubmatch(strings.TrimSpace(bound))
	if m == nil {
		return ""
	}
	return normClass(m[1])
}

func normClass(s string) string {
	s = strings.ReplaceAll(s, "lg", "log")
	s = ascRE.ReplaceAllString(s, "$1")
	return dropRE.ReplaceAllString(s, "")
}

// CheckClaims compares the time claims of a node that has lean.cost with the class lemmas of its module (rule L05). A claim with no class lemma of the same class is a finding. Space claims count only when lean.space is set.
func CheckClaims(n *node.Node, ds Decls) []Finding {
	mod := n.Lean["module"]
	if mod == "" || n.Complexity == nil {
		return nil
	}
	have := map[string]bool{}
	for _, d := range ds {
		if d.Module == mod && d.Kind == "theorem" && len(d.Disallowed()) == 0 {
			if c := ClassOf(d.Type); c != "" {
				have[c] = true
			}
		}
	}
	var out []Finding
	check := func(group string, m map[string]string) {
		for _, k := range slices.Sorted(maps.Keys(m)) {
			c := ClaimClass(m[k])
			if c == "" || have[c] {
				continue
			}
			out = append(out, Finding{Rule: "L05", Where: n.Slug + " " + group + "." + k, Msg: fmt.Sprintf("%s has no class lemma in %s", m[k], mod), Node: n.Slug})
		}
	}
	if n.Lean["cost"] != "" {
		check("time", n.Complexity.Time)
	}
	if n.Lean["space"] != "" {
		check("space", n.Complexity.Space)
	}
	return out
}
