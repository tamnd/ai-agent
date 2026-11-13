package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/tamnd/ai-agent/check"
	"github.com/tamnd/ai-agent/contract"
	"github.com/tamnd/ai-agent/lang"
	"github.com/tamnd/ai-agent/node"
	"github.com/tamnd/ai-agent/site"
	"github.com/tamnd/ai-agent/status"
	"github.com/tamnd/ai-agent/viz"
)

func newFlags(name, args string) *flag.FlagSet {
	fs := flag.NewFlagSet("aik "+name, flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: aik %s %s\n\nFlags:\n", name, args)
		fs.PrintDefaults()
	}
	return fs
}

func isBool(fl *flag.Flag) bool {
	b, ok := fl.Value.(interface{ IsBoolFlag() bool })
	return ok && b.IsBoolFlag()
}

// parse parses the flags. A bad flag is a usage error. The flag package has already printed the error and the usage, so the error has no text.
//
// Flags can come before or after the other arguments, so "aik new heap-sort -area sorting" works. After "--", each argument is a plain argument.
func parse(f *flag.FlagSet, args []string) error {
	var flags, rest []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			rest = append(rest, args[i+1:]...)
			break
		}
		if len(a) < 2 || a[0] != '-' {
			rest = append(rest, a)
			continue
		}
		flags = append(flags, a)
		name := strings.TrimLeft(a, "-")
		if strings.Contains(name, "=") {
			continue
		}
		if fl := f.Lookup(name); fl != nil && !isBool(fl) && i+1 < len(args) {
			flags = append(flags, args[i+1])
			i++
		}
	}
	err := f.Parse(append(append(flags, "--"), rest...))
	if err == nil || errors.Is(err, flag.ErrHelp) {
		return err
	}
	return usageError{}
}

func load(root string) (*node.Corpus, error) {
	if _, err := os.Stat(filepath.Join(root, "nodes")); err != nil {
		return nil, fmt.Errorf("%s has no nodes directory; run aik in a clone of tamnd/ai or use -root", root)
	}
	return node.Load(root)
}

