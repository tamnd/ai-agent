// SPDX-License-Identifier: Apache-2.0

package lang

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tamnd/ai-agent/contract"
)

// The generators below write the adapter of each language. Each adapter answers hello, then reads one call per line, decodes the arguments to the native types of spec 08.3.3, calls the entry, and writes the result. For an inout parameter, the result is the value of the parameter after the call. The entry has the name of the algorithm by the case rule of the language (spec 08.3.3): binary_search in Python, binarySearch in JavaScript, BinarySearch.search in Java.

func elem(t contract.Type) contract.Type { return t.Args[0] }

func isList(t contract.Type) bool { return t.Name == "list" }

// Python.

func pyDec(t contract.Type, e string, depth int) (string, error) {
	switch t.Name {
	case "int", "index":
		return "int(" + e + ")", nil
	case "bool", "string", "float":
		return e, nil
	case "list":
		x := fmt.Sprintf("x%d", depth)
		d, err := pyDec(elem(t), x, depth+1)
		if err != nil {
			return "", err
		}
		return "[" + d + " for " + x + " in " + e + "]", nil
	}
	return "", unsupported("python", t)
}

const pyMain = `
OUT = sys.stdout
sys.stdout = sys.stderr

for line in sys.stdin:
    if not line.strip():
        continue
    q = json.loads(line)
    op = q.get("op")
    if op == "bye":
        break
    if op == "hello":
        OUT.write(HELLO + "\n")
        OUT.flush()
        continue
    try:
        f = CALLS.get(q.get("T", "int"))
        if f is None:
            raise ValueError("no variant for T = " + str(q.get("T")))
        r, ns = f(q["args"])
        reply = {"id": q["id"], "ok": True, "result": r, "ns": ns}
    except Exception as e:
        reply = {"id": q["id"], "ok": False, "error": type(e).__name__ + ": " + str(e)}
    OUT.write(json.dumps(reply, separators=(",", ":")) + "\n")
    OUT.flush()
`

func pythonAdapter(j *adapterJob) error {
	main := j.Impl.Main()
	if main == nil {
		return errors.New("python: no source file")
	}
	fn := snake(j.Node.Slug)
	var b strings.Builder
	fmt.Fprintf(&b, "import json, os, sys, time\nsys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))\nfrom %s import %s as entry\n\nHELLO = %q\n", stem(main.Name), fn, helloLine("python", j.Node.Slug, fn))
	var calls []string
	for _, v := range j.Variants {
		fmt.Fprintf(&b, "\ndef call_%s(a):\n", v.T)
		var args []string
		for i, t := range v.Params {
			d, err := pyDec(t, fmt.Sprintf("a[%q]", v.Names[i]), 0)
			if err != nil {
				return err
			}
			fmt.Fprintf(&b, "    a%d = %s\n", i, d)
			args = append(args, fmt.Sprintf("a%d", i))
		}
		fmt.Fprintf(&b, "    t0 = time.perf_counter_ns()\n    r = entry(%s)\n    ns = time.perf_counter_ns() - t0\n", strings.Join(args, ", "))
		if v.Inout >= 0 {
			fmt.Fprintf(&b, "    r = a%d\n", v.Inout)
		}
		b.WriteString("    return r, ns\n")
		calls = append(calls, fmt.Sprintf("%q: call_%s", v.T, v.T))
	}
	fmt.Fprintf(&b, "\nCALLS = {%s}\n", strings.Join(calls, ", "))
	b.WriteString(pyMain)
	if err := write(j.Dir, "algo_adapter.py", b.String()); err != nil {
		return err
	}
	j.Run = []string{"python3", "-I", "-B", "algo_adapter.py"}
	return nil
}

// JavaScript and TypeScript. Node runs TypeScript by type stripping, and the adapter has no type annotations, so one generator writes both.

func jsDec(t contract.Type, e string, depth int) (string, error) {
	switch t.Name {
	case "int":
		return "toInt(" + e + ")", nil
	case "index":
		return "Number(" + e + ")", nil
	case "bool", "string", "float":
		return e, nil
	case "list":
		x := fmt.Sprintf("x%d", depth)
		d, err := jsDec(elem(t), x, depth+1)
		if err != nil {
			return "", err
		}
		return "(" + e + ").map((" + x + ") => " + d + ")", nil
	}
	return "", unsupported("javascript", t)
}

const jsMain = `
const out = process.stdout;
console.log = console.error;
const rl = createInterface({ input: process.stdin, crlfDelay: Infinity });
const enc = (_k, v) => (typeof v === "bigint" ? (Number.isSafeInteger(Number(v)) ? Number(v) : v.toString()) : v);
for await (const line of rl) {
  if (line.trim() === "") continue;
  const q = JSON.parse(line);
  if (q.op === "bye") break;
  if (q.op === "hello") {
    out.write(HELLO + "\n");
    continue;
  }
  let reply;
  try {
    const f = CALLS[q.T ?? "int"];
    if (f === undefined) throw new Error("no variant for T = " + q.T);
    const [r, ns] = f(q.args);
    reply = { id: q.id, ok: true, result: r === undefined ? null : r, ns };
  } catch (e) {
    reply = { id: q.id, ok: false, error: String(e instanceof Error ? e.name + ": " + e.message : e) };
  }
  out.write(JSON.stringify(reply, enc) + "\n");
}
rl.close();
`

