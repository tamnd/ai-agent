// SPDX-License-Identifier: Apache-2.0

// Package contract reads the contract and the shared cases of a problem node, and decides whether an output is right (spec 09.1 to 09.3).
package contract

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

// CaseFiles are the case files of a problem, in the order that the runner uses (spec 09.2).
var CaseFiles = []string{"basic", "edge", "regress"}

// Param is one parameter of the entry.
type Param struct {
	Name string `yaml:"name" json:"name"`
	Type string `yaml:"type" json:"type"`
	// Mode is "in" or "inout". An inout parameter is changed by the call, and its value after the call is the output when the entry returns unit.
	Mode string `yaml:"mode,omitempty" json:"mode,omitempty"`
	Pre  string `yaml:"pre,omitempty" json:"pre,omitempty"`
}

// Return is the return value of the entry.
type Return struct {
	Type string `yaml:"type" json:"type"`
	Post string `yaml:"post,omitempty" json:"post,omitempty"`
}

// Checker is the checker kind of a contract. In YAML it is a word, such as exact, or a mapping with the tolerances of the float kind.
type Checker struct {
	Kind string  `yaml:"kind" json:"kind"`
	Abs  float64 `yaml:"abs,omitempty" json:"abs,omitempty"`
	Rel  float64 `yaml:"rel,omitempty" json:"rel,omitempty"`
}

// UnmarshalYAML accepts a word or a mapping.
func (c *Checker) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode {
		c.Kind = n.Value
		return nil
	}
	type plain Checker
	var p plain
	if err := n.Decode(&p); err != nil {
		return err
	}
	*c = Checker(p)
	return nil
}

// Checkers are the checker kinds of spec 09.3.
var Checkers = []string{"exact", "unordered", "multiset", "float", "any_of", "custom"}

// Contract is contract.yaml of a problem node.
type Contract struct {
	Problem     string            `yaml:"problem" json:"problem"`
	Entry       string            `yaml:"entry" json:"entry"`
	Params      []Param           `yaml:"params" json:"params"`
	Returns     Return            `yaml:"returns" json:"returns"`
	Generic     map[string]string `yaml:"generic,omitempty" json:"generic,omitempty"`
	Bounds      map[string]any    `yaml:"bounds,omitempty" json:"bounds,omitempty"`
	Checker     Checker           `yaml:"checker" json:"checker"`
	Oracle      map[string]any    `yaml:"oracle,omitempty" json:"oracle,omitempty"`
	EdgeClasses []string          `yaml:"edge_classes,omitempty" json:"edge_classes,omitempty"`
	Metamorphic []string          `yaml:"metamorphic,omitempty" json:"metamorphic,omitempty"`
	Reviewed    map[string]string `yaml:"reviewed,omitempty" json:"reviewed,omitempty"`

	// Dir is the directory of the problem node.
	Dir string `yaml:"-" json:"-"`
}

// Case is one line of a case file.
type Case struct {
	Name      string                     `json:"name"`
	Class     []string                   `json:"class,omitempty"`
	T         string                     `json:"T,omitempty"`
	Args      map[string]json.RawMessage `json:"args"`
	Expect    json.RawMessage            `json:"expect,omitempty"`
	ExpectAny []json.RawMessage          `json:"expect_any,omitempty"`
	// Flaky marks a case that gave two results on the same input (rule S09). It counts as a failure, and Issue names the open issue.
	Flaky bool   `json:"flaky,omitempty"`
	Issue string `json:"issue,omitempty"`
	// Found names the node and the language that a regression case broke, such as merge-sort/go (spec 09.11).
	Found string `json:"found,omitempty"`

	// File is the case file without .jsonl, such as basic. Line is the line in it.
	File string `json:"-"`
	Line int    `json:"-"`
}

// ID is the id of the case in the protocol and in reports: the file and the name, such as basic/duplicates.
func (c Case) ID() string { return c.File + "/" + c.Name }

// Type is the type of T for the case: int unless the case says otherwise.
func (c Case) Type() string {
	if c.T == "" {
		return "int"
	}
	return c.T
}

// Exists tells if dir has contract.yaml.
func Exists(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "contract.yaml"))
	return err == nil
}

// Load reads contract.yaml and the case files of the problem node in dir.
func Load(dir string) (*Contract, []Case, error) {
	b, err := os.ReadFile(filepath.Join(dir, "contract.yaml"))
	if err != nil {
		return nil, nil, err
	}
	var k Contract
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(&k); err != nil {
		return nil, nil, fmt.Errorf("%s: %w", filepath.Join(dir, "contract.yaml"), err)
	}
	k.Dir = dir
	if err := k.Validate(); err != nil {
		return nil, nil, fmt.Errorf("%s: %w", filepath.Join(dir, "contract.yaml"), err)
	}
	var cases []Case
	seen := map[string]string{}
	for _, f := range CaseFiles {
		cs, err := readCases(filepath.Join(dir, "cases", f+".jsonl"), f)
		if err != nil {
			return nil, nil, err
		}
		for _, c := range cs {
			if prev, dup := seen[c.Name]; dup {
				return nil, nil, fmt.Errorf("cases/%s.jsonl:%d: the name %q is also in cases/%s.jsonl", f, c.Line, c.Name, prev)
			}
			seen[c.Name] = f
			if err := k.CheckCase(c); err != nil {
				return nil, nil, fmt.Errorf("cases/%s.jsonl:%d: %s: %w", f, c.Line, c.Name, err)
			}
		}
		cases = append(cases, cs...)
	}
	return &k, cases, nil
}

