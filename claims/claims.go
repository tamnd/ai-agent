// SPDX-License-Identifier: Apache-2.0

// Package claims reads the typed claims of a node and computes their evidence labels (spec 07).
//
// A writer writes the claim records in claims.yaml. The build computes the labels from the evidence and from the results of the checks. Nobody writes a label (rule T05).
package claims

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

// File is the name of the claim file in a node directory.
const File = "claims.yaml"

// The claim types of spec 07.2.
const (
	Definitional      = "definitional"
	Mathematical      = "mathematical"
	Empirical         = "empirical"
	Historical        = "historical"
	Attributive       = "attributive"
	NormativeReported = "normative-reported"
	Forecast          = "forecast"
)

// Types are the claim types, in the order of spec 07.2.
var Types = []string{Definitional, Mathematical, Empirical, Historical, Attributive, NormativeReported, Forecast}

// The evidence labels of spec 07.4.
const (
	Proved       = "proved"
	MeasuredHere = "measured-here"
	Replicated   = "replicated"
	Reported     = "reported"
	Contested    = "contested"
)

// HeadlineOrder is the order in which a label wins the chip (spec 07.4, Q-26).
var HeadlineOrder = []string{Proved, Contested, MeasuredHere, Replicated, Reported}

// Roles are the roles of an evidence entry.
var Roles = []string{"original", "replication", "reanalysis", "review", "meta-analysis", "primary-document", "official-text", "official-guidance", "statement"}

// Stances are the stances of an evidence entry.
var Stances = []string{"for", "against", "mixed"}

// Reuses are the values of reuses for a replication.
var Reuses = []string{"code", "data", "weights", "none"}

// The standings of a source (spec 07.4).
const (
	PeerReviewed       = "peer-reviewed"
	Preprint           = "preprint"
	Official           = "official"
	Developer          = "developer"
	SecondaryReference = "secondary-reference"
	Journalism         = "journalism"
	Community          = "community"
	Retracted          = "retracted"
)

// allowed lists the roles that each type accepts (rule K03). A definitional claim accepts any role, because it only shows its source.
var allowed = map[string][]string{
	Mathematical:      {"original", "review", "primary-document"},
	Empirical:         {"original", "replication", "reanalysis", "review", "meta-analysis"},
	Historical:        {"original", "review", "primary-document", "statement"},
	Attributive:       {"original", "primary-document", "statement", "official-text"},
	NormativeReported: {"official-text", "official-guidance", "original", "statement", "review"},
	Forecast:          {"original", "statement", "primary-document"},
}

// Allows tells if a claim of the type can cite evidence with the role.
func Allows(typ, role string) bool {
	if typ == Definitional {
		return slices.Contains(Roles, role)
	}
	return slices.Contains(allowed[typ], role)
}

// Labelled tells if a claim of the type can have a label about its content. Definitional claims have none. Attributive, normative-reported and forecast claims are labelled only as statements.
func Labelled(typ string) bool { return typ != Definitional }

// Evidence is one source of a claim.
type Evidence struct {
	Work    string `yaml:"work" json:"work"`
	Role    string `yaml:"role" json:"role"`
	Locator string `yaml:"locator,omitempty" json:"locator,omitempty"`
	Stance  string `yaml:"stance" json:"stance"`
	Reuses  string `yaml:"reuses,omitempty" json:"reuses,omitempty"`
}

// Scope says where an empirical claim was measured (spec 07.8).
type Scope struct {
	Models  string   `yaml:"models,omitempty" json:"models,omitempty"`
	Data    []string `yaml:"data,omitempty" json:"data,omitempty"`
	Scale   string   `yaml:"scale,omitempty" json:"scale,omitempty"`
	Setting string   `yaml:"setting,omitempty" json:"setting,omitempty"`
}

// Empty tells if the scope states nothing.
func (s *Scope) Empty() bool {
	return s == nil || (s.Models == "" && len(s.Data) == 0 && s.Scale == "" && s.Setting == "")
}

