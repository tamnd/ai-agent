# 03. Architecture

The architecture copies algorithm (2150/03) and keeps the changes that programming (2150p/03) made after algorithm's first year.
Where this document differs from algorithm, it says so and says why.

## 3.1 Two repositories, and the work directory

| Repository | Holds | Licence |
|---|---|---|
| `tamnd/ai` | the content: node directories (text, derivations, implementations, experiments, claims, scenes), the card registers, the `AI` Lean package, the registers, the topic files, the manifests, the reports | prose CC BY-SA 4.0; implementations, tests, experiments and scenes MIT; Lean Apache-2.0; cards and metadata CC0 1.0 where the sources allow it, otherwise the licence of the source, per field (19.2) |
| `tamnd/ai-agent` | the toolchain: the Go binary `aik` (check, build, test, exp, lean, viz, site, search, harvest, universe, coverage, agent, mcp), the visualization runtime `@aik/viz`, the runner images for each framework, the prompts, and `work/`, the pipeline scripts (14) | Apache-2.0; `@aik/viz` MIT |

The split is the one of algorithm and math2.
Content changes are pull requests on `tamnd/ai`, and the checks are the gate.
Toolchain changes are releases of `tamnd/ai-agent`, and `manifests/releases/next.yaml` in the content repository pins the version of `aik` that CI uses.

**The work directory.** algorithm was written by the Python scripts of `algorithm-work` (catalog, curate, canon, enrich, finalize, mint, writer, audit, pr, ship, deploy, report, mkarea), not by the `algo agent` package that its spec described (01.6).
Those scripts worked, and they are the part of algorithm that this project copies most closely.
So `tamnd/ai-agent/work/` holds them from day one, as a first-class part of the toolchain, with tests and a version, instead of a private directory next to the repositories.
The scripts call `aik` for every check, so there is one gate.
Porting a script to Go is allowed when the Go version passes the same tests; it is never required (Default, Q-05).

## 3.2 The boxes

```
                     ┌───────────────────────────────────────────────────────┐
 sources (13) ─────▶ │ harvest: adapters on *-cli → raw store (ai-raw)       │
                     │   → inventory → universe register → cards (04.6)      │
                     └───────────────┬───────────────────────────────────────┘
                                     │ gaps (18), new items (14.9)
                                     ▼
 people ──PR──▶ ┌──────────────────────────────────────────────────────────┐ ◀──PR── agents (14)
                │ tamnd/ai: nodes/, cards/, lean/, registers/, topics/     │         (work/ scripts)
                └───┬───────────┬──────────────┬──────────────┬────────────┘
                    │           │              │              │
                    ▼           ▼              ▼              ▼
          ┌──────────────┐ ┌──────────┐ ┌──────────────┐ ┌──────────────────┐
          │ check, audit,│ │ lean     │ │ test: impl   │ │ exp: experiments │
          │ claims, cite │ │ build    │ │ per framework│ │ by budget class  │
          │ (07, 15)     │ │ (11)     │ │ (08)         │ │ (09) → ai-runs   │
          └──────┬───────┘ └────┬─────┘ └──────┬───────┘ └────────┬─────────┘
                 └──────────────┴───────┬──────┴──────────────────┘
                                        ▼
                     ┌──────────────────────────────────────┐
                     │ build: graph, facts, results tables, │
                     │ ladders, coverage, search index,     │
                     │ scene bundles, frames, datasets      │
                     └──────────────────┬───────────────────┘
                                        ▼
                     ┌──────────────────────────────────────┐
                     │ site: HTML, .md, .json, llms.txt,    │──▶ Cloudflare Worker `ai` + R2 `ai-site`
                     │ feeds, datasets, MCP                 │──▶ datasets in R2 `ai-data`
                     └──────────────────────────────────────┘
```

Two boxes are new compared with algorithm:

- **cite** checks that each cited claim appears in the stored snapshot of its source (07.5, 13.6). Algorithm had no such check, and its audits found invented citations by hand. Programming added a verifier role for this; here it is a build step as well as a role.
- **exp** runs experiments: small training runs that reproduce the qualitative claim of a page (09). Algorithm's tests are deterministic and fast. Experiments are statistical and can be slow, so they have their own box, their own budget and their own store.

## 3.3 The stores

