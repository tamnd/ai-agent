// SPDX-License-Identifier: Apache-2.0

package lang

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/tamnd/ai-agent/contract"
	"github.com/tamnd/ai-agent/internal/jsonl"
	"github.com/tamnd/ai-agent/node"
	"github.com/tamnd/ai-agent/status"
)

//go:embed codec
var codecs embed.FS

var (
	// HelloTimeout is the longest time that an adapter can take to start. The JVM and .NET start slowly.
	HelloTimeout = 60 * time.Second
	// CaseTimeout is the longest time of one shared case.
	CaseTimeout = 10 * time.Second
)

// testEnv is added to the environment of each build, test and adapter.
var testEnv = []string{"NO_COLOR=1", "CARGO_TERM_COLOR=never", "DOTNET_CLI_TELEMETRY_OPTOUT=1", "DOTNET_NOLOGO=1", "PYTHONDONTWRITEBYTECODE=1"}

// adapterJob is what a generator gets and fills in. The generator writes the adapter files to Dir and sets Build and Run.
type adapterJob struct {
	r    *Runner
	Dir  string
	Node *node.Node
	Impl node.Impl
	K    *contract.Contract
	// Variants are the types of T to build. The first try has int and string for a generic contract; the second try has int only.
	Variants []variant
	// First is true on the first try. A generator can use it to name the type of T, as C# does.
	First bool
	Build [][]string
	Run   []string
}

// variant is the signature of the entry for one type of T.
type variant struct {
	T      string
	Params []contract.Type
	Names  []string
	IsT    []bool // the declared type of the parameter is T itself
	Out    contract.Type
	Inout  int
}

func variants(k *contract.Contract, types []string) []variant {
	var out []variant
	for _, t := range types {
		v := variant{T: t, Inout: k.Output()}
		for _, p := range k.Params {
			ty, _ := contract.Parse(p.Type)
			v.IsT = append(v.IsT, ty.IsVar())
			v.Params = append(v.Params, ty.Subst("T", t))
			v.Names = append(v.Names, p.Name)
		}
		ret, _ := contract.Parse(k.Returns.Type)
		v.Out = ret.Subst("T", t)
		out = append(out, v)
	}
	return out
}

type generator func(j *adapterJob) error

// adapters are the adapter generators of the tier 1 languages (spec 08.5 and 09.2).
var adapters = map[string]generator{
	"python":     pythonAdapter,
	"javascript": jsAdapter,
	"typescript": jsAdapter,
	"go":         goAdapter,
	"rust":       rustAdapter,
	"c":          cAdapter,
	"cpp":        cppAdapter,
	"java":       javaAdapter,
	"kotlin":     kotlinAdapter,
	"csharp":     csharpAdapter,
}

// errNoGenerics is the error of a generator for a language that has no generics, such as C. The entry works on int only (spec 08.3.4), so the cases with another T are skipped, and the skip is expected.
var errNoGenerics = errors.New("the language has no generics, so the entry works on int only (spec 08.3.4)")

// unsupported is the error of a type that the adapter of a language cannot decode yet.
func unsupported(lang string, t contract.Type) error {
	return fmt.Errorf("the %s adapter does not support the type %s yet", lang, t)
}

