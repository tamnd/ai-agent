// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/tamnd/ai-agent/exp"
	"github.com/tamnd/ai-agent/lang"
	"github.com/tamnd/ai-agent/node"
)

// cmdExp runs the experiments of spec 09.
func cmdExp(args []string) error {
	if len(args) == 0 {
		return usageError{"use aik exp list or aik exp run"}
	}
	switch args[0] {
	case "list":
		return cmdExpList(args[1:])
	case "run":
		return cmdExpRun(args[1:])
	}
	return usageError{"use aik exp list or aik exp run, not aik exp " + args[0]}
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
	if err := parse(f, args); err != nil {
		return err
	}
	if f.NArg() == 0 {
		return usageError{"name the experiments to run: slug or slug/experiment"}
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
	work, err := filepath.Abs(filepath.Join(c.Root, "build", "exp", r.n.Slug, e.Slug))
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
	if !update {
		fmt.Println("  results.json is not written; give -update to write it")
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
