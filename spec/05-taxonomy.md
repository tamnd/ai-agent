# 05. Taxonomy

This document defines the topic tree that holds the nodes, the entry indexes (tasks, models, benchmarks, risks, laws), the facets, the levels, the core list, the prerequisites and paths, the crosswalks to outside classifications, and the border with algorithm, math2 and programming.

A reader comes with one of five questions:

1. "What is X?": they know the name (an acronym, a model, a paper). Search answers it (17).
2. "How do I do this?": they know the task, not the method. The task index answers it (5.3).
3. "What is in this field?": they want a map. The area tree answers it (5.2).
4. "What should I learn next?": they want an order. Prerequisites, paths and courses answer it (5.7).
5. "What is known, by whom, and since when?": they want the record. The catalogues of models, benchmarks, results, works, incidents and laws answer it (5.4), with the evidence labels of 07.

## 5.1 The principles

- **One primary area for each node.** The primary area gives the directory (04.4), the breadcrumb and the hue of the page. Other areas are secondary (`areas`) and list the node too. This is algorithm's rule.
- **The tree is three levels.** Group (A to I), top area, sub-area; then the node. The group is a heading on the map, not a directory level: directories are `nodes/<top>/<sub>/<slug>/`, as in algorithm. A finer split is a facet or a section of the sub-area page.
- **Tasks are first-class.** Each method, architecture and model that is used for something `solves` at least one task (04.7), and each task page lists them with the results that compare them (10.7). This is the AI form of algorithm's "problems are first-class".
- **Ideas and artifacts are separate.** The tree holds ideas (concepts, methods, architectures, theorems, findings, arguments, risks). Artifacts (models, datasets, benchmarks, software, hardware, systems, policies) also have a home area, but readers mostly reach them through the catalogues of 5.4, which cut across the tree.
- **Methods cut across areas.** Attention appears in NLP, vision, speech and graphs. A method has one home (`deep-learning-architectures/attention-mechanisms`), and the other areas link it. They do not repeat it.
- **Crosswalks, not adoption.** The tree is our own. Each area maps to ACM CCS 2012, the CS2023 AI knowledge area, arXiv categories, the OpenAlex topics, Wikidata and the Wikipedia outlines (5.8), so that every outside list can be compared with it. No outside scheme fits teaching, research and reference use together; OpenAlex has 4,516 topics with no hierarchy below its 252 subfields, and CCS 2012 has no node for transformers (01.3).

## 5.2 The area tree

`topics/areas.yaml` defines the tree. Each top area and sub-area is also an `area` node with a page. Each sub-area slug is unique over the global namespace (04.3).

The first draft has 9 groups, 52 top areas and 615 sub-areas. The sizes are Estimates at M5 (18.8) from the inventory of 01.4.

### A. Foundations

| Top area | Sub-areas (first draft) | Crosswalk |
|---|---|---|
| `ai-foundations` | definitions-of-ai, agents-and-environments, rationality-and-bounded-rationality, decision-theory-for-ai, problem-formulation, paradigms-of-ai, measuring-intelligence, general-intelligence-concepts, computability-and-limits-of-ai | CCS I.2.0; CS2023 AI-Introduction; AIMA ch. 1 to 2 |
| `learning-theory` | statistical-learning-framework, pac-learning, vc-dimension-and-growth, rademacher-and-covering, generalization-bounds, pac-bayes, stability-and-generalization, online-learning-theory, learning-in-the-limit, computational-hardness-of-learning, no-free-lunch-and-inductive-bias, deep-learning-theory, kernel-regime-and-ntk, implicit-bias-and-overparameterization | CCS "Theory of computation › Machine learning theory"; arXiv stat.ML, cs.LG |
| `math-for-ai` | linear-algebra-for-ml, matrix-calculus-and-autodiff, probability-for-ml, statistics-for-ml, information-theory-for-ml, high-dimensional-geometry, concentration-and-random-matrices, numerical-methods-for-ml, graphs-for-ml, functional-analysis-for-ml | mostly bridge cards to math2 (5.9); MML book table of contents |
| `optimization-for-learning` | empirical-risk-minimization, gradient-descent-and-variants, stochastic-gradient-methods, momentum-and-acceleration, adaptive-optimizers, second-order-and-quasi-newton, constrained-and-proximal-methods, nonconvex-optimization-theory, learning-rate-schedules, large-batch-and-distributed-optimization, zeroth-order-and-black-box, hyperparameter-optimization, bilevel-and-meta-optimization | arXiv math.OC ∩ cs.LG; bridge to algorithm `optimization/gradient-methods` |

### B. Symbolic and classical AI

