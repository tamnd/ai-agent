# 20. Repository layout

This document fixes the trees of `tamnd/ai` and `tamnd/ai-agent`, where the stores are, how cards are sharded, the `aik` command tree, the CI workflows, the releases and the sync with the neighbours.
It follows algorithm (2150/20) and programming (2150p/20), with the additions that cards, experiments, facts and the `work/` scripts need.

## 20.1 `tamnd/ai`

```
tamnd/ai/
  README.md  CONTRIBUTING.md  LICENSE-PROSE (CC BY-SA 4.0)  LICENSE-CODE (MIT)  LICENSE-LEAN (Apache-2.0)  LICENSE-DATA (CC0 1.0)
  nodes/
    <top>/<sub>/<slug>/                 one node directory (04.4)
      node.md  claims.yaml  facts.yaml  shapes.yaml  contract.yaml  releases.yaml
      derivations/*.md
      impl/<framework>/*                 numpy pytorch jax c cpp rust julia typescript (08.2)
      vectors/*.jsonl                    metric test vectors (08)
      exp/<name>/                        experiment.yaml run.py expect.yaml results.json figures/ (09)
      viz/                               scene.ts presets.yaml (12)
      figures/*.svg
  cards/
    <kind>/<prefix>.jsonl                one card per line, sharded by tag prefix (20.3)
    bridge/<site>/<prefix>.jsonl         bridge cards of al, m2, pg (5.9)
  lean/
    lakefile.lean  lean-toolchain (v4.35.0)  lake-manifest.json
    AI.lean
    AI/<Area>/<Name>.lean                (11)
  registers/
    tags.reg  aliases.reg  reserved.yaml  disambiguation.yaml
    universe/<top>.jsonl                 the universe rows with outcomes other than row-only (18.3)
    inventory/<source>.jsonl             the harvested inventories (13)
    sources.yaml  canon.yaml  surveys.yaml  watch.yaml  leaderboards.yaml  venues.yaml  awards.yaml  standards.yaml
    core.yaml  gold.yaml  scope.yaml  purposes.yaml  banned-words.yaml  site-tiers.yaml
    notation.yaml  notation/<top>.yaml  (06.4)
    frameworks.yaml                      versions and image digests of 08.2
    datasets-pinned.yaml                 the data pins of experiments (09.5)
    models.yaml                          the agent models of 14.4
    reviewers.yaml                       (14.11)
    positions/<topic>.yaml               the position lists of contested topics (06.9)
  topics/
    areas.yaml  paradigms.yaml  comparisons.yaml
    courses/<slug>.yaml                  paths and courses (5.7)
  crosswalk/
    acm-ccs-2012.yaml  arxiv.yaml  openalex.yaml  wikidata.yaml  wikipedia.yaml  pwc.yaml  hf-tasks.yaml
    epoch.yaml  aiid.yaml  mit-ai-risk.yaml  nist-ai-100.yaml  iso-22989.yaml  cs2023-ai.yaml  philpapers.yaml
    algorithm.yaml  math2.yaml  programming.yaml
  prov/
    <tag>.jsonl                          provenance of node files (04.8)
    cite/<tag>.jsonl                     the records of the citation check of the claims of a node (13.6)
    cards/<prefix>.jsonl                 changes of card rows by people or agents
    runs/<run-id>.yaml                   agent batch records (14.7)
  manifests/
    releases/next.yaml                   the aik version, framework images, Lean pin, data pins of the next release
    releases/<YYYY.MM>.yaml              the frozen manifest of each release
    <source>.jsonl                       harvest snapshot manifests: source id, fetch time, BLAKE3 hash in ai-raw
    works.jsonl                          the snapshots of the cited versions of works: work, version, URL, hash, extractor (13.6)
  reports/
    coverage.json  audit.json  tests.json  experiments.json  facts.json  cost.json  inventory/<source>.json
  .github/
    workflows/*.yml                      (20.5)
    ISSUE_TEMPLATE/  CODEOWNERS  pull_request_template.md
```

