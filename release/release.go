// SPDX-License-Identifier: Apache-2.0

// Package release cuts the monthly release of spec 20.6: it freezes manifests/releases/<version>.yaml with the pins
// of the release, writes the release notes, and tags the content commit.
package release

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/tamnd/ai-agent/coverage"
	"github.com/tamnd/ai-agent/lang"
	"github.com/tamnd/ai-agent/node"
)

// Dir is the directory of the release manifests, relative to the content root.
const Dir = "manifests/releases"

var versionRE = regexp.MustCompile(`^[0-9]{4}\.(0[1-9]|1[0-2])(\.[1-9][0-9]*)?$`)

// Valid tells if v is a release version: 2027.03, or 2027.03.1 for a patch release.
func Valid(v string) bool { return versionRE.MatchString(v) }

// Manifest is the frozen record of one release.
type Manifest struct {
	Version string `yaml:"version"`
	Date    string `yaml:"date"`
	// Commit is the content commit that the release tags.
	Commit string `yaml:"commit"`
	Aik    string `yaml:"aik"`
	// Images are the framework and language images of manifests/images.yaml, pinned by digest.
	Images map[string]string `yaml:"images"`
	Lean   *Lean             `yaml:"lean"`
	Data   []Data            `yaml:"data"`
	// Models is the version of registers/models.yaml and Canon the version of registers/canon.yaml.
	Models string `yaml:"models"`
	Canon  string `yaml:"canon,omitempty"`
	// Nodes counts the nodes by top area and Cards the cards by kind.
	Nodes map[string]int `yaml:"nodes"`
	Cards map[string]int `yaml:"cards"`
	// Coverage counts the items at C0 to C6 (18.2).
	Coverage coverage.Counts `yaml:"coverage,flow"`
	// Tags maps each permanent tag of a node to its slug, so the next release can name the new nodes and the retired
	// tags.
	Tags map[string]string `yaml:"tags"`
}

// Lean is the pin of the Lean package: the toolchain and the Mathlib revision.
type Lean struct {
	Toolchain string `yaml:"toolchain"`
	Mathlib   string `yaml:"mathlib,omitempty"`
}

// Data is the pin of one dataset of registers/data-pins.yaml.
type Data struct {
	ID      string            `yaml:"id"`
	Dataset string            `yaml:"dataset"`
	Version string            `yaml:"version"`
	Files   map[string]string `yaml:"files,omitempty"` // name to sha256
}

// Options are the inputs of Cut.
type Options struct {
	Root     string
	Version  string
	Corpus   *node.Corpus
	Coverage *coverage.Report
	Aik      string
	Now      time.Time
	// Dirty allows a work tree with changes that are not committed. The manifest then names a commit that is not
	// what was built, so only tests and dry runs use it.
	Dirty bool
	// Tag makes the annotated git tag of the version on the content commit.
	Tag bool
	// Out is the directory of the manifest and the notes. It defaults to Root/manifests/releases.
	Out string
}

// Result is what Cut wrote.
type Result struct {
	Manifest *Manifest
	Previous *Manifest
	Files    []string
	Tagged   bool
	// Missing names the pins and the parts of the notes that the content tree does not have yet.
	Missing []string
}