// Validate checks the parts of the contract that the runner needs.
func (k *Contract) Validate() error {
	if k.Entry == "" {
		return errors.New("the contract has no entry")
	}
	if len(k.Params) == 0 {
		return errors.New("the contract has no params")
	}
	names := map[string]bool{}
	for _, p := range k.Params {
		if p.Name == "" || names[p.Name] {
			return fmt.Errorf("the parameter name %q is empty or used twice", p.Name)
		}
		names[p.Name] = true
		if _, err := Parse(p.Type); err != nil {
			return fmt.Errorf("the parameter %s: %w", p.Name, err)
		}
		if p.Mode != "" && p.Mode != "in" && p.Mode != "inout" {
			return fmt.Errorf("the parameter %s has the mode %q; use in or inout", p.Name, p.Mode)
		}
	}
	if _, err := Parse(k.Returns.Type); err != nil {
		return fmt.Errorf("the return type: %w", err)
	}
	if k.Returns.Type == "unit" && k.Output() < 0 {
		return errors.New("the entry returns unit, so one parameter must have the mode inout")
	}
	if !slices.Contains(Checkers, k.Checker.Kind) {
		return fmt.Errorf("%q is not a checker kind; use one of %s", k.Checker.Kind, strings.Join(Checkers, ", "))
	}
	if k.Checker.Kind == "custom" {
		if _, err := os.Stat(filepath.Join(k.Dir, "checker.py")); err != nil {
			return errors.New("the checker is custom, but the node has no checker.py")
		}
	}
	return nil
}

// Output is the index of the inout parameter whose value is the output, or -1 when the return value is the output.
func (k *Contract) Output() int {
	if k.Returns.Type != "unit" {
		return -1
	}
	for i, p := range k.Params {
		if p.Mode == "inout" {
			return i
		}
	}
	return -1
}

// OutputType is the type of the output for a case whose T is t.
func (k *Contract) OutputType(t string) Type {
	s := k.Returns.Type
	if i := k.Output(); i >= 0 {
		s = k.Params[i].Type
	}
	ty, _ := Parse(s)
	return ty.Subst("T", t)
}

// Types are the types that a generic contract is tested with (spec 08.3.4). A contract that is not generic has only int.
func (k *Contract) Types() []string {
	if len(k.Generic) == 0 {
		return []string{"int"}
	}
	return []string{"int", "string"}
}

// CheckCase checks that a case gives each parameter a value of its type, and that its expect has the output type.
func (k *Contract) CheckCase(c Case) error {
	if c.Name == "" {
		return errors.New("the case has no name")
	}
	if err := k.CheckArgs(c); err != nil {
		return err
	}
	return k.checkExpect(c)
}

// CheckArgs checks the type of T of a case and that it gives each parameter a value of its type. It does not look at expect, so it also checks a generated input.
func (k *Contract) CheckArgs(c Case) error {
	if c.T != "" && (len(k.Generic) == 0 || !slices.Contains(k.Types(), c.T)) {
		return fmt.Errorf("the case has T = %q, but the contract is tested only with %s", c.T, strings.Join(k.Types(), " and "))
	}
	for name := range c.Args {
		if !slices.ContainsFunc(k.Params, func(p Param) bool { return p.Name == name }) {
			return fmt.Errorf("the contract has no parameter %q", name)
		}
	}
	for _, p := range k.Params {
		raw, ok := c.Args[p.Name]
		if !ok {
			return fmt.Errorf("no value for the parameter %q", p.Name)
		}
		ty, _ := Parse(p.Type)
		v, err := decode(raw)
		if err != nil {
			return fmt.Errorf("the parameter %s: %w", p.Name, err)
		}
		if err := ty.Subst("T", c.Type()).Check(v); err != nil {
			return fmt.Errorf("the parameter %s: %w", p.Name, err)
		}
	}
	return nil
}

func (k *Contract) checkExpect(c Case) error {
	if len(c.Expect) > 0 {
		v, err := decode(c.Expect)
		if err != nil {
			return fmt.Errorf("expect: %w", err)
		}
		if err := k.OutputType(c.Type()).Check(v); err != nil {
			return fmt.Errorf("expect: %w", err)
		}
	} else if k.Checker.Kind != "custom" && k.Checker.Kind != "any_of" {
		return fmt.Errorf("the checker is %s, so the case needs expect", k.Checker.Kind)
	}
	if k.Checker.Kind == "any_of" && len(c.ExpectAny) == 0 {
		return errors.New("the checker is any_of, so the case needs expect_any")
	}
	return nil
}

func readCases(path, file string) ([]Case, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []Case
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 64*1024), 64*1024*1024)
	line := 0
	for s.Scan() {
		line++
		t := bytes.TrimSpace(s.Bytes())
		if len(t) == 0 {
			continue
		}
		var c Case
		dec := json.NewDecoder(bytes.NewReader(t))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&c); err != nil {
			return nil, fmt.Errorf("cases/%s.jsonl:%d: %w", file, line, err)
		}
		c.File, c.Line = file, line
		out = append(out, c)
	}
	return out, s.Err()
}

// decode reads a JSON value and keeps numbers exact.
func decode(b []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	if dec.More() {
		return nil, errors.New("more than one JSON value")
	}
	return v, nil
}