var slugRE = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// cmdNew makes nodes/<area>/<slug>/node.md with the sections of the kind (spec 06.3).
func cmdNew(args []string) error {
	f := newFlags("new", "-area <top/sub> [-name <name>] [<kind>] <slug>")
	root := f.String("root", ".", "the root of the content repository")
	kind := f.String("kind", "method", "the kind of the node")
	area := f.String("area", "", "the area path, for example sorting/comparison")
	name := f.String("name", "", "the display name; the default comes from the slug")
	level := f.String("level", "core", "the level: intro, core, advanced or research")
	if err := parse(f, args); err != nil {
		return err
	}
	var slug string
	switch f.NArg() {
	case 1:
		slug = f.Arg(0)
	case 2:
		*kind, slug = f.Arg(0), f.Arg(1)
	default:
		return usageError{"give a slug, or a kind and a slug"}
	}
	if !slugRE.MatchString(slug) || len(slug) > 60 {
		return usageError{fmt.Sprintf("%q is not lowercase kebab-case of at most 60 characters", slug)}
	}
	if !node.IsKind(*kind) {
		return usageError{fmt.Sprintf("%q is not a kind; use one of %s", *kind, strings.Join(node.Kinds, ", "))}
	}
	if *area == "" {
		return usageError{"give the area with -area"}
	}
	c, err := load(*root)
	if err != nil {
		return err
	}
	if _, ok := c.AreaByID[*area]; !ok {
		return usageError{fmt.Sprintf("the area %q is not in topics/areas.yaml", *area)}
	}
	if c.BySlug[slug] != nil {
		return fmt.Errorf("the slug %q is used by %s", slug, c.BySlug[slug].Rel)
	}
	if c.Reserved[slug] || c.LangByID[slug] != nil || c.AreaByID[slug] != nil {
		return fmt.Errorf("the slug %q is a reserved word, a language id or an area id", slug)
	}
	dir := filepath.Join(*root, "nodes", filepath.FromSlash(*area), slug)
	if _, err := os.Stat(dir); err == nil {
		return fmt.Errorf("%s exists", dir)
	}
	if *name == "" {
		*name = strings.ToUpper(slug[:1]) + strings.ReplaceAll(slug[1:], "-", " ")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "---\ntag: NEW-%s\nkind: %s\nslug: %s\nname: %s\narea: %s\nlevel: %s\nsummary: >-\n  Write one sentence that says what this is.\n---\n\n", slug, *kind, slug, *name, *area, *level)
	for _, h := range check.Order[*kind] {
		fmt.Fprintf(&b, "## %s\n\n", h)
		if h == "The algorithm" {
			b.WriteString("```pseudo id=main\nNAME(A)\nL1  ▷ write the first step\n```\n\n")
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	p := filepath.Join(dir, "node.md")
	if err := os.WriteFile(p, []byte(strings.TrimRight(b.String(), "\n")+"\n"), 0o644); err != nil {
		return err
	}
	fmt.Println(p)
	return nil
}

// cmdCheck runs the audit rules. It exits with 1 only when a rule blocks.
func cmdCheck(args []string) error {
	f := newFlags("check", "[flags] [slug ...]")
	root := f.String("root", ".", "the root of the content repository")
	rule := f.String("rule", "", "run only this rule, for example T08")
	changed := f.String("changed", "", "check only the nodes that changed since this git ref, for example origin/main")
	main := f.Bool("main", false, "the run is on the main branch, so provisional ids block")
	asJSON := f.Bool("json", false, "print the findings as JSON")
	withViz := f.Bool("viz", false, "also build and check the scenes")
	rules := f.Bool("rules", false, "print the rules and exit")
	decls := f.String("decls", "build/lean/decls.jsonl", "the declarations that aik lean build wrote, for the L rules; a missing file skips them")
	checkers := f.Bool("checkers", false, "run each custom checker on its checker_cases.jsonl (M04); the checker runs in the sandbox")
	unsafe := f.Bool("unsafe-local", false, "with -checkers, run the checkers on this machine, not in the sandbox; CI sets CI=true instead")
	sandbox := f.String("sandbox", "auto", "with -checkers, the container engine of the sandbox: docker, podman, or none")
	images := f.String("images", "manifests/images.yaml", "with -checkers, the manifest of the runner images")
	if err := parse(f, args); err != nil {
		return err
	}
	if *rules {
		var ids []string
		for id := range check.Rules {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			fmt.Printf("%s  %s\n", id, check.Rules[id])
		}
		return nil
	}
	c, err := load(*root)
	if err != nil {
		return err
	}
	ds, err := status.LoadDecls(under(*root, *decls))
	if err != nil {
		return err
	}
	opt := check.Options{Rule: *rule, Main: *main, Decls: ds}
	if *checkers {
		box, err := openSandbox(*sandbox, under(c.Root, *images), *unsafe)
		if err != nil {
			return err
		}
		if box == nil && !*unsafe && os.Getenv("CI") != "true" {
			return usageError{"the checkers are code from the repository; use the sandbox or CI, or give -unsafe-local"}
		}
		opt.Checker = func(k *contract.Contract) (*contract.Custom, error) {
			if box != nil {
				if err := box.Ready("python"); err != nil {
					return nil, fmt.Errorf("the checker runs in the Python image: %w", err)
				}
			}
			return k.StartCustom(lang.CheckerWrap(box, k))
		}
		if box != nil {
			defer box.Reap()
		}
	}
	if f.NArg() > 0 || *changed != "" {
		opt.Only = map[string]bool{}
		for _, s := range f.Args() {
			if c.BySlug[s] == nil {
				return usageError{fmt.Sprintf("no node has the slug %q", s)}
			}
			opt.Only[s] = true
		}
		if *changed != "" {
			slugs, err := changedSlugs(c, *changed)
			if err != nil {
				return err
			}
			for _, s := range slugs {
				opt.Only[s] = true
			}
			if len(opt.Only) == 0 {
				fmt.Fprintln(os.Stderr, "no node changed")
				return nil
			}
		}
	}
	findings := check.Run(c, opt)
	if *withViz {
		findings = append(findings, vizFindings(c, opt.Only)...)
	}
	blocks := 0
	for _, x := range findings {
		if x.Severity == check.Block {
			blocks++
		}
	}
	if *asJSON {
		if findings == nil {
			findings = []check.Finding{}
		}
		if err := printJSON(findings); err != nil {
			return err
		}
	} else {
		gha := os.Getenv("GITHUB_ACTIONS") == "true"
		for _, x := range findings {
			fmt.Println(x.String())
			if gha {
				level := "warning"
				if x.Severity == check.Block {
					level = "error"
				}
				loc := "file=" + x.Path
				if x.Line > 0 {
					loc += fmt.Sprintf(",line=%d", x.Line)
				}
				fmt.Printf("::%s %s,title=%s::%s\n", level, loc, x.Rule, x.Msg)
			}
		}
		fmt.Fprintf(os.Stderr, "%d nodes, %d findings, %d block\n", countOnly(c, opt.Only), len(findings), blocks)
	}
	if blocks > 0 {
		return errFindings
	}
	return nil
}

func countOnly(c *node.Corpus, only map[string]bool) int {
	if len(only) == 0 {
		return len(c.Nodes)
	}
	return len(only)
}

// changedSlugs gives the nodes with a file that changed between base and HEAD, or in the work tree.
func changedSlugs(c *node.Corpus, base string) ([]string, error) {
	files, err := changedFiles(c.Root, base)
	if err != nil {
		return nil, err
	}
	var slugs []string
	for _, n := range c.Nodes {
		prefix := n.Rel + "/"
		for _, p := range files {
			if strings.HasPrefix(p, prefix) {
				slugs = append(slugs, n.Slug)
				break
			}
		}
	}
	return slugs, nil
}

func vizFindings(c *node.Corpus, only map[string]bool) []check.Finding {
	var out []check.Finding
	for _, n := range c.Nodes {
		if n.Scene == "" || (len(only) > 0 && !only[n.Slug]) {
			continue
		}
		r := checkScene(c, n)
		for _, msg := range r.Fails {
			out = append(out, check.Finding{Rule: "V00", Severity: check.Block, Path: n.Scene, Msg: msg})
		}
	}
	return out
}

// checkScene checks the step scene and the architecture scene of a node, as either exists, into one report.
func checkScene(c *node.Corpus, n *node.Node) viz.Report {
	r := viz.Report{Slug: n.Slug}
	if n.Scene != "" {
		r = checkStepScene(c, n)
	}
	if n.ArchScene != "" {
		_, a := viz.NodeArch(c, n)
		r.Fails = append(r.Fails, a.Fails...)
		r.Steps = append(r.Steps, a.Steps...)
		if a.Skipped != "" {
			r.Skipped = a.Skipped
		}
	}
	return r
}

func checkStepScene(c *node.Corpus, n *node.Node) viz.Report {
	js, err := viz.Build(filepath.Join(c.Root, n.Scene), false)
	if err != nil {
		return viz.Report{Slug: n.Slug, Fails: []string{"the scene does not build: " + err.Error()}}
	}
	var presets []viz.Preset
	for _, p := range n.Presets {
		presets = append(presets, viz.Preset{Name: p.Name, Input: p.Input})
	}
	ids, _ := check.PseudoIDs(n.Body)
	ref, refErr := sceneRef(n)
	return viz.Check(viz.Scene{Slug: n.Slug, Bundle: js, Presets: presets, IDs: ids["main"], Ref: ref, RefErr: refErr})
}

// sceneRef gives the TypeScript reference of the node with its markers turned into probes, for V06, or nil when the node has none.
func sceneRef(n *node.Node) ([]byte, error) {
	im := n.Lang("typescript")
	if im == nil || im.Main() == nil {
		return nil, nil
	}
	return viz.Reference(im.Main().Content, im.Main().Path)
}

// cmdStatus prints the ladders of one node.
func cmdStatus(args []string) error {
	f := newFlags("status", "[flags] <slug>")
	root := f.String("root", ".", "the root of the content repository")
	results := f.String("results", "build/results", "the directory of the test results")
	decls := f.String("decls", "build/lean/decls.jsonl", "the declarations that aik lean build wrote")
	asJSON := f.Bool("json", false, "print the status as JSON")
	if err := parse(f, args); err != nil {
		return err
	}
	if f.NArg() != 1 {
		return usageError{"give one slug"}
	}
	c, err := load(*root)
	if err != nil {
		return err
	}
	n := c.BySlug[f.Arg(0)]
	if n == nil {
		return usageError{fmt.Sprintf("no node has the slug %q", f.Arg(0))}
	}
	res, err := status.LoadResults(filepath.Join(*root, *results))
	if err != nil {
		return err
	}
	ds, err := status.LoadDecls(under(*root, *decls))
	if err != nil {
		return err
	}
	env := status.Env{Corpus: c, Results: res, Decls: ds}
	if n.Scene != "" || n.ArchScene != "" {
		r := checkScene(c, n)
		if r.Skipped == "" {
			env.VizChecked = map[string][]string{n.Slug: r.Fails}
		}
	}
	st := status.Of(n, env)
	if *asJSON {
		return printJSON(st)
	}
	fmt.Printf("%s (%s, %s)\n\n", n.Name, n.Kind, n.Rel)
	show := func(what string, r status.Rung) {
		fmt.Printf("  %-14s %s  %s\n", what, r.Label, r.Name)
		for _, s := range r.Next {
			fmt.Printf("  %-14s    next: %s\n", "", s)
		}
	}
	show("writing", st.W)
	show("proof", st.H)
	show("lean", st.L)
	show("visual", st.V)
	var ids []string
	for id := range st.I {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		show("code "+id, st.I[id])
	}
	for _, id := range ids {
		if t, ok := st.Tests[id]; ok {
			state := "fail"
			if t.Pass {
				state = "pass"
			}
			fmt.Printf("  %-14s native tests %s in %d ms\n", "test "+id, state, t.Ms)
		}
	}
	return nil
}

// cmdTest runs the native tests of the implementations and writes build/results/<lang>.json.
func cmdTest(args []string) error {
	f := newFlags("test", "[flags] [slug ...]")
	root := f.String("root", ".", "the root of the content repository")
	only := f.String("lang", "", "a comma list of languages to test; the default is all")
	out := f.String("out", "build/results", "the directory for the results")
	unsafe := f.Bool("unsafe-local", false, "run the tests on this machine, not in the sandbox; CI sets CI=true instead")
	sandbox := f.String("sandbox", "auto", "the container engine of the sandbox: docker, podman, or none; auto uses docker when the image manifest is there (08.7)")
	images := f.String("images", "manifests/images.yaml", "the manifest of the runner images, pinned by digest")
	keep := f.Bool("keep", false, "keep the temporary directories")
	changed := f.String("changed", "", "test only the implementations that changed since this git ref, for example origin/main (09.8)")
	cacheDir := f.String("cache", "", "keep the results of passing tests in this directory, and reuse them while the inputs stay the same")
	kind := f.String("kind", "cases", "cases runs the native tests and the shared cases; diff runs the differential test against the oracle on generated inputs (09.5); mutate runs the mutation test (09.6)")
	maxMutants := f.Int("mutants", 150, "with -kind mutate, the largest number of mutants of one file, sampled with a fixed seed; 0 runs all")
	var d diffFlags
	f.StringVar(&d.classes, "classes", "tiny,small,medium", "with -kind diff or mutate, the size classes to generate")
	f.StringVar(&d.seeds, "seeds", "", "with -kind diff or mutate, a comma list of seeds; the default is seeds.pr of generated.yaml")
	f.BoolVar(&d.write, "write-regress", false, "with -kind diff, add each shrunk failure to cases/regress.jsonl of the problem")
	if err := parse(f, args); err != nil {
		return err
	}
	if *kind != "cases" && *kind != "diff" && *kind != "mutate" {
		return usageError{"-kind is cases, diff or mutate, not " + *kind}
	}
	if *changed != "" && f.NArg() > 0 {
		return usageError{"give slugs or -changed, not both"}
	}
	c, err := load(*root)
	if err != nil {
		return err
	}
	box, err := openSandbox(*sandbox, under(c.Root, *images), *unsafe)
	if err != nil {
		return err
	}
	if box == nil && !*unsafe && os.Getenv("CI") != "true" {
		return usageError{"the tests run code from the repository; use the sandbox or CI, or give -unsafe-local"}
	}
	nodes, err := selectNodes(c, f.Args())
	if err != nil {
		return err
	}
	want, err := langSet(*only)
	if err != nil {
		return err
	}
	r := &lang.Runner{Keep: *keep, Box: box, Log: func(format string, a ...any) { fmt.Fprintf(os.Stderr, format+"\n", a...) }}
	if box != nil {
		// The engine mounts the build trees at their own paths, so they must be in a directory that it can see. The system temporary directory of macOS is not.
		abs, err := filepath.Abs(filepath.Join(c.Root, "build", "sandbox"))
		if err != nil {
			return err
		}
		r.TempDir = abs
		if err := os.MkdirAll(r.TempDir, 0o755); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "the tests run in %s containers with the images of %s\n", box.Engine, *images)
	}
	problems := contractsOf(c)
	var tc *testCache
	if *cacheDir != "" {
		if tc, err = newTestCache(under(*root, *cacheDir), c.Root); err != nil {
			return err
		}
	}
	if *kind == "diff" || *kind == "mutate" {
		d.root, d.box, d.out = c.Root, box, under(*root, *out)
		if *kind == "mutate" {
			return runMutate(r, nodes, want, problems, d, *maxMutants, tc, f.NArg() == 0)
		}
		return runDiff(r, nodes, want, problems, d)
	}
	var aff affected
	if *changed != "" {
		files, err := changedFiles(c.Root, *changed)
		if err != nil {
			return err
		}
		aff = findAffected(c, files)
		switch {
		case aff.All:
			fmt.Fprintf(os.Stderr, "all implementations run, because %s\n", aff.Why)
		case len(aff.Langs) == 0:
			fmt.Println("no implementation changed")
			return nil
		}
	}
	for _, n := range nodes {
		for _, p := range problems(n) {
			if p.err != nil {
				return fmt.Errorf("the contract of a problem that %s solves: %w", n.Slug, p.err)
			}
		}
	}
	byLang := map[string]*status.Results{}
	failed := 0
	for _, n := range nodes {
		for _, im := range n.Impls {
			if len(want) > 0 && !want[im.Lang] || *changed != "" && !aff.Has(n, im.Lang) {
				continue
			}
			rc, ok := lang.Recipes[im.Lang]
			if !ok {
				continue
			}
			if err := r.Available(rc); err != nil {
				var skip lang.Skip
				if errors.As(err, &skip) {
					fmt.Fprintf(os.Stderr, "skip %s/%s: %v\n", n.Slug, im.Lang, err)
					continue
				}
				return err
			}
			res := byLang[im.Lang]
			if res == nil {
				res = &status.Results{Lang: im.Lang, Version: r.ToolVersion(rc)}
				byLang[im.Lang] = res
			}
			var key string
			if tc != nil {
				var ks []*contract.Contract
				for _, p := range problems(n) {
					ks = append(ks, p.k)
				}
				tool := res.Version
				if box != nil {
					// The image can change while the version line stays the same, for example with a new pytest.
					ref, _ := box.Image(im.Lang)
					tool += " image " + ref
				}
				key = tc.key(tool, n, im, ks)
				if t, ok := tc.Get(im.Lang, key); ok {
					fmt.Printf("pass %s/%s native tests and shared cases (cached)\n", n.Slug, im.Lang)
					res.Tests = append(res.Tests, t)
					continue
				}
			}
			t := r.Test(n, im)
			state := "pass"
			if !t.Pass {
				state = "FAIL"
			}
			fmt.Printf("%-4s %s/%s native tests (%d ms)\n", state, n.Slug, im.Lang, t.Ms)
			if !t.Pass {
				fmt.Println(indent(t.Output))
			}
			printFramework(n.Slug, im.Lang, t)
			for _, p := range problems(n) {
				start := time.Now()
				s := r.Shared(n, im, p.k, p.cases)
				t.Shared = append(t.Shared, s)
				printShared(n.Slug, im.Lang, s, time.Since(start))
			}
			if !t.OK() {
				failed++
			}
			if tc != nil {
				if err := tc.Put(im.Lang, key, t); err != nil {
					return err
				}
			}
			res.Tests = append(res.Tests, t)
		}
	}
	dir := under(*root, *out)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for id, res := range byLang {
		prev := filepath.Join(dir, id+".json")
		if f.NArg() > 0 || *changed != "" {
			res.Tests = mergeTests(prev, res.Tests)
		}
		var buf bytes.Buffer
		if err := writeJSON(&buf, res); err != nil {
			return err
		}
		if err := os.WriteFile(prev, buf.Bytes(), 0o644); err != nil {
			return err
		}
	}
	if tc != nil && f.NArg() == 0 && *changed == "" {
		langs := make([]string, 0, len(byLang))
		for id := range byLang {
			langs = append(langs, id)
		}
		tc.Prune(langs)
	}
	if failed > 0 {
		fmt.Fprintf(os.Stderr, "%d implementations failed\n", failed)
		return errFindings
	}
	return nil
}

// openSandbox returns the sandbox of the mode, or nil for host mode.
func openSandbox(mode, manifest string, unsafe bool) (*lang.Sandbox, error) {
	if unsafe || mode == "none" {
		return nil, nil
	}
	switch mode {
	case "auto":
		if _, err := os.Stat(manifest); err != nil {
			return nil, nil
		}
		mode = "docker"
	case "docker", "podman":
	default:
		return nil, usageError{"-sandbox is docker, podman, none or auto, not " + mode}
	}
	imgs, err := lang.LoadImages(manifest)
	if err != nil {
		return nil, err
	}
	return lang.NewSandbox(mode, imgs), nil
}

// selectNodes returns the nodes with the slugs, or all nodes when no slug is given.
func selectNodes(c *node.Corpus, slugs []string) ([]*node.Node, error) {
	if len(slugs) == 0 {
		return c.Nodes, nil
	}
	var nodes []*node.Node
	for _, s := range slugs {
		n := c.BySlug[s]
		if n == nil {
			return nil, usageError{fmt.Sprintf("no node has the slug %q", s)}
		}
		nodes = append(nodes, n)
	}
	return nodes, nil
}

// langSet reads a comma list of languages. An empty list gives an empty set, which means all languages.
func langSet(only string) (map[string]bool, error) {
	want := map[string]bool{}
	for _, l := range strings.Split(only, ",") {
		if l = strings.TrimSpace(l); l != "" {
			if _, ok := lang.Recipes[l]; !ok {
				return nil, usageError{fmt.Sprintf("no recipe for the language %q", l)}
			}
			want[l] = true
		}
	}
	return want, nil
}

// contractsOf returns a function that gives the contracts of the problems that a node solves. The contract of each problem is read once. A contract that does not load is an error of the run, because every test against it would be wrong.
func contractsOf(c *node.Corpus) func(n *node.Node) []*sharedCases {
	contracts := map[string]*sharedCases{}
	return func(n *node.Node) []*sharedCases {
		var out []*sharedCases
		for _, s := range n.Solves {
			p := c.BySlug[s.Problem]
			if p == nil || !contract.Exists(p.Dir) {
				continue
			}
			sc, ok := contracts[p.Slug]
			if !ok {
				k, cases, err := contract.Load(p.Dir)
				if err != nil {
					sc = &sharedCases{err: err}
				} else {
					sc = &sharedCases{k: k, cases: cases}
				}
				contracts[p.Slug] = sc
			}
			out = append(out, sc)
		}
		return out
	}
}

// sharedCases is the contract of one problem and its cases.
type sharedCases struct {
	k     *contract.Contract
	cases []contract.Case
	err   error
}

// printShared prints the counts of the shared cases and each failure, with the case id, so that a CI log names the case.
func printShared(slug, lang string, s status.Shared, took time.Duration) {
	state := "pass"
	if !s.OK() {
		state = "FAIL"
	}
	line := fmt.Sprintf("%-4s %s/%s shared cases of %s: %d of %d pass", state, slug, lang, s.Problem, s.Passed, s.Cases)
	if s.Skipped > 0 {
		line += fmt.Sprintf(", %d skipped", s.Skipped)
	}
	fmt.Printf("%s (%d ms)\n", line, took.Milliseconds())
	if s.SkipReason != "" {
		fmt.Println(indent("skipped: " + s.SkipReason))
	}
	if s.Error != "" {
		fmt.Println(indent(s.Error))
	}
	for _, f := range s.Failures {
		msg := fmt.Sprintf("case %s (%s): %s", f.Case, lang, f.Reason)
		if f.Got != "" {
			msg += "\n  got:  " + f.Got
		}
		if f.Want != "" {
			msg += "\n  want: " + f.Want
		}
		fmt.Println(indent(msg))
	}
}

// printFramework prints the checks of 08.10 that a framework version has besides its native tests: the test vectors
// of a metric, which fail the run, and the format, lint and type checks, which only lower the rung (X13).
func printFramework(slug, lang string, t status.Test) {
	if v := t.Vectors; v != nil {
		state := "pass"
		if !v.OK() {
			state = "FAIL"
		}
		fmt.Printf("%-4s %s/%s test vectors: %d of %d pass\n", state, slug, lang, v.Passed, v.Cases)
		if v.Error != "" {
			fmt.Println(indent(v.Error))
		}
		for _, f := range v.Failures {
			fmt.Println(indent(fmt.Sprintf("vector %s (%s): %s", f.Case, lang, f.Reason)))
		}
	}
	for _, l := range t.Lint {
		switch {
		case l.Missing:
			fmt.Printf("note %s/%s %s is not installed, so X3 waits\n", slug, lang, l.Tool)
		case l.Pass:
			fmt.Printf("pass %s/%s %s\n", slug, lang, l.Tool)
		default:
			fmt.Printf("note %s/%s %s is not clean, so X3 waits\n", slug, lang, l.Tool)
			fmt.Println(indent(l.Output))
		}
	}
}

// mergeTests keeps the old results of the nodes that this run did not test.
func mergeTests(path string, now []status.Test) []status.Test {
	data, err := os.ReadFile(path)
	if err != nil {
		return now
	}
	var old status.Results
	if readJSON(data, &old) != nil {
		return now
	}
	seen := map[string]bool{}
	for _, t := range now {
		seen[t.Slug] = true
	}
	for _, t := range old.Tests {
		if !seen[t.Slug] {
			now = append(now, t)
		}
	}
	sort.Slice(now, func(i, j int) bool { return now[i].Slug < now[j].Slug })
	return now
}

func indent(s string) string {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return ""
	}
	return "    " + strings.ReplaceAll(s, "\n", "\n    ")
}