func jsAdapter(j *adapterJob) error {
	main := j.Impl.Main()
	if main == nil {
		return errors.New(j.Impl.Lang + ": no source file")
	}
	fn := camel(j.Node.Slug)
	file := "algo_adapter.mjs"
	if j.Impl.Lang == "typescript" {
		file = "algo_adapter.ts"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "import { createInterface } from \"node:readline\";\nimport { %s as entry } from \"./%s\";\n\nconst HELLO = %q;\nconst toInt = (v) => (typeof v === \"string\" ? BigInt(v) : v);\n", fn, filepath.ToSlash(main.Name), helloLine(j.Impl.Lang, j.Node.Slug, fn))
	var calls []string
	for _, v := range j.Variants {
		fmt.Fprintf(&b, "\nfunction call_%s(a) {\n", v.T)
		var args []string
		for i, t := range v.Params {
			d, err := jsDec(t, fmt.Sprintf("a[%q]", v.Names[i]), 0)
			if err != nil {
				return err
			}
			fmt.Fprintf(&b, "  const a%d = %s;\n", i, d)
			args = append(args, fmt.Sprintf("a%d", i))
		}
		fmt.Fprintf(&b, "  const t0 = process.hrtime.bigint();\n  let r = entry(%s);\n  const ns = Number(process.hrtime.bigint() - t0);\n", strings.Join(args, ", "))
		if v.Inout >= 0 {
			fmt.Fprintf(&b, "  r = a%d;\n", v.Inout)
		}
		b.WriteString("  return [r, ns];\n}\n")
		calls = append(calls, fmt.Sprintf("%s: call_%s", v.T, v.T))
	}
	fmt.Fprintf(&b, "\nconst CALLS = { %s };\n", strings.Join(calls, ", "))
	b.WriteString(jsMain)
	if err := write(j.Dir, "package.json", "{\"type\": \"module\", \"private\": true}\n"); err != nil {
		return err
	}
	if err := write(j.Dir, file, b.String()); err != nil {
		return err
	}
	j.Run = []string{"node", file}
	return nil
}

// Go.

func goType(t contract.Type) (string, error) {
	switch t.Name {
	case "int":
		return "int64", nil
	case "index":
		return "int", nil
	case "bool":
		return "bool", nil
	case "string":
		return "string", nil
	case "float":
		return "float64", nil
	case "list":
		e, err := goType(elem(t))
		return "[]" + e, err
	}
	return "", unsupported("go", t)
}

func goDec(t contract.Type, e string, depth int) (string, error) {
	switch t.Name {
	case "int":
		return "toInt(" + e + ")", nil
	case "index":
		return "int(toInt(" + e + "))", nil
	case "bool":
		return e + ".(bool)", nil
	case "string":
		return e + ".(string)", nil
	case "float":
		return "toFloat(" + e + ")", nil
	case "list":
		et, err := goType(elem(t))
		if err != nil {
			return "", err
		}
		x := fmt.Sprintf("x%d", depth)
		d, err := goDec(elem(t), x, depth+1)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("toList(%s, func(%s any) %s { return %s })", e, x, et, d), nil
	}
	return "", unsupported("go", t)
}

const goHead = `package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	impl %q
)

var _ = strconv.ParseInt

func toInt(v any) int64 {
	var s string
	switch x := v.(type) {
	case json.Number:
		s = x.String()
	case string:
		s = x
	default:
		panic(fmt.Sprintf("%%v is not an integer", v))
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		panic(err)
	}
	return n
}

func toFloat(v any) float64 {
	f, err := v.(json.Number).Float64()
	if err != nil {
		panic(err)
	}
	return f
}

func toList[T any](v any, f func(any) T) []T {
	xs := v.([]any)
	out := make([]T, len(xs))
	for i, x := range xs {
		out[i] = f(x)
	}
	return out
}
`

const goMain = `
type query struct {
	Op   string         ` + "`json:\"op\"`" + `
	ID   string         ` + "`json:\"id\"`" + `
	T    string         ` + "`json:\"T\"`" + `
	Args map[string]any ` + "`json:\"args\"`" + `
}

type reply struct {
	ID     string ` + "`json:\"id\"`" + `
	OK     bool   ` + "`json:\"ok\"`" + `
	Result any    ` + "`json:\"result\"`" + `
	NS     int64  ` + "`json:\"ns,omitempty\"`" + `
	Error  string ` + "`json:\"error,omitempty\"`" + `
}

func call(q query) (rep reply) {
	rep.ID = q.ID
	defer func() {
		if e := recover(); e != nil {
			rep = reply{ID: q.ID, Error: fmt.Sprintf("panic: %v", e)}
		}
	}()
	t := q.T
	if t == "" {
		t = "int"
	}
	f, ok := calls[t]
	if !ok {
		return reply{ID: q.ID, Error: "no variant for T = " + t}
	}
	r, ns := f(q.Args)
	return reply{ID: q.ID, OK: true, Result: r, NS: ns}
}

func main() {
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 1<<20), 1<<30)
	out := bufio.NewWriter(os.Stdout)
	for in.Scan() {
		if len(bytes.TrimSpace(in.Bytes())) == 0 {
			continue
		}
		d := json.NewDecoder(bytes.NewReader(in.Bytes()))
		d.UseNumber()
		var q query
		if err := d.Decode(&q); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		switch q.Op {
		case "bye":
			return
		case "hello":
			out.WriteString(hello + "\n")
		default:
			b, err := json.Marshal(call(q))
			if err != nil {
				b, _ = json.Marshal(reply{ID: q.ID, Error: err.Error()})
			}
			out.Write(append(b, '\n'))
		}
		out.Flush()
	}
}
`

// goModule returns the module path of the staged files, and writes go.mod when the implementation has none.
func goModule(dir string) (string, error) {
	b, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return "example.com/impl", write(dir, "go.mod", "module example.com/impl\n\ngo 1.24\n")
	}
	for _, l := range strings.Split(string(b), "\n") {
		if m, ok := strings.CutPrefix(strings.TrimSpace(l), "module "); ok {
			return strings.Trim(strings.TrimSpace(m), `"`), nil
		}
	}
	return "", errors.New("go: go.mod has no module line")
}

