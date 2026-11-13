# 10. Tasks, benchmarks and results

A large part of what AI knows is written as numbers: this model scored this on that benchmark under these settings, according to this source, on this date.
Papers with Code collected those numbers for seven years and shut down on 2025-07-24; its URLs now redirect to Hugging Face Trending Papers, which has no leaderboards, and its final data dump is the last public copy of its results (01.2).
This document defines the replacement: task contracts, versioned benchmarks, a results register of result cards with sources and settings, computed comparability, and computed tables that print their date.
The site never runs closed models and never ranks outside a computed table (02.6).

## 10.1 The three kinds

| Kind | It is | It owns | Example |
|---|---|---|---|
| `task` | the problem: its input, its output, what success means | its contract (10.2), its metrics, its benchmarks | extractive question answering |
| `benchmark` | a fixed test of a task: data, protocol and metrics, in versions | its versions (10.3), its harness adapter (10.4), its result tables (10.7) | SQuAD, version 1.1, dev set |
| `result` | one published or measured number for one model on one benchmark version under one setting | its source, its locator, its setting, its evidence | "model X, SQuAD 1.1 dev, F1 93.2, Table 2 of the paper" |

Results attach to benchmark versions, never to tasks or to benchmarks without a version (rule M02).

## 10.2 Task contracts

Each task node has `contract.yaml`. It says what an input and an output are, how an output is scored, and how a harness calls a model. It is the AI form of algorithm's problem contract (2150/09.1).

```yaml
# nodes/natural-language-processing/question-answering/extractive-question-answering/contract.yaml
task: extractive-question-answering
input:
  question: {type: text}
  context:  {type: text, max_tokens_hint: 512}
output:
  answer: {type: span, of: context, nullable: false}   # SQuAD 2.0 makes it nullable: a variant task
modality: text
family: structured-prediction
metrics:
  - {metric: exact-match, normalization: squad-normalize}
  - {metric: f1-token, normalization: squad-normalize}
variants:
  - {task: extractive-question-answering-with-unanswerable, changes: "output.answer.nullable: true"}
adapter:                                   # how the harness turns a model into a solver (10.4)
  generative: {prompt_template: "templates/qa-extractive.txt", parse: "first-line-span"}
  span_head: {fields: [start_logits, end_logits]}
cases:                                     # tiny cases that every adapter must pass
  - {file: cases/basic.jsonl}
  - {file: cases/normalization.jsonl}      # punctuation, articles, casing
checker: checker.py                        # scores one output; must agree with the metric nodes' vectors
```

Rules:

- The metrics named by a contract are metric nodes with test vectors (08.7.5). The checker of the contract must agree with them on the task's cases (rule M04).
- A change that changes what counts as a correct output (nullable answers, a different normalization) is a variant task, not an edit of the contract.
- Tasks that no harness can run (protein structure prediction at full scale, autonomous driving) still have contracts: the input and output types and the metrics, with `adapter: none`.

## 10.3 Benchmark versions

A benchmark has versions. A result belongs to exactly one version. Two results on different versions are never in one table (10.6).

A new version exists when any of these changes:

| Change | Example |
|---|---|
| the data (items added, removed or fixed) | SQuAD 1.1 to 2.0; MMLU to MMLU-Pro and to MMLU-Redux; SWE-bench to SWE-bench Verified; HumanEval to HumanEval+ |
| the split that results report on | dev versus test; a public versus a hidden test set |
| the official protocol, when it changes the scores | a new official prompt format; a new scorer version; a changed time limit for agents |
| the scorer | a fixed bug in the official evaluation script |

Each version records:

```yaml
# releases.yaml of a benchmark node, or the benchmark card (part)
- id: verified
  name: SWE-bench Verified
  date: {v: "2024-08-13", src: "S:openai-swebench-verified"}
  derived_from: {version: full, rule: "a human-validated subset of 500 tasks"}
  size: {v: 500, unit: tasks, src: "S:openai-swebench-verified"}
  data: {location: "the official dataset repository", hash: "blake3:…", licence: "the dataset's licence card"}
  protocol: {scorer: "the official harness, version recorded per result", unit: "resolved tasks / all tasks"}
  metric: resolve-rate
  ceiling: {v: 1.0, note: "some tasks may be unsolvable; see the contamination and label-noise reports"}
  human_baseline: null
```

The values illustrate the format; the harvester and the D rules decide the real ones.

## 10.4 Harnesses

A harness runs a model on a benchmark version and scores it. Results from our own harness runs have the evidence `measured-here` (07.4).

