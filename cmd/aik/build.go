// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/tamnd/ai-agent/results"
)

// buildStep is the result of one step of aik build.
type buildStep struct {
	Name   string `json:"name"`
	Status string `json:"status"` // ok, failed, cached, skipped or not built
	Note   string `json:"note,omitempty"`
	Log    string `json:"log,omitempty"`
	Ms     int64  `json:"ms"`
}

// runSelf runs aik itself with the arguments and gives its output, as aik gate does. Tests replace it.
var runSelf = func(args []string) ([]byte, error) {
	self, err := os.Executable()
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(context.Background(), self, args...) // #nosec G204 -- aik runs itself
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err = cmd.Run()
	return out.Bytes(), err
}

// buildCache holds the input key of each step that last passed, in build/cache/build.json.
type buildCache map[string]string

// skipDirs are the directories that are not inputs of the build: its own outputs, git, and the caches of the
// toolchains.
var skipDirs = map[string]bool{".git": true, "build": true, ".lake": true, "node_modules": true, "__pycache__": true,
	".venv": true, ".pytest_cache": true, ".ruff_cache": true, ".mypy_cache": true}

// treeHash hashes the paths and the contents of the files under dir, outside skipDirs. It gives "" for a directory
// that does not exist.
func treeHash(dir string) (string, error) {
	if _, err := os.Stat(dir); errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	h := sha256.New()
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != dir && skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		r, _ := filepath.Rel(dir, p)
		fh, err := os.Open(p)
		if err != nil {
			return err
		}
		defer fh.Close()
		fmt.Fprintf(h, "%s\x00", filepath.ToSlash(r))
		if _, err := io.Copy(h, fh); err != nil {
			return err
		}
		_, err = h.Write([]byte{0})
		return err
	})
	return hex.EncodeToString(h.Sum(nil)), err
}