// cmdViz bundles or checks the scenes.
func cmdViz(args []string) error {
	if len(args) == 0 {
		return usageError{"use aik viz build <slug>, aik viz check [slug ...] or aik viz shapes <slug>"}
	}
	sub, args := args[0], args[1:]
	switch sub {
	case "shapes":
		f := newFlags("viz shapes", "[flags] <slug>")
		root := f.String("root", ".", "the root of the content repository")
		write := f.Bool("w", false, "write viz/shapes.scene.yaml in the node instead of printing it")
		force := f.Bool("force", false, "with -w, replace a scene that exists, and its layout")
		if err := parse(f, args); err != nil {
			return err
		}
		if f.NArg() != 1 {
			return usageError{"give one slug"}
		}
		c, err := load(*root)
		if err != nil {
			return err
		}
		n := c.BySlug[f.Arg(0)]
		if n == nil || n.Shapes == nil {
			return usageError{fmt.Sprintf("no node with the slug %q has a shapes.yaml", f.Arg(0))}
		}
		if ps := n.Shapes.Check(); len(ps) > 0 {
			return fmt.Errorf("%s: %w", n.Shapes.Path, ps[0])
		}
		b, err := viz.GenArch(n.Shapes, n.Name).Encode()
		if err != nil {
			return err
		}
		if !*write {
			_, err = os.Stdout.Write(b)
			return err
		}
		p := filepath.Join(n.Dir, filepath.FromSlash(viz.ArchFile))
		if _, err := os.Stat(p); err == nil && !*force {
			return fmt.Errorf("%s/%s exists; give -force to replace it", n.Rel, viz.ArchFile)
		}
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(p, b, 0o644); err != nil {
			return err
		}
		fmt.Printf("wrote %s/%s\n", n.Rel, viz.ArchFile)
		return nil
	case "build":
		f := newFlags("viz build", "[flags] <slug>")
		root := f.String("root", ".", "the root of the content repository")
		out := f.String("o", "", "the output file; the default is standard output")
		minify := f.Bool("minify", false, "minify the bundle")
		if err := parse(f, args); err != nil {
			return err
		}
		if f.NArg() != 1 {
			return usageError{"give one slug"}
		}
		c, err := load(*root)
		if err != nil {
			return err
		}
		n := c.BySlug[f.Arg(0)]
		if n == nil || n.Scene == "" {
			return usageError{fmt.Sprintf("no node with the slug %q has a scene", f.Arg(0))}
		}
		js, err := viz.Build(filepath.Join(c.Root, n.Scene), *minify)
		if err != nil {
			return err
		}
		if *out == "" {
			_, err = os.Stdout.Write(js)
			return err
		}
		return os.WriteFile(*out, js, 0o644)
	case "check":
		f := newFlags("viz check", "[flags] [slug ...]")
		root := f.String("root", ".", "the root of the content repository")
		asJSON := f.Bool("json", false, "print the reports as JSON")
		if err := parse(f, args); err != nil {
			return err
		}
		c, err := load(*root)
		if err != nil {
			return err
		}
		only := map[string]bool{}
		for _, s := range f.Args() {
			only[s] = true
		}
		var reports []viz.Report
		failed := 0
		for _, n := range c.Nodes {
			if (n.Scene == "" && n.ArchScene == "") || (len(only) > 0 && !only[n.Slug]) {
				continue
			}
			r := checkScene(c, n)
			reports = append(reports, r)
			if len(r.Fails) > 0 {
				failed++
			}
		}
		if *asJSON {
			if reports == nil {
				reports = []viz.Report{}
			}
			if err := printJSON(reports); err != nil {
				return err
			}
		} else {
			for _, r := range reports {
				switch {
				case r.Skipped != "":
					fmt.Printf("skip %s: %s\n", r.Slug, r.Skipped)
				case len(r.Fails) == 0 && len(r.Lines) == 0: // an architecture scene has no reference lines
					fmt.Printf("pass %s: %d runs\n", r.Slug, len(r.Steps))
				case len(r.Fails) == 0:
					fmt.Printf("pass %s: %d runs, %d lines reached\n", r.Slug, len(r.Steps), len(r.Lines))
				default:
					fmt.Printf("FAIL %s\n", r.Slug)
					for _, m := range r.Fails {
						fmt.Println("    " + m)
					}
				}
			}
		}
		if failed > 0 {
			return errFindings
		}
		return nil
	}
	return usageError{fmt.Sprintf("unknown subcommand %q; use build, check or shapes", sub)}
}