- **Reuse, do not rebuild.** The harness adapters wrap existing open evaluation software, pinned as `software` items with exact versions in `manifests/harnesses.yaml`: EleutherAI's lm-evaluation-harness, the UK AI Security Institute's Inspect, Stanford's HELM, Hugging Face lighteval, the official scorers of benchmarks (the SQuAD script, the COCO API, the SWE-bench harness), and the metric implementations of 08.7.5. The node's own metric implementation is run beside the official scorer, and any disagreement is a finding (rule M05).
- **Only open-weight models we can run.** The harness runs only models with open weights whose licence allows evaluation, in the `gpu-small` or `gpu-medium` class (09.4): at M3 about models up to 14 billion parameters at 16 bits and up to 70 billion with 4-bit quantization, with the quantization recorded in the setting. It never calls closed models or paid APIs (rule M06, Q-31).
- **What it is for.** Not to compete with published leaderboards, but to: give a second, independent number for published results of open models (which makes them `replicated` or `contested`, 07.4); measure small models on benchmarks that their developers did not report; check the effect of settings (shots, prompt format) that the comparability rules use (10.6); and produce the numbers of teaching pages (how a 1B model does on GSM8K with and without chain of thought).
- **Runs are experiments.** A harness run is an experiment of kind `eval` in the benchmark's node directory, with `experiment.yaml`, pinned model weights by hash, pinned harness version, seeds for sampling, and `results.json`. The budget and the rules of 09 apply.
- **Contamination guard.** Benchmark test data never enters training runs of our experiments (rule M07): `registers/data-pins.yaml` marks test splits, and `aik exp check` rejects an experiment that trains on one.

## 10.5 Settings

The setting says how a number was obtained. Two numbers with different settings may not be comparable. `schema/setting.v1.yaml` defines the keys:

| Key | Values | Hard key |
|---|---|---|
| `shots` | 0, 1, 5, 25, … | yes |
| `prompt` | `official`, `harness:<name>@<version>`, `custom` with the template text or its hash | yes |
| `cot` | none, `cot`, `cot-sc@<n>` (self-consistency with n samples), `reasoning-model` (internal reasoning, with the budget if known) | yes |
| `decoding` | greedy, sampling with `temperature`, `top_p`, `n` | yes for metrics over samples |
| `pass_at` | k for pass@k, with n samples | yes |
| `tools` | none, or a list (code interpreter, search, a retriever) | yes |
| `scaffold` | none, or the agent scaffold with its version (for agent benchmarks) | yes |
| `compute` | test-time compute limits: tokens, attempts, time | yes when the benchmark has a budget |
| `split` | the split of the version | yes |
| `quantization` | none, or the method and bits | no, recorded and shown |
| `model_variant` | base, instruct, chat, a checkpoint id | yes |
| `harness` | the harness and its version | no, recorded |
| `date_run` | when the number was obtained, when the source says | no |
| `notes` | free text from the source | no |

A setting is written with the keys that the source states. A key that the source does not state is `unknown`, never guessed (rule M03).

## 10.6 The results register

### 10.6.1 A result card

```json
{"tag":"4MZK8P","kind":"result",
 "model":"llama-3-1-405b","model_variant":"instruct",
 "benchmark":"mmlu","version":"original-test",
 "metric":"accuracy","score":{"v":0.886,"unit":"fraction"},
 "setting":{"shots":5,"prompt":"custom","cot":"none","decoding":"greedy","split":"test"},
 "source":{"src":"S:meta-llama31-paper","loc":"Table 2","kind":"primary","date":"2024-07-23"},
 "evidence":"reported",
 "harvest":{"source":"paper-tables","snapshot":"b3:…","date":"2026-11-12","extractor":"r-2026-11-12-aa31"},
 "checks":{"cite":"found","comparable_group":"mmlu/original-test/5-shot/no-cot/greedy"}}
```

The values illustrate the format; the harvester and the citation check decide the real ones.

Rules:

- A result card has a model (a model node or card), a benchmark version, a metric that is a metric node, a score with its unit, a setting (10.5), a source with a locator, and the source's date (rule M01).
- The **citation check** (07.5) must find the number in the stored snapshot of the source at the locator, within the rounding of the source, before the card is published (`checks.cite: found`). A card whose number is not found is kept with `checks.cite: not-found`, listed in the inventory report, and not shown in any table (rule M08).
- A result has one score. A source table with ten benchmarks is ten cards.
- A result card writes no slug and no name. A result belongs to its benchmark, so `aik` takes the benchmark as the owner of the card, the tag in lowercase as its slug, and makes the name from the model and the benchmark version. The versions of a benchmark are in the field `versions` of its card, or in `releases.yaml` of its node (10.3).
- `evidence` is computed (07.4): `reported` for one source; `replicated` when an independent source (another lab's paper, our harness, an independent leaderboard) has a result in the same comparable group within the tolerance of the metric (`schema/metrics-tolerance.yaml`, default 1 point of percentage for accuracy-like metrics); `contested` when independent results differ by more than the tolerance; `measured-here` when our harness produced it.
- Results that the developer reports about its own model are marked `self_reported: true`. The page shows it.

### 10.6.2 Sources and ingest

| Source | What it gives | How it is ingested | Evidence of its cards |
|---|---|---|---|
| papers and technical reports (arXiv, OpenReview, venues) | most published results | the results extractor role reads tables from the PDF text in the raw store, writes candidate cards with locators; the citation check confirms each number | `reported` |
| model cards and official announcements | the developer's results | the same extractor on the stored snapshot of the card or page | `reported`, `self_reported` |
| Epoch AI's Benchmarking Hub and its data | results run by Epoch AI on a fixed protocol, with dates | the `epoch` adapter (13.2), CC BY 4.0, with attribution | `reported`; an independent source for `replicated` |
| official benchmark leaderboards (SWE-bench, ARC Prize, GAIA, the HELM leaderboards, the Open LLM Leaderboard archive) | results under the benchmark's own protocol | adapters that snapshot the leaderboard with its date | `reported`; independent when the maintainer runs the model |
| preference arenas (LMArena and others) | ratings from human votes, as of a date | snapshots with the date; the score is a rating with its interval, never an accuracy (10.10) | `reported` |
| the final Papers with Code data dump | about seven years of results, linked to papers | the `pwc` adapter; each row becomes a candidate card whose source is the original paper (with the dump as the secondary source); a row whose number the citation check cannot find in the paper is kept as `pwc-unverified` and not shown in tables (Q-35) | `reported` after the citation check |
| our harness | `measured-here` results (10.4) | experiment outputs | `measured-here` |

Deduplication: two candidate cards are the same result when the model, the version, the metric, the setting and the source are the same. Two sources that report the same number for the same run (a paper and the model card) are one card with two sources. Two independent runs are two cards in one comparable group.

### 10.6.3 Comparability

The build computes a **comparable group** for each card: the benchmark version, the metric, and the values of every hard key of the setting (10.5). Rules:

- A computed table holds one comparable group, or several groups shown in separate columns with their settings in the header (rule M09).
- A card with an `unknown` hard key is in a group of its own, and the table shows it only in an "other settings" section, with the unknown keys named.
- `registers/comparability.yaml` may declare two values of a key equivalent for one benchmark, with a source (for example two prompt templates that the benchmark's maintainers showed to give the same scores within the tolerance). Our own harness runs can provide that evidence.
- The comparability of results across benchmark versions is never assumed. A version derived from another (Verified from full) may show the other version's table as a link, not merged.

## 10.7 Task and benchmark pages: the comparison tables

A benchmark version page has a computed results table; a task page has a summary table over its benchmarks. Both are computed by `aik results table` at build time:

```
SQuAD 1.1, dev set · F1 · results as of 2026-11-13 · 214 results in 31 comparable groups
Setting: fine-tuned span model, no external data                                                      evidence
  1  model A (2019)   93.2   [paper, Table 2]                                                          replicated
  2  model B (2019)   92.8   [paper, Table 3]                                                          reported
  …
Setting: 0-shot, generative, official prompt
  …
Human performance (from the benchmark paper): 91.2
```

Rules:

- Every table prints its date, its population ("all result cards in this comparable group that passed the citation check"), its filters and the number of results (rule M10). Phrases like "the highest reported score" come only from this template, with the date, never from prose (K04).
- Rows show the source and the evidence label. Self-reported rows have a mark.
- The default view hides `contested` rows behind a toggle and shows a note; it never hides them silently.
- Task pages show, for each benchmark, the best reported result per comparable group with its date, the trend over time as a chart (12), and the methods and architectures used by the top rows, computed from the model cards' `architecture` and `uses` edges.
- Model pages show the reverse: all results of the model, grouped by benchmark version and setting.
- The tables are datasets too: `/benchmarks/<slug>/<version>/results.json` and the Parquet export of all results (16.10).

## 10.8 Saturation and contamination

### 10.8.1 Saturation (computed)

A benchmark version is **saturated** in a comparable group when the build finds, as of the build date (rule M11):

- the best `reported` or better score is within the version's ceiling margin: within the estimated label-noise rate of the version (from a source in `known_issues`), or within 2 points of percentage of the ceiling when no estimate exists; or
- the human baseline is passed by at least 3 models from at least 2 developers, and the spread of the top 5 scores is smaller than the typical 95% interval of the metric on that test size.

The benchmark page shows "Saturated as of <date>, by rule 10.8.1 (a|b)" in the facts box. It is computed, never written (T05). A saturated benchmark stays in the catalogue and in history; the task page lists it under "Saturated benchmarks".

### 10.8.2 Contamination

- **Reports, not guesses.** A contamination claim about a model and a benchmark is a claim (07.2) with sources: a study that found test items in the training data, a developer's own decontamination report, or a canary-string test. `registers/contamination.yaml` holds them, and the result cards of the affected pairs get `flags: [contamination-reported]` with the source (rule M12). The site never says that a model is contaminated without a source.
- **Release-date flag.** The build marks a result `post-release` when the model's training data cut-off (a fact, 04.9) is later than the benchmark version's release. It is shown as a neutral fact ("Training data cut-off after the benchmark's release"), not as a finding of contamination.
- **Our own experiments** never train on test splits (rule M07).

## 10.9 Human baselines and ceilings

Each benchmark version may have `human_baseline` and `ceiling` facts with sources: who the humans were (crowd workers, experts, the authors), how many, and the protocol. The tables print the baseline with its description, never as a bare line labelled "human".

## 10.10 Ratings and arenas

Preference ratings (Elo, Bradley-Terry) are results of metric `rating`, with the interval, the number of votes and the snapshot date. They are comparable only inside one snapshot of one arena. The page says what the rating measures (preference of the voters of that arena in that period) and links the arena's methodology page.

## 10.11 Data facts: the D ladder

Model, dataset, benchmark, hardware, policy and system items carry facts (04.9). The D ladder (04.10) measures them. Each kind has a list of **required facts** in `schema/facts.v1.yaml`:

| Kind | Required facts |
|---|---|
| `model` | developer, release date, architecture, parameters (total, and active for mixtures), access, licence, modalities; for notable models (18.5) also training compute, training tokens, context length, training data description |
| `dataset` | size (items, and tokens or bytes), modality, source type, collection dates, licence, access, documentation link |
| `benchmark` version | release date, size, splits, metric, protocol, licence, data hash |
| `hardware` | vendor, release date, type, peak FLOP/s by precision, memory, memory bandwidth, power |
| `policy` | jurisdiction, type, dates (adoption, entry into force, application per part), official text link |
| `system` | developer, years, domain, status |

| Level | Needs | Rules |
|---|---|---|
| D0 | a name and a source | D02 |
| D1 | every required fact present, each with a source | D01, D03 |
| D2 | D1, and every fact cited to a primary source where one exists, with a locator; every disagreement between sources recorded as `disputed` with both values; every time-bound fact has `as_of` | D04, D05, D06, D07 |
| D3 | D2, and a reviewer checked the facts against the primary sources for this exact hash (15.5) | |

## 10.12 Who writes this

The results extractor role (14.2) reads tables from sources and writes candidate result cards with locators; the verifier role and the citation check confirm them; the curator places new benchmarks and tasks in the tree and writes their contracts; the experimenter writes harness runs. People review the contracts of core tasks and the protocol fields of core benchmarks.

## 10.13 The rules of this document

The rules are listed in 15.3 with their severities. In short:

**M: metrics and results**

| Rule | Checks |
|---|---|
| M01 | a result card has a model, a benchmark version, a metric node, a score with unit, a setting, and a source with a locator and a date |
| M02 | results attach to benchmark versions, never to tasks or unversioned benchmarks |
| M03 | setting keys that the source does not state are `unknown`, never filled by guess |
| M04 | a task contract's checker agrees with its metric nodes' test vectors on the task's cases |
| M05 | the node's metric implementation and the official scorer agree on every harness run |
| M06 | the harness runs only open-weight models whose licence allows evaluation, within the budget classes; never closed models or paid APIs |
| M07 | no experiment trains on a split marked as test |
| M08 | a result whose number the citation check did not find is not shown in any table |
| M09 | a computed table holds one comparable group per column, with the setting in the header |
| M10 | a computed table prints its date, population, filters and count |
| M11 | saturation is computed by 10.8.1 and never written by hand |
| M12 | a contamination flag has a source in `registers/contamination.yaml` |
| M13 | a metric node has a contract, official test vectors and the variants that change its score |
| M14 | a rating result names its arena, snapshot date, votes and interval, and is compared only inside its snapshot |

**D: data facts**

| Rule | Checks |
|---|---|
| D01 | no bare number for a fact field of 04.9; every fact has a value, a unit, and at least one source |
| D02 | every item has a name and at least one source |
| D03 | the required facts of the kind are present (10.11) |
| D04 | a primary source is cited where one exists, with a locator |
| D05 | sources that differ beyond the field's tolerance make the fact `disputed`, and the page shows both values |
| D06 | an `estimated` fact names its estimator and method |
| D07 | a time-bound fact has `as_of` |
| D08 | computed facts (ranks, "first", "largest") are produced by `aik facts compute`, with population and date |
| D09 | units follow `schema/units.yaml` (FLOP, parameters, tokens, bytes, FLOP/s by precision) |
| D10 | a fact that changed between harvests keeps its history in the card's `harvest` records |
