// Package check runs the audit rules of spec 13.3 that `aik check` needs.
//
// Each rule has an id, such as T01. A rule with the severity "block" stops a merge. A rule with the severity "level" keeps a part below the level that needs it. A rule with the severity "warn" only reports.
package check

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/tamnd/ai-agent/contract"
	"github.com/tamnd/ai-agent/formal"
	"github.com/tamnd/ai-agent/mutate"
	"github.com/tamnd/ai-agent/node"
	"github.com/tamnd/ai-agent/status"
)

// Severity is how much a finding matters.
type Severity string

const (
	Block Severity = "block"
	Level Severity = "level"
	Warn  Severity = "warn"
)

// Finding is one result of a rule.
type Finding struct {
	Rule     string   `json:"rule"`
	Severity Severity `json:"severity"`
	Path     string   `json:"path"`
	Line     int      `json:"line,omitempty"`
	Msg      string   `json:"msg"`
}

func (f Finding) String() string {
	loc := f.Path
	if f.Line > 0 {
		loc = fmt.Sprintf("%s:%d", f.Path, f.Line)
	}
	return fmt.Sprintf("%s: %s %s: %s", loc, f.Rule, f.Severity, f.Msg)
}

// Rules lists the rules that this package runs, with their text.
var Rules = map[string]string{
	"T01": "node.md front matter, contract.yaml and the shared cases are valid; each complexity variable of each solving algorithm is a size parameter or is defined",
	"T02": "each tag is in registers/tags.reg, once, with its slug, and no provisional id is left on main",
	"T03": "each slug is unique, lowercase kebab-case, at most 60 characters, and not an old slug in aliases.reg",
	"T08": "the path of the node matches its area",
	"T11": "each card shard is sorted by tag, and each card is in the shard of its tag",
	"T12": "a tag is a card or a node, never both",
	"T05": "no status, level or badge is written in front matter",
	"E01": "each edge target exists, as a node or a card",
	"E02": "each implements target is an abstract type",
	"E03": "prereqs edges form a DAG",
	"E04": "variant_of and generalizes edges have no cycle",
	"E05": "each solves target is a problem node",
	"E08": "no edge points to its own node; see_also has at most 8 targets",
	"D01": "every fact has a value, a unit and at least one source, and its value is the value of its best source",
	"D02": "every model, dataset, benchmark, hardware item, policy and system has at least one source",
	"D03": "the required facts of the kind are present (10.11)",
	"D04": "a primary source is cited where one exists, with a locator",
	"D05": "sources that differ beyond the tolerance make the fact disputed, and the page shows both values",
	"D06": "an estimated fact names its estimator and method",
	"D07": "a time-bound fact has as_of",
	"D09": "units are in the unit list of 04.9",
	"G01": "every source of a claim names a work",
	"G02": "every claim has at least one source, unless it is a mathematical claim with a derivation or a Lean proof",
	"G03": "every evidence entry has a locator",
	"K01": "every {#c-...} anchor names a claim of claims.yaml once, a sentence has at most two anchors, and every ::claim reference names a claim that exists",
	"K03": "the evidence of each claim has a role that its type allows (07.2)",
	"K04": "no \"state of the art\", \"SOTA\", \"currently\", \"leading\", \"frontier\" as a rank, or superlative about a model, a lab, a score or a dataset in the site voice",
	"K07": "a forecast claim has the date of the forecast in as_of",
	"K09": "a superseded claim is not anchored in the main text",
	"K13": "no claim is past its review_after date",
	"S11": "each complexity string parses, and each of its variables is defined in vars, and the model exists",
	"S04": "the section headings of the kind are in order",
	"S10": "a card summary is at most 30 words",
	"X11": "pseudocode line ids are unique, each @line marker names an existing id, and each line id of the main pseudocode has a marker in each tier-1 implementation",
	"X19": "each code file starts with the licence header, which names the page and the tag of the node in a tree with the framework register",
	"X01": "the NumPy reference exists before any other framework version of the node",
	"X07": "the code imports only the packages of the pinned set of its framework",
	"X12": "the framework is in the register, the file names follow its conventions, and no implementation has a build file or a generated file under build/; aik gen writes them",
	"X14": "a reference version has at most 200 lines",
	"M04": "the task contract is sound: a custom checker has valid and invalid checker cases, edge.jsonl covers each edge class, each generator uses only its rng, a flaky case names its open issue, the oracles agree, and a person reviewed the contract of each core problem",
	"X10": "mutants.ignore of an implementation lists at most 5% of its mutants, each with a reason, and each entry names a mutant of the sample",
	"F01": "each Lean name in the front matter exists in decls.jsonl with the expected kind, and each time claim of a node with lean.cost has a class lemma of the same class",
	"F02": "the correctness theorem mentions the spec and the model, and the fidelity review of the spec of a core node is current",
	"F03": "each declaration uses only propext, Classical.choice and Quot.sound",
	"F06": "the model is total and computable",
	"F07": "the module path mirrors the node path",
	"F10": "each module builds in at most 60 seconds; aik lean build -time reports it, and a module above 300 seconds blocks",
	"L01": "each link in a node to /<slug>/ names a node, an area or a site page",
	"T04": "no node slug is a reserved word, a language id or an area id",
	"R01": "experiment.yaml is valid, names claims that exist in claims.yaml, and has a control arm for each comparative test",
	"R02": "every input of an experiment is a generator in its directory or a dataset of registers/data-pins.yaml with a resolved licence and hashed files",
	"R03": "scale_note states the scale of the experiment and the scale of the claim of the source",
	"R04": "no page text presents a small-scale experiment as a reproduction of the result of a paper",
	"R05": "results.json is written only by aik exp run -update, with the run id, the hashes, the seeds and every arm",
	"R06": "expect.yaml exists before the first full run and does not change after it",
	"R08": "every arm has the same seeds and the same tuning budget",
	"R09": "the run stayed within the cap of its budget class",
}

