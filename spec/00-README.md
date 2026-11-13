# Spec 2150ai: ai

**All human knowledge about artificial intelligence in one place, with no gaps.** Every idea, method, architecture, model, task, benchmark, dataset, result, system, risk, law, argument, person and event of AI has a page or a card. Every equation is derived or cited. Every reference implementation is tested in NumPy, PyTorch and JAX. Every empirical claim carries a computed evidence label and a date. Every benchmark number is traced to its source. Every gap is measured. Everything is published at https://ai.tamnd.com.

- Repositories: `tamnd/ai` (content) and `tamnd/ai-agent` (the Go toolchain, binary `aik`, and the `work/` pipeline scripts). This copies the architecture of `tamnd/algorithm` and `tamnd/algorithm-agent` (spec 2150), including the pipeline of `algorithm-work` that actually wrote algorithm, and the changes that programming (spec 2150p) made after algorithm's first year.
- Date: 2026-11-13. Status: draft, research complete.

## Documents

| # | Document | What it settles |
|---|---|---|
| 00 | README | this index, the settled decisions, the conventions, the name registry |
| 01 | [research](01-research.md) | the state of the field in November 2026: the size of AI knowledge, the sources and their licences, what algorithm and programming taught us, the existing sites, the numbers behind the design |
| 02 | [the goal](02-the-goal.md) | the one sentence, what counts, the border, the quality promise, the scale (about 32,000 nodes and 480,000 cards), the readers, the non-goals |
| 03 | [architecture](03-architecture.md) | the two repositories and `work/`, the stores, the `aik` packages, the seven flows, deployment |
| 04 | [object model](04-object-model.md) | nodes and cards, the 28 kinds, tags, slugs and URIs, the node directory, front matter, cards, edges, provenance, facts, the computed ladders W G X R F V D |
| 05 | [taxonomy](05-taxonomy.md) | the 9 groups, 52 top areas and 615 sub-areas, the task index, the catalogues, facets, levels, paths and courses, crosswalks, the border with algorithm, math2 and programming, the core list |
| 06 | [content standard](06-content-standard.md) | the sections of each kind of page, the style, the notation register, derivations, worked examples, dated facts in prose, contested topics, the writing ladder W0 to W5 |
| 07 | [claims and evidence](07-claims-and-evidence.md) | claims as typed objects, the evidence labels (proved, measured here, replicated, reported, contested), how they are computed, the citation check, contested questions, the rules K |
| 08 | [implementations](08-implementations.md) | the framework register and tiers, NumPy from scratch as the reference, conventions, pinned package sets, shape contracts, gradient checks, differential tests, metric test vectors, the ladder X0 to X4 |
| 09 | [experiments and reproduction](09-experiments-and-reproduction.md) | the experiment as the AI analogue of a proof, configs, seeds, data pins, budget classes, statistical tests, the GPU pool, results, the ladder R0 to R4 |
| 10 | [tasks, benchmarks and results](10-tasks-benchmarks-and-results.md) | task contracts, benchmark versions, harnesses, the results register that replaces Papers with Code, settings and comparability, computed tables, saturation, contamination |
| 11 | [formal](11-formal.md) | the `AI` Lean package, the pins, what is formalized (learning theory, probability, RL, logic), the ladder F0 to F3 |
| 12 | [visualization](12-visualization.md) | scenes for architectures and training, explorables, the in-browser notebooks, charts of the registers, accessibility, the ladder V0 to V3 |
| 13 | [sources and harvest](13-sources-and-harvest.md) | the source register, the adapters on the `*-cli` tools, the raw store, the inventory, the canon, snapshots for the citation check, the no-copy rule |
| 14 | [agents and pipeline](14-agents-and-pipeline.md) | the roles, the `work/` scripts copied from algorithm-work, the model ladder, the gate, the cross-family audit, budgets, the daily job, provenance |
| 15 | [quality and audit](15-quality-and-audit.md) | badges, the audit rule families T E S G K X R M F V D C P N L, the audit JSON, human review, sampling |
| 16 | [site](16-site.md) | the UI and UX: routes, page layouts for each kind, catalogues, comparison and lineage views, calculators, paths, the agent surface, performance, hosting |
| 17 | [search](17-search.md) | the index, fields, the Cmd-K palette, identifier search (arXiv ids, DOIs, Hub repos), "I need to…" search, ranking, evaluation |
| 18 | [coverage](18-coverage.md) | what "no missing" means, the universe register, outcomes, scope rules for models, works, datasets and incidents, the levels C0 to C6 |
| 19 | [legal and licensing](19-legal-and-licensing.md) | licence zones, source classes, people and privacy, AI-generated text, model and dataset licences, takedown |
| 20 | [repository layout](20-repository-layout.md) | the trees of both repositories, card sharding, the `aik` command tree, the workflows, releases |
| 21 | [milestones](21-milestones.md) | M0 (2026-12) to M5 (2028-12) with exit criteria and costs (about $54k in total, Estimate) |
| 22 | [open questions](22-open-questions.md) | the decisions still to make, each with a default |

Reading order: start with 02, 03 and 04.
Then read 05 and 06 (where a page sits and what it is), 07 (the central idea: claims and evidence), 08, 09, 10, 11 and 12 (the five kinds of evidence: code, experiments, results, proofs and pictures), 13, 14 and 18 (how the corpus gets complete), 15 (how it stays correct), 16 and 17 (how people and agents use it), and 19 to 22 (how to build it).

## Settled decisions