Rules:

- Only `aik` writes `registers/tags.reg`, `results.json`, `reports/` and the computed parts of `cards/` (04.10, 04.11). A pull request that edits them by hand fails T rules.
- `schema/` lives in `tamnd/ai-agent` (`schema/node.v1.json`, `schema/targets.v1.yaml`, and the others), and the content repository pins its version through `manifests/releases/next.yaml`.
- There is no `site/` directory and no built output in git. The site is built in CI.
- Each top area has a `CODEOWNERS` line for its editors (14.11).

## 20.2 `tamnd/ai-agent`

```
tamnd/ai-agent/
  go.mod  go.sum  LICENSE (Apache-2.0)  Makefile
  cmd/aik/main.go
  internal/                              one directory per package of 03.4
    node/ card/ schema/ tag/ graph/ taxonomy/ markup/ katex/ notation/ facts/ claims/ cite/ lean/
    framework/ runner/ testkit/ exp/ results/ harness/ viz/ status/ audit/ harvest/ universe/
    coverage/ copycheck/ agent/ queue/ prov/ site/ search/ mcp/ data/ release/
  schema/
    node.v1.json  card.v1.json  record.v1.json  facts.v1.yaml  claims.v1.json
    experiment.v1.json  expect.v1.json  levels.v1.yaml  targets.v1.yaml  edges.md
  harvest/<source>/                      one adapter per source (13.2), on the *-cli libraries
  images/<framework>/                    Dockerfiles and lock files of the runner images (08.9)
  harness/<benchmark>/                   benchmark adapters for measured-here results (10.4)
  viz/                                   @aik/viz, the scene runtime (MIT, 12)
  site/
    templates/  assets/  worker/         the Worker `ai-site` (16.20)
  work/                                  the pipeline scripts (14.1)
    lib.py  catalog.py  curate.py  canon.py  match.py  enrich.py  finalize.py  stub.py  kinds.py
    writer.py  gate.py  audit.py  verify.py  code.py  experiment.py  daily.py
    prov.py  pr.py  report.py  mkarea.py  ship.sh  deploy.sh
    prompts/<role>/v<n>.md  prompts/areas/<top>.md
    tests/                               unit tests of the scripts, with recorded model replies
    state/  catalog/  logs/               local, git-ignored (14.7)
  testdata/                              golden node directories, cards and sites for the Go tests
  docs/
```

Rules:

- `work/state/`, `work/catalog/` and `work/logs/` are git-ignored, as in `algorithm-work`. Every decision in them that matters ends in a register of `tamnd/ai` through a PR (14.1).
- A release of `ai-agent` is a tag `vX.Y.Z` with binaries for darwin and linux on arm64 and amd64, and the runner images pushed with their digests.
- The scripts of `work/` have tests that run without a model, from recorded replies in `work/tests/`. A port to Go (Q-05) must pass the same tests.

## 20.3 Storage and card sharding

The stores are fixed in 03.3. The layout rules:

- **Card shards.** A card file is `cards/<kind>/<prefix>.jsonl`, where `<prefix>` is the first characters of the card's tag. A kind starts with one shard `_.jsonl`. When a shard passes **5 MB**, `aik card sync` splits it by one more character of the prefix (`5.jsonl` becomes `5T.jsonl`, `5V.jsonl`, …) in one commit. With 32 symbols, the 200,000 work cards at about 1.2 KB each fit in 32 shards of about 7.5 MB, so works split to two characters (1,024 shards of about 240 KB); results split the same way. Lines are sorted by tag, so a change is a small diff. Git LFS is not used (03.3).
- **Large registers.** `row-only` universe rows (about 5 million, 18.8) and the full inventories of OpenAlex, the Hub and arXiv are Parquet in R2 `ai-data/universe/` and `ai-data/inventory/`, with their manifests in git. `registers/inventory/<source>.jsonl` in git holds only the rows that have an outcome other than `row-only`.
- **Raw.** `ai-raw/<blake3>` holds each fetched object once, and `manifests/<source>.jsonl` maps source ids to hashes. Nothing in `ai-raw` is public, because many sources may be read but not redistributed (19).
- **Runs.** `ai-runs/exp/<tag>/<name>/<run-id>/` and `ai-runs/agents/<run-id>/` (09.6, 14.7).
- **Size alarm.** `aik audit` warns when the content repository passes 6 GB or a single directory passes 50 MB, so that a big file is found before it is merged.