// Options select what to check.
type Options struct {
	// Only, when not empty, limits the run to these slugs.
	Only map[string]bool
	// Rule, when not empty, runs only this rule.
	Rule string
	// Main is true on the main branch, where provisional ids are an error (T02).
	Main bool
	// Decls are the declarations that aik lean build wrote. When it is nil, the rules F01 to F07 that need them do not run.
	Decls formal.Decls
	// Checker, when it is not nil, starts the custom checker of a contract. M04 then runs the checker on each line of checker_cases.jsonl. The caller decides where the checker runs, such as in the sandbox.
	Checker func(k *contract.Contract) (*contract.Custom, error)
}

type run struct {
	c    *node.Corpus
	opt  Options
	out  []Finding
	root string
}

func (r *run) add(rule string, sev Severity, path string, line int, format string, args ...any) {
	if !r.wants(rule) {
		return
	}
	r.out = append(r.out, Finding{Rule: rule, Severity: sev, Path: path, Line: line, Msg: fmt.Sprintf(format, args...)})
}

// wants reports whether the run includes the rule.
func (r *run) wants(rule string) bool { return r.opt.Rule == "" || r.opt.Rule == rule }

func (r *run) want(n *node.Node) bool {
	return len(r.opt.Only) == 0 || r.opt.Only[n.Slug]
}

