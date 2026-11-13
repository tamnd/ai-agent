// SPDX-License-Identifier: Apache-2.0

// Package exp reads the experiments of a node, runs them and decides their claims with the test of expect.yaml (spec 09).
//
// An experiment is the directory exp/<slug>/ of a node. It holds experiment.yaml (the design), expect.yaml (the test, fixed before the first full run), run.py (the script), results.json (written only by aik exp run -update) and figures/.
package exp

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// The files of an experiment directory.
const (
	Dir         = "exp"
	DesignFile  = "experiment.yaml"
	ExpectFile  = "expect.yaml"
	ResultsFile = "results.json"
	Script      = "run.py"
	Figures     = "figures"
)

// Budget is one budget class of spec 9.4.1.
type Budget struct {
	Name    string
	CPUs    string
	Memory  string
	Seconds int // the cap of one run of the experiment: all arms and all seeds
	Seeds   int // the default number of seeds
}

// Budgets are the budget classes. aik exp run runs only cpu-small in M0. The GPU classes run in the GPU pool, which comes later.
var Budgets = map[string]Budget{
	"cpu-small":  {Name: "cpu-small", CPUs: "2", Memory: "4g", Seconds: 600, Seeds: 10},
	"gpu-small":  {Name: "gpu-small", CPUs: "8", Memory: "32g", Seconds: 2 * 3600, Seeds: 5},
	"gpu-medium": {Name: "gpu-medium", CPUs: "16", Memory: "128g", Seconds: 8 * 3600, Seeds: 3},
}

// Arm is one condition of the design.
type Arm struct {
	ID     string         `yaml:"id"`
	Config map[string]any `yaml:"config"`
	// Seeds and Tuning are here only so that rule R08 can report them. Every arm runs with the seeds of the design and the tuning budget of the design.
	Seeds  *Seeds         `yaml:"seeds,omitempty"`
	Tuning map[string]any `yaml:"tuning,omitempty"`
}

// ScaleNote states the scale of the experiment and the scale of the claim of the source (rule R03).
type ScaleNote struct {
	Here   string `yaml:"here"`
	Source string `yaml:"source"`
}

// Replicates names the table of a work whose own small setting the experiment reruns.
type Replicates struct {
	Work    string `yaml:"work"`
	Locator string `yaml:"locator"`
}

// Data is one input of the experiment: a pinned dataset, or a generator file in the experiment directory.
type Data struct {
	Dataset   string `yaml:"dataset,omitempty"`
	Version   string `yaml:"version,omitempty"`
	Subset    string `yaml:"subset,omitempty"`
	Pin       string `yaml:"pin,omitempty"`
	Hash      string `yaml:"hash,omitempty"`
	Generator string `yaml:"generator,omitempty"`
}

// Seeds gives the seeds of every arm: base, base+1, and so on.
type Seeds struct {
	N    int   `yaml:"n"`
	Base int64 `yaml:"base"`
}

// List gives the seeds.
func (s Seeds) List() []int64 {
	out := make([]int64, 0, max(s.N, 0))
	for i := range max(s.N, 0) {
		out = append(out, s.Base+int64(i))
	}
	return out
}

// Design is the file experiment.yaml.
type Design struct {
	ID                  string         `yaml:"id"`
	Claims              []string       `yaml:"claims"`
	Question            string         `yaml:"question"`
	ScaleNote           ScaleNote      `yaml:"scale_note"`
	Replicates          *Replicates    `yaml:"replicates"`
	Arms                []Arm          `yaml:"arms"`
	BaseConfig          map[string]any `yaml:"base_config"`
	Tuning              map[string]any `yaml:"tuning,omitempty"`
	Data                []Data         `yaml:"data"`
	Seeds               Seeds          `yaml:"seeds"`
	Budget              string         `yaml:"budget"`
	Image               string         `yaml:"image,omitempty"`
	Framework           string         `yaml:"framework"`
	UsesImpl            string         `yaml:"uses_impl,omitempty"`
	Metrics             []string       `yaml:"metrics"`
	Deterministic       *bool          `yaml:"deterministic,omitempty"`
	DeterministicReason string         `yaml:"deterministic_reason,omitempty"`
	KeepCheckpoints     bool           `yaml:"keep_checkpoints,omitempty"`
}