// Claim is one claim record of claims.yaml.
type Claim struct {
	ID           string     `yaml:"id" json:"id"`
	Type         string     `yaml:"type" json:"type"`
	Text         string     `yaml:"text" json:"text"`
	Scope        *Scope     `yaml:"scope,omitempty" json:"scope,omitempty"`
	Assumes      []string   `yaml:"assumes,omitempty" json:"assumes,omitempty"`
	Evidence     []Evidence `yaml:"evidence,omitempty" json:"evidence,omitempty"`
	Exp          []string   `yaml:"exp,omitempty" json:"exp,omitempty"`
	Derivation   string     `yaml:"derivation,omitempty" json:"derivation,omitempty"`
	Lean         string     `yaml:"lean,omitempty" json:"lean,omitempty"`
	Result       []string   `yaml:"result,omitempty" json:"result,omitempty"`
	AsOf         string     `yaml:"as_of,omitempty" json:"as_of,omitempty"`
	SelfReported bool       `yaml:"self_reported,omitempty" json:"self_reported,omitempty"`
	SupersededBy string     `yaml:"superseded_by,omitempty" json:"superseded_by,omitempty"`
	ReviewAfter  string     `yaml:"review_after,omitempty" json:"review_after,omitempty"`
	// SmallScale tells if the claim has a small-scale form that an experiment can test: yes, no or partial (spec 9.2). SmallScaleNote says why.
	SmallScale     string `yaml:"small_scale,omitempty" json:"small_scale,omitempty"`
	SmallScaleNote string `yaml:"small_scale_note,omitempty" json:"small_scale_note,omitempty"`

	// Extra catches the other keys of the record while it is read. Read rejects an unknown key, and keeps a computed key in Written.
	Extra map[string]any `yaml:",inline" json:"-"`
	// Written holds the computed fields that somebody wrote in the file. Rule T05 rejects them.
	Written []string `yaml:"-" json:"-"`

	// The computed fields.
	Labels         []string `yaml:"-" json:"labels"`
	Headline       string   `yaml:"-" json:"headline,omitempty"`
	IndependentFor int      `yaml:"-" json:"independent_for"`
	Against        int      `yaml:"-" json:"against"`
}

// computed are the keys of build/claims.json that a claim file may not hold.
var computed = []string{"labels", "label", "headline", "independent_for", "against", "cite", "measured_here", "stale", "status"}

// Read reads the claim file of a node directory. It returns nil and no error when the file does not exist.
func Read(dir string) ([]Claim, error) {
	b, err := os.ReadFile(filepath.Join(dir, File))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Claim
	if err := yaml.NewDecoder(bytes.NewReader(b)).Decode(&out); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%s: %w", File, err)
	}
	for i := range out {
		c := &out[i]
		keys := make([]string, 0, len(c.Extra))
		for k := range c.Extra {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		for _, k := range keys {
			if !slices.Contains(computed, k) {
				return nil, fmt.Errorf("%s: the claim %q has the unknown field %q", File, c.ID, k)
			}
			c.Written = append(c.Written, k)
		}
		c.Extra = nil
		c.Labels = []string{}
	}
	return out, nil
}

var idRe = regexp.MustCompile(`^c-[a-z0-9]+(-[a-z0-9]+)*$`)

// ValidID tells if id is c- and a slug.
func ValidID(id string) bool { return idRe.MatchString(id) }

// Env gives the results of the checks that the labels need. The citation check (13.6), the experiments (09) and the Lean build (11) fill it.
type Env interface {
	// Grounded tells if the citation check found the claim in the source of the evidence entry (G2).
	Grounded(c *Claim, e Evidence) bool
	// Standing gives the standing of a work, or "" when the build does not know it.
	Standing(work string) string
	// Independent tells if two works share no author and no institution (spec 07.4).
	Independent(a, b string) bool
	// Proved tells if a Lean proof at F2 or above, an audited full derivation or a reviewed cited proof states the claim.
	Proved(c *Claim) bool
	// Measured tells if an experiment of the node at R2 or above tests the claim at its full scope. failed tells if such an experiment fails its test.
	Measured(c *Claim) (passed, failed bool)
}

// NoChecks is the Env of a build that has none of the checks yet. No source is grounded, so no claim has a label.
type NoChecks struct{}

func (NoChecks) Grounded(*Claim, Evidence) bool { return false }
func (NoChecks) Standing(string) string         { return "" }
func (NoChecks) Independent(a, b string) bool   { return false }
func (NoChecks) Proved(*Claim) bool             { return false }
func (NoChecks) Measured(*Claim) (bool, bool)   { return false, false }

// countsForReplication tells if a source of the standing can be one of the independent sources of replicated (spec 07.4).
func countsForReplication(c *Claim, standing string) bool {
	switch standing {
	case PeerReviewed, Preprint:
		return true
	case Official:
		return c.Type == NormativeReported
	case Developer:
		return !c.SelfReported
	}
	return false
}

