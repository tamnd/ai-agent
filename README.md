# ai-agent

[![CI](https://github.com/tamnd/ai-agent/actions/workflows/ci.yml/badge.svg)](https://github.com/tamnd/ai-agent/actions/workflows/ci.yml)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/tamnd/ai-agent/badge)](https://scorecard.dev/viewer/?uri=github.com/tamnd/ai-agent)
[![Go Reference](https://pkg.go.dev/badge/github.com/tamnd/ai-agent.svg)](https://pkg.go.dev/github.com/tamnd/ai-agent)

This repository holds `aik`, the toolchain of [ai.tamnd.com](https://ai.tamnd.com). The content is in [tamnd/ai](https://github.com/tamnd/ai).

ai.tamnd.com is an open encyclopedia of artificial intelligence. Each idea, method, architecture, model, benchmark, dataset, result, risk, law, person and event of AI gets a page or a card. Each claim has a source, a date and an evidence label that the build computes.

`aik` reads the content tree, checks it against the specification, runs the tests of the reference implementations, bundles the visualizations and renders the site.

## Status

`aik` starts as a copy of `algo`, the toolchain of [tamnd/algorithm-agent](https://github.com/tamnd/algorithm-agent). The node reader, the audit rules, the ladders, the Lean build, the scene player and the site come from there. Milestone M0 changes them for the kinds, the cards, the claims and the frameworks of AI. [Spec 21](https://ai.tamnd.com/spec/21-milestones/) lists the work.

## Install

You need Go 1.27 or later.

```sh
go install github.com/tamnd/ai-agent/cmd/aik@latest
```

You can also download a binary from the [releases](https://github.com/tamnd/ai-agent/releases). Each binary has a build provenance attestation. To verify a binary, run `gh attestation verify aik_linux_amd64.tar.gz -R tamnd/ai-agent`.

## Use

Run these commands in a clone of tamnd/ai.

| Command | What it does |
| --- | --- |
| `aik new <kind> <slug> -area <area>` | Makes a node directory with the sections of its kind. |
| `aik fmt` | Puts node.md, the registers and the topics in canonical form. Add `-check` to only report. It never changes code. |
| `aik mint` | Gives each provisional id `NEW-<slug>` a permanent tag and writes it into `registers/tags.reg`. Add `-dry-run` to see the tags first. |
| `aik mv <slug> <new-slug>` | Renames a node. It changes each edge and link to the old slug, and adds the old slug to `registers/aliases.reg`, so the old URL redirects. The tag does not change. |
| `aik promote <tag\|slug>` | Promotes a card to a node with the same tag, so the URL stays (spec 04.1). It writes `node.md` with the fields of the card that a node of its kind has (04.5) and the section headings of the kind, and `facts.yaml` with the facts of the card. A fact whose source is the harvest source of the card is secondary, with the harvested row and date. Any other source is written as primary, for the writer to check. A plain field that is a required fact of the kind (10.11), such as the developer of a model, becomes a fact whose source is the harvested row. Time-bound facts take the date of the harvest as `as_of`. The command then removes the line from its shard, sets the outcome of the universe row to `node`, and starts the provenance log. It lists the card fields it did not move. The area defaults to the area of the universe row. `-n` prints the files and changes nothing; `-commit` puts the promotion in one commit. |
| `aik claims extract <slug>... \| -all` | Runs the cheap part of the extraction pass of spec 07.6. It splits the prose of a node into sentences and classes each one as `claim`, `definition-in-place`, `structure` or `quote`. It matches the anchors and writes the sentences to `build/claims/<slug>.jsonl` for the model part. It reports the K01, K04 and K05 findings and the claims that no sentence anchors, and exits 1 on a K04 or K05 phrase. It also lists the K02 candidates: unanchored claim sentences with a year, a citation, a number or a reporting verb. Only `-strict` fails on those, because the model part decides which ones are claims. The gate runs it as step 6. |
| `aik check` | Runs the audit rules of spec 15. Add `-changed origin/main` to check only the nodes that a branch changes. |
| `aik status <slug>` | Shows the ladders of a node and what the next level needs. |
| `aik test <slug>` | Runs the tests of the implementations of a node in the sandbox. Without the sandbox, it runs only with `-unsafe-local`. |
| `aik exp list` | Lists the experiments of the nodes, with the verdict and the date of the last run, and tells when `results.json` is stale. |
| `aik exp run <slug>` | Runs the experiments of a node in the sandbox, with the limits of the budget class `cpu-small`, and prints the verdict of each test of `expect.yaml`. Add `-update` to write `results.json`. |
| `aik shapes sample -w <slug>` | Writes `fixtures/shapes.json` of an architecture from its `shapes.yaml`: the example config, two small sampled configs and the named configs, each with the expected shapes, parameter counts and FLOP counts that the tests of every framework check (spec 8.6). `aik shapes show <slug>` prints the counts. |
| `aik results table <benchmark>@<version>` | Prints the computed results table of a benchmark version: one block for each comparable group, from the best score down, with the source and the evidence label of each row (spec 10.7). Add `-json` for JSON. |
| `aik bridge sync al\|m2\|pg` | Reads the index of a neighbour site (algorithm, math2 or programming) and writes its bridge cards to `cards/bridge/<site>/`. A card follows the tag of its item: a renamed item keeps its old slug, and an item that left the neighbour stays with the date of the sync in `retired`. Add `-n` to see the changes and write nothing, and `-from <file>` to read the index from a file (spec 5.9). |
| `aik build` | Builds the whole tree, step by step: the audit rules, the Lean package, the tests of each implementation (with the result cache of `aik test`), the computed facts in `build/facts`, the results tables in `build/tables`, the scenes, the ladders in `build/status.json` and the coverage report in `build/coverage.json`. A step whose inputs hash as at its last pass is not run again (`-force` runs it). The build goes on after a failed step, writes each log to `build/logs/`, and exits with 1 at the end. The graph is not built yet. |
| `aik coverage` | Computes the coverage level C0 to C6 of each node and card (spec 18.2) and the coverage report of 18.7: the universe register, the promises, the counts by area, kind and core flag with the change since the last release, the gaps by type and the canon coverage. Add `-json` for the whole report and `-items` for each item and what its next level needs. It exits with 1 only on a broken promise: a core node row below C3 after M3. The targets are the defaults of spec 04.10 until `schema/targets.v1.yaml` exists; freshness and demand are not computed yet. |
| `aik queue` | Builds the work queue of spec 14.5 into `work/state/queue.jsonl` from the coverage report and the audit rules: the rows that promise a node or a card that does not exist, the nodes and cards below the coverage level of the milestone (`-milestone M1` to `M5`), the pending rows of the universe and the result cards that fail an M rule. The core comes first, then the targets of the milestone, then the pending rows, oldest first. A lease on an item stays while it runs. `-n` prints and does not write; `-area` filters what is printed. |
| `aik prov show <slug>` | Prints the provenance log of a node, `prov/<tag>.jsonl` (spec 04.8), the newest line first, and the state of each file: recorded (its last line has its hash), changed since, created (only the line `*` of `aik mint` names it), unrecorded or removed. `aik prov check` checks the lines of every log (dates, paths, operations, hashes, authors, auditors) and fails on a bad line; `-strict` fails also on a node with no log and on a change not recorded. `aik prov record -author <login> <slug>...` appends a line for each file that changed, is new or is gone, for a change made by a person; `work/prov.py` records the changes of agents. |
| `aik quarantine -by <login> -why <reason> <slug>` | Hides a node from the site at the next build (spec 15.7, 19.11), or only some of its files with `-file impl/python/x.py`: it writes a `quarantine` line in `prov/<tag>.jsonl`. A hidden node keeps its name, kind and links but loses its body, summary, code, proofs, facts and claims, its Markdown copy and its search entry, and its page says that it is withdrawn for review, never why; a hidden file leaves the page and the page says which. `-clear` writes a `confirm` line that shows it again; `-list` lists what is hidden. |
| `aik search eval` | Runs the evaluation set `registers/search-eval.yaml` (spec 17.10) on the built `search.json` with `find.js`, the ranking script of the palette, in goja, so it measures the code that the reader runs. Each query has a class (title, alias, id, acronym, version, question, log) that sets what it must do: rank 1, every sense in the top 5, or the top 3. It prints the gates of 17.10 (success at 1 or 3 per class, MRR at 10 with `-baseline` for the drop from the last release, the gzip size of the palette index) and each failed query with its expected items and the top 5 with their scores. A query whose items are not in the index yet is pending and no gate counts it. `aik site build` and `aik site check` run it too when the set exists, and fail when a gate fails. |
| `aik sample -week <YYYY-Www>` | Draws the weekly sample of spec 15.6 from the pages that changed in an ISO week, read from `git log`: every changed gold page of `registers/gold.yaml`, 10% of the changed core nodes, 15% of the changed contested, policy and person pages, 10% of the changed cards of models released in the last 6 months, 1% (at least 50 per source) of the new harvested cards, then 2% of every other page. The draw is seeded by the week: each page gets the key sha256(week, stratum, id), so anyone can recompute the sample. Without `-week` it draws the last whole week. |
| `aik release cut <YYYY.MM>` | Freezes `manifests/releases/<YYYY.MM>.yaml` with the content commit, the `aik` version, the image digests of `manifests/images.yaml`, the Lean pin, the data pins and the `models.yaml` version, writes the release notes `<YYYY.MM>.md` and a copy of the coverage report, and tags the commit. It refuses a work tree with changes that are not committed and a release that exists. Add `-no-tag` to write the files only. `YYYY.MM.N` is a patch release. |
| `aik lean build` | Builds the Lean package, extracts the declarations and checks the axiom policy. |
| `aik lean bump` | The monthly bump of spec 11.11. It moves `lean/lean-toolchain` and the `rev` of the Mathlib require in `lean/lakefile.toml` to the latest release tag of Mathlib, or to `-to vX.Y.Z`, runs `lake update mathlib` and `lake exe cache get`, and builds each module of `AI` on its own, so one broken module does not hide the others. When every module builds, it extracts the declarations again and compares them with `build/lean/decls.jsonl`: lost, now depending on `sorry` or on an axiom outside the policy, or with a changed type (a fidelity review, 11.7). It writes `build/lean/bump.md` with the modules that fail and their first errors, the changed declarations and each node whose formal level moves, and exits 1 when a node at F2 or above breaks. `-n` prints the target and changes nothing, `-pr` commits the three pin files on `lean-bump-<tag>` and opens a pull request whose body is the report, and `-sandbox docker` runs lake in the Lean image. |
| `aik viz check` | Builds each scene and runs each preset two times. |
| `aik site serve` | Renders the site and serves it at http://localhost:8150. |
| `aik site check` | Renders the site and checks each internal link. |
| `aik site deploy` | Renders the site, checks the limits of the host and uploads the tree as the static assets of the Cloudflare Worker `ai-site` on ai.tamnd.com. Add `-preview` to upload a preview version under the alias of the branch. It reads `CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID` from the environment and does not print them. |

`aik help` shows all commands. A command that is planned but not built yet exits with code 3 and names its milestone.

The exit codes are 0 for success, 1 when a check finds a problem, 2 for a usage error and 3 for a planned command.

## Safety of `aik test`

`aik test` and `aik exp run` run code from the content tree. When the content repository has `manifests/images.yaml`, each test runs in a new container with no network, a read-only root and limits on memory, CPU and processes. Give `-sandbox podman` to use Podman. Without the sandbox, `aik test` runs on your computer only with the flag `-unsafe-local`. Use it only on code that you have read. In CI, the jobs that run content code have no secrets.

## Layout

| Directory | Contents |
| --- | --- |
| `cmd/aik` | The command. |
| `node` | Reads the content tree. |
| `check` | The audit rules. |
| `status` | The ladders of a node. |
| `formal` | Builds the Lean package and checks the axiom policy. |
| `contract` | Reads contracts and shared cases, and decides if an output is right. |
| `lang` | Writes the build trees and runs the tests of each language. |
| `viz` | Bundles the scenes with esbuild and checks them in a JavaScript runtime. |
| `markup` | Markdown and math. |
| `katex` | Renders TeX to HTML on the server. |
| `site` | The templates, assets and pages of the site. |
| `spec` | The specification. The site publishes it at [/spec/](https://ai.tamnd.com/spec/). |
| `testdata/corpus` | A small content tree for the tests. |
| `work` | The agent pipeline scripts of spec 14: catalog, stubs, writer, gate, audit, provenance, pull requests and deploy. See [work/README.md](work/README.md). |

## Develop

```sh
make test     # go vet and go test
make lint     # golangci-lint
make site     # render testdata/corpus into build/site
```

Read [CONTRIBUTING.md](CONTRIBUTING.md) before you open a pull request. To report a security problem, read [SECURITY.md](SECURITY.md).

To release, push a tag `vX.Y.Z`. The release workflow builds the archives from a clean tree of the tag, attests them and publishes the release. A patch release comes after a few merged pull requests. A minor release comes at the end of each milestone.

## Roadmap

The work has six milestones, M0 to M5, from November 2026 to December 2028. [Spec 21](https://ai.tamnd.com/spec/21-milestones/) gives the scope and the exit criteria of each one. The [milestones](https://github.com/tamnd/ai-agent/milestones) list the open issues.

## Licence

The code is under the [Apache License 2.0](LICENSE). The scene package `viz/packages/core` is under the MIT licence, so scenes in the content repository can use it. The parts from other projects are listed in [THIRD_PARTY.md](THIRD_PARTY.md).
