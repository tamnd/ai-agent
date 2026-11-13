// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/tamnd/ai-agent/check"
	"github.com/tamnd/ai-agent/exp"
	"github.com/tamnd/ai-agent/node"
)

// A problem is one thing that the gate found: a step of 14.6, the file, and what is wrong.
type problem struct {
	Step     string `json:"step"`
	Path     string `json:"path,omitempty"`
	Rule     string `json:"rule,omitempty"`
	Severity string `json:"severity"`
	Msg      string `json:"msg"`
}

type gateReport struct {
	Slug     string    `json:"slug"`
	Pass     bool      `json:"pass"`
	Steps    []string  `json:"steps"`
	Problems []problem `json:"problems"`
}

// stubMarkers are the texts that aik new writes and that a written page must replace.
var stubMarkers = []string{"Write one sentence that says what this is"}

// badChars are the characters that no text file of a node may hold, as in algorithm.
var badChars = []struct {
	r    rune
	name string
}{{'\t', "a tab"}, {'\f', "a form feed"}, {'\r', "a carriage return"}, {'—', "an em dash"}, {'–', "an en dash"}}

// cmdGate is the writer gate of spec 14.6: the cheap checks first, so that they fail first.
func cmdGate(args []string) error {
	f := newFlags("gate", "[flags] <slug>...")
	root := f.String("root", ".", "the root of the content repository")
	asJSON := f.Bool("json", false, "print the reports as JSON")
	sandbox := f.String("sandbox", "auto", "the container engine of the tests and experiments: docker, podman, or none")
	unsafe := f.Bool("unsafe-local", false, "run the tests and experiments on this machine, not in the sandbox; CI sets CI=true instead")
	noTests := f.Bool("no-tests", false, "skip the framework tests (step 7)")
	noExp := f.Bool("no-exp", false, "skip the cpu-small experiments (step 8)")
	withCite := f.Bool("cite", false, "also run the citation check (step 9), as the end of a write does")
	if err := parse(f, args); err != nil {
		return err
	}
	if f.NArg() == 0 {
		return usageError{"give a slug"}
	}
	c, err := load(*root)
	if err != nil {
		return err
	}
	for _, s := range f.Args() {
		if c.BySlug[s] == nil {
			return usageError{fmt.Sprintf("no node has the slug %q", s)}
		}
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	sub := func(args ...string) (string, error) {
		cmd := exec.CommandContext(context.Background(), self, args...) // #nosec G204 -- aik runs itself
		var out bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &out
		err := cmd.Run()
		return out.String(), err
	}
	box := []string{"-root", *root, "-sandbox", *sandbox}
	if *unsafe {
		box = append(box, "-unsafe-local")
	}
	var reports []gateReport
	failed := false
	for _, slug := range f.Args() {
		r := gateReport{Slug: slug, Problems: []problem{}}
		add := func(step, path, rule, msg string) {
			r.Problems = append(r.Problems, problem{Step: step, Path: path, Rule: rule, Severity: "error", Msg: msg})
		}

		// 1. fmt: put the files of the node in canonical form.
		r.Steps = append(r.Steps, "fmt")
		files, err := fmtFiles(*root, c, map[string]bool{slug: true})
		if err != nil {
			return err
		}
		for _, p := range files {
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			out := node.FormatText(b)
			if filepath.Base(p) == "node.md" {
				if out, err = node.FormatNode(b); err != nil {
					add("fmt", rel(*root, p), "", err.Error())
					continue
				}
			}
			if !bytes.Equal(b, out) {
				if err := os.WriteFile(p, out, 0o644); err != nil { // #nosec G306 -- a file of the repository
					return err
				}
			}
		}
		if c, err = load(*root); err != nil {
			return err
		}
		n := c.BySlug[slug]

		// 2. check: the rules that block, on this node.
		r.Steps = append(r.Steps, "check")
		for _, x := range check.Run(c, check.Options{Only: map[string]bool{slug: true}}) {
			if x.Severity == check.Block {
				p := x.Path
				if x.Line > 0 {
					p = fmt.Sprintf("%s:%d", x.Path, x.Line)
				}
				add("check", p, x.Rule, x.Msg)
			}
		}

		// 3 to 5: stub texts, In plain words, and the characters of the text files.
		r.Steps = append(r.Steps, "stubs", "plain words", "characters")
		r.Problems = append(r.Problems, textProblems(*root, n)...)

		// 6. claims: the node has claims.yaml; the G rules of step 2 check the type and the sources of each claim.
		r.Steps = append(r.Steps, "claims")
		if !exists(filepath.Join(n.Dir, "claims.yaml")) {
			add("claims", rel(*root, n.Dir), "", "claims.yaml is missing (07.3)")
		}

		// 7. the tests of each framework present.
		if !*noTests && hasCode(n) {
			r.Steps = append(r.Steps, "tests")
			if out, err := sub(append(append([]string{"test"}, box...), slug)...); err != nil {
				add("tests", rel(*root, n.Dir), "", "aik test fails:\n"+tail(out, 2500))
			}
		}

		// 8. the cpu-small experiments and their expect.yaml.
		if !*noExp {
			for _, e := range n.Exps {
				if e.Design == nil || exp.Budgets[e.Design.Budget].Name != "cpu-small" {
					continue
				}
				r.Steps = append(r.Steps, "exp "+e.Slug)
				py := []string{"exp", "run"}
				py = append(py, box...)
				if out, err := sub(append(py, slug+"/"+e.Slug)...); err != nil {
					add("exp", rel(*root, filepath.Join(n.Dir, "exp", e.Slug)), "", "aik exp run fails:\n"+tail(out, 2500))
				}
			}
		}

		// 9. the citation check, at the end of a write.
		if *withCite && len(n.Claims) > 0 {
			r.Steps = append(r.Steps, "cite")
			if out, err := sub("cite", "-root", *root, slug); err != nil {
				add("cite", rel(*root, n.Dir), "G05", "aik cite finds a claim not in its source:\n"+tail(out, 2500))
			}
		}

		r.Pass = len(r.Problems) == 0
		failed = failed || !r.Pass
		reports = append(reports, r)
	}
	if *asJSON {
		if len(reports) == 1 {
			if err := printJSON(reports[0]); err != nil {
				return err
			}
		} else if err := printJSON(reports); err != nil {
			return err
		}
	} else {
		for _, r := range reports {
			verdict := "PASS"
			if !r.Pass {
				verdict = "FAIL"
			}
			fmt.Printf("%s: %s (%s)\n", r.Slug, verdict, strings.Join(r.Steps, ", "))
			for _, p := range r.Problems {
				fmt.Printf("  - %s: %s: %s %s\n", p.Step, p.Path, p.Rule, indent(p.Msg))
			}
		}
	}
	if failed {
		return errFindings
	}
	return nil
}

var (
	plainHeadRE = regexp.MustCompile(`(?m)^## In plain words\s*$`)
	nextHeadRE  = regexp.MustCompile(`(?m)^#{1,2} `)
	inlineMath  = regexp.MustCompile(`\$\$[\s\S]*?\$\$|\$[^$\n]+\$`)
	// opRE finds what makes a formula more than one term: a relation, a sum or a difference, a product or a fraction.
	opRE = regexp.MustCompile(`[=<>+]|[^\s^_{(]\s*-|\\(frac|sum|prod|int|cdot|times|le|ge|leq|geq|approx|neq|sim|propto|to|mid)\b`)
)

// textProblems are the steps 3 to 5 of the gate: no stub text, In plain words short and without long formulas, and
// no tab, form feed, carriage return or long dash in a text file of the node.
func textProblems(root string, n *node.Node) []problem {
	var out []problem
	add := func(step, path, msg string) {
		out = append(out, problem{Step: step, Path: path, Severity: "error", Msg: msg})
	}
	_ = filepath.WalkDir(n.Dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			if d != nil && d.IsDir() && (d.Name() == "__pycache__" || d.Name() == "node_modules" || strings.HasPrefix(d.Name(), ".")) && p != n.Dir {
				return filepath.SkipDir
			}
			return nil
		}
		b, err := os.ReadFile(p) // #nosec G304 -- a file of the node directory
		if err != nil || !utf8.Valid(b) || bytes.IndexByte(b, 0) >= 0 {
			return nil //nolint:nilerr // a file that cannot be read as text is not a text file
		}
		s := string(b)
		for _, m := range stubMarkers {
			if strings.Contains(s, m) {
				add("stubs", rel(root, p), fmt.Sprintf("still holds the stub text %q", m))
			}
		}
		for _, c := range badChars {
			if i := strings.IndexRune(s, c.r); i >= 0 {
				if c.r == '\t' && filepath.Base(p) == "Makefile" {
					continue
				}
				add("characters", fmt.Sprintf("%s:%d", rel(root, p), strings.Count(s[:i], "\n")+1),
					fmt.Sprintf("holds %s character; a tab or form feed in math is often a broken escape such as \\times or \\frac written through a string", c.name))
			}
		}
		return nil
	})
	b, err := os.ReadFile(filepath.Join(n.Dir, "node.md"))
	if err != nil {
		return out
	}
	body := string(b)
	if loc := plainHeadRE.FindStringIndex(body); loc != nil {
		sec := body[loc[1]:]
		if j := nextHeadRE.FindStringIndex(sec); j != nil {
			sec = sec[:j[0]]
		}
		words := len(strings.Fields(inlineMath.ReplaceAllString(sec, "x")))
		if words > 150 {
			add("plain words", rel(root, filepath.Join(n.Dir, "node.md")), fmt.Sprintf("'In plain words' has %d words; the limit is 150", words))
		}
		for _, m := range inlineMath.FindAllString(sec, -1) {
			if strings.HasPrefix(m, "$$") || opRE.MatchString(strings.Trim(m, "$")) {
				add("plain words", rel(root, filepath.Join(n.Dir, "node.md")), fmt.Sprintf("'In plain words' has the formula %s, longer than one term", m))
			}
		}
	}
	return out
}

// hasCode tells if the node has an implementation in some framework.
func hasCode(n *node.Node) bool {
	ents, err := os.ReadDir(filepath.Join(n.Dir, "impl"))
	if err != nil {
		return false
	}
	for _, e := range ents {
		if e.IsDir() {
			return true
		}
	}
	return false
}

func tail(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return "..." + s[len(s)-n:]
	}
	return s
}
