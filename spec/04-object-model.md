# 04. The object model

This document defines what the content is made of: the kinds of item, the difference between a node and a card, the identifiers, the directory of a node, the front matter, the edges, the fact model, and the ladders that the build computes.
It follows 2150/04 (algorithm) and 2150p/04 (programming) where it can, and says where it differs.

## 4.1 Nodes and cards

AI has far more named things than algorithms: about 3,700 models in Epoch AI's database and more than 3 million on the Hugging Face Hub, about 1,800 incidents in the AI Incident Database, more than 4 million works in the OpenAlex AI subfield, and tens of thousands of benchmark results (01.2).
A written page for each would be thin, would never finish, and would hide the pages that matter.
So an item exists in one of two forms, as in programming.

| Form | What it is | Where it lives | Rendered as | Written by |
|---|---|---|---|---|
| **card** | a row of checked facts with sources: names, dates, numbers, links, a one-sentence summary, the edges | a JSONL register under `cards/<kind>/` | an entry on a collection page, with a stable anchor and its own JSON record | the harvester from structured sources (13), checked by rules; a person or an agent only to fix it |
| **node** | a directory with a written page and, by kind, derivations, implementations, experiments, a Lean part and a scene | `nodes/<top>/<sub>/<slug>/` | a page of its own | writers, people and agents (14) |

Rules:

- Every kind can exist as a card. Most kinds start as cards and are **promoted** to nodes. `aik promote <id>` makes the node directory from the card, moves the facts into the front matter, and removes the row from the register in one commit. The tag does not change.
- A card is never a stub page. A page exists only for a node at W1 or more (06.10). Below that, the reader sees the card on its collection page, with a link "Help write this page" (16.19). This fixes algorithm's mistake of publishing 4,779 stubs as pages (01.6).
- Six kinds are **card-only by default**: `result`, `incident`, `work`, `exercise`, `provision` and `event`. They become nodes only when the editors decide that the item needs prose: a landmark paper such as "Attention Is All You Need", an incident with a long public record, an article of a law that many pages cite.
- A **bridge card** is a card whose item lives on a neighbour site (5.9). It has the neighbour's tag, slug, name and summary, copied by the harvester from the neighbour's `.json` twin, and the AI edges that our nodes need. Search finds it, and collection pages list it with a mark "on algorithm.tamnd.com".
- The coverage levels (18.2) count cards and nodes on one scale. A card is C1. A node starts at C2.

## 4.2 The kinds