func goAdapter(j *adapterJob) error {
	mod, err := goModule(j.Dir)
	if err != nil {
		return err
	}
	fn := pascal(j.K.Entry)
	var b strings.Builder
	fmt.Fprintf(&b, goHead, mod)
	fmt.Fprintf(&b, "\nconst hello = %q\n", helloLine("go", j.Node.Slug, fn))
	var calls []string
	for _, v := range j.Variants {
		fmt.Fprintf(&b, "\nfunc call_%s(a map[string]any) (any, int64) {\n", v.T)
		var args []string
		for i, t := range v.Params {
			d, err := goDec(t, fmt.Sprintf("a[%q]", v.Names[i]), 0)
			if err != nil {
				return err
			}
			fmt.Fprintf(&b, "\ta%d := %s\n", i, d)
			args = append(args, fmt.Sprintf("a%d", i))
		}
		b.WriteString("\tt0 := time.Now()\n")
		call := fmt.Sprintf("impl.%s(%s)", fn, strings.Join(args, ", "))
		if v.Inout >= 0 {
			fmt.Fprintf(&b, "\t%s\n\tns := time.Since(t0).Nanoseconds()\n\treturn a%d, ns\n}\n", call, v.Inout)
		} else {
			fmt.Fprintf(&b, "\tr := %s\n\tns := time.Since(t0).Nanoseconds()\n\treturn r, ns\n}\n", call)
		}
		calls = append(calls, fmt.Sprintf("%q: call_%s", v.T, v.T))
	}
	fmt.Fprintf(&b, "\nvar calls = map[string]func(map[string]any) (any, int64){%s}\n", strings.Join(calls, ", "))
	b.WriteString(goMain)
	if err := os.MkdirAll(filepath.Join(j.Dir, "algoadapter"), 0o755); err != nil {
		return err
	}
	if err := write(j.Dir, "algoadapter/main.go", b.String()); err != nil {
		return err
	}
	j.Build = [][]string{{"go", "build", "-o", "algo_adapter", "./algoadapter"}}
	j.Run = []string{"./algo_adapter"}
	return nil
}

// Rust.

func rustType(t contract.Type) (string, error) {
	switch t.Name {
	case "int":
		return "i64", nil
	case "index":
		return "usize", nil
	case "bool":
		return "bool", nil
	case "string":
		return "String", nil
	case "list":
		e, err := rustType(elem(t))
		return "Vec<" + e + ">", err
	}
	return "", unsupported("rust", t)
}

const rustMain = `
fn main() {
    let stdin = std::io::stdin();
    let mut out = std::io::stdout().lock();
    for line in stdin.lock().lines() {
        let line = match line {
            Ok(l) => l,
            Err(_) => break,
        };
        if line.trim().is_empty() {
            continue;
        }
        let q = match parse(&line) {
            Ok(q) => q,
            Err(e) => {
                eprintln!("{e}");
                std::process::exit(2);
            }
        };
        let op = q.get("op").ok().and_then(|v| v.as_str()).unwrap_or("");
        if op == "bye" {
            break;
        }
        if op == "hello" {
            let _ = writeln!(out, "{}", HELLO);
            let _ = out.flush();
            continue;
        }
        let id = q.get("id").ok().and_then(|v| v.as_str()).unwrap_or("").to_string();
        let t = q.get("T").ok().and_then(|v| v.as_str()).unwrap_or("int").to_string();
        let r = catch_unwind(AssertUnwindSafe(|| -> Result<(String, u128), String> {
            let a = q.get("args")?;
            call(&t, a)
        }));
        let text = match r {
            Ok(Ok((res, ns))) => format!("{{\"id\":{},\"ok\":true,\"result\":{},\"ns\":{}}}", quote(&id), res, ns),
            Ok(Err(e)) => format!("{{\"id\":{},\"ok\":false,\"error\":{}}}", quote(&id), quote(&e)),
            Err(p) => {
                let msg = if let Some(s) = p.downcast_ref::<&str>() {
                    s.to_string()
                } else if let Some(s) = p.downcast_ref::<String>() {
                    s.clone()
                } else {
                    "a panic".to_string()
                };
                format!("{{\"id\":{},\"ok\":false,\"error\":{}}}", quote(&id), quote(&format!("panic: {msg}")))
            }
        };
        let _ = writeln!(out, "{text}");
        let _ = out.flush();
    }
}
`

