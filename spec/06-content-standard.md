# 06. The content standard

This document says what a page contains and how its text is written.
Document 04 says where the files are and what the front matter holds.
Document 07 says how the claims of a page are typed and how their evidence is computed.
Documents 08 to 12 cover code, experiments, results, Lean and scenes.
The rules named here are the S family, listed in 6.13 and in the catalogue of 15.3.

## 6.1 Principles

1. **Intuition, then precision, then evidence.** Each page starts with what a reader can grasp in one minute. Then it gives the exact statement: the equations, the procedure, the definitions. Then it gives the evidence: derivations, code, experiments, results and sources.
2. **One idea, one page, one order.** Each kind has a fixed order of sections (6.3). A reader who has read one method page knows where to look on every other one. This is algorithm's rule, unchanged.
3. **Every claim is checkable.** A sentence that states a fact about AI is a claim with a type and sources (07). A derivation shows every step (6.5). A number comes from a fact or a result card with a date (6.7).
4. **The page reports, the build judges.** The prose never says how strong the evidence is ("it is well established", "proven to work", "state of the art"). The build computes the evidence label of each claim and shows it next to the claim (07.4). The prose never says what is best or newest; computed, dated tables do (6.7).
5. **Neutral where the field disagrees.** On contested questions, the page gives every serious position in the same form, with its best sources, and takes no side (6.9, rule N01).
6. **Our own words.** The text is written fresh. It cites its sources and never copies them (13.7, rule C01). An abstract, a model card or a law is summarized, never pasted.
7. **Simple English.** Short sentences, common words, active voice. A term is defined or linked the first time a page uses it. The style follows ASD-STE100 in spirit, as in algorithm's writer prompt.
8. **The machine is not a person.** The site's voice does not say that a model "knows", "understands", "wants", "believes" or "thinks", except inside a quote or as a defined technical term with a link (rule S14). Standard field terms (learning, attention, memory, hallucination, reasoning model) are allowed, because each has a definition page.

## 6.2 Style and markup

### Style

- **Plain English.** At most 25 words per sentence as a target; rule S06 warns above 30. Active voice. One idea per sentence.
- **One sentence per line** in the Markdown source of `node.md`, derivations and card summaries, as in math2 and algorithm (rule S02). A paragraph is a group of lines with no blank line between them. `aik fmt` splits lines that hold two sentences.
- **No tab character, no em dash, no en dash** anywhere in content (rule S01). Ranges are written "2012 to 2017" or "0.1 to 0.5". Eponyms use a hyphen: "Kullback-Leibler", "Nadaraya-Watson" (5.11). The audit gate of algorithm-work checked exactly this, and it stays.
- **In plain words.** Each node kind that has this section (6.3) explains the idea without notation, as if to a friend who knows only the prerequisites. At most 150 words; no formula longer than one term; no symbol that the prerequisites do not define (rule S05). For dropout: "While the network trains, switch off a random part of its units at each step. The network cannot count on any one unit, so it learns features that work even when some are missing. At test time all units are on, scaled so that the total signal is the same as in training. The effect is close to averaging many smaller networks that share their weights."
- **Acronyms** are expanded on first use on each page, with the acronym in parentheses: "reinforcement learning from human feedback (RLHF)" (rule S13).
- **Banned words.** `registers/banned-words.yaml` holds the filler and hype words that the site voice may not use: "revolutionary", "groundbreaking", "game-changing", "magic", "powerful", "simple", "easy", "just", "obviously", "clearly", "of course", "note that", "it is worth noting", "delve", "leverage", "robust" (outside its technical sense in robustness pages), "seamless", "cutting-edge", "unlock", "harness", "paradigm shift" (rule S03). Time words that rot are a separate list (6.7, rules K04 and S07).
- **Numbers.** Large counts use the SI word or scientific notation with the unit: "405 billion parameters", "3.8 × 10^25 FLOP" rendered from the fact (6.7). The prose never retypes a number that a fact or a result card holds.

### Markup

The body of `node.md` and of each derivation is Markdown read by goldmark, with these additions. The `markup` package (03.4) renders them.

| Markup | Form | Renders as |
|---|---|---|
| inline math | `$\nabla_\theta \mathcal{L}$` | KaTeX at build time, no script on the page |
| display math | `$$ \mathrm{Attention}(Q, K, V) = \mathrm{softmax}\left(\frac{QK^\top}{\sqrt{d_k}}\right) V $$ {#eq-sdpa}` | a centred block with an equation anchor |
| cross-link | `[[dropout]]`, `[[dropout\|this method]]`, `[[transformer#eq-sdpa]]` | a link to the node or to a part; a missing target is red, as in math2 |
| neighbour link | `[[al:k-means-clustering]]`, `[[m2:kl-divergence]]`, `[[pg:numpy]]` | a link to the neighbour, through its bridge card (5.9) |
| claim anchor | a sentence ending in `{#c-inverted-scaling}` | the claim with its evidence chip (07.3) |
| claim reference | `::claim{ref=dropout#c-inverted-scaling}` | the claim of another page, with its chip and a link |
| fact | `{fact:llama-3-1-405b/params.total}` | the value, its unit, a source footnote and "as of" when the fact is time-bound (6.7) |
| computed table | `::table{results bench=mmlu/original setting=5-shot limit=10}` | a results table from the register with its date (10.7) |
| citation | `[^srivastava-2014]` with the locator in `sources` | a footnote with the locator |
| equation citation | `{eq:vaswani-2017-attention#1}` | "Eq. 1 of Vaswani et al. (2017)", linked |
| callout | `> [!pitfall]`, `> [!note]`, `> [!history]`, `> [!warning]`, `> [!scope]` | a tinted box with an icon and a label |
| figure | `![caption](figures/residual-block.svg)` | a figure with caption and text alternative (12) |
| scene embed | `::scene{preset=attention-heads}` | the player of the node's scene (12) |
| notebook embed | `::notebook{impl=numpy cell=train}` | an in-browser cell over the node's code (12.8) |
| spoiler | `<details>` | a closed block, for exercise hints |

