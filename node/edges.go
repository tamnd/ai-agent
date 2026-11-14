// SPDX-License-Identifier: Apache-2.0

package node

// Edge is one type of edge of spec 04.7: a field of the front matter or of a card that names other items. The build
// derives the reverse edges. schema/edges.md is made from Edges.
type Edge struct {
	Type    string // the field, such as prereqs
	From    string // the kinds that have the field
	To      string // the kinds or the things that it names
	Meaning string
	// Acyclic is true for the edge types that may have no cycle (rule E of 15.3).
	Acyclic bool
}

// Edges are the edge types of 04.7, in the order of its table.
var Edges = []Edge{
	{"prereqs", "any", "any", "read the target first; it forms the reading DAG (5.7)", true},
	{"uses", "method, architecture, system, model", "method, architecture, concept, algorithm bridge", "it calls, contains or depends on the target", false},
	{"components", "architecture", "architecture, method", "the blocks it is made of", false},
	{"variant_of", "method, architecture, metric, benchmark", "same kind", "it changes the target in a named way", true},
	{"related_to", "any", "any", "a sibling or a close idea, with a one-line note on the difference", false},
	{"generalizes", "concept, method, task", "same kind", "the target is a special case", true},
	{"applies_to", "method", "architecture, task", "the method is used with the target", false},
	{"solves", "method, architecture, model, system", "task", "it is used for the task", false},
	{"evaluated_on", "model, method", "benchmark version", "it has results there (derived from result cards)", false},
	{"measured_by", "task, benchmark", "metric", "the metric of the task or the benchmark", false},
	{"trained_on", "model", "dataset", "with the share, when known", false},
	{"architecture", "model", "architecture", "the architecture of the model", false},
	{"fine_tuned_from", "model", "model", "the lineage of weights", true},
	{"distilled_from", "model", "model", "the lineage of weights", false},
	{"merged_from", "model", "model", "the lineage of weights", false},
	{"member_of", "model", "model family", "the family", false},
	{"developed_by", "model, system, software, hardware, benchmark, dataset", "organization, person", "who made it", false},
	{"introduced_by", "any", "work", "the work that introduced it", false},
	{"cites", "work", "work", "from OpenAlex or Semantic Scholar, only between works that have cards", false},
	{"proves", "theorem", "concept, method, architecture", "the theorem is about the target", false},
	{"explains", "concept, theorem", "finding", "a proposed explanation, with its status in claims.yaml", false},
	{"evidence_for", "finding, work, result, experiment", "finding, open-problem, claim", "evidence for the target, with the claim id (07.3)", false},
	{"evidence_against", "finding, work, result, experiment", "finding, open-problem, claim", "evidence against the target, with the claim id (07.3)", false},
	{"instance_of", "incident", "risk", "the incident is a case of the risk", false},
	{"mitigates", "method, policy, standard", "risk", "it reduces the risk, with the evidence label", false},
	{"regulates", "policy, provision", "system kind, risk, practice", "the law covers it", false},
	{"implements_policy", "standard", "policy, provision", "a harmonized standard or a code of practice for the provision", false},
	{"amends", "policy", "policy", "the amendment", false},
	{"responds_to", "argument, work", "argument", "a reply to the target", false},
	{"succeeded_by", "model, hardware, software, benchmark", "same kind", "the next version", true},
	{"people", "any", "person", "who found, named, built or improved it", false},
	{"orgs", "any", "organization", "who found, named, built or improved it", false},
	{"events", "any", "event", "it took part in the event", false},
	{"exercises", "any", "exercise", "practice for the target", false},
	{"same_as", "any", "external ids", "Wikidata, Wikipedia, arXiv, DOI, OpenAlex, Semantic Scholar, DBLP, Epoch, Hub, AIID, ORCID", false},
	{"see_also", "any", "any", "a related page, at most 8", false},
}