func rustAdapter(j *adapterJob) error {
	main := files(j.Impl, false, ".rs")
	if len(main) == 0 {
		return errors.New("rust: no source file")
	}
	fn := snake(j.Node.Slug)
	toml := fmt.Sprintf("[package]\nname = %q\nversion = \"0.0.0\"\nedition = \"2024\"\npublish = false\n\n[lib]\npath = %q\n\n[[bin]]\nname = \"algo_adapter\"\npath = \"algo_adapter/main.rs\"\n", crate(j.Node.Slug), main[0])
	if err := write(j.Dir, "Cargo.toml", toml); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(j.Dir, "algo_adapter"), 0o755); err != nil {
		return err
	}
	if err := codec(j.Dir, "algo_json.rs", "algo_adapter/algo_json.rs"); err != nil {
		return err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "mod algo_json;\n\nuse algo_json::{parse, quote, FromJ, ToJ, V};\nuse std::io::{BufRead, Write};\nuse std::panic::{catch_unwind, AssertUnwindSafe};\nuse std::time::Instant;\nuse %s::%s as entry;\n\nconst HELLO: &str = %q;\n", crate(j.Node.Slug), fn, helloLine("rust", j.Node.Slug, fn))
	var arms []string
	for _, v := range j.Variants {
		fmt.Fprintf(&b, "\nfn call_%s(a: &V) -> Result<(String, u128), String> {\n", v.T)
		var args []string
		for i, t := range v.Params {
			ty, err := rustType(t)
			if err != nil {
				return err
			}
			mut := ""
			if i == v.Inout {
				mut = "mut "
			}
			fmt.Fprintf(&b, "    let %sa%d: %s = FromJ::from_j(a.get(%q)?)?;\n", mut, i, ty, v.Names[i])
			switch {
			case isList(t) && i == v.Inout:
				args = append(args, fmt.Sprintf("&mut a%d[..]", i))
			case isList(t):
				args = append(args, fmt.Sprintf("&a%d[..]", i))
			case v.IsT[i] || t.Name == "string":
				args = append(args, fmt.Sprintf("&a%d", i))
			default:
				args = append(args, fmt.Sprintf("a%d", i))
			}
		}
		b.WriteString("    let t0 = Instant::now();\n")
		if v.Inout >= 0 {
			fmt.Fprintf(&b, "    entry(%s);\n    let ns = t0.elapsed().as_nanos();\n    let mut o = String::new();\n    a%d.to_j(&mut o);\n", strings.Join(args, ", "), v.Inout)
		} else {
			fmt.Fprintf(&b, "    let r = entry(%s);\n    let ns = t0.elapsed().as_nanos();\n    let mut o = String::new();\n    r.to_j(&mut o);\n", strings.Join(args, ", "))
		}
		b.WriteString("    Ok((o, ns))\n}\n")
		arms = append(arms, fmt.Sprintf("        %q => call_%s(a),\n", v.T, v.T))
	}
	fmt.Fprintf(&b, "\nfn call(t: &str, a: &V) -> Result<(String, u128), String> {\n    match t {\n%s        _ => Err(format!(\"no variant for T = {t}\")),\n    }\n}\n", strings.Join(arms, ""))
	b.WriteString(rustMain)
	if err := write(j.Dir, "algo_adapter/main.rs", b.String()); err != nil {
		return err
	}
	j.Build = [][]string{{"cargo", "build", "--offline", "--quiet", "--bin", "algo_adapter"}}
	j.Run = []string{"target/debug/algo_adapter"}
	return nil
}

// C. C has no generics, so the C adapter has only the int variant (spec 08.3.4).

var sanitize = []string{"-g", "-O1", "-fsanitize=address,undefined", "-fno-sanitize-recover=all", "-fno-omit-frame-pointer"}

const cNow = `static long long now_ns(void) {
    struct timespec t;
    clock_gettime(CLOCK_MONOTONIC, &t);
    return (long long)t.tv_sec * 1000000000LL + (long long)t.tv_nsec;
}
`

const cMain = `
int main(void) {
    char *line = NULL;
    size_t cap = 0;
    while (getline(&line, &cap, stdin) > 0) {
        aj *q = aj_parse(line);
        if (q == NULL) {
            fprintf(stderr, "the line is not JSON\n");
            return 2;
        }
        const aj *op = aj_get(q, "op");
        if (op != NULL && op->kind == AJ_STR && strcmp(op->s, "bye") == 0) {
            aj_free(q);
            break;
        }
        if (op != NULL && op->kind == AJ_STR && strcmp(op->s, "hello") == 0) {
            fputs(HELLO "\n", stdout);
            fflush(stdout);
            aj_free(q);
            continue;
        }
        const aj *id = aj_get(q, "id");
        const aj *t = aj_get(q, "T");
        fputs("{\"id\":", stdout);
        aj_put_str(id->s, id->len);
        if (t != NULL && t->kind == AJ_STR && strcmp(t->s, "int") != 0) {
            fputs(",\"ok\":false,\"error\":\"the C entry has only the int variant\"}\n", stdout);
        } else {
            fputs(",\"ok\":true,\"result\":", stdout);
            long long ns = call_int(aj_get(q, "args"));
            printf(",\"ns\":%lld}\n", ns);
        }
        fflush(stdout);
        aj_free(q);
    }
    free(line);
    return 0;
}
`