// Shared runs the shared cases of the contract k on one implementation (spec 09.2). It builds the adapter in a new temporary directory, starts it once and sends the cases in order. A crash fails the case, and the next case gets a new adapter.
func (r *Runner) Shared(n *node.Node, im node.Impl, k *contract.Contract, cases []contract.Case) status.Shared {
	s := status.Shared{Problem: k.Problem, Cases: len(cases)}
	ss, err := r.Open(n, im, k)
	if err != nil {
		s.Error = err.Error()
		return s
	}
	defer ss.Close()
	s.SkipReason, s.SkipExpected = ss.SkipReason, ss.SkipExpected
	fail := func(c contract.Case, reason string, got, want []byte) {
		if c.Flaky {
			reason += "; " + flakyNote(c)
		}
		f := status.CaseFailure{Case: c.ID(), Reason: reason}
		if len(got) > 0 {
			f.Got = contract.Short(compact(got))
		}
		if len(want) > 0 {
			f.Want = contract.Short(compact(want))
		}
		s.Failures = append(s.Failures, f)
	}
	for _, c := range cases {
		if !ss.Supports(c) {
			s.Skipped++
			continue
		}
		got, reason, err := ss.Run(c)
		if err != nil {
			s.Error = err.Error()
			return s
		}
		if reason != "" {
			fail(c, reason, got, c.Expect)
			continue
		}
		if c.Flaky {
			// A flaky case counts as a failure, also when this run passes it (rule S09).
			f := status.CaseFailure{Case: c.ID(), Reason: "the case passes in this run, but " + flakyNote(c)}
			s.Failures = append(s.Failures, f)
			continue
		}
		s.Passed++
	}
	return s
}

// Session is the built adapter of one implementation for one contract, with the custom checker of the contract. It runs one case at a time, so a caller can run many cases, as the differential run and the shrinker do (spec 09.5, 09.11).
type Session struct {
	r      *Runner
	lang   string
	dir    string
	run    []string
	k      *contract.Contract
	custom *contract.Custom
	p      *jsonl.Proc
	// Types are the types of T that the adapter supports. SkipReason says why a type of the contract is not in Types, and SkipExpected tells if the skip is expected (a language without generics).
	Types        []string
	SkipReason   string
	SkipExpected bool
}

// Open builds the adapter of the implementation for the contract in a new temporary directory, and starts the custom checker when the contract has one. Close removes the directory.
func (r *Runner) Open(n *node.Node, im node.Impl, k *contract.Contract) (*Session, error) {
	gen, ok := adapters[im.Lang]
	if !ok {
		return nil, errors.New("no adapter for the language " + im.Lang)
	}
	dir, err := os.MkdirTemp(r.TempDir, "aik-shared-"+im.Lang+"-")
	if err != nil {
		return nil, err
	}
	ss := &Session{r: r, lang: im.Lang, dir: dir, k: k}
	if r.Keep {
		r.logf("keep %s", dir)
	}
	if err := stage(dir, im, false); err != nil {
		ss.Close()
		return nil, err
	}
	types := k.Types()
	run, err := r.buildAdapter(gen, dir, n, im, k, types, true)
	if err != nil && len(types) > 1 {
		first := err
		types = []string{"int"}
		if run, err = r.buildAdapter(gen, dir, n, im, k, types, false); err == nil {
			if errors.Is(first, errNoGenerics) {
				ss.SkipReason, ss.SkipExpected = first.Error(), true
			} else {
				ss.SkipReason = "the entry does not build with T = string: " + errorLine(first.Error())
			}
		}
	}
	if err != nil {
		ss.Close()
		return nil, err
	}
	ss.run, ss.Types = run, types
	if k.Checker.Kind == "custom" {
		if r.Box != nil {
			if err := r.Box.Ready("python"); err != nil {
				ss.Close()
				return nil, errors.New("the custom checker runs in the Python image: " + err.Error())
			}
		}
		if ss.custom, err = k.StartCustom(r.checkerArgv(k)); err != nil {
			ss.Close()
			return nil, err
		}
	}
	return ss, nil
}

// Supports tells if the adapter supports the type of T of the case.
func (ss *Session) Supports(c contract.Case) bool { return slices.Contains(ss.Types, c.Type()) }