| Top area | Sub-areas (first draft) | Crosswalk |
|---|---|---|
| `search-and-evolutionary-computation` | state-space-search, uninformed-search, informed-search-and-heuristics, local-search, metaheuristics, genetic-algorithms, genetic-programming, evolution-strategies-and-cma, swarm-intelligence, neuroevolution, quality-diversity-and-open-endedness, artificial-life | CCS I.2.8; arXiv cs.NE; AIMA ch. 3 to 4; algorithm `games-and-search` bridges |
| `constraints-and-satisfiability` | constraint-satisfaction-problems, constraint-propagation, constraint-programming, sat-in-ai, smt-and-optimization-modulo-theories, distributed-constraints, constraint-learning | AIMA ch. 6; algorithm `logic-and-constraints` bridges |
| `planning-and-scheduling` | classical-planning, planning-languages, heuristic-planning, hierarchical-task-networks, temporal-and-resource-planning, planning-under-uncertainty, epistemic-and-multi-agent-planning, scheduling-in-ai, plan-recognition, generalized-planning, planning-with-language-models | CCS I.2.8; ICAPS topics; Ghallab-Nau-Traverso |
| `knowledge-representation` | propositional-and-first-order-logic-for-ai, description-logics-and-ontologies, semantic-web, knowledge-graphs, frames-and-semantic-networks, commonsense-knowledge, nonmonotonic-reasoning, belief-revision, reasoning-about-action-and-change, spatial-and-temporal-reasoning, answer-set-programming, argumentation-frameworks, neuro-symbolic-ai | CCS I.2.4; KR conference topics |
| `automated-reasoning` | automated-theorem-proving, interactive-theorem-proving-and-ai, logic-programming, inductive-logic-programming, program-synthesis, neural-theorem-proving, ai-for-formal-verification | CCS F.4.1, I.2.3; arXiv cs.LO ∩ cs.AI |
| `probabilistic-reasoning` | uncertainty-and-probability-in-ai, bayesian-networks, markov-random-fields, exact-inference, sampling-based-inference, variational-inference, temporal-probabilistic-models, filtering-and-tracking, probabilistic-programming, decision-networks-and-value-of-information, dempster-shafer-and-other-calculi | AIMA ch. 12 to 16; Koller-Friedman; algorithm `machine-learning/probabilistic-inference` bridges |
| `classical-ai-systems` | expert-systems, rule-based-production-systems, case-based-reasoning, fuzzy-logic-and-soft-computing, cognitive-architectures, blackboard-and-agent-architectures, natural-language-systems-before-statistics | CCS I.2.1, I.2.5; history sources (13.2) |
| `games-and-multi-agent-systems` | game-theory-for-ai, adversarial-search-in-ai, monte-carlo-tree-search, imperfect-information-games, general-game-playing, game-ai-milestones, agent-architectures-and-bdi, coordination-and-cooperation, negotiation-and-bargaining, mechanism-design-and-auctions, computational-social-choice, agent-communication, emergent-communication | CCS I.2.11; arXiv cs.MA, cs.GT; AAMAS topics |

### C. Machine learning

| Top area | Sub-areas (first draft) | Crosswalk |
|---|---|---|
| `supervised-learning` | learning-problem-setup, linear-regression, logistic-regression-and-glms, nearest-neighbours, decision-trees, naive-bayes-and-generative-classifiers, support-vector-machines, kernel-methods, ensembles-and-bagging, boosting, gradient-boosted-trees, model-selection-and-validation, classical-regularization, feature-engineering, imbalanced-and-cost-sensitive, structured-prediction, learning-to-rank-classical, tabular-learning | CCS I.2.6, I.5; ESL 2e; algorithm `machine-learning/supervised-learning` bridges |
| `unsupervised-learning` | clustering-in-practice, dimensionality-reduction, manifold-learning, density-estimation, matrix-factorization, topic-models, anomaly-detection, independent-component-and-source-separation, classical-representation-learning | algorithm `clustering`, `dimensionality-reduction` bridges |
| `probabilistic-machine-learning` | bayesian-inference-for-ml, latent-variable-models, mixture-models-and-em, gaussian-processes, bayesian-nonparametrics, bayesian-neural-networks, uncertainty-quantification, conformal-prediction, approximate-bayesian-computation | Murphy PML 1 and 2; Bishop PRML |
| `learning-settings` | semi-supervised-learning, weak-supervision, positive-unlabeled-learning, active-learning, transfer-learning, domain-adaptation, multi-task-learning, meta-learning, few-shot-and-zero-shot, continual-learning, federated-learning, online-and-streaming-learning, curriculum-learning, learning-with-noisy-labels | arXiv cs.LG; NeurIPS subject areas |
| `time-series-and-sequential-data` | forecasting-classical, deep-forecasting, time-series-classification, sequence-anomaly-detection, spatio-temporal-learning, time-series-foundation-models, event-sequences-and-point-processes | arXiv stat.ML; Monash and other archives (13.2) |
| `reinforcement-learning` | markov-decision-processes, dynamic-programming-in-rl, monte-carlo-and-td-learning, function-approximation-in-rl, deep-q-learning, policy-gradient-methods, actor-critic-methods, model-based-rl, exploration, multi-armed-bandits, contextual-bandits, offline-rl, imitation-learning, inverse-rl-and-reward-learning, hierarchical-rl, multi-agent-rl, partially-observable-rl, rl-theory, reward-design-and-shaping, rl-environments-and-benchmarks | Sutton-Barto 2e chapters; CCS I.2.6; algorithm `machine-learning/reinforcement-learning` bridges |
| `causal-inference-and-discovery` | causal-graphical-models, structural-causal-models, identification-and-do-calculus, potential-outcomes, treatment-effect-estimation, causal-discovery, counterfactual-reasoning, causal-representation-learning, causality-and-invariance-in-ml | Pearl; Peters-Janzing-Schölkopf |

### D. Deep learning and foundation models

