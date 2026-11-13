# 02. The goal

## 2.1 One sentence

**Every idea, method, architecture, model, task, benchmark, dataset, result, system, risk, law, argument, person and event of artificial intelligence, in one place, with every equation derived, every reference implementation tested, every empirical claim labelled with its evidence and its date, every benchmark number traced to its source, and every gap measured, at https://ai.tamnd.com.**

## 2.2 What counts as knowledge about AI

The user's goal is "ALL human knowledge related to AI, NO MISSING". The scope is everything that a researcher, an engineer, a student, a policy maker or a curious reader knows or looks up about AI, from the Dartmouth proposal of 1955 to the model released last week. It falls into nine groups (05.2):

| Group | What | Examples | Spec |
|---|---|---|---|
| A. Foundations | what AI is, agents and rationality, learning theory, the mathematics that AI uses as a bridge to math2 | rational agent, PAC learning, VC dimension, bias–variance, no free lunch | 05.2 A |
| B. Symbolic and classical AI | search, planning, knowledge representation, logic, reasoning under uncertainty, expert systems, game playing, multi-agent systems | A\*, STRIPS, description logics, Bayesian networks, MYCIN, minimax, contract net | 05.2 B |
| C. Machine learning | supervised, unsupervised and semi-supervised learning, probabilistic models, kernels, ensembles, reinforcement learning, bandits, causal inference, evolutionary computation | SVM, EM, Gaussian processes, gradient boosting, Q-learning, PPO, Thompson sampling, do-calculus | 05.2 C |
| D. Deep learning and foundation models | neural network foundations, training, architectures, generative models, self-supervised learning, language models, post-training, reasoning models, multimodal models, efficient models | backpropagation, Adam, ResNet, Transformer, Mamba, diffusion, BERT, scaling laws, RLHF, DPO, chain of thought, CLIP, LoRA, quantization | 05.2 D |
| E. Perception, language and action | NLP, speech, vision, robotics, agents and tool use, retrieval and recommendation | machine translation, ASR, object detection, SLAM, ReAct, RAG, collaborative filtering | 05.2 E |
| F. Applications | AI in science, medicine, code, law, finance, games, art, education, transport, defence | AlphaFold, code assistants, autonomous driving levels, AI-generated art | 05.2 F |
| G. Systems and engineering | data, training and serving systems, parallelism, kernels, hardware, MLOps, evaluation practice, software frameworks | FineWeb, ZeRO, FlashAttention, vLLM, the H100, the TPU, PyTorch, model cards | 05.2 G |
| H. Safety, ethics and governance | alignment, interpretability, robustness and security, privacy, fairness, the law and policy of AI, standards, incidents | reward hacking, sparse autoencoders, prompt injection, differential privacy, equalized odds, the EU AI Act, ISO/IEC 42001, an AIID incident | 05.2 H |
| I. Context | history, philosophy and cognitive science of AI, people, organizations, works, events, AI and the economy and society | the Dartmouth workshop, the AI winters, the Chinese room, Geoffrey Hinton, DeepMind, "Attention Is All You Need", the ImageNet moment | 05.2 I |

Three neighbours own parts of this space, and ai.tamnd.com links to them and does not repeat them (5.9):