| Kind | It is | Example | Default form | Has derivation | Has impl | Has experiment | Has Lean | Has scene |
|---|---|---|---|---|---|---|---|---|
| `area` | a node of the topic tree (05) | Reinforcement learning; RL › Policy gradient methods | node | no | no | no | no | optional |
| `concept` | an idea that AI uses to think about learning, reasoning or agents | overfitting, inductive bias, attention, exploration versus exploitation, emergence, alignment | node | optional | optional | optional | optional | optional |
| `definition` | a precise term, as in a glossary | logit, epoch, token, context window, hallucination, frontier model | card, then node | no | no | no | optional | no |
| `method` | a named way to learn, train, infer, prompt, align, compress or evaluate, judged by its empirical results | dropout, LoRA, RLHF, DPO, beam search for decoding, chain-of-thought prompting, speculative decoding, Monte Carlo dropout | node | yes | yes | yes | optional | optional |
| `architecture` | a design of a model: its layers, its computation, its parameter shapes | perceptron, LeNet-5, LSTM, ResNet, U-Net, Transformer, Vision Transformer, Mamba, mixture of experts | node | yes | yes | yes | optional | yes |
| `model` | a specific trained artifact or a family of them, with its developer, date, size, data, licence and results | AlexNet, BERT-base, GPT-3 175B, Llama 3.1 405B, AlphaFold 2, Stable Diffusion 1.5, Whisper large-v3 | card; node for notable models (18.5) | no | no | no | no | no |
| `task` | a problem that AI systems solve, with its input, its output and how success is measured | image classification, named entity recognition, machine translation, protein structure prediction, text-to-SQL, theorem proving | node | no | contract (10.2) | no | optional | optional |
| `benchmark` | a fixed evaluation: data, a protocol and metrics, with its versions | MNIST, ImageNet ILSVRC-2012, GLUE, MMLU, HumanEval, SWE-bench Verified, ARC-AGI, GPQA Diamond | card; node for notable ones | no | harness adapter (10.4) | no | no | no |
| `dataset` | a collection of data used to train or fine-tune, with its source, size, licence and documentation | Common Crawl, The Pile, LAION-5B, FineWeb, C4, ImageNet-21k, Open X-Embodiment | card; node for notable ones | no | no | no | no | no |
| `metric` | a measure of quality or of cost, with a formula and a reference implementation | accuracy, F1, BLEU, ROUGE-L, perplexity, pass@k, mean average precision, FID, Elo, expected calibration error | node | yes | yes, with test vectors | no | optional | optional |
| `theorem` | a proved result about learning, inference or agents | the perceptron convergence theorem, the universal approximation theorem, the PAC bound for finite classes, the policy improvement theorem, Arrow-style impossibility of fairness criteria | node | the proof | no | optional (an illustration) | yes | optional |
| `finding` | an empirical regularity or result that the field relies on or debates | neural scaling laws, double descent, grokking, the lottery ticket hypothesis, emergent abilities, the bitter lesson as an observation, the reversal curse | node | no | optional | yes | no | optional |
| `argument` | a philosophical or conceptual argument or thought experiment, with premises, conclusion and replies | the Turing test, the Chinese room, the symbol grounding problem, the orthogonality thesis, instrumental convergence, the frame problem | node | the argument's structure | no | no | optional | no |
| `open-problem` | a question that the field has not settled, with the positions and the evidence for each | does scaling alone reach general intelligence; is in-context learning implicit optimization; can deceptive alignment arise from training | node | no | no | optional | no | no |
| `system` | a built and named AI system, often with many models and parts, historical or deployed | Logic Theorist, ELIZA, SHRDLU, MYCIN, Deep Blue, Watson, AlphaGo, ChatGPT, Waymo Driver | card; node for notable ones | no | no | no | no | optional |
| `software` | a library, framework, engine or tool built for AI | scikit-learn, PyTorch, JAX, TensorFlow, Hugging Face Transformers, vLLM, llama.cpp, DeepSpeed, LangChain, Gymnasium | card; node for notable ones | no | usage examples | no | no | no |
| `hardware` | a chip, accelerator, system or interconnect built for AI or used for it | NVIDIA H100, Google TPU v5p, Cerebras WSE-3, NVLink, the Connection Machine, the Mark I Perceptron | card; node for notable ones | no | no | no | no | no |
| `risk` | a class of harm or failure that AI can cause or suffer | reward hacking, specification gaming, prompt injection, data poisoning, membership inference, algorithmic discrimination, deepfake fraud, loss of control | node | no | optional (a demo) | optional | no | optional |
| `incident` | a dated event where AI caused or nearly caused harm | an AI Incident Database entry; a documented model jailbreak used in an attack | card | no | no | no | no | no |
| `policy` | a law, a regulation, an executive act, a treaty, a voluntary framework or a lab policy about AI | the EU AI Act (Regulation (EU) 2024/1689), the Digital Omnibus on AI (Regulation (EU) 2026/1744), the NIST AI RMF 1.0, the Council of Europe Framework Convention on AI, a responsible scaling policy | card; node for notable ones | no | no | no | no | optional (timeline) |
| `provision` | one article, section or requirement of a policy or a standard | EU AI Act Article 5 (prohibited practices); Article 50 (transparency); NIST AI RMF "GOVERN 1.1" | card | no | no | no | no | no |
| `standard` | a technical standard or specification about AI | ISO/IEC 22989, ISO/IEC 42001, ISO/IEC 23894, IEEE 7001, C2PA, the ONNX specification, the Model Context Protocol | card; node for notable ones | no | no | no | no | no |
| `result` | one published number: a model, a benchmark version, a setting, a score, a source, a date | "GPT-4, MMLU 5-shot, 86.4, OpenAI technical report, 2023-03" | card | no | no | no | no | no |
| `work` | a paper, book, report, essay, talk or course that is part of the record of AI | "Computing Machinery and Intelligence"; *Perceptrons*; AIMA 4e; "Attention Is All You Need"; the AI Index 2026 | card; node for the canon (13.3) | no | no | no | no | no |
| `person` | a person who made, named or shaped items | Alan Turing, John McCarthy, Marvin Minsky, Judea Pearl, Fei-Fei Li, Geoffrey Hinton | card; node for the notable (19.6) | no | no | no | no | no |
| `organization` | a lab, a company, a university group, an institute, a standards body, a regulator or a funder | Stanford AI Lab, DeepMind, OpenAI, Anthropic, FAIR, AI2, the UK AI Security Institute, ISO/IEC JTC 1/SC 42, DARPA | card; node for the notable | no | no | no | no | no |
| `event` | a dated happening: a workshop, a competition, a summit, a winter, a milestone game | the Dartmouth workshop (1956), the Lighthill report (1973), Deep Blue against Kasparov (1997), the ImageNet challenge of 2012, the Bletchley Park summit (2023) | card; node for the notable | no | no | no | no | no |
| `exercise` | a practice problem in a public course or on a public site, as a link with metadata | a Kaggle "Getting Started" competition; an exercise of a public course; a Gymnasium environment to solve | card | no | no | no | no | no |

Notes:

- **Algorithms are not a kind here.** Algorithm-shaped items live on algorithm.tamnd.com (5.9) and appear here as bridge cards, linked with `[[al:k-means-clustering]]`. A method page here can `use` an algorithm: "Lloyd's algorithm" is on algorithm; "k-means++ seeding in practice, and how k-means is used for vector quantization in VQ-VAE" is the AI view and links to it.
- **A method and an architecture are different things.** An architecture says what the model computes (the Transformer block). A method says how it is trained, used or changed (pre-layer normalization as a training-stability method, KV caching as an inference method). A model is a trained artifact of an architecture with methods.
- **A model and a model family.** `model` nodes and cards have `role: family | release | checkpoint`. "Llama" is a family; "Llama 3.1" a release; "Llama 3.1 405B Instruct" a checkpoint. Checkpoints are cards; families and notable releases are nodes. `member_of` and `fine_tuned_from` link them (4.7).
- **A task and a benchmark.** A task is the problem ("extractive question answering"); a benchmark is a fixed test of it ("SQuAD 1.1, dev set, exact match and F1"). Results attach to benchmark versions, never to tasks (10.6).
- **A finding and a theorem.** A theorem is proved. A finding is observed, and its strength is the evidence of 07: how many independent replications, at what scales, with which known failures. "Scaling laws" is a finding; "a PAC bound" is a theorem.
- **A risk and an incident.** A risk is a class ("prompt injection"); an incident is a dated event that the class explains. Incidents link to risks with `instance_of`.
- **Variants.** A variant has `variant_of`. RMSNorm `variant_of` layer normalization; GRU `variant_of` LSTM is wrong (it is a sibling), so GRU uses `related_to` with a note. A variant has its own page when it has its own equations, analysis or results; a difference of one line is a section of the parent page.
- **Proofs and derivations** are not nodes. They belong to the node that they prove or derive, as in algorithm (06.5).

## 4.3 Tags, slugs, owners and URIs

- **Tag.** 6 characters of Crockford base32, the alphabet of math2, algorithm and programming, for example `5TQH2W`. A tag is permanent and never reused. `registers/tags.reg` lists each tag. A new item has the provisional id `NEW-<slug>` until `aik mint` gives it a tag before the merge. Cards get their tags at harvest time, in bulk, from the same minting code.
- **URI.** `ai://5TQH2W`, and `ai://5TQH2W@2027.03` for the item in a release. Fragments name parts: `ai://5TQH2W#derivation-gradient`, `ai://5TQH2W/impl/pytorch`, `ai://5TQH2W/exp/dropout-mlp-mnist`. The scheme differs from `al://`, `pg://` and `m2://`, so a link always says which project it means (Q-02).
- **Global slugs.** These kinds have one slug that is unique over the global namespace and is the URL: `area` (top level), `concept`, `definition`, `method`, `architecture`, `task`, `metric`, `theorem`, `finding`, `argument`, `open-problem`, `risk`, `person`, `organization`, `event`, `system`, `software`, `hardware`, `standard`, `policy`. Examples: `/attention/`, `/lora/`, `/transformer/`, `/bleu/`, `/grokking/`, `/chinese-room/`, `/reward-hacking/`, `/eu-ai-act/`.
- **Owned slugs.** These kinds are named inside an owner, and their URL has the owner first:

| Kind | Owner | URL form | Example |
|---|---|---|---|
| `model` (release, checkpoint) | a model family, or the developer when there is no family | `/models/<family>/<slug>/` | `/models/llama/llama-3-1-405b/`, `/models/bert/bert-base-uncased/` |
| `model` (family) | none | `/models/<family>/` | `/models/llama/`, `/models/alphafold/` |
| `benchmark` versions | the benchmark | `/benchmarks/<slug>/<version>/` | `/benchmarks/swe-bench/verified/`, `/benchmarks/mmlu/pro/` |
| `dataset` | none, under the section | `/datasets/<slug>/` | `/datasets/fineweb/` |
| `result` | the benchmark version | `/benchmarks/<slug>/<version>/#r-<tag>` | an anchor on the results table |
| `provision` | the policy or the standard | `/<policy>/<provision>/` | `/eu-ai-act/article-50/`, `/nist-ai-rmf/govern-1-1/` |
| `incident` | the section | `/incidents/<source>-<id>/` | `/incidents/aiid-1625/` |
| `work` | the section | `/works/<slug>/` | `/works/attention-is-all-you-need/` |
| `exercise` | a site | `/exercises/<site>/<id>/` | `/exercises/kaggle/titanic/` |

  An owned slug is unique inside its owner. The owner must be a node or card of a kind that can own.
