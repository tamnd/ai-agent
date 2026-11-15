// SPDX-License-Identifier: Apache-2.0

// Package agent reads what the agents of spec 14 run on: the register of models (registers/models.yaml, 14.4) with the
// roles of 14.2, the versioned prompts of work/prompts (14.4) and the run records of prov/runs (14.7). The agents
// themselves are the scripts of work/; this package is the part of them that aik checks and reports.
package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// ModelsFile is the register of models in the content repository.
const ModelsFile = "registers/models.yaml"

// Backends are the backends that the scripts of work/ can call.
var Backends = []string{"claude", "codex"}

// Role is a role of 14.2 with the key of models.yaml that pins its model, if any.
type Role struct {
	Name string
	Key  string // empty for the roles whose model is the writer's or that run no model
	Note string
}

// Roles are the roles of 14.2 in its order.
var Roles = []Role{
	{"cataloguer", "cataloguer", "two models of two families (14.3)"},
	{"curator", "curator", ""},
	{"harvester", "harvester_extract", "most sources need no model"},
	{"writer", "writer_ladder", "the ladder of 14.6; writer_ladder_hard for the hard kinds"},
	{"coder", "coder", ""},
	{"experimenter", "experimenter", ""},
	{"verifier", "verifier", "never of the family of a writer of the node (15.4)"},
	{"auditor", "auditor_for", "never of the family of a writer of the node (15.4)"},
	{"fixer", "", "the writer's current rung"},
	{"reviewer-assistant", "reviewer_assistant", "never a verdict"},
}

// Models is the content of registers/models.yaml.
type Models struct {
	Version string
	Path    string
	raw     map[string]any
	rules   [][2]string // the patterns of auditor_for and their auditors, in the order of the file
}

// LoadModels reads the register of models.
func LoadModels(file string) (*Models, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var raw map[string]any
	if err := yaml.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	if _, ok := raw["classes"].(map[string]any); !ok {
		return nil, fmt.Errorf("%s has no classes block (14.4)", file)
	}
	m := &Models{Path: file, raw: raw}
	var ordered struct {
		AuditorFor yaml.Node `yaml:"auditor_for"`
	}
	if err := yaml.Unmarshal(b, &ordered); err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	if n := ordered.AuditorFor; n.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(n.Content); i += 2 {
			m.rules = append(m.rules, [2]string{n.Content[i].Value, n.Content[i+1].Value})
		}
	}
	if v, ok := raw["version"]; ok && v != nil {
		m.Version = fmt.Sprint(v)
	}
	return m, nil
}

// Ref is a model as backend:model.
type Ref struct{ Backend, Model string }

func (r Ref) String() string { return r.Backend + ":" + r.Model }

// registerID is the form of models.yaml: a bare id for Claude, backend:model for the other family.
func (r Ref) registerID() string {
	if r.Backend == "claude" {
		return r.Model
	}
	return r.String()
}

// Resolve turns a reference of the register into a model: a model id (claude-sonnet-5-5), a backend and a model
// (codex:gpt-6-sol), other.<role>, or a class name, which gives the first model of the class. It follows the rules of
// Models.resolve in work/lib.py.
func (m *Models) Resolve(ref string) (Ref, error) {
	return m.resolve(strings.TrimSpace(ref), 0)
}

func (m *Models) resolve(ref string, depth int) (Ref, error) {
	if depth > 8 {
		return Ref{}, fmt.Errorf("the reference %q loops", ref)
	}
	classes := m.raw["classes"].(map[string]any)
	if role, ok := strings.CutPrefix(ref, "other."); ok {
		other, _ := classes["other"].(map[string]any)
		v, ok := other[role]
		if !ok {
			return Ref{}, fmt.Errorf("classes.other has no %q", role)
		}
		return m.resolve(fmt.Sprint(v), depth+1)
	}
	if list, ok := classes[ref].([]any); ok && len(list) > 0 {
		return m.resolve(fmt.Sprint(list[0]), depth+1)
	}
	if b, model, ok := strings.Cut(ref, ":"); ok {
		if !contains(Backends, b) {
			return Ref{}, fmt.Errorf("unknown backend %q in %q", b, ref)
		}
		return Ref{b, model}, nil
	}
	if strings.HasPrefix(ref, "claude-") {
		return Ref{"claude", ref}, nil
	}
	return Ref{}, fmt.Errorf("cannot resolve the model reference %q", ref)
}