## 6.3 The sections of each kind

The headings are fixed `##` headings.
`aik new` writes them, `aik fmt` keeps their order, and rule S04 checks the names and the order.
A section that does not apply is left out, never left empty.

In each list: **bold** sections are required at W2; normal sections are required at W3 when the item has the facts for them (the auditor judges "has the facts"); *italic* sections are optional.
The first bold section after "In plain words" is the **main section** of the kind, required at W1 (04.10).
"(generated)" marks a section that the build writes from front matter, cards, registers or other files; the writer writes no text in it except, where the list says so, a short note.
The renderer adds the title, the summary, the badges and the facts box from the front matter; the body does not repeat them.

Every node page starts with the **lead**: one paragraph with no heading that says what the item is, what it is for, and why it matters, in at most 80 words.

### Ideas

**area**

1. **Scope**: what the area holds and what it does not, with the border (5.9) and links to the neighbour areas.
2. **Map** (generated): the sub-areas, or the nodes of a sub-area, in reading order (5.7), with level and kind.
3. **Key ideas**: 5 to 12 bullets, each one sentence with a link, in reading order.
4. History: the main steps, each with a source, at most 300 words; the history pages hold the rest.
5. Open problems (generated): the open-problem nodes of the area.
6. Courses and paths (generated).
7. **References** (generated).

**concept**

1. In plain words.
2. **The idea**: what it is and why the field needs it, in 1 to 4 paragraphs with the smallest example.
3. Formal statement: the definition in the notation of 6.4, when the concept has one.
4. **Examples**: at least two, one of them a non-example or a borderline case.
5. Where it is used (generated, with a note): the methods, architectures and findings that use it.
6. Related ideas: each with one sentence on the difference.
7. Common confusions: the misreadings that the canon or the literature names, with sources.
8. *Formal proof* (generated, 11).
9. History: who named it, when, where, with sources; competing uses of the term.
10. **References** (generated).

**definition**

1. **Definition**: exact, in words and, when it has one, in the notation of 6.4.
2. **Examples and non-examples**.
3. Usage: how different communities use the term, each with a source (for "hallucination": NLG, LLM evaluation, vision).
4. Related terms (generated from edges, with a note).
5. **References** (generated).

**method**

1. In plain words.
2. **Setting**: the problem it addresses, the assumptions, the models and tasks it applies to.
3. **The method**: the equations and the procedure, step by step, in the notation of 6.4. Pseudocode only when the procedure has more than one loop; the code is in Implementations.
4. **Derivation**: the main derivation inline when it is short, otherwise its statement and a link to `derivations/` (6.5).
5. **Worked example**: numbers small enough to follow by hand, matching a fixture of the NumPy reference (6.6).
6. **Hyperparameters**: each one, what it controls, the range in practice with sources, and the defaults of the main frameworks (from the front matter).
7. **Cost**: the extra compute and memory at training and at inference, with the variables named.
8. **Evidence** (generated, with a note): the claims of `claims.yaml` with their labels (07.12), the experiments of the node (09) and the results of methods compared (10). The writer writes at most 150 words of context, and every sentence of it is a claim.
9. Limitations and failure modes: when it does not help, when it hurts, its interactions with other methods (dropout with batch normalization), each with sources.
10. Variants: named variants, each with one or two sentences and a link when it has a page.
11. When to use it: the trade-offs against the alternatives for the same purpose, linked to the comparison page of the purpose (16.5).
12. Pitfalls: implementation bugs that the literature or the frameworks' issue trackers name, each with the test that catches it (rule S12).
13. Implementations (generated): the framework tabs (08); the writer writes only what is special.
14. Experiments (generated, 09).
15. *Formal proof* (generated, 11).
16. History: the first publication, the canonical one, the people, with sources; independent discoveries.
17. **References** (generated).

**architecture**