// cmdSite builds, serves or checks the static site.
func cmdSite(args []string) error {
	if len(args) == 0 {
		return usageError{"use aik site build, aik site serve, aik site check or aik site deploy"}
	}
	sub, args := args[0], args[1:]
	f := newFlags("site "+sub, "[flags]")
	root := f.String("root", ".", "the root of the content repository")
	out := f.String("out", "build/site", "the output directory")
	base := f.String("base", "https://ai.tamnd.com", "the base URL")
	results := f.String("results", "build/results", "the directory of the test results")
	decls := f.String("decls", "build/lean/decls.jsonl", "the declarations that aik lean build wrote")
	spec := f.String("spec", "", "a directory of the specification to publish at /spec/")
	commit := f.String("commit", "", "the commit of the content; the default comes from git")
	minify := f.Bool("minify", false, "minify the scenes")
	addr := f.String("addr", "localhost:8150", "the address for serve")
	var d deployOptions
	// The Worker ai on the account is another app, so the site Worker is ai-site.
	f.StringVar(&d.Worker, "worker", "ai-site", "the Cloudflare Worker that serves the site, for deploy")
	f.StringVar(&d.Domain, "domain", "ai.tamnd.com", "the custom domain of production, for deploy")
	f.StringVar(&d.Branch, "branch", "", "the branch of the deployment; the default is main, or the git branch with -preview")
	f.BoolVar(&d.Preview, "preview", false, "deploy to a branch alias, not to production")
	noBuild := f.Bool("no-build", false, "deploy the directory of -out as it is, for deploy")
	if err := parse(f, args); err != nil {
		return err
	}
	opt := site.Options{
		Out:      under(*root, *out),
		BaseURL:  strings.TrimRight(*base, "/"),
		Results:  filepath.Join(*root, *results),
		Decls:    under(*root, *decls),
		SpecDir:  *spec,
		Version:  ver(),
		Commit:   *commit,
		Repo:     "https://github.com/tamnd/ai",
		AgentURL: "https://github.com/tamnd/ai-agent",
		Minify:   *minify,
		Log:      func(format string, a ...any) { fmt.Fprintf(os.Stderr, "warn: "+format+"\n", a...) },
	}
	if filepath.IsAbs(*out) {
		opt.Out = *out
	}
	if filepath.IsAbs(*results) {
		opt.Results = *results
	}
	if opt.Commit == "" {
		cmd := exec.Command("git", "rev-parse", "HEAD")
		cmd.Dir = *root
		if b, err := cmd.Output(); err == nil {
			opt.Commit = strings.TrimSpace(string(b))
		}
	}
	build := func() error {
		start := time.Now()
		c, err := load(*root)
		if err != nil {
			return err
		}
		rep, err := site.Build(c, opt)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "built %d pages, %d files, %.1f MB in %s\n", rep.Pages, rep.Files, float64(rep.Bytes)/1e6, time.Since(start).Round(time.Millisecond))
		for _, m := range rep.MathErrors {
			fmt.Fprintln(os.Stderr, "math: "+m)
		}
		return nil
	}
	switch sub {
	case "build":
		return build()
	case "check":
		if err := build(); err != nil {
			return err
		}
		bad, err := checkLinks(opt.Out)
		if err != nil {
			return err
		}
		for _, b := range bad {
			fmt.Println(b)
		}
		if len(bad) > 0 {
			fmt.Fprintf(os.Stderr, "%d broken links\n", len(bad))
			return errFindings
		}
		return nil
	case "serve":
		opt.BaseURL = "http://" + *addr
		if err := build(); err != nil {
			return err
		}
		go watch(*root, build)
		fmt.Fprintf(os.Stderr, "serving %s at http://%s\n", opt.Out, *addr)
		srv := &http.Server{Addr: *addr, Handler: serveDir(opt.Out), ReadHeaderTimeout: 10 * time.Second}
		return srv.ListenAndServe()
	case "deploy":
		if !*noBuild {
			if err := build(); err != nil {
				return err
			}
		}
		d.Dir, d.Commit = opt.Out, opt.Commit
		if d.Branch == "" {
			d.Branch = "main"
			if d.Preview {
				d.Branch = gitBranch(*root)
			}
		}
		return deploy(d)
	}
	return usageError{fmt.Sprintf("unknown subcommand %q; use build, serve, check or deploy", sub)}
}