func cAdapter(j *adapterJob) error {
	fn := snake(j.Node.Slug)
	var b strings.Builder
	b.WriteString("#define _POSIX_C_SOURCE 200809L\n#include <stdbool.h>\n#include <stdint.h>\n#include <stdio.h>\n#include <stdlib.h>\n#include <string.h>\n#include <time.h>\n\n#include \"algo_json.h\"\n")
	for _, h := range files(j.Impl, false, ".h") {
		fmt.Fprintf(&b, "#include %q\n", h)
	}
	fmt.Fprintf(&b, "\n#define HELLO %q\n\n%s", helloLine("c", j.Node.Slug, fn), cNow)
	if len(j.Variants) != 1 || j.Variants[0].T != "int" {
		return errNoGenerics
	}
	v := &j.Variants[0]
	b.WriteString("\n// call_int writes the result and returns the time of the call.\nstatic long long call_int(const aj *a) {\n")
	var args, frees []string
	for i, t := range v.Params {
		get := fmt.Sprintf("aj_get(a, %q)", v.Names[i])
		switch {
		case t.Name == "int":
			fmt.Fprintf(&b, "    int64_t a%d = aj_int(%s);\n", i, get)
			args = append(args, fmt.Sprintf("a%d", i))
		case t.Name == "index":
			fmt.Fprintf(&b, "    size_t a%d = (size_t)aj_int(%s);\n", i, get)
			args = append(args, fmt.Sprintf("a%d", i))
		case t.Name == "bool":
			fmt.Fprintf(&b, "    bool a%d = %s->b;\n", i, get)
			args = append(args, fmt.Sprintf("a%d", i))
		case isList(t) && elem(t).Name == "int":
			fmt.Fprintf(&b, "    size_t n%d = 0;\n    int64_t *a%d = aj_int_list(%s, &n%d);\n", i, i, get, i)
			args = append(args, fmt.Sprintf("a%d", i), fmt.Sprintf("n%d", i))
			frees = append(frees, fmt.Sprintf("    free(a%d);\n", i))
		default:
			return unsupported("c", t)
		}
	}
	call := fmt.Sprintf("%s(%s)", fn, strings.Join(args, ", "))
	b.WriteString("    long long t0 = now_ns();\n")
	switch {
	case v.Inout >= 0:
		if !isList(v.Params[v.Inout]) {
			return unsupported("c", v.Params[v.Inout])
		}
		fmt.Fprintf(&b, "    (void)%s;\n    long long ns = now_ns() - t0;\n    aj_put_int_list(a%d, n%d);\n", call, v.Inout, v.Inout)
	case v.Out.Name == "int" || v.Out.Name == "index" || v.Out.Name == "option" && (elem(v.Out).Name == "int" || elem(v.Out).Name == "index"):
		fmt.Fprintf(&b, "    long long r = (long long)%s;\n    long long ns = now_ns() - t0;\n    aj_put_int(r);\n", call)
	case v.Out.Name == "bool":
		fmt.Fprintf(&b, "    bool r = %s;\n    long long ns = now_ns() - t0;\n    fputs(r ? \"true\" : \"false\", stdout);\n", call)
	default:
		return unsupported("c", v.Out)
	}
	b.WriteString(strings.Join(frees, ""))
	b.WriteString("    return ns;\n}\n")
	b.WriteString(cMain)
	for _, f := range []string{"algo_json.h", "algo_json.c"} {
		if err := codec(j.Dir, f, f); err != nil {
			return err
		}
	}
	if err := write(j.Dir, "algo_adapter.c", b.String()); err != nil {
		return err
	}
	args = append([]string{"cc", "-std=c2x"}, sanitize...)
	args = append(append(args, "algo_adapter.c", "algo_json.c"), files(j.Impl, false, ".c")...)
	j.Build = [][]string{append(args, "-o", "algo_adapter")}
	j.Run = []string{"./algo_adapter"}
	return nil
}

// C++.

func cppType(t contract.Type) (string, error) {
	switch t.Name {
	case "int":
		return "std::int64_t", nil
	case "index":
		return "std::size_t", nil
	case "bool":
		return "bool", nil
	case "string":
		return "std::string", nil
	case "list":
		e, err := cppType(elem(t))
		return "std::vector<" + e + ">", err
	}
	return "", unsupported("cpp", t)
}

const cppMain = `
int main() {
    std::ios::sync_with_stdio(false);
    std::string line;
    while (std::getline(std::cin, line)) {
        if (line.find_first_not_of(" \t\r") == std::string::npos) continue;
        aj::V q = aj::parse(line);
        std::string op = q.has("op") ? q["op"].s : "";
        if (op == "bye") break;
        if (op == "hello") {
            std::cout << HELLO << '\n' << std::flush;
            continue;
        }
        std::string id = q["id"].s;
        std::string t = q.has("T") ? q["T"].s : "int";
        std::string o = "{\"id\":";
        aj::quote(o, id);
        try {
            long long ns = 0;
            std::string r = call(t, q["args"], ns);
            o += ",\"ok\":true,\"result\":" + r + ",\"ns\":" + std::to_string(ns) + "}";
        } catch (const std::exception& e) {
            o = "{\"id\":";
            aj::quote(o, id);
            o += ",\"ok\":false,\"error\":";
            aj::quote(o, std::string("exception: ") + e.what());
            o += "}";
        }
        std::cout << o << '\n' << std::flush;
    }
    return 0;
}
`

func cppAdapter(j *adapterJob) error {
	fn := snake(j.Node.Slug)
	var b strings.Builder
	b.WriteString("#include <chrono>\n#include <cstdint>\n#include <iostream>\n#include <optional>\n#include <span>\n#include <stdexcept>\n#include <string>\n#include <vector>\n\n#include \"algo_json.hpp\"\n")
	for _, h := range files(j.Impl, false, ".hpp", ".h", ".hh") {
		fmt.Fprintf(&b, "#include %q\n", h)
	}
	fmt.Fprintf(&b, "\nnamespace aik {}\nusing namespace aik;\nnamespace aj = algo_json;\n\nstatic const char* HELLO = %q;\n", helloLine("cpp", j.Node.Slug, fn))
	var arms []string
	for _, v := range j.Variants {
		fmt.Fprintf(&b, "\nstatic std::string call_%s(const aj::V& a, long long& ns) {\n", v.T)
		var args []string
		for i, t := range v.Params {
			ty, err := cppType(t)
			if err != nil {
				return err
			}
			fmt.Fprintf(&b, "    auto a%d = aj::from<%s>(a[%q]);\n", i, ty, v.Names[i])
			if isList(t) {
				et, _ := cppType(elem(t))
				if i == v.Inout {
					args = append(args, fmt.Sprintf("std::span<%s>(a%d)", et, i))
				} else {
					args = append(args, fmt.Sprintf("std::span<const %s>(a%d)", et, i))
				}
			} else {
				args = append(args, fmt.Sprintf("a%d", i))
			}
		}
		call := fmt.Sprintf("%s(%s)", fn, strings.Join(args, ", "))
		b.WriteString("    auto t0 = std::chrono::steady_clock::now();\n")
		if v.Inout >= 0 {
			fmt.Fprintf(&b, "    %s;\n", call)
		} else {
			fmt.Fprintf(&b, "    auto r = %s;\n", call)
		}
		b.WriteString("    ns = std::chrono::duration_cast<std::chrono::nanoseconds>(std::chrono::steady_clock::now() - t0).count();\n    std::string o;\n")
		if v.Inout >= 0 {
			fmt.Fprintf(&b, "    aj::put(o, a%d);\n", v.Inout)
		} else {
			b.WriteString("    aj::put(o, r);\n")
		}
		b.WriteString("    return o;\n}\n")
		arms = append(arms, fmt.Sprintf("    if (t == %q) return call_%s(a, ns);\n", v.T, v.T))
	}
	fmt.Fprintf(&b, "\nstatic std::string call(const std::string& t, const aj::V& a, long long& ns) {\n%s    throw std::runtime_error(\"no variant for T = \" + t);\n}\n", strings.Join(arms, ""))
	b.WriteString(cppMain)
	if err := codec(j.Dir, "algo_json.hpp", "algo_json.hpp"); err != nil {
		return err
	}
	if err := write(j.Dir, "algo_adapter.cpp", b.String()); err != nil {
		return err
	}
	args := append([]string{"c++", "-std=c++23"}, sanitize...)
	args = append(append(args, "algo_adapter.cpp"), files(j.Impl, false, ".cpp", ".cc")...)
	j.Build = [][]string{append(args, "-o", "algo_adapter")}
	j.Run = []string{"./algo_adapter"}
	return nil
}

