// Package status computes the ladders of a node (spec 04.10): H for the human proof, L for Lean, I for each language, V for the visualization, W for the writing and D for the data facts.
//
// Nobody writes a level by hand. Each level comes with the reasons that it is not higher, so that a page can say what it needs next.
package status

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/tamnd/ai-agent/bridge"
	"github.com/tamnd/ai-agent/cite"
	"github.com/tamnd/ai-agent/formal"
	"github.com/tamnd/ai-agent/node"
	"github.com/tamnd/ai-agent/review"
)

// Rung is the level of one ladder, with the reasons that it is not higher.
type Rung struct {
	Level int      `json:"level"`
	Max   int      `json:"max"`
	Label string   `json:"label"` // for example "H2"
	Name  string   `json:"name"`  // for example "full proof"
	Next  []string `json:"next,omitempty"`
}

// Status is the set of ladders of one node.
type Status struct {
	Slug string          `json:"slug"`
	H    Rung            `json:"h"`
	L    Rung            `json:"l"`
	I    map[string]Rung `json:"i"`
	V    Rung            `json:"v"`
	W    Rung            `json:"w"`
	// G is the ladder of the grounding of the claims (04.10).
	G Rung `json:"g"`
	// D is the ladder of the data facts. It is nil for a kind that carries no facts.
	D *Rung `json:"d,omitempty"`
	// R is the ladder of the experiments (spec 9.11). It is nil for a node that has no experiment and no claim with a small-scale form.
	R     *Rung           `json:"r,omitempty"`
	Tests map[string]Test `json:"tests,omitempty"`
}

// Test is the result of the native tests of one implementation, as `aik test` writes it.
type Test struct {
	Slug string `json:"slug"`
	Lang string `json:"lang"`
	// Pass tells if the native tests pass.
	Pass   bool   `json:"pass"`
	Output string `json:"output,omitempty"`
	Ms     int64  `json:"ms"`
	// Shared holds the results of the shared cases of each problem that the node solves (spec 09.2).
	Shared []Shared `json:"shared,omitempty"`
	// Diff holds the results of the differential run of each problem that the node solves (spec 09.5). It comes from the file <lang>.diff.json.
	Diff []Diff `json:"diff,omitempty"`
	// Mutation is the result of the mutation run (spec 09.6). It comes from the file <lang>.mutate.json.
	Mutation *Mutation `json:"mutation,omitempty"`
	// Passed and Failed are the names of the native tests of a framework version, from the JUnit report of pytest,
	// with the parameters cut: test_shapes[base] is test_shapes. A name is in Passed when every run of it passed.
	// X2 looks for the tests of its kind by name (08.10).
	Passed []string `json:"passed,omitempty"`
	Failed []string `json:"failed,omitempty"`
	// Vectors is the run of the metric's test vectors through the score function (08.7.5, rule X05).
	Vectors *Vectors `json:"vectors,omitempty"`
	// Lint is the format, lint and type check of a framework version (8.3.2, rule X13).
	Lint []Lint `json:"lint,omitempty"`
}

// Vectors is the result of the test vectors of a metric on one framework version.
type Vectors struct {
	Cases    int           `json:"cases"`
	Passed   int           `json:"passed"`
	Failures []CaseFailure `json:"failures,omitempty"`
	// Error is set when the vectors could not run: no contract, a bad vector file, or a score function that does not load.
	Error string `json:"error,omitempty"`
}

// OK tells if every vector ran and passed.
func (v Vectors) OK() bool {
	return v.Error == "" && len(v.Failures) == 0 && v.Passed == v.Cases && v.Cases > 0
}

// Lint is one check of the code: ruff format, ruff check or pyright.
type Lint struct {
	Tool string `json:"tool"`
	Pass bool   `json:"pass"`
	// Missing is set when the tool is not installed, in the image or on this machine.
	Missing bool   `json:"missing,omitempty"`
	Output  string `json:"output,omitempty"`
}

