# 01. Research: the state of the field, November 2026

This document is the evidence for the design.
The research was done between 2026-11-09 and 2026-11-13.
Each section ends with what it means for ai.tamnd.com.
A number marked "Estimate" is to be measured in M0 to M1 (21).
Counts are as published by each source on or near the date given; they move every week, and the harvest (13) re-measures them.

## 1.1 What exists, in one paragraph

Knowledge about AI is larger, faster and more scattered than knowledge about algorithms or programming.
It lives in about 100,000 new AI papers a year on arXiv alone (Estimate from 1.2), in a few great textbooks under copyright, in lab blogs and technical reports that change after release, in model cards on the Hugging Face Hub, in structured databases run by small teams (Epoch AI, the AI Incident Database, the MIT AI Risk Repository), in annual reports (the Stanford AI Index), in Wikipedia, in official legal texts, and in forums whose best posts are never indexed by topic.
The one site that tied tasks, datasets, methods and benchmark results together, Papers with Code, shut down on 2025-07-24.
No site gives, for each idea of the field, a derivation, tested code, a reproduction, a dated evidence label and a link to every result that bears on it.

**What it means.** The project is a library, not a news feed: it harvests the structured sources first (13), turns them into cards with sources and dates (04.6, 04.9), and writes pages for the ideas that the cards cannot hold (04.1).

## 1.2 Counts: the size of AI knowledge

### Models

| Source | Count | Note |
|---|---|---|
| Hugging Face Hub | more than 3 million model repositories (August 2026) | 1.5% of the models receive 99.2% of the downloads; most repositories are fine-tunes, quantizations, merges, tests and empty repositories |
| Epoch AI, "Data on AI Models" | more than 3,700 models (database updated 2026-11-12) | about 800 or more marked "notable" by Epoch's criteria (state of the art, highly cited, historically significant or widely used); "frontier" means among the top 5 by training compute at release; "large-scale" means at least 1e23 FLOP of training compute |
| Stanford AI Index 2026 (published 2026-04-13) | about 95 notable models released in 2025, using Epoch's data | by country: United States 59, China 35, South Korea 8; industry made 91.6%; by organization: OpenAI 19, Google 12, Alibaba 11 |

**What it means.** A page for every Hub repository is impossible and useless.
The scope rule for model cards (18.5) starts from Epoch's whole database, adds Hub models above a download and like threshold and every base model that a notable model was fine-tuned from, and gives a node only to model families and notable releases.
The Estimate of 25,000 model cards at M5 (02.4) comes from: about 3,700 Epoch models, about 15,000 Hub models above the threshold (the 1.5% head of 3 million is about 45,000 repositories, most of them variants of one base model that collapse into one card with checkpoints), and about 6,000 historical and lab models named in works but in neither source.

### Papers and works

| Source | Count | Note |
|---|---|---|
| arXiv, 2025, counting cross-lists | cs.AI 45,133; cs.LG 46,005; cs.CL 23,753 | |
| arXiv, 2025, primary category only | cs.CV 27,739; cs.LG 25,747; cs.CL 16,942 | all of arXiv in 2025: 284,486 new submissions |
| OpenAlex | 4 domains, 26 fields, 252 subfields, 4,516 topics | subfield 1702 "Artificial Intelligence" has 4,063,595 works; computer vision and other AI-heavy subfields are separate |
| Semantic Scholar, DBLP, ACL Anthology, OpenReview, CVF open access, NeurIPS proceedings | to be measured at M0 | they give venues, citations, reviews and open versions; DBLP has the cleanest venue and author records |

**What it means.** A paper index is a non-goal (02.6).
Work cards exist for every work that a node cites, for the canon (5.10), for the works that the scope rule makes notable by citations and by venue awards (18.5), and for the work that introduced each item.
The Estimate of 200,000 work cards is about 5% of subfield 1702.

### Tasks, benchmarks and results