// Run runs the rules on the corpus and returns the findings, sorted by path and line.
func Run(c *node.Corpus, opt Options) []Finding {
	r := &run{c: c, opt: opt, root: c.Root}
	for _, err := range c.Errors {
		r.add("T01", Block, rel(c.Root, err.Error()), 0, "%v", err)
	}
	r.tags()
	r.cards()
	r.groups()
	r.oldSlugs()
	r.committedBuild()
	slugs := map[string][]*node.Node{}
	for _, n := range c.Nodes {
		slugs[n.Slug] = append(slugs[n.Slug], n)
	}
	for _, n := range c.Nodes {
		if !r.want(n) {
			continue
		}
		r.front(n)
		r.facts(n)
		r.claims(n)
		r.exps(n)
		r.voice(n)
		if len(slugs[n.Slug]) > 1 {
			r.add("T03", Block, n.Rel+"/node.md", 0, "the slug %q is used by %d nodes", n.Slug, len(slugs[n.Slug]))
		}
		if to, ok := c.OldSlugs[n.Slug]; ok {
			r.add("T03", Block, n.Rel+"/node.md", 0, "the slug %q is an old slug in registers/aliases.reg that moved to %q; an old URL never changes its target", n.Slug, to)
		}
		r.edges(n)
		r.complexity(n)
		r.sections(n)
		r.pseudo(n)
		r.impls(n)
		r.links(n)
		r.contracts(n)
		r.leanNode(n)
	}
	r.leanDecls()
	r.cycles("E03", "prereqs", func(n *node.Node) []string { return n.Prereqs })
	r.cycles("E04", "variant_of and generalizes", func(n *node.Node) []string {
		out := slices.Clone(n.Generalizes)
		if n.VariantOf != "" {
			out = append(out, n.VariantOf)
		}
		return out
	})
	sort.SliceStable(r.out, func(i, j int) bool {
		if r.out[i].Path != r.out[j].Path {
			return r.out[i].Path < r.out[j].Path
		}
		return r.out[i].Line < r.out[j].Line
	})
	return r.out
}

func rel(root, s string) string {
	if strings.HasPrefix(s, root+string(filepath.Separator)) {
		s = s[len(root)+1:]
	}
	if i := strings.Index(s, ": "); i > 0 {
		return s[:i]
	}
	return s
}

var (
	slugRE   = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	tagRE    = regexp.MustCompile(`^[0-9A-HJKMNP-TV-Z]{6}$`) // Crockford base32: no I, L, O, U
	provRE   = regexp.MustCompile(`^NEW-[a-z0-9-]+$`)
	idLineRE = regexp.MustCompile(`^(L\d+)\s`)
	markerRE = regexp.MustCompile(`@line\s+((?:L\d+)(?:[ ,]+L\d+)*)`)
)

func (r *run) front(n *node.Node) {
	p := n.Rel + "/node.md"
	for _, k := range n.Forbidden {
		r.add("T05", Block, p, 0, "the front matter has the key %q; the build computes it (04.8)", k)
	}
	req := map[string]string{"kind": n.Kind, "slug": n.Slug, "name": n.Name, "summary": n.Summary}
	if n.Kind != "person" && n.Kind != "exercise" {
		req["area"] = n.Area
		req["level"] = n.Level
	}
	for _, k := range sortedKeys(req) {
		if req[k] == "" {
			r.add("T01", Block, p, 0, "the front matter has no %q", k)
		}
	}
	if n.Kind != "" && !node.IsKind(n.Kind) {
		r.add("T01", Block, p, 0, "%q is not a kind (04.1)", n.Kind)
	}
	if n.Level != "" && !slices.Contains(node.Levels, n.Level) {
		r.add("T01", Block, p, 0, "%q is not a level; use one of %s", n.Level, strings.Join(node.Levels, ", "))
	}
	if len(n.Summary) > 200 {
		r.add("T01", Block, p, 0, "the summary has %d characters; the limit is 200", len(n.Summary))
	}
	if !slugRE.MatchString(n.Slug) || len(n.Slug) > 60 {
		r.add("T03", Block, p, 0, "the slug %q is not lowercase kebab-case of at most 60 characters", n.Slug)
	}
	if filepath.Base(n.Rel) != n.Slug {
		r.add("T08", Block, p, 0, "the directory is %q but the slug is %q", filepath.Base(n.Rel), n.Slug)
	}
	if n.Area != "" {
		if _, ok := r.c.AreaByID[n.Area]; !ok {
			r.add("T08", Block, p, 0, "the area %q is not in topics/areas.yaml", n.Area)
		} else if want := "nodes/" + n.Area + "/" + n.Slug; filepath.ToSlash(n.Rel) != want {
			r.add("T08", Block, p, 0, "the node is at %s but its area puts it at %s", n.Rel, want)
		}
	}
	for _, a := range n.Areas {
		if _, ok := r.c.AreaByID[a]; !ok {
			r.add("T08", Block, p, 0, "the secondary area %q is not in topics/areas.yaml", a)
		}
	}
	if r.c.Reserved[n.Slug] || r.c.LangByID[n.Slug] != nil {
		r.add("T04", Block, p, 0, "the slug %q is a reserved word or a language id (14.2)", n.Slug)
	}
	for _, a := range r.c.AreaByID {
		if a.ID == n.Slug {
			r.add("T04", Block, p, 0, "the slug %q is the id of an area, and the area page uses the same URL (14.2)", n.Slug)
			break
		}
	}
	switch {
	case provRE.MatchString(n.Tag):
		if r.opt.Main {
			r.add("T02", Block, p, 0, "the provisional id %s is on main; run aik mint", n.Tag)
		}
	case !tagRE.MatchString(n.Tag):
		r.add("T02", Block, p, 0, "the tag %q is not 6 characters of Crockford base32", n.Tag)
	}
}