| Top area | Sub-areas (first draft) | Crosswalk |
|---|---|---|
| `deep-learning-foundations` | neural-network-basics, activation-functions, loss-functions, backpropagation-and-autodiff, initialization, expressivity-and-approximation, depth-and-width, inductive-biases-of-networks, biological-inspiration | Goodfellow ch. 6; Prince UDL ch. 1 to 7; D2L |
| `deep-learning-training` | regularization, normalization, data-augmentation, training-dynamics, batch-size-and-learning-rate, mixed-precision-and-numerics, training-stability, loss-landscapes, grokking-and-double-descent, hyperparameter-transfer, training-recipes | Goodfellow ch. 7 to 8; Prince UDL ch. 8 to 9 |
| `deep-learning-architectures` | feedforward-networks, convolutional-networks, recurrent-networks, residual-and-skip-connections, attention-mechanisms, transformers, position-encodings, state-space-and-linear-recurrent-models, mixture-of-experts, graph-neural-networks, equivariant-and-geometric-networks, memory-augmented-networks, neural-odes-and-implicit-models, hypernetworks-and-dynamic-networks, hybrid-architectures | Goodfellow ch. 9 to 10; Bronstein et al. GDL; arXiv cs.LG, cs.NE |
| `generative-models` | autoencoders, variational-autoencoders, generative-adversarial-networks, normalizing-flows, diffusion-models, flow-matching, autoregressive-models, energy-based-models, discrete-and-masked-generative-models, latent-generative-models, guidance-and-conditioning, consistency-and-distillation-of-samplers | Prince UDL ch. 14 to 18; Bishop-Bishop 2024 |
| `self-supervised-learning` | pretext-tasks, contrastive-learning, masked-modelling, self-distillation, joint-embedding-predictive-architectures, embeddings-and-metric-learning, word-and-sentence-embeddings | arXiv cs.CV, cs.LG |
| `language-models` | statistical-language-models, neural-language-models, tokenization, pretraining-objectives, encoder-models, decoder-only-models, encoder-decoder-models, scaling-laws, in-context-learning, long-context, decoding-and-sampling, emergent-and-surprising-behaviour, knowledge-and-memorization-in-lms, hallucination, language-model-evaluation | SLP3 ch. 3, 7 to 12; arXiv cs.CL |
| `post-training` | supervised-fine-tuning, instruction-tuning, reward-modelling, rlhf, preference-optimization, ai-feedback-and-constitutional-methods, parameter-efficient-fine-tuning, model-merging, safety-training-and-refusals, system-prompts-and-personas, continued-pretraining-and-domain-adaptation | arXiv cs.CL, cs.LG |
| `reasoning-and-test-time-compute` | prompting, chain-of-thought, self-consistency-and-voting, search-at-inference, reinforcement-learning-with-verifiable-rewards, process-and-outcome-reward-models, test-time-scaling, reasoning-faithfulness, mathematical-reasoning-in-models, planning-in-language-models | arXiv cs.CL, cs.AI |
| `multimodal-models` | vision-language-pretraining, vision-language-models, text-to-image, text-to-video, text-to-3d, audio-language-models, any-to-any-models, multimodal-grounding, multimodal-evaluation | arXiv cs.CV ∩ cs.CL |
| `efficient-ai` | quantization, pruning-and-sparsity, knowledge-distillation, efficient-attention, kv-cache-and-memory-reduction, speculative-and-parallel-decoding, efficient-architectures-for-devices, neural-architecture-search, early-exit-and-adaptive-compute, energy-and-carbon-of-ai | arXiv cs.LG, cs.AR |
| `foundation-models` | model-families-and-lineage, frontier-closed-models, open-weight-llms, small-language-models, code-models, vision-foundation-models, speech-foundation-models, scientific-foundation-models, multimodal-model-families, model-releases-and-cards, open-and-closed-access | Epoch AI, the AI Index, the Hub (13.2); the home of model nodes |

### E. Perception, language and action

| Top area | Sub-areas (first draft) | Crosswalk |
|---|---|---|
| `natural-language-processing` | text-preprocessing, text-classification, sequence-labelling, syntactic-parsing, semantic-parsing, lexical-and-compositional-semantics, information-extraction, entity-linking, question-answering, machine-translation, summarization, dialogue-systems, natural-language-generation, sentiment-and-affect, discourse-and-pragmatics, multilingual-and-low-resource-nlp, computational-linguistics-foundations | CCS I.2.7; arXiv cs.CL; ACL Anthology areas; SLP3 |
| `speech-and-audio` | speech-recognition, speech-synthesis, speaker-recognition-and-diarization, spoken-language-understanding, speech-translation, audio-classification-and-tagging, audio-generation, music-information-retrieval, music-generation, source-separation-and-enhancement | arXiv eess.AS, cs.SD; Interspeech areas |
| `computer-vision` | image-formation-and-features, image-classification, object-detection, semantic-and-instance-segmentation, open-vocabulary-and-promptable-vision, 3d-reconstruction-and-geometry, neural-rendering, video-understanding, tracking, human-pose-and-motion, faces-and-biometrics, ocr-and-document-understanding, low-level-vision, image-generation-and-editing, visual-reasoning, remote-sensing | CCS I.4, I.5; arXiv cs.CV; Szeliski 2e; CVF topics |
| `robotics-and-embodied-ai` | robot-perception, localization-and-slam, motion-planning-in-robotics, manipulation, locomotion, robot-learning, sim-to-real, vision-language-action-models, human-robot-interaction, autonomous-driving, aerial-and-field-robotics, embodied-ai-simulators | CCS I.2.9; arXiv cs.RO; Thrun et al. |
| `ai-agents` | agent-loops-and-architectures, tool-use-and-function-calling, agent-memory, agent-planning-and-reflection, multi-agent-llm-systems, web-and-computer-use-agents, coding-agents, research-agents, agent-protocols, agent-sandboxing-and-permissions, agent-evaluation | arXiv cs.AI, cs.CL, cs.MA |
| `retrieval-and-recommendation` | information-retrieval-models, sparse-retrieval, dense-retrieval, vector-indexes-in-practice, reranking, retrieval-augmented-generation, recommender-systems, collaborative-filtering, sequential-recommendation, search-and-ranking-systems | CCS H.3; arXiv cs.IR; algorithm `searching/similarity-search` bridges |

### F. Applications

| Top area | Sub-areas (first draft) | Crosswalk |
|---|---|---|
| `ai-for-science` | protein-structure-and-design, drug-discovery, chemistry-and-materials, genomics-and-biology, weather-and-climate, physics-and-simulation, astronomy-and-cosmology, mathematics-and-ai, earth-and-environment, autonomous-laboratories, science-agents | arXiv q-bio, physics.comp-ph, cond-mat ∩ cs.LG; the AI Index science chapter |
| `ai-applications` | healthcare-and-medicine, medical-imaging, software-engineering-and-code, law-and-legal-practice, finance-and-trading, education, creative-arts-and-design, writing-and-media, games-and-entertainment, transportation-and-logistics, manufacturing-and-industry, energy-and-utilities, agriculture-and-food, public-sector-and-government, military-and-defence, marketing-and-commerce, customer-service, accessibility-and-assistive, cybersecurity-applications | CCS "Applied computing"; the AI Index applications chapters |