// Mutation is the result of the mutation run of one implementation.
type Mutation struct {
	// Mutants is the number of mutants of the sample, Ignored the number of them in mutants.ignore, and Killed the number of the others that the tests kill.
	Mutants int `json:"mutants"`
	Killed  int `json:"killed"`
	Ignored int `json:"ignored,omitempty"`
	// Survivors are the ids of the mutants that no test kills, as mutants.ignore writes them.
	Survivors []string `json:"survivors,omitempty"`
	// Error is set when the run could not start, or when the code fails its tests with no mutant.
	Error string `json:"error,omitempty"`
}

// MinScore is the mutation score of rule I04.
const MinScore = 0.85

// Score is the killed mutants divided by the mutants that are not ignored. A run with no mutant to count has the score 1.
func (m Mutation) Score() float64 {
	if m.Mutants-m.Ignored <= 0 {
		return 1
	}
	return float64(m.Killed) / float64(m.Mutants-m.Ignored)
}

// Diff is the result of the differential run of one problem on one implementation.
type Diff struct {
	Problem string `json:"problem"`
	Cases   int    `json:"cases"`
	Passed  int    `json:"passed"`
	Skipped int    `json:"skipped,omitempty"`
	// Failures holds the shrunk failures, at most lang.MaxDiffFailures.
	Failures []CaseFailure `json:"failures,omitempty"`
	// Error is set when the run could not start, for example when the adapter does not build.
	Error string `json:"error,omitempty"`
}

// Shared is the result of the shared cases of one problem on one implementation.
type Shared struct {
	Problem    string `json:"problem"`
	Cases      int    `json:"cases"`
	Passed     int    `json:"passed"`
	Skipped    int    `json:"skipped,omitempty"`
	SkipReason string `json:"skip_reason,omitempty"`
	// SkipExpected is true when the language cannot run the skipped cases by design, as C cannot run T = string. Such a skip does not hold back I1.
	SkipExpected bool          `json:"skip_expected,omitempty"`
	Failures     []CaseFailure `json:"failures,omitempty"`
	// Error is set when the cases could not run, for example when the adapter does not build.
	Error string `json:"error,omitempty"`
}

// OK tells if the native tests and the shared cases pass.
func (t Test) OK() bool {
	for _, s := range t.Shared {
		if !s.OK() {
			return false
		}
	}
	if t.Vectors != nil && !t.Vectors.OK() {
		return false
	}
	return t.Pass
}

// OK tells if every case that ran passed.
func (s Shared) OK() bool { return s.Error == "" && len(s.Failures) == 0 }

// CaseFailure is one shared case that failed, with the reason.
type CaseFailure struct {
	Case   string `json:"case"`
	Reason string `json:"reason"`
	Got    string `json:"got,omitempty"`
	Want   string `json:"want,omitempty"`
}

// Results is the content of one file build/results/<lang>.json.
type Results struct {
	Lang    string `json:"lang"`
	Version string `json:"version,omitempty"`
	// Kind is "diff" for the file <lang>.diff.json of the differential run, "mutate" for the file <lang>.mutate.json of the mutation run, and empty for the native tests and the shared cases.
	Kind  string `json:"kind,omitempty"`
	Tests []Test `json:"tests"`
}

// Env is what the ladders read besides the node.
type Env struct {
	Corpus  *node.Corpus
	Results map[string]map[string]Test // slug, then lang
	// VizChecked holds the scenes that passed `aik viz check` in this build, by slug, with the failures of the others.
	VizChecked map[string][]string
	// Decls holds the declarations that `aik lean build` extracted, or nil when it did not run.
	Decls formal.Decls
	// Cite holds the snapshots and the records of the citation check (13.6), or nil when there are none.
	Cite *cite.Index
}