| Store | Where | What | Size (Estimate at M5) |
|---|---|---|---|
| content | git, `tamnd/ai` | node directories, Lean, registers, topics, schemas | 3 to 5 GB, most of it implementations, experiments and SVG |
| cards | git, `tamnd/ai/cards/`, JSONL sharded by tag prefix (20.3) | the 480,000 cards (04.6) | 0.8 to 1.2 GB; Git LFS is not used, because shards stay under 5 MB each and change by small diffs |
| raw | R2 bucket `ai-raw`, content addressed by BLAKE3 | every fetched page, PDF, JSON and file of every source, for provenance, the citation check and the copy check | 0.5 to 2 TB, most of it arXiv PDFs and their extracted text |
| data | R2 bucket `ai-data` | the pinned datasets that experiments use (09.5), mirrored only when the licence allows it; the published datasets of the site (16.10) | 50 to 200 GB |
| runs | R2 bucket `ai-runs` | experiment outputs (logs, checkpoints of small models, metrics, figures) and agent run records (14.7) | 200 GB to 1 TB, with a retention rule (09.9) |
| build | `build/` in CI, cached by input hash | the graph, the facts, the ladders, test and experiment summaries, Lean extraction, scene bundles, frames, the search index | 10 to 20 GB, not committed |
| reports | `reports/` in git | coverage, audit, test matrix, experiment matrix, inventory summaries, the cost ledger | small, text and Parquet |
| site | R2 bucket `ai-site` and the Worker's static assets | the rendered site of `main` and of each release | 30 to 60 GB |

The content store is the only source of truth for what we write.
The raw store is the only source of truth for what the sources said, on the date that we fetched it.
Every other store can be rebuilt from these two, except `ai-runs`, which records experiments that cost money to repeat; its results that the site uses are copied into the node's `results.json` in git (09.6), so the site never depends on `ai-runs`.

## 3.4 The packages of `aik`

| Package | Job | From | Spec |
|---|---|---|---|
| `node` | parse and write node directories; front matter; the body sections of each kind | algorithm `node` | 04.4, 06 |
| `card` | read, write, shard, merge and promote cards | programming `card` | 04.6 |
| `schema` | JSON schemas of front matter, cards, facts, claims, contracts, experiments, registers | algorithm | 04 |
| `tag` | mint, check and retire tags; the tag register | algorithm `mutate` mint | 04.3 |
| `graph` | the typed edges, reverse edges, closure, the prerequisite DAG, lineage trees | algorithm | 04.7, 05.7 |
| `taxonomy` | the area tree, facets, courses, the core flag | algorithm | 05 |
| `markup` | Markdown with math, cross-links `[[slug]]`, `[[al:slug]]`, `[[m2:slug]]`, claim anchors `{#c-…}` | algorithm `markup` | 06 |
| `katex` | server-side KaTeX through goja | algorithm `katex` | 06.4 |
| `notation` | the notation register and the symbol check | new | 06.4 |
| `facts` | the fact model: merge sources, compute status, tolerance, `as_of`, computed facts | new | 04.9 |
| `claims` | extract, type and label claims; compute evidence labels | new | 07 |
| `cite` | the citation check against the raw store | new, from programming's verifier | 07.5 |
| `lean` | `lake build`, the extractor, axioms, `sorry` scan, the declaration–node bindings | algorithm `formal` | 11 |
| `framework` | the framework register, versions, the pinned package sets, conventions | algorithm `lang` | 08.2 |
| `runner` | run tests and experiments in their pinned images, with no network, limits and a cache | algorithm | 08.9 |
| `testkit` | contracts, fixtures, gradient checks, differential tests across frameworks, test vectors, mutation | algorithm `contract`, `mutate` | 08.7 |
| `exp` | experiments: configs, seeds, budget classes, the scheduler, the statistical tests, `results.json` | new | 09 |
| `results` | the results register: result cards, settings, comparability, computed tables | new | 10.6 |
| `harness` | adapters that run small open models on benchmarks that we can run, for result cards of kind `measured-here` | new | 10.4 |
| `viz` | bundle scenes, run them headless for frames, check traces | algorithm `viz` | 12 |
| `status` | compute the ladders W, G, X, R, F, V, D and the badges | algorithm `status` | 04.10, 15.1 |
| `audit` | the audit rules of all families | algorithm `check` | 15.3 |
| `harvest` | source adapters on the `*-cli` libraries; the raw store; snapshots | algorithm and programming `ingest` | 13 |
| `universe` | the inventory, the universe register, outcomes, scope rules | programming | 18 |
| `coverage` | the coverage levels and reports | algorithm | 18 |
| `copycheck` | similarity of prose and code against the source corpora | math2-agent | 13.7 |
| `agent` | the roles, the ladder of models, prompts, budgets, run records; called by `work/` | algorithm-work `lib.py`, `writer.py` | 14 |
| `queue` | the work queue from gaps and from the daily job | algorithm | 14.5 |
| `prov` | provenance logs of each file and card | math2-agent | 04.8 |
| `site` | the renderer, templates, assets, deploy | algorithm `site` | 16 |
| `search` | the search index builder | algorithm | 17 |
| `mcp` | the MCP server | math2-agent | 16.10 |
| `data` | the published datasets: Parquet and JSONL exports of cards, results and the graph | programming | 16.10 |
| `release` | the monthly release, its manifest and archive | algorithm | 20.6 |

The packages that come from algorithm-agent and math2-agent start as copies, adapted.
They do not import those modules, so each project can change without breaking the others, as in algorithm.