### G. Systems and engineering

| Top area | Sub-areas (first draft) | Crosswalk |
|---|---|---|
| `data-for-ai` | web-crawls-and-corpora, data-curation-and-filtering, deduplication, data-labelling-and-annotation, synthetic-data, data-mixtures-and-curricula, data-documentation, dataset-licensing-and-consent, multimodal-datasets, evaluation-data-construction, data-attribution-and-valuation | the datasheets and data cards literature; Hub datasets (13.2) |
| `ml-systems` | training-systems, data-parallelism, tensor-and-sequence-parallelism, pipeline-parallelism, expert-parallelism, memory-optimization, compilers-and-graph-optimization, gpu-kernels, inference-serving, batching-and-scheduling, fault-tolerance-and-checkpointing, mlops-and-deployment, experiment-tracking-and-reproducibility, cluster-and-datacenter-scale | arXiv cs.DC ∩ cs.LG; MLSys topics |
| `ai-hardware` | gpus, tpus-and-custom-asics, other-accelerators, wafer-scale-and-dataflow, interconnects, memory-and-hbm, ai-datacenters-and-power, edge-and-mobile-npus, neuromorphic-and-analog, historical-ai-hardware, compute-trends-and-supply-chain | arXiv cs.AR; Epoch AI hardware data |
| `ai-software` | deep-learning-frameworks, classical-ml-libraries, model-hubs-and-weight-formats, training-libraries, serving-and-inference-engines, local-inference-runtimes, llm-application-frameworks, agent-frameworks-and-sdks, evaluation-harnesses, data-and-labelling-tools, rl-and-simulation-environments, notebooks-and-experiment-tools | the AI software that programming leaves to us (5.9); GitHub, PyPI (13.2) |
| `evaluation` | evaluation-methodology, classification-and-regression-metrics, ranking-and-retrieval-metrics, text-generation-metrics, image-and-video-generation-metrics, speech-metrics, statistical-testing-for-ml, benchmarks-and-their-design, leaderboards-and-comparability, human-evaluation-and-arenas, model-graded-evaluation, contamination-and-leakage, calibration-evaluation, capability-and-dangerous-capability-evaluations, efficiency-and-cost-evaluation | the home of metric and benchmark nodes (10) |

### H. Safety, ethics and governance

| Top area | Sub-areas (first draft) | Crosswalk |
|---|---|---|
| `ai-safety-and-alignment` | the-alignment-problem, specification-gaming-and-reward-hacking, goal-misgeneralization, scalable-oversight, deceptive-alignment-and-scheming, sycophancy-and-honesty, power-seeking-and-instrumental-convergence, ai-control, model-organisms-of-misalignment, catastrophic-and-existential-risk, agent-foundations, cooperative-ai, corrigibility, situational-awareness, safety-cases | the Alignment Forum and safety lab research (13.2); Hendrycks |
| `interpretability` | interpretable-models, feature-attribution, explainable-ai-methods, probing, mechanistic-interpretability, circuits, superposition-and-features, sparse-autoencoders-and-dictionaries, representation-engineering-and-steering, concept-based-explanations, interpretability-evaluation | Molnar (interpretable-ml); Distill and Transformer Circuits (13.2) |
| `robustness-security-and-privacy` | adversarial-examples, certified-and-adversarial-robustness, distribution-shift-and-ood, jailbreaks, prompt-injection, data-poisoning-and-backdoors, model-extraction, membership-and-attribute-inference, differential-privacy-in-ml, machine-unlearning, watermarking-and-content-provenance, synthetic-media-and-deepfakes, red-teaming, ai-supply-chain-security, ai-threat-taxonomies | arXiv cs.CR ∩ cs.LG; MITRE ATLAS; OWASP LLM Top 10 |
| `fairness-and-ethics` | ethics-of-ai, ai-principles-and-guidelines, fairness-definitions, bias-measurement, bias-mitigation, transparency-and-documentation, accountability-and-auditing, human-oversight-and-contestability, human-ai-interaction-and-trust, ai-and-human-rights, ai-and-labour-ethics, ai-welfare-and-moral-status | FAccT topics; arXiv cs.CY |
| `ai-governance-and-law` | eu-ai-law, united-states-ai-policy, china-ai-regulation, uk-ai-policy, other-national-ai-policies, international-ai-governance, ai-standards, frontier-ai-policies-and-commitments, compute-and-export-controls, copyright-and-ai, liability-and-product-safety, data-protection-and-ai, sector-regulation, ai-incidents-and-reporting, ai-safety-institutes | the policy sources of 13.2 (EUR-Lex, OECD.AI, NIST, legislation registers, AIID) |

### I. Context