// Role gives the models of a key of the register, as a list.
func (m *Models) Role(key string) ([]Ref, error) {
	v, ok := m.raw[key]
	if !ok || v == nil {
		return nil, fmt.Errorf("no %s", key)
	}
	var refs []string
	switch x := v.(type) {
	case []any:
		for _, e := range x {
			refs = append(refs, fmt.Sprint(e))
		}
	default:
		refs = []string{fmt.Sprint(x)}
	}
	var out []Ref
	for _, r := range refs {
		ref, err := m.Resolve(r)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", key, err)
		}
		out = append(out, ref)
	}
	return out, nil
}

// Ladder gives the writer ladder; hard gives writer_ladder_hard.
func (m *Models) Ladder(hard bool) ([]Ref, error) {
	if hard {
		return m.Role("writer_ladder_hard")
	}
	return m.Role("writer_ladder")
}

// AuditorFor gives the auditor of a node that the writers wrote, and the fallback when the auditor is not available,
// by the rules of auditor_for (14.4, 15.4); fallback is nil when the item would wait. It does not ask whether a
// backend is available: the scripts do that at the time of the call.
func (m *Models) AuditorFor(writers []Ref) (auditor Ref, fallback *Ref, err error) {
	if len(m.rules) == 0 {
		return Ref{}, nil, errors.New("no auditor_for")
	}
	if len(writers) == 0 {
		return Ref{}, nil, errors.New("no writer")
	}
	fams := map[string]bool{}
	for _, w := range writers {
		fams[w.Backend] = true
	}
	// The first rule that matches, in the order of the file, as fnmatch in work/lib.py.
	fb := ""
	var allowed []Ref
	for _, w := range writers {
		hit := ""
		for _, r := range m.rules {
			if r[0] == "fallback_if_other_unavailable" {
				fb = r[1]
				continue
			}
			if ok, _ := path.Match(r[0], w.registerID()); ok && hit == "" {
				hit = r[1]
			}
		}
		if hit == "" {
			return Ref{}, nil, fmt.Errorf("auditor_for has no rule for the writer %s", w.registerID())
		}
		a, err := m.Resolve(hit)
		if err != nil {
			return Ref{}, nil, fmt.Errorf("auditor_for: %w", err)
		}
		if !fams[a.Backend] && !containsRef(allowed, a) {
			allowed = append(allowed, a)
		}
	}
	if len(allowed) == 0 {
		return Ref{}, nil, fmt.Errorf("the writers are all of the families of the auditors that auditor_for gives them, so the item waits (15.4, Q-08)")
	}
	if fb != "" {
		f, err := m.Resolve(fb)
		if err != nil {
			return Ref{}, nil, fmt.Errorf("fallback_if_other_unavailable: %w", err)
		}
		if !fams[f.Backend] {
			fallback = &f
		}
	}
	return allowed[0], fallback, nil
}

// Check gives the problems of the register by the rules of 14.2, 14.3 and 14.4: a version, every role pinned, every
// reference resolved, an auditor of another family for each rung of both ladders, a verifier that can be of another
// family than some rung, and a cataloguer of two models of two families.
func (m *Models) Check() []string {
	var out []string
	if m.Version == "" {
		out = append(out, "no version; a change of models.yaml is a PR with a new version (14.4)")
	}
	for _, r := range Roles {
		if r.Key == "" || r.Key == "auditor_for" {
			continue
		}
		if _, err := m.Role(r.Key); err != nil {
			out = append(out, fmt.Sprintf("the role %s: %v", r.Name, err))
		}
	}
	if _, err := m.Role("writer_ladder_hard"); err != nil {
		out = append(out, fmt.Sprintf("the hard kinds: %v", err))
	}
	for _, hard := range []bool{false, true} {
		ladder, err := m.Ladder(hard)
		if err != nil {
			continue
		}
		for _, w := range ladder {
			a, _, err := m.AuditorFor([]Ref{w})
			if err != nil {
				out = append(out, fmt.Sprintf("the rung %s: %v", w, err))
				continue
			}
			if a.Backend == w.Backend {
				out = append(out, fmt.Sprintf("the rung %s is audited by %s, of its own family (15.4)", w, a))
			}
		}
	}
	if cat, err := m.Role("cataloguer"); err == nil {
		fams := map[string]bool{}
		for _, r := range cat {
			fams[r.Backend] = true
		}
		if len(cat) < 2 || len(fams) < 2 {
			out = append(out, "the cataloguer needs two models of two families (14.3)")
		}
	}
	return out
}

