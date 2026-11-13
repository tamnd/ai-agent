# 21. Milestones

Six milestones, from M0 at the end of 2026 to M5 at the end of 2028.
Each one has a scope, a list of deliverables, exit criteria that `aik` can check, and a budget.
A milestone is done when every exit criterion holds on the release of its month, not when its date passes.
If a date passes first, the release ships with what holds, and the gap is listed in the release notes and in the coverage report (18.7).

The plan follows the lesson of algorithm (01.6): the pipeline of `algorithm-work` could write 3,617 pages in 7 days, and the gates failed first (the deploy, the audits, the stubs).
So M0 builds the gates and the gold set before any bulk writing, and every later milestone grows the corpus only as fast as the gates and the reviewers can follow.

## 21.1 Overview

| Milestone | Date | Theme | Nodes | Cards | Core at | Release |
|---|---|---|---|---|---|---|
| M0 | 2026-12 | the toolchain, the gates and the gold set | about 30 | about 10,000 | (core not yet set) | `2026.12` |
| M1 | 2027-03 | harvest everything; three pilot areas; the core list approved | about 1,500 | about 100,000 | C1 (all carded) | `2027.03` |
| M2 | 2027-07 | the core written | about 8,000 | about 220,000 | C2 | `2027.07` |
| M3 | 2027-12 | the core grounded and evidenced; all 52 top areas started | about 16,000 | about 330,000 | C3; C4 for core methods, architectures and metrics | `2027.12` |
| M4 | 2028-06 | breadth; the results register complete | about 25,000 | about 420,000 | C5 for half of the core | `2028.06` |
| M5 | 2028-12 | complete | about 32,000 | about 480,000 | C5; C6 for the gold set and 10% of the core | `2028.12` |

The node and card counts are Estimates (18.8), and M1 measures them on three pilot areas.
The growth from M1 to M5 is about 1,500 nodes and 18,000 cards a month.

## 21.2 M0: toolchain, gates and gold set (2026-11-13 to 2026-12-31)

**Scope.** Everything that a writer needs before it writes the first bulk page.

Deliverables:

1. **Repositories.** `tamnd/ai` and `tamnd/ai-agent` with the trees of 20.1 and 20.2, the licences, `CODEOWNERS`, and the workflows `check.yml`, `tags.yml`, `lean.yml`, `build.yml`, `deploy.yml`.
2. **`aik`, copied and adapted.** The packages of 03.4 that come from algorithm-agent and programming (`node`, `card`, `schema`, `tag`, `graph`, `taxonomy`, `markup`, `katex`, `lean`, `framework`, `runner`, `testkit`, `viz`, `status`, `audit`, `site`, `search`, `prov`, `release`), working on the new kinds. The new packages in a first version: `facts`, `claims`, `cite`, `exp` for `cpu-small`, `results`, `harvest`, `universe`, `coverage`, `queue`.
3. **`work/`, copied.** All scripts of 14.1 with their tests on recorded replies, calling `aik`. `writer.py` runs the loop of 14.6 end to end on one node. `ship.sh` and `deploy.sh` ship one PR to the live site.
4. **Schemas.** `node.v1`, `card.v1`, `facts.v1`, `claims.v1`, `experiment.v1`, `expect.v1`, `levels.v1`, `targets.v1`.
5. **Registers.** `topics/areas.yaml` with the 9 groups, 52 top areas and about 615 sub-areas of 05.2; `registers/canon.yaml` with the 19 canon works of 5.10; `registers/models.yaml` (14.4); `registers/frameworks.yaml` with the tier-1 images (08.2); `registers/notation.yaml` (06.4); `registers/scope.yaml` (18.5); `registers/watch.yaml`.
6. **Harvest of five sources** into `ai-raw` and the inventory: Epoch AI, the Papers with Code dump, the AI Incident Database, arXiv metadata of the categories of `crosswalk/arxiv.yaml`, and Wikidata. `aik universe merge` and `aik card sync` produce the first cards.
7. **Bridge cards.** `aik bridge sync al`, `m2` and `pg` for the items of 5.9.
8. **The gold set** (06.12), each page to W5 and to the top of each other ladder that applies to its kind, reviewed line by line by a person:

| Kind | Gold page | Kind | Gold page |
|---|---|---|---|
| method | `dropout` | dataset and benchmark | `imagenet`, `imagenet-ilsvrc-2012` |
| architecture | `transformer` | task | `image-classification` |
| metric | `bleu` | policy | `eu-ai-act` |
| concept and theorem | `pac-learning`, `pac-bound-finite-hypothesis-class` | system | `alphago` |
| finding | `neural-scaling-laws` | software and hardware | `pytorch`, `nvidia-h100` |
| argument | `chinese-room` | work | `vaswani-2017-attention` |
| open problem | `does-scaling-lead-to-general-intelligence` | person, organization, event | `alan-turing`, `deepmind`, `dartmouth-workshop` |
| risk | `reward-hacking` | model | `llama` (family) |