| Top area | Sub-areas (first draft) | Crosswalk |
|---|---|---|
| `history-of-ai` | precursors-and-automata, cybernetics-and-early-computing, the-birth-of-ai, early-symbolic-ai, first-ai-winter, knowledge-based-systems-era, second-ai-winter, statistical-and-probabilistic-turn, connectionism-revival, deep-learning-revolution, transformer-and-foundation-model-era, histories-by-country | Wikipedia "History of AI" and timelines; Nilsson; McCorduck; Russell-Norvig ch. 1 |
| `philosophy-and-cognitive-science` | philosophy-of-ai, can-machines-think, consciousness-and-machine-minds, meaning-and-understanding, symbol-grounding, embodiment-and-situated-cognition, cognitive-science-and-ai, neuroscience-and-ai, computational-models-of-mind, theories-of-intelligence | PhilPapers "Philosophy of Artificial Intelligence" categories; SEP entries (names only) |
| `ai-and-society` | economics-of-ai, labour-and-automation, ai-industry-and-investment, ai-geopolitics, open-source-and-open-weights-movement, public-opinion-and-discourse, ai-in-culture-and-fiction, ai-forecasting-and-timelines, ai-and-education-systems, ai-and-information-ecosystem, ai-and-the-environment | the AI Index economy and policy chapters; arXiv cs.CY, econ |
| `people-and-institutions` | researchers, engineers-and-builders, labs-and-companies, universities-and-research-groups, government-bodies-and-regulators, standards-bodies, funders, conferences-and-venues, awards-and-prizes, competitions-and-challenges, landmark-works, notable-events | the home of person, organization, event and canon work nodes |

Rules:

- A top area or sub-area is a change to `topics/areas.yaml` in its own pull request, with a note on what it holds and what it does not, as in algorithm.
- A sub-area with more than 200 nodes splits, and one with fewer than 5 nodes after M3 merges into a neighbour (rule T09 reports both). The limit is 200, not algorithm's 150, because many AI sub-areas hold model and benchmark nodes that the reader reaches through the catalogues.
- Each sub-area slug is reviewed against the crosswalks at each release: every arXiv category of 5.8, every OpenAlex topic inside subfield 1702 with more than 2,000 works, and every Wikipedia "Outline of artificial intelligence" heading must map to some area (18.4).
- The tree is a draft. M0 and M1 test it against the inventory (13.3): the curate step of 14.3 places 2,000 random inventory items, and any item with no good home is a tree gap.

## 5.3 The task index

The task index is the "how do I do this" entrance, as algorithm's problem index is.
It lists the `task` nodes by area, each with a one-line description in plain words, its input and output, its main metrics, its main benchmarks, and the methods and models used for it, with dated results:

```
Computer vision › Object detection
  2D object detection            image → boxes + labels      mAP@[.5:.95]   COCO 2017, LVIS v1        two-stage detectors, one-stage detectors, DETR family
  Open-vocabulary detection      image + text → boxes        AP (novel)     LVIS v1 (rare), ODinW     grounded vision-language detectors
  3D object detection (LiDAR)    point cloud → 3D boxes      mAP, NDS       nuScenes, Waymo Open      voxel, pillar and point methods
Natural language processing › Question answering
  Extractive QA                  question + passage → span   EM, F1         SQuAD 1.1, SQuAD 2.0       encoder models, span heads
  Open-domain QA                 question → answer           EM             Natural Questions, TriviaQA retrievers + readers, RAG, LLMs
```

Each task node also has facet keys that the "I need to…" search (17.6) filters on: `modality` (text, image, video, audio, speech, tabular, graph, time series, 3D, multimodal, action), `output_type` (label, labels, span, sequence, box, mask, scalar, ranking, policy, program, structure), `supervision` (labelled, unlabelled, weak, reward, preference, none), `setting` (offline, online, interactive, embodied), and `compute_hint` (a rough class of the compute that common methods need, from the results register).

The task list starts from the inventory: the Papers with Code task list in its final data dump (about 4,000 tasks, many duplicates; 01.2), the Hugging Face Hub task taxonomy, the ACL and CVF topic lists, the Kaggle competition categories, and the canon. The curate step merges duplicates and gives each surviving task a place in the tree (14.3).

## 5.4 The catalogues

Readers reach artifacts mostly through catalogues, not the tree. Each catalogue is a collection page over a kind, with facets (5.5), computed tables, a timeline and a dataset download (16.6). The site routes of 04.3 belong to them.

| Catalogue | Kind | Route | Default sort | Key facets |
|---|---|---|---|---|
| Models | `model` | `/models/` | release date, newest first | developer, country, architecture family, parameters (log buckets), training compute (log buckets), modality, access, licence, notable (18.5) |
| Benchmarks | `benchmark` | `/benchmarks/` | task area, then introduction date | task, modality, metric, saturated (10.8), contamination reports, licence |
| Datasets | `dataset` | `/datasets/` | area, then size | modality, size, source type, licence, documentation present |
| Results | `result` | `/results/` and each benchmark version | per benchmark version, score | model, setting, evidence (reported, measured here, replicated), date |
| Works | `work` | `/works/` | year | venue, area, canon, citations (computed, dated) |
| Systems | `system` | `/systems/` | year | domain, developer, status |
| Software | `software` | `/software/` | area | category, language, licence |
| Hardware | `hardware` | `/hardware/` | release date | vendor, type, peak FLOP/s, memory |
| Risks | `risk` | `/risks/` | area | source taxonomy, affected part, incidents (count) |
| Incidents | `incident` | `/incidents/` | date, newest first | risk, sector, harm type, country, system |
| Laws and policies | `policy` | `/policy/` | jurisdiction, date | jurisdiction, type, status (computed), topic |
| Standards | `standard` | `/standards/` | body | body, status, topic |
| People | `person` | `/people/` | name | area, era, roles, awards |
| Organizations | `organization` | `/orgs/` | name | kind, country, area |
| Events | `event` | `/timeline/` | date | type, area |
| Glossary | `definition`, and the lead sentence of every concept | `/glossary/` | alphabetical | area, level |

## 5.5 Facets

Every list page (area, task, catalogue, search results) can filter and sort on these facets.
They come from the front matter, the cards and the build, never from a separate index.