// Java.

func javaDec(t contract.Type, e string) (string, string, error) {
	switch t.Name {
	case "int":
		return "long", "AlgoJson.toLong(" + e + ")", nil
	case "index":
		return "int", "AlgoJson.toInt(" + e + ")", nil
	case "bool":
		return "boolean", "AlgoJson.toBool(" + e + ")", nil
	case "string":
		return "String", "AlgoJson.str(" + e + ")", nil
	case "list":
		switch elem(t).Name {
		case "int":
			return "long[]", "AlgoJson.toLongArray(" + e + ")", nil
		case "index":
			return "int[]", "AlgoJson.toIntArray(" + e + ")", nil
		case "bool":
			return "boolean[]", "AlgoJson.toBoolArray(" + e + ")", nil
		case "string":
			return "String[]", "AlgoJson.toStringArray(" + e + ")", nil
		}
	}
	return "", "", unsupported("java", t)
}

const javaMain = `
    public static void main(String[] argv) throws IOException {
        PrintStream out = new PrintStream(new FileOutputStream(FileDescriptor.out), false, StandardCharsets.UTF_8);
        System.setOut(System.err);
        BufferedReader in = new BufferedReader(new InputStreamReader(System.in, StandardCharsets.UTF_8));
        String line;
        while ((line = in.readLine()) != null) {
            if (line.isBlank()) continue;
            @SuppressWarnings("unchecked")
            Map<String, Object> q = (Map<String, Object>) AlgoJson.parse(line);
            Object op = q.get("op");
            if ("bye".equals(op)) break;
            if ("hello".equals(op)) {
                out.print(HELLO + "\n");
                out.flush();
                continue;
            }
            String id = (String) q.get("id");
            String t = q.get("T") == null ? "int" : (String) q.get("T");
            @SuppressWarnings("unchecked")
            Map<String, Object> a = (Map<String, Object>) q.get("args");
            StringBuilder o = new StringBuilder("{\"id\":");
            AlgoJson.quote(o, id);
            try {
                long[] ns = new long[1];
                Object r = call(t, a, ns);
                o.append(",\"ok\":true,\"result\":");
                AlgoJson.put(o, r);
                o.append(",\"ns\":").append(ns[0]).append('}');
            } catch (Throwable e) {
                o.setLength(0);
                o.append("{\"id\":");
                AlgoJson.quote(o, id);
                o.append(",\"ok\":false,\"error\":");
                AlgoJson.quote(o, e.toString());
                o.append('}');
            }
            out.print(o.append('\n'));
            out.flush();
        }
    }
}
`

func javaAdapter(j *adapterJob) error {
	fn := pascal(j.Node.Slug) + "." + camel(j.K.Entry)
	var b strings.Builder
	fmt.Fprintf(&b, "import java.io.*;\nimport java.nio.charset.StandardCharsets;\nimport java.util.Map;\n\npublic final class AlgoAdapter {\n    private static final String HELLO = %q;\n", helloLine("java", j.Node.Slug, fn))
	var arms []string
	for _, v := range j.Variants {
		fmt.Fprintf(&b, "\n    private static Object call_%s(Map<String, Object> a, long[] ns) {\n", v.T)
		var args []string
		for i, t := range v.Params {
			ty, d, err := javaDec(t, fmt.Sprintf("a.get(%q)", v.Names[i]))
			if err != nil {
				return err
			}
			fmt.Fprintf(&b, "        %s a%d = %s;\n", ty, i, d)
			args = append(args, fmt.Sprintf("a%d", i))
		}
		call := fmt.Sprintf("%s(%s)", fn, strings.Join(args, ", "))
		b.WriteString("        long t0 = System.nanoTime();\n")
		if v.Inout >= 0 {
			fmt.Fprintf(&b, "        %s;\n        ns[0] = System.nanoTime() - t0;\n        return a%d;\n    }\n", call, v.Inout)
		} else {
			fmt.Fprintf(&b, "        Object r = %s;\n        ns[0] = System.nanoTime() - t0;\n        return r;\n    }\n", call)
		}
		arms = append(arms, fmt.Sprintf("            case %q -> call_%s(a, ns);\n", v.T, v.T))
	}
	fmt.Fprintf(&b, "\n    private static Object call(String t, Map<String, Object> a, long[] ns) {\n        return switch (t) {\n%s            default -> throw new IllegalArgumentException(\"no variant for T = \" + t);\n        };\n    }\n", strings.Join(arms, ""))
	b.WriteString(javaMain)
	if err := codec(j.Dir, "AlgoJson.java", "AlgoJson.java"); err != nil {
		return err
	}
	if err := write(j.Dir, "AlgoAdapter.java", b.String()); err != nil {
		return err
	}
	args := append([]string{"javac", "-nowarn", "-d", "out", "AlgoJson.java", "AlgoAdapter.java"}, files(j.Impl, false, ".java")...)
	j.Build = [][]string{args}
	j.Run = []string{"java", "-Xss64m", "-cp", "out", "AlgoAdapter"}
	return nil
}

