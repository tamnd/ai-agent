// SPDX-License-Identifier: Apache-2.0

package check

import (
	"slices"
	"strings"
	"testing"
)

const expDesign = `id: fit-toy
claims: [c-fits]
question: "Does the toy model fit the data?"
scale_note:
  here: "Ten points of a line."
  source: ""
arms:
  - {id: control}
  - {id: treated, seeds: {n: 3, base: 1}, tuning: {budget: 5}}
  - {id: other, tuning: {budget: 9}}
base_config: {epochs: 3}
data:
  - {generator: points.py}
  - {generator: missing.py}
  - {pin: "S:mnist-ai-data"}
  - {pin: "S:closed-data"}
  - {pin: "S:no-such-pin"}
  - {dataset: mnist}
seeds: {n: 3, base: 1}
budget: cpu-huge
framework: tensorflow
uses_impl: ../../impl/numpy/toy.py
metrics: [loss]
`

const expExpect = `claim: c-other
primary:
  - {metric: loss, test: welch-t, compare: {treatment: treated, control: absent}, direction: lower}
  - {metric: accuracy, test: shape}
  - {metric: loss, test: fit}
  - {metric: loss, test: magic}
`

const expPins = `- {id: mnist-ai-data, dataset: mnist, version: lecun-1998, licence: {spdx: CC-BY-SA-3.0}, mirror: allowed, files: [{name: a.gz, sha256: abc}]}
- {id: closed-data, dataset: closed, version: "1", licence: {spdx: unknown}, mirror: unknown}
`

const expResults = `{"experiment":"other","node":"7TQYMD","run":"","seeds":[1,2],"arms":{"control":{}},"primary":{"metric":"loss","test":"welch-t","passed":false,"verdict":"inconclusive"},"verdict":"inconclusive","hardware":{"class":"gpu-small"},"wall_time_s":900,"expect_hash":"sha256:00"}`

func TestExperimentRules(t *testing.T) {
	ed := modelDir + "exp/fit-toy/"
	c := fixture(t, map[string]string{
		modelDir + "node.md":                       modelNode + "\nWe reproduced the paper on a laptop.\n",
		modelDir + "claims.yaml":                   claimFile,
		ed + "experiment.yaml":                     expDesign,
		ed + "expect.yaml":                         expExpect,
		ed + "results.json":                        expResults,
		ed + "points.py":                           "\n",
		"registers/data-pins.yaml":                 expPins,
		modelDir + "exp/empty/run.py":              "\n",
		modelDir + "exp/broken/experiment.yaml":    "id: [\n",
		modelDir + "exp/no-expect/experiment.yaml": strings.Replace(expDesign, "id: fit-toy", "id: no-expect", 1),
	})
	fs := Run(c, Options{})
	for rule, want := range map[string][]string{
		"R01": {
			"exp/fit-toy/experiment.yaml: level: the experiment has no run.py",
			`exp/fit-toy/experiment.yaml: level: the budget "cpu-huge" is not cpu-small`,
			`exp/fit-toy/experiment.yaml: level: the framework "tensorflow" is not in`,
			"exp/fit-toy/experiment.yaml: level: uses_impl: the file ../../impl/numpy/toy.py is not there",
			`exp/fit-toy/expect.yaml: level: the claim "c-other" is not a claim of experiment.yaml`,
			`exp/fit-toy/expect.yaml: level: loss: the arm "absent" is not an arm of experiment.yaml`,
			`exp/fit-toy/expect.yaml: level: the metric "accuracy" is not in the metrics`,
			"exp/fit-toy/expect.yaml: warn: accuracy: aik cannot run the test shape yet",
			"exp/fit-toy/expect.yaml: level: loss: a trend needs at least 5 points of the variable; the design has 3 arms",
			"exp/fit-toy/expect.yaml: level: loss: the test fit needs x",
			`exp/fit-toy/expect.yaml: level: loss: the test "magic" is not welch-t`,
			"exp/empty/experiment.yaml: level: the experiment empty has no experiment.yaml",
			"exp/broken: block: experiment.yaml:",
		},
		"R02": {
			"data 2: the generator missing.py is not in the experiment directory",
			"data 4: the licence of the pin closed-data is not resolved",
			`data 5: the pin "S:no-such-pin" is not in registers/data-pins.yaml`,
			"data 6: give a pin of registers/data-pins.yaml or a generator",
		},
		"R03": {"exp/fit-toy/experiment.yaml: level: scale_note needs here and source"},
		"R04": {`node.md: level: "We reproduced the paper" presents a small-scale experiment as a reproduction`},
		"R05": {
			"exp/fit-toy/results.json: block: results.json has no run id or no hashes",
			`exp/fit-toy/results.json: block: results.json is for the experiment "other", not "fit-toy"`,
			"exp/fit-toy/results.json: block: the seeds of results.json are not the seeds of the design",
			"exp/fit-toy/results.json: block: results.json has no values of the arm treated",
		},
		"R06": {
			"exp/fit-toy/results.json: block: expect.yaml changed after the run",
			"exp/no-expect/expect.yaml: block: the experiment has no expect.yaml",
		},
		"R08": {
			"the arm treated has seeds of its own",
			"the arm other has the tuning budget 9, but the arm treated has 5",
		},
		"R09": {
			"exp/fit-toy/results.json: level: the run was in the class \"gpu-small\", but the design asks for cpu-huge",
		},
	} {
		got := fired(fs, rule)
		for _, w := range want {
			if !slices.ContainsFunc(got, func(s string) bool { return strings.Contains(s, w) }) {
				t.Errorf("%s does not report %q; it reports:\n%s", rule, w, strings.Join(got, "\n"))
			}
		}
	}
	if got := fired(fs, "R02"); slices.ContainsFunc(got, func(s string) bool {
		return strings.Contains(s, "fit-toy") && (strings.Contains(s, "data 1:") || strings.Contains(s, "data 3:"))
	}) {
		t.Errorf("R02 reports a good input:\n%s", strings.Join(got, "\n"))
	}
}

func TestExperimentClean(t *testing.T) {
	ed := modelDir + "exp/fit-toy/"
	good := strings.NewReplacer(`source: ""`, `source: "The source fits a line too."`, "budget: cpu-huge", "budget: cpu-small", "framework: tensorflow\n", "", "uses_impl: ../../impl/numpy/toy.py\n", "",
		"  - {id: treated, seeds: {n: 3, base: 1}, tuning: {budget: 5}}\n", "  - {id: treated}\n", "  - {id: other, tuning: {budget: 9}}\n", "",
		"  - {generator: missing.py}\n", "", "  - {pin: \"S:closed-data\"}\n", "", "  - {pin: \"S:no-such-pin\"}\n", "", "  - {dataset: mnist}\n", "").Replace(expDesign)
	c := fixture(t, map[string]string{
		modelDir + "node.md":       modelNode,
		modelDir + "claims.yaml":   claimFile,
		ed + "experiment.yaml":     good,
		ed + "expect.yaml":         "claim: c-fits\nprimary: {metric: loss, test: welch-t, compare: {treatment: treated, control: control}, direction: lower}\n",
		ed + "run.py":              "\n",
		ed + "points.py":           "\n",
		"registers/data-pins.yaml": expPins,
	})
	for _, f := range Run(c, Options{}) {
		if f.Rule[0] == 'R' {
			t.Errorf("a good experiment gives %s %s: %s", f.Rule, f.Path, f.Msg)
		}
	}
}
