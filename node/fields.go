// SPDX-License-Identifier: Apache-2.0

package node

import (
	"reflect"
	"slices"
	"strings"
)

// KindFields are the front matter fields of each kind (spec 04.5), beyond the fields that every kind may have.
// A node keeps them in Front.Fields, by name. A field that is neither common nor of the node's kind is an error,
// so that a misspelt key does not pass. Computed fields (a benchmark's saturation, a policy's status, a risk's
// incidents) are not in the lists: the build writes them.
var KindFields = map[string][]string{
	"architecture": {"family", "inputs", "outputs", "components", "inductive_biases", "params", "flops_per_token", "flops_per_example", "introduced_by", "models"},
	"model":        {"role", "developer", "released", "architecture", "params", "training", "modalities", "context_length", "access", "licence", "fine_tuned_from", "distilled_from", "member_of", "card_url"},
	"task":         {"inputs", "outputs", "modality", "family", "metrics", "benchmarks", "subtask_of"},
	"benchmark":    {"task", "versions", "metrics", "protocol", "contamination", "licence", "introduced_by"},
	"dataset":      {"modality", "size", "source", "collection", "licence", "access", "documentation", "derived_from", "known_issues"},
	"metric":       {"measures", "range", "better", "formula", "reference_impl"},
	"theorem":      {"statement", "hypotheses", "proof", "tight"},
	"finding":      {"claim", "first_reported", "replications", "scale_range", "failures", "explanations"},
	"argument":     {"premises", "conclusion", "replies", "responds_to"},
	"open-problem": {"question", "positions", "evidence", "would_settle_it"},
	"risk":         {"taxonomy", "affects", "mitigations"},
	"policy":       {"jurisdiction", "type", "dates", "provisions", "amends", "amended_by", "official_text"},
	"system":       {"developer", "years", "parts", "domain", "milestones"},
	"software":     {"language", "repo", "licence", "first_release", "maintainer", "category", "releases"},
	"hardware":     {"vendor", "type", "released", "specs"},
	"person":       {"roles"},
	"organization": {"kind_of"},
}

// kindField tells if key is a field of the kind.
func kindField(kind, key string) bool {
	return slices.Contains(KindFields[kind], key)
}

var common = func() map[string]bool {
	m := map[string]bool{}
	t := reflect.TypeOf(Front{})
	for i := 0; i < t.NumField(); i++ {
		if name, _, _ := strings.Cut(t.Field(i).Tag.Get("yaml"), ","); name != "" && name != "-" {
			m[name] = true
		}
	}
	return m
}()

// commonField tells if key is a field that every kind may have, a field of Front.
func commonField(key string) bool {
	return common[key]
}