1. In plain words.
2. **Overview**: the structure in one paragraph and one figure or scene.
3. **Components**: each block with its equations, in data-flow order.
4. **Shapes and parameters** (generated table from `shapes.yaml`, with a note): the tensor shapes and the parameter count formula, with a link to its derivation (08.6).
5. **Computation and cost**: FLOPs per token or per example, memory, and the cost in the input size (sequence length, image size), derived.
6. Training: how it is trained in practice: objectives, initialization, normalization placement, typical recipes, each with sources.
7. Inductive biases: what the design assumes about the data and why.
8. **Evidence** (generated, with a note), as for methods.
9. Limitations: the known weaknesses with sources.
10. Variants and descendants (generated lineage from `variant_of` and the reverse edges, with a note).
11. Notable models (generated from models whose `architecture` is this node).
12. Implementations (generated).
13. Experiments (generated).
14. History.
15. **References** (generated).

**theorem**

1. **Statement**: exact, with all hypotheses, in the notation of 6.4.
2. In plain words.
3. **Proof**: inline when short, otherwise a link to `derivations/proof.md`; it meets 6.5.
4. Assumptions: what each hypothesis does, and what fails without it (an example).
5. Tightness: an example that shows that the bound cannot improve, or a note that the tight form is open.
6. Consequences (generated from `proves` and reverse `uses`, with a note).
7. *Illustration*: an experiment or a scene that shows the bound against measured values (09, 12).
8. Formal proof (generated, 11).
9. History.
10. **References** (generated).

**finding**

1. In plain words.
2. **The finding**: one precise sentence with its scope (the models, data, scales and settings where it was observed), then the measured form (a law, a curve, a threshold), with its source.
3. **Evidence** (generated, with a note): the first report, the replications, the failures to replicate, each with its label and its role (07.4).
4. **Scope and conditions**: where it holds, where it fails, and how the measurement choices change it (metric choice for emergent abilities; tokenizer and data for scaling exponents).
5. Explanations: the competing explanations, each in the form of 6.9, with sources and with the claims that test them.
6. Reproduction here (generated from the experiments, with the scope sentence of 07.8).
7. Implications: as reported by named sources, never in the site's voice.
8. Open questions (generated from open-problem nodes, with a note).
9. History.
10. **References** (generated).

**argument**

1. In plain words.
2. **The argument**: the premises and the conclusion, numbered, in the strongest form that its proponents give, with sources.
3. **Context**: what question it addresses and what it responds to.
4. **Replies**: each named reply (for the Chinese room: the systems reply, the robot reply, the brain simulator reply, ...), with its proponents, its form, and the main counter-replies, each with sources. Replies are listed in the order of their standing in the literature (6.9).
5. Influence: how the argument shaped research, with sources.
6. Related arguments (generated, with a note).
7. **References** (generated).

**open-problem**

1. **The question**: one exact question, and the terms defined.
2. **Why it matters**: what depends on the answer, as named sources say.
3. **Positions**: each position in the symmetric form of 6.9.
4. **Evidence so far** (generated, with a note): the claims that bear on the question, with their labels.
5. What would settle it: results or observations that the positions agree would decide it, with sources; when they do not agree on this, say so.
6. History of the debate.
7. **References** (generated).

**risk**

