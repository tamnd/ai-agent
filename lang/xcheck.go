// SPDX-License-Identifier: Apache-2.0

package lang

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/tamnd/ai-agent/metric"
	"github.com/tamnd/ai-agent/node"
	"github.com/tamnd/ai-agent/status"
)

// The checks of the execution ladder that aik runs on a framework version besides its native tests (spec 08.10):
// the names of the tests that passed, the metric's test vectors, and the format, lint and type checks.

// JUnitFile is the JUnit report that pytest writes in the build tree.
const JUnitFile = ".aik-junit.xml"

//go:embed aik_vectors.py
var vectorsHarness string

// RuffConfig is the ruff configuration of every framework version (8.3.2). The reference is read more than it is
// edited, so a line may be longer than ruff's default of 88.
const RuffConfig = "line-length = 120\n\n[lint]\nselect = [\"E4\", \"E7\", \"E9\", \"F\", \"B\", \"I\", \"UP\", \"NPY\"]\n"

// PyrightConfig sets the basic level of pyright (8.3.2).
const PyrightConfig = "{\"typeCheckingMode\": \"basic\", \"reportMissingModuleSource\": false}\n"

type junitCase struct {
	Name    string    `xml:"name,attr"`
	Failure *struct{} `xml:"failure"`
	Error   *struct{} `xml:"error"`
	Skipped *struct{} `xml:"skipped"`
}

type junitSuite struct {
	Cases  []junitCase  `xml:"testcase"`
	Suites []junitSuite `xml:"testsuite"`
}

var param = regexp.MustCompile(`\[.*\]$`)

// junitNames reads a JUnit report and gives the tests that passed and the tests that failed, by name with the
// parameters cut. A test passes when no run of it failed and at least one run passed; a test that only skipped is in
// neither list.
func junitNames(b []byte) (passed, failed []string, err error) {
	var root junitSuite
	if err := xml.Unmarshal(b, &root); err != nil {
		return nil, nil, err
	}
	pass, fail := map[string]bool{}, map[string]bool{}
	var walk func(s junitSuite)
	walk = func(s junitSuite) {
		for _, c := range s.Cases {
			name := param.ReplaceAllString(c.Name, "")
			switch {
			case c.Failure != nil || c.Error != nil:
				fail[name] = true
			case c.Skipped == nil:
				pass[name] = true
			}
		}
		for _, sub := range s.Suites {
			walk(sub)
		}
	}
	walk(root)
	for n := range pass {
		if !fail[n] {
			passed = append(passed, n)
		}
	}
	for n := range fail {
		failed = append(failed, n)
	}
	sort.Strings(passed)
	sort.Strings(failed)
	return passed, failed, nil
}

// frameworkChecks runs the checks of 08.10 that a framework version needs besides its tests, in the tree dir where
// the tests ran.
func (r *Runner) frameworkChecks(ctx context.Context, tree Tree, n *node.Node, im node.Impl, t *status.Test) {
	dir := tree.Dir
	if b, err := os.ReadFile(filepath.Join(dir, JUnitFile)); err == nil {
		t.Passed, t.Failed, _ = junitNames(b)
	}
	if n.Kind == "metric" {
		t.Vectors = r.vectors(ctx, dir, n, im)
	}
	if !r.NoLint {
		t.Lint = r.lint(ctx, tree, im)
	}
}

