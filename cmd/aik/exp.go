// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/tamnd/ai-agent/exp"
	"github.com/tamnd/ai-agent/lang"
	"github.com/tamnd/ai-agent/node"
)

// cmdExp runs the experiments of spec 09.
func cmdExp(args []string) error {
	if len(args) == 0 {
		return usageError{"use aik exp list, plan, run, power or verify"}
	}
	switch args[0] {
	case "list":
		return cmdExpList(args[1:])
	case "run":
		return cmdExpRun(args[1:])
	case "power":
		return cmdExpPower(args[1:])
	case "plan":
		return cmdExpPlan(args[1:])
	case "verify":
		return cmdExpVerify(args[1:])
	}
	return usageError{"use aik exp list, plan, run, power or verify, not aik exp " + args[0]}
}

type expRef struct {
	n *node.Node
	e *exp.Experiment
}

// selectExps gives the experiments of the nodes with the slugs, or of all nodes. A slug/id names one experiment.
func selectExps(c *node.Corpus, args []string) ([]expRef, error) {
	var out []expRef
	if len(args) == 0 {
		for _, n := range c.Nodes {
			for _, e := range n.Exps {
				out = append(out, expRef{n, e})
			}
		}
		return out, nil
	}
	for _, a := range args {
		slug, id, _ := strings.Cut(a, "/")
		n := c.BySlug[slug]
		if n == nil {
			return nil, usageError{fmt.Sprintf("no node has the slug %q", slug)}
		}
		found := false
		for _, e := range n.Exps {
			if id == "" || e.Slug == id {
				out = append(out, expRef{n, e})
				found = true
			}
		}
		if !found {
			return nil, usageError{fmt.Sprintf("the node %s has no experiment %q", slug, id)}
		}
	}
	return out, nil
}

func cmdExpList(args []string) error {
	f := newFlags("exp list", "[flags] [slug | slug/experiment ...]")
	root := f.String("root", ".", "the root of the content repository")
	if err := parse(f, args); err != nil {
		return err
	}
	c, err := load(*root)
	if err != nil {
		return err
	}
	refs, err := selectExps(c, f.Args())
	if err != nil {
		return err
	}
	for _, r := range refs {
		state := "not run"
		if res := r.e.Results; res != nil {
			state = res.Verdict + " on " + res.Date
			if why := r.e.Stale(); why != "" {
				state += ", stale: " + why
			}
		}
		budget := "?"
		if r.e.Design != nil {
			budget = r.e.Design.Budget
		}
		fmt.Printf("%s/%s\t%s\t%s\n", r.n.Slug, r.e.Slug, budget, state)
	}
	return nil
}

func cmdExpRun(args []string) error {
	f := newFlags("exp run", "[flags] slug | slug/experiment ...")
	root := f.String("root", ".", "the root of the content repository")
	update := f.Bool("update", false, "write results.json; without it, aik prints the verdict and writes nothing (rule R05)")
	unsafe := f.Bool("unsafe-local", false, "run run.py with python3 on this machine, not in the sandbox; CI sets CI=true instead")
	sandbox := f.String("sandbox", "auto", "the container engine of the sandbox: docker, podman, or none; auto uses docker when the image manifest is there (08.7)")
	images := f.String("images", "manifests/images.yaml", "the manifest of the runner images, pinned by digest")
	python := f.String("python", "python3", "the Python of a run with -unsafe-local or in CI")
	seeds := f.Int("seeds", 0, "run a pilot with this many seeds instead of the design's, for aik exp power; not with -update")
	if err := parse(f, args); err != nil {
		return err
	}
	if f.NArg() == 0 {
		return usageError{"name the experiments to run: slug or slug/experiment"}
	}
	if *seeds < 0 || *seeds > 0 && *update {
		return usageError{"-seeds runs a pilot, whose results are not results.json: give it without -update, and more than 0"}
	}
	c, err := load(*root)
	if err != nil {
		return err
	}
	refs, err := selectExps(c, f.Args())
	if err != nil {
		return err
	}
	box, err := openSandbox(*sandbox, under(c.Root, *images), *unsafe)
	if err != nil {
		return err
	}
	if box == nil && !*unsafe && os.Getenv("CI") != "true" {
		return usageError{"an experiment runs code from the repository; use the sandbox or CI, or give -unsafe-local"}
	}
	failed := 0
	for _, r := range refs {
		if *seeds > 0 {
			e, d := *r.e, *r.e.Design
			d.Seeds.N = *seeds
			e.Design = &d
			r.e = &e
		}
		if err := runExp(c, r, box, *python, *update); err != nil {
			fmt.Fprintf(os.Stderr, "%s/%s: %v\n", r.n.Slug, r.e.Slug, err)
			failed++
		}
	}
	if box != nil {
		box.Reap()
	}
	if failed > 0 {
		return errFindings
	}
	return nil
}