| Source | Count | Note |
|---|---|---|
| Papers with Code | shut down 2025-07-24; `paperswithcode.com` now redirects to Hugging Face "Trending Papers", which has no leaderboards | the final data dump stays on GitHub (`paperswithcode/paperswithcode-data`): papers with abstracts, links between papers and code, evaluation tables, methods, datasets; about 4,000 tasks, with many duplicates; licence CC BY-SA 4.0 |
| Epoch AI benchmarking hub | results of frontier models on a few dozen benchmarks that Epoch runs itself | the source of `measured` results by a third party |
| Hugging Face Hub | task taxonomy and pipeline tags; evaluation results in model card metadata (`model-index`) | self-reported by the uploader |
| lab technical reports and model cards | the main source of frontier results | self-reported, often with a setting that other reports do not use |

**What it means.** The results register (10.6) is needed because nothing else holds results with their settings.
The Papers with Code dump is the starting inventory of tasks, methods, datasets and historical results; its rows are imported as cards with `harvest.source: pwc-dump` and checked against their papers before they are shown (10.6).

### Datasets

The Hugging Face Hub lists several hundred thousand dataset repositories (Estimate; measured at M0 with the Hub API).
Kaggle, the UCI Machine Learning Repository, OpenML, Zenodo and the Papers with Code dump add more.
Many large training datasets have documented problems: LAION-5B was withdrawn in December 2023 after the Stanford Internet Observatory found links to child sexual abuse material, and was re-released as Re-LAION-5B in August 2024 after cleaning.

**What it means.** Dataset cards record source, size, licence, documentation and known issues as facts (04.5), and the project never mirrors web-scraped image datasets (19.8).

### Incidents and risks

| Source | Count | Note |
|---|---|---|
| AI Incident Database (AIID) | highest incident id about 1,797 at the end of October 2026; the October snapshot export had 1,713 incidents | ids are not dense; each incident has reports (news articles) and a taxonomy classification |
| MIT AI Risk Repository | a living database of risks extracted from published taxonomies, with a causal and a domain taxonomy | licence CC BY 4.0 (checked at M1) |
| MITRE ATLAS | tactics, techniques and case studies of attacks on AI systems | |
| OWASP Top 10 for LLM Applications | the ten main risk classes of LLM applications | CC BY-SA 4.0 |

**What it means.** Incidents are cards linked to risks with `instance_of` (04.7), and risk nodes cite the taxonomies that list them (04.5).

### Law and policy

| Fact | Value |
|---|---|
| EU AI Act | Regulation (EU) 2024/1689, in force 2024-08-01; prohibitions applied from 2025-02-02; general-purpose AI model obligations from 2025-08-02 |
| Digital Omnibus on AI | Regulation (EU) 2026/1744, in force 2026-07-27; moves the high-risk deadline for Annex III systems to 2027-12-02 and for Annex I (products) to 2028-08-02; Article 50 transparency obligations apply from 2026-08-02; adds a prohibition on AI systems for generating non-consensual intimate material and child sexual abuse material |
| other instruments | the Council of Europe Framework Convention on AI, the NIST AI RMF 1.0 and its Generative AI Profile (NIST AI 600-1), national AI laws and executive acts in the United States, China, the United Kingdom, South Korea, Japan and others; the OECD.AI policy observatory tracks more than a thousand national initiatives (Estimate, measured at M0) |

**What it means.** A law changes after it is passed, and its parts apply on different dates.
So `policy` nodes have dated parts and a computed status (04.5), each provision is a card with its own dates, and the daily job watches the official journals (14.9).

## 1.3 Outside classifications