| Facet | Values | From |
|---|---|---|
| kind | the 28 kinds of 04.2 | `kind` |
| group, area | the tree | `area`, `areas` |
| task | task slugs | `solves`, `task` |
| modality | the values of 5.3 | `modality`, `inputs`, `outputs` |
| paradigm | symbolic, statistical, connectionist, probabilistic, evolutionary, hybrid | `topics/paradigms.yaml`, set per area and overridden per node |
| learning signal | supervised, unsupervised, self-supervised, reinforcement, preference, none | front matter |
| level | intro, core, advanced, research | `level` (5.6) |
| era | decade of `history.year`, and the named eras of `history-of-ai` | `history` |
| evidence | proved (F2+ or derivation), measured here (R2+), replicated, reported, contested | the claim labels of 07, at the node's main claim |
| implementation | frameworks with X2 or better | build |
| scale | parameters and training compute of models, in log buckets | facts |
| access and licence | open weights, open source, API, closed; SPDX | facts |
| people, organizations | slugs | `people`, `orgs`, `developer` |
| jurisdiction | ISO 3166 codes, `eu`, `intl` | policy nodes and cards |

## 5.6 Levels

| Level | Who it is for | Example |
|---|---|---|
| `intro` | a reader with school mathematics and some programming, in a first AI or ML course | agents, search, linear regression, gradient descent, overfitting, a neural network, a confusion matrix, the Turing test |
| `core` | a student who has finished an undergraduate AI and ML sequence, or a working ML engineer | backpropagation, SVMs, EM, Q-learning, CNNs, the Transformer, BERT, LoRA, BLEU, Bayesian networks, PPO, RLHF |
| `advanced` | a graduate student or specialist | PAC-Bayes, normalizing flows, the neural tangent kernel, ZeRO, FlashAttention, mechanistic circuits, do-calculus identification |
| `research` | a researcher at the edge of a field, or an item less than two years old whose standing is not settled | a method from a recent conference with one replication; an open problem; a new benchmark |

`level` is an editorial field, set by the curate step and checked by the audit (rule T07 samples it).
An item younger than two years starts at `research` unless the canon (5.10) already teaches it.

## 5.7 Prerequisites, paths and courses

- **Prerequisites.** `prereqs` edges form a DAG (rule E03). Each page shows a "Read first" strip of its direct prerequisites, and a full reading path on request, as in algorithm and math2. A core node must have a `prereqs` path back to `ai-foundations`, `math-for-ai` or `deep-learning-foundations` (rule E09). Prerequisites may point to bridge cards (5.9), so a path can cross into math2 for the linear algebra and come back.
- **Order.** As in algorithm 05.4: sub-areas and items are listed in a reading order, from the prereqs DAG first, then by level, then by a hand-set `order` field.
- **Paths.** A path is the topological order of the prerequisites of one target node. The site computes it (16.12). Readers can ask for "from zero to the Transformer" or "from zero to PPO".
- **Courses.** A course is a curated, ordered list of nodes with section headings, in `topics/courses/<slug>.yaml`. The first courses, each cross-checked against public syllabi (names and orders only, 19.3):

| Course | Basis | Size (Estimate) |
|---|---|---|
| Introduction to AI | the chapter order of AIMA 4e, cross-checked with CS2023 AI and with public "Introduction to AI" syllabi | 250 nodes |
| Machine learning | the order of a standard ML course, cross-checked with ESL 2e, Bishop PRML and Murphy PML 1 | 250 nodes |
| Deep learning | the order of Prince UDL and D2L, cross-checked with public deep learning syllabi | 220 nodes |
| Language models from scratch | tokenization to pretraining to post-training to evaluation, with the from-scratch code of 08 at every step | 150 nodes |
| Reinforcement learning | the chapter order of Sutton-Barto 2e, then deep RL | 160 nodes |
| Computer vision | Szeliski 2e and deep vision | 180 nodes |
| NLP | SLP3 chapter order | 180 nodes |
| Probabilistic graphical models | Koller-Friedman order | 120 nodes |
| Learning theory | Shalev-Shwartz and Ben-David, then Mohri et al. | 100 nodes |
| AI safety and alignment | the order of Hendrycks's textbook, then the alignment sub-areas | 140 nodes |
| AI policy and law | the governance sub-areas, with the provisions that a reader needs | 100 nodes |
| History of AI | the eras of `history-of-ai`, in time order | 120 nodes |
| The proved core | the nodes with F2, in an order for reading the Lean proofs (11) | 80 nodes |

A course file has an `id` (its file name), a `title`, a `basis` (the works and syllabi it follows), and `sections`, each with a `heading` and `items`. An item has a `name`, a `kind` (concept when it is not given) and the `node` slug when the node exists. Before the node exists the item is a name, so a course can be written before its nodes, and the harvest reads it into the inventory (13.2), where an item that two courses name is frontier core (5.10).

