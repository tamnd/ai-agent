// Package status computes the ladders of a node (spec 04.10): H for the human proof, L for Lean, I for each language, V for the visualization, W for the writing and D for the data facts.
//
// Nobody writes a level by hand. Each level comes with the reasons that it is not higher, so that a page can say what it needs next.
package status

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/tamnd/ai-agent/formal"
	"github.com/tamnd/ai-agent/node"
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
	// D is the ladder of the data facts. It is nil for a kind that carries no facts.
	D     *Rung           `json:"d,omitempty"`
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

// Review is a review record, reviews/<tag>/<part>.yaml (spec 13.2).
type Review struct {
	Part     string `yaml:"part"`
	Hash     string `yaml:"hash"`
	Reviewer string `yaml:"reviewer"`
	Date     string `yaml:"date"`
	Verdict  string `yaml:"verdict"`
}

// Env is what the ladders read besides the node.
type Env struct {
	Corpus  *node.Corpus
	Results map[string]map[string]Test // slug, then lang
	// VizChecked holds the scenes that passed `aik viz check` in this build, by slug, with the failures of the others.
	VizChecked map[string][]string
	// Decls holds the declarations that `aik lean build` extracted, or nil when it did not run.
	Decls formal.Decls
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

// Hash is the hash that a review binds to: sha256 of the file, in hex.
func Hash(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:]), nil
}

// Reviewed tells if reviews/<tag>/<part>.yaml exists, approves, and binds to the current hash of file.
func Reviewed(root, tag, part, file string) (bool, string) {
	if tag == "" {
		return false, "the node has no tag yet"
	}
	b, err := os.ReadFile(filepath.Join(root, "reviews", tag, part+".yaml"))
	if errors.Is(err, os.ErrNotExist) {
		return false, "no review record reviews/" + tag + "/" + part + ".yaml"
	}
	if err != nil {
		return false, err.Error()
	}
	var r Review
	if err := yaml.Unmarshal(b, &r); err != nil {
		return false, "the review record does not parse"
	}
	h, err := Hash(file)
	if err != nil {
		return false, err.Error()
	}
	if r.Hash != h {
		return false, "the review is for an older version of the file"
	}
	if r.Verdict != "" && r.Verdict != "approve" {
		return false, "the review does not approve"
	}
	return true, ""
}

// Of computes the ladders of n.
func Of(n *node.Node, env Env) Status {
	s := Status{Slug: n.Slug, I: map[string]Rung{}, Tests: map[string]Test{}}
	s.H = human(n, env)
	s.L = lean(n, env)
	for _, im := range n.Impls {
		s.I[im.Lang] = impl(n, im, env)
		if t, ok := env.Results[n.Slug][im.Lang]; ok {
			s.Tests[im.Lang] = t
		}
	}
	s.V = visual(n, env)
	s.W = Writing(n, env)
	s.D = Data(n, env)
	return s
}

var hNames = []string{"no proof", "sketch", "full proof", "reviewed proof"}

func human(n *node.Node, env Env) (r Rung) {
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
		if ok, why := Reviewed(root, n.Tag, "proof-"+p.ID, filepath.Join(root, p.Path)); !ok {
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

var lNames = []string{"no Lean part", "spec and model", "correctness proved", "cost proved", "refinement proved"}

func lean(n *node.Node, env Env) (r Rung) {
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
		if ok, why := Reviewed(env.Corpus.Root, n.Tag, "lean-spec", file); !ok {
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
	if n.Scene == "" && len(figs) == 0 {
		r.Next = []string{"add viz/scene.ts or a figure (10)"}
		return r
	}
	if n.Scene == "" {
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
		return func() bool { return len(listItem.FindAllString(Section(n.Body, h), -1)) >= k }
	}
	linked := func() bool {
		for _, p := range n.Prereqs {
			if _, ok := env.Corpus.BySlug[p]; !ok {
				return false
			}
		}
		return true
	}
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
	levels[3] = []need{{"at least three sources", sources(3)}, {"every prerequisite exists", linked}}
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
	if ok, why := Reviewed(env.Corpus.Root, n.Tag, "text", file); !ok {
		r.Level = 4
		r.Next = []string{"a review of the text: " + why}
		return r
	}
	r.Level = 5
	return r
}