// Prompt is one versioned prompt file, work/prompts/<role>/v<n>.md.
type Prompt struct {
	Role    string `json:"role"`
	Version string `json:"version"`
	File    string `json:"file"`
	SHA256  string `json:"sha256"`
	Bytes   int    `json:"bytes"`
}

// LoadPrompts lists the prompts under dir, by role and version.
func LoadPrompts(dir string) ([]Prompt, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*", "v*.md"))
	if err != nil {
		return nil, err
	}
	var out []Prompt
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		h := sha256.Sum256(b)
		out = append(out, Prompt{
			Role:    filepath.Base(filepath.Dir(f)),
			Version: strings.TrimSuffix(filepath.Base(f), ".md"),
			File:    filepath.ToSlash(f),
			SHA256:  hex.EncodeToString(h[:]),
			Bytes:   len(b),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Role != out[j].Role {
			return out[i].Role < out[j].Role
		}
		return versionLess(out[i].Version, out[j].Version)
	})
	return out, nil
}

func versionLess(a, b string) bool {
	var x, y int
	_, ea := fmt.Sscanf(a, "v%d", &x)
	_, eb := fmt.Sscanf(b, "v%d", &y)
	if ea == nil && eb == nil && x != y {
		return x < y
	}
	return a < b
}

// Run is the summary of one run record, prov/runs/<run-id>.yaml (14.7).
type Run struct {
	Run           string         `json:"run"`
	Date          string         `json:"date"`
	Role          string         `json:"role"`
	Prompt        string         `json:"prompt"`
	ModelsVersion string         `json:"models_version"`
	Harness       string         `json:"harness"`
	Items         int            `json:"items"`
	Slugs         int            `json:"slugs"`
	Status        map[string]int `json:"status,omitempty"`  // the writer's items by final status
	Verdict       map[string]int `json:"verdict,omitempty"` // the verifier's claims by verdict
	Spent         float64        `json:"spent,omitempty"`
	Budget        float64        `json:"budget,omitempty"`
	Unpriced      int            `json:"unpriced_calls,omitempty"`
	Models        map[string]any `json:"models,omitempty"`
	Rates         any            `json:"rates,omitempty"`
	File          string         `json:"file"`
}

type runFile struct {
	Run           string           `yaml:"run"`
	Date          string           `yaml:"date"`
	Role          string           `yaml:"role"`
	Prompt        string           `yaml:"prompt"`
	ModelsVersion string           `yaml:"models_version"`
	Harness       string           `yaml:"harness"`
	Items         []map[string]any `yaml:"items"`
	Spent         float64          `yaml:"spent"`
	Budget        float64          `yaml:"budget"`
	Unpriced      int              `yaml:"unpriced_calls"`
	Models        map[string]any   `yaml:"models"`
	Rates         any              `yaml:"rates"`
}

// RunsDir is the directory of the run records in the content repository.
const RunsDir = "prov/runs"

// LoadRuns reads the run records of the content repository, newest first. A missing directory gives none.
func LoadRuns(root string) ([]Run, error) {
	files, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(RunsDir), "*.yaml"))
	if err != nil {
		return nil, err
	}
	var out []Run
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		var rf runFile
		if err := yaml.Unmarshal(b, &rf); err != nil {
			return nil, fmt.Errorf("%s: %w", f, err)
		}
		r := Run{Run: rf.Run, Date: rf.Date, Role: rf.Role, Prompt: rf.Prompt, ModelsVersion: rf.ModelsVersion, Harness: rf.Harness,
			Items: len(rf.Items), Spent: rf.Spent, Budget: rf.Budget, Unpriced: rf.Unpriced, Models: rf.Models, Rates: rf.Rates,
			File: filepath.ToSlash(filepath.Join(RunsDir, filepath.Base(f)))}
		if r.Run == "" {
			r.Run = strings.TrimSuffix(filepath.Base(f), ".yaml")
		}
		slugs := map[string]bool{}
		for _, it := range rf.Items {
			slugs[fmt.Sprint(it["slug"])] = true
			if s, ok := it["status"]; ok {
				if r.Status == nil {
					r.Status = map[string]int{}
				}
				r.Status[fmt.Sprint(s)]++
			}
			if v, ok := it["verdict"]; ok {
				if r.Verdict == nil {
					r.Verdict = map[string]int{}
				}
				r.Verdict[fmt.Sprint(v)]++
			}
		}
		r.Slugs = len(slugs)
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Date != out[j].Date {
			return out[i].Date > out[j].Date
		}
		return out[i].Run > out[j].Run
	})
	return out, nil
}

func contains(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}

func containsRef(xs []Ref, x Ref) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}