- **Reserved words.** No global slug may be a reserved word of the site (16.2): `models`, `benchmarks`, `datasets`, `results`, `works`, `incidents`, `people`, `orgs`, `events`, `exercises`, `compare`, `timeline`, `glossary`, `paths`, `search`, `about`, `spec`, `t`, `x`, and the rest of `registers/reserved.yaml`.
- **Aliases.** `aliases` lists other names, each with a language when it is not English. Acronyms are aliases (`rlhf`, `dpo`, `vae`, `gan`, `cnn`), and search resolves them directly. arXiv ids, DOIs, AIID ids, Epoch ids and Hub repo ids are identifiers in `same_as` (4.5), and search resolves them too: `1706.03762` goes to the work card of "Attention Is All You Need".
- **Disambiguation.** Shared names get qualified slugs and an entry in `registers/disambiguation.yaml`. Examples: "attention" (the mechanism, `attention`; the psychology, a card that links out); "transformer" (the architecture; the electrical device is out of scope); "agent" (the AI concept; the software pattern on programming); "model" (a trained model; a model of computation on algorithm, `comparison-model`); "kernel" (kernel methods, `kernel-methods`; a GPU kernel, `gpu-kernel`); "regularization"; "Gemini" (the model family; the spacecraft is out of scope); "Claude" and "Llama" (families; people and animals out of scope); "PaLM"; "BERT"; "attention head"; "alignment" (AI alignment; sequence alignment on algorithm).
- **Renames.** A rename moves the old slug to `registers/aliases.reg`, and the site redirects it, as in algorithm. Model families that rebrand (Bard to Gemini) keep one node with the old name as an alias and a dated `names` list.

## 4.4 The node directory

A node lives at `nodes/<top>/<sub>/<slug>/`. The two area levels are the primary area of the node (05.2), as in algorithm. Model, benchmark and dataset nodes live under their area like every other node; the URL form of 4.3 is a site route, not a path.

A method directory, the fullest form:

```
nodes/deep-learning-training/regularization/dropout/
  node.md                      front matter and the text (06)
  derivations/
    expectation-scaling.md     why inverted dropout scales by 1/(1-p) (06.5)
    as-ensemble.md             the approximate ensemble view, with its assumptions
  impl/
    numpy/dropout.py           the reference from scratch: forward and backward (08)
    numpy/test_dropout.py      shapes, expectation, gradient check, edge cases
    pytorch/dropout.py         the same layer as a torch.nn.Module
    pytorch/test_dropout.py    agreement with the NumPy reference and with torch.nn.Dropout
    jax/dropout.py
    jax/test_dropout.py
  exp/
    mlp-mnist/                 one experiment per directory (09)
      experiment.yaml          the claim, the config, the data pins, the seeds, the budget class
      run.py                   the training and evaluation script
      expect.yaml              the statistical test that decides the claim
      results.json             written only by `aik exp run -update`
      figures/                 plots made by the run, as SVG
  claims.yaml                  the typed claims of the page and their evidence (07)
  viz/
    scene.ts                   a scene (12)
    presets.yaml
  figures/                     optional SVG figures by hand
```

An architecture directory adds a shape contract:

```
nodes/deep-learning-architectures/transformers/transformer/
  node.md
  shapes.yaml                  the tensor shapes of each block, the parameter count and FLOP formulas (08.6)
  derivations/
    scaled-dot-product.md      why the 1/sqrt(d_k) factor (06.5)
    parameter-count.md         the count of 12 d^2 per layer, step by step
  impl/numpy/ impl/pytorch/ impl/jax/
  exp/tiny-copy-task/          a 2-layer model learns to copy: the smallest check that the code learns
  claims.yaml
  viz/
```

A task directory has a contract (10.2). A metric directory has test vectors:

```
nodes/evaluation/text-generation-metrics/bleu/
  node.md
  derivations/brevity-penalty.md
  contract.yaml                the signature, the input and output types, the tolerance
  vectors/
    official.jsonl             inputs and outputs computed by the official implementation (sacreBLEU), with its version
    edge.jsonl                 empty hypothesis, no n-gram match, one-word references, Unicode
  impl/numpy/ impl/pytorch/ impl/jax/
```

A model, benchmark or dataset node has the facts and the narrative, and no code:

```
nodes/foundation-models/open-weight-llms/llama/
  node.md                      the family page: lineage, releases, architecture choices, data, licence history, reception
  facts.yaml                   dated facts with sources (4.9)
  releases.yaml                the releases and checkpoints, each pointing to its card
```

The Lean part lives in the Lean package, `lean/AI/<Area>/<Name>.lean`, as in algorithm (11). The front matter names the module and the declarations.

## 4.5 The front matter

YAML front matter of `node.md`. The schema is `schema/node.v1.json` in ai-agent. Fields that do not apply to a kind are absent. A method:

```yaml
---
tag: 3KD9QX
kind: method
slug: dropout
name: Dropout
aliases: [inverted dropout, "dropout regularization"]
area: deep-learning-training/regularization       # primary area (05.2)
areas: [deep-learning-foundations/neural-network-basics]
level: core                                       # intro | core | advanced | research (5.6)
summary: >-
  During training, set each unit of a layer to zero with probability p and scale the others by 1/(1-p), so that the network cannot rely on any single unit.
purpose: regularization                           # a value of registers/purposes.yaml (06.3)
applies_to: [feedforward-neural-network, convolutional-neural-network, transformer]
uses: [bernoulli-distribution@m2, stochastic-gradient-descent@al]   # links to neighbours carry the site suffix
variant_of: null
related_to: [dropconnect, stochastic-depth, gaussian-dropout, monte-carlo-dropout]
hyperparameters:
  - {name: p, meaning: "the probability of dropping a unit", typical: "0.1 to 0.5", default_in: {pytorch: 0.5}}
cost:
  train: "one Bernoulli mask per activation; O(1) extra time per activation"
  inference: "none (identity), with inverted scaling"
history:
  year: 2012
  people: [geoffrey-hinton, nitish-srivastava]
  first: {work: hinton-2012-improving-neural-networks-arxiv, note: "arXiv 1207.0580"}
  canonical: {work: srivastava-2014-dropout-jmlr}
prereqs: [overfitting, feedforward-neural-network, regularization]
same_as:
  wikidata: Q25339462
  wikipedia: Dilution_(neural_networks)
  openalex_topic: null
  arxiv: ["1207.0580"]
  al: dropout                                     # the neighbour node, if one exists (5.9)
lean: null
sources:
  - {work: srivastava-2014-dropout-jmlr, locator: "Section 4, Eq. 1 to 3"}
  - {work: goodfellow-2016-deep-learning, locator: "Section 7.12"}
  - {work: zhang-d2l, locator: "Section 5.6 (Dropout)"}
---
```

Fields by kind (the full list is in the schema):

- **architecture:** `family` (convolutional, recurrent, attention, state-space, graph, mixture, hybrid), `inputs` and `outputs` (modalities and shapes), `components` (the blocks it is made of, as slugs), `inductive_biases`, `params` (a formula in the variables of `shapes.yaml`), `flops_per_token` or `flops_per_example`, `introduced_by` (a work), `models` (notable models that use it).
- **model:** `role` (family, release, checkpoint), `developer` (organizations), `released` (date), `architecture` (slug), `params` (total and active, with source), `training` (`compute_flop`, `tokens`, `data` as dataset slugs, `hardware`, each a fact of 4.9), `modalities`, `context_length`, `access` (open weights, open source, API, closed, none), `licence` (SPDX id or a licence card), `fine_tuned_from`, `distilled_from`, `member_of`, `card_url`, `same_as` (Epoch id, Hub repo id, Wikidata).
- **task:** `inputs`, `outputs`, `modality`, `family` (classification, structured prediction, generation, decision making, retrieval, ranking), `metrics` (slugs), `benchmarks` (slugs), `subtask_of`.
- **benchmark:** `task`, `versions` (each with `id`, `date`, `size`, `splits`, `hash` of the published data, `harness` (10.4)), `metrics`, `protocol` (shots, prompt format, decoding, scoring), `contamination` (known contamination reports), `saturation` (computed, 10.8), `licence`, `introduced_by`.
- **dataset:** `modality`, `size` (items, tokens, bytes, each a fact), `source` (where it came from: crawl, curation, synthetic, human annotation), `collection` (dates), `licence`, `access`, `documentation` (datasheet, data card), `derived_from`, `known_issues` (risk slugs and incident ids).
- **metric:** `measures` (what quality or cost), `range`, `better` (higher or lower), `formula` (in the notation of 06.4), `reference_impl` (the official implementation with its version), `properties` (`differentiable`, `symmetric`, `bounded`, `decomposable`).
- **theorem:** `statement` (one line, in math), `hypotheses`, `proof` (inline or `derivations/proof.md`), `lean`, `tight` (with the example).
- **finding:** `claim` (one sentence), `first_reported`, `replications` (works), `scale_range` (the scales at which it was observed), `failures` (works that did not find it), `explanations` (competing explanations, each with sources).
- **argument:** `premises` (numbered), `conclusion`, `replies` (each with a name, a summary and sources), `responds_to`.
- **open-problem:** `question`, `positions` (each with a name, a summary, its proponents as person slugs, and sources), `evidence` (claims that bear on it), `would_settle_it` (what result would decide it).
- **risk:** `taxonomy` (the source taxonomies that list it: MIT AI Risk Repository, NIST AI 600-1, the EU AI Act, OWASP Top 10 for LLM Applications, MITRE ATLAS), `affects` (system parts), `mitigations` (method slugs), `incidents` (computed from the reverse edge).
- **policy:** `jurisdiction`, `type` (law, regulation, executive order, guidance, treaty, voluntary framework, lab policy), `status` (computed from `dates`), `dates` (proposed, adopted, in force, application dates per part, amendments), `provisions` (card ids), `amends`, `amended_by`, `official_text` (the URL of the authoritative text).
- **system:** `developer`, `years`, `parts` (models, software, hardware), `domain`, `status` (active, retired, historical), `milestones` (events).
- **software:** `language`, `repo`, `licence`, `first_release`, `maintainer`, `category` (framework, library, serving engine, training system, agent framework, evaluation harness, data tool), `releases` (card ids).
- **hardware:** `vendor`, `type` (GPU, TPU, NPU, wafer-scale, interconnect, system), `released`, `specs` (peak FLOP/s by precision, memory, bandwidth, power, each a fact).
- **person, organization, event:** as in programming, plus `roles` for people (researcher, engineer, founder, policy maker, critic) and `kind_of` for organizations (lab, company, university group, institute, government body, standards body, non-profit).

