// SPDX-License-Identifier: Apache-2.0

package check

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/tamnd/ai-agent/contract"
	"github.com/tamnd/ai-agent/node"
	"github.com/tamnd/ai-agent/status"
)

// sizeRE matches a size parameter of the bounds of a contract, such as len(xs), and gives its argument.
var sizeRE = regexp.MustCompile(`^len\((\w+)\)$`)

// contracts runs the S rules on a problem node that has contract.yaml (spec 09, 13.3).
func (r *run) contracts(n *node.Node) {
	dir := filepath.Join(r.root, n.Rel)
	if n.Kind != "task" || !contract.Exists(dir) {
		return
	}
	p := n.Rel + "/contract.yaml"
	k, cases, err := contract.Load(dir)
	if err != nil {
		msg := strings.TrimPrefix(err.Error(), filepath.Join(dir, "contract.yaml")+": ")
		if strings.HasPrefix(msg, "cases/") {
			r.add("S02", Block, n.Rel+"/"+msg, 0, "the case does not match the contract")
		} else {
			r.add("S01", Block, p, 0, "%s", msg)
		}
		return
	}
	r.sizeVars(n, k)
	r.edgeClasses(n, k, cases)
	r.checkerCases(n, k)
	r.generators(n, k)
	r.flaky(n)
	if n.Level == "core" {
		if ok, why := status.Reviewed(r.root, n.Tag, "contract", filepath.Join(dir, "contract.yaml")); !ok {
			r.add("S06", Level, p, 0, "the contract of a core problem needs a review by a person: %s", why)
		}
	}
}

// sizeVars checks that each variable of each claim of each solver is a size parameter of the contract or is defined in the vars of the solver (S01).
func (r *run) sizeVars(n *node.Node, k *contract.Contract) {
	size := map[string]bool{}
	for b := range k.Bounds {
		if m := sizeRE.FindStringSubmatch(b); m != nil {
			size[m[1]] = true
		}
		size[b] = true
	}
	for _, s := range r.c.Solvers(n.Slug) {
		if s.Complexity == nil {
			continue
		}
		for _, g := range []map[string]string{s.Complexity.Time, s.Complexity.Space} {
			for _, key := range sortedKeys(g) {
				for _, v := range Vars(g[key]) {
					if _, ok := s.Complexity.Vars[v]; !ok && !size[v] {
						r.add("S01", Block, s.Rel+"/node.md", 0, "the claim %q uses %q, which is not a size parameter of the contract of %s and not in vars", g[key], v, n.Slug)
					}
				}
			}
		}
	}
}

// edgeClasses checks that edge.jsonl has a case for each class of edge_classes (S04).
func (r *run) edgeClasses(n *node.Node, k *contract.Contract, cases []contract.Case) {
	have := map[string]bool{}
	for _, c := range cases {
		if c.File == "edge" {
			for _, cl := range c.Class {
				have[cl] = true
			}
		}
	}
	for _, cl := range k.EdgeClasses {
		if !have[cl] {
			r.add("S04", Level, n.Rel+"/cases/edge.jsonl", 0, "no case has the edge class %q; the implementations of %s stay below I2", cl, n.Slug)
		}
	}
	for cl := range have {
		if !slices.Contains(k.EdgeClasses, cl) {
			r.add("S04", Level, n.Rel+"/cases/edge.jsonl", 0, "the class %q is not in edge_classes of the contract", cl)
		}
	}
}

// checkerCase is one line of checker_cases.jsonl: an output and the verdict that the checker must give (spec 09.3).
type checkerCase struct {
	Name   string                     `json:"name"`
	Args   map[string]json.RawMessage `json:"args"`
	Output json.RawMessage            `json:"output"`
	Valid  *bool                      `json:"valid"`
	Note   string                     `json:"note,omitempty"`
}

// checkerCases checks that a custom checker has its test cases, each with a verdict, and at least one valid and one invalid output (S03). aik test runs the checker on them.
func (r *run) checkerCases(n *node.Node, k *contract.Contract) {
	if k.Checker.Kind != "custom" {
		return
	}
	p := n.Rel + "/checker_cases.jsonl"
	b, err := os.ReadFile(filepath.Join(r.root, p))
	if errors.Is(err, os.ErrNotExist) {
		r.add("S03", Block, n.Rel+"/contract.yaml", 0, "the checker is custom, so the node needs checker_cases.jsonl with valid and invalid outputs")
		return
	}
	if err != nil {
		r.add("S03", Block, p, 0, "%v", err)
		return
	}
	valid, invalid := 0, 0
	type numbered struct {
		c    checkerCase
		line int
	}
	var good []numbered
	sc := bufio.NewScanner(bytes.NewReader(b))
	sc.Buffer(make([]byte, 64*1024), 64*1024*1024)
	for line := 1; sc.Scan(); line++ {
		t := bytes.TrimSpace(sc.Bytes())
		if len(t) == 0 {
			continue
		}
		var c checkerCase
		dec := json.NewDecoder(bytes.NewReader(t))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&c); err != nil {
			r.add("S03", Block, p, line, "%v", err)
			continue
		}
		switch {
		case c.Name == "" || c.Args == nil || len(c.Output) == 0:
			r.add("S03", Block, p, line, "a checker case needs name, args and output")
		case c.Valid == nil:
			r.add("S03", Block, p, line, "the checker case %s has no valid field", c.Name)
		case *c.Valid:
			valid++
			good = append(good, numbered{c, line})
		default:
			invalid++
			if c.Note == "" {
				r.add("S03", Block, p, line, "the invalid output %s needs a note that says what is wrong with it", c.Name)
			}
			good = append(good, numbered{c, line})
		}
	}
	if valid == 0 || invalid == 0 {
		r.add("S03", Block, p, 0, "the checker cases need at least one valid and one invalid output; there are %d and %d", valid, invalid)
	}
	if r.opt.Checker == nil || len(good) == 0 || !r.wants("S03") {
		return
	}
	ck, err := r.opt.Checker(k)
	if err != nil {
		r.add("S03", Block, p, 0, "%v", err)
		return
	}
	defer ck.Close()
	for _, x := range good {
		reason, err := ck.Check(contract.Case{Name: x.c.Name, Args: x.c.Args}, x.c.Output, nil)
		switch {
		case err != nil:
			r.add("S03", Block, p, x.line, "%v", err)
			return
		case *x.c.Valid && reason != "":
			r.add("S03", Block, p, x.line, "the checker rejects the valid output %s: %s", x.c.Name, reason)
		case !*x.c.Valid && reason == "":
			r.add("S03", Block, p, x.line, "the checker accepts the invalid output %s (%s)", x.c.Name, x.c.Note)
		}
	}
}