## 20.4 The `aik` command tree

Every command prints text for people and JSON with `-json`.
Exit codes, as in algorithm: **0** success; **1** the check found problems; **2** bad input or usage; **3** an internal or environment error (a missing image, no network when needed, a store that cannot be reached).

```
aik
  new <kind> <slug> -area <top>/<sub>      write a node directory with the sections of its kind and stub markers (06.3)
  new viz <slug>                            add a scene skeleton (12)
  fmt [paths]                               format front matter, markup, one sentence per line
  check [paths] [-changed] [-rules F,..]    the audit rules of 15.3 on the given files; -changed for the files of the branch
  gate <slug>                               the writer gate of 14.6: fmt, check, stubs, plain words, characters, claims, tests, cpu-small experiments, cite
  mint                                      give tags to every NEW- id; update tags.reg
  promote <id>                              turn a card into a node directory, same tag (04.1)
  card sync [-kind K] [-source S]           write and update cards from inventories and universe outcomes; split shards
  claims extract <slug>                     find claim sentences and match them with claims.yaml (07.3)
  cite <slug..|-all> [-offline]             the citation check against ai-raw snapshots; writes prov/cite and manifests/works.jsonl (07.5, 13.6)
  cite show <work> ["<locator>"]            the places that the extraction of a work found, or the text of one place (13.6)
  cite passages <slug>... [-redo]           the passages of the records that passed and have no verdict, as JSON, for the verifier (15.4)
  cite verdict -file F -verifier B:M -run R the verifier's verdicts [{tag, key, verdict}] into the records (15.4)
  facts compute [-kind K]                   merge fact sources, compute status, as_of and computed facts (04.9)
  test <slug> [-framework F]                run the tests of a node in their pinned images (08)
  vectors make <slug>                       compute metric test vectors with the official implementation (08)
  fixtures make|sample <slug>               test fixtures (08.7)
  shapes sample <slug>                      check shapes.yaml against the implementations (08.6)
  flopcount <slug>                          check the FLOP formulas of shapes.yaml (08.6)
  mutate <slug>                             mutation testing of the tests (08.8)
  exp plan [-class C]                       list the experiments to run and their cost (09)
  exp run <slug>[/<name>] [-class C] [-update]   run; -update writes results.json
  exp check|stats|power|verify|pool         validate configs; statistical tests; power analysis; re-run check; GPU pool control (09)
  results table <benchmark@version>         the computed results table of a benchmark version (10.6)
  lean build | lean check                   lake build; extraction, axioms, sorry scan, bindings (11)
  viz check|frames|record|shapes|serve|export <slug>   (12)
  harvest <source> [-since DATE]            run an adapter; write to ai-raw, inventory and manifests (13)
  universe merge                            join inventories into the universe register; apply scope rules (18.3, 18.5)
  coverage [-area A]                        the coverage levels and report (18.7)
  bridge sync al|m2|pg                      refresh bridge cards from a neighbour's .json twins (5.9)
  copycheck [paths]                         similarity against the source corpora (13.7)
  queue [-milestone M] [-area A]            build the work queue (14.5)
  report cost|levels|tests|experiments|quality   reports into reports/; quality feeds /about/quality/ (15.9)
  sample [-week YYYY-Www]                   the seeded weekly review sample (15.6)
  build [-out DIR]                          the full build: graph, facts, ladders, results tables, coverage, search, bundles
  site build|check|serve|deploy [-out DIR] [-commit H]   render, check links and budgets, serve locally, deploy to the Worker and R2 (16)
  mcp                                       the MCP server over the built data (16)
  release cut <YYYY.MM> | release archive   the monthly release (20.6)
  version
```