// LoadDecls reads decls.jsonl. A missing file gives nil, which means that the Lean build did not run.
func LoadDecls(path string) (formal.Decls, error) {
	d, err := formal.Load(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return d, err
}

// LoadResults reads every build/results/*.json under dir. A missing dir gives no results.
func LoadResults(dir string) (map[string]map[string]Test, error) {
	out := map[string]map[string]Test{}
	var diffs []Results
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		var r Results
		if err := json.Unmarshal(b, &r); err != nil {
			return nil, fmt.Errorf("%s: %w", f, err)
		}
		if r.Kind == "diff" || r.Kind == "mutate" {
			diffs = append(diffs, r)
			continue
		}
		for _, t := range r.Tests {
			if out[t.Slug] == nil {
				out[t.Slug] = map[string]Test{}
			}
			out[t.Slug][t.Lang] = t
		}
	}
	// The differential and the mutation runs add to the results of the native tests. A run without native results does not count as a test run.
	for _, r := range diffs {
		for _, t := range r.Tests {
			base, ok := out[t.Slug][t.Lang]
			if !ok {
				continue
			}
			if r.Kind == "diff" {
				base.Diff = t.Diff
			} else {
				base.Mutation = t.Mutation
			}
			out[t.Slug][t.Lang] = base
		}
	}
	return out, nil
}

// Reviewed tells whether the review record of the part of n counts for file (spec 15.5), and why not.
func Reviewed(root string, n *node.Node, part, file string) (bool, string) {
	return review.Check(root, n, part, file)
}

// Of computes the ladders of n.
func Of(n *node.Node, env Env) Status {
	s := Status{Slug: n.Slug, I: map[string]Rung{}, Tests: map[string]Test{}}
	s.H = human(n, env)
	s.L = lean(n, env)
	for _, im := range n.Impls {
		if Frameworks[im.Lang] {
			s.I[im.Lang] = xImpl(n, im, env)
		} else {
			s.I[im.Lang] = impl(n, im, env)
		}
		if t, ok := env.Results[n.Slug][im.Lang]; ok {
			s.Tests[im.Lang] = t
		}
	}
	s.V = visual(n, env)
	s.W = Writing(n, env)
	s.G = Grounding(n, env)
	s.D = Data(n, env)
	s.R = Repro(n, env)
	return s
}

var hNames = []string{"no proof", "sketch", "full proof", "reviewed proof"}

func human(n *node.Node, env Env) (r Rung) {
	if Formal(n) {
		return theoremProof(n, env)
	}
	r = Rung{Max: 3}
	defer func() { r.Label, r.Name = fmt.Sprintf("H%d", r.Level), hNames[r.Level] }()
	if len(n.Proofs) == 0 {
		r.Next = []string{"add a proof in proofs/ (06.5)"}
		return r
	}
	r.Level = 1
	proves := map[string]bool{}
	var full []node.Proof
	for _, p := range n.Proofs {
		if p.Front.Status != "full" {
			continue
		}
		full = append(full, p)
		for _, c := range p.Front.Proves {
			proves[c] = true
		}
	}
	var need []string
	if !proves["correctness"] {
		need = append(need, "a full proof of correctness")
	}
	for _, c := range claims(n) {
		if !proves[c] && !proves["complexity"] {
			need = append(need, "a full proof of "+c)
		}
	}
	if len(need) > 0 {
		r.Next = need
		return r
	}
	r.Level = 2
	root := env.Corpus.Root
	for _, p := range full {
		if ok, why := Reviewed(root, n, "proof-"+p.ID, filepath.Join(root, p.Path)); !ok {
			r.Next = append(r.Next, p.ID+": "+why)
		}
	}
	if len(r.Next) == 0 {
		r.Level = 3
	}
	return r
}

// claims lists the complexity claims of the front matter as "time.worst", "space.extra" and so on.
func claims(n *node.Node) []string {
	c := n.Complexity
	if c == nil {
		return nil
	}
	var out []string
	add := func(group string, m map[string]string) {
		for _, k := range sortedKeys(m) {
			out = append(out, group+"."+k)
		}
	}
	add("time", c.Time)
	add("space", c.Space)
	for _, g := range sortedKeys(c.Other) {
		add(g, c.Other[g])
	}
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	slices.Sort(ks)
	return ks
}