Plus gold cards, one for each card-only kind and for the card forms of the main kinds: a definition card (`logit`), a model card (Llama 3.1 405B), a result card (a Llama 3.1 405B score on MMLU with its setting), a benchmark card (MMLU), a dataset card (FineWeb), a work card, an incident card from the AI Incident Database, a provision card (EU AI Act Article 50), an event card, and an exercise card. And `q-learning` as the first page written by the pipeline (not by hand), as the test of the whole loop.

9. **The site.** The Worker `ai` with R2 `ai-site`, serving the gold set, the area pages, the collection pages of the M0 cards, search, the `.md` and `.json` twins, and `/about/coverage/`. The live check of `deploy.sh` passes.

**Exit criteria.**

- `aik check` and `aik build` pass on `main`; `aik site check` passes; the live site shows the commit.
- Every gold page has W5, G3, the ladders that apply at their top (X3 in the three tier-1 frameworks for `dropout`, `transformer` and `bleu`; R3 for `dropout`; F2 for `pac-bound-finite-hypothesis-class`; V2 for `transformer`; D3 for the facts of `llama`, `imagenet-ilsvrc-2012` and `nvidia-h100`).
- `q-learning` went through `writer.py` with the cross-family audit and reached W3 without a hand edit, and its run record is in `prov/runs/`.
- Cost per node measured on the gold set and on `q-learning`; the M1 budget lines reset from it (14.8).
- Release `2026.12`.

## 21.3 M1: harvest everything; three pilot areas (2027-01 to 2027-03)

**Scope.** The universe complete for every source of 18.4; the pipeline proven on three top areas; the core list approved.

Deliverables:

- All adapters of 18.4 in production, with weekly harvests; the universe register for all 52 top areas; every row with an outcome or counted as `pending`.
- The daily job of 14.9 live; the one-day delay measured.
- The catalog phase (14.3) on all 52 top areas, so that the core list (5.10) can be computed; `registers/core.yaml` approved by the editors.
- The write phase (14.6) on three pilot top areas, chosen to test different kinds (Default): one area of methods and architectures (`deep-learning-training`), one of evidence and results (`evaluation`), and one of argument and policy (`ai-safety-and-alignment` or `ai-governance-and-law`).
- The results register (10.6) loaded from the Papers with Code dump with the citation check (Q-35) and from the leaderboards of `registers/leaderboards.yaml`.
- Licence checks of the sources marked "checked at M1" (Q-23).
- The person review process (14.11): at least one reviewer for each of the 9 groups.

**Exit criteria.**

- Universe: no source of 18.4 is `unharvested`; `pending` rows under 5% of rows with an outcome.
- About 1,500 nodes, all in the pilot areas or the core, every published node at W2 or more; about 100,000 cards, all at D1.
- The core list approved and every core item at C1.
- Gate pass rate on the first try and audit pass rate measured by kind; the item caps and prompt versions reset.
- The scale of 18.8 re-estimated from the pilot areas, and this document updated.
- Release `2027.03`.

## 21.4 M2: the core written (2027-04 to 2027-07)

**Scope.** Every core item has a page; the evidence machinery runs at scale.

Deliverables:

- The write phase on every top area that holds core items, in reading order (5.7).
- Implementations at X2 in NumPy, PyTorch and JAX for every core method, architecture and metric with code (08).
- Experiments at `cpu-small` for every core finding and method with a small-scale form (09.2); the GPU pool (09.4) in use at the M2 cap of 09.10.
- The Lean package with the first learning-theory, probability and RL modules (11).
- The comparison tables and results tables of the core benchmarks computed (10.7); `/about/queue/` public.
- Courses (5.7) published for the first three paths.

**Exit criteria.**

- Every core item at C2 or more (about 4,000).
- About 8,000 nodes and about 220,000 cards.
- No claim on a core page without a type and a source (G1); the citation check run on all core pages.
- Release `2027.07`.

## 21.5 M3: the core grounded and evidenced (2027-08 to 2027-12)

**Scope.** Depth on the core; breadth started in every top area.

Deliverables:

- Every core item at C3: W3, G2, labels computed on every claim, D2 on every fact field.
- Every core method, architecture and metric at C4: X2 in the tier-1 frameworks, R1 where a small-scale form exists, G3.
- Tier-2 frameworks for the core architectures and metrics (Q-36).
- `gpu-medium` experiments at the cap of 09.10; the GPU provider revisited (Q-32).
- The write phase started in all 52 top areas.
- About 300 Lean theorems (11).

**Exit criteria.**

- Core at C3, and the core methods, architectures and metrics at C4.
- About 16,000 nodes and 330,000 cards.
- A core item below C3 after this date is a failed promise (18.7), and the build reports it.
- Release `2027.12`.

## 21.6 M4: breadth and the results register (2028-01 to 2028-06)