func runExp(c *node.Corpus, r expRef, box *lang.Sandbox, python string, update bool) error {
	e := r.e
	if err := e.Ready(); err != nil {
		return err
	}
	b := exp.Budgets[e.Design.Budget]
	if b.Name != "cpu-small" {
		return fmt.Errorf("the class %s runs in the GPU pool, which aik does not have yet; aik exp run runs cpu-small only", b.Name)
	}
	work, err := filepath.Abs(expWork(c, r))
	if err != nil {
		return err
	}
	if err := os.RemoveAll(work); err != nil {
		return err
	}
	if err := os.MkdirAll(work, 0o755); err != nil {
		return err
	}
	nodeDir, err := filepath.Abs(r.n.Dir)
	if err != nil {
		return err
	}
	script := filepath.Join(nodeDir, exp.Dir, e.Slug, exp.Script)
	fw := e.Design.Framework
	if fw == "" {
		fw = "python"
	}
	// The sandbox has no network, so the pinned data is fetched and checked here, before the run (9.4.2).
	data, err := exp.Prepare(context.Background(), c.Pins, e.Design, filepath.Join(c.Root, filepath.FromSlash(exp.DataDir)), fetchData)
	if err != nil {
		return err
	}
	mounts := []lang.Mount{{Dir: work}, {Dir: nodeDir, ReadOnly: true}}
	for _, id := range slices.Sorted(maps.Keys(data)) {
		mounts = append(mounts, lang.Mount{Dir: data[id], ReadOnly: true})
	}
	o := exp.Options{
		Work: work, Node: r.n.Tag, Image: "local", Data: data,
		Hardware: exp.Hardware{CPU: runtime.GOARCH, Threads: runtime.NumCPU(), OS: runtime.GOOS},
		Log:      func(format string, a ...any) { fmt.Fprintf(os.Stderr, format+"\n", a...) },
	}
	env := []string{"PYTHONHASHSEED=0", "PYTHONDONTWRITEBYTECODE=1", "OMP_NUM_THREADS=" + b.CPUs, "AIK_BUDGET=" + b.Name}
	if box != nil {
		box.Memory, box.CPUs = b.Memory, b.CPUs
		if err := box.Ready(fw); err != nil {
			return err
		}
		ref, _ := box.Image(fw)
		o.Image = ref
		o.Hardware.Threads = 0
		o.Exec = func(ctx context.Context, _ exp.Job, dir string) error {
			argv := box.Argv(fw, false, mounts, append([]string{"env"}, append(env, "python3", script, "--job", filepath.Join(dir, exp.JobFile), "--out", dir)...))
			return runLogged(ctx, argv, dir)
		}
	} else {
		o.Exec = func(ctx context.Context, _ exp.Job, dir string) error {
			argv := []string{python, script, "--job", filepath.Join(dir, exp.JobFile), "--out", dir}
			return runLogged(ctx, argv, dir, env...)
		}
	}
	res, err := exp.Run(context.Background(), e, o)
	if err != nil {
		return err
	}
	fmt.Printf("%s/%s: %s (%s, %.0f s)\n", r.n.Slug, e.Slug, res.Verdict, res.Run, res.WallTimeS)
	for _, v := range res.Primary {
		p := ""
		if v.P != nil {
			p = fmt.Sprintf(", p = %.3g", *v.P)
		}
		fmt.Printf("  %s %s: %s%s\n", v.Test, v.Metric, v.Verdict, p)
	}
	// The run's own copy, under build/, is the pilot that aik exp power reads.
	if err := exp.WriteTo(filepath.Join(work, exp.ResultsFile), res); err != nil {
		return err
	}
	if !update {
		fmt.Printf("  results.json is not written; give -update to write it. The run's results are in %s\n", filepath.Join(work, exp.ResultsFile))
		return nil
	}
	return exp.Write(e, res)
}

// runLogged runs the command, and keeps its output in the file log.txt of dir.
func runLogged(ctx context.Context, argv []string, dir string, env ...string) error {
	log, err := os.Create(filepath.Join(dir, "log.txt"))
	if err != nil {
		return err
	}
	defer log.Close()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Stdout, cmd.Stderr, cmd.Dir = log, log, dir
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		tail, _ := os.ReadFile(filepath.Join(dir, "log.txt"))
		lines := strings.Split(strings.TrimSpace(string(tail)), "\n")
		if len(lines) > 5 {
			lines = lines[len(lines)-5:]
		}
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return fmt.Errorf("run.py exited with %d:\n    %s", ee.ExitCode(), strings.Join(lines, "\n    "))
		}
		return err
	}
	return nil
}