// Kotlin.

func kotlinDec(t contract.Type, e string, depth int) (string, string, error) {
	switch t.Name {
	case "int":
		return "Long", "AlgoJson.toLong(" + e + ")", nil
	case "index":
		return "Int", "AlgoJson.toInt(" + e + ")", nil
	case "bool":
		return "Boolean", "AlgoJson.toBool(" + e + ")", nil
	case "string":
		return "String", "AlgoJson.str(" + e + ")", nil
	case "list":
		x := fmt.Sprintf("x%d", depth)
		et, d, err := kotlinDec(elem(t), x, depth+1)
		if err != nil {
			return "", "", err
		}
		return "MutableList<" + et + ">", "AlgoJson.list(" + e + ").map { " + x + " -> " + d + " }.toMutableList()", nil
	}
	return "", "", unsupported("kotlin", t)
}

const kotlinMain = `
fun main() {
    val out = PrintStream(FileOutputStream(FileDescriptor.out), false, "UTF-8")
    System.setOut(System.err)
    val input = System.` + "`in`" + `.bufferedReader(Charsets.UTF_8)
    while (true) {
        val line = input.readLine() ?: break
        if (line.isBlank()) continue
        @Suppress("UNCHECKED_CAST")
        val q = AlgoJson.parse(line) as Map<String, Any?>
        val op = q["op"]
        if (op == "bye") break
        if (op == "hello") {
            out.print(HELLO + "\n")
            out.flush()
            continue
        }
        val id = q["id"] as String
        val t = (q["T"] as String?) ?: "int"
        @Suppress("UNCHECKED_CAST")
        val a = q["args"] as Map<String, Any?>
        val o = StringBuilder("{\"id\":")
        AlgoJson.quote(o, id)
        try {
            val ns = LongArray(1)
            val r = call(t, a, ns)
            o.append(",\"ok\":true,\"result\":")
            AlgoJson.put(o, r)
            o.append(",\"ns\":").append(ns[0]).append('}')
        } catch (e: Throwable) {
            o.setLength(0)
            o.append("{\"id\":")
            AlgoJson.quote(o, id)
            o.append(",\"ok\":false,\"error\":")
            AlgoJson.quote(o, e.toString())
            o.append('}')
        }
        out.print(o.append('\n'))
        out.flush()
    }
}
`

func kotlinAdapter(j *adapterJob) error {
	fn := camel(j.Node.Slug)
	stdlib, err := j.r.kotlinStdlib()
	if err != nil {
		return err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "import java.io.FileDescriptor\nimport java.io.FileOutputStream\nimport java.io.PrintStream\n\nprivate const val HELLO = %q\n", helloLine("kotlin", j.Node.Slug, fn))
	var arms []string
	for _, v := range j.Variants {
		fmt.Fprintf(&b, "\nprivate fun call_%s(a: Map<String, Any?>, ns: LongArray): Any? {\n", v.T)
		var args []string
		for i, t := range v.Params {
			ty, d, err := kotlinDec(t, fmt.Sprintf("a[%q]", v.Names[i]), 0)
			if err != nil {
				return err
			}
			fmt.Fprintf(&b, "    val a%d: %s = %s\n", i, ty, d)
			args = append(args, fmt.Sprintf("a%d", i))
		}
		call := fmt.Sprintf("%s(%s)", fn, strings.Join(args, ", "))
		b.WriteString("    val t0 = System.nanoTime()\n")
		if v.Inout >= 0 {
			fmt.Fprintf(&b, "    %s\n    ns[0] = System.nanoTime() - t0\n    return a%d\n}\n", call, v.Inout)
		} else {
			fmt.Fprintf(&b, "    val r = %s\n    ns[0] = System.nanoTime() - t0\n    return r\n}\n", call)
		}
		arms = append(arms, fmt.Sprintf("    %q -> call_%s(a, ns)\n", v.T, v.T))
	}
	fmt.Fprintf(&b, "\nprivate fun call(t: String, a: Map<String, Any?>, ns: LongArray): Any? = when (t) {\n%s    else -> throw IllegalArgumentException(\"no variant for T = $t\")\n}\n", strings.Join(arms, ""))
	b.WriteString(kotlinMain)
	if err := codec(j.Dir, "AlgoJson.kt", "AlgoJson.kt"); err != nil {
		return err
	}
	if err := write(j.Dir, "AlgoAdapter.kt", b.String()); err != nil {
		return err
	}
	args := append([]string{"kotlinc", "-nowarn", "-d", "out", "AlgoJson.kt", "AlgoAdapter.kt"}, files(j.Impl, false, ".kt")...)
	j.Build = [][]string{args}
	j.Run = []string{"java", "-Xss64m", "-cp", "out" + string(os.PathListSeparator) + stdlib, "AlgoAdapterKt"}
	return nil
}

// C#.

func csharpDec(t contract.Type, e string, depth int) (string, string, error) {
	switch t.Name {
	case "int":
		return "long", "L(" + e + ")", nil
	case "index":
		return "int", "I(" + e + ")", nil
	case "bool":
		return "bool", "B(" + e + ")", nil
	case "string":
		return "string", "S(" + e + ")", nil
	case "list":
		x := fmt.Sprintf("x%d", depth)
		et, d, err := csharpDec(elem(t), x, depth+1)
		if err != nil {
			return "", "", err
		}
		return et + "[]", "Arr<" + et + ">(" + e + ", " + x + " => " + d + ")", nil
	}
	return "", "", unsupported("csharp", t)
}