**Scope.** Every area deep; results complete.

Deliverables:

- Every sub-area with at least the items that its canon and frontier sources name at C2.
- The results register with every result of S-R1 (18.5), the computed tables of every benchmark node, and the contamination and saturation notes (10.8).
- The "I need to…" search (17) and the comparison pages (16.5) on all core pairs.
- Half of the core at C5 (the targets of `schema/targets.v1.yaml`).
- About 450 Lean theorems.

**Exit criteria.**

- About 25,000 nodes and 420,000 cards.
- Half of the core at C5; no `pending` row older than 90 days.
- Release `2028.06`.

## 21.7 M5: complete (2028-07 to 2028-12)

**Scope.** The claim of 18.1 holds.

Deliverables:

- Every row with outcome `node` at the target of its kind and level; every row with outcome `card` valid.
- About 9,000 implementations, about 2,500 experiments and about 600 Lean theorems (02.4).
- The gold set and at least 10% of the core reviewed by people at their current hashes (C6).
- The datasets of 16.10 at each release, on the site and on the Hub.
- The decision on languages other than English (Q-17).

**Exit criteria.**

- About 32,000 nodes and 480,000 cards (18.8), with the counts of the last measurement.
- The core at C5; the coverage report has no failed promise.
- Release `2028.12`.

## 21.8 Cost Estimates

All costs are Estimates; M0 measures the cost per node, and each milestone resets its lines from the measured cost of the previous one (14.8).

| Line | M0 | M1 | M2 | M3 | M4 | M5 | Total |
|---|---|---|---|---|---|---|---|
| months | 1.5 | 3 | 4 | 5 | 6 | 6 | 25.5 |
| model APIs (writers, auditors, verifiers, coders, experimenters, catalog) | $500 | $2,000 | $8,000 | $12,000 | $12,000 | $10,000 | $44,500 |
| GPU pool (09.10) | $150 | $300 | $1,000 | $2,000 | $2,400 | $2,400 | $8,250 |
| hosting: Workers Paid and R2 `ai-site` (03.7) | $10 | $20 | $40 | $60 | $90 | $120 | $340 |
| R2 `ai-raw`, `ai-data`, `ai-runs` (03.7) | $15 | $60 | $100 | $150 | $200 | $240 | $765 |
| CI and the build server | the server exists | | | | | | $0 |
| **Total** | **$675** | **$2,380** | **$9,140** | **$14,210** | **$14,690** | **$12,760** | **$53,855** |

How the model line is estimated: 32,000 nodes at about $1 to $1.50 each for the write, gate and audit loop (algorithm's metered pages cost less than $0.10 each, but AI pages have claims, facts and code, and we pay for the cross-family audit and the verifier on every node); about 9,000 implementations at about $0.50; about 2,500 experiments at about $1 for the experimenter; and the catalog phase. The line is high in M2 to M4, when most nodes are written, and lower in M5, when the work is mainly fixes and reviews.

The time of people (editors and reviewers) is not in this table. It is the scarce resource, and 14.11 plans for it.

## 21.9 Risks

| Risk | Effect | What we do |
|---|---|---|
| Models invent facts, scores, dates and sources | false pages that look right | facts only from fact records (04.9, D family); the citation check (07.5); the cross-family audit and verifier (14.6); invented sources are blockers |
| The gates fail at scale, as algorithm's deploy did | a stalled pipeline or a broken site | Workers and R2 from day one (03.7); the live check of `deploy.sh`; M1 tests the pipeline on three areas before M2 writes the core |
| Model costs pass the lines | the milestone slips | metered runs, item caps and batch caps (14.8); budgets reset from measured cost |
| The other model family is rate-limited or unavailable | audits wait | items wait instead of breaking the cross-family rule (Q-08); a third family as a fallback |
| Too few reviewers | C6 stays low; contested pages unbalanced | the reviewer-assistant (14.2); review targets only for the gold set and a sample of the core (C6 at M5); review by subject in `registers/reviewers.yaml` |
| A source changes its licence or closes, as Papers with Code did | lost data or a takedown | the raw store keeps snapshots (13.6); licence classes per field (19.2); facts and names only from class D sources |
| Bias on contested topics | the site takes a side | positions as data (06.9), rule N01, the reviewer rules of 15.5 and Q-19 |
| Hazardous content in security and safety pages | harm | `hazard: review` and person review before publishing (Q-24) |
| The field moves faster than the pipeline | stale pages | the daily job (14.9); facts with `as_of`; no "state of the art" in prose (K04) |
| The content repository grows too large for git | slow clones and CI | card sharding and Parquet in R2 for `row-only` rows (20.3); the size alarm |
| Framework releases break tests | X levels fall | pinned images; `frameworks.yml` opens a PR only when tests pass (20.5) |
| Privacy of living people | harm and takedowns | public professional facts only (P family, 19); person nodes need a reviewer (Q-14) |