// Run runs one case. It gives the result of the call, and the reason why the case fails, which is empty when the case passes. A crash, a timeout or a reply that is not valid fails the case, and the next case gets a new adapter. The error is for a problem of the run itself, such as a checker that does not answer.
func (ss *Session) Run(c contract.Case) (json.RawMessage, string, error) {
	if ss.p == nil {
		p, err := ss.r.startAdapter(ss.lang, ss.dir, ss.run)
		if err != nil {
			return nil, "", err
		}
		ss.p = p
	}
	limit := CaseTimeout
	if ss.r.CaseTimeout > 0 {
		limit = ss.r.CaseTimeout
	}
	line, err := ss.p.Call(map[string]any{"op": "call", "id": c.ID(), "T": c.Type(), "args": c.Args}, limit)
	if err != nil {
		reason := "the adapter crashed: " + err.Error()
		if errors.Is(err, jsonl.ErrTimeout) {
			reason = fmt.Sprintf("no result in %s", limit)
		}
		ss.kill()
		return nil, cut(reason, 1500), nil
	}
	var rep struct {
		ID     string          `json:"id"`
		OK     bool            `json:"ok"`
		Result json.RawMessage `json:"result"`
		Error  string          `json:"error"`
	}
	if err := json.Unmarshal(line, &rep); err != nil || rep.ID != c.ID() {
		ss.kill()
		return nil, "the adapter wrote a line that is not the reply: " + contract.Short(strings.TrimSpace(string(line))), nil
	}
	if !rep.OK {
		return nil, "the call failed: " + cut(rep.Error, 1500), nil
	}
	if len(rep.Result) == 0 {
		rep.Result = json.RawMessage("null")
	}
	reason, err := ss.k.Verdict(c, rep.Result, ss.custom)
	if err != nil {
		return rep.Result, "", err
	}
	return rep.Result, reason, nil
}

func (ss *Session) kill() {
	if ss.p != nil {
		ss.p.Kill()
		ss.p = nil
	}
}

// Close stops the adapter and the checker, and removes the directory unless the runner keeps it.
func (ss *Session) Close() {
	if ss.p != nil {
		ss.p.Close()
		ss.p = nil
	}
	if ss.custom != nil {
		ss.custom.Close()
		ss.custom = nil
	}
	ss.r.reap()
	if !ss.r.Keep {
		_ = os.RemoveAll(ss.dir)
	}
}

// flakyNote says that the case is flaky and names its issue.
func flakyNote(c contract.Case) string {
	if c.Issue == "" {
		return "it is marked flaky, so it counts as a failure"
	}
	return "it is marked flaky (" + c.Issue + "), so it counts as a failure"
}

// compact removes the spaces of a JSON text, so that the outputs of all languages look the same in a report.
func compact(b []byte) string {
	var buf bytes.Buffer
	if json.Compact(&buf, b) != nil {
		return strings.TrimSpace(string(b))
	}
	return buf.String()
}

func (r *Runner) logf(format string, args ...any) {
	if r.Log != nil {
		r.Log(format, args...)
	}
}

// buildAdapter writes the adapter for the types and builds it. It returns the command that starts the adapter.
func (r *Runner) buildAdapter(gen generator, dir string, n *node.Node, im node.Impl, k *contract.Contract, types []string, first bool) ([]string, error) {
	j := &adapterJob{r: r, Dir: dir, Node: n, Impl: im, K: k, Variants: variants(k, types), First: first}
	if err := gen(j); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), Timeout)
	defer cancel()
	for _, c := range j.Build {
		var out bytes.Buffer
		cmd := r.command(ctx, im.Lang, dir, c)
		cmd.Stdout, cmd.Stderr = &out, &out
		if err := cmd.Run(); err != nil {
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return nil, fmt.Errorf("the adapter build took more than %s", Timeout)
			}
			src := Tree{Dir: dir, Sources: map[string]string{}}
			for _, f := range im.Files {
				if !f.Test {
					src.Sources[f.Name] = f.Path
				}
			}
			return nil, fmt.Errorf("the adapter does not build (%s): %s", strings.Join(c[:1], " "), tail(src.Rewrite(out.String()), 1500))
		}
	}
	return j.Run, nil
}