// fetchData gets one file of a data pin over HTTPS, with the generic user agent of aik.
func fetchData(ctx context.Context, url string) (io.ReadCloser, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		cancel()
		return nil, err
	}
	req.Header.Set("User-Agent", "aik/"+strings.TrimPrefix(ver(), "v")+" (+https://github.com/tamnd/ai-agent)")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		cancel()
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		cancel()
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return cancelBody{resp.Body, cancel}, nil
}

// cancelBody ends the context of a request when the body is closed.
type cancelBody struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (b cancelBody) Close() error {
	err := b.ReadCloser.Close()
	b.cancel()
	return err
}

// expWork is the work directory of a run of the experiment, under build/.
func expWork(c *node.Corpus, r expRef) string {
	return filepath.Join(c.Root, "build", "exp", r.n.Slug, r.e.Slug)
}

func cmdExpPower(args []string) error {
	f := newFlags("exp power", "[flags] slug | slug/experiment ...")
	root := f.String("root", ".", "the root of the content repository")
	pilot := f.String("pilot", "", "the results.json of the pilot; without it, the last run under build/exp (aik exp run -seeds n), or the committed results.json")
	effect := f.String("effect", "", "the claimed effect in the claimed direction, absolute (0.02) or relative to the control mean (5%); without it, the pilot's effect")
	sd := f.String("sd", "", "the spread of one seed in each arm, absolute or relative to the control mean (2%); without it, the pilot's SD, which needs two seeds in an arm")
	seeds := f.Int("seeds", 0, "the number of seeds to compute the power at; without it, the design's, or the default of its budget class")
	target := f.Float64("target", exp.PowerTarget, "the least power (rule R07)")
	sims := f.Int("sims", 2000, "the number of simulated runs for each number of seeds")
	most := f.Int("max", 100, "the most seeds to try")
	asJSON := f.Bool("json", false, "print the power of each test as JSON")
	if err := parse(f, args); err != nil {
		return err
	}
	o := exp.PowerOptions{Seeds: *seeds, Target: *target, Sims: *sims, Max: *most}
	var err error
	if o.Effect, err = parseEffect("effect", *effect); err != nil {
		return err
	}
	if o.SD, err = parseEffect("sd", *sd); err != nil {
		return err
	}
	if *seeds < 0 || *sims < 1 || *most < 2 || *target <= 0 || *target >= 1 {
		return usageError{"-seeds is 0 or more, -sims 1 or more, -max 2 or more, and -target between 0 and 1"}
	}
	c, err := load(*root)
	if err != nil {
		return err
	}
	refs, err := selectExps(c, f.Args())
	if err != nil {
		return err
	}
	if *pilot != "" && len(refs) != 1 {
		return usageError{"-pilot is the pilot of one experiment: name it as slug/experiment"}
	}
	type report struct {
		Experiment string      `json:"experiment"`
		Pilot      string      `json:"pilot,omitempty"`
		Tests      []exp.Power `json:"tests,omitempty"`
		Error      string      `json:"error,omitempty"`
	}
	var out []report
	failed := false
	for _, r := range refs {
		rep := report{Experiment: r.n.Slug + "/" + r.e.Slug}
		res, from, err := readPilot(c, r, *pilot)
		if err != nil {
			rep.Error = err.Error()
		} else {
			rep.Pilot = from
			rep.Tests = exp.PowerOf(r.e, res, o)
		}
		for _, p := range rep.Tests {
			failed = failed || !p.OK()
		}
		out = append(out, rep)
	}
	if *asJSON {
		if err := printJSON(out); err != nil {
			return err
		}
	} else {
		for _, rep := range out {
			if rep.Error != "" {
				fmt.Printf("%s: %s\n", rep.Experiment, rep.Error)
				continue
			}
			fmt.Printf("%s: the pilot is %s\n", rep.Experiment, rep.Pilot)
			for _, p := range rep.Tests {
				printPower(p)
			}
		}
	}
	if failed {
		return errFindings
	}
	return nil
}

// readPilot reads the pilot of an experiment: the file given, the last run under build/exp, or the committed results.
func readPilot(c *node.Corpus, r expRef, file string) (*exp.Results, string, error) {
	if file != "" {
		res, err := exp.ReadResults(file)
		return res, file, err
	}
	last := filepath.Join(expWork(c, r), exp.ResultsFile)
	if res, err := exp.ReadResults(last); err == nil {
		if rel, err := filepath.Rel(c.Root, last); err == nil {
			last = filepath.ToSlash(rel)
		}
		return res, last, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, "", err
	}
	if r.e.Results != nil {
		return r.e.Results, filepath.Join(r.e.Rel, exp.ResultsFile), nil
	}
	return nil, "", errors.New("no pilot: run one with aik exp run -seeds 2, or give -pilot")
}