// vectors runs every test vector of the metric through the score function of the implementation (rule X05).
func (r *Runner) vectors(ctx context.Context, dir string, n *node.Node, im node.Impl) *status.Vectors {
	v := &status.Vectors{}
	k, err := metric.Load(n.Dir)
	if err != nil {
		v.Error = "the metric contract: " + err.Error()
		return v
	}
	vs, err := metric.ReadVectors(n.Dir, k)
	if err != nil {
		v.Error = err.Error()
		return v
	}
	if len(vs) == 0 {
		v.Error = "the metric has no test vectors in " + metric.VectorsDir + "/ (08.7.5)"
		return v
	}
	mods := files(im, false, ".py")
	if len(mods) != 1 {
		v.Error = fmt.Sprintf("the score function must be in one source file, and the implementation has %d", len(mods))
		return v
	}
	if err := write(dir, "aik_vectors.py", vectorsHarness); err != nil {
		v.Error = err.Error()
		return v
	}
	vfiles, _ := metric.Files(n.Dir)
	c := append([]string{"python3", "-B", "aik_vectors.py", strings.TrimSuffix(mods[0], ".py"), k.Entry,
		strconv.FormatFloat(k.Tolerance.Rtol, 'g', -1, 64), strconv.FormatFloat(k.Tolerance.Atol, 'g', -1, 64)}, vfiles...)
	var out, errOut bytes.Buffer
	cmd := r.command(ctx, im.Lang, dir, c)
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if err := cmd.Run(); err != nil {
		v.Error = "the vectors did not run: " + tail(errOut.String()+err.Error(), 1000)
		return v
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	var res struct {
		Cases    int                  `json:"cases"`
		Failures []status.CaseFailure `json:"failures"`
		Error    string               `json:"error"`
	}
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &res); err != nil {
		v.Error = "the vector harness wrote no result: " + tail(out.String()+errOut.String(), 1000)
		return v
	}
	if res.Error != "" {
		v.Error = "the score function does not load: " + tail(res.Error, 1000)
		return v
	}
	v.Cases, v.Failures = res.Cases, res.Failures
	v.Passed = res.Cases - len(res.Failures)
	if len(v.Failures) > MaxDiffFailures {
		v.Failures = v.Failures[:MaxDiffFailures]
	}
	return v
}

// lint runs ruff format, ruff check and pyright on the files of the implementation (8.3.2, rule X13). A tool that is not
// installed gives a check with Missing set.
func (r *Runner) lint(ctx context.Context, tree Tree, im node.Impl) []status.Lint {
	dir := tree.Dir
	py := all(im, ".py")
	if len(py) == 0 {
		return nil
	}
	if err := write(dir, "ruff.toml", RuffConfig); err != nil {
		return []status.Lint{{Tool: "ruff", Output: err.Error()}}
	}
	if err := write(dir, "pyrightconfig.json", PyrightConfig); err != nil {
		return []status.Lint{{Tool: "pyright", Output: err.Error()}}
	}
	checks := []struct {
		tool string
		cmd  []string
	}{
		{"ruff format", append([]string{"ruff", "format", "--check", "--diff", "--no-cache"}, py...)},
		{"ruff check", append([]string{"ruff", "check", "--no-cache", "--output-format", "concise"}, py...)},
		// basedpyright is pyright with its own Node.js in the wheel, so the image needs no network; it reads the same
		// configuration.
		{"pyright", append([]string{"sh", "-c", `if command -v basedpyright >/dev/null 2>&1; then exec basedpyright "$@"; elif command -v pyright >/dev/null 2>&1; then exec pyright "$@"; else exit 127; fi`, "sh"}, py...)},
	}
	var out []status.Lint
	for _, c := range checks {
		l := status.Lint{Tool: c.tool}
		var buf bytes.Buffer
		cmd := r.command(ctx, im.Lang, dir, c.cmd)
		cmd.Stdout, cmd.Stderr = &buf, &buf
		err := cmd.Run()
		var exit *exec.ExitError
		switch {
		case err == nil:
			l.Pass = true
		case errors.Is(err, exec.ErrNotFound), errors.As(err, &exit) && exit.ExitCode() == 127:
			l.Missing = true
		default:
			l.Output = head(lintFirst(tree.Rewrite(buf.String())), 2000)
		}
		out = append(out, l)
	}
	return out
}

// head keeps the first k bytes of a tool's output, where the first findings are, at a line break.
func head(s string, k int) string {
	if len(s) <= k {
		return s
	}
	s = s[:k]
	if i := strings.LastIndexByte(s, '\n'); i > 0 {
		s = s[:i]
	}
	return s + "\n..."
}

var lintLoc = regexp.MustCompile(`:\d+:\d+`)

// lintFirst moves the first line that points at a place in the code to the top of a tool's output, as pyright prints
// the name of the file first.
func lintFirst(s string) string {
	s = strings.TrimSpace(s)
	for i, line := range strings.Split(s, "\n") {
		if lintLoc.MatchString(line) {
			if i == 0 {
				return s
			}
			return strings.TrimSpace(line) + "\n" + s
		}
	}
	return s
}