func (r *run) tags() {
	seenTag := map[string]int{}
	seenSlug := map[string]int{}
	for _, t := range r.c.Tags {
		if l, dup := seenTag[t.Tag]; dup {
			r.add("T02", Block, "registers/tags.reg", t.Line, "the tag %s is also on line %d", t.Tag, l)
		}
		seenTag[t.Tag] = t.Line
		if l, dup := seenSlug[t.Slug]; dup {
			r.add("T02", Block, "registers/tags.reg", t.Line, "the slug %s is also on line %d", t.Slug, l)
		}
		seenSlug[t.Slug] = t.Line
	}
	reg := map[string]string{}
	for _, t := range r.c.Tags {
		reg[t.Tag] = t.Slug
	}
	for _, n := range r.c.Nodes {
		if !r.want(n) || n.Tag == "" || provRE.MatchString(n.Tag) {
			continue
		}
		s, ok := reg[n.Tag]
		switch {
		case !ok:
			r.add("T02", Block, n.Rel+"/node.md", 0, "the tag %s is not in registers/tags.reg", n.Tag)
		case s != n.Slug:
			r.add("T02", Block, n.Rel+"/node.md", 0, "registers/tags.reg gives the tag %s to %q, not %q", n.Tag, s, n.Slug)
		}
	}
}

func (r *run) target(n *node.Node, field, slug string, kinds ...string) {
	p := n.Rel + "/node.md"
	if slug == n.Slug {
		r.add("E08", Warn, p, 0, "%s points to its own node", field)
		return
	}
	t, ok := r.c.BySlug[slug]
	if cd := r.c.CardByKey[slug]; !ok && cd != nil {
		// A slug in a field can name a card (spec 4.5).
		if len(kinds) > 0 && !slices.Contains(kinds, cd.Kind) {
			r.add("E01", Block, p, 0, "%s: %q is a %s card, not a %s", field, slug, cd.Kind, strings.Join(kinds, " or "))
		}
		return
	}
	if !ok {
		if to, moved := r.c.OldSlugs[slug]; moved {
			r.add("E01", Block, p, 0, "%s: the slug %q moved to %q", field, slug, to)
			return
		}
		r.add("E01", Block, p, 0, "%s: no node has the slug %q", field, slug)
		return
	}
	if len(kinds) > 0 && !slices.Contains(kinds, t.Kind) {
		rule := map[string]string{"solves": "E05", "implements": "E02"}[field]
		if rule == "" {
			rule = "E01"
		}
		r.add(rule, Block, p, 0, "%s: %q is a %s, not a %s", field, slug, t.Kind, strings.Join(kinds, " or "))
	}
}

func (r *run) edges(n *node.Node) {
	for _, s := range n.Solves {
		r.target(n, "solves", s.Problem, "task")
	}
	for _, s := range n.Implements {
		r.target(n, "implements", s, "method", "architecture")
	}
	for _, s := range n.Techniques {
		r.target(n, "techniques", s, "method", "concept")
	}
	for _, s := range n.Uses {
		r.target(n, "uses", s)
	}
	for _, s := range n.Prereqs {
		r.target(n, "prereqs", s)
	}
	for _, s := range n.Generalizes {
		r.target(n, "generalizes", s)
	}
	if n.VariantOf != "" {
		r.target(n, "variant_of", n.VariantOf)
	}
	for _, s := range n.SeeAlso {
		r.target(n, "see_also", s)
	}
	for _, s := range n.People {
		r.target(n, "people", s, "person")
	}
	if len(n.SeeAlso) > 8 {
		r.add("E08", Warn, n.Rel+"/node.md", 0, "see_also has %d targets; the limit is 8", len(n.SeeAlso))
	}
	if n.Model != "" {
		r.target(n, "model", n.Model, "concept")
	}
}