Rules:

- `level` is one of `intro`, `core`, `advanced`, `research` (5.6).
- Status fields do not exist. The build computes the ladders (4.10) and every computed status (a policy's "in force", a benchmark's "saturated", a model's "deprecated"), and rule T05 rejects a status written by hand.
- Numbers that change or that sources disagree on are never plain front-matter values. They are facts (4.9). The schema marks which fields are facts.
- Every slug in a field must exist, as a node, a card or a bridge card (rule E01). The suffixes `@al`, `@m2` and `@pg` name a neighbour (5.9).

## 4.6 Cards

A card is one JSON object on one line in `cards/<kind>/<shard>.jsonl`, sharded by prefix so that no file passes 5 MB (20.3). The fields are the front-matter fields of the kind, plus:

```json
{"tag":"8RWQ2D","kind":"model","slug":"llama-3-1-405b","owner":"llama","name":"Llama 3.1 405B",
 "role":"release","developer":["meta-ai"],"released":{"v":"2024-07-23","src":"S:meta-llama31-blog"},
 "architecture":"decoder-only-transformer",
 "params":{"total":{"v":4.05e11,"src":"S:meta-llama31-paper","loc":"Table 3"}},
 "training":{"compute_flop":{"v":3.8e25,"src":"S:meta-llama31-paper","loc":"Section 3.3.1"},
             "tokens":{"v":1.56e13,"src":"S:meta-llama31-paper","loc":"Section 3.1"}},
 "access":"open-weights","licence":"llama-3-1-community-license",
 "same_as":{"epoch":"Llama 3.1-405B","hub":"meta-llama/Llama-3.1-405B","wikidata":null},
 "summary":"The largest model of the Llama 3.1 release, a dense decoder-only transformer.",
 "harvest":{"source":"epoch-ai-models","snapshot":"b3:9f2c…","row":"Llama 3.1-405B","date":"2026-11-12"},
 "rules":{"version":"r-2026.11","passed":true}}
```

The values above illustrate the format; the harvester and the D rules of 15.3 decide the real ones.

Rules:

- Every fact field has `v` (the value), `src` (a source id of `registers/sources.yaml` or a work tag) and, when the source has one, `loc` (a table, a section, a page).
- A card has no prose beyond `summary`, at most 30 words, written by a model from the facts and checked by rule S10 (no claim that the facts do not support).
- A card's `harvest` record says where each row came from. A card that two sources describe has two `harvest` records, and the fact model of 4.9 merges them.
- A card that is promoted keeps its tag; its row moves to the node's `facts.yaml` (4.4).
- A work card that is promoted moves its fields (`type`, `authors`, `year`, `venue`, `standing`, `licence_class`, `cited`) to the front matter of the work's node, and aik reads the node as the work's card, so claims that cite the work keep their source.

## 4.7 Edges

Edges are typed fields of the front matter or the card. The build derives the reverse edges. `schema/edges.md` defines each one, and audit rules E01 to E14 check them (15.3).

| Edge | From → to | Meaning |
|---|---|---|
| `prereqs` | any → any | read the target first; it forms the reading DAG (5.7) |
| `uses` | method, architecture, system, model → method, architecture, concept, algorithm bridge | it calls, contains or depends on the target |
| `components` | architecture → architecture, method | the blocks it is made of (the Transformer has multi-head attention, a position-wise MLP, residual connections, layer normalization) |
| `variant_of` | method, architecture, metric, benchmark → same kind | it changes the target in a named way |
| `related_to` | any → any | a sibling or a close idea, with a one-line note on the difference |
| `generalizes` | concept, method, task → same kind | the target is a special case |
| `applies_to` | method → architecture, task | the method is used with the target |
| `solves` | method, architecture, model, system → task | it is used for the task |
| `evaluated_on` | model, method → benchmark version | it has results there (derived from result cards) |
| `measured_by` | task, benchmark → metric | the metric of the task or the benchmark |
| `trained_on` | model → dataset | with the share, when known |
| `architecture` | model → architecture | the architecture of the model |
| `fine_tuned_from`, `distilled_from`, `merged_from` | model → model | the lineage of weights |
| `member_of` | model → model family | the family |
| `developed_by` | model, system, software, hardware, benchmark, dataset → organization, person | who made it |
| `introduced_by` | any → work | the work that introduced it |
| `cites` | work → work | from OpenAlex or Semantic Scholar, only between works that have cards |
| `proves` | theorem → concept, method, architecture | the theorem is about the target |
| `explains` | concept, theorem → finding | a proposed explanation, with its status in `claims.yaml` |
| `evidence_for`, `evidence_against` | finding, work, result, experiment → finding, open-problem, claim | evidence, with the claim id (07.3) |
| `instance_of` | incident → risk | the incident is a case of the risk |
| `mitigates` | method, policy, standard → risk | it reduces the risk, with the evidence label |
| `regulates` | policy, provision → system kind, risk, practice | the law covers it |
| `implements_policy` | standard → policy, provision | a harmonized standard or a code of practice for the provision |
| `amends` | policy → policy | the amendment |
| `responds_to` | argument, work → argument | a reply to the target |
| `succeeded_by` | model, hardware, software, benchmark → same kind | the next version |
| `people`, `orgs` | any → person, organization | who found, named, built or improved it |
| `events` | any → event | it took part in the event |
| `exercises` | any → exercise | practice for the target |
| `same_as` | any → external ids | Wikidata, Wikipedia, arXiv, DOI, OpenAlex, Semantic Scholar, DBLP, Epoch, Hub, AIID, ORCID |
| `see_also` | any → any | a related page, at most 8 |
| `@al`, `@m2`, `@pg` targets | any → neighbour node | a link to a neighbour, carried by a bridge card (5.9) |

Graph rules (15.3 E): no cycle in `prereqs`, `variant_of`, `generalizes`, `fine_tuned_from` or `succeeded_by`; `solves` targets are tasks; `evaluated_on` targets are benchmark versions; `architecture` targets are architectures; each core node has a `prereqs` path from a foundation node (5.7).

## 4.8 Provenance

Each file of a node directory and each card row has a provenance record, as in algorithm and programming. Node files: `prov/<tag>.jsonl`, one line per change: the date, the file, the operation, the hash, the author (a GitHub login or an agent run id with the model ids and the prompt versions), the auditor, the reviewer when there is one, and the source ids that the change used. Card rows: the `harvest` record (4.6) and `prov/cards/<shard>.jsonl` for changes by people or agents. The page shows the record under "History" (16.4). `aik prov show` prints the log of a node with the state of each file, `aik prov check` checks every line, and `aik prov record` appends the lines of a change made by a person; `work/prov.py` appends those of an agent run.

## 4.9 Facts

AI facts change, disagree and age. "GPT-3 has 175 billion parameters" is stable. "The training compute of model X" has three published estimates. "The best score on SWE-bench Verified" changes every month. So a number is never a bare field. It is a **fact**:

```yaml
# nodes/.../llama/facts.yaml, or a fact field inside a card
- id: params.total
  value: 4.05e11
  unit: parameters
  sources:
    - {src: "S:meta-llama31-paper", loc: "Table 3", value: 4.05e11, kind: primary}
    - {src: "S:epoch-ai-models", row: "Llama 3.1-405B", value: 4.05e11, kind: secondary, as_of: 2026-11-12}
- id: training.compute_flop
  value: 3.8e25
  unit: FLOP
  sources:
    - {src: "S:meta-llama31-paper", loc: "Section 3.3.1", value: 3.8e25, kind: primary}
    - {src: "S:epoch-ai-models", value: 3.8e25, kind: secondary, method: "reported", as_of: 2026-11-12}
- id: training.tokens
  value: 1.5e13
  unit: tokens
  no_primary: "The developer gives only an approximate count in a blog post."
  sources:
    - {src: "S:epoch-ai-models", row: "Example model", value: 1.5e13, kind: secondary, as_of: 2026-11-12}
```

The build computes the status of each fact from its sources, and the page and `/api/nodes.json` show it. The file never holds a status (T05). The statuses:

| Status | When |
|---|---|
| `agreed` | a primary source and at least one other source give the same value |
| `primary-only` | only primary sources give the value |
| `secondary-only` | only secondary sources give the value, and no estimator computes it |
| `disputed` | two sources differ beyond the tolerance |
| `estimated` | no primary source gives the value, and an estimator computes it |

A fact with no primary source says why in `no_primary`, for example when the developer never published the number. Rule D04 then accepts it. When the file gives no `value`, the build takes the value of the best source: the first primary source, else the first estimate, else the first secondary source.

Rules:

- **Source order.** A primary source (the developer's paper, model card, documentation or official text) wins over a secondary source (Epoch AI, the AI Index, Wikipedia, news). When they differ by more than the tolerance of the field (`schema/facts.v1.yaml`: 1% for numbers, the precision of the less precise date for dates, case for text and order for lists), the status is `disputed`, and the page shows both values with their sources.
- **Estimates.** A value that no primary source gives but that an estimator computes (Epoch AI's compute estimates) has status `estimated`, with the estimator and its method. The page says "estimated by Epoch AI".
- **Time.** A fact that is true only at a date (a price, a rank, an "in force" status, a count of users) has `as_of`, and the page prints the date next to the value. Rule D07 rejects a time-bound fact without `as_of`. The time-bound ids are `access`, `licence`, `status`, `price`, `users`, `downloads`, `likes`, `stars` and `rank`, and the ids under them.
- **Computed facts.** Ranks, "first", "largest", "most cited" and similar facts are computed by the build from the registers (`aik facts compute`), with the date and the population ("the largest open-weights dense model by total parameters among the model cards, as of 2026-11-13"). They are never written by hand.

## 4.10 The computed ladders

The build computes seven ladders for each node that can have them. No person and no agent writes them (rule T05). A review counts only for the hash that it reviewed; when the file changes, the level drops until a new review, as in math2 and algorithm.

**W. Writing** (06.10)

| Level | Meaning |
|---|---|
| W0 | registered: a card or a stub directory, not published as a page |
| W1 | a page: the lead, "In plain words" and the main section of its kind, passing S rules |
| W2 | all required sections of the kind (06.3), with links and sources |
| W3 | W2, the claims of `claims.yaml` extracted and each one grounded (G2), the derivations complete |
| W4 | W3, the cross-family audit passed with no blocker or major issue (15.4), the page covers everything that the canon says about the item (13.3) |
| W5 | W4, and a reviewer of the subject reviewed this exact hash |

**G. Grounding** (07, 13.6)

| Level | Meaning |
|---|---|
| G0 | no sources |
| G1 | sources with locators, in the right form (G01 to G04) |
| G2 | every claim of `claims.yaml` has a source, and the citation check found the claimed text or number in the stored snapshot of the source (G05) |
| G3 | G2, and the verifier (a model of another family than the writer) judged every claim supported, with no claim contradicted (15.4) |

**K. Claims** (07): the share of the page's claims with a computed evidence label above `reported` is shown as a number, not a level. K is a set of rules (15.3), not a ladder.

**X. Execution** (for each framework of 08.2: `numpy`, `pytorch`, `jax`, and the tier-2 ones)

| Level | Meaning |
|---|---|
| X0 | no implementation |
| X1 | it runs in its pinned image with no network and passes its own tests: shapes, edge cases, the worked example of the page |
| X2 | X1, and it agrees with the NumPy reference on the shared fixtures within the tolerance of the contract (the differential test, 08.7); gradients pass the finite-difference check; metric implementations reproduce the official test vectors |
| X3 | X2, and lint and format are clean, the code follows the conventions of 08.3, and its tests kill at least 80% of the mutants (08.8) |
| X4 | X3, and a person reviewed this exact file hash |

**R. Reproduction** (for methods, architectures, findings and risks with a demo, 09)

| Level | Meaning |
|---|---|
| R0 | no experiment, and the page shows the evidence label of the published results only |
| R1 | an experiment exists, runs in its budget class, and records its results with seeds and confidence intervals |
| R2 | R1, and its `expect.yaml` test passes: the qualitative claim of the page holds at the experiment's scale (for example, "with dropout the test loss is lower than without, over 5 seeds, Welch test p < 0.01") |
| R3 | R2 at a second scale or on a second dataset named in the claim, so the result is not a single point |
| R4 | R3, and a person reviewed the experiment design and this exact results hash |

**F. Formal** (for theorems, and the concepts and metrics with a Lean part, 11)

| Level | Meaning |
|---|---|
| F0 | no Lean part |
| F1 | the statement is in Lean, and a reviewer checked that it says what the page says (fidelity) |
| F2 | the theorem is proved: no `sorry`, only the standard axioms |
| F3 | F2, and the page's proof and the Lean proof follow the same steps, linked step by step (11.6) |

**V. Visualization** (12)

| Level | Meaning |
|---|---|
| V0 | none |
| V1 | static figures only |
| V2 | an interactive scene or explorable that passes the trace checks, with step back, custom input and code sync where it has code |
| V3 | V2, and a person reviewed it for clarity and accessibility |

**D. Data facts** (for model, dataset, benchmark, hardware, policy and system nodes and cards, 4.9)

| Level | Meaning |
|---|---|
| D0 | a name and a source only |
| D1 | every required fact of the kind present, each with a source |
| D2 | D1, and every fact cited to a primary source where one exists, with locators, and every disagreement recorded |
| D3 | D2, and a reviewer checked the facts against the primary sources for this exact hash |

A node is **complete** for its kind when it reaches the targets of `schema/targets.v1.yaml`. The defaults: W4 and G3 for all; X3 in NumPy and PyTorch and X2 in JAX for kinds with implementations; R2 for methods, architectures and findings whose claim has a small-scale form (09.2); F2 for theorems with `lean: expected`; V2 for nodes with `viz: expected`; D2 for fact kinds. The badges of the site (15.1) and the coverage levels (18.2) are computed from these ladders.

## 4.11 Prohibitions

- No status, level, badge, rank or "state of the art" in front matter or prose (T05, K04).
- No bare number for a fact of 4.9 (D01).
- No copied text, code or figure from a source whose licence forbids it (19, C01).
- No implementation that imports a package outside the pinned set of its framework (08.4, X07).
- No result card without a benchmark version, a setting and a source with a locator (M01).
- No experiment result written by hand: `results.json` comes only from `aik exp run -update` (R05).
- No `sorry`, `admit`, `native_decide` or new axiom in a module that a node cites as proved (F03).
- No position on a contested question in the site's own voice (N01).
- No private fact about a living person (P01, 19.6).