## 20.5 Workflows

GitHub Actions of `tamnd/ai`, and the self-hosted build server for the heavy jobs, as for algorithm.

| Workflow | Trigger | What | Blocks merge |
|---|---|---|---|
| `check.yml` | every PR | `aik fmt -check`, `aik check -changed`, `aik claims extract`, `aik test` and `aik exp run -class cpu-small` for the changed nodes, `aik copycheck` on changed prose and code | yes |
| `tags.yml` | every PR | no tag reused or retired; `NEW-` ids only on PRs that run `aik mint` before merge | yes |
| `lean.yml` | PRs that touch `lean/` | `aik lean build`, `aik lean check` | yes |
| `build.yml` | push to `main`, nightly | `aik build` and `aik site build` and `aik site check` on the build server | no; a failure opens an issue |
| `deploy.yml` | after `build.yml` on `main` | `aik site deploy -commit $H` and the live check (14.10) | no |
| `harvest.yml` | weekly per source | `aik harvest`, `aik universe merge`, `aik card sync`; one PR per source | no |
| `daily.yml` | 06:00 UTC | the daily freshness job of 14.9 | no |
| `exp.yml` | monthly, and on demand with a label `exp: gpu` | `aik exp plan`, then `aik exp run -update` on the GPU pool under the cap (09.10); a PR with the new `results.json` | no |
| `audit.yml` | daily | the full `aik check`, `aik cite -all`, link check; `reports/audit.json` in a PR when it changes | no |
| `frameworks.yml` | a new framework release (08.2) | build the image, run all tests of that framework, open a PR with the new pin when they pass, or issues when they fail | no |
| `release.yml` | the first Monday of each month | `aik release cut <YYYY.MM>` and the archive (20.6) | no |
| `bridge.yml` | at each neighbour release, and weekly | `aik bridge sync al`, `m2`, `pg`; a PR | no |

Secrets (the Cloudflare token, the GPU provider key, the model API keys) are in the repository's environments, never in files; local runs read `$HOME/data/.env` (14.10).
Model API keys are used only by `daily.yml` and by `work/` runs on the build server; PR checks never call a model.

## 20.6 Releases and sync

- **Monthly releases.** `aik release cut 2027.03` freezes `manifests/releases/2027.03.yaml` (the content commit, the `aik` version, the framework image digests, the Lean pin, the data pins, the `models.yaml` version), tags the commit `2027.03`, and writes the release notes: new nodes and cards by area, ladder and coverage changes, new evidence labels, contested items that changed, retired tags.
- **Versioned URIs.** `ai://TAG@2027.03` resolves to the item as it was in that release. The site keeps each release at `https://2027-03.ai.tamnd.com/` from the release archive in R2 `ai-site/releases/2027.03/`, as programming did (Q-12). There is no GitHub Pages mirror.
- **The release archive.** Each release publishes a downloadable archive: the content repository at the release commit, the built `.json` twins, the datasets of 16.10 (Parquet and JSONL), and the search index, with SHA-256 sums. It is the offline copy.
- **Patch releases.** `2027.03.1` for a fix that cannot wait a month (a false fact, a takedown, 19); `report.py next-patch` gives the number.
- **Neighbour sync.** The bridge cards follow the neighbours' releases: `aik bridge sync al` reads `algorithm.tamnd.com`'s `.json` twins, `m2` math2's and `pg` programming's. A neighbour item that was renamed keeps its tag, so bridge cards follow the tag; a retired tag becomes a gap of type `broken-bridge` (18.7). Each neighbour reads our `.json` twins the same way, so the four sites link both ways.
- **Data downstream.** The datasets are published at each release on the site and on the Hugging Face Hub as a dataset repository, with the CC0 and CC BY-SA parts in separate configs (19).