// Formal tells if the Lean part of a node is a statement and its proof, on the F ladder of 04.10, rather than a model,
// a spec and the theorems about them: a theorem node, and a concept or a metric that names lean.statement and no model.
func Formal(n *node.Node) bool {
	return n.Kind == "theorem" || (n.Lean["statement"] != "" && n.Lean["model"] == "" && n.Lean["spec"] == "")
}

// theoremProof is the H ladder of a theorem: its proof is the derivation of kind proof (06.5), a sketch at H1, a full
// proof at H2, and a full proof that a person reviewed at H3.
func theoremProof(n *node.Node, env Env) (r Rung) {
	r = Rung{Max: 3}
	defer func() { r.Label, r.Name = fmt.Sprintf("H%d", r.Level), hNames[r.Level] }()
	var proofs, full []node.Derivation
	for _, d := range n.Derivations {
		if d.Front.Kind == "proof" {
			proofs = append(proofs, d)
			if d.Front.Status == "full" {
				full = append(full, d)
			}
		}
	}
	if len(proofs) == 0 {
		r.Next = []string{"add derivations/proof.md, a derivation of kind proof (06.5)"}
		return r
	}
	r.Level = 1
	if len(full) == 0 {
		r.Next = []string{proofs[0].ID + " is a sketch; a full proof covers every step (06.5)"}
		return r
	}
	r.Level = 2
	root := env.Corpus.Root
	for _, d := range full {
		if ok, why := Reviewed(root, n, "proof-"+d.ID, filepath.Join(root, filepath.FromSlash(d.Path))); !ok {
			r.Next = append(r.Next, d.ID+": "+why)
		}
	}
	if len(r.Next) < len(full) {
		r.Level = 3
		r.Next = nil
	}
	return r
}

var fNames = []string{"no Lean part", "statement checked", "proved", "steps linked"}

// formalRung is the F ladder of 04.10 and 11: F1 when the statement builds, passes the rules of the lean field and, on
// a core node, has a fidelity review (11.7); F2 when its proof uses only the allowed axioms, so no sorry (11.5); F3
// when the steps of the page proof are linked to the Lean proof (11.6).
func formalRung(n *node.Node, env Env) (r Rung) {
	r = Rung{Max: 3}
	defer func() { r.Label, r.Name = fmt.Sprintf("F%d", r.Level), fNames[r.Level] }()
	mod := n.Lean["module"]
	if mod == "" {
		r.Next = []string{"name a Lean module and the theorem in lean.module and lean.statement (11.4)"}
		return r
	}
	file := filepath.Join(env.Corpus.Root, "lean", filepath.FromSlash(strings.ReplaceAll(mod, ".", "/"))+".lean")
	if _, err := os.Stat(file); err != nil {
		r.Next = []string{"the module " + mod + " does not exist"}
		return r
	}
	stmt := n.Lean["statement"]
	if stmt == "" {
		r.Next = []string{"name the theorem in lean.statement (11.4)"}
		return r
	}
	if env.Decls == nil {
		r.Next = []string{"run `aik lean build` to check the Lean part"}
		return r
	}
	var need []string
	for _, f := range formal.CheckNode(n, env.Decls) {
		need = append(need, f.Where+": "+f.Msg+" ("+f.Rule+")")
	}
	if n.Level == "core" {
		if ok, why := Reviewed(env.Corpus.Root, n, "lean-statement", file); !ok {
			need = append(need, "a fidelity review of the statement: "+why)
		}
	}
	if len(need) > 0 {
		r.Next = need
		return r
	}
	r.Level = 1
	if bad := env.Decls[stmt].Disallowed(); len(bad) > 0 {
		r.Next = []string{stmt + " uses the axiom " + strings.Join(bad, ", ") + " (F03)"}
		return r
	}
	r.Level = 2
	page, steps, ok, err := formal.NodeSteps(env.Corpus.Root, n, env.Decls)
	if !ok {
		r.Next = []string{"F3 links the steps of the page proof to the Lean proof: name it in lean.steps, give its steps ids {#step-…}, and mark them with -- step: comments, which `aik lean steps` checks (11.6, F08)"}
		return r
	}
	if err != nil {
		r.Next = []string{"lean.steps: " + err.Error()}
		return r
	}
	for _, f := range formal.CheckSteps(n, page, steps) {
		need = append(need, f.Msg+" (F08)")
	}
	if n.Level == "core" {
		proof := filepath.Join(n.Dir, filepath.FromSlash(n.Lean["steps"]))
		if ok, why := Reviewed(env.Corpus.Root, n, "lean-statement", proof); !ok {
			need = append(need, "a review of the steps, with "+n.Lean["steps"]+" in the hashes of the lean-statement review: "+why)
		}
	}
	if len(need) > 0 {
		r.Next = need
		return r
	}
	r.Level = 3
	return r
}

