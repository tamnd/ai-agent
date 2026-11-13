# 14. Agents and the pipeline

Most of the writing is done by AI agents, under the checks of 15 and the review of people.
This document defines what we copy from `algorithm-work`, the roles, the pipeline from an empty sub-area to a merged pull request and a live page, the models, the work queue, the writer loop with its gate and audit, run records, budgets, the daily freshness job, and the shipping scripts.

The design follows one lesson of algorithm and programming: **writing was never the bottleneck; the gates were** (01.6).
Algorithm went from 0 to 8,396 stubs and 3,617 written pages in 7 days with about 1,850 lines of Python in `algorithm-work`.
So this project copies those scripts nearly line for line, and spends its own effort on the AI-specific gates: claims, the citation check, facts, implementations across frameworks and experiments.

## 14.1 What we copy from algorithm-work

`tamnd/ai-agent/work/` holds the pipeline from day one (03.1).
Each script keeps the name, the command line and the state files of its algorithm original, so that a person who ran algorithm can run this one.
Every check goes through `aik`, so there is one gate for people, agents and CI.

| algorithm-work | What it did there | In `ai-agent/work/` |
|---|---|---|
| `lib.py` | model backends (`run_codex`, `run_claude`, `run_model`), the event log `logs/events.jsonl`, JSON extraction, the rate-limit backoff for the other family | the same; backends read `registers/models.yaml` (14.4) instead of hard-coded names; each call logs tokens and cost |
| `catalog.py` | one model call per sub-area lists every item whose primary area is that sub-area; writes `catalog/raw/<top>--<sub>.json` | the same, but the input is the harvested universe rows of the sub-area, and the model is asked only for what is missing (14.3 step 1) |
| `curate.py` | normalize, group exact and near duplicates, a model decides each group, drop existing nodes and bad edge targets; writes `catalog/curated.jsonl` | the same, plus the universe outcomes (18.3), the card or node decision (04.1) and the border test with bridge cards (5.9) |
| `canon.py` | lists the named items of each canon reference, one model call per section; `catalog/canon/<ref>--<n>.json` | the same, over the 19 canon works of 5.10 and the frontier sources (`registers/surveys.yaml`) |
| `match.py` | matches the canon inventories to the catalog; `canon_refs.json`, `additions.jsonl` | the same |
| `enrich.py` | reviews canon additions, drops the ones that do not deserve a page, writes summaries | the same |
| `finalize.py` | merges additions, sets `core` when two or more canon refs name an item, writes `topics/core.yaml` | the same rule (5.10), plus the frontier core; writes `registers/core.yaml` with the reason per item |
| `stub.py` | writes a skeleton node per curated item, then mints tags | `aik new` in bulk for node outcomes and `aik card sync` for card outcomes; `aik mint` |
| `kinds.py` | per kind: what the writer must produce and which gold pages to read | the same, for the 28 kinds of 04.2, with the gold set of 21 M0 |
| `writer.py` | the ladder of writer models, the gate, the cross-family audit, fix rounds, `write_prov` | the same loop (14.6); the gate adds the AI checks |
| `gate.py` | runs the writer gate on named slugs | the same, calling `aik gate` |
| `audit.py` | an audit by a reader of another family, `state/audits/<slug>.json` | the same JSON (14.6, 15.4) |
| `prov.py` | appends provenance lines for each file | the same format (04.8) |
| `pr.py` | opens a PR from files of the writer tree, labels, milestone, `--merge`, waits for CI, reports | the same; one PR per sub-area (14.10) |
| `ship.sh` | prov, then `pr.py --merge`, then deploy, then `report.py tick`, with 3 tries | the same (14.10) |
| `deploy.sh` | deploys `origin/main`, waits for disk space, retries until the live page shows the commit hash | the same live check against `https://ai.tamnd.com/dropout/` (14.10) |
| `report.py` | ticks checklist lines in issues, comments, cuts releases | the same |
| `mkarea.py` | creates the area milestone, the area issue and one issue per sub-area with page checklists | the same (14.10) |
| `prompts/*.md` | `writer.md`, `fix.md`, `audit.md`, one brief per top area, `opus-brief.md` for interactive sessions | versioned files `work/prompts/<role>/v<n>.md`, one area brief per top area of 05.2 |