// Compute sets the labels, the headline and the counts of c (spec 07.4).
func Compute(c *Claim, env Env) {
	c.Labels, c.Headline, c.IndependentFor, c.Against = []string{}, "", 0, 0
	var support []string
	roles := map[string]bool{}
	for _, e := range c.Evidence {
		st := env.Standing(e.Work)
		if st == Retracted || !env.Grounded(c, e) {
			continue
		}
		if st == Community && c.Type != Attributive {
			continue
		}
		switch e.Stance {
		case "for":
			support = append(support, e.Work)
			roles[e.Role] = true
		case "against":
			if independentOf(e.Work, c, env) {
				c.Against++
			}
		}
	}
	if !Labelled(c.Type) {
		return
	}
	set := map[string]bool{}
	c.IndependentFor = independentCount(c, support, env)
	switch c.Type {
	case Mathematical:
		if env.Proved(c) {
			set[Proved] = true
		}
	case Empirical, Historical:
		if c.Type == Empirical {
			passed, failed := env.Measured(c)
			if passed && !c.SelfReported {
				set[MeasuredHere] = true
			}
			if failed {
				set[Contested] = true
			}
		}
		if !c.SelfReported && (c.IndependentFor >= 2 || (c.Type == Historical && roles["primary-document"] && len(support) >= 2)) {
			set[Replicated] = true
		}
		if c.Against > 0 {
			set[Contested] = true
		}
	case NormativeReported:
		if roles["official-text"] && roles["official-guidance"] {
			set[Replicated] = true
		}
		if c.Against > 0 {
			set[Contested] = true
		}
	}
	if len(support) > 0 && !set[Proved] && !set[MeasuredHere] && !set[Replicated] {
		set[Reported] = true
	}
	for _, l := range HeadlineOrder {
		if set[l] {
			c.Labels = append(c.Labels, l)
		}
	}
	c.Headline = Headline(c.Labels)
}

// independentOf tells if an opposing work is independent of each supporting work of the claim. An opposing source by the same authors is a correction, not a dispute.
func independentOf(work string, c *Claim, env Env) bool {
	for _, e := range c.Evidence {
		if e.Stance == "for" && e.Work != work && !env.Independent(work, e.Work) {
			return false
		}
	}
	return true
}

// independentCount counts the supporting works that are pairwise independent and whose standing counts for replicated. It keeps the first work of each group of related works.
func independentCount(c *Claim, works []string, env Env) int {
	var kept []string
	for _, w := range works {
		if !countsForReplication(c, env.Standing(w)) || slices.Contains(kept, w) {
			continue
		}
		if !slices.ContainsFunc(kept, func(k string) bool { return !env.Independent(k, w) }) {
			kept = append(kept, w)
		}
	}
	return len(kept)
}

// Headline gives the label of the chip: the first label of HeadlineOrder that the set has.
func Headline(labels []string) string {
	for _, l := range HeadlineOrder {
		if slices.Contains(labels, l) {
			return l
		}
	}
	return ""
}

// Anchor is a claim anchor {#c-...} in the prose of a node.
type Anchor struct {
	ID   string
	Line int
}

// Ref is a reference ::claim{ref=<slug>#c-<id>} to the claim of another node.
type Ref struct {
	Slug string
	ID   string
	Line int
}

var (
	anchorRe = regexp.MustCompile(`\{#(c-[^}\s]*)\}`)
	refRe    = regexp.MustCompile(`::claim\{ref=([a-z0-9-]+)#(c-[^}\s]*)\}`)
	runRe    = regexp.MustCompile(`(?:\{#c-[^}\s]*\}\s*){3,}`)
)

// Scan finds the anchors, the references and the runs of more than two anchors in the prose, outside of code.
func Scan(body string) (anchors []Anchor, refs []Ref, runs []int) {
	fence := ""
	for i, l := range strings.Split(body, "\n") {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
			if fence == "" {
				fence = t[:3]
			} else if strings.HasPrefix(t, fence) {
				fence = ""
			}
			continue
		}
		if fence != "" {
			continue
		}
		l = stripCode(l)
		for _, m := range refRe.FindAllStringSubmatch(l, -1) {
			refs = append(refs, Ref{Slug: m[1], ID: m[2], Line: i + 1})
		}
		for _, m := range anchorRe.FindAllStringSubmatch(l, -1) {
			anchors = append(anchors, Anchor{ID: m[1], Line: i + 1})
		}
		if runRe.MatchString(l) {
			runs = append(runs, i+1)
		}
	}
	return anchors, refs, runs
}

var codeSpan = regexp.MustCompile("`[^`]*`")

func stripCode(l string) string { return codeSpan.ReplaceAllString(l, "") }

// AnchorRe matches a claim anchor with the space before it, for the renderer.
var AnchorRe = regexp.MustCompile(`\s*\{#(c-[a-z0-9-]+)\}`)

// RefRe matches a paragraph that holds only a claim reference, in rendered HTML.
var RefRe = regexp.MustCompile(`<p>::claim\{ref=([a-z0-9-]+)#(c-[a-z0-9-]+)\}</p>`)