var lNames = []string{"no Lean part", "spec and model", "correctness proved", "cost proved", "refinement proved"}

func lean(n *node.Node, env Env) (r Rung) {
	if Formal(n) {
		return formalRung(n, env)
	}
	r = Rung{Max: 4}
	defer func() { r.Label, r.Name = fmt.Sprintf("L%d", r.Level), lNames[r.Level] }()
	mod := n.Lean["module"]
	if mod == "" {
		r.Next = []string{"name a Lean module in the lean field (07)"}
		return r
	}
	file := filepath.Join(env.Corpus.Root, "lean", filepath.FromSlash(strings.ReplaceAll(mod, ".", "/"))+".lean")
	if _, err := os.Stat(file); err != nil {
		r.Next = []string{"the module " + mod + " does not exist"}
		return r
	}
	if env.Decls == nil {
		r.Next = []string{"run `aik lean build` to check the Lean part"}
		return r
	}
	ds := env.Decls
	var need []string
	for _, f := range formal.CheckNode(n, ds) {
		// F02 is the rule of L2, and the rung of L2 below checks it.
		if f.Rule != "F02" {
			need = append(need, f.Where+": "+f.Msg+" ("+f.Rule+")")
		}
	}
	model, spec := n.Lean["model"], n.Lean["spec"]
	if model == "" {
		need = append(need, "name the model in lean.model")
	}
	if spec == "" {
		need = append(need, "name the spec of the problem in lean.spec")
	}
	if n.Level == "core" {
		if ok, why := Reviewed(env.Corpus.Root, n, "lean-spec", file); !ok {
			need = append(need, "a fidelity review of the spec: "+why)
		}
	}
	if len(need) > 0 {
		r.Next = need
		return r
	}
	r.Level = 1
	proved := func(key, what string) (formal.Decl, string) {
		name := n.Lean[key]
		if name == "" {
			return formal.Decl{}, "name the " + what + " in lean." + key
		}
		d := ds[name]
		if bad := d.Disallowed(); len(bad) > 0 {
			return d, name + " uses the axiom " + strings.Join(bad, ", ") + " (F03)"
		}
		return d, ""
	}
	d, why := proved("correct", "correctness theorem")
	if why == "" && (!strings.Contains(d.Type, model) || !strings.Contains(d.Type, spec)) {
		why = d.Decl + " does not state " + spec + " of " + model + " (F02)"
	}
	if why != "" {
		r.Next = []string{why}
		return r
	}
	r.Level = 2
	if _, why := proved("cost", "cost theorem"); why != "" {
		r.Next = []string{why}
		return r
	}
	if !hasClassLemma(ds, mod) {
		r.Next = []string{"a class lemma in " + mod + " that links the cost bound to its class with IsBigO (F01)"}
		return r
	}
	r.Level = 3
	r.Next = []string{"L4 needs the refinement theorem for the Rust implementation (L08), which is M3 work"}
	return r
}