What changes, and why:

- **Harvest first.** Algorithm's catalog asked a model to recall a closed field. AI has structured sources for most artifacts (13.2), and model recall of models, scores and dates is the main source of invented facts. So artifacts come only from harvest; the model's catalog adds ideas (concepts, methods, findings, arguments) and names a source for each.
- **Facts never come from a model.** A writer may not put a number of 04.9 into front matter or prose unless it is in a fact record with a source in the raw store. The gate enforces it (D family).
- **More gate steps.** The claim extraction, the citation check, the framework tests and the `cpu-small` experiments are part of the gate (14.6).
- **The state is in the repository.** Algorithm kept `state/` and `catalog/` in the private work directory. Here `work/state/` is still local, but every decision that matters (outcomes, core, levels) ends in a register of `tamnd/ai` with a pull request.

## 14.2 Roles

| Role | Input | Output | Model class (14.4) |
|---|---|---|---|
| **cataloguer** | a sub-area, its universe rows, the existing nodes and cards | the list of items that belong there, with kind, a one-line note and a named source for each item that the universe does not have | large |
| **curator** | raw catalogs, universe rows, matcher scores | merged items with primary area, level, form (card or node), outcome, aliases, edges | medium, large for disputed groups |
| **harvester** | an adapter's raw snapshot (13.2) | cards and universe rows; no model, or a small model for field extraction from semi-structured pages (a model card's README, a law's article list) | none or small |
| **writer** | a node stub, its cards and facts, the gold pages of its kind, the area brief | `node.md`, derivations, `claims.yaml`, `same_as`, edges | ladder (14.6) |
| **coder** | a method, architecture or metric node | `impl/<framework>/` with tests, `shapes.yaml`, metric vectors (08) | medium, large for the hard kinds |
| **experimenter** | a node with a claim that has a small-scale form (09.2) | `exp/<name>/` with `experiment.yaml`, `run.py`, `expect.yaml`; runs `aik exp run` at `cpu-small` | medium |
| **verifier** | a node at W3 | for each claim of `claims.yaml`, a verdict from the raw source snapshot: supported, unsupported, contradicted, with the quote (07.5, 13.6) | large, of another family than the writer |
| **auditor** | a node that passes the gate | the audit JSON of 14.6 | of another family than the writer |
| **fixer** | a gate report or an audit JSON | the edits that fix it, in the node's files only | the writer's current rung |
| **reviewer-assistant** | a node queued for person review | a review brief: the claims with their labels and sources, the diff since the last review, the facts with disagreements, the questions a reviewer should ask; never a verdict | medium |

Rules:

- The verifier and the auditor never use the model family of any writer of the node (15.4), as `auditor_for` in algorithm's `writer.py`.
- No role writes a ladder level, a status, an evidence label or a `results.json` by hand (04.10, 04.11).
- A role edits only the files of the item it was given; the gate fails a run that touched other paths.

## 14.3 The catalog phase: from universe to stubs

One unit of work is a **sub-area**, as in algorithm.
`mkarea.py` makes the GitHub milestone and issues of a top area first (14.10).
The catalog phase then runs once per top area, in this order:

```
harvest ─▶ universe ─▶ catalog ─▶ curate ─▶ canon ─▶ match ─▶ enrich ─▶ finalize ─▶ stub + mint ─▶ (write phase, 14.6)
```

