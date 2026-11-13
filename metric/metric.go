// SPDX-License-Identifier: Apache-2.0

// Package metric reads the contract and the test vectors of a metric node (spec 04.4 and 08.7.5). Every framework
// version's score must reproduce every vector within the contract's tolerance (rule X05).
package metric

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// ContractFile is the contract of a metric, in the node directory.
const ContractFile = "contract.yaml"

// VectorsDir holds the test vectors, in the node directory.
const VectorsDir = "vectors"

// Arg is an input, an option or an output of the score function.
type Arg struct {
	Type    string `yaml:"type"`
	Meaning string `yaml:"meaning"`
	Default any    `yaml:"default,omitempty"`
	Values  []any  `yaml:"values,omitempty"`
}

// Tolerance is how close a framework's number must be to the official one: |got - want| <= atol + rtol |want|.
type Tolerance struct {
	Rtol float64 `yaml:"rtol"`
	Atol float64 `yaml:"atol"`
}

// Contract is contract.yaml of a metric node.
type Contract struct {
	// Entry is the name of the score function; the default is score (08.3.3).
	Entry     string          `yaml:"entry"`
	Inputs    map[string]*Arg `yaml:"inputs"`
	Options   map[string]*Arg `yaml:"options"`
	Output    map[string]*Arg `yaml:"output"`
	Tolerance Tolerance       `yaml:"tolerance"`
}

// Exists tells if dir has a metric contract.
func Exists(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ContractFile))
	return err == nil
}

// Load reads contract.yaml of the metric node in dir.
func Load(dir string) (*Contract, error) {
	b, err := os.ReadFile(filepath.Join(dir, ContractFile))
	if err != nil {
		return nil, err
	}
	return Parse(b)
}

// Parse reads a metric contract and checks it.
func Parse(b []byte) (*Contract, error) {
	var k Contract
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(&k); err != nil {
		return nil, err
	}
	if k.Entry == "" {
		k.Entry = "score"
	}
	var bad []string
	if !ident(k.Entry) {
		bad = append(bad, fmt.Sprintf("the entry %q is not a Python name", k.Entry))
	}
	for _, need := range []string{"predictions", "references"} {
		if k.Inputs[need] == nil {
			bad = append(bad, "the inputs have no "+need+" (08.3.3: score(predictions, references, **options))")
		}
	}
	if len(k.Output) == 0 {
		bad = append(bad, "the contract names no output")
	}
	if k.Tolerance.Rtol < 0 || k.Tolerance.Atol < 0 || k.Tolerance.Rtol+k.Tolerance.Atol == 0 {
		bad = append(bad, "the tolerance needs rtol and atol, not negative, and not both 0")
	}
	for _, m := range []map[string]*Arg{k.Inputs, k.Options, k.Output} {
		for name, a := range m {
			if a == nil || a.Type == "" || a.Meaning == "" {
				bad = append(bad, name+" needs a type and a meaning")
			}
		}
	}
	if len(bad) > 0 {
		sort.Strings(bad)
		return nil, errors.New(strings.Join(bad, "; "))
	}
	return &k, nil
}

func ident(s string) bool {
	for i, r := range s {
		letter := r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
		digit := i > 0 && r >= '0' && r <= '9'
		if !letter && !digit {
			return false
		}
	}
	return s != ""
}

// Vector is one row of a vectors/*.jsonl file: the inputs, the options that differ from the defaults, and the
// output of the official implementation.
type Vector struct {
	ID          string         `json:"id"`
	Predictions any            `json:"predictions"`
	References  any            `json:"references"`
	Options     map[string]any `json:"options,omitempty"`
	Want        any            `json:"want"`
	// By names the official implementation and its version, as "sacrebleu 2.5.1"; a vector computed by hand says
	// "hand" and has a note.
	By        string `json:"by"`
	Signature string `json:"signature,omitempty"`
	Note      string `json:"note,omitempty"`
	// File and Line say where the vector is, for messages.
	File string `json:"-"`
	Line int    `json:"-"`
}

// Files lists the vector files of the node in dir, relative to dir, sorted: official.jsonl, edge.jsonl and the files
// of variants.yaml.
func Files(dir string) ([]string, error) {
	m, err := filepath.Glob(filepath.Join(dir, VectorsDir, "*.jsonl"))
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(m))
	for _, p := range m {
		out = append(out, VectorsDir+"/"+filepath.Base(p))
	}
	sort.Strings(out)
	return out, nil
}

// ReadVectors reads every vector file of the node in dir and checks each row against the contract.
func ReadVectors(dir string, k *Contract) ([]Vector, error) {
	files, err := Files(dir)
	if err != nil {
		return nil, err
	}
	var out []Vector
	var bad []string
	seen := map[string]string{}
	for _, f := range files {
		vs, err := readFile(filepath.Join(dir, filepath.FromSlash(f)))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f, err)
		}
		for _, v := range vs {
			v.File = f
			where := fmt.Sprintf("%s:%d", f, v.Line)
			switch {
			case v.ID == "":
				bad = append(bad, where+": the vector has no id")
			case seen[v.ID] != "":
				bad = append(bad, fmt.Sprintf("%s: the id %s is also at %s", where, v.ID, seen[v.ID]))
			default:
				seen[v.ID] = where
			}
			if v.Predictions == nil || v.References == nil || v.Want == nil {
				bad = append(bad, where+": the vector needs predictions, references and want")
			}
			if v.By == "" {
				bad = append(bad, where+": the vector does not say which implementation computed it (by)")
			} else if v.By == "hand" && v.Note == "" {
				bad = append(bad, where+": a vector computed by hand needs a note that says why (08.7.5)")
			}
			if k != nil {
				for name := range v.Options {
					if k.Options[name] == nil {
						bad = append(bad, fmt.Sprintf("%s: the option %s is not in %s", where, name, ContractFile))
					}
				}
				if w, ok := v.Want.(map[string]any); ok {
					for name := range w {
						if k.Output[name] == nil {
							bad = append(bad, fmt.Sprintf("%s: the output %s is not in %s", where, name, ContractFile))
						}
					}
				}
			}
			out = append(out, v)
		}
	}
	if len(bad) > 0 {
		return out, errors.New(strings.Join(bad, "; "))
	}
	return out, nil
}

func readFile(p string) ([]Vector, error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []Vector
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 16<<20)
	for line := 1; sc.Scan(); line++ {
		if len(bytes.TrimSpace(sc.Bytes())) == 0 {
			continue
		}
		var v Vector
		dec := json.NewDecoder(bytes.NewReader(sc.Bytes()))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&v); err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		v.Line = line
		out = append(out, v)
	}
	return out, sc.Err()
}
