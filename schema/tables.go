// SPDX-License-Identifier: Apache-2.0

package schema

import (
	"bytes"
	"fmt"

	"go.yaml.in/yaml/v3"

	"github.com/tamnd/ai-agent/facts"
)

// generated heads every YAML file that the package makes from the code.
const generated = "# Generated from package %s by go test ./schema -update. Do not edit; change the code.\n"

func encodeYAML(pkg string, v any) ([]byte, error) {
	var b bytes.Buffer
	fmt.Fprintf(&b, generated, pkg)
	e := yaml.NewEncoder(&b)
	e.SetIndent(2)
	if err := e.Encode(v); err != nil {
		return nil, err
	}
	return b.Bytes(), e.Close()
}

// FactsTable is facts.v1.yaml (spec 10.11, 04.9): the required facts of each kind, the units, the fact ids that need
// as_of, the tolerances under which two sources agree, and the names of the common fact ids.
type FactsTable struct {
	Version   int    `yaml:"version"`
	Spec      string `yaml:"spec"`
	Tolerance struct {
		Number float64 `yaml:"number"`
		Date   string  `yaml:"date"`
		Text   string  `yaml:"text"`
		List   string  `yaml:"list"`
	} `yaml:"tolerance"`
	Units     []string            `yaml:"units"`
	CardUnits map[string]string   `yaml:"card_units"`
	TimeBound []string            `yaml:"time_bound"`
	Required  map[string][]string `yaml:"required"`
	Labels    map[string]string   `yaml:"labels"`
}

// Facts gives the table of facts.v1.yaml.
func Facts() FactsTable {
	t := FactsTable{Version: 1, Spec: "04.9, 10.11", Units: facts.Units, CardUnits: facts.CardUnits,
		TimeBound: facts.TimeBoundIDs, Required: facts.Required, Labels: facts.Labels}
	t.Tolerance.Number = facts.Tolerance
	t.Tolerance.Date = "the precision of the less precise date"
	t.Tolerance.Text = "case and surrounding space"
	t.Tolerance.List = "order"
	return t
}

// FactsYAML encodes facts.v1.yaml.
func FactsYAML() ([]byte, error) { return encodeYAML("facts", Facts()) }