1. **Catalog** (`python3 work/catalog.py --only <top>/<sub>`). For each sub-area, the cataloguer gets the tree (05.2), the universe rows mapped to the sub-area, the existing nodes, cards and bridge cards, and the area brief. It returns a JSON list: `{name, kind, slug, note, aliases, prereqs, source}`. Rows from the universe are kept as they are; the model's additions must each name a source (a work, a canon section, a URL) and enter the universe as `pending` with that source. Two catalog runs with models of different families must agree; an item that only one run lists goes to the curator with a flag (the two-model rule of programming, Default).
2. **Curate** (`curate.py`). Normalize names, group exact and near duplicates (same normalized key, an alias of another item, the same `same_as` id), and let a model decide each group: one item survives with its primary area, the others become aliases. Then, for each survivor: the form (card or node, by the defaults of 04.2 and the scope rules of 18.5), the level (5.6), the outcome in the universe register, and the border test of 5.9 (an algorithm-shaped item becomes a bridge card with `elsewhere:algorithm`).
3. **Canon** (`canon.py`). For each canon work of 5.10, one model call per chapter or section lists the named items with kind and locator, from the table of contents and the index in `registers/canon.yaml` (names only, 19.3). The frontier sources (the current AI Index, the surveys of `registers/surveys.yaml`, the courses of 5.7) are listed the same way.
4. **Match** (`match.py`). Match the canon lists to the curated catalog by name, alias and `same_as`, with a model deciding doubtful pairs (cached in `match_decisions.json`). Output: `canon_refs.json` (slug to the refs that name it) and `additions.jsonl` (canon items that the catalog does not have).
5. **Enrich** (`enrich.py`). A model reviews the additions: drop those that are not items (a chapter title, an example), merge those that are aliases, write a summary and a home area for the rest.
6. **Finalize** (`finalize.py merge`). Merge the additions into the catalog and set `core: true` when **two or more canon works name the item**, or when the frontier-core rule holds (5.10). `finalize.py core` writes `registers/core.yaml` with the refs as the reason.
7. **Stub and mint** (`stub.py`). For each `node` outcome without a node, `aik new <kind> <slug> -area <top>/<sub>` writes the directory with the sections of its kind (06.3) and stub markers; for each `card` outcome, `aik card sync` writes or updates the card. `aik mint` gives tags to every `NEW-` id. The stubs are committed but not published (04.1); the cards are published.

The phase ends with one PR per top area that carries the stubs, the cards, the universe outcomes and `registers/core.yaml` changes.
A person approves the core list and the tree changes (21 M1).

## 14.4 Models

`registers/models.yaml` pins the models by class and by role, as `algorithm-work` pinned them in its command lines.
In November 2026:

```yaml
version: m-2026.11
classes:
  large:  [claude-opus-5-5, claude-fable-5-2]
  medium: [claude-sonnet-5-5]
  small:  [claude-haiku-5-5]
  other:                                   # a second family, for audits, verification and the first writer rung
    writer: codex:gpt-6-luna               # as in algorithm-work
    auditor: codex:gpt-6-sol
writer_ladder: [other.writer, claude-haiku-5-5, claude-sonnet-5-5, claude-opus-5-5]
writer_ladder_hard: [claude-sonnet-5-5, claude-opus-5-5, claude-fable-5-2]   # theorem, argument, open-problem, policy, finding (Default)
auditor_for:                               # never the writer's family
  claude-*: other.auditor
  codex:*: claude-sonnet-5-5
  fallback_if_other_unavailable: claude-opus-5-5   # only when the writer was not Claude; else the item waits
verifier: other.auditor
cataloguer: [claude-opus-5-5, other.auditor]     # the two-model rule (14.3)
curator: claude-sonnet-5-5
harvester_extract: claude-haiku-5-5
coder: claude-sonnet-5-5
experimenter: claude-sonnet-5-5
reviewer_assistant: claude-sonnet-5-5
```

Rules:

- A change of `models.yaml` is a pull request with a new `version`. Provenance records the version, the model id and the prompt version of every change (04.8), so a page written with an old model can be found and requeued.
- Algorithm's `auditor_for` gave the audit to Claude Opus when the other family was rate-limited. That broke the cross-family rule for Claude-written pages. Here the item waits instead, unless the writer was of the other family (Default, Q-08).
- Prompts are versioned files `work/prompts/<role>/v<n>.md`. Every writer prompt carries the style rules of 06.2 and the prohibitions of 04.11, as algorithm's did ("plain English, no em or en dashes, never invent page or theorem numbers, link only to slugs that exist"), and these new ones:
  - never write a number that changes or that sources disagree on outside a fact record (04.9);
  - never write "state of the art", "currently", "the best", "recently" or a rank in prose (K04);
  - never state a position on a contested question in the site's voice (N01);
  - never cite a source that is not in the raw store or the given source list; ask for it with a `needs-source` note instead;
  - never put a private fact about a living person on a page (P01).