| Scheme | What it is | Fit for our tree |
|---|---|---|
| ACM CCS 2012 | the ACM classification; "Computing methodologies › Artificial intelligence" and "› Machine learning" | stable but old: no node for transformers, diffusion, foundation models, alignment or agents; used as a crosswalk only (5.8) |
| CS2023 (ACM, IEEE-CS, AAAI) | the curriculum; the Artificial Intelligence knowledge area (AI) with its knowledge units, plus AI parts of Society, Ethics and the Profession | good for the teaching core and the paths; too coarse for reference use |
| arXiv categories | cs.AI, cs.LG, cs.CL, cs.CV, cs.NE, cs.RO, cs.MA, cs.IR, stat.ML, eess.AS and others | a fast signal of what is new; categories overlap and say nothing about ideas |
| OpenAlex topics | 4,516 topics made by clustering citations, assigned to 252 subfields | good for "missing" signals and citation counts; no hierarchy below subfield; names are machine-made |
| Wikipedia | "Outline of artificial intelligence", "Outline of machine learning", "Glossary of artificial intelligence", "Timeline of artificial intelligence", and the AI and ML category trees | the best free list of named concepts; uneven depth; text CC BY-SA |
| ISO/IEC 22989:2022 | AI concepts and terminology | names and term numbers only; the text is sold |
| Hugging Face task taxonomy | pipeline tags and tasks of the Hub | practical, modality-first; good for the task index |
| Papers with Code areas, tasks and methods | the final dump | wide; duplicated; no longer maintained |
| PhilPapers categories | "Philosophy of Artificial Intelligence" and its children | the philosophy sub-areas |

**What it means.** No outside scheme fits teaching, research and reference use together, so the tree is our own (05.1), with every scheme above kept as a crosswalk register (5.8) whose unmapped rows are gaps (18.4).

## 1.4 The inventory estimate behind the tree and the scale

The numbers of 02.4 and 05.2 come from this rough inventory, made by listing the named items of each outside list and removing obvious duplicates.
Each line is an Estimate to be measured by the harvest at M0 and M1 (18.8).

| Item | Source of the estimate | Before dedup | After dedup (Estimate) | Form |
|---|---|---|---|---|
| concepts and definitions | Wikipedia outlines, glossaries and category trees; ISO/IEC 22989 terms; NIST AI terms; the canon indexes (5.10) | about 30,000 | about 12,000 | about 7,000 nodes, the rest definition cards |
| methods and architectures | Papers with Code methods; canon indexes; survey works; arXiv titles with a named method above a citation threshold | about 25,000 | about 14,000 | about 9,000 nodes |
| tasks | Papers with Code tasks; Hub tasks; ACL and CVF topic lists | about 6,000 | about 2,500 | about 2,000 nodes |
| benchmarks and datasets | Papers with Code datasets; Hub datasets above a threshold; Epoch benchmarks; canon | about 40,000 | about 18,000 | about 2,500 nodes, the rest cards |
| metrics | canon; evaluation libraries; Papers with Code evaluation tables | about 1,500 | about 600 | nodes |
| theorems, findings, arguments, open problems | canon; learning theory texts; philosophy sources; surveys | about 4,000 | about 2,500 | nodes |
| models | 1.2 Models | | about 25,000 | about 2,000 nodes (families and notable releases), the rest cards |
| systems, software, hardware | Wikidata; GitHub topics; PyPI classifiers; Epoch hardware data; history sources | about 20,000 | about 9,000 | about 1,800 nodes |
| risks, incidents, policies, provisions, standards | MIT AI Risk Repository; AIID; OECD.AI; EUR-Lex; NIST; standards bodies' catalogues | about 30,000 | about 20,000 | about 1,200 nodes |
| people, organizations, events | Wikidata; awards; venues; canon; history sources | about 60,000 | about 30,000 | about 3,000 nodes |
| works | 1.2 Papers and works | | about 200,000 | about 600 nodes (the canon and landmark papers) |
| results | Papers with Code evaluation tables; Epoch; model cards; lab reports | about 400,000 rows | about 150,000 | cards |
| exercises | Kaggle; public course sites | | about 10,000 | cards |

Totals: about 32,000 nodes and about 480,000 cards (02.4).
The tree of 05.2 has 615 sub-areas, so a sub-area holds about 50 nodes on average, with model, benchmark and person nodes in a few large sub-areas.

## 1.5 Reference sites and what they lack