1. In plain words.
2. **Definition and scope**: what the risk is, what is inside and outside it, and the competing definitions in the source taxonomies.
3. **How it arises**: the mechanism, with an example.
4. **Examples**: documented demonstrations and incidents (generated list of `instance_of` incidents, plus the writer's selection with sources).
5. **Evidence** (generated, with a note): how often, how severe, at which scales, with labels.
6. **Mitigations**: each method or policy that targets it, with the evidence label of its effect.
7. Taxonomies (generated from the crosswalks: MIT AI Risk Repository, NIST AI 600-1, OWASP Top 10 for LLM Applications, MITRE ATLAS, the EU AI Act).
8. Debates: positions on its likelihood or severity, in the form of 6.9.
9. **References** (generated).

### Artifacts

Artifact pages start with a **facts box** (generated from facts, 04.9), with a source chip for each value and "as of" dates.
The text never repeats a number of the facts box in prose without the `{fact:}` markup (rule D01).

**model** (family or notable release)

1. **Overview**: what it is, who made it, when, and why it matters, with sources.
2. **Architecture**: the base architecture (linked) and every documented difference from it.
3. **Training data**: what the developer documents, with the share of each source when known; what is not documented is said to be undocumented.
4. **Training**: compute, hardware, recipe, post-training methods, each from facts or with sources.
5. Releases and versions (generated from `releases.yaml` and the cards).
6. **Results** (generated from result cards, with a note): computed tables of the main benchmarks, each with its setting and its source kind (self-reported by the developer, reported by a third party, measured here).
7. Access and licence: how it can be used, the licence and its main terms, with the dated history of changes.
8. Safety and evaluations: what the developer's system card and third-party evaluations report, attributed.
9. Reception and impact: attributed, with sources.
10. Known issues (generated from incidents and risks, with a note).
11. Lineage (generated from `fine_tuned_from`, `distilled_from`, `merged_from`, `member_of`, `succeeded_by`).
12. **References** (generated).

**task**

1. In plain words.
2. **Definition**: the input, the output and what makes an output right, exactly.
3. Contract (generated box from `contract.yaml`, 10.2).
4. **Metrics** (generated from `measured_by`, with a note on which metric is used when).
5. **Benchmarks** (generated, with a note on what each one tests).
6. **Approaches**: the families of methods in historical order, each with a link and a sentence, ending with the families that the results register lists.
7. Results (generated): the computed, dated table of the main benchmark (10.7).
8. Difficulties: ambiguity, annotation disagreement, known shortcuts.
9. Subtasks and related tasks (generated, with a note).
10. Applications.
11. History.
12. **References** (generated).

**benchmark**

1. **What it measures**: the task, the abilities claimed by its authors (attributed), and what it does not measure.
2. **Data**: construction, size, splits, sources, licence, from facts.
3. **Protocol**: shots, prompt format, decoding, scoring, answer extraction; what settings are in use and how they change scores.
4. Versions (generated).
5. **Results** (generated per version, with dates).
6. Known issues: label errors, contamination reports, shortcuts, saturation (computed, 10.8), each with sources.
7. Reception: attributed.
8. History.
9. **References** (generated).

**dataset**

1. **Contents**: what is in it, by modality and source, from facts.
2. **Collection and construction**: how it was made: crawling, filtering, annotation, synthesis; who did the work when documented.
3. **Documentation and licence**: the datasheet or data card, the licence, the terms, the consent and opt-out mechanisms.
4. Uses (generated from `trained_on` and benchmark edges).
5. Known issues: personal data, harmful content, copyright claims, takedowns, retractions, biases, each attributed with sources.
6. Versions and derivatives (generated from `derived_from`).
7. History.
8. **References** (generated).

**metric**

1. In plain words.
2. **Definition**: the formula in the notation of 6.4, with every term defined.
3. **Properties**: range, direction, invariances, decomposability, with derivations.
4. **Worked example**: by hand, equal to a row of `vectors/official.jsonl`.
5. **Reference implementation**: the official implementation and version that the test vectors come from (08); the known differences between popular implementations (tokenization in BLEU, interpolation in mAP).
6. Interpretation: what a value means, what differences are meaningful, comparability across settings.
7. Limitations and criticism, with sources.
8. Variants.
9. Implementations (generated).
10. History.
11. **References** (generated).

**system**

1. **Overview**: what it was or is, who built it, when, in which domain.
2. **How it works**: its parts (models, search, rules, hardware), each linked.
3. Development: the timeline with sources.
4. Results and milestones: dated, with sources and links to events.
5. Impact and reception: attributed.
6. Limitations and incidents.
7. Legacy (generated successors, with a note).
8. **References** (generated).

**software**

1. **What it is for**: its purpose and its users.
2. **Core concepts**: its main abstractions, with the vocabulary that its documentation uses.
3. **Example**: one small program that runs in the software's pinned image (08), with its output from the runner.
4. Ecosystem and alternatives: a comparison with the same category, each difference with a source.
5. History: releases that changed the field, from facts.
6. **References** (generated).

**hardware**

1. **Architecture**: what in the design matters for AI workloads, with sources.
2. **Use in AI** (generated from model facts, with a note): notable training runs that used it.
3. Comparison (generated from facts of the same type).
4. History.
5. **References** (generated).

### Governance

**policy**

1. **What it does**: in plain words, at most 200 words.
2. **Scope and definitions**: who and what it covers, its key definitions in our words with the article numbers.
3. **Main provisions** (generated list of provision cards, with a note per group).
4. **Obligations by actor**: providers, deployers, importers, users, authorities; a table generated from provision cards where it can be.
5. **Timeline** (generated from `dates`): proposal, adoption, entry into force, application of each part, amendments; the status is computed (04.5).
6. Enforcement and penalties.
7. Amendments and implementing acts (generated from `amends` and `amended_by`, with a note).
8. Interpretation and debate: official guidance first, then positions in the form of 6.9.
9. Relation to other laws and standards (generated from edges, with a note).
10. **References** (generated, official text first).

Each policy and provision page carries a generated notice that the page is a summary and not legal advice, with a link to the official text.

**provision** (when promoted): **What it requires** (our words); **Applies to**; **Dates** (generated); Related provisions (generated); Official guidance; **References**.

**standard**: **What it specifies**; Structure (the clause titles only, 19.3); Adoption and status (generated); Relation to policy (generated `implements_policy`, with a note); History; **References**.

**incident** (when promoted): **What happened** (dated, attributed to sources); **Systems and parties** (organizations; people only under 19.6); **Harms**; Response; Risk classes (generated `instance_of`); **References**.

### The record

**work** (canon works and landmark papers)

1. **Contributions**: what the work introduced or showed, in our words, each contribution a claim with a locator in the work.
2. **Context**: the problem and the prior work it answered.
3. Methods and results: the main results as claims linked to result cards where they exist.
4. Reception and influence (generated citation counts with date, follow-up works and evidence edges, with a note).
5. Criticism, replications and corrections (generated from `evidence_against`, errata and retractions, with a note).
6. **References** (generated).

**person**: lead (name, years when public, roles); **Contributions** (generated from edges, with a note in sentences with sources); Career (public roles only, 19.6); Recognition (generated awards); **References**. No In plain words, and no personal life beyond what 19.6 allows.

**organization**: **Overview**; History; Work (generated models, systems, works, policies); People (public roles only); **References**.

**event**: **What happened** (date, place, participants); Outcomes and significance (attributed); **References**.

### Card-only kinds

`result`, `exercise` and, until promoted, `incident`, `work`, `provision` and `event` have no page and no sections.
Their only prose is the card `summary`, at most 30 words, checked by S10.
An exercise card never holds the problem statement of its site, only the link, the metadata and a hint (19.3).

### `registers/purposes.yaml`

The `purpose` field of a method (04.5) takes one value of this register, which drives the "When to use it" comparisons and the facets.

| Purpose | Examples |
|---|---|
| `optimization` | SGD with momentum, Adam, learning-rate warmup, gradient clipping |
| `regularization` | dropout, weight decay, label smoothing, early stopping |
| `normalization` | batch normalization, layer normalization, RMSNorm |
| `initialization` | Xavier initialization, He initialization, muP |
| `data-augmentation` | random crops, mixup, back-translation |
| `representation` | word2vec, contrastive pretraining, masked autoencoding |
| `generation-and-sampling` | classifier-free guidance, nucleus sampling, DDIM sampling |
| `inference-efficiency` | KV caching, speculative decoding, continuous batching |
| `compression` | quantization, pruning, knowledge distillation |
| `adaptation` | fine-tuning, LoRA, prompt tuning, domain-adaptive pretraining |
| `alignment-and-preference` | RLHF, DPO, constitutional training |
| `prompting-and-reasoning` | few-shot prompting, chain of thought, self-consistency |
| `retrieval-and-grounding` | dense retrieval, RAG, reranking |
| `exploration` | epsilon-greedy, UCB in RL, intrinsic motivation |
| `credit-assignment` | TD learning, GAE, eligibility traces |
| `uncertainty` | Monte Carlo dropout, deep ensembles, conformal prediction |
| `interpretability` | integrated gradients, linear probes, sparse autoencoders |
| `robustness-and-security` | adversarial training, randomized smoothing, input filtering |
| `privacy` | DP-SGD, federated averaging, machine unlearning |
| `fairness` | reweighing, equalized-odds post-processing |
| `evaluation` | cross-validation, bootstrap confidence intervals, LLM-as-judge |
| `parallelism-and-scale` | ZeRO, tensor parallelism, activation checkpointing |
| `search-and-planning` | MCTS with learned priors, beam search in decoding |

A new value is a pull request with a reason and at least five methods that need it.

## 6.4 The notation register

AI books disagree on notation, and a site written by many agents would disagree with itself on every page.
So notation is a register, `registers/notation.yaml`, with a global part and per-area overrides, and the `notation` package (03.4) checks every display and inline formula against it.

The global choices (mostly those of Goodfellow et al. and Prince, cited in the register):

| Object | Notation | Notes |
|---|---|---|
| scalar, vector, matrix, tensor | $a$, $\mathbf{a}$, $\mathbf{A}$, $\mathsf{A}$ | vectors are column vectors |
| input, target, prediction | $\mathbf{x}$, $y$ (or $\mathbf{y}$), $\hat{y}$ | |
| dataset, size, example index | $\mathcal{D} = \{(\mathbf{x}_i, y_i)\}_{i=1}^{N}$ | $N$ examples |
| parameters | $\theta$ (all), $\mathbf{W}$, $\mathbf{b}$ (a layer) | |
| model, model distribution | $f_\theta$, $p_\theta$ | |
| data distribution | $p_{\text{data}}$ | |
| per-example loss, total loss | $\ell$, $\mathcal{L}(\theta)$ | $\mathcal{L}$ is calligraphic; $L$ is reserved for layers |
| learning rate, step | $\eta$, $t$ | |
| gradient | $\nabla_\theta \mathcal{L}$ | |
| expectation, probability | $\mathbb{E}$, $\Pr$ | $p(\cdot)$ for densities and mass functions |
| logistic function, softmax | $\sigma(\cdot)$, $\mathrm{softmax}(\cdot)$ | $\sigma$ with no argument is a standard deviation only in statistics pages, by override |
| KL divergence, entropy | $D_{\mathrm{KL}}(p \,\|\, q)$, $H(p)$ | |
| hypothesis class, VC dimension | $\mathcal{H}$, $d_{\mathrm{VC}}$ | |
| batch size, sequence length | $B$, $T$ | |
| model width, heads, head width, layers | $d_{\text{model}}$, $h$, $d_k$ and $d_v$, $L$ | |
| feedforward width, vocabulary | $d_{\text{ff}}$, $\mathcal{V}$ with size $|\mathcal{V}|$ | $V$ is the value function in RL pages |
| tensor shapes in prose | `[B, T, d_model]` | the same order as `shapes.yaml` (08.6) |

Area overrides (`registers/notation/<top>.yaml`):

- **reinforcement-learning**: the conventions of Sutton and Barto 2e. $S_t$, $A_t$, $R_{t+1}$ are random variables and $s$, $a$, $r$ their values; $\pi$, $\gamma$, $G_t$, $v_\pi$ and $q_\pi$ (with $V$ and $Q$ for estimates), $A$ for advantage only in policy-gradient pages, $H$ for the horizon.
- **probabilistic-reasoning** and **probabilistic-machine-learning**: $X$, $Y$ for random variables; $\mathbf{z}$ for latent variables; $q_\phi$ for variational distributions.
- **learning-theory**: $\mathcal{X}$, $\mathcal{Y}$, $\mathcal{H}$, $m$ for the sample size (following Shalev-Shwartz and Ben-David), $\epsilon$, $\delta$.
- **generative-models**: $\mathbf{x}_0$ data and $\mathbf{x}_t$ noised data at time $t$ in diffusion pages; $\boldsymbol{\epsilon}_\theta$ the noise predictor.

Rules:

- A page may define a local symbol in a front-matter block `notation:` with a meaning. A local symbol may not redefine a global one (rule S09).
- Every symbol in a formula is in the global register, the area override, or the local block (rule S09, level W3). The check parses the KaTeX tree, so `\mathbf{x}` and `x` are different symbols.
- When a source uses another notation, the page uses ours and says once, in a `> [!note]`, how the source's symbols map to ours, so that a reader can follow the source.
- Equations that come from a source carry the source's equation number in `{eq:...}`, even when the notation is changed.

## 6.5 Derivations

A derivation is the AI form of algorithm's human proof.
It shows how an equation, a gradient, a bound, a parameter count or a cost follows from definitions and earlier results.
It lives in `derivations/<id>.md` with front matter:

```yaml
---
id: expectation-scaling
derives: [c-inverted-scaling, eq-inverted-dropout]   # claim ids (07.3) and equation ids of node.md
kind: identity            # identity | gradient | bound | count | cost | proof | limit
assumes: ["units are dropped independently with probability p", "0 <= p < 1"]
uses: [bernoulli-distribution@m2, linearity-of-expectation@m2]
status: full              # full | sketch
check: expectation-scaling.check.py                  # optional numeric check
lean: null
sources: [{work: srivastava-2014-dropout-jmlr, locator: "Section 10"}]
---
```

The body has four parts, each a `###` heading:

1. **Statement**: the result, exactly, in the notation of 6.4.
2. **Assumptions**: each one numbered, with what it is used for.
3. **Steps**: numbered steps. Each step is one equation or one line of reasoning, and it says why it holds: a definition, an assumption by number, a cited lemma, or an algebraic rule named in words. "Clearly", "it is easy to see", "by a standard argument" and skipped cases are defects (rule S11).
4. **Result and check**: the result restated, and its check.

The **numeric check** is new compared with algorithm.
Many AI derivations can be tested by numbers: a gradient against finite differences, a parameter count against a built model, an expectation against a Monte Carlo average, a FLOP count against a profiler count.
When `check:` is set, the file is a NumPy script in the `numpy` image (08) that evaluates both sides on random inputs with a fixed seed and asserts agreement within a stated tolerance.
`aik test` runs it, and a derivation whose check fails is not counted as full.
Rule S11 requires a numeric check for derivations of kind `gradient`, `count` and `cost` at W3.

A `sketch` derivation shows the main steps and says what it leaves out; it never counts for the `proved` label (07.4).
A `full` derivation that passed the cross-family audit with no blocker or major issue counts for `proved` on the claims that it derives.
A derivation of kind `proof` for a theorem node meets the same standard as algorithm's human proofs: induction names its hypothesis, every case is covered, inequalities name the rule they use, and constants are tracked or stated as absorbed.

An equation that is **cited, not derived** must name the source and its equation number with `{eq:...}`, and the page says "as given by" the source.
Rule S11 counts the equations of the main section of a method, architecture, metric and theorem: at W3 each one is derived or cited.

## 6.6 Worked examples

Each method, architecture, metric and theorem with a formula has one worked example small enough to follow by hand: a 2 by 3 weight matrix, a sequence of 4 tokens with $d_k = 2$, a batch of 3 predictions.

- The numbers are rational or short decimals, and every intermediate value is shown in a table.
- The same example is a fixture of the NumPy reference (`impl/numpy/test_*.py`, named `worked_example`), so the build checks every number of the page (rule S12, with X rules of 08).
- When the node has a scene, the example is also a preset with the same name, as in algorithm.
- For metrics, the example is a row of `vectors/official.jsonl`, so the official implementation agrees with the page.

## 6.7 Dated facts in prose

AI facts rot faster than any other kind of knowledge on the four sites.
A page written in 2026 that says "the largest open model" is wrong in 2027 without any edit.
So time-bound facts are never written as plain prose.

- **Banned in the site voice** (rule K04, 07.11): "state of the art", "SOTA", "currently", "the best model", "the largest", "the most capable", "leading", "frontier" as a ranking word (it is allowed as the defined term "frontier model" with a link), and any rank or superlative about a model, a lab, a benchmark score or a dataset.
- **Rotting words** (rule S07): "recent", "recently", "new", "latest", "modern", "today", "nowadays", "now", "so far", "to date", "at the time of writing". History sections may say "at the time" about a past date that the sentence names.
- **Facts inline.** A number that a fact holds is written with `{fact:<item>/<field>}`. The renderer prints the value, the unit, the source footnote and, for time-bound facts, "(as of 2026-11-12)" (04.9). A disputed fact prints both values with their sources.
- **Computed statements.** Superlatives and ranks come only from computed facts (`aik facts compute`, 04.9) with the population and the date, written as `{fact:computed/largest-open-weights-dense-by-params}`. They render as a full sentence of the build: "Among the model cards, the largest open-weights dense model by total parameters is X (as of 2026-11-13)."
- **Computed tables.** Comparisons of models on a benchmark come only from `::table{results ...}` (10.7), which prints the benchmark version, the setting, the source kind of each row and the date of computation.
- **Past results are fine.** "AlexNet's top-5 error on the ILSVRC-2012 test set was 15.3%" is a historical claim about a fixed event, with a source; it is not time-bound. "AlexNet won ILSVRC-2012" is a historical claim too. Rule K04 checks time words and superlatives, not past tense.
- **Forecasts.** A prediction is a claim of type `forecast` (07.2): it is attributed, dated, and never stated in the site voice. "Bostrom (2014) argues that ..." is allowed; "AGI will arrive by ..." is not.

## 6.8 Attribution, voice and words

- **Self-reported results.** A number from a developer about its own model is attributed: "the developer reports" or the developer's name, and the result card has `self_reported: true` (10.6). The page never states a developer's capability claim as fact.
- **Reporting verbs.** "shows" and "proves" are used only when the claim's computed label is `proved` or the sentence describes a proof. For empirical work the verbs are "reports", "finds", "measures", "observes", "argues". The audit checks this against the labels (rule K05).
- **Names.** Model, organization and product names are spelled as their owners spell them (5.11). People are named by full name on first use and by surname after.
- **Anthropomorphism.** As in 6.1, principle 8 (rule S14).
- **Hedging.** No vague hedges ("some believe", "it is often said", "many experts"). A view has a named holder and a source, or it is not on the page (rule N02, 07.11).

## 6.9 Contested topics

Some questions are open in the field and in public debate: whether emergent abilities are real or a measurement effect, whether scaling alone leads to general intelligence, how likely catastrophic risks from AI are, whether large language models understand language, whether machines can be conscious, how copyright applies to training, whether open weights are safer or more dangerous.
A page on such a question is written in this form, and rule N01 forbids the site voice from taking a side.

1. **The question first.** The page states the question exactly and defines its terms, because many debates are about different questions under one name. When the parties disagree on the definition, the page gives each definition.
2. **Every serious position.** A position is serious when it is held in at least three sources by distinct authors of the standing classes of 07.4 (peer-reviewed, preprint by researchers of the area, official, or a book from an academic publisher), or by one named official body; editors can add a position with a recorded reason (Default, Q-30).
3. **The same form for each position.** Each position has the same parts, in the same order: the claim; the main argument, in the strongest form its proponents give (a steelman, in their terms, with sources by proponents, rule N03); the best evidence for it, as claims with labels; the main objections, with sources by the objectors. No position gets an adjective that another lacks (rule N05).
4. **Order by evidence, not by preference.** Positions are listed in the order of the number of independent sources that hold them, computed from the evidence edges, and the order rule is printed on the page.
5. **Expert surveys are data.** A survey of researchers is reported with its sample, its response rate, its question wording and its date, as a claim of type `forecast` or `empirical` (rule N06).
6. **No loaded labels.** Words that one side uses for the other ("doomer", "hype", "denier", "accelerationist" as an insult, "snake oil") appear only inside quotes or as the name of a self-described movement with a source (rule N04).
7. **The evidence section is shared.** Claims that bear on the question are listed once with their labels, and each position links the ones it relies on, so a reader sees the same evidence under every position.

These pages use the contested-topic layout (16.8). They are sampled for human review at a higher rate (15.6), and a reviewer who is a proponent of one position on record must disclose it and cannot be the only reviewer (15.5, Q-19).

## 6.10 The writing ladder W0 to W5

The writing level says how complete the text of a page is.
It does not measure truth; the G, X, R, F, V and D ladders and the claim labels do.
The build computes it (04.10), with the thresholds in `schema/levels.v1.yaml`.

| Level | Name | Needs |
|---|---|---|
| W0 | registered | a card, or a node directory with valid front matter; no page is published, and the card shows on its collection page (04.1) |
| W1 | page | the lead, In plain words (for kinds that have it), and the main section of the kind (6.3); the S rules pass with no block; at least one source with a locator; the page is published |
| W2 | draft | every bold section of the kind; every link resolves (L01); G1 |
| W3 | standard | W2, and every normal section for which the item has the facts; `claims.yaml` extracted and every claim grounded (G2, 07.5); every equation of the main section derived or cited, with the derivations complete (6.5); the worked example matches its fixture; the notation check passes (S09); all prerequisites linked |
| W4 | complete | W3, and the cross-family audit passed with no blocker or major issue for this hash (15.4); the canon coverage check found no missing subtopic (15.4); for `intro` and `core` nodes at least one exercise or course link |
| W5 | reviewed | W4, and a reviewer of the node's subject, who is not an author of any reviewed hash, reviewed the text of this exact hash (15.5) |

For artifact kinds (model, benchmark, dataset, hardware, policy, system, software, standard), W3 also needs D1 and W4 also needs D2 (04.10).
For contested pages (open-problem, argument, and any page with a position block), W4 also needs the neutrality check of the audit to pass (15.4).
Each page says what it needs for the next level, in its evidence bar and its "History of this page" section (16.4), as in math2.

## 6.11 Length

| Kind | Target words at W3, excluding code, tables and generated sections | Hard limit |
|---|---|---|
| area | 300 to 900 | 2,000 |
| concept | 600 to 2,000 | 6,000 |
| definition | 50 to 400 | 1,000 |
| method, architecture | 1,200 to 3,500 | 8,000 |
| metric | 600 to 1,800 | 4,000 |
| theorem | 400 to 1,500, plus the proof | 4,000 plus the proof |
| finding | 800 to 2,500 | 6,000 |
| argument, open-problem | 1,000 to 3,500 | 8,000 |
| risk | 800 to 2,500 | 6,000 |
| model, benchmark, dataset, system | 500 to 2,000 | 5,000 |
| task | 400 to 1,500 | 4,000 |
| software, hardware, standard | 300 to 1,200 | 3,000 |
| policy | 800 to 3,000 | 8,000; provisions hold the rest |
| work | 300 to 1,200 | 3,000 |
| person, organization, event | 150 to 800 | 2,000 |

A page over its hard limit fails rule S08. The fix is to split it into nodes, not to cut facts.

## 6.12 The gold pages

algorithm-work's writer prompt told every writer to read the gold pages (merge sort and a few others) before it wrote, and the audit compared each page with them.
That was the strongest quality lever of algorithm, and it is copied.

At M0, people write the gold set, at least one gold page for each main kind and 25 pages in all (an exit criterion of M0, 21), by hand or by agents under line-by-line human editing, to W5 and to the top of every other ladder that applies.
They are the model of style and depth for the writer and the bar for the auditor (14.6, 15.4).

| Kind | Gold page | What it tests |
|---|---|---|
| method | `dropout`, `lora` | derivation with numeric check, three frameworks, an experiment at R3, evidence and limitations (interaction with batch normalization) |
| architecture | `transformer`, `resnet` | components, `shapes.yaml`, parameter and FLOP derivations, a scene, a tiny experiment, lineage |
| metric | `bleu` | formula, brevity penalty derivation, sacreBLEU test vectors, interpretation and criticism |
| concept and theorem | `pac-learning` and `pac-bound-finite-class` | definitions, a full proof, a Lean proof at F2 |
| finding | `double-descent`, `neural-scaling-laws` | scope, replications, competing measurements, dated results |
| argument | `chinese-room` | the argument, the named replies, neutrality |
| open-problem | `does-scaling-lead-to-general-intelligence` | symmetric positions, shared evidence |
| risk | `reward-hacking` | mechanism, documented examples, mitigations with labels, taxonomies |
| model | `llama` (family), `bert` | facts with sources, releases, lineage, results tables, licence history |
| dataset and benchmark | `imagenet` and `imagenet-ilsvrc-2012` | facts, construction, known issues, versions, results by setting |
| task | `image-classification` | contract, metrics, approaches, dated table |
| policy | `eu-ai-act` | provisions, obligations, a timeline that includes Regulation (EU) 2026/1744, official text first |
| system | `alphago` | parts, milestones, legacy |
| software and hardware | `pytorch` and `nvidia-h100` | facts, example that runs, comparison |
| work | `vaswani-2017-attention` | contributions as claims, influence |
| person, organization, event | `alan-turing`, `deepmind`, `dartmouth-workshop` | public facts only, sources |

The gold list is `registers/gold.yaml`.
A gold page is re-reviewed at each milestone, and a change to it needs a reviewer's record.

## 6.13 The rules of this document

These rules are listed in 15.3 with their severities.

| Rule | Checks | Severity |
|---|---|---|
| S01 | no tab, em dash or en dash in content files | block |
| S02 | one sentence per line in Markdown source | warn; `aik fmt` fixes it |
| S03 | no word of `registers/banned-words.yaml` in the site voice | level (W1) |
| S04 | the section headings of the kind, in order, with no empty section | level (W1) |
| S05 | In plain words: at most 150 words, no formula longer than one term, no undefined symbol | level (W1) |
| S06 | sentence length: warn above 30 words | warn |
| S07 | no rotting time word of 6.7 in the site voice | level (W2) |
| S08 | length within the hard limit of 6.11 | level (W3) |
| S09 | every symbol of every formula is in the notation register, the area override or the local block; no local redefinition of a global symbol | level (W3) |
| S10 | a card summary is at most 30 words and states nothing that the card's facts do not support (checked by the verifier, 15.4) | block (new cards); level (D1) |
| S11 | derivations meet 6.5: every step justified, numeric check for gradient, count and cost derivations, every main-section equation derived or cited | level (W3) |
| S12 | the worked example equals its fixture; each pitfall names a test that exists | level (W3) |
| S13 | acronyms expanded on first use on each page | warn |
| S14 | no mental-state verb about a model in the site voice outside a quote or a defined term | level (W2) |
| S15 | the lead is at most 80 words and names what the item is | warn |
| S16 | a promoted card-only kind has the sections of 6.3 for it; an unpromoted one has no page | block |