- The writer gets as context: the stub, the cards and facts of the item, the bridge cards it may link, the list of existing slugs (as `slugs.tsv` in algorithm), the gold pages of its kind (14.6), the area brief, and the source snapshots that the universe row names.

## 14.5 The work queue

The queue is computed, as in algorithm and programming, and stored in `work/state/queue.jsonl` with a lease per item.
`aik queue` builds it from:

- universe rows with outcome `node` and no node at W1 (18.3);
- nodes below the target level of the current milestone (21), by ladder: W, G, X, R, F, V, D;
- claims labelled `reported` on core nodes where a cheap experiment could raise them to `measured-here` (07, 09.2);
- facts with status `disputed` that a primary source could settle (04.9);
- result cards that fail an M rule (10.6);
- new items from the daily job (14.9);
- search queries with no result (17.9), and the "missing" signals of 18.6;
- reader reports (16.19);
- regressions: an implementation that fails after a framework pin update, an experiment whose `expect.yaml` fails after a data pin change, a citation whose snapshot changed.

Priority: the core first (5.10), then the milestone targets, then demand (18.6), then age.
Within a sub-area, items go in reading order (5.7), so prerequisites are written first and later pages can link them.
The queue is published at `/about/queue/` so that people can take items, as in programming.

## 14.6 The write phase: writer loop, gate and audit

`python3 work/writer.py --queue --limit 50 --jobs 6` (or `--slugs …`) runs this loop for each node.
It is algorithm's `process()` with the same counters, and the AI gate.

```
pick rung 0 of the ladder ─▶ write ─▶ gate ──fail──▶ fix-gate (same rung) ─┐
                               ▲        │                                  │ after 2 failed fixes: climb a rung
                               │        └──pass──▶ audit (other family)    │ after 4: failed
                               │                     │                      │
                               │          fail ◀─────┴─────▶ pass, no blocker or major
                               │            │                    │
                               └─ fix-audit ┘ (after 2 failed    ├─ minor issues: one fix-minor round, gate again
                                  audits: climb; after 4: failed)└─▶ done ─▶ write_prov
```

**The writer prompt** (`work/prompts/writer/v1.md`) is built from `kinds.py`: the item (slug, name, kind, area, level, core note, tag, catalog hints), the node directory, what to read first (the content standard 06, the gold pages of the kind, 07 for claims, 08 for code), what to write for the kind (the sections of 06.3), the style rules, and the front matter rules.
A resumed item keeps its draft: the writer is not called again when the page already has three or more sections, as in algorithm.

**The gold pages.** `kinds.py` reads `registers/gold.yaml` (06.12) and names the gold page of each kind, for example `method: dropout`, `architecture: transformer`, `metric: bleu`, `theorem: pac-bound-finite-hypothesis-class`, `finding: neural-scaling-laws`, `argument: chinese-room`, `model: llama`, `policy: eu-ai-act`, `risk: reward-hacking`. From M1, a second page per kind that reached W5 can be added to the list, so that the writer sees two examples.
As in algorithm, the gold pages are the quality bar, and the audit prompt tells the auditor to read the gold page of the kind.

**The gate** (`aik gate <slug> -json`, called by `gate()` in `writer.py`). Cheap checks first, so they fail first:

1. `aik fmt <slug>` succeeds (front matter, markup, one sentence per line).
2. `aik check <slug> -json` returns no error: schema, links and slugs that exist (E family), sections of the kind (S family), sources in the right form (G01 to G04), fact records for every fact field (D family), no forbidden phrases (K family), no position in the site's voice on a node marked contested (N family).
3. No stub marker is left in any file (the stub texts that `aik new` writes).
4. "In plain words" has at most 150 words and no formula longer than one term.
5. No tab, form feed, carriage return, em dash or en dash in any text file, as in algorithm.
6. `aik claims extract <slug>` finds every claim sentence of the page in `claims.yaml`, and every claim has a type and at least one source (07.3).
7. The tests of each framework present pass in their images: `aik test <slug> -framework numpy|pytorch|jax` (X1, X2), including gradient checks and metric vectors (08).
8. `aik exp run <slug> -class cpu-small` for experiments of that class, and their `expect.yaml` passes (R2, 09).
9. `aik cite <slug>` finds each quoted claim in its source snapshot (G05, 07.5). This runs only at the end of a write, not on each fix, because it is the slowest check that needs no model.

A gate failure returns a JSON list of problems; `fix_prompt()` gives them to the fixer with the path of the original writer prompt ("Fix every problem below, in the files of that directory only. Do not rewrite parts that are right. Do not weaken a test or delete a correct section to make a problem go away.").

**The audit** (`work/prompts/audit/v1.md`). A strict reviewer of another family reads every file of the node directory and the gold page of its kind, without editing, and checks in this order of importance:

1. **Facts and claims.** Every claim is true and its type is right; every fact matches its fact record; names, years, people and works in History are right (an invented attribution is a blocker); every source looks real (an invented DOI, arXiv id or locator is a blocker); no claim states more than its source shows (a result at one scale written as general is major).
2. **Derivations.** Each derivation proves what it states, with no gap; notation follows the register (06.4).
3. **Code and experiments.** The NumPy reference follows the equations of the page with the same names; tests are meaningful (shapes, the worked example, gradient checks, edge cases, an independent oracle, never the code against itself); the experiment tests the claim it names, with enough seeds, and the page does not present it as a reproduction of the full result.
4. **Completeness.** Every required section of the kind is present and substantial; nothing that the canon says about the item is missing (the canon refs are given to the auditor); the standard variants, pitfalls and failure cases are there.
5. **Neutrality and dates.** No position in the site's voice on a contested question; every time-bound statement has a date; no "state of the art".
6. **Style.** Plain English, one sentence per line, "In plain words" in limits, terms linked on first use, no filler.

The reply is only a JSON object, the same as algorithm's:

```json
{"verdict": "pass" | "fail", "score": 1-10,
 "issues": [{"severity": "blocker" | "major" | "minor", "file": "...", "quote": "exact short quote",
             "problem": "what is wrong", "fix": "what the right text or code is"}]}
```

"fail" when there is any blocker or major issue, or when the score is below 8.
Blockers: a false statement, a wrong derivation, a bug, an invented source or attribution, a fact without a source, a position in the site's voice.
Major: a missing required part, a gap in a derivation, a misleading explanation, an overclaimed result, a missing date.
Every issue must quote the text and give the fix; taste is not reported.

**Counters, as in algorithm.** At most 6 rounds plus 4 for gate fixes. Two failed gate fixes climb one rung; more than four gate failures end the item as `failed`. Two failed audits climb one rung; four end it as `failed`. A pass with only minor issues gets one cheap `fix-minor` round and the gate again. A `failed` item goes to the person queue with its last gate report and its last audit, and is not retried until a person changes the item or the prompt version.

**The verifier** runs after the audit passes, for nodes that are going to W3 or more: it reads `claims.yaml` and the raw snapshots and gives each claim a verdict (15.4). A contradicted claim sends the node back to `fix-audit` with the verifier's quote as the issue. The verifier's verdicts are inputs to the evidence labels (07), never the labels themselves.

**Coders and experimenters** run the same loop on their own files, with their own gate subset (steps 1, 2, 5, 7 or 8) and their own audit prompt (`audit-code/v1.md`, `audit-exp/v1.md`).

## 14.7 Run records and provenance