// checkerArgv runs the custom checker of k in the Python image, with the problem directory mounted read-only. The checker is code from the content repository too.
func (r *Runner) checkerArgv(k *contract.Contract) func([]string) []string {
	return CheckerWrap(r.Box, k)
}

// CheckerWrap returns the wrap that runs the custom checker of the contract in the Python image of the sandbox, with the node directory as a read-only mount. A nil sandbox gives a nil wrap, so the checker runs on the host.
func CheckerWrap(box *Sandbox, k *contract.Contract) func([]string) []string {
	if box == nil {
		return nil
	}
	return func(argv []string) []string {
		dir, err := filepath.Abs(k.Dir)
		if err != nil {
			dir = k.Dir
		}
		return box.Argv("python", true, []Mount{{Dir: dir, ReadOnly: true}}, argv)
	}
}

func (r *Runner) startAdapter(lang, dir string, run []string) (*jsonl.Proc, error) {
	if r.Box != nil {
		run = r.Box.Argv(lang, true, []Mount{{Dir: dir}}, run)
	}
	p, err := jsonl.Start(dir, run, testEnv)
	if err != nil {
		return nil, fmt.Errorf("start the adapter: %w", err)
	}
	line, err := p.Call(map[string]string{"op": "hello"}, HelloTimeout)
	if err != nil {
		p.Kill()
		return nil, fmt.Errorf("the adapter did not answer hello: %w", err)
	}
	var h struct {
		Op string `json:"op"`
	}
	if json.Unmarshal(line, &h) != nil || h.Op != "hello" {
		p.Kill()
		return nil, jsonl.Errorf(line, "the adapter did not answer hello")
	}
	return p, nil
}

// stage copies the files of an implementation to dir. With tests false, it copies only the files that are not tests.
func stage(dir string, im node.Impl, tests bool) error {
	for _, f := range im.Files {
		if f.Test && !tests {
			continue
		}
		dst := filepath.Join(dir, filepath.FromSlash(f.Name))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dst, []byte(f.Content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// codec copies a codec file of the agent to dir.
func codec(dir, name, as string) error {
	b, err := codecs.ReadFile("codec/" + name)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, as), b, 0o644)
}

// errorLine returns the first line of s that names an error, or the first line.
func errorLine(s string) string {
	lines := strings.Split(s, "\n")
	// A compiler message has "error:" or "error " with a code, such as "error[E0308]:" in Rust or "error CS1503:" in C#. A count, such as "1 error", is not the message.
	for _, l := range lines {
		low := strings.ToLower(l)
		if strings.Contains(low, "error:") || strings.Contains(low, "error[") || strings.Contains(low, "error cs") {
			return cut(strings.TrimSpace(l), 300)
		}
	}
	for _, l := range lines {
		if strings.Contains(strings.ToLower(l), "error") && !strings.HasPrefix(l, "the adapter does not build") {
			return cut(strings.TrimSpace(l), 300)
		}
	}
	return cut(strings.TrimSpace(lines[0]), 300)
}

func cut(s string, k int) string {
	if len(s) <= k {
		return s
	}
	return s[:k] + "…"
}

func words(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool { return r == '-' || r == '_' || r == ' ' })
}

func snake(s string) string { return strings.Join(words(s), "_") }

func pascal(s string) string {
	w := words(s)
	for i, x := range w {
		w[i] = strings.ToUpper(x[:1]) + x[1:]
	}
	return strings.Join(w, "")
}

func camel(s string) string {
	p := pascal(s)
	if p == "" {
		return p
	}
	return strings.ToLower(p[:1]) + p[1:]
}

// stem returns the file name without its directory and extension.
func stem(name string) string {
	b := filepath.Base(name)
	return strings.TrimSuffix(b, filepath.Ext(b))
}

// helloLine is the reply to hello, as a JSON text.
func helloLine(lang, slug, entry string) string {
	b, _ := json.Marshal(map[string]string{"op": "hello", "lang": lang, "node": slug, "entry": entry})
	return string(b)
}