func printPower(p exp.Power) {
	head := fmt.Sprintf("  %s of %s", p.Test, p.Metric)
	if p.Direction != "" {
		head += ", " + p.Direction
	}
	if p.Error != "" {
		fmt.Printf("%s: %s\n", head, p.Error)
		return
	}
	fmt.Printf("%s, alpha %.3g\n", head, p.Alpha)
	from := "the pilot's"
	if p.EffectFrom == "given" {
		from = "given"
	}
	need := ""
	if p.Need > 0 {
		need = fmt.Sprintf(", min_effect %.4g", p.Need)
		if p.Test == "tost" {
			need = fmt.Sprintf(", margin %.4g", p.Need)
		}
	}
	fmt.Printf("    one seed: %s %.4g ± %.3g, %s %.4g ± %.3g (pilot of %d and %d seeds); effect %.4g, %s%s\n",
		p.Treatment, p.Means[0], p.SD[0], p.Control, p.Means[1], p.SD[1], p.Pilot[0], p.Pilot[1], p.Effect, from, need)
	line := fmt.Sprintf("    power %.2f at %s", p.Power, fmt.Sprintf("%d %s", p.Seeds, plural(p.Seeds, "seed", "seeds")))
	if !p.OK() {
		line += fmt.Sprintf(", under %.2g (rule R07)", p.Target)
	}
	if p.Needed > 0 {
		line += fmt.Sprintf("; %.2g needs %s", p.Target, fmt.Sprintf("%d %s", p.Needed, plural(p.Needed, "seed", "seeds")))
	} else {
		line += fmt.Sprintf("; no number of seeds up to the most tried reaches %.2g: claim a smaller effect", p.Target)
	}
	fmt.Println(line)
	if p.Note != "" {
		fmt.Printf("    note: %s\n", p.Note)
	}
}

// parseEffect reads an effect or a spread: a number is absolute, and a number with % is relative to the control mean.
func parseEffect(name, s string) (*exp.Effect, error) {
	if s == "" {
		return nil, nil
	}
	e := &exp.Effect{Kind: "absolute"}
	if v, ok := strings.CutSuffix(s, "%"); ok {
		e.Kind, s = "relative", v
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
		return nil, usageError{fmt.Sprintf("-%s is a number of 0 or more, or a percentage such as 5%%, not %q", name, s)}
	}
	if e.Kind == "relative" {
		v /= 100
	}
	e.Value = v
	return e, nil
}

