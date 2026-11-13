package node

// Group is a heading of the area map (spec 05.2). It is not a directory level: a node directory is nodes/<top>/<sub>/<slug>/.
type Group struct {
	Letter string
	Name   string
	Intro  string
}

// Groups are the 9 groups of spec 05.2, in reading order.
var Groups = []Group{
	{"A", "Foundations", "What AI is, learning theory, and the mathematics and optimization that AI uses."},
	{"B", "Symbolic and classical AI", "Search, planning, knowledge representation, logic, reasoning under uncertainty, games and multi-agent systems."},
	{"C", "Machine learning", "Supervised and unsupervised learning, probabilistic models, kernels, ensembles, reinforcement learning and causal inference."},
	{"D", "Deep learning and foundation models", "Neural networks and their training, the architectures, generative models, language models and post-training."},
	{"E", "Perception, language and action", "Language, speech, vision, robotics, agents and tool use, retrieval and recommendation."},
	{"F", "Applications", "AI in science, medicine, code, law, finance, games, art, education, transport and defence."},
	{"G", "Systems and engineering", "Data, training and serving systems, hardware, MLOps, evaluation practice and software frameworks."},
	{"H", "Safety, ethics and governance", "Alignment, interpretability, robustness, privacy, fairness, the law and policy of AI, standards and incidents."},
	{"I", "Context", "History, philosophy and cognitive science of AI, people, organizations, works, events, the economy and society."},
}

// GroupByLetter returns the group with the letter, or nil if no group has it.
func GroupByLetter(letter string) *Group {
	for i := range Groups {
		if Groups[i].Letter == letter {
			return &Groups[i]
		}
	}
	return nil
}