// Arm gives the arm with the id, or nil.
func (d *Design) Arm(id string) *Arm {
	for i := range d.Arms {
		if d.Arms[i].ID == id {
			return &d.Arms[i]
		}
	}
	return nil
}

// Config gives the configuration of an arm: the base configuration with the configuration of the arm on top.
func (d *Design) Config(a *Arm) map[string]any { return Merge(d.BaseConfig, a.Config) }

// Merge gives a deep copy of base with over on top. A map in over merges into the map in base. Any other value replaces it.
func Merge(base, over map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range base {
		out[k] = clone(v)
	}
	for k, v := range over {
		if bm, ok := out[k].(map[string]any); ok {
			if om, ok := v.(map[string]any); ok {
				out[k] = Merge(bm, om)
				continue
			}
		}
		out[k] = clone(v)
	}
	return out
}

func clone(v any) any {
	switch x := v.(type) {
	case map[string]any:
		return Merge(x, nil)
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = clone(e)
		}
		return out
	}
	return v
}

// Lookup gives the value at a dot path of a configuration, such as model.width.
func Lookup(cfg map[string]any, dotted string) (any, bool) {
	var cur any = cfg
	for _, k := range strings.Split(dotted, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		if cur, ok = m[k]; !ok {
			return nil, false
		}
	}
	return cur, true
}

// Effect is a size of effect: relative to the mean of the control, or absolute in the unit of the metric.
type Effect struct {
	Kind  string  `yaml:"kind"` // relative | absolute
	Value float64 `yaml:"value"`
}

// Of gives the effect as an absolute size, for a control mean.
func (e *Effect) Of(control float64) float64 {
	if e == nil {
		return 0
	}
	if e.Kind == "relative" {
		return e.Value * abs(control)
	}
	return e.Value
}

// Compare names the two arms of a comparative test.
type Compare struct {
	Treatment string `yaml:"treatment"`
	Control   string `yaml:"control"`
}

// Test is one test of expect.yaml.
type Test struct {
	Metric    string   `yaml:"metric"`
	At        string   `yaml:"at,omitempty"` // final | best | step:<n> | curve
	Compare   *Compare `yaml:"compare,omitempty"`
	Direction string   `yaml:"direction,omitempty"` // lower | higher
	Test      string   `yaml:"test"`
	Alpha     float64  `yaml:"alpha,omitempty"`
	MinEffect *Effect  `yaml:"min_effect,omitempty"`
	// Margin is the margin of an equivalence test (tost).
	Margin *Effect `yaml:"margin,omitempty"`
	// Arm and Rate are for the test rate: the arm whose failure rate is tested, and the claimed rate.
	Arm  string  `yaml:"arm,omitempty"`
	Rate float64 `yaml:"rate,omitempty"`
	// X, Form, R2Min and Range are for the test fit: the dot path of the variable in the configuration of the arms, the form (power or linear), the least R², and the claimed range of the fitted exponent or slope.
	X     string    `yaml:"x,omitempty"`
	Form  string    `yaml:"form,omitempty"`
	R2Min float64   `yaml:"r2_min,omitempty"`
	Range []float64 `yaml:"range,omitempty"`
	// MinShare is for the test shape: the least share of seeds whose curve shows the feature.
	MinShare float64 `yaml:"min_share,omitempty"`
	Note     string  `yaml:"note,omitempty"`
}

// Tests is one test or a list of tests.
type Tests []Test

// UnmarshalYAML reads one test or a list of tests.
func (t *Tests) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.MappingNode {
		var one Test
		if err := n.Decode(&one); err != nil {
			return err
		}
		*t = Tests{one}
		return nil
	}
	var many []Test
	if err := n.Decode(&many); err != nil {
		return err
	}
	*t = many
	return nil
}

// Report sets the confidence level and the bootstrap of the report.
type Report struct {
	CI        float64 `yaml:"ci,omitempty"`
	Bootstrap struct {
		Resamples int    `yaml:"resamples,omitempty"`
		Method    string `yaml:"method,omitempty"`
		Seed      uint64 `yaml:"seed,omitempty"`
	} `yaml:"bootstrap,omitempty"`
}