| Site | What we learn | What it lacks | Licence |
|---|---|---|---|
| Wikipedia | one article per named idea; outlines, glossaries and timelines; Wikidata ids | derivations and code are rare and untested; numbers without dates; uneven depth | CC BY-SA 4.0 |
| Papers with Code (closed) | the task, method, dataset and result graph | gone since 2025-07-24; results were self-reported with settings in free text | dump CC BY-SA 4.0 |
| Epoch AI | a curated model database with sources for each number, estimates with their method, hardware and benchmark data | ideas, derivations, code; only models | CC BY 4.0 |
| Hugging Face Hub and docs | model and dataset cards, licences, tasks, `model-index` results | cards are self-reported; no curation of ideas | per repository |
| Dive into Deep Learning (D2L) | one idea per section with runnable code in several frameworks | covers deep learning only; code tested by the book's CI, not across frameworks with a reference | text CC BY-SA 4.0; code under a permissive licence (checked at M1) |
| Distill | interactive explanations of a few ideas, reviewed | paused since 2021; a few dozen articles | CC BY 4.0 |
| Transformer Circuits, lab research blogs | the main record of interpretability and frontier research | not structured; change after publication | per post |
| Lilian Weng's blog, Jay Alammar's illustrated guides, Karpathy's lectures and repositories | clear long-form explanations with figures | personal, not exhaustive, not maintained as reference | all rights reserved unless stated |
| Stanford Encyclopedia of Philosophy | long, reviewed, versioned entries on AI and mind | philosophy only | all rights reserved; names only (19.3) |
| Stanford AI Index | the yearly numbers of the field | a report, not a reference; one year at a time | CC BY-ND 4.0 |
| OECD.AI | the policy observatory; national initiatives; incidents monitor | policy only | per dataset (checked at M1) |
| AI Incident Database | incidents with reports and taxonomies | incidents only | per snapshot terms (checked at M1) |
| Interpretable Machine Learning (Molnar), the Deep Learning book (Goodfellow et al.) online, course sites (CS229, CS231n, fast.ai) | good structure and order | under copyright or tied to one course | as stated per source |

**What it means.** Each piece exists somewhere, and no one joins them.
The project takes from each only what its licence allows (19.3), writes the rest fresh, and adds what none of them has: tested code across frameworks against a reference (08), small reproductions (09), the results register (10) and computed evidence labels (07).

## 1.6 What algorithm and programming taught

From `tamnd/algorithm`, `tamnd/algorithm-agent` and the private `algorithm-work` scripts (2150), and from the first design of programming (2150p):

| Fact | Value |
|---|---|
| algorithm nodes | 8,396 in 31 areas after the first week, of which 3,617 written pages |
| stubs | 4,779 stub nodes were published as pages, and readers saw thin pages |
| hosting | GitHub Pages failed at about 3,500 nodes (about 19,500 files) |
| ML and neighbours | algorithm holds 419 nodes in `machine-learning`, 209 in `games-and-search` and 325 in `logic-and-constraints` |
| what wrote algorithm | the Python scripts of `algorithm-work`: catalog (one model call per sub-area), curate, canon match, enrich, finalize (core = named in at least 2 canon works), stub and mint, writer, audit, pr, ship, deploy, report, mkarea; not the `algo agent` package that the spec described |
| the writer | a model ladder `codex:gpt-6-luna` → `claude:haiku` → `claude:sonnet`; climb one rung after repeated failure |
| the gate | `algo fmt`, `algo check -json`, no stub markers, "In plain words" at most 150 words, no tab, em dash or en dash, the Python tests pass |
| the audit | a model of another family returns JSON `{verdict, score, issues: [{severity: blocker or major or minor, file, quote, problem, fix}]}`; fix rounds until no blocker or major issue; provenance appended to `prov/<tag>.jsonl` |
| shipping | one pull request per sub-area with labels and a milestone, merged by `pr.py --merge`; `ship.sh`; `deploy.sh` with a live check; `report.py` ticks the tracking issues; `mkarea.py` makes the milestone and issues |
| the quality bar | a few gold pages (merge sort and others) that every prompt points to |
| programming's changes | cards for the long tail and nodes for ideas; harvest before writing; Workers and R2 from day one; `.md` and `.json` twins, `llms.txt`, MCP and datasets for agents; a verifier role that checks claims against raw snapshots; a universe register with outcomes and scope rules |