func fileHash(p string) string {
	b, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// cmdBuild runs every part of the build for the whole tree (spec 03.6): the audit, the Lean package, the tests of
// each framework, the computed facts, the results tables, the scenes, the ladders and the coverage. A step whose
// inputs have the same hash as at its last pass is not run again. The build goes on after a step fails, and fails at
// the end.
func cmdBuild(args []string) error {
	f := newFlags("build", "[flags]")
	root := f.String("root", ".", "the root of the content repository")
	sandbox := f.String("sandbox", "auto", "the container engine of the tests and of Lean: docker, podman, none or auto (as aik test)")
	unsafe := f.Bool("unsafe-local", false, "run the tests on this machine, not in the sandbox")
	noTests := f.Bool("no-tests", false, "skip the tests")
	noLean := f.Bool("no-lean", false, "skip the Lean build")
	onMain := f.Bool("main", false, "the build is of the main branch, so provisional ids block (aik check -main)")
	force := f.Bool("force", false, "run every step, whatever the cache says")
	asJSON := f.Bool("json", false, "print the steps as JSON")
	if err := parse(f, args); err != nil {
		return err
	}
	if f.NArg() > 0 {
		return usageError{"aik build takes no arguments; it builds the whole tree"}
	}
	c, err := load(*root)
	if err != nil {
		return err
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	out := filepath.Join(*root, "build")
	logs := filepath.Join(out, "logs")
	if err := os.MkdirAll(logs, 0o755); err != nil {
		return err
	}
	cachePath := filepath.Join(out, "cache", "build.json")
	cache := buildCache{}
	if b, err := os.ReadFile(cachePath); err == nil {
		_ = json.Unmarshal(b, &cache) // a bad cache is an empty cache
	}
	tree, err := treeHash(*root)
	if err != nil {
		return err
	}
	base := tree + fileHash(self) + ver()

	var steps []buildStep
	failed := false
	report := func(s buildStep) {
		steps = append(steps, s)
		if s.Status == "failed" {
			failed = true
		}
		if *asJSON {
			return
		}
		line := fmt.Sprintf("  %-15s %-9s", s.Name, s.Status)
		if s.Ms > 0 {
			line += fmt.Sprintf(" %6.1fs", float64(s.Ms)/1000)
		}
		if s.Note != "" {
			line += "  " + s.Note
		}
		if s.Status == "failed" && s.Log != "" {
			line += "  (see " + s.Log + ")"
		}
		fmt.Println(line)
	}
	// run runs aik itself for a step, unless the inputs have the key of its last pass and its output, when it names
	// one, is still there. An empty key is never cached.
	run := func(name, key, output string, args ...string) {
		if key != "" && !*force && cache[name] == key && (output == "" || exists(output)) {
			report(buildStep{Name: name, Status: "cached", Note: "the inputs are the same as at the last pass"})
			return
		}
		start := time.Now()
		b, err := runSelf(args)
		logPath := filepath.Join(logs, name+".log")
		if werr := os.WriteFile(logPath, b, 0o644); werr != nil {
			err = errors.Join(err, werr)
		}
		s := buildStep{Name: name, Status: "ok", Log: rel(*root, logPath), Ms: time.Since(start).Milliseconds()}
		if err != nil {
			s.Status = "failed"
			delete(cache, name)
			if last := lastLine(string(b)); last != "" {
				s.Note = last
			}
		} else if key != "" {
			cache[name] = key
		}
		report(s)
	}
	skip := func(name, why string) { report(buildStep{Name: name, Status: "skipped", Note: why}) }

	// 1. The audit rules of 15.
	check := []string{"check", "-root", *root}
	if *onMain {
		check = append(check, "-main")
	}
	run("check", "check"+base, "", check...)

	// 2. The Lean package.
	switch {
	case *noLean:
		skip("lean", "-no-lean")
	case !exists(filepath.Join(*root, "lean", "lean-toolchain")):
		skip("lean", "the content tree has no lean/ package")
	default:
		lsb := "none"
		if *sandbox == "docker" || *sandbox == "podman" {
			lsb = *sandbox
		}
		run("lean", "lean"+base, filepath.Join(out, "lean", "decls.jsonl"), "lean", "build", "-root", *root, "-sandbox", lsb)
	}

	// 3. The tests of each implementation, with the result cache of aik test.
	impls := 0
	scenes := 0
	for _, n := range c.Nodes {
		impls += len(n.Impls)
		if n.Scene != "" || n.ArchScene != "" {
			scenes++
		}
	}
	switch {
	case *noTests:
		skip("tests", "-no-tests")
	case impls == 0:
		skip("tests", "no node has an implementation")
	default:
		a := []string{"test", "-root", *root, "-sandbox", *sandbox, "-cache", filepath.Join(out, "cache", "tests")}
		if *unsafe {
			a = append(a, "-unsafe-local")
		}
		run("tests", "", "", a...)
	}

	// 4. The computed facts of each node (04.9), as the build gives them to the site and the API.
	start := time.Now()
	nf := 0
	err = func() error {
		dir := filepath.Join(out, "facts")
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
		for _, n := range c.Nodes {
			if len(n.Facts) == 0 {
				continue
			}
			if err := writeFileJSON(filepath.Join(dir, n.Slug+".json"), n.Facts); err != nil {
				return err
			}
			nf++
		}
		return nil
	}()
	if err != nil {
		return err
	}
	report(buildStep{Name: "facts", Status: "ok", Note: fmt.Sprintf("%d %s with facts", nf, plural(nf, "node", "nodes")) + ", in build/facts", Ms: time.Since(start).Milliseconds()})

	// 5. The computed results table of each benchmark version (10.7).
	start = time.Now()
	reg := results.Load(c)
	nt := 0
	err = func() error {
		dir := filepath.Join(out, "tables")
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
		var benches []string
		for b := range reg.Versions {
			benches = append(benches, b)
		}
		sort.Strings(benches)
		for _, b := range benches {
			var vs []string
			for v := range reg.Versions[b] {
				vs = append(vs, v)
			}
			sort.Strings(vs)
			for _, v := range vs {
				if err := writeFileJSON(filepath.Join(dir, b+"@"+v+".json"), reg.TableOf(b, v, nil)); err != nil {
					return err
				}
				nt++
			}
		}
		return nil
	}()
	if err != nil {
		return err
	}
	s := buildStep{Name: "results tables", Status: "ok", Note: fmt.Sprintf("%d %s, in build/tables", nt, plural(nt, "table", "tables")), Ms: time.Since(start).Milliseconds()}
	if len(reg.VersionErrs) > 0 {
		s.Status, s.Note = "failed", fmt.Sprintf("%d %s does not parse: %v", len(reg.VersionErrs), plural(len(reg.VersionErrs), "benchmark version", "benchmark versions"), reg.VersionErrs[0])
	}
	report(s)

	// 6. The scenes: each builds and each preset runs.
	if scenes == 0 {
		skip("scenes", "no node has a scene")
	} else {
		run("scenes", "scenes"+base, "", "viz", "check", "-root", *root)
	}

	// 7. The graph of 03.4 is not in aik yet.
	report(buildStep{Name: "graph", Status: "not built", Note: "aik has no graph package yet (03.4); the site builds its own links"})

	// 8. The ladders of every node and the coverage report (18.7), from the outputs of the steps above.
	cov := []string{"coverage", "-root", *root, "-o", filepath.Join(out, "coverage.json"), "-ladders", filepath.Join(out, "status.json")}
	if scenes > 0 {
		cov = append(cov, "-viz")
	}
	resHash, err := treeHash(filepath.Join(out, "results"))
	if err != nil {
		return err
	}
	run("coverage", "coverage"+base+resHash+fileHash(filepath.Join(out, "lean", "decls.jsonl")), filepath.Join(out, "status.json"), cov...)

	if b, err := json.MarshalIndent(cache, "", "  "); err == nil {
		if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(cachePath, append(b, '\n'), 0o644); err != nil {
			return err
		}
	}
	if *asJSON {
		if err := printJSON(steps); err != nil {
			return err
		}
	}
	if failed {
		return errFindings
	}
	return nil
}

// lastLine is the last line of a step's output that is not empty, for the summary line.
func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	l := strings.TrimSpace(lines[len(lines)-1])
	if len(l) > 120 {
		l = l[:117] + "..."
	}
	return l
}