// cycles reports each cycle of the edges that next gives, once.
func (r *run) cycles(rule, what string, next func(*node.Node) []string) {
	const (
		white = iota
		grey
		black
	)
	colour := map[string]int{}
	var stack []string
	var visit func(s string)
	visit = func(s string) {
		colour[s] = grey
		stack = append(stack, s)
		n := r.c.BySlug[s]
		for _, t := range next(n) {
			if _, ok := r.c.BySlug[t]; !ok {
				continue
			}
			switch colour[t] {
			case white:
				visit(t)
			case grey:
				i := slices.Index(stack, t)
				cyc := append(slices.Clone(stack[i:]), t)
				r.add(rule, Block, r.c.BySlug[t].Rel+"/node.md", 0, "the %s edges have a cycle: %s", what, strings.Join(cyc, " -> "))
			}
		}
		stack = stack[:len(stack)-1]
		colour[s] = black
	}
	for _, n := range r.c.Nodes {
		if colour[n.Slug] == white {
			visit(n.Slug)
		}
	}
}

func sortedKeys[V any](m map[string]V) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	slices.Sort(ks)
	return ks
}

func (r *run) complexity(n *node.Node) {
	c := n.Complexity
	if c == nil {
		return
	}
	p := n.Rel + "/node.md"
	groups := map[string]map[string]string{"time": c.Time, "space": c.Space}
	for k, v := range c.Other {
		groups[k] = v
	}
	for _, g := range sortedKeys(groups) {
		for _, k := range sortedKeys(groups[g]) {
			v := groups[g][k]
			if err := ParseClaim(v); err != nil {
				r.add("S11", Block, p, 0, "complexity.%s.%s %q: %v", g, k, v, err)
				continue
			}
			for _, x := range Vars(v) {
				if _, ok := c.Vars[x]; !ok {
					r.add("S11", Block, p, 0, "complexity.%s.%s uses %q, which vars does not define", g, k, x)
				}
			}
		}
	}
	if len(c.Time) > 0 && n.Model == "" && (n.Kind == "method" || n.Kind == "architecture") {
		r.add("S11", Block, p, 0, "the node has complexity claims but no model")
	}
}

// Order lists the headings of each kind in their order (spec 06.3).
var Order = func() map[string][]string {
	out := map[string][]string{}
	for _, k := range node.KindTable {
		if !k.CardOnly() {
			out[k.ID] = k.Sections
		}
	}
	return out
}()

func (r *run) sections(n *node.Node) {
	order, ok := Order[n.Kind]
	if !ok {
		return
	}
	p := n.Rel + "/node.md"
	last := -1
	// The build fills the generated sections, so an empty heading only marks the place.
	k, _ := node.Kind(n.Kind)
	filled := map[string]bool{}
	for _, h := range k.Generated {
		filled[h] = true
	}
	for _, h := range status.Sections(n.Body) {
		i := slices.Index(order, h)
		if i < 0 {
			r.add("S04", Warn, p, 0, "the heading %q is not a section of the kind %s (06.3)", h, n.Kind)
			continue
		}
		if i < last {
			r.add("S04", Warn, p, 0, "the heading %q must come before %q", h, order[last])
		}
		if status.Section(n.Body, h) == "" && !filled[h] {
			r.add("S04", Warn, p, 0, "the section %q is empty; leave it out", h)
		}
		last = max(last, i)
	}
}