// Cut freezes the release.
func Cut(o Options) (*Result, error) {
	if !Valid(o.Version) {
		return nil, fmt.Errorf("%q is not a release version: give YYYY.MM, or YYYY.MM.N for a patch release", o.Version)
	}
	out := o.Out
	if out == "" {
		out = filepath.Join(o.Root, filepath.FromSlash(Dir))
	}
	path := filepath.Join(out, o.Version+".yaml")
	if _, err := os.Stat(path); err == nil {
		return nil, fmt.Errorf("%s exists: a release is frozen once it is cut", path)
	}
	commit, err := git(o.Root, "rev-parse", "HEAD")
	if err != nil {
		return nil, err
	}
	if !o.Dirty {
		st, err := git(o.Root, "status", "--porcelain")
		if err != nil {
			return nil, err
		}
		if st != "" {
			return nil, errors.New("the work tree has changes that are not committed; a release names a commit")
		}
	}
	if o.Tag {
		if _, err := git(o.Root, "rev-parse", "-q", "--verify", "refs/tags/"+o.Version); err == nil {
			return nil, fmt.Errorf("the tag %s exists", o.Version)
		}
	}
	r := &Result{}
	m := &Manifest{Version: o.Version, Date: o.Now.UTC().Format("2006-01-02"), Commit: commit, Aik: o.Aik,
		Images: map[string]string{}, Data: []Data{}, Nodes: map[string]int{}, Cards: map[string]int{}, Tags: map[string]string{}}

	if p := filepath.Join(o.Root, "manifests", "images.yaml"); exists(p) {
		if m.Images, err = lang.LoadImages(p); err != nil {
			return nil, err
		}
	} else {
		r.Missing = append(r.Missing, "framework images: manifests/images.yaml does not exist")
	}
	if m.Lean, err = leanPin(filepath.Join(o.Root, "lean")); err != nil {
		return nil, err
	}
	if m.Lean == nil {
		r.Missing = append(r.Missing, "Lean pin: the content tree has no lean/lean-toolchain")
	}
	var ids []string
	for id := range o.Corpus.Pins {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		p := o.Corpus.Pins[id]
		d := Data{ID: p.ID, Dataset: p.Dataset, Version: p.Version}
		for _, f := range p.Files {
			if d.Files == nil {
				d.Files = map[string]string{}
			}
			d.Files[f.Name] = f.SHA256
		}
		m.Data = append(m.Data, d)
	}
	if m.Models, err = registerVersion(o.Root, "models.yaml"); err != nil {
		return nil, err
	}
	if m.Models == "" {
		r.Missing = append(r.Missing, "models version: registers/models.yaml has no version")
	}
	if m.Canon, err = registerVersion(o.Root, "canon.yaml"); err != nil {
		return nil, err
	}
	for _, n := range o.Corpus.Nodes {
		m.Nodes[strings.Split(n.Area, "/")[0]]++
		if n.Tag != "" && !strings.HasPrefix(n.Tag, "NEW-") {
			m.Tags[n.Tag] = n.Slug
		}
	}
	for _, c := range o.Corpus.Cards {
		m.Cards[c.Kind]++
	}
	if o.Coverage != nil {
		m.Coverage = o.Coverage.Levels.Total
	}
	r.Manifest = m

	if r.Previous, err = Latest(out, o.Version); err != nil {
		return nil, err
	}
	b, err := yaml.Marshal(m)
	if err != nil {
		return nil, err
	}
	head := fmt.Sprintf("# The release %s of spec 20.6, written by aik release cut. It is frozen: a fix is a patch release.\n", o.Version)
	if err := os.MkdirAll(out, 0o755); err != nil {
		return nil, err
	}
	notes := filepath.Join(out, o.Version+".md")
	files := map[string][]byte{path: append([]byte(head), b...), notes: Notes(m, r.Previous, r.Missing)}
	if o.Coverage != nil {
		cv := *o.Coverage
		cv.Items = nil // the levels of each item are in the build; the notes keep the counts and the gaps
		cb, err := json.MarshalIndent(cv, "", "  ")
		if err != nil {
			return nil, err
		}
		files[filepath.Join(out, o.Version+".coverage.json")] = append(cb, '\n')
	}
	for _, p := range sortedKeys(files) {
		if err := os.WriteFile(p, files[p], 0o644); err != nil {
			return nil, err
		}
		r.Files = append(r.Files, p)
	}
	if o.Tag {
		if _, err := git(o.Root, "tag", "-a", o.Version, "-m", "release "+o.Version, commit); err != nil {
			return nil, err
		}
		r.Tagged = true
	}
	return r, nil
}