## 3.5 The seven flows

**F1. Harvest (daily for fast sources, weekly for the rest).**
`aik harvest <source>` fetches the lists and records of a source through its `*-cli` adapter, stores raw captures in `ai-raw`, and writes inventory rows (13.3).
`aik universe merge` maps the rows to existing items or proposes new universe items (18.3).
`aik card sync` turns accepted items into cards and updates the facts of existing cards (04.9).
A pull request carries the change, one per source per run.

**F2. Write (each change).**
A person or an agent changes one node directory, often one part of it: the text, one framework, one experiment, the scene.
`aik check -changed` runs the schema checks, the audit rules of the changed nodes, the claim extraction and the citation check of changed sentences, the Lean build of changed modules, the tests of changed implementations in their images, and the scene checks.
Experiments do not run in F2 unless they are in the budget class `cpu-small` (09.4).
The pull request shows the ladders before and after, as in algorithm.

**F3. Build (each push to `main`, and nightly).**
`aik build` runs everything for the whole tree with a cache keyed by input hashes: the Lean package, the tests of each framework, the computed facts, the results tables, the scenes and frames, the graph, the ladders and the coverage.
It writes `build/` and the reports.

**F4. Publish (after F3).**
`aik site build` renders the site from the tree and `build/`.
`aik site check` checks links, sizes, accessibility rules and the file budget.
`aik site deploy` uploads changed files to `ai-site` and the Worker assets, then checks a sample of live URLs (the live check of algorithm's `deploy.sh`).
A change on `main` is live in 15 to 30 minutes (16.20).

**F5. Agents (continuous, with budgets).**
`aik queue` turns gaps of the coverage report into work items.
`work/` scripts take items by sub-area, as algorithm-work did: one writer run per node with the gate, a cross-family audit, fix rounds, the model ladder, then one pull request per sub-area (14).

**F6. Experiments (scheduled).**
`aik exp plan` lists the experiments whose inputs changed or that have never run, by budget class.
`aik exp run` runs `cpu-small` experiments in CI, and `gpu-small` and `gpu-medium` experiments on the GPU pool (09.4) within the monthly compute budget.
Results go to `ai-runs`, and `aik exp run -update` writes the summary to the node's `results.json` in a pull request.

**F7. Freshness (daily).**
The daily job (14.9) runs F1 on the fast sources (arXiv listings, the Hugging Face Hub trending and new models, Epoch AI's database, OpenReview, the AI Incident Database, official journals for AI law), makes cards for new items within a day, recomputes time-bound facts, and opens work items for new items that the scope rule makes notable (18.5).

## 3.6 Cross-cutting decisions

- **Determinism where it is possible, statistics where it is not.** Every build step is a function of its inputs and the pins: images, framework versions, Lean and Mathlib, model ids and prompt versions. Experiments are not deterministic across hardware even with fixed seeds, so their checks are statistical tests over several seeds (09.3), never equality of numbers.
- **Sandbox.** Each runner image runs with no network, a read-only tree, a time limit and a memory limit (08.9). Code from a pull request never runs outside it. Experiments read data only from the mounted, hashed copy of their pinned datasets.
- **Incremental by default.** A change to one implementation runs its tests only. A change to a shared fixture or a contract runs every implementation that uses it. A change to a dataset pin reruns the experiments that read it in the next F6. A change to a Lean module rebuilds its dependants only.
- **Dates everywhere.** Each fact, result and computed table carries the date of its sources and the date of computation (04.9). The build fails when a page prints a time-bound fact without a date (K04).
- **Machine copies.** Each page has `.md` and `.json` twins, and the site has `llms.txt`, feeds, datasets and an MCP server (16.10), as in programming.
- **Offline.** Each release has an archive of the node pages that opens from disk (20.6). The cards are published as datasets, not as offline pages.

## 3.7 Deployment

| Part | Where | Cost (Estimate) |
|---|---|---|
| main site | Cloudflare Worker `ai` with static assets for the shell and the hot pages, and R2 `ai-site` for the rest; Workers and R2 from day one, because GitHub Pages failed for algorithm at about 3,500 nodes and 19,500 files (01.6) | Workers Paid $5/month; R2 storage and operations under $15/month at M5 |
| mirror | none in v1; the release archive is the offline copy (Q-12) | |
| CI | GitHub Actions for pull requests, with `cpu-small` experiments; a self-hosted build server for the nightly full build, as for math2 and algorithm | the server exists |
| GPU pool | on-demand rented GPUs for `gpu-small` (one 24 GB GPU) and `gpu-medium` (one 80 GB GPU) experiments, started by F6 and stopped after the batch (09.4) | a monthly cap of 09.10, Default $400/month at M3 |
| raw, data, runs | R2 `ai-raw`, `ai-data`, `ai-runs` | $10 to $40/month at M5, most of it `ai-raw` |
| agents | model API spend under the monthly budgets of 14.8 | see 21 |
