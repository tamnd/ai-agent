# 18. Coverage

"No missing" is the goal of 02.1, and it has to be a measurement, not a hope.
This document defines what the claim means, the coverage levels C0 to C6, the universe register and its outcomes, the sources that fill it, the scope rules that decide card or node, the demand signals, the coverage report and the scale.

## 18.1 The claim

The site makes this claim, and the coverage report (18.7) checks it every day:

> Every item about AI that a trusted source names has a row in the universe register with an outcome.
> Every row with outcome `node` or `card` has that node or card.
> Every gap is listed with its reason.

So "no missing" is three things:

1. **Nothing unknown.** The universe register holds every item that a source of 18.4 names, whether or not we write about it. An item that is not in the register is a failure of harvest, and the report counts the sources that we have not harvested.
2. **Nothing undecided.** Every row has an outcome (18.3). `pending` is allowed, but it is a gap that the report counts and ages.
3. **Nothing promised and absent.** A row that says `node` must have a node at the level that the milestone promised for it (21), and a row that says `card` must have a valid card.

The claim is about items, not about depth.
Depth is measured by the ladders of 04.10 (W, G, X, R, F, V, D), and the coverage level of an item (18.2) combines them.

What does not count as missing:

- items that the scope rules of 18.5 leave as universe rows only (a Hub repository with 3 downloads, a work with no citations and no link to anything we cover); they are known and decided;
- items owned by a neighbour (algorithm, math2, programming) that have a bridge card (5.9);
- items of the non-goals of 02.6.

## 18.2 Coverage levels C0 to C6

Each item has a computed coverage level.
As every ladder, it is computed by the build from the files, and nobody writes it (04.10).

| Level | Name | Condition |
|---|---|---|
| C0 | registered | a universe row with an outcome other than `pending` |
| C1 | carded | a valid card with its required facts at D1 (each fact has a source); for a node outcome, the card exists before the page |
| C2 | paged | a published node at W2 and G1: every required section of its kind is present and every claim has a source |
| C3 | grounded | W3 and G2; every claim of `claims.yaml` has a type and a computed label (07); every fact field at D2 (two sources, or one primary source with the status `primary-only`) |
| C4 | evidenced | the evidence that its kind can have: X2 in the tier-1 frameworks for methods, architectures and metrics with code; R1 for findings and methods with a small-scale claim; G3 (citation check passed on every quoted claim) |
| C5 | complete | every target of `schema/targets.v1.yaml` for its kind and level (for example: a core method has X3 in NumPy, PyTorch and JAX, R2 for its main claim, V1, and F1 when a Lean statement exists); for a benchmark, its results table is computed and its contamination note is present (10) |
| C6 | reviewed | C5, and a person reviewed it at its current hash: W5, and X4, R4 or D3 where the targets name them (14.11, 15.5) |

Rules:

- Card-only items stop at C1. Card-only kinds by default are `result`, `incident`, `work`, `exercise`, `provision` and `event` (04.2); they can reach C2 only when promoted to a node with `aik promote <id>`.
- An item with outcome `bridge` is complete at C1 when its bridge card is current (`aik bridge sync al`, 20.6).
- A level falls when a condition stops holding: a framework pin update that fails a test drops C4 to C3; a changed source snapshot that fails the citation check drops G3; a new hash without review drops C6 to C5.
- `schema/targets.v1.yaml` holds the targets per kind, level (5.6) and core flag. Changing it is a PR with a new version, and the report shows both versions for one release.

The page shows the coverage level next to the ladders, and the area pages show the counts by level.

## 18.3 The universe register

The universe register is `registers/universe/<top-area>.jsonl` in `tamnd/ai`, with one row per item (sharded as cards, 20.3), built by `aik universe merge` from the harvested inventories (F1, 03.5).

A row:

```json
{"id": "u:arxiv:1207.0580", "name": "Dropout", "kind": "method", "area": "deep-learning-training/regularization",
 "aliases": ["dropout regularization"], "same_as": {"wikidata": "Q25339462", "openalex": "W2095705004"},
 "sources": ["arxiv", "openalex", "pwc-dump:methods", "canon:goodfellow-2016:7.12"],
 "signals": {"canon_refs": 4, "citations": 41000, "hub_downloads": null, "search_misses": 0},
 "outcome": "node", "target": "ai://NEW-dropout", "reason": "core: 4 canon refs",
 "decided_by": "curator:claude-sonnet-5-5@m-2026.11", "decided": "2026-12-03", "core": true}
```