**What it means.**
Writing speed is not the limit; gates, evidence and review are.
The `algorithm-work` pipeline worked, so it is copied as `ai-agent/work/` (03.1, 14).
Cards exist so that no stub is published (04.1).
Hosting is a Worker with R2 from day one (03.7).
algorithm's ML, search and logic nodes become bridge cards at M0, and this site writes the AI view only where it adds facts (5.9).

## 1.7 Evidence, evaluation and reproduction in 2026

| Fact | What it means for us |
|---|---|
| Many published ML results do not replicate, or replicate only with the original code and a lucky setting; NeurIPS and other venues ask for reproducibility checklists, and the ML Reproducibility Challenge runs every year | a claim is not settled because one paper reports it: the evidence label `reported` is the default, and `replicated` needs independent works (07) |
| Benchmark results depend on prompts, shots, decoding, harness version and contamination; the same model's score on one benchmark differs by several points between reports | a result card records the setting, and the build compares only results with comparable settings (10.6) |
| Open evaluation harnesses (EleutherAI's lm-evaluation-harness, Stanford HELM, the UK AI Security Institute's Inspect, OpenAI's simple-evals and others) exist and can be pinned | the harness of 10.4 wraps pinned open harnesses for the results that we measure ourselves |
| Benchmarks saturate quickly; many 2023 benchmarks are near ceiling for frontier models | saturation is computed per benchmark version (10.8), and "best" is never written in prose (K04) |
| Small-scale versions of many classic findings run in minutes on one GPU (dropout, batch normalization, residual connections, double descent on small models, grokking on modular arithmetic, scaling curves of tiny language models) | experiments in budget classes `cpu-small`, `gpu-small` and `gpu-medium` (09.4) can check the qualitative claim of many method and finding pages |

## 1.8 Code in the browser and on the server

- NumPy runs in the browser through Pyodide, so the from-scratch reference implementations (08) can run in an in-browser notebook (12.8).
- ONNX Runtime Web, Transformers.js and WebGPU let small models run in the browser, so a page can show a tiny model working without a server (12.8).
- PyTorch and JAX do not run in the browser in a practical way; their implementations are tested in CI only, and the page shows their output from the build.
- Server-side execution is not offered in v1 (Q-09), as in programming.

## 1.9 Formal work

Lean 4 with Mathlib has probability theory, measure theory and the beginnings of statistics.
Formalizations of learning theory results (PAC bounds for finite classes, VC dimension facts, Rademacher bounds) and of MDP results exist as research projects in Lean, Isabelle and Coq, but no maintained library covers the canon of learning theory (Estimate: checked at M1 by a survey of the Lean Zulip, the Archive of Formal Proofs and GitHub).
algorithm pins Lean v4.35.0 with Mathlib, and the `AI` package uses the same pin (11).

## 1.10 Agents as readers

Readers now ask agents, and agents read the web.
Programming's research (2150p 01.7) found `llms.txt` on about 8.7% of the top 1,000 sites, MCP servers that serve documentation to coding agents, and Markdown served on `Accept: text/markdown` by Cloudflare.
AI is the subject that agents are asked about most often after code, and agents repeat stale or invented numbers.
So every page has `.md` and `.json` twins, the cards are published as datasets, and the MCP server answers with dated facts and evidence labels (16.10).

## 1.11 The design in one list

1. Cards for the long tail and nodes for ideas (04.1), because the field has millions of named artifacts.
2. Harvest first (13), because most facts are in structured databases and dumps.
3. Claims with computed evidence labels (07), because the field reports more than it replicates.
4. Facts with sources and dates (04.9), because AI numbers change and disagree.
5. Tested code against a NumPy reference (08), and small reproductions (09), because no site does either across the field.
6. A results register with settings (10.6), because Papers with Code is gone.
7. Neutral positions on contested questions (06.9), because the field's debates are live.
8. algorithm's pipeline and programming's hosting and agent surface (14, 16.20, 16.10).