1. **Copy algorithm, then focus on writing.** The repositories, the Go toolchain, the tags, the node directory, the computed ladders, the gate and the per-sub-area pipeline of `algorithm-work` are copied, adapted and not reinvented (03, 14). The effort of the project goes into content.
2. **Cards first, nodes where they matter.** AI has millions of named artifacts. Every item is first a card with checked facts and sources; it becomes a node with a written page when the scope and core rules say so (04.1, 18.5). No stub pages are published.
3. **Harvest first.** The universe of items is harvested from structured sources (Epoch AI, the Hub, OpenAlex, arXiv, Wikidata, the Papers with Code dump, the AI Incident Database, official legal texts) before any page is written (13, 18).
4. **Claims are typed objects, and evidence labels are computed.** Every factual sentence is a claim with a type and sources; its label (proved, measured here, replicated, reported, contested) is computed by the build and never written (07). Prose never says "state of the art" or "currently" (K04).
5. **Facts have sources and dates.** Numbers that change or that sources disagree on are facts with sources, a status and an `as_of` date (04.9). Ranks and "firsts" are computed.
6. **Five kinds of evidence, each with a ladder.** Derivations and Lean proofs (F), reference implementations (X), experiments (R), results with sources (D and the results register), and visualizations (V), plus writing (W) and grounding (G). Nobody writes a level by hand (04.10).
7. **NumPy from scratch is the reference.** Each method, architecture and metric with code has a NumPy implementation written from first principles, and PyTorch and JAX versions that must agree with it (08). Metrics reproduce the official test vectors.
8. **Experiments are the AI analogue of proofs.** A method or finding whose claim has a small-scale form has an experiment with seeds, pinned data, a budget class and a statistical test (09). A small-scale reproduction is never presented as a reproduction of the full result.
9. **One results register.** Benchmark results are result cards (model, benchmark version, setting, score, source, date) with computed comparability (10.6). This replaces the leaderboards of Papers with Code, which shut down on 2025-07-24 (01.2).
10. **Neutral on contested questions.** Safety, policy, philosophy and open scientific debates are written as positions with their sources, never in the site's own voice (06.9, N01).
11. **Four wings, one library.** algorithm, math2 and programming own their items. This site shows them as bridge cards and writes the AI view only when it adds facts (5.9).
12. **Lean v4.35.0 and Mathlib, pinned as in algorithm.** The `AI` package formalizes learning theory, probability for ML, RL theory and logic for AI (11).
13. **Workers and R2 from day one.** The site is served by the Cloudflare Worker `ai-site` with R2 buckets `ai-site`, `ai-raw`, `ai-data` and `ai-runs` (03.7, 16.20).
14. **Agents write, checks gate, people review.** Agents write most of the text and code under the gate and a cross-family audit; every page shows what an agent wrote, what checks passed and what a person reviewed (14, 15.5).
15. **No copying.** Prose, code and figures are written fresh and cite their sources. The copy check compares each change with the source corpora (13.7, 19).
16. **"No missing" is measured.** Every item that a trusted source names has an outcome in the universe register, and the coverage report shows each gap (18).

## Conventions

- Section references look like 07.4 (document 07, section 4). Audit rules look like K04 (family K, rule 4), defined in 15.3.
- Tags have 6 characters of Crockford base32, for example `5TQH2W`. URIs look like `ai://5TQH2W`. Slugs look like `dropout`. Neighbour links look like `[[al:k-means-clustering]]`, `[[m2:kl-divergence]]`, `[[pg:numpy]]`.
- "Estimate" marks a number from research that M0 to M1 must measure. "Default" marks a decision of 22 that can still change.
- The words are simple English. In content and in this spec, one sentence on each line in Markdown source where practical, as in math2 and algorithm. No em dashes or en dashes in content (06.2).
- Dates are ISO 8601. Model ids of agents are pinned in `registers/models.yaml` (14.4).

## Name registry

These names are fixed for all documents.

| Thing | Names |
|---|---|
| Site, repos, binary | ai.tamnd.com; `tamnd/ai`; `tamnd/ai-agent`; `aik` (Q-01); `work/` |
| Stores | Worker `ai-site` (the name `ai` is taken by another Worker on the account); R2 `ai-site`, `ai-raw`, `ai-data`, `ai-runs`; D1 `ai-search` for full-text search from M3 (17.11) |
| Lean | package `AI`, modules `AI.<Area>.<Name>` |
| Ladders | W0–W5 writing; G0–G3 grounding; X0–X4 execution per framework; R0–R4 reproduction; F0–F3 formal; V0–V3 visualization; D0–D3 data facts; C0–C6 coverage |
| Evidence labels | `proved`, `measured-here`, `replicated`, `reported`, `contested` (07) |
| Fact status | `agreed`, `primary-only`, `disputed`, `estimated` (04.9) |
| Rule families (15.3) | T structure; E edges; S style and sections; G grounding and citations; K claims and time; X implementations; R experiments; M metrics and results; F formal; V visualization; D data facts; C copying and licences; P people and privacy; N neutrality; L links |
| Frameworks (08.2) | tier 1: `numpy`, `pytorch`, `jax`; tier 2: `c`, `cpp`, `rust`, `julia`, `typescript` |
| Budget classes (09.4) | `cpu-small`, `gpu-small`, `gpu-medium` |
| Milestones (21) | M0 2026-12, M1 2027-03, M2 2027-07, M3 2027-12, M4 2028-06, M5 2028-12 |