A course page shows progress (stored in the reader's browser only, as in programming Q-15), and each node in a course shows a step strip with the previous and the next node.

## 5.8 Crosswalks

The build keeps crosswalks to the classifications that readers, libraries and other sites use.
Each crosswalk is a register; each row maps one outside id to one or more of our tags, with the kind of match (`exact`, `broader`, `narrower`, `related`), as in programming 5.7.

| Crosswalk | Outside system | Licence of the outside ids | Used for |
|---|---|---|---|
| `crosswalk/acm-ccs-2012.yaml` | ACM CCS 2012, "Computing methodologies › Artificial intelligence" and "› Machine learning", and the AI parts of "Theory of computation" and "Applied computing" | free to use with credit | linking papers to areas; the coverage check of 18.4 |
| `crosswalk/cs2023-ai.yaml` | the CS2023 Artificial Intelligence knowledge area (AI) and its knowledge units, and the AI parts of the SEP and SEC areas | names of a public curriculum, cited | the coverage report and the learning paths |
| `crosswalk/arxiv.yaml` | arXiv categories cs.AI, cs.LG, cs.CL, cs.CV, cs.NE, cs.RO, cs.MA, cs.IR, cs.SD, cs.CY, cs.CR (AI part), stat.ML, eess.AS, eess.IV | arXiv metadata CC0 | the daily job (14.9) and the harvest priority |
| `crosswalk/openalex.yaml` | OpenAlex domain 3, field 17, subfield 1702 (Artificial Intelligence) and its topics, plus the AI-related topics of subfields 1707 (computer vision and pattern recognition), 1710 and others | CC0 | the work universe, the "missing" signal (18.6), citation counts |
| `crosswalk/wikidata.yaml` | Wikidata QIDs for every item that has one | CC0 | `same_as`, people, organizations, models, software |
| `crosswalk/wikipedia.yaml` | English Wikipedia articles in the "Outline of artificial intelligence", "Outline of machine learning", "Glossary of artificial intelligence", "Timeline of artificial intelligence" and the AI and ML category trees | titles only (CC BY-SA for text, which we never copy) | the concept universe (18.3) |
| `crosswalk/hf-tasks.yaml` | the Hugging Face Hub task taxonomy and its pipeline tags | names cited | task cards and the model scope rule |
| `crosswalk/pwc.yaml` | the tasks, methods, datasets and areas of the final Papers with Code data dump | CC BY-SA 4.0 for the dump; we keep ids and names | the task and method universes; the historical results (10.6) |
| `crosswalk/epoch.yaml` | Epoch AI model, hardware and benchmark ids | CC BY 4.0 | model, hardware and result cards |
| `crosswalk/aiid.yaml` | AI Incident Database incident ids | the AIID terms, ids as facts | incident cards |
| `crosswalk/mit-ai-risk.yaml` | the domains and subdomains of the MIT AI Risk Repository | CC BY 4.0 | risk nodes |
| `crosswalk/iso-22989.yaml` | the terms of ISO/IEC 22989 (AI concepts and terminology) | names only; the standard's text is not free (19.3) | the glossary check |
| `crosswalk/nist-ai-100.yaml` | the NIST AI terms and the AI RMF functions and categories | public domain | policy provisions and the glossary |
| `crosswalk/philpapers.yaml` | the PhilPapers categories under "Philosophy of Artificial Intelligence" | names cited | the philosophy sub-areas |
| `crosswalk/algorithm.yaml`, `crosswalk/math2.yaml`, `crosswalk/programming.yaml` | the tags of our neighbours | CC0, our own | the border (5.9) and bridge cards |

Rules:

- The crosswalks drive the "no missing" checks of 18.4. A row of an outside list that maps to no item is a gap, and the universe register (18.3) takes it in with an outcome.
- An outside system's text is never copied into a node. Only ids and names are kept, with the source (19).

## 5.9 The border with algorithm, math2 and programming

The four sites are one library with four wings. The border rules:

| Question | Goes to | Example |
|---|---|---|
| Is it a procedure with an input and output contract whose correctness and cost can be stated and proved without a learning experiment? | algorithm | Lloyd's algorithm, the junction tree algorithm, alpha-beta pruning, A\*, DPLL, CDCL, Viterbi, the simplex method, reverse-mode automatic differentiation, HNSW construction and search |
| Is it judged mainly by its empirical results when it learns from data, or is it a model, a trained artifact, a dataset, a benchmark, an AI system, a risk, a law, or an idea about intelligence? | ai | dropout, Adam as a training method, LoRA, the Transformer, BERT, ImageNet, MMLU, AlphaGo, reward hacking, the EU AI Act, the Chinese room |
| Is it a mathematical object or theorem that AI uses but that does not belong to AI? | math2 | eigendecomposition, the KL divergence as mathematics, Jensen's inequality, Markov chains, convex duality |
| Is it a general language, library or tool, or how programmers work? | programming | Python, NumPy as a general library, Docker, Git, CUDA C++ as a language |

- **Items on two sides get two pages, each about its own side**, and they link each other with `same_as.al`, `same_as.m2` or `same_as.pg`. Examples: "k-means" on algorithm (Lloyd's algorithm, its convergence, its cost, k-means++ seeding and its O(log k) guarantee) and "k-means clustering" here (when it is used, how to choose k in practice, its empirical record, its role in vector quantization and in tokenizers); "backpropagation" on algorithm (reverse-mode differentiation of a computation graph, its cost) and here (its history from Linnainmaa to Rumelhart, Hinton and Williams, vanishing gradients, its role in training, its biological plausibility debate); "attention" here only.
- **The test for a second page** is that the AI view adds at least three facts with sources that the algorithm page does not have and should not have (rule T10). Otherwise the item is a bridge card (04.1), and the universe register records the outcome `elsewhere:algorithm` with the target tag.
- **Algorithm's existing ML, search and logic nodes.** Algorithm has 419 nodes in `machine-learning`, 209 in `games-and-search` and 325 in `logic-and-constraints` (01.6). At M0, `aik bridge sync al` makes a bridge card for each of them from algorithm's `.json` twins, with its tag, slug, name, summary and URL. The curate step then decides for each one whether this site needs its own node (by the T10 test). The Estimate is that about 250 of the 419 ML nodes get a second page here, and that most of the search and logic nodes stay bridge cards.
- **Optimizers.** SGD, momentum, Adam and their variants are on both sides. Algorithm owns them as optimization algorithms with convergence analysis. This site owns them as training methods: defaults, interactions with normalization and batch size, the empirical record, the variants that the literature compares. The two pages link each other.
- **AI software.** PyTorch, JAX, TensorFlow, scikit-learn, Hugging Face Transformers, vLLM, llama.cpp, DeepSpeed, LangChain and other AI software belong here (`ai-software`). Programming keeps a card for each with `same_as.ai`. NumPy and pandas belong to programming, and this site keeps bridge cards.
- **Mathematics.** `math-for-ai` holds bridge cards to math2 for the mathematics, and nodes only for the AI-specific views that math2 does not have (the reparameterization trick, the log-sum-exp trick in practice, matrix calculus conventions of ML).
- **Bridge cards are refreshed** at each release of the neighbour (20.6), and a neighbour's rename becomes an alias here.
- **The scope of the bridge cards.** `aik bridge sync` reads the index `/api/nodes.json` of the neighbour. The key `bridge` of `crosswalk/<site>.yaml` selects the items, by `areas`, `msc` (the primary MSC class), `kinds` and `tags`. When the key does not exist, the defaults are: on algorithm, the areas `machine-learning`, `games-and-search` and `logic-and-constraints` (about 950 items); on math2, the concepts, definitions, statements and objects whose primary MSC class starts with 15A, 60, 62, 68T, 90C or 94A (about 1,900 items); on programming, all items (about 460). A card that a sync made stays in later syncs, also when its item leaves the scope, so that a link to it does not break.
- **Bridge cards in fields and in prose.** An edge field names a bridge card as `slug@al`, `slug@m2` or `slug@pg`, and prose links it as `[[al:slug]]`. Rule E01 blocks a target that has no bridge card. Rule L02 levels a link to an old slug and a link to a retired card, which is a `broken-bridge` gap (18.7). The site shows the link with the mark "on algorithm.tamnd.com".

## 5.10 The core list

The core is the set of items that must be complete before the site is called "complete for the core". It answers "what does every student and practitioner of AI need?" It is the measure for M2 and M3 (21).
The rule is the one of algorithm: **an item is core when at least two canon works name it** (the canon match step, 14.3).

The canon for the core rule (13.3):

| Work | Covers |
|---|---|
| Russell and Norvig, *Artificial Intelligence: A Modern Approach*, 4th ed. (AIMA 4e) | the whole field, symbolic and statistical |
| Goodfellow, Bengio and Courville, *Deep Learning* | deep learning foundations |
| Bishop, *Pattern Recognition and Machine Learning* | probabilistic ML |
| Bishop and Bishop, *Deep Learning: Foundations and Concepts* (2024) | modern deep learning |
| Murphy, *Probabilistic Machine Learning: An Introduction* and *Advanced Topics* | ML, both volumes |
| Sutton and Barto, *Reinforcement Learning: An Introduction*, 2nd ed. | RL |
| Hastie, Tibshirani and Friedman, *The Elements of Statistical Learning*, 2nd ed. | statistical learning |
| Shalev-Shwartz and Ben-David, *Understanding Machine Learning* | learning theory |
| Mohri, Rostamizadeh and Talwalkar, *Foundations of Machine Learning*, 2nd ed. | learning theory |
| Prince, *Understanding Deep Learning* | deep learning |
| Zhang, Lipton, Li and Smola, *Dive into Deep Learning* | deep learning with code |
| Jurafsky and Martin, *Speech and Language Processing*, 3rd ed. draft | NLP and speech |
| Szeliski, *Computer Vision: Algorithms and Applications*, 2nd ed. | vision |
| Koller and Friedman, *Probabilistic Graphical Models* | graphical models |
| MacKay, *Information Theory, Inference, and Learning Algorithms* | inference and information |
| Thrun, Burgard and Fox, *Probabilistic Robotics* | robotics |
| Pearl, *Causality*, 2nd ed. | causal inference |
| Ghallab, Nau and Traverso, *Automated Planning and Acting* | planning |
| Hendrycks, *Introduction to AI Safety, Ethics, and Society* | safety, ethics, governance |

Rules:

- The canon is a register (`registers/canon.yaml`) with each work's table of contents and index terms, entered by hand or extracted from public tables of contents (names only, 19.3). It changes by pull request with a reason.
- The canon covers the stable core. It does not cover the last two years. So the core list has a second part, the **frontier core**: items that the AI Index of the current year, at least two of the survey works of `registers/surveys.yaml`, or at least two of the courses of 5.7 name. The frontier core is recomputed each year, and the page of an item shows why it is core.
- Core sizes (Estimate): about 2,400 concepts, methods, architectures, theorems and tasks from the canon; about 600 from the frontier core; about 300 notable models, 200 benchmarks and 150 datasets by the scope rules of 18.5; the core policies (the EU AI Act and the Digital Omnibus on AI with their main provisions, the NIST AI RMF, the main national acts and executive acts, the Council of Europe convention, the main frontier commitments); the 80 people and 60 organizations that the canon names most often. The total is about 4,000 items.
- The core is computed and stored in `registers/core.yaml` with the reason for each item, as in algorithm. A person approves it at the end of M1.

## 5.11 Names and disambiguation

- One English `name`. Shared names get qualified slugs and a disambiguation entry (04.3).
- Eponyms use a hyphen in this project's style (no en dash, 06.2): "Bellman equation", "Kullback-Leibler divergence", "Nadaraya-Watson estimator". Slugs drop accents and use hyphens.
- Acronyms are aliases, and a search for one goes straight to its page when it is unique (`rlhf`, `dpo`, `vae`, `gan`, `ppo`, `mcts`, `rag`). When an acronym is ambiguous (`ppo`, `sae`, `moe` are unique; `gp`, `rl`, `ner`, `lm` are mostly unique; `ica`, `pca`, `svm` are unique; `ssm`, `ar` are not), search shows the candidates.
- Model names keep the developer's spelling and capitals (GPT-4o, Llama 3.1, Gemini 2.5 Pro, DeepSeek-R1, Qwen2.5), and the slug is the lower-case form with hyphens (`gpt-4o`, `llama-3-1`, `gemini-2-5-pro`, `deepseek-r1`, `qwen2-5`).
- Paper titles are the titles of the works, and slugs are short forms with the first author and year when the title is generic (`vaswani-2017-attention`, with the alias "Attention Is All You Need").