// Expect is the file expect.yaml.
type Expect struct {
	Claim      string `yaml:"claim"`
	Primary    Tests  `yaml:"primary"`
	Secondary  Tests  `yaml:"secondary,omitempty"`
	Correction string `yaml:"correction,omitempty"` // holm | none
	Report     Report `yaml:"report,omitempty"`
}

// TestNames are the names of the tests of spec 9.3.2.
var TestNames = map[string]bool{"welch-t": true, "mann-whitney": true, "bootstrap-diff": true, "tost": true, "fit": true, "shape": true, "rate": true}

// Comparative are the tests that compare a treatment arm with a control arm.
var Comparative = map[string]bool{"welch-t": true, "mann-whitney": true, "bootstrap-diff": true, "tost": true}

// Experiment is one experiment directory.
type Experiment struct {
	Slug string // the name of the directory
	Dir  string // the path of the directory
	Rel  string // the path of the directory, relative to the content root
	// Design, Expect and Results are nil when the file is not there or cannot be read. Errs holds the errors.
	Design    *Design
	Expect    *Expect
	Results   *Results
	HasScript bool
	Errs      []error
}

// File gives the path of a file of the experiment, relative to the content root.
func (e *Experiment) File(name string) string { return e.Rel + "/" + name }

// Read reads the experiments of a node directory. rel is the path of the node directory, relative to the content root. It returns nil when the node has no exp directory.
func Read(nodeDir, rel string) ([]*Experiment, error) {
	ents, err := os.ReadDir(filepath.Join(nodeDir, Dir))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []*Experiment
	for _, d := range ents {
		if !d.IsDir() || strings.HasPrefix(d.Name(), ".") {
			continue
		}
		e := &Experiment{Slug: d.Name(), Dir: filepath.Join(nodeDir, Dir, d.Name()), Rel: rel + "/" + Dir + "/" + d.Name()}
		e.read()
		out = append(out, e)
	}
	return out, nil
}

func (e *Experiment) read() {
	var d Design
	if ok, err := readYAML(filepath.Join(e.Dir, DesignFile), &d); err != nil {
		e.Errs = append(e.Errs, fmt.Errorf("%s: %w", DesignFile, err))
	} else if ok {
		e.Design = &d
	}
	var x Expect
	if ok, err := readYAML(filepath.Join(e.Dir, ExpectFile), &x); err != nil {
		e.Errs = append(e.Errs, fmt.Errorf("%s: %w", ExpectFile, err))
	} else if ok {
		e.Expect = &x
	}
	if _, err := os.Stat(filepath.Join(e.Dir, Script)); err == nil {
		e.HasScript = true
	}
	b, err := os.ReadFile(filepath.Join(e.Dir, ResultsFile))
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		e.Errs = append(e.Errs, err)
	default:
		var r Results
		if err := json.Unmarshal(b, &r); err != nil {
			e.Errs = append(e.Errs, fmt.Errorf("%s: %w", ResultsFile, err))
		} else {
			e.Results = &r
		}
	}
}

// readYAML reads a YAML file with known fields only. It returns false and no error when the file does not exist.
func readYAML(path string, v any) (bool, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(v); err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	return true, nil
}

// ExpectHash gives the hash of expect.yaml, as results.json records it.
func (e *Experiment) ExpectHash() (string, error) { return hashFiles(e.Dir, []string{ExpectFile}) }

// Inputs gives the files whose content decides the result, relative to the experiment directory: experiment.yaml, run.py, the generators, and the implementation of the node that the experiment uses.
func (e *Experiment) Inputs() []string {
	in := []string{DesignFile, Script}
	if e.Design != nil {
		for _, d := range e.Design.Data {
			if d.Generator != "" {
				in = append(in, d.Generator)
			}
		}
		if e.Design.UsesImpl != "" {
			in = append(in, e.Design.UsesImpl)
		}
	}
	sort.Strings(in)
	return in
}

// InputsHash gives the hash of the inputs, as results.json records it.
func (e *Experiment) InputsHash() (string, error) { return hashFiles(e.Dir, e.Inputs()) }

// hashFiles hashes the names and the contents of the files, in order. A file that is not there counts as empty, so that the hash changes when it comes back.
func hashFiles(dir string, names []string) (string, error) {
	h := sha256.New()
	for _, n := range names {
		b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(n)))
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		fmt.Fprintf(h, "%s\x00%d\x00", n, len(b))
		h.Write(b)
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
