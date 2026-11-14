// SPDX-License-Identifier: Apache-2.0

package status

// Ladder is one computed ladder (spec 04.10): its letter, its name, the part of the spec that defines it, and the name of
// each level. schema/levels.v1.yaml is made from Ladders and Thresholds.
type Ladder struct {
	ID     string   `yaml:"id"`
	Name   string   `yaml:"name"`
	Spec   string   `yaml:"spec"`
	Levels []string `yaml:"levels"`
}

// Ladders are the ladders that the build computes, in the order of 04.10. H, L and I are the proof, Lean and
// implementation ladders of an algorithm node, which the site shares with algorithm.tamnd.com.
var Ladders = []Ladder{
	{"W", "writing", "06.10", wNames},
	{"G", "grounding", "07, 13.6", gNames},
	{"X", "execution", "08.10", xNames},
	{"R", "reproduction", "09.11", rNames},
	{"F", "formal", "11", fNames},
	{"V", "visualization", "12", vNames},
	{"D", "data facts", "04.9, 10.11", dNames},
	{"H", "human proof", "06.5", hNames},
	{"L", "Lean algorithm", "11", lNames},
	{"I", "implementation", "08", iNames},
}

// Thresholds are the numbers that the ladders compare with (06.10, 08.8).
var Thresholds = struct {
	// Sources is the least number of sources of W1 to W4; Sources[0] is for W0 and is 0.
	Sources [5]int `yaml:"w_sources"`
	// PitfallItems is the least number of items of the Pitfalls section at W3.
	PitfallItems int `yaml:"w3_pitfall_items"`
	// MutationScore is the share of the mutants that the tests of an implementation kill at I2 (rule I04).
	MutationScore float64 `yaml:"i_mutation_score"`
	// XMutationScore is the share for a framework version at X3 (rule X06).
	XMutationScore float64 `yaml:"x_mutation_score"`
}{
	Sources:        [5]int{0, 1, 2, 3, 5},
	PitfallItems:   2,
	MutationScore:  MinScore,
	XMutationScore: XMinScore,
}
