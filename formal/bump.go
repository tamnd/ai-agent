// SPDX-License-Identifier: Apache-2.0

package formal

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// MathlibGit is the repository of Mathlib, whose release tags the monthly bump follows (spec 11.11).
const MathlibGit = "https://github.com/leanprover-community/mathlib4"

// Pins are the toolchain of the package, from lean-toolchain, and the Mathlib tag, the rev of the require of
// lakefile.toml. The package pins the two to the same release (11.2).
type Pins struct {
	Toolchain string `json:"toolchain"` // such as leanprover/lean4:v4.35.0
	Mathlib   string `json:"mathlib"`   // such as v4.35.0
}

// Tag is the release of the toolchain, the part after the colon.
func (p Pins) Tag() string { return p.Toolchain[strings.LastIndex(p.Toolchain, ":")+1:] }

var (
	requireRe = regexp.MustCompile(`(?m)^\[\[require\]\]\s*$`)
	nameRe    = regexp.MustCompile(`(?m)^name\s*=\s*"([^"]*)"\s*$`)
	revRe     = regexp.MustCompile(`(?m)^(rev\s*=\s*)"([^"]*)"(.*)$`)
	tagRe     = regexp.MustCompile(`^v(\d+)\.(\d+)\.(\d+)$`)
)

// mathlibRev finds the rev of the require of Mathlib in a lakefile.toml: the start and end of the quoted value.
func mathlibRev(toml string) (start, end int, err error) {
	blocks := requireRe.FindAllStringIndex(toml, -1)
	for i, b := range blocks {
		stop := len(toml)
		if i+1 < len(blocks) {
			stop = blocks[i+1][0]
		}
		// The block ends at the next table.
		if j := strings.Index(toml[b[1]:stop], "\n["); j >= 0 {
			stop = b[1] + j + 1
		}
		body := toml[b[1]:stop]
		if m := nameRe.FindStringSubmatch(body); m == nil || m[1] != "mathlib" {
			continue
		}
		r := revRe.FindStringSubmatchIndex(body)
		if r == nil {
			return 0, 0, fmt.Errorf("lakefile.toml: the require of mathlib has no rev")
		}
		return b[1] + r[4], b[1] + r[5], nil
	}
	return 0, 0, fmt.Errorf("lakefile.toml has no require of mathlib")
}

// ReadPins reads the pins of the package in dir.
func ReadPins(dir string) (Pins, error) {
	tc, err := os.ReadFile(filepath.Join(dir, "lean-toolchain"))
	if err != nil {
		return Pins{}, err
	}
	lf, err := os.ReadFile(filepath.Join(dir, "lakefile.toml"))
	if err != nil {
		return Pins{}, err
	}
	s, e, err := mathlibRev(string(lf))
	if err != nil {
		return Pins{}, err
	}
	return Pins{Toolchain: strings.TrimSpace(string(tc)), Mathlib: string(lf[s:e])}, nil
}

// WritePins sets the toolchain and the Mathlib tag of the package to tag, and keeps the rest of lakefile.toml.
func WritePins(dir, tag string) error {
	if !tagRe.MatchString(tag) {
		return fmt.Errorf("the tag %q is not a release such as v4.35.0", tag)
	}
	p := filepath.Join(dir, "lakefile.toml")
	lf, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	s, e, err := mathlibRev(string(lf))
	if err != nil {
		return err
	}
	out := string(lf[:s]) + tag + string(lf[e:])
	if err := os.WriteFile(p, []byte(out), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "lean-toolchain"), []byte("leanprover/lean4:"+tag+"\n"), 0o644)
}

// LatestTag is the highest release tag, vX.Y.Z with no -rc, among the refs that git ls-remote --tags prints.
func LatestTag(lsRemote string) string {
	best, bv := "", [3]int{}
	sc := bufio.NewScanner(strings.NewReader(lsRemote))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) != 2 {
			continue
		}
		t := strings.TrimSuffix(strings.TrimPrefix(f[1], "refs/tags/"), "^{}")
		m := tagRe.FindStringSubmatch(t)
		if m == nil {
			continue
		}
		var v [3]int
		for i := range v {
			v[i], _ = strconv.Atoi(m[i+1])
		}
		if best == "" || slices.Compare(v[:], bv[:]) > 0 {
			best, bv = t, v
		}
	}
	return best
}

