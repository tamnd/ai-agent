// The kinds of spec 04.2 with the sections of spec 06.3.

package node

import "slices"

// KindInfo describes one of the 28 kinds of spec 04.2 and the sections of spec 06.3.
type KindInfo struct {
	ID        string   // the value of the kind field
	Name      string   // the name in a sentence
	Plural    string   // the plural name
	Group     string   // ideas, artifacts, governance, record or card (spec 06.3)
	Form      string   // node or card: the default form of spec 04.2
	Intro     string   // one or two sentences for the page of the kind
	Rank      int      // the order in search results; a higher rank comes first
	Sections  []string // the fixed ## headings, in order
	Required  []string // the sections in bold in spec 06.3, required at W2
	Generated []string // the sections that the build writes
}

// Main is the main section of the kind, required at W1 (spec 04.10). It is the first required section.
func (k KindInfo) Main() string {
	if len(k.Required) == 0 {
		return ""
	}
	return k.Required[0]
}

// CardOnly tells if the kind has no page and no sections (spec 06.3).
func (k KindInfo) CardOnly() bool {
	return len(k.Sections) == 0
}

// KindTable holds the 28 kinds in the order of spec 04.2.
var KindTable = []KindInfo{
	{
		ID: "area", Name: "area", Plural: "areas", Group: "ideas", Form: "node", Rank: 30,
		Intro:     "A node of the area tree. It lists the sub-areas or the nodes of a sub-area in reading order.",
		Sections:  []string{"Scope", "Map", "Key ideas", "History", "Open problems", "Courses and paths", "References"},
		Required:  []string{"Scope", "Map", "Key ideas", "References"},
		Generated: []string{"Map", "Open problems", "Courses and paths", "References"},
	},
	{
		ID: "concept", Name: "concept", Plural: "concepts", Group: "ideas", Form: "node", Rank: 44,
		Intro:     "An idea that AI uses to think about learning, reasoning or agents, such as overfitting or attention.",
		Sections:  []string{"In plain words", "The idea", "Formal statement", "Examples", "Where it is used", "Related ideas", "Common confusions", "Formal proof", "History", "References"},
		Required:  []string{"The idea", "Examples", "References"},
		Generated: []string{"Where it is used", "Formal proof", "References"},
	},
	{
		ID: "definition", Name: "definition", Plural: "definitions", Group: "ideas", Form: "card", Rank: 34,
		Intro:     "The exact meaning of a term, as in a glossary.",
		Sections:  []string{"Definition", "Examples and non-examples", "Usage", "Related terms", "References"},
		Required:  []string{"Definition", "Examples and non-examples", "References"},
		Generated: []string{"Related terms", "References"},
	},
	{
		ID: "method", Name: "method", Plural: "methods", Group: "ideas", Form: "node", Rank: 50,
		Intro:     "A named way to learn, train, infer, prompt, align, compress or evaluate. Each page gives the equations, a derivation, the cost, tested code and the evidence.",
		Sections:  []string{"In plain words", "Setting", "The method", "Derivation", "Worked example", "Hyperparameters", "Cost", "Evidence", "Limitations and failure modes", "Variants", "When to use it", "Pitfalls", "Implementations", "Experiments", "Formal proof", "History", "References"},
		Required:  []string{"Setting", "The method", "Derivation", "Worked example", "Hyperparameters", "Cost", "Evidence", "References"},
		Generated: []string{"Evidence", "Implementations", "Experiments", "Formal proof", "References"},
	},
	{
		ID: "architecture", Name: "architecture", Plural: "architectures", Group: "ideas", Form: "node", Rank: 49,
		Intro:     "A design of a model: its layers, its computation and the shapes of its parameters.",
		Sections:  []string{"In plain words", "Overview", "Components", "Shapes and parameters", "Computation and cost", "Training", "Inductive biases", "Evidence", "Limitations", "Variants and descendants", "Notable models", "Implementations", "Experiments", "History", "References"},
		Required:  []string{"Overview", "Components", "Shapes and parameters", "Computation and cost", "Evidence", "References"},
		Generated: []string{"Shapes and parameters", "Evidence", "Variants and descendants", "Notable models", "Implementations", "Experiments", "References"},
	},
	{
		ID: "model", Name: "model", Plural: "models", Group: "artifacts", Form: "card", Rank: 46,
		Intro:     "A trained model or a family of models, with its developer, date, size, data, licence and results.",
		Sections:  []string{"Overview", "Architecture", "Training data", "Training", "Releases and versions", "Results", "Access and licence", "Safety and evaluations", "Reception and impact", "Known issues", "Lineage", "References"},
		Required:  []string{"Overview", "Architecture", "Training data", "Training", "Results", "References"},
		Generated: []string{"Releases and versions", "Results", "Known issues", "Lineage", "References"},
	},
	{
		ID: "task", Name: "task", Plural: "tasks", Group: "artifacts", Form: "node", Rank: 47,
		Intro:     "A problem that AI systems solve, with its input, its output and how success is measured.",
		Sections:  []string{"In plain words", "Definition", "Contract", "Metrics", "Benchmarks", "Approaches", "Results", "Difficulties", "Subtasks and related tasks", "Applications", "History", "References"},
		Required:  []string{"Definition", "Metrics", "Benchmarks", "Approaches", "References"},
		Generated: []string{"Contract", "Metrics", "Benchmarks", "Results", "Subtasks and related tasks", "References"},
	},
	{
		ID: "benchmark", Name: "benchmark", Plural: "benchmarks", Group: "artifacts", Form: "card", Rank: 42,
		Intro:     "A fixed evaluation with data, a protocol and metrics. Results attach to a version of a benchmark.",
		Sections:  []string{"What it measures", "Data", "Protocol", "Versions", "Results", "Known issues", "Reception", "History", "References"},
		Required:  []string{"What it measures", "Data", "Protocol", "Results", "References"},
		Generated: []string{"Versions", "Results", "References"},
	},
	{
		ID: "dataset", Name: "dataset", Plural: "datasets", Group: "artifacts", Form: "card", Rank: 40,
		Intro:     "A collection of data for training or fine-tuning, with its source, size, licence and documentation.",
		Sections:  []string{"Contents", "Collection and construction", "Documentation and licence", "Uses", "Known issues", "Versions and derivatives", "History", "References"},
		Required:  []string{"Contents", "Collection and construction", "Documentation and licence", "References"},
		Generated: []string{"Uses", "Versions and derivatives", "References"},
	},
	{
		ID: "metric", Name: "metric", Plural: "metrics", Group: "artifacts", Form: "node", Rank: 45,
		Intro:     "A measure of quality or cost, with a formula, test vectors and a reference implementation.",
		Sections:  []string{"In plain words", "Definition", "Properties", "Worked example", "Reference implementation", "Interpretation", "Limitations and criticism", "Variants", "Implementations", "History", "References"},
		Required:  []string{"Definition", "Properties", "Worked example", "Reference implementation", "References"},
		Generated: []string{"Implementations", "References"},
	},
	{
		ID: "theorem", Name: "theorem", Plural: "theorems", Group: "ideas", Form: "node", Rank: 43,
		Intro:     "A proved result about learning, inference or agents.",
		Sections:  []string{"Statement", "In plain words", "Proof", "Assumptions", "Tightness", "Consequences", "Illustration", "Formal proof", "History", "References"},
		Required:  []string{"Statement", "Proof", "References"},
		Generated: []string{"Consequences", "Formal proof", "References"},
	},
	{
		ID: "finding", Name: "finding", Plural: "findings", Group: "ideas", Form: "node", Rank: 41,
		Intro:     "An empirical result that the field relies on or debates, with its first report, its replications and its failures to replicate.",
		Sections:  []string{"In plain words", "The finding", "Evidence", "Scope and conditions", "Explanations", "Reproduction here", "Implications", "Open questions", "History", "References"},
		Required:  []string{"The finding", "Evidence", "Scope and conditions", "References"},
		Generated: []string{"Evidence", "Reproduction here", "Open questions", "References"},
	},
	{
		ID: "argument", Name: "argument", Plural: "arguments", Group: "ideas", Form: "node", Rank: 38,
		Intro:     "A philosophical or conceptual argument, with its premises, its conclusion and the replies to it.",
		Sections:  []string{"In plain words", "The argument", "Context", "Replies", "Influence", "Related arguments", "References"},
		Required:  []string{"The argument", "Context", "Replies", "References"},
		Generated: []string{"Related arguments", "References"},
	},
	{
		ID: "open-problem", Name: "open problem", Plural: "open problems", Group: "ideas", Form: "node", Rank: 37,
		Intro:     "A question that the field has not settled, with each position and the evidence for it.",
		Sections:  []string{"The question", "Why it matters", "Positions", "Evidence so far", "What would settle it", "History of the debate", "References"},
		Required:  []string{"The question", "Why it matters", "Positions", "Evidence so far", "References"},
		Generated: []string{"Evidence so far", "References"},
	},
	{
		ID: "system", Name: "system", Plural: "systems", Group: "artifacts", Form: "card", Rank: 36,
		Intro:     "A built and named AI system, historical or deployed, such as Deep Blue or AlphaGo.",
		Sections:  []string{"Overview", "How it works", "Development", "Results and milestones", "Impact and reception", "Limitations and incidents", "Legacy", "References"},
		Required:  []string{"Overview", "How it works", "References"},
		Generated: []string{"Legacy", "References"},
	},
	{
		ID: "software", Name: "software", Plural: "software", Group: "artifacts", Form: "card", Rank: 35,
		Intro:     "A library, framework, engine or tool built for AI.",
		Sections:  []string{"What it is for", "Core concepts", "Example", "Ecosystem and alternatives", "History", "References"},
		Required:  []string{"What it is for", "Core concepts", "Example", "References"},
		Generated: []string{"References"},
	},
	{
		ID: "hardware", Name: "hardware", Plural: "hardware", Group: "artifacts", Form: "card", Rank: 33,
		Intro:     "A chip, accelerator, system or interconnect built for AI or used for it.",
		Sections:  []string{"Architecture", "Use in AI", "Comparison", "History", "References"},
		Required:  []string{"Architecture", "Use in AI", "References"},
		Generated: []string{"Use in AI", "Comparison", "References"},
	},
	{
		ID: "risk", Name: "risk", Plural: "risks", Group: "ideas", Form: "node", Rank: 39,
		Intro:     "A class of harm or failure that AI can cause or suffer.",
		Sections:  []string{"In plain words", "Definition and scope", "How it arises", "Examples", "Evidence", "Mitigations", "Taxonomies", "Debates", "References"},
		Required:  []string{"Definition and scope", "How it arises", "Examples", "Evidence", "Mitigations", "References"},
		Generated: []string{"Examples", "Evidence", "Taxonomies", "References"},
	},
	{
		ID: "incident", Name: "incident", Plural: "incidents", Group: "governance", Form: "card", Rank: 22,
		Intro:     "A dated event where AI caused harm or nearly caused harm.",
		Sections:  []string{"What happened", "Systems and parties", "Harms", "Response", "Risk classes", "References"},
		Required:  []string{"What happened", "Systems and parties", "Harms", "References"},
		Generated: []string{"Risk classes", "References"},
	},
	{
		ID: "policy", Name: "policy", Plural: "policies", Group: "governance", Form: "card", Rank: 32,
		Intro:     "A law, regulation, treaty, voluntary framework or lab policy about AI.",
		Sections:  []string{"What it does", "Scope and definitions", "Main provisions", "Obligations by actor", "Timeline", "Enforcement and penalties", "Amendments and implementing acts", "Interpretation and debate", "Relation to other laws and standards", "References"},
		Required:  []string{"What it does", "Scope and definitions", "Main provisions", "Obligations by actor", "Timeline", "References"},
		Generated: []string{"Main provisions", "Timeline", "Amendments and implementing acts", "Relation to other laws and standards", "References"},
	},
	{
		ID: "provision", Name: "provision", Plural: "provisions", Group: "governance", Form: "card", Rank: 24,
		Intro:     "One article, section or requirement of a policy or a standard.",
		Sections:  []string{"What it requires", "Applies to", "Dates", "Related provisions", "Official guidance", "References"},
		Required:  []string{"What it requires", "Applies to", "Dates", "References"},
		Generated: []string{"Dates", "Related provisions", "References"},
	},
	{
		ID: "standard", Name: "standard", Plural: "standards", Group: "governance", Form: "card", Rank: 28,
		Intro:     "A technical standard or specification about AI.",
		Sections:  []string{"What it specifies", "Structure", "Adoption and status", "Relation to policy", "History", "References"},
		Required:  []string{"What it specifies", "References"},
		Generated: []string{"Adoption and status", "Relation to policy", "References"},
	},
	{
		ID: "result", Name: "result", Plural: "results", Group: "card", Form: "card", Rank: 12,
		Intro: "One published number: a model, a benchmark version, a setting, a score, a source and a date.",
	},
	{
		ID: "work", Name: "work", Plural: "works", Group: "record", Form: "card", Rank: 26,
		Intro:     "A paper, book, report, essay, talk or course that is part of the record of AI.",
		Sections:  []string{"Contributions", "Context", "Methods and results", "Reception and influence", "Criticism, replications and corrections", "References"},
		Required:  []string{"Contributions", "Context", "References"},
		Generated: []string{"Reception and influence", "Criticism, replications and corrections", "References"},
	},
	{
		ID: "person", Name: "person", Plural: "people", Group: "record", Form: "card", Rank: 20,
		Intro:     "A person who made, named or shaped an item of AI. Pages give public professional facts only.",
		Sections:  []string{"Contributions", "Career", "Recognition", "References"},
		Required:  []string{"Contributions", "References"},
		Generated: []string{"Contributions", "Recognition", "References"},
	},
	{
		ID: "organization", Name: "organization", Plural: "organizations", Group: "record", Form: "card", Rank: 21,
		Intro:     "A lab, company, university group, institute, standards body, regulator or funder.",
		Sections:  []string{"Overview", "History", "Work", "People", "References"},
		Required:  []string{"Overview", "References"},
		Generated: []string{"Work", "References"},
	},
	{
		ID: "event", Name: "event", Plural: "events", Group: "record", Form: "card", Rank: 23,
		Intro:     "A dated happening, such as a workshop, a competition, a summit or a milestone game.",
		Sections:  []string{"What happened", "Outcomes and significance", "References"},
		Required:  []string{"What happened", "References"},
		Generated: []string{"References"},
	},
	{
		ID: "exercise", Name: "exercise", Plural: "exercises", Group: "card", Form: "card", Rank: 10,
		Intro: "A practice problem in a public course or on a public site.",
	},
}

// Kind returns the description of the kind k.
func Kind(k string) (KindInfo, bool) {
	i := slices.IndexFunc(KindTable, func(x KindInfo) bool { return x.ID == k })
	if i < 0 {
		return KindInfo{}, false
	}
	return KindTable[i], true
}
