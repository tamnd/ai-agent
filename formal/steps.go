// SPDX-License-Identifier: Apache-2.0

package formal

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/tamnd/ai-agent/node"
)

// Step is a step of a Lean proof that a `-- step: <id>` comment marks (spec 11.6), one line of build/lean/steps.jsonl.
type Step struct {
	Node   string `json:"node"`
	ID     string `json:"id"` // the id without the prefix "step-"
	Module string `json:"module"`
	Line   int    `json:"line"` // the line of the comment
	// Decl is the declaration that proves the step: the theorem that the comment heads, or the one whose proof holds
	// the marked have, calc or tactic block.
	Decl string `json:"decl"`
	// Text is the statement of the step: the elaborated type of the theorem it heads, or the source of the marked
	// block up to its ":=".
	Text string `json:"text"`
	// Builds is true when Decl is in the build, has no sorry, and uses only the allowed axioms (11.5).
	Builds bool `json:"builds"`
}

var (
	pageStepRE = regexp.MustCompile(`\{#(step-[A-Za-z0-9_-]+)\}`)
	leanStepRE = regexp.MustCompile(`^\s*--\s*step:\s*([A-Za-z0-9_.-]+)\s*$`)
)

// StepID is the id of a step without the prefix "step-", which the page writes and a Lean comment may leave out.
func StepID(s string) string { return strings.TrimPrefix(s, "step-") }

// PageSteps are the step ids of a page proof, in order, without the prefix "step-".
func PageSteps(path string) ([]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, m := range pageStepRE.FindAllStringSubmatch(string(b), -1) {
		out = append(out, StepID(m[1]))
	}
	return out, nil
}

// ModuleFile is the source file of a module of the AI package under root.
func ModuleFile(root, mod string) string {
	return filepath.Join(root, "lean", filepath.FromSlash(strings.ReplaceAll(mod, ".", "/"))+".lean")
}

// LeanSteps are the marked steps of the source of a module, in the order of the file. The declarations of ds give
// each step its Decl, Text and Builds; with nil ds a step has only its id and line.
func LeanSteps(src []byte, mod string, ds Decls) []Step {
	lines := strings.Split(string(src), "\n")
	// The declarations of the module that have a position, by first line.
	var own []Decl
	for _, d := range ds {
		if d.Module == mod && d.Pos != nil {
			own = append(own, d)
		}
	}
	sort.Slice(own, func(i, j int) bool {
		if own[i].Pos.Line != own[j].Pos.Line {
			return own[i].Pos.Line < own[j].Pos.Line
		}
		return own[i].Decl < own[j].Decl
	})
	var out []Step
	for i, l := range lines {
		m := leanStepRE.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		s := Step{ID: StepID(m[1]), Module: mod, Line: i + 1}
		inside := false
		// The innermost declaration whose range holds the comment, else the first one after it.
		for _, d := range own {
			if d.Pos.Line <= s.Line && s.Line <= d.Pos.EndLine {
				if s.Decl == "" || d.Pos.Line >= ds[s.Decl].Pos.Line {
					s.Decl, inside = d.Decl, true
				}
			}
		}
		if s.Decl == "" {
			for _, d := range own {
				if d.Pos.Line > s.Line {
					s.Decl = d.Decl
					break
				}
			}
		}
		if s.Decl != "" {
			d := ds[s.Decl]
			s.Builds = !d.Sorry && len(d.Disallowed()) == 0
			if !inside {
				s.Text = d.Type
			}
		}
		if s.Text == "" {
			s.Text = blockText(lines[i+1:])
		}
		out = append(out, s)
	}
	return out
}

