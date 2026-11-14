// SPDX-License-Identifier: Apache-2.0

package schema

import (
	"bytes"
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/tamnd/ai-agent/coverage"
	"github.com/tamnd/ai-agent/facts"
	"github.com/tamnd/ai-agent/node"
	"github.com/tamnd/ai-agent/status"
)

// generated heads every YAML file that the package makes from the code.
const generated = "# Generated from the code of %s by go test ./schema -update. Do not edit; change the code.\n"

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
func FactsYAML() ([]byte, error) { return encodeYAML("package facts", Facts()) }

// LevelsTable is levels.v1.yaml (spec 04.10, 06.10, 18.2): the ladders with the names of their levels, the coverage
// levels, and the thresholds that the ladders compare with.
type LevelsTable struct {
	Version    int             `yaml:"version"`
	Spec       string          `yaml:"spec"`
	Ladders    []status.Ladder `yaml:"ladders"`
	Coverage   []string        `yaml:"coverage"`
	Thresholds any             `yaml:"thresholds"`
}

// Levels gives the table of levels.v1.yaml.
func Levels() LevelsTable {
	return LevelsTable{Version: 1, Spec: "04.10, 06.10, 18.2", Ladders: status.Ladders, Coverage: coverage.Names,
		Thresholds: status.Thresholds}
}

// TargetsTable is targets.v1.yaml (spec 04.10, 18.2): what each coverage level from C2 to C6 needs of the ladders.
type TargetsTable struct {
	Version   int               `yaml:"version"`
	Spec      string            `yaml:"spec"`
	CodeKinds []string          `yaml:"code_kinds"`
	Targets   []coverage.Target `yaml:"targets"`
}

// Targets gives the table of targets.v1.yaml.
func Targets() TargetsTable {
	return TargetsTable{Version: 1, Spec: "04.10, 18.2", CodeKinds: coverage.CodeKinds, Targets: coverage.Targets}
}

// EdgesMarkdown is edges.md (spec 04.7): the table of the edge types.
func EdgesMarkdown() []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "<!-- %s -->\n\n", strings.TrimSpace(strings.TrimPrefix(fmt.Sprintf(generated, "package node"), "#")))
	b.WriteString("# Edges, version 1\n\n")
	b.WriteString("Edges are typed fields of the front matter or the card (spec 04.7). The build derives the reverse edges, and the audit rules E01 to E14 check them (15.3). A neighbour node of algorithm, math2 or programming is named through a bridge card (5.9).\n\n")
	b.WriteString("| Edge | From | To | Meaning | Acyclic |\n|---|---|---|---|---|\n")
	var acyclic []string
	for _, e := range node.Edges {
		a := ""
		if e.Acyclic {
			a = "yes"
			acyclic = append(acyclic, "`"+e.Type+"`")
		}
		fmt.Fprintf(&b, "| `%s` | %s | %s | %s | %s |\n", e.Type, e.From, e.To, e.Meaning, a)
	}
	list := strings.Join(acyclic[:len(acyclic)-1], ", ") + " or " + acyclic[len(acyclic)-1]
	fmt.Fprintf(&b, "\nNo cycle is allowed in %s. The targets of `solves` are tasks, of `evaluated_on` benchmark versions and of `architecture` architectures, and each core node has a `prereqs` path from a foundation node (5.7).\n", list)
	return b.Bytes()
}
