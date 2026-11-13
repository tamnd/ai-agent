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
| `aik check` | Runs the audit rules of spec 15. Add `-changed origin/main` to check only the nodes that a branch changes. |
| `aik status <slug>` | Shows the ladders of a node and what the next level needs. |
| `aik test <slug>` | Runs the tests of the implementations of a node in the sandbox. Without the sandbox, it runs only with `-unsafe-local`. |
| `aik exp list` | Lists the experiments of the nodes, with the verdict and the date of the last run, and tells when `results.json` is stale. |
| `aik exp run <slug>` | Runs the experiments of a node in the sandbox, with the limits of the budget class `cpu-small`, and prints the verdict of each test of `expect.yaml`. Add `-update` to write `results.json`. |
| `aik results table <benchmark>@<version>` | Prints the computed results table of a benchmark version: one block for each comparable group, from the best score down, with the source and the evidence label of each row (spec 10.7). Add `-json` for JSON. |
| `aik bridge sync al\|m2\|pg` | Reads the index of a neighbour site (algorithm, math2 or programming) and writes its bridge cards to `cards/bridge/<site>/`. A card follows the tag of its item: a renamed item keeps its old slug, and an item that left the neighbour stays with the date of the sync in `retired`. Add `-n` to see the changes and write nothing, and `-from <file>` to read the index from a file (spec 5.9). |
| `aik lean build` | Builds the Lean package, extracts the declarations and checks the axiom policy. |
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