// PseudoIDs returns the line ids of the pseudocode blocks of a body, by block id. A block with no id is "main".
func PseudoIDs(body string) (map[string][]string, []Finding) {
	out := map[string][]string{}
	var bad []Finding
	in, block := false, ""
	for _, l := range strings.Split(body, "\n") {
		t := strings.TrimSpace(l)
		if !in && strings.HasPrefix(t, "```pseudo") {
			in, block = true, "main"
			if i := strings.Index(t, "id="); i >= 0 {
				block = strings.Fields(t[i+3:])[0]
			}
			continue
		}
		if in && strings.HasPrefix(t, "```") {
			in = false
			continue
		}
		if in {
			if m := idLineRE.FindStringSubmatch(l); m != nil {
				if slices.Contains(out[block], m[1]) {
					bad = append(bad, Finding{Rule: "X11", Severity: Block, Msg: fmt.Sprintf("the line id %s is twice in the block %s", m[1], block)})
				}
				out[block] = append(out[block], m[1])
			}
		}
	}
	return out, bad
}

func (r *run) pseudo(n *node.Node) {
	_, bad := PseudoIDs(n.Body)
	for _, f := range bad {
		r.add(f.Rule, f.Severity, n.Rel+"/node.md", 0, "%s", f.Msg)
	}
}

// Markers returns the line ids that the @line markers of a file name.
func Markers(content string) []string {
	var out []string
	for _, m := range markerRE.FindAllStringSubmatch(content, -1) {
		for id := range strings.FieldsFuncSeq(m[1], func(r rune) bool { return r == ' ' || r == ',' }) {
			out = append(out, id)
		}
	}
	return out
}

// mutantsIgnore checks mutants.ignore of the implementation (rule X10): each entry has a reason and names a mutant of the sample, and the list has at most 5% of the mutants.
func (r *run) mutantsIgnore(im node.Impl) {
	var file *node.File
	for i := range im.Files {
		if im.Files[i].Name == mutate.IgnoreFile {
			file = &im.Files[i]
		}
	}
	if file == nil {
		return
	}
	set, err := mutate.Of(im, mutate.MaxPerFile)
	if err != nil {
		r.add("X10", Block, file.Path, 0, "%v", err)
		return
	}
	for _, e := range set.Stale {
		r.add("X10", Block, file.Path, e.Line, "the entry names no mutant of the sample; the code changed, or the id is not as aik test -kind mutate writes it: %s", e.ID)
	}
	if k, all := set.Ignored(), len(set.Mutants); float64(k) > mutate.MaxIgnored*float64(all) {
		r.add("X10", Block, file.Path, 0, "the list has %d of %d mutants, and at most 5%% can be marked equivalent (09.6); add tests that kill some of them", k, all)
	}
}

func (r *run) impls(n *node.Node) {
	ids, _ := PseudoIDs(n.Body)
	all := map[string]bool{}
	for _, b := range ids {
		for _, id := range b {
			all[id] = true
		}
	}
	for _, im := range n.Impls {
		lang := r.c.LangByID[im.Lang]
		if lang == nil {
			r.add("X12", Level, n.Rel+"/impl/"+im.Lang, 0, "the framework %q is not in %s (8.2.1)", im.Lang, r.c.LangFile)
		} else if r.c.Frameworks {
			r.framework(n, im, lang)
		}
		r.mutantsIgnore(im)
		seen := map[string]bool{}
		for _, f := range im.Files {
			if node.IsBuildFile(f.Name) {
				r.add("X12", Block, f.Path, 0, "%s is a build file; aik gen writes it in build/lang (08.5)", path.Base(f.Name))
			}
			if f.Name == mutate.IgnoreFile {
				continue
			}
			if !r.c.Frameworks && !strings.Contains(firstLines(f.Content, 4), "SPDX-License-Identifier: MIT") {
				r.add("X19", Level, f.Path, 1, "the file does not start with the SPDX header (8.12)")
			}
			if f.Test {
				continue
			}
			for _, id := range Markers(f.Content) {
				seen[id] = true
				if !all[id] {
					r.add("X11", Block, f.Path, 0, "the marker @line %s names no line id of the pseudocode", id)
				}
			}
		}
		if lang != nil && lang.Tier == 1 {
			var miss []string
			for _, id := range ids["main"] {
				if !seen[id] {
					miss = append(miss, id)
				}
			}
			if len(miss) > 0 {
				r.add("X11", Warn, n.Rel+"/impl/"+im.Lang, 0, "no @line marker for %s", strings.Join(miss, ", "))
			}
		}
	}
	if n.Scene != "" {
		b, err := os.ReadFile(filepath.Join(r.root, n.Scene))
		if err == nil {
			for _, m := range regexp.MustCompile(`\.line\(\s*["'](L\d+)["']`).FindAllStringSubmatch(string(b), -1) {
				if !all[m[1]] {
					r.add("X11", Block, n.Scene, 0, "v.line(%q) names no line id of the pseudocode", m[1])
				}
			}
		}
	}
}