const csharpHead = `using System.Diagnostics;
using System.Text;
using System.Text.Json;
using System.Text.Json.Nodes;
using Algorithm;

namespace Algorithm { }

static class AlgoAdapter
{
    static long L(JsonNode? n) => n!.GetValueKind() == JsonValueKind.String ? long.Parse(n.GetValue<string>()) : n.GetValue<long>();
    static int I(JsonNode? n) => checked((int)L(n));
    static bool B(JsonNode? n) => n!.GetValue<bool>();
    static string S(JsonNode? n) => n!.GetValue<string>();
    static T[] Arr<T>(JsonNode? n, Func<JsonNode?, T> f) => n!.AsArray().Select(f).ToArray();
`

const csharpMain = `
    static int Main()
    {
        var output = new StreamWriter(Console.OpenStandardOutput(), new UTF8Encoding(false));
        Console.SetOut(Console.Error);
        var input = new StreamReader(Console.OpenStandardInput(), new UTF8Encoding(false));
        string? line;
        while ((line = input.ReadLine()) != null)
        {
            if (string.IsNullOrWhiteSpace(line)) continue;
            var q = JsonNode.Parse(line)!.AsObject();
            var op = q["op"]?.GetValue<string>();
            if (op == "bye") break;
            if (op == "hello")
            {
                output.Write(Hello + "\n");
                output.Flush();
                continue;
            }
            var id = q["id"]!.GetValue<string>();
            var t = q["T"]?.GetValue<string>() ?? "int";
            string reply;
            try
            {
                var r = Call(t, q["args"]!.AsObject(), out long ns);
                reply = "{\"id\":" + JsonSerializer.Serialize(id) + ",\"ok\":true,\"result\":" + JsonSerializer.Serialize(r) + ",\"ns\":" + ns + "}";
            }
            catch (Exception e)
            {
                reply = "{\"id\":" + JsonSerializer.Serialize(id) + ",\"ok\":false,\"error\":" + JsonSerializer.Serialize(e.GetType().Name + ": " + e.Message) + "}";
            }
            output.Write(reply + "\n");
            output.Flush();
        }
        return 0;
    }
}
`

const csharpProj = `<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <OutputType>Exe</OutputType>
    <TargetFramework>net%s</TargetFramework>
    <Nullable>enable</Nullable>
    <ImplicitUsings>enable</ImplicitUsings>
    <EnableDefaultCompileItems>false</EnableDefaultCompileItems>
    <AssemblyName>AlgoAdapter</AssemblyName>
    <RootNamespace>AlgoAdapter</RootNamespace>
  </PropertyGroup>
  <ItemGroup>
%s  </ItemGroup>
</Project>
`

func csharpAdapter(j *adapterJob) error {
	fn := pascal(j.Node.Slug) + "." + pascal(j.K.Entry)
	major, err := j.r.dotnetMajor()
	if err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString(csharpHead)
	fmt.Fprintf(&b, "    const string Hello = %q;\n", helloLine("csharp", j.Node.Slug, fn))
	targ := map[string]string{"int": "long", "string": "string"}
	var arms []string
	for _, v := range j.Variants {
		fmt.Fprintf(&b, "\n    static object? Call_%s(JsonObject a, out long ns)\n    {\n", v.T)
		var args []string
		for i, t := range v.Params {
			ty, d, err := csharpDec(t, fmt.Sprintf("a[%q]", v.Names[i]), 0)
			if err != nil {
				return err
			}
			fmt.Fprintf(&b, "        %s a%d = %s;\n", ty, i, d)
			args = append(args, fmt.Sprintf("a%d", i))
		}
		name := fn
		// The first try names the type of T, so that an array converts to a span. The second try leaves it to the compiler, for an entry that is not generic.
		if j.First && len(j.K.Generic) > 0 {
			name += "<" + targ[v.T] + ">"
		}
		call := fmt.Sprintf("%s(%s)", name, strings.Join(args, ", "))
		b.WriteString("        long t0 = Stopwatch.GetTimestamp();\n")
		if v.Inout >= 0 {
			fmt.Fprintf(&b, "        %s;\n        ns = (long)Stopwatch.GetElapsedTime(t0).TotalNanoseconds;\n        return a%d;\n    }\n", call, v.Inout)
		} else {
			fmt.Fprintf(&b, "        object? r = %s;\n        ns = (long)Stopwatch.GetElapsedTime(t0).TotalNanoseconds;\n        return r;\n    }\n", call)
		}
		arms = append(arms, fmt.Sprintf("            %q => Call_%s(a, out ns),\n", v.T, v.T))
	}
	fmt.Fprintf(&b, "\n    static object? Call(string t, JsonObject a, out long ns)\n    {\n        ns = 0;\n        return t switch\n        {\n%s            _ => throw new ArgumentException(\"no variant for T = \" + t),\n        };\n    }\n", strings.Join(arms, ""))
	b.WriteString(csharpMain)
	if err := write(j.Dir, "AlgoAdapter.g.cs", b.String()); err != nil {
		return err
	}
	var items strings.Builder
	for _, f := range append(files(j.Impl, false, ".cs"), "AlgoAdapter.g.cs") {
		fmt.Fprintf(&items, "    <Compile Include=%q />\n", f)
	}
	if err := write(j.Dir, "AlgoAdapter.csproj", fmt.Sprintf(csharpProj, major+".0", items.String())); err != nil {
		return err
	}
	j.Build = [][]string{{"dotnet", "build", "AlgoAdapter.csproj", "-o", "out", "--nologo", "-v", "q", "-clp:ErrorsOnly"}}
	j.Run = []string{"dotnet", "out/AlgoAdapter.dll"}
	return nil
}