// hasClassLemma tells if the module has a theorem that states an asymptotic class with the standard axioms only.
func hasClassLemma(ds formal.Decls, mod string) bool {
	for _, d := range ds {
		if d.Module == mod && d.Kind == "theorem" && len(d.Disallowed()) == 0 && formal.IsClassLemma(d.Type) {
			return true
		}
	}
	return false
}

var iNames = []string{"not checked yet", "shared cases pass", "differential and mutation", "clean and measured", "reviewed"}

func impl(n *node.Node, im node.Impl, env Env) (r Rung) {
	r = Rung{Max: 4}
	defer func() { r.Label, r.Name = fmt.Sprintf("I%d", r.Level), iNames[r.Level] }()
	t, ran := env.Results[n.Slug][im.Lang]
	switch {
	case !im.HasTests():
		r.Next = append(r.Next, "add native tests (09.9)")
	case !ran:
		r.Next = append(r.Next, "the tests did not run in this build")
	case len(t.Shared) == 0:
		if !t.Pass {
			r.Next = append(r.Next, "the native tests fail")
		}
		r.Next = append(r.Next, "no problem that the node solves has a contract with shared cases (09.2)")
	default:
		ok := true
		for _, s := range t.Shared {
			switch {
			case s.Error != "":
				r.Next = append(r.Next, "the shared cases of "+s.Problem+" did not run: "+firstLine(s.Error))
			case len(s.Failures) > 0:
				r.Next = append(r.Next, fmt.Sprintf("%d of %d shared cases of %s fail, the first is %s", len(s.Failures), s.Cases, s.Problem, s.Failures[0].Case))
			case s.Skipped > 0 && !s.SkipExpected:
				r.Next = append(r.Next, fmt.Sprintf("%d shared cases of %s did not run: %s", s.Skipped, s.Problem, firstLine(s.SkipReason)))
			default:
				continue
			}
			ok = false
		}
		if ok && !t.Pass {
			r.Next = append(r.Next, "the native tests fail")
			ok = false
		}
		if ok {
			r.Level = 1
		}
	}
	if r.Level == 1 {
		next := diffNext(t)
		if next == nil {
			next = mutationNext(t)
		}
		if next == nil {
			r.Level = 2
			next = []string{"I3 needs the formatter, the linter, the @line markers and the complexity check (09.7), which are not checked yet"}
		}
		r.Next = append(r.Next, next...)
	}
	return r
}

// mutationNext says what an implementation whose differential run passes needs for I2: a mutation score of at least 85% (rule I04). It gives nil when the score is enough.
func mutationNext(t Test) []string {
	m := t.Mutation
	switch {
	case m == nil:
		return []string{"I2 needs a mutation score of at least 85% (09.6, I04); the mutation run did not run in this build"}
	case m.Error != "":
		return []string{"the mutation run did not run: " + firstLine(m.Error)}
	case m.Score() < MinScore:
		msg := fmt.Sprintf("the tests kill %d of %d mutants (%.0f%%), and I2 needs 85%% (I04)", m.Killed, m.Mutants-m.Ignored, 100*m.Score())
		if len(m.Survivors) > 0 {
			msg += "; add a case that kills " + m.Survivors[0]
		}
		return []string{msg}
	}
	return nil
}

// diffNext says what an implementation at I1 needs for I2 from the differential run: a run that passes on each input (rule I03). It gives nil when the run passes.
func diffNext(t Test) []string {
	if len(t.Diff) == 0 {
		return []string{"I2 needs the differential run on the inputs of cases/generated.yaml (09.5); it did not run in this build"}
	}
	var out []string
	for _, d := range t.Diff {
		switch {
		case d.Error != "":
			out = append(out, "the differential run of "+d.Problem+" did not run: "+firstLine(d.Error))
		case len(d.Failures) > 0:
			out = append(out, fmt.Sprintf("the differential run of %s fails on %s (I03); add the shrunk case to cases/regress.jsonl and fix the code", d.Problem, d.Failures[0].Case))
		}
	}
	return out
}