// Modules are the modules of the library AI in the package, sorted.
func Modules(dir string) ([]string, error) {
	var out []string
	root := filepath.Join(dir, "AI")
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(p, ".lean") {
			return nil
		}
		rel, _ := filepath.Rel(dir, p)
		out = append(out, strings.ReplaceAll(strings.TrimSuffix(filepath.ToSlash(rel), ".lean"), "/", "."))
		return nil
	})
	if os.IsNotExist(err) {
		return nil, nil
	}
	sort.Strings(out)
	return out, err
}

// Failure is a module that does not build after the bump, with the first errors of lake.
type Failure struct {
	Module string   `json:"module"`
	Errors []string `json:"errors"`
}

// Update moves the package to the pins that WritePins wrote: lake update mathlib rewrites lake-manifest.json, and
// lake exe cache get fetches the built Mathlib of the release.
func Update(ctx context.Context, o Options) error {
	dir, err := filepath.Abs(o.Dir)
	if err != nil {
		return err
	}
	if err := o.run(ctx, dir, nil, "lake", "update", "mathlib"); err != nil {
		return fmt.Errorf("lake update mathlib: %w", err)
	}
	if err := o.run(ctx, dir, nil, "lake", "exe", "cache", "get"); err != nil {
		return fmt.Errorf("lake exe cache get: %w", err)
	}
	return nil
}

// BuildModules builds each module on its own, so that one broken module does not hide the others, and gives the
// modules that fail.
func BuildModules(ctx context.Context, o Options, mods []string) ([]Failure, error) {
	dir, err := filepath.Abs(o.Dir)
	if err != nil {
		return nil, err
	}
	var out []Failure
	for _, m := range mods {
		var buf strings.Builder
		oo := o
		oo.Log = &buf
		if o.Log != nil {
			oo.Log = io.MultiWriter(&buf, o.Log)
		}
		if err := oo.run(ctx, dir, nil, "lake", "build", m); err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			out = append(out, Failure{Module: m, Errors: lakeErrors(buf.String(), 5)})
		}
	}
	return out, nil
}

// lakeErrors are the first n lines of lake's output that start with error.
func lakeErrors(s string, n int) []string {
	var out []string
	for l := range strings.SplitSeq(s, "\n") {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "error") && len(out) < n {
			out = append(out, l)
		}
	}
	if len(out) == 0 {
		out = []string{"lake build failed with no error line"}
	}
	return out
}

// DeclChange is a declaration of the library that the bump broke or changed.
type DeclChange struct {
	Decl   string `json:"decl"`
	Module string `json:"module"`
	What   string `json:"what"` // lost, sorry, axioms or type
}

// CompareDecls lists the declarations of the library AI that the bump lost, that now depend on sorry or on an axiom
// outside the policy, or whose type changed, which a change of a Mathlib definition can cause and a fidelity review
// should see (11.7).
func CompareDecls(old, cur Decls) []DeclChange {
	var out []DeclChange
	for name, d := range old {
		if !strings.HasPrefix(name, "AI.") {
			continue
		}
		n, ok := cur[name]
		switch {
		case !ok:
			out = append(out, DeclChange{name, d.Module, "lost"})
		case n.Sorry && !d.Sorry:
			out = append(out, DeclChange{name, d.Module, "sorry"})
		case !allowedOnly(n.Axioms) && allowedOnly(d.Axioms):
			out = append(out, DeclChange{name, d.Module, "axioms"})
		case n.TypeHash != d.TypeHash && d.TypeHash != "":
			out = append(out, DeclChange{name, d.Module, "type"})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Decl < out[j].Decl })
	return out
}

func allowedOnly(axioms []string) bool {
	for _, a := range axioms {
		if !slices.Contains(Allowed, a) {
			return false
		}
	}
	return true
}