// generatedFile is cases/generated.yaml (spec 09.4).
type generatedFile struct {
	Generators []struct {
		Name        string         `yaml:"name"`
		File        string         `yaml:"file"`
		Params      map[string]any `yaml:"params,omitempty"`
		Adversarial bool           `yaml:"adversarial,omitempty"`
		Targets     []string       `yaml:"targets,omitempty"`
	} `yaml:"generators"`
	Sizes map[string]any `yaml:"sizes,omitempty"`
}

var randomRE = regexp.MustCompile(`(?m)^\s*(import random|from random import|import secrets|import time|from time import)|random\.(random|randint|choice|shuffle|seed)\(|time\.time\(`)

// generators checks cases/generated.yaml: each generator has a unique name and a file under generators/, and the file uses only the rng that it gets, so that a seed gives the same input on each machine (S05).
func (r *run) generators(n *node.Node, k *contract.Contract) {
	p := n.Rel + "/cases/generated.yaml"
	b, err := os.ReadFile(filepath.Join(r.root, p))
	if errors.Is(err, os.ErrNotExist) {
		return
	}
	if err != nil {
		r.add("S05", Block, p, 0, "%v", err)
		return
	}
	var g generatedFile
	if err := yaml.Unmarshal(b, &g); err != nil {
		r.add("S05", Block, p, 0, "%v", err)
		return
	}
	seen := map[string]bool{}
	for _, gen := range g.Generators {
		if gen.Name == "" || seen[gen.Name] {
			r.add("S05", Block, p, 0, "the generator name %q is empty or used twice", gen.Name)
		}
		seen[gen.Name] = true
		if !strings.HasPrefix(gen.File, "generators/") || !strings.HasSuffix(gen.File, ".py") || strings.Contains(gen.File, "..") {
			r.add("S05", Block, p, 0, "the generator %s has the file %q; use a Python file under generators/", gen.Name, gen.File)
			continue
		}
		src, err := os.ReadFile(filepath.Join(r.root, n.Rel, filepath.FromSlash(gen.File)))
		if err != nil {
			r.add("S05", Block, p, 0, "the generator %s: the file %s does not exist", gen.Name, gen.File)
			continue
		}
		if !bytes.Contains(src, []byte("def generate(rng")) {
			r.add("S05", Block, n.Rel+"/"+gen.File, 0, "the generator needs a function generate(rng, n, ...)")
		}
		if m := randomRE.Find(src); m != nil {
			line := bytes.Count(src[:bytes.Index(src, m)], []byte("\n")) + 1
			r.add("S05", Block, n.Rel+"/"+gen.File, line, "the generator uses %q; use only the rng that it gets, so that the seed fixes the input", strings.TrimSpace(string(m)))
		}
	}
}

// flakyCase is the part of a case line that S09 reads.
type flakyCase struct {
	Name  string `json:"name"`
	Flaky bool   `json:"flaky,omitempty"`
	Issue string `json:"issue,omitempty"`
}

var issueRE = regexp.MustCompile(`^(https://github\.com/[\w.-]+/[\w.-]+/issues/|#)\d+$`)

// flaky checks that each case marked flaky names its open issue (S09).
func (r *run) flaky(n *node.Node) {
	for _, f := range contract.CaseFiles {
		p := n.Rel + "/cases/" + f + ".jsonl"
		b, err := os.ReadFile(filepath.Join(r.root, p))
		if err != nil {
			continue
		}
		for i, line := range bytes.Split(b, []byte("\n")) {
			var c flakyCase
			if len(bytes.TrimSpace(line)) == 0 || json.Unmarshal(line, &c) != nil || !c.Flaky {
				continue
			}
			if !issueRE.MatchString(c.Issue) {
				r.add("S09", Block, p, i+1, "the flaky case %s needs an issue, such as \"#12\"; it counts as a failure until the issue is fixed", c.Name)
			}
		}
	}
}