// blockText is the source of the block after a step comment and its doc comment, up to its ":=", in at most four lines.
func blockText(lines []string) string {
	var b []string
	inDoc := false
	for _, l := range lines {
		t := strings.TrimSpace(l)
		// A doc comment before the block is not its statement.
		if len(b) == 0 && (inDoc || strings.HasPrefix(t, "/-")) {
			inDoc = !strings.Contains(t, "-/")
			continue
		}
		if t == "" && len(b) == 0 {
			continue
		}
		if t == "" || strings.HasPrefix(t, "--") {
			break
		}
		if k := strings.Index(t, ":="); k >= 0 {
			b = append(b, strings.TrimSpace(t[:k]))
			break
		}
		b = append(b, t)
		if len(b) == 4 {
			break
		}
	}
	return strings.Join(b, " ")
}

// NodeSteps reads the page steps and the Lean steps of n, the node of a theorem with lean.steps. ok is false when n
// has no lean.steps or no lean.module.
func NodeSteps(root string, n *node.Node, ds Decls) (page []string, lean []Step, ok bool, err error) {
	rel, mod := n.Lean["steps"], n.Lean["module"]
	if rel == "" || mod == "" {
		return nil, nil, false, nil
	}
	page, err = PageSteps(filepath.Join(n.Dir, filepath.FromSlash(rel)))
	if err != nil {
		return nil, nil, true, err
	}
	src, err := os.ReadFile(ModuleFile(root, mod))
	if err != nil {
		return nil, nil, true, err
	}
	lean = LeanSteps(src, mod, ds)
	for i := range lean {
		lean[i].Node = n.Slug
	}
	return page, lean, true, nil
}

// CheckSteps is rule F08 (11.6) for a node: every step id of the page proof is marked in the Lean proof, in the same
// order; each mark names a step of the page, once; and the declaration of each marked step builds with the allowed
// axioms. The Lean proof may have unmarked steps of its own.
func CheckSteps(n *node.Node, page []string, lean []Step) []Finding {
	where := n.Slug + " lean.steps"
	f := func(msg string) Finding { return Finding{Rule: "F08", Where: where, Msg: msg, Node: n.Slug} }
	var out []Finding
	if len(page) == 0 {
		return []Finding{f(n.Lean["steps"] + " gives no step an id {#step-…}")}
	}
	onPage := map[string]bool{}
	for _, id := range page {
		if onPage[id] {
			out = append(out, f("the page gives the id step-"+id+" to two steps"))
		}
		onPage[id] = true
	}
	seen := map[string]int{}
	var order []string
	for _, s := range lean {
		at := fmt.Sprintf("%s line %d", s.Module, s.Line)
		if !onPage[s.ID] {
			out = append(out, f("the Lean step "+s.ID+" ("+at+") is not a step of "+n.Lean["steps"]))
			continue
		}
		if l, dup := seen[s.ID]; dup {
			out = append(out, f(fmt.Sprintf("the step %s is marked twice, at lines %d and %d of %s", s.ID, l, s.Line, s.Module)))
			continue
		}
		seen[s.ID] = s.Line
		order = append(order, s.ID)
		switch {
		case s.Decl == "":
			out = append(out, f("the Lean step "+s.ID+" ("+at+") is in no declaration of the build"))
		case !s.Builds:
			out = append(out, f("the Lean step "+s.ID+" ("+at+") is in "+s.Decl+", which has a sorry or an axiom outside the policy"))
		}
	}
	for _, id := range page {
		if _, ok := seen[id]; !ok {
			out = append(out, f("the step step-"+id+" of the page has no -- step: "+id+" in the Lean proof"))
		}
	}
	// The marked page steps must come in the order of the page.
	want := slices.DeleteFunc(slices.Clone(page), func(id string) bool { _, ok := seen[id]; return !ok })
	if !slices.Equal(order, want) {
		out = append(out, f("the Lean proof marks the steps in the order "+strings.Join(order, ", ")+", but the page has "+strings.Join(want, ", ")))
	}
	return out
}

// WriteSteps writes the steps as JSON lines.
func WriteSteps(path string, steps []Step) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	enc := json.NewEncoder(w)
	for _, s := range steps {
		if err := enc.Encode(s); err != nil {
			return errors.Join(err, f.Close())
		}
	}
	if err := w.Flush(); err != nil {
		return errors.Join(err, f.Close())
	}
	return f.Close()
}