func firstLine(s string) string {
	l, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	return l
}

var vNames = []string{"none", "static frames", "interactive and checked", "reviewed"}

func visual(n *node.Node, env Env) (r Rung) {
	r = Rung{Max: 3}
	defer func() { r.Label, r.Name = fmt.Sprintf("V%d", r.Level), vNames[r.Level] }()
	figs, _ := filepath.Glob(filepath.Join(n.Dir, "figures", "*.svg"))
	if n.Scene == "" && n.ArchScene == "" && len(figs) == 0 {
		if n.Kind == "architecture" && n.Shapes != nil {
			r.Next = []string{"run aik viz shapes -w " + n.Slug + " for the architecture scene (12.6)"}
			return r
		}
		r.Next = []string{"add viz/scene.ts or a figure (12)"}
		return r
	}
	if n.Scene == "" && n.ArchScene == "" {
		r.Level = 1
		r.Next = []string{"add an interactive scene in viz/scene.ts"}
		return r
	}
	fails, checked := env.VizChecked[n.Slug]
	if !checked {
		r.Next = []string{"the scene was not checked in this build"}
		return r
	}
	if len(fails) > 0 {
		r.Next = fails
		return r
	}
	if n.ArchScene != "" && n.Kind == "architecture" {
		// The generated scene of 12.6 is enough for V2 (12.15): it has controls, presets and a text view, and
		// aik viz check ran its numbers against shapes.yaml (V07, V14).
		r.Level = 2
		r.Next = []string{"V3 needs a review of the scene for clarity and accessibility by a person who is not the author (15.4)"}
		return r
	}
	r.Level = 1
	r.Next = []string{"V2 needs the oracle check V03, line-trace agreement V06, the counter check V07 and the budgets V08 (10.8.2)"}
	return r
}

var wNames = []string{"stub", "outline", "basic", "standard", "complete", "reviewed"}

// Sections returns the level 2 headings of the body, in order.
func Sections(body string) []string {
	var out []string
	fence := false
	for _, l := range strings.Split(body, "\n") {
		if strings.HasPrefix(l, "```") || strings.HasPrefix(l, "~~~") {
			fence = !fence
			continue
		}
		if !fence && strings.HasPrefix(l, "## ") {
			out = append(out, strings.TrimSpace(l[3:]))
		}
	}
	return out
}

// Section returns the text of the level 2 section with the heading h, without the heading.
func Section(body, h string) string {
	var b strings.Builder
	in, fence := false, false
	for _, l := range strings.Split(body, "\n") {
		if strings.HasPrefix(l, "```") || strings.HasPrefix(l, "~~~") {
			fence = !fence
		}
		if !fence && strings.HasPrefix(l, "## ") {
			in = strings.TrimSpace(l[3:]) == h
			continue
		}
		if in {
			b.WriteString(l)
			b.WriteByte('\n')
		}
	}
	return strings.TrimSpace(b.String())
}

// Lead returns the text before the first heading.
func Lead(body string) string {
	i := strings.Index(body, "\n## ")
	if strings.HasPrefix(body, "## ") {
		return ""
	}
	if i < 0 {
		return strings.TrimSpace(body)
	}
	return strings.TrimSpace(body[:i])
}

var listItem = regexp.MustCompile(`(?m)^\s*(?:[-*]|\d+\.)\s+\S`)

// callout finds the start of a callout box, such as > [!pitfall] (06.3); each one counts as an item of its section.
var callout = regexp.MustCompile(`(?m)^>\s*\[![a-z]+\]`)