func firstLines(s string, k int) string {
	for i, c := range s {
		if c == '\n' {
			k--
			if k == 0 {
				return s[:i]
			}
		}
	}
	return s
}

// linkRE finds the target of a Markdown link that starts with "/".
var linkRE = regexp.MustCompile(`\]\((/[^)\s#?]*)`)

// links is the part of L01 that the audit can do without a build: a link to /<x>/ must name a node, an area or a site page. aik site check does the rest.
func (r *run) links(n *node.Node) {
	files := []struct {
		path string
		body string
		line int
	}{{n.Rel + "/node.md", n.Body, n.BodyLine}}
	for _, pr := range n.Proofs {
		files = append(files, struct {
			path string
			body string
			line int
		}{pr.Path, pr.Body, 0})
	}
	for _, f := range files {
		for _, m := range linkRE.FindAllStringSubmatchIndex(f.body, -1) {
			target := f.body[m[2]:m[3]]
			parts := strings.Split(strings.Trim(target, "/"), "/")
			if target == "/" || len(parts) != 1 || r.known(parts[0]) {
				continue
			}
			line := 0
			if f.line > 0 {
				line = f.line + strings.Count(f.body[:m[0]], "\n")
			}
			r.add("L01", Block, f.path, line, "the link %s names no node, area or site page", target)
		}
	}
}

func (r *run) known(x string) bool {
	if r.c.BySlug[x] != nil || r.c.Reserved[x] {
		return true
	}
	for _, a := range r.c.AreaByID {
		if a.ID == x {
			return true
		}
	}
	return false
}

// oldSlugs checks that each line of registers/aliases.reg points to a node, so no redirect goes to a missing page.
func (r *run) oldSlugs() {
	olds := make([]string, 0, len(r.c.OldSlugs))
	for o := range r.c.OldSlugs {
		olds = append(olds, o)
	}
	sort.Strings(olds)
	for _, o := range olds {
		if to := r.c.OldSlugs[o]; r.c.BySlug[to] == nil {
			r.add("T03", Block, "registers/aliases.reg", 0, "the old slug %q points to %q, but no node has that slug", o, to)
		}
	}
}

// committedBuild finds files under build/ that git tracks (rule X12). The check runs only in a git work tree.
func (r *run) committedBuild() {
	out, err := exec.Command("git", "-C", r.root, "ls-files", "-z", "--", "build").Output()
	if err != nil {
		return
	}
	for _, p := range strings.Split(string(out), "\x00") {
		if p != "" {
			r.add("X12", Block, p, 0, "the file is under build/, which only the build writes; remove it with git rm --cached")
		}
	}
}

// groups checks that each top area has one of the 9 groups of spec 05.2 and that no sub-area has a group.
func (r *run) groups() {
	const p = "topics/areas.yaml"
	for _, a := range r.c.Areas {
		switch {
		case a.Group == "":
			r.add("T01", Block, p, 0, "the top area %q has no group; give it a letter from A to I (spec 05.2)", a.ID)
		case node.GroupByLetter(a.Group) == nil:
			r.add("T01", Block, p, 0, "the top area %q has the group %q; the groups are the letters A to I (spec 05.2)", a.ID, a.Group)
		}
		for _, s := range a.Children {
			if s.Group != "" {
				r.add("T02", Block, p, 0, "the sub-area %q has a group; only a top area has one", s.Path())
			}
		}
	}
}