Outcomes:

| Outcome | Meaning | Required |
|---|---|---|
| `node` | it gets a written page | `target` is a node tag or a `NEW-<slug>` id |
| `card` | it gets a card and no page | `target` is a card id |
| `alias` | another item covers it | `target` is that item; the alias is in its `aliases` |
| `part` | a section of another page covers it (a variant, a hyperparameter, a trick) | `target` is the node and the section anchor |
| `bridge` | a neighbour owns it | a bridge card with `elsewhere:al|m2|pg` |
| `row-only` | known, below the scope rule | the rule id of 18.5 that excluded it |
| `out-of-scope` | not about AI (02.6) | a reason |
| `merged` | a duplicate row | `target` is the surviving row |
| `pending` | not yet decided | nothing; it ages in the report |

Rules:

- An outcome is set by the curator (14.3) or a person, never by the harvester. A harvester may only add rows as `pending` or update `sources` and `signals`.
- A `row-only` row is re-tested at every harvest, because signals change: a work that passes the citation threshold, a Hub model that passes the download threshold, or an item that a node starts to cite becomes `card` automatically (the rule is in code, so this is not a judgement).
- A change of outcome from `node` or `card` to anything else needs a person, so that nothing disappears silently.
- `same_as` ids are the join keys across sources: arXiv id, DOI, OpenAlex id, Wikidata QID, Epoch id, Hub repo id, AIID id, CELEX number, Semantic Scholar id, DBLP key, ORCID, ROR. Two rows with a shared id are merged by `aik universe merge`; conflicts go to the curator. A join that would put two rows of one source in one item is a conflict, since a source names a thing once, except for the sources of headings, canon and courses (13.2): two works or two sections that name one item are the canon match (14.3), so they join by the name and the kind.

## 18.4 Sources of the universe

Each source has an adapter (13.2) and a share of the universe.
Counts are from the research of 01 in November 2026 (Estimate where marked).