- **Events.** Every model call appends a line to `work/logs/events.jsonl`: time, job, slug, role, backend, model, ok, seconds, input and output tokens, cost, and `limited` when the backend was rate-limited, as `lib.log` did.
- **Transcripts.** Every prompt and reply is kept in `work/logs/runs/<slug>/<time>-<role>-<n>.txt`, as `record()` did, and uploaded to R2 `ai-runs/agents/<run-id>/` at the end of a batch. They are not public, because they may hold source snapshots that we may not redistribute (19.3).
- **Item state.** `work/state/<slug>.json` holds the status (`todo`, `writing`, `done`, `failed`, `blocked`), the history of calls, the audits with verdict and score, the last gate report and the final writer and auditor.
- **Run records.** Each batch writes `prov/runs/<run-id>.yaml` in `tamnd/ai`: the run id, the date, the `models.yaml` and prompt versions, the items with their final status, the totals of tokens and cost by model, and the gate and audit pass rates.
- **Provenance.** On `done`, `write_prov()` appends one line per file of the node directory to `prov/<tag>.jsonl` (04.8): date, file, op, `sha256` hash, author (`{agent: writer, models: [...], harness: "ai-agent/work/writer.py", prompt: "writer/v1", run: <run-id>}`), the auditor, `reviewer: null`, and the source ids that the writer was given. `prov.py` writes the same lines for interactive sessions, with a `--note`, as in algorithm.

## 14.8 Budgets

Every run is metered; there is no unmetered path in the plan, even though most of algorithm was written in interactive sessions (about $73 metered over 957 API events, 01.6).

- **Per item.** `writer.py` stops an item when its cost passes the item cap of its kind (Default: $3 for most kinds, $8 for theorems, arguments, open problems and policies, $1 for promoted cards), and marks it `blocked: budget`.
- **Per batch.** `--budget USD` stops the batch when the sum passes it. The default batch is one sub-area.
- **Per milestone.** Each milestone has a model line, a GPU line and a hosting line (21). `aik report cost` sums `events.jsonl` and the GPU bills by milestone and by role. The queue stops at the line until a person raises it.
- **Measured, then set.** The first lines are set from the gold set's measured cost per node at the end of M0, then reset at the start of each milestone from the measured cost of the previous one. Prices come from the `claude-api` reference at the time of the run and from the other vendor's price list; no price is fixed in this spec.
- **What costs most** (Estimate): writing and auditing 32,000 nodes (the main cost); the verifier over about 2 million claims; coders for 9,000 implementations; experimenters for 2,500 experiments. Harvest costs little, because most sources are structured.

## 14.9 The daily freshness job and other scheduled jobs

AI changes daily. The promise of 02.3 point 8 is that a new model, benchmark, result, incident or law that a trusted source lists becomes a card within one day.

**The daily job** (`daily.yml`, 20.5, 06:00 UTC):