- **algorithm.tamnd.com (2150)** owns algorithm-shaped items: a procedure with an input and output contract whose correctness and cost can be proved without a learning experiment. It already holds 419 machine-learning nodes, 209 games-and-search nodes and 325 logic-and-constraints nodes (01.6). Examples: k-means (Lloyd's algorithm), the junction tree algorithm, alpha-beta pruning, DPLL, backpropagation as reverse-mode differentiation. ai.tamnd.com shows each of them as a **bridge card** with its summary and a link, and writes the AI view (when to use it in practice, its empirical record, its history in AI) only when the AI view adds facts that the algorithm page does not have.
- **math2 (2148)** owns mathematics: linear algebra, probability, optimization theory, information theory as mathematics.
- **programming.tamnd.com (2150p)** owns general programming: languages, general libraries, tools. AI software (PyTorch, JAX, vLLM, llama.cpp, LangChain) belongs to ai.tamnd.com, and programming keeps a card with a `same_as` link.

"No missing" has a precise meaning (18.1). Every item that any trusted source names has an outcome: a node, a card, an alias, a section, a merge, a bridge to a neighbour, or out of scope under a published rule. The site shows the gaps. It does not mean a page for each of the 3 million models on the Hugging Face Hub (01.2). It means that the rule that selects which models get a card is published, and every model that the rule selects has one (18.5).

## 2.3 The quality promise

A page on this site says what it claims and shows evidence that a reader can check:

1. **Every claim has a type and an evidence label.** A sentence that states a fact about AI is a claim (07). It is proved (a theorem, with a proof on the page or in Lean), measured here (an experiment in this repository reproduced it), replicated (independent published replications exist), reported (one source says it), or contested (sources disagree, and the page gives each position with its sources). The label is computed, never written by hand.
2. **Every number has a source and a date.** A parameter count, a training compute figure, a benchmark score or a release date comes from a fact card with a cited source and an "as of" date (04.9). Prose never says "state of the art", "the best model" or "currently". Those phrases come only from computed tables with their dates (06.7, rule K04).
3. **Every equation is derived or cited.** A loss, an update rule or an architecture equation is either derived on the page step by step, or cited to the exact equation of its source, and its notation follows the notation register (06.4).
4. **Every reference implementation runs.** Each method, architecture and metric with a reference implementation has tests: shape tests, gradient checks, test vectors from the official implementation, and agreement between the NumPy, PyTorch and JAX versions (08). The build runs them in pinned images with no network.
5. **Empirical claims are reproduced where it is cheap.** A method or finding whose claim can be shown at small scale has an experiment in its node (09). It runs with fixed seeds and pinned data in a fixed compute budget, and it reports confidence intervals. The page never presents a small-scale reproduction as a reproduction of the paper's full result.
6. **Contested questions show every serious position.** Safety, policy, philosophy and open scientific debates (emergent abilities, the limits of scaling, AI consciousness, existential risk) are written as positions with their best sources, not as the site's opinion (06.9, rule N01).
7. **Agents write, checks gate, people review.** Every page shows what an agent wrote, what the checks verified, and what a person reviewed (15.5).
8. **Nothing stale.** Frontier facts are computed from the registers and dated. A daily job adds new models, benchmarks, results, incidents and laws as cards within one day of a trusted source listing them (14.9).

## 2.4 The scale

| Item | At M5 (Estimate, 18.8) |
|---|---|
| nodes (full pages) | about 32,000 |
| cards (catalogue entries) | about 480,000 |
| bridge cards to algorithm, math2 and programming | about 3,000 |
| top areas / sub-areas | 52 / about 615 |
| model cards (Epoch AI, model cards on the Hub by the scope rule, lab announcements) | about 25,000 |
| result cards (model × benchmark × setting × score × source) | about 150,000 |
| work cards (papers, books, reports) | about 200,000 |
| reference implementations (node × framework) | about 9,000 |
| experiments (reproductions in the repository) | about 2,500 |
| Lean theorems in the `AI` package | about 600 |

This is about four times algorithm in nodes and similar to programming in cards. The cards drive the card model (04.1), the hosting (16.20) and the search design (17).

## 2.5 Who it is for

| Reader | Needs | Served by |
|---|---|---|
| learner | a path from zero to "I understand how a transformer is trained", with runnable code | paths (16.12), level `intro` (5.6), the in-browser notebooks (12.8) |
| student and teacher | precise definitions, derivations, exercises, a course map | concept and method pages, derivations (06.5), exercises, the crosswalks to CS2023 and university syllabi (5.8) |
| engineer | "which method for this task", "what does this hyperparameter do", "how much memory does this need" | task pages, method pages with pitfalls, the calculators (16.14), system pages |
| researcher | the related work of an idea, its lineage, who replicated it, which benchmark numbers are comparable | the edge graph (04.7), the results register (10), the lineage views (16.9) |
| policy maker, lawyer, journalist | what a term means, what a law requires, what happened in an incident, what the evidence says | policy, risk and incident pages, the glossary, the evidence labels (07) |
| agent | correct, dated, machine-readable facts | `.md` and `.json` twins, datasets, `llms.txt`, MCP (16.10) |

## 2.6 Non-goals

- **Not a news site.** New releases become cards and nodes with a fixed form (06.3), not news posts.
- **Not a leaderboard host.** The results register records published results with sources and settings (10.6). It never runs closed models, and it never ranks models outside a computed table with its date, its benchmark version and its settings.
- **Not a model or dataset host.** Weights and datasets stay where they are published. The site links to them and pins hashes. Small datasets that experiments use are mirrored in R2 only when their licences allow it (09.5).
- **Not an opinion site.** No position on contested questions (rule N01). No rankings of labs or people.
- **Not a paper index.** Work cards exist for the works that nodes cite and for the works that the scope rule selects (18.5), not for every one of the more than 4 million works of the OpenAlex AI subfield (01.2).
- **No server execution in v1.** Readers run code and small models in the browser only (12.8, Q-09).
- **No copying.** Text and figures from sources that we may not adapt are never copied (19).
- **English only until M5** (Q-17).

## 2.7 Relation to other tamnd projects

| Project | Relation |
|---|---|
| algorithm (2150) | the model for everything: repositories, packages, tags, ladders, the pipeline of `algorithm-work`, hosting; owns algorithm-shaped ML, search, planning and logic items (5.9) |
| programming (2150p) | the second model: cards, harvest first, the agent surface, Workers and R2 from day one, the verifier; owns general languages and tools |
| math2 (2148) | owns mathematics; the source of `markup`, `copycheck`, `prov` and the MCP pattern |
| 2111, 2114 (training tracks and corpora) | worked examples of real training runs that the systems pages can cite as experience reports, not as sources of fact |
| the `*-cli` tools in the author's workspace (`arxiv-cli`, `openalex-cli`, `semanticscholar-cli`, `dblp-cli`, `openreview-cli`, `aclanthology-cli`, `cvf-cli`, `neurips-cli`, `huggingface-cli`, `kaggle-cli`, `wikidata-cli`, `wikipedia-cli`, `distill-cli`, `alignmentforum-cli`, `lesswrong-cli`, `philpapers-cli`, `eleutherai-cli`, `interpretableml-cli` and others) | wrapped by harvest adapters (13.2) |