| Source | What it names | Size in November 2026 | Kinds it fills | Licence for our use (19) |
|---|---|---|---|---|
| Epoch AI, Data on AI Models | models with training compute, parameters, dates, developers | more than 3,700 models; about 800+ notable; frontier and large-scale (≥1e23 FLOP) flags | model, org, hardware facts | CC BY 4.0 |
| Hugging Face Hub | model and dataset repositories, model cards, licences, downloads | more than 3M models; 1.5% of models get 99.2% of downloads | model, dataset, software | metadata by API; model cards under their own licences, facts only |
| OpenAlex | works, authors, institutions, topics | AI subfield 1702: 4,063,595 works | work, person, org | CC0 |
| arXiv | preprints and their categories | 2025: cs.LG 46,005 and cs.AI 45,133 with cross-lists | work; new terms (18.6) | metadata CC0; texts by their licences, snapshots for the citation check only |
| Semantic Scholar, DBLP, OpenReview, ACL Anthology, CVF | works, venues, reviews, awards | Estimate: overlaps OpenAlex for 90%+. The ACL Anthology's ten main venues have 64,742 papers, 80% with a DOI, and 349 with a best, outstanding or test-of-time award (2026-11-14); DBLP's pages are behind a bot check, so only its dump (1.1 GB) is open | work, event, person | by source (13.2) |
| Papers with Code dump | methods, tasks, datasets, results, code links | frozen at the shutdown of 2025-07-24 | method, task, dataset, benchmark, result | CC BY-SA 4.0; facts and names only (19) |
| Wikidata | items with QIDs, people, orgs, events | Estimate: about 60k items under AI classes | every kind; `same_as` | CC0 |
| AI Incident Database | incidents and reports | highest id about 1,797 | incident, risk links | CC BY-SA 4.0; facts and our own summaries |
| OECD.AI policy database, EUR-Lex, the Federal Register, national gazettes | AI laws, policies, provisions | OECD.AI has 2,642 published initiatives in 107 jurisdictions, 544 of them instruments of S-L1 by their type (140 guidance documents, 135 frameworks, 87 national AI strategies, 61 laws, 58 regulations, 11 amendments, and the declarations, recommendations, directives, decisions and treaty of intergovernmental bodies), which the adapter gives the signal `instrument` (2026-11-14); the Federal Register has 1,352 documents with the phrase "artificial intelligence", 175 with an AI title, 11 of them executive orders (2026-11-14); the adapter gives a rule, a proposed rule or a presidential document the signal `instrument`; the Cellar of EUR-Lex has 288 works under the EuroVoc concept artificial intelligence, of which 235 are rows (53 corrigenda, case law and notices skipped) and 43 instruments of S-L1 by their type (the regulations, directives, decisions and recommendations of the Union, its agreements, and the proposals of acts), which the adapter gives the signal `instrument` (2026-11-15); the EU AI Act and the Digital Omnibus 2026/1744 in force | policy, provision, standard, org | official texts; public domain or reuse terms by jurisdiction |
| Standards bodies (ISO/IEC JTC 1/SC 42, NIST, CEN-CENELEC JTC 21, IEEE) | standards and their status | Estimate: about 200 | standard | metadata only |
| Leaderboards (HELM, LMArena, SWE-bench, ARC Prize, Epoch's benchmarking hub, the Open LLM Leaderboard archive) | results | Estimate: about 100k rows; SWE-bench has 323 entries in 5 leaderboards, 240 of them with one named model, for 99 models (2026-11-14) | result, benchmark | by source; facts only |
| Canon works (5.10) and frontier sources (`registers/surveys.yaml`) | named concepts, methods, theorems, findings | 19 canon works; Estimate: about 40 surveys | concept, method, theorem, finding, argument, open problem | names and locators only |
| Courses and syllabi (5.7) | topics in reading order | Estimate: about 30 courses | concept, method; paths | names only |
| GitHub | software repositories, stars, licences | 1,977 repositories with an AI topic and 5,000 stars (2026-11-14) | software | metadata by API |
| Award lists (Turing Award, the test-of-time and best paper awards of the main venues) | people, works | Estimate: about 1,500 entries | work, person, event | facts |

A source that we know of and have not harvested is listed in the report as a gap of type `unharvested`, with its estimated size.
A source that spec 13 gives no adapter, because the harvester cannot read it under the rules of 13.1 (a challenge, or a key that the project does not hold), is listed as `closed` with the reason instead (`coverage.ClosedSources`): OpenReview and Semantic Scholar on 2026-11-15. When such a source opens, it loses the entry and gets an adapter.

## 18.5 Scope rules

The scope rules decide, in code, the default outcome of a row from its signals.
They are in `registers/scope.yaml`, each with an id, so a `row-only` row names the rule that excluded it.
`aik universe merge` applies a rule to a `pending` row of its kind: a rule with a source and no thresholds to each row of the source; a rule with thresholds when a signal of the row meets one of them (10,000 downloads or 100 likes), and, if the rule names a source, the row has a row of that source. The adapters write the signals: `downloads_30d` and `likes` (Hub), `citation_percentile` (OpenAlex), `stars` (GitHub).
A curator or a person can override a rule for one item with a reason.

**Models.**

- S-M1: every model in Epoch AI's database is a card (more than 3,700).
- S-M2: a Hub repository is a card when it is a base or instruction model (not a quantization, merge or fine-tune of a carded model) and has at least 10,000 downloads in the last 30 days or 100 likes (Default, Q-06). This selects at most about 20,000 repositories (Estimate); the first harvest (2026-11-14) found about 11,000 models over the thresholds, of which 8,047 are cards. Quantizations, merges, adapters and fine-tunes of a carded model are listed on the card of their base model as counts, not as cards; a fine-tune by the owner of its base (the developer's instruction model) and a derivative with more downloads than its base stay cards (`aik harvest huggingface`).
- S-M3: every model that a developer in `registers/watch.yaml` announces on its own pages is a card, even when the weights are closed.
- S-M4: a model is a **node** when it is notable: Epoch marks it notable or frontier (top 5 by training compute at release), or it introduced a method or architecture that has a node, or a canon or frontier source names it, or a family node needs it (Default, Q-04). About 2,000 model nodes at M5, most of them family nodes (04.2) with members as cards.
- Total at M5: about 25,000 model cards.

**Works.**

- S-W1: a work of OpenAlex subfield 1702 (or of the arXiv categories of `crosswalk/arxiv.yaml`) is a card when it is in the top 2% by citations for its year and subfield (Default, Q-07).
- S-W2: a work cited by any node, card, result or fact is a card, whatever its citations.
- S-W3: a work that won a best paper, an outstanding paper or a test-of-time award at a venue of `registers/venues.yaml` is a card. The adapters of venues write the signal `awards`, the number of such awards of the work (not a runner-up, an area chair's highlight or a people's choice), so the rule is the threshold `awards: 1`.
- S-W4: a work is a node only as a canon work (5.10), or when a person promotes it (`aik promote`) because the work itself is the subject (for example "Attention Is All You Need"). About 600 work nodes.
- Total at M5: about 200,000 work cards. The other works of the subfield (about 4 million) stay `row-only`; search still finds them by id (17).

**Datasets and benchmarks.**

- S-D1: every dataset of the Papers with Code dump with at least one result, every dataset that a carded model card names as training data, and every Hub dataset with 1,000 downloads in the last 30 days or 50 likes is a card. About 15,000 (Estimate); the first harvest (2026-11-14) found 27,626 Hub datasets over the thresholds alone, so Q-06 should raise them if the cards are too many.
- S-D2: a dataset is a node when it is a benchmark's base, or a pretraining corpus of a notable model, or canon. About 800.
- S-B1: every benchmark with at least one result card is a card, about 8,000; a benchmark is a node when it has 20 or more results from 5 or more developers, or is used in a notable model's card, or is canon. About 1,500.

**Results.**

- S-R1: every result of a notable or carded model on a carded benchmark that a source reports with a locator is a result card (10.6). About 150,000.

**Incidents.**

- S-I1: every incident of the AI Incident Database is a card, about 1,800 now and about 2,500 at M5. Incidents are linked to risk nodes by `instance_of` (curated).

**Law and policy.**

- S-L1: every AI-specific act, regulation, executive order, guidance and treaty of the OECD.AI database and of the official journals in `registers/watch.yaml` is a policy card; each article or section that a node or another card cites is a provision card. Policies in force in the EU, the US (federal and the states with AI laws), China, the UK, Japan, Korea, Canada, India and Brazil, and international instruments (the Council of Europe convention, the OECD principles, the G7 Hiroshima code), are nodes. About 600 policy nodes and 12,000 policy and provision cards.
- S-L2: standards of `registers/standards.yaml` are cards; about 200 are nodes.

**People and organizations.**

- S-P1: a person is a card when they are an author of 5 or more carded works, or of a canon work, or of a notable model's technical report, or hold an award of `registers/awards.yaml`. About 40,000. A card holds only public professional facts (19, P family).
- S-P2: a person is a node when a history section of a node names their contribution and a person reviewer approves the page (Q-14). About 2,000.
- S-O1: an organization is a card when it develops a carded model, hosts 20 or more carded authors, issues a policy, or funds a notable model. About 10,000; about 800 nodes.

**Software and hardware.**

- S-S1: a software repository is a card when a node or a model card names it, or it has 5,000 GitHub stars and an AI topic. The first harvest (2026-11-14) found 1,977 repositories that are not forks over the threshold with one of the 53 topics of `crosswalk/github-topics.yaml` (87 searches), far under the estimate of 8,000, so the topic list is the lever if the cards are too few; about 800 nodes, with bridge cards to programming where it owns the item.
- S-H1: every accelerator in Epoch's hardware data and every chip that a notable model trained on is a card; about 200 nodes and 500 cards.

**Exercises and events.**

- S-E1: exercises come from courses of 5.7 and canon works as cards with a locator, never copied (19); about 6,000.
- S-V1: conferences, workshops, competitions and milestones (the Dartmouth workshop, ImageNet ILSVRC-2012, AlphaGo vs Lee Sedol) are cards; about 300 are nodes. About 2,000 event cards.

**Ideas.**

- S-X1: concepts, definitions, methods, architectures, tasks, metrics, theorems, findings, arguments, open problems and risks have no harvest threshold: every one that a canon work, a frontier source, a course, or two independent works name is a node (5.10). One that only one work names is a `part` of the page that it varies, or a card when it has a name that people search for.

## 18.6 Demand signals

The universe is filled from sources, but readers and the field show where it is thin.
Each signal adds a `pending` row or raises the priority of an existing one in the queue (14.5).

| Signal | How it is measured | What it does |
|---|---|---|
| search misses | queries with no result or no click, by count per week (17.9) | a `pending` row when a query names an item; the term goes to the cataloguer |
| rising terms | n-grams whose frequency in arXiv titles and abstracts grows fast over 90 days and match no item or alias | a `pending` row with the works that use the term |
| broken links | inbound links and `[[...]]` links to slugs that do not exist | a row and a queue item |
| reader reports | "missing" reports of 16.19 | a row; a reply when it is decided |
| citations in our own pages | an item named in a node that has no card | a row; S-W2 makes it a card |
| neighbour gaps | a bridge card whose target was removed or renamed | a bridge refresh |
| agent demand | MCP and API lookups that fail (16) | as search misses |

## 18.7 The coverage report

`aik coverage -json` computes the report on every build; the site shows it at `/about/coverage/`, and the release notes keep a copy.

It contains:

- **The universe:** rows by source, by outcome and by top area; the `pending` rows and their age; the `unharvested` sources.
- **The promises:** rows with outcome `node` and no node, or below the milestone's target; rows with outcome `card` and no valid card.
- **The levels:** counts of C0 to C6 by area, kind and core flag, and the change since the last release.
- **The core:** the core items (5.10) and their levels; a core item below C3 at M3 is a failed promise.
- **Freshness:** cards whose sources were harvested more than their refresh period ago; facts past `as_of` plus their period; the delay between a source's new item and its card (the one-day promise of 14.9).
- **Gaps by type:** `unharvested`, `pending`, `missing-node`, `missing-card`, `below-target`, `stale`, `broken-bridge`, `demand` (from 18.6).
- **Canon coverage:** for each canon work and each course, the share of its named items with a node at C2 or higher. The rows of a work are the rows with a source `canon:<work>:...` and those of a course the rows with a source `courses:<course>:...` (13.2, our own registers).

The report fails the build only on promises that a person made (a `node` row of the core below its milestone target after the milestone date); everything else is shown, not blocked.
The C family of rules (15.3) holds the copy and licence checks, not coverage; coverage is a report, not a lint.

## 18.8 Scale

The estimates of 02.4 broken down by kind, at M5 (Estimate; M1 measures them on 3 top areas).

**Nodes, about 32,000:**

| Kind | Nodes | Kind | Nodes |
|---|---|---|---|
| concepts and definitions | 8,000 | models (mostly families) | 2,000 |
| methods | 6,000 | benchmarks | 1,500 |
| architectures | 1,500 | datasets | 800 |
| tasks | 2,500 | systems | 500 |
| metrics | 600 | software | 800 |
| theorems | 1,200 | hardware | 200 |
| findings | 500 | policies | 600 |
| arguments | 200 | standards | 200 |
| open problems | 300 | people | 2,000 |
| risks | 600 | organizations | 800 |
| canon works | 600 | events | 300 |
| areas (52 top, about 615 sub) | 667 | | |

**Cards, about 480,000:**

| Kind | Cards | Kind | Cards |
|---|---|---|---|
| works | 200,000 | organizations | 10,000 |
| results | 150,000 | software | 8,000 |
| people | 40,000 | exercises | 6,000 |
| models | 25,000 | incidents | 2,500 |
| datasets | 15,000 | events | 2,000 |
| policies and provisions | 12,000 | standards | 1,000 |
| benchmarks | 8,000 | hardware | 500 |

Plus about 3,000 bridge cards (5.9), about 9,000 implementations, about 2,500 experiments and about 600 Lean theorems (02.4).

**The universe** at M5: about 5 million rows (Estimate), most of them `row-only` works and Hub repositories. At about 400 bytes per row compressed in JSONL, the register is about 2 GB, so the `row-only` rows of works and Hub repositories live in R2 `ai-data` as Parquet, and only the rows with other outcomes are in git (20.3).

**How fast it must grow.** From M1 to M5 the corpus grows by about 1,500 nodes and 18,000 cards a month (21); the field adds about 8,000 AI works a month to arXiv (01), of which about 2% pass S-W1, so the daily job (14.9) adds about 160 work cards a month from new works and the rest come from the backlog.