func cmdExpPlan(args []string) error {
	f := newFlags("exp plan", "[flags] [slug | slug/experiment ...]")
	root := f.String("root", ".", "the root of the content repository")
	class := f.String("class", "", "plan only the experiments of this budget class")
	compute := f.String("compute", exp.ComputeFile, "the register of the GPU pool and the monthly cap")
	ledger := f.String("ledger", exp.LedgerFile, "the ledger of the batches that ran")
	month := f.String("month", "", "the month YYYY-MM of the cap; without it, this month")
	outFile := f.String("o", "", "write the batches to this file as JSON, for aik exp pool")
	asJSON := f.Bool("json", false, "print the plan as JSON")
	if err := parse(f, args); err != nil {
		return err
	}
	if _, ok := exp.Budgets[*class]; *class != "" && !ok {
		return usageError{fmt.Sprintf("-class is cpu-small, gpu-small or gpu-medium, not %q", *class)}
	}
	now := time.Now().UTC()
	if *month != "" {
		t, err := time.Parse("2006-01", *month)
		if err != nil {
			return usageError{fmt.Sprintf("-month is a month YYYY-MM, not %q", *month)}
		}
		now = t
	}
	c, err := load(*root)
	if err != nil {
		return err
	}
	refs, err := selectExps(c, f.Args())
	if err != nil {
		return err
	}
	var items []exp.PlanItem
	var broken []string
	for _, r := range refs {
		ref := r.n.Slug + "/" + r.e.Slug
		if err := r.e.Ready(); err != nil {
			broken = append(broken, fmt.Sprintf("%s: %v", ref, err))
			continue
		}
		if it, ok := exp.Needs(ref, r.e); ok && (*class == "" || it.Class == *class) {
			items = append(items, it)
		}
	}
	var comp *exp.Compute
	if cf := under(c.Root, *compute); exists(cf) {
		if comp, err = exp.ReadCompute(cf); err != nil {
			return err
		}
	}
	led, err := exp.ReadLedger(under(c.Root, *ledger))
	if err != nil {
		return err
	}
	p, err := exp.MakePlan(items, comp, led, now)
	if err != nil {
		return err
	}
	if *outFile != "" {
		b, err := json.MarshalIndent(p.Batches, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(*outFile, append(b, '\n'), 0o644); err != nil {
			return err
		}
	}
	if *asJSON {
		return printJSON(p)
	}
	for _, b := range broken {
		fmt.Printf("cannot run %s\n", b)
	}
	if len(p.CPU) > 0 {
		fmt.Printf("cpu-small, in CI: %d %s\n", len(p.CPU), plural(len(p.CPU), "experiment", "experiments"))
		for _, it := range p.CPU {
			fmt.Printf("  %s (%s)\n", it.Experiment, it.Why)
		}
	}
	if len(p.Batches)+len(p.Waiting) > 0 {
		fmt.Printf("%s: cap $%.2f, spent $%.2f, remaining $%.2f\n", p.Month, p.Cap, p.Spent, p.Remaining)
	}
	for _, b := range p.Batches {
		fmt.Printf("batch %s: %d %s, %.1f GPU-hours, $%.2f\n", b.ID, len(b.Items), plural(len(b.Items), "experiment", "experiments"), b.GPUHours, b.USD)
		for _, it := range b.Items {
			printPlanItem(it)
		}
	}
	if len(p.Waiting) > 0 {
		fmt.Printf("waiting for the next month, past the cap: %d\n", len(p.Waiting))
		for _, it := range p.Waiting {
			printPlanItem(it)
		}
	}
	if len(items)+len(broken) == 0 {
		fmt.Println("every experiment has current results")
	}
	return nil
}

func printPlanItem(it exp.PlanItem) {
	approval := ""
	if it.Approval {
		approval = "; a person approves it first"
	}
	fmt.Printf("  %s %s: %.1f GPU-hours (%s), $%.2f; %s%s\n", it.Class, it.Experiment, it.Hours, it.Estimate, it.USD, it.Why, approval)
}

func cmdExpVerify(args []string) error {
	f := newFlags("exp verify", "[flags] [slug | slug/experiment ...]")
	root := f.String("root", ".", "the root of the content repository")
	outputs := f.String("outputs", "", "the directory of the recorded run of one experiment (arm/seed/metrics.json), such as a copy from ai-runs; without it, the run under build/exp when its run id is the recorded one")
	strict := f.Bool("strict", false, "fail also on results.json whose inputs changed since the run")
	asJSON := f.Bool("json", false, "print the verification of each experiment as JSON")
	if err := parse(f, args); err != nil {
		return err
	}
	c, err := load(*root)
	if err != nil {
		return err
	}
	refs, err := selectExps(c, f.Args())
	if err != nil {
		return err
	}
	if *outputs != "" && len(refs) != 1 {
		return usageError{"-outputs is the run of one experiment: name it as slug/experiment"}
	}
	var out []exp.Verification
	failed := false
	for _, r := range refs {
		if r.e.Results == nil && len(f.Args()) == 0 {
			continue // an experiment that never ran has nothing to verify; aik exp list shows it
		}
		dir := *outputs
		if dir == "" && r.e.Results != nil {
			work := expWork(c, r)
			if last, err := exp.ReadResults(filepath.Join(work, exp.ResultsFile)); err == nil && last.Run == r.e.Results.Run {
				dir = work
			}
		}
		v := exp.Verify(r.n.Slug+"/"+r.e.Slug, r.e, dir)
		failed = failed || !v.OK(*strict)
		out = append(out, v)
	}
	if *asJSON {
		if err := printJSON(out); err != nil {
			return err
		}
	} else {
		for _, v := range out {
			state := "ok"
			switch {
			case len(v.Problems) > 0:
				state = fmt.Sprintf("%d %s", len(v.Problems), plural(len(v.Problems), "problem", "problems"))
			case v.Stale != "":
				state = "stale: " + v.Stale + "; the experiment is at R1 until it runs again"
			}
			where := "the outputs of the run are not here, so its values are checked against themselves"
			if v.Outputs != "" {
				where = "checked against the outputs in " + v.Outputs
			}
			fmt.Printf("%s %s: %s (%s)\n", v.Experiment, v.Run, state, where)
			for _, p := range v.Problems {
				fmt.Printf("  %s\n", p)
			}
		}
	}
	if failed {
		return errFindings
	}
	return nil
}