// prereqsExist tells if each prerequisite of n is a node of the corpus or, for slug@al and the like, a neighbour
// that a bridge card names and that the last sync still found (5.9).
func prereqsExist(n *node.Node, c *node.Corpus) bool {
	for _, p := range n.Prereqs {
		if _, _, ok := bridge.Split(p); ok {
			if b := c.Bridges.Find(p); b == nil || b.Retired != "" {
				return false
			}
			continue
		}
		if _, ok := c.BySlug[p]; !ok {
			return false
		}
	}
	return true
}

// need is one requirement of a writing level.
type need struct {
	what string
	ok   func() bool
}

// Writing computes the W ladder from the sections, their content and the links (spec 06.8).
func Writing(n *node.Node, env Env) (r Rung) {
	r = Rung{Max: 5}
	defer func() { r.Label, r.Name = fmt.Sprintf("W%d", r.Level), wNames[r.Level] }()
	has := func(h string) func() bool {
		return func() bool { return Section(n.Body, h) != "" }
	}
	sources := func(k int) func() bool { return func() bool { return len(n.Sources) >= k } }
	located := func() bool {
		for _, s := range n.Sources {
			if s.Locator != "" {
				return true
			}
		}
		return false
	}
	items := func(h string, k int) func() bool {
		return func() bool {
			sec := Section(n.Body, h)
			return len(listItem.FindAllString(sec, -1))+len(callout.FindAllString(sec, -1)) >= k
		}
	}
	linked := func() bool { return prereqsExist(n, env.Corpus) }
	lead := func() bool { return Lead(n.Body) != "" }

	var levels [5][]need
	levels[0] = []need{{"a name", func() bool { return n.Name != "" }}, {"a summary", func() bool { return n.Summary != "" }}}
	k, _ := node.Kind(n.Kind)
	generated := func(h string) bool { return slices.Contains(k.Generated, h) }
	levels[1] = []need{{"the lead paragraph", lead}}
	if slices.Contains(k.Sections, "In plain words") {
		levels[1] = append(levels[1], need{"In plain words", has("In plain words")})
	}
	if m := k.Main(); m != "" && !generated(m) {
		levels[1] = append(levels[1], need{m, has(m)})
	}
	levels[1] = append(levels[1], need{"one source with a locator", located})
	for _, h := range k.Required {
		if !generated(h) && h != k.Main() {
			levels[2] = append(levels[2], need{h, has(h)})
		}
	}
	levels[2] = append(levels[2], need{"at least two sources", sources(2)})
	// The claims of a page are in the right form at W2 and found in their sources at W3 (06.11).
	g := Grounding(n, env).Level
	if len(n.Claims) > 0 {
		levels[2] = append(levels[2], need{"every claim with a source and a locator (G1)", func() bool { return g >= 1 }})
	}
	levels[3] = append(levels[3], need{"the claims in claims.yaml, each found in a source by the citation check (G2)", func() bool { return g >= 2 }})
	levels[3] = append(levels[3], need{"at least three sources", sources(3)}, need{"every prerequisite exists", linked})
	if slices.Contains(k.Sections, "History") {
		levels[3] = append(levels[3], need{"History with a source", func() bool { return has("History")() && n.History != nil }})
	}
	if slices.Contains(k.Sections, "Pitfalls") {
		levels[3] = append(levels[3], need{"Pitfalls with at least two items", items("Pitfalls", 2)})
	}
	// The cross-family audit of spec 15.4 is not recorded yet, so W4 asks for the breadth that the canon needs.
	levels[4] = []need{{"at least five sources", sources(5)}}
	for lv := range 5 {
		var miss []string
		for _, nd := range levels[lv] {
			if !nd.ok() {
				miss = append(miss, nd.what)
			}
		}
		if len(miss) > 0 {
			if lv == 0 {
				r.Level = 0
			} else {
				r.Level = lv - 1
			}
			r.Next = miss
			return r
		}
	}
	file := filepath.Join(n.Dir, "node.md")
	if ok, why := Reviewed(env.Corpus.Root, n, "text", file); !ok {
		r.Level = 4
		r.Next = []string{"a review of the text: " + why}
		return r
	}
	r.Level = 5
	return r
}