// serveDir serves the site like the static assets of a Cloudflare Worker: /a/ maps to a/index.html, and a missing page gets 404.html.
func serveDir(dir string) http.Handler {
	files := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := filepath.Join(dir, filepath.FromSlash(filepath.Clean("/"+r.URL.Path)))
		if st, err := os.Stat(p); err != nil || (st.IsDir() && !exists(filepath.Join(p, "index.html"))) {
			if data, err := os.ReadFile(filepath.Join(dir, "404.html")); err == nil {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write(data)
				return
			}
		}
		w.Header().Set("Cache-Control", "no-store")
		files.ServeHTTP(w, r)
	})
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// watch rebuilds the site when a file under root changes. It polls, so it needs no extra package.
func watch(root string, build func() error) {
	last := stamp(root)
	for {
		time.Sleep(700 * time.Millisecond)
		now := stamp(root)
		if now.Equal(last) {
			continue
		}
		last = now
		if err := build(); err != nil {
			fmt.Fprintln(os.Stderr, "build: "+err.Error())
		}
	}
}

func stamp(root string) time.Time {
	var t time.Time
	for _, d := range []string{"nodes", "topics", "registers", "reviews"} {
		_ = filepath.WalkDir(filepath.Join(root, d), func(_ string, e fs.DirEntry, err error) error {
			if err != nil || e.IsDir() {
				return nil
			}
			if info, err := e.Info(); err == nil && info.ModTime().After(t) {
				t = info.ModTime()
			}
			return nil
		})
	}
	return t
}

var hrefRE = regexp.MustCompile(`(?:href|src)="(/[^"#?]*)`)

// checkLinks finds the internal links of the built site that point at no file.
func checkLinks(dir string) ([]string, error) {
	var bad []string
	seen := map[string]bool{}
	err := filepath.WalkDir(dir, func(p string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() || !strings.HasSuffix(p, ".html") {
			return err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		for _, m := range hrefRE.FindAllSubmatch(data, -1) {
			u := string(m[1])
			if strings.HasPrefix(u, "//") {
				continue
			}
			key := rel + " " + u
			if seen[key] {
				continue
			}
			seen[key] = true
			t := filepath.Join(dir, filepath.FromSlash(u))
			if strings.HasSuffix(u, "/") {
				t = filepath.Join(t, "index.html")
			}
			if !exists(t) && !exists(t+".html") {
				bad = append(bad, fmt.Sprintf("%s: the link %s points at no file", filepath.ToSlash(rel), u))
			}
		}
		return nil
	})
	sort.Strings(bad)
	return bad, err
}

// under returns the path p under root. An absolute p stays as it is.
func under(root, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(root, p)
}