| Step | Source (13.2) | Output |
|---|---|---|
| new models | Epoch AI database diff; the Hub's new and trending models over the scope rule (18.5); the release pages and model cards of the developers in `registers/watch.yaml` | model cards with facts; family membership; a queue item when the model meets the notable rule (18.5) |
| new works | the arXiv listings of the categories in `crosswalk/arxiv.yaml`; OpenReview decisions; the ACL Anthology and CVF additions | work cards for works that the scope rule selects (18.5) or that a model card, a result or a node names; the rest only as universe rows |
| new results | the results tables of the new works and model cards; the leaderboards in `registers/leaderboards.yaml` (HELM, LMArena, the Open LLM Leaderboard archive, SWE-bench, ARC Prize, Epoch's benchmarking hub) | result cards with setting and locator (10.6); the computed tables of the touched benchmark versions |
| new benchmarks and datasets | the works above that introduce one; the Hub's new datasets over the threshold | benchmark and dataset cards |
| new incidents | the AI Incident Database new ids | incident cards with `instance_of` proposals for the curator |
| new law and policy | EUR-Lex for AI acts and implementing acts, the Federal Register, the US state trackers, the OECD.AI policy database, the official sites of the safety institutes and regulators in `registers/watch.yaml` | policy and provision cards; status recomputed from dates (04.5) |
| time-bound facts | every fact with `as_of` older than its refresh period | recomputed facts and computed tables (04.9) |
| new terms | n-grams that rise fast in arXiv titles and abstracts and match no item (18.6) | universe rows `pending`, for the cataloguer |

The job opens one PR per source, merged automatically when `aik check` passes and no fact of an existing card changed by more than its tolerance; otherwise a person merges it.
A card made by the daily job shows its harvest date, so a reader sees how fresh it is.

**Other scheduled jobs:**

| Job | When | What |
|---|---|---|
| harvest refresh | weekly per source, or the source's own feed | re-harvest; diff the cards; changed facts go to review |
| framework refresh | when a framework of 08.2 publishes a release | build and pin the image; run all tests of that framework; new failures go to the queue |
| experiment refresh | monthly | F6 (03.5): experiments whose inputs or pins changed |
| link check | weekly | every external URL; dead links go to the queue with an archive fallback |
| citation recheck | monthly | re-run the citation check against fresh snapshots; changed or vanished quotes go to the queue |
| bridge refresh | at each release of a neighbour | `aik bridge sync al|m2|pg` (20.6) |
| audit sweep | daily | the full audit; a PR when the report changes |

## 14.10 Shipping: pr, ship, deploy, report, mkarea

The scripts are algorithm's, with the names of this project.

- **`mkarea.py <top>`** creates the GitHub milestone `Area: <Name>`, one area issue with the sub-area list, and one issue per sub-area with a checklist line per page (`- [ ] /dropout/ (method, core)`), sorted by level and reading order. `--dry` prints them.
- **`pr.py`** opens one PR per sub-area from the files of the writer tree: `--slugs` copies the node directories, their cards and their `prov/` files into a clean checkout of `origin/main` (`../ai-pr`, as `../algorithm-pr`), runs `aik check` on them, commits, pushes the branch, and opens the PR with labels (`type: node`, `area: <top>`, `kind: <kind>`, `priority: p0` for the core) and the milestone of the area. The body carries the gate and audit summary table: each node with its ladders before and after, the writer and auditor models, the audit score. `--merge` squash-merges after the local gate, as algorithm did (the local `aik check` is the gate; CI confirms after the merge).
- **`ship.sh <issue> <branch> <title> <bodyfile> <prov> <slugs…>`** writes provenance when asked, runs `pr.py --merge` with up to 3 tries on a fresh checkout, runs `deploy.sh` when the deploy inside `pr.py` failed, and on success runs `report.py tick <issue>` for each page. On failure it deletes the remote branch, waits, and tries again; after 3 failures it prints `SHIP FAILED <branch>`.
- **`deploy.sh`** checks out `origin/main`, waits until the disk has at least 2 GB free, runs `aik site deploy -out build/site -commit $H` with the Cloudflare token read from `$HOME/data/.env` with `set -a` (never printed), removes `build/`, and then **checks the live site**: the gold page `https://ai.tamnd.com/dropout/` must show `commit/$H`. It retries up to 6 times and prints `deployed $H` or `DEPLOY FAILED`.
- **`report.py`** ticks checklist lines that contain the page URLs (`tick 39 /dropout/ /lora/`), posts comments, cuts releases (`release 2027.03 --notes-file n.md`), and prints the next patch version.

## 14.11 People

People are editors, reviewers and contributors, as in algorithm and programming.

- **Editors** own a group or a top area: they approve the area tree, the core list, the scope rules, and the position lists of contested topics (06.9).
- **Reviewers** own a subject: they review nodes to W5, implementations to X4, experiments to R4, Lean bindings to F1, facts to D3. A review counts only for the hash it reviewed (04.10). Reviewers of contested topics are chosen so that each position of a page is checked by a reviewer who can state it fairly (Q-19).
- **Contributors** are anyone, through pull requests, with DCO sign-off.
- `registers/reviewers.yaml` lists reviewers and subjects. The reviewer-assistant (14.2) prepares each review, so a person spends their time on judgement, not on finding the sources.