// Notes writes the release notes: the new nodes and cards by area, the coverage changes and the retired tags.
func Notes(m, prev *Manifest, missing []string) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "# Release %s\n\nCut on %s from the commit %s with aik %s.\n", m.Version, m.Date, m.Commit, m.Aik)
	if prev == nil {
		prev = &Manifest{}
		fmt.Fprintf(&b, "This is the first release.\n")
	} else {
		fmt.Fprintf(&b, "The changes are since %s.\n", prev.Version)
	}
	table := func(title, col string, now, before map[string]int) {
		fmt.Fprintf(&b, "\n## %s\n\n| %s | Count | Change |\n|---|---|---|\n", title, col)
		keys := map[string]bool{}
		for k := range now {
			keys[k] = true
		}
		for k := range before {
			keys[k] = true
		}
		total, dt := 0, 0
		for _, k := range sortedKeys(keys) {
			fmt.Fprintf(&b, "| %s | %d | %+d |\n", k, now[k], now[k]-before[k])
			total, dt = total+now[k], dt+now[k]-before[k]
		}
		fmt.Fprintf(&b, "| all | %d | %+d |\n", total, dt)
	}
	table("Nodes by area", "Area", m.Nodes, prev.Nodes)
	var added []string
	for tag, slug := range m.Tags {
		if _, ok := prev.Tags[tag]; !ok {
			added = append(added, fmt.Sprintf("%s (%s)", slug, tag))
		}
	}
	if len(added) > 0 {
		sort.Strings(added)
		fmt.Fprintf(&b, "\nNew nodes: %s.\n", strings.Join(added, ", "))
	}
	table("Cards by kind", "Kind", m.Cards, prev.Cards)
	fmt.Fprintf(&b, "\n## Coverage\n\n| Level | Items | Change |\n|---|---|---|\n")
	for i, n := range m.Coverage {
		fmt.Fprintf(&b, "| C%d %s | %d | %+d |\n", i, coverage.Names[i], n, n-prev.Coverage[i])
	}
	var retired []string
	for tag, slug := range prev.Tags {
		if _, ok := m.Tags[tag]; !ok {
			retired = append(retired, fmt.Sprintf("%s (%s)", tag, slug))
		}
	}
	fmt.Fprintf(&b, "\n## Retired tags\n\n")
	if len(retired) == 0 {
		fmt.Fprintf(&b, "None.\n")
	} else {
		sort.Strings(retired)
		for _, t := range retired {
			fmt.Fprintf(&b, "- %s\n", t)
		}
	}
	fmt.Fprintf(&b, "\n## Pins\n\n")
	if m.Lean != nil {
		fmt.Fprintf(&b, "- Lean: %s, Mathlib %s\n", m.Lean.Toolchain, orNone(m.Lean.Mathlib))
	}
	fmt.Fprintf(&b, "- images: %d, pinned by digest\n- data pins: %d\n- models: %s\n", len(m.Images), len(m.Data), orNone(m.Models))
	missing = append(missing, "new evidence labels and contested items that changed: not in the notes yet (the claim labels are not kept per release)")
	fmt.Fprintf(&b, "\n## Not in this release\n\n")
	for _, s := range missing {
		fmt.Fprintf(&b, "- %s\n", s)
	}
	return b.Bytes()
}

// Latest reads the manifest of the newest release in dir before the version not, or nil when there is none.
func Latest(dir, not string) (*Manifest, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return nil, err
	}
	var vs []string
	for _, p := range files {
		v := strings.TrimSuffix(filepath.Base(p), ".yaml")
		if Valid(v) && (not == "" || Less(v, not)) {
			vs = append(vs, v)
		}
	}
	if len(vs) == 0 {
		return nil, nil
	}
	sort.Slice(vs, func(i, j int) bool { return Less(vs[i], vs[j]) })
	p := filepath.Join(dir, vs[len(vs)-1]+".yaml")
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := yaml.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("%s: %w", p, err)
	}
	return &m, nil
}

// Less orders release versions: 2027.03 < 2027.03.1 < 2027.04.
func Less(a, b string) bool {
	pa, pb := parts(a), parts(b)
	for i := range 3 {
		if pa[i] != pb[i] {
			return pa[i] < pb[i]
		}
	}
	return false
}

func parts(v string) [3]int {
	var p [3]int
	for i, s := range strings.SplitN(v, ".", 3) {
		p[i], _ = strconv.Atoi(s)
	}
	return p
}

// leanPin reads lean/lean-toolchain and the Mathlib revision of lean/lake-manifest.json. It gives nil when the
// content tree has no Lean package.
func leanPin(dir string) (*Lean, error) {
	b, err := os.ReadFile(filepath.Join(dir, "lean-toolchain"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	l := &Lean{Toolchain: strings.TrimSpace(string(b))}
	mb, err := os.ReadFile(filepath.Join(dir, "lake-manifest.json"))
	if errors.Is(err, os.ErrNotExist) {
		return l, nil
	}
	if err != nil {
		return nil, err
	}
	var lm struct {
		Packages []struct {
			Name string `json:"name"`
			Rev  string `json:"rev"`
		} `json:"packages"`
	}
	if err := json.Unmarshal(mb, &lm); err != nil {
		return nil, fmt.Errorf("lean/lake-manifest.json: %w", err)
	}
	for _, p := range lm.Packages {
		if strings.EqualFold(p.Name, "mathlib") {
			l.Mathlib = p.Rev
		}
	}
	return l, nil
}

// registerVersion reads the version field of a register. It gives "" when the register does not exist.
func registerVersion(root, name string) (string, error) {
	b, err := os.ReadFile(filepath.Join(root, "registers", name))
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	var v struct {
		Version string `yaml:"version"`
	}
	if err := yaml.Unmarshal(b, &v); err != nil {
		return "", fmt.Errorf("registers/%s: %w", name, err)
	}
	return v.Version, nil
}

func git(root string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(string(out)), nil
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func orNone(s string) string {
	if s == "" {
		return "none"
	}
	return s
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
