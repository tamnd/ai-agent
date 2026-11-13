# 16. The site: ai.tamnd.com

The site is where people and agents use the corpus.
`aik site build` builds it from the content tree, the cards and `build/`.
The renderer is the `site` package of algorithm-agent, copied into ai-agent at M0 (03.4).
It keeps the following from algorithm and programming: Go `html/template`, KaTeX at build time, Chroma highlighting at build time, the math2 visual design (one reading column, Inter, a hue for each area, light and dark themes, no rounded corners), pages that work with no script, the commands `aik site serve` (local, on `:8152`), `aik site check` and `aik site deploy`, the Markdown and JSON twins, and hosting on a Cloudflare Worker with R2 from day one.

This document covers what AI adds: catalogues of artifacts, the results tables, evidence labels on claims, dated facts, the contested-topic layout, lineage views, calculators, in-browser notebooks, and hosting at about 40 times algorithm's item count.
Where this document is silent, 2150/14 and 2150p/16 apply.

## 16.1 Design principles

Algorithm's eight principles apply (2150/14.1): reading first, the answer at the top, one page with many depths, honest evidence, the same look for the same idea, no script needed to read, fast on a cheap phone, and stable links.
Programming's "every fact is clickable to its evidence" and "cards are first-class" apply (2150p/16.1).
This site adds four:

1. **Every claim shows its evidence.** A claim of a page (07) carries its computed evidence label, and one click shows the sources, the experiment or the proof behind it. A reader can tell "proved" from "reported by one lab" without reading the citations.
2. **Every number shows its date.** A time-bound fact is printed with its `as_of` date (04.9), and a computed table with its "Data as of" date. A reader never has to guess whether a number is old.
3. **No ranking in our voice.** The site shows computed tables with their population, setting and date. It never says "best", "leading" or "state of the art" (K04), and it never ranks labs or people (02.6).
4. **Positions, not opinions.** On contested questions, the page lays out the positions side by side with their best sources (16.8), and the layout itself is the same for every contested topic, so no position gets a better place by design.

## 16.2 URLs and reserved words

There are two namespaces (04.3): global slugs at the root, and owned slugs under their owner or section.
A URL never changes when an item moves in the tree (05.1), and never changes when a card is promoted to a node (04.1).

| URL | Page | Kind |
|---|---|---|
| `/` | home (16.13) | |
| `/<slug>/` | the page of any global-slug kind: `/attention/`, `/dropout/`, `/transformer/`, `/bleu/`, `/grokking/`, `/chinese-room/`, `/reward-hacking/`, `/eu-ai-act/`, `/alan-turing/`, `/deepmind/`, `/pytorch/`, `/nvidia-h100/` | area, concept, definition, method, architecture, task, metric, theorem, finding, argument, open-problem, risk, person, organization, event, system, software, hardware, standard, policy |
| `/<top>/` and `/<top>/<sub>/` | area pages: `/reinforcement-learning/`, `/reinforcement-learning/policy-gradient-methods/` | area |
| `/models/` | the model catalogue (16.6) | catalogue |
| `/models/<family>/` | a model family node: `/models/llama/`, `/models/alphafold/` | model (family) |
| `/models/<family>/<slug>/` | a release or checkpoint: `/models/llama/llama-3-1-405b/`. A node page when promoted; otherwise a redirect to its anchor on the family page (16.6) | model |
| `/benchmarks/` and `/benchmarks/<slug>/` | the benchmark catalogue, and a benchmark node | benchmark |
| `/benchmarks/<slug>/<version>/` | a benchmark version page with its results table (16.7): `/benchmarks/swe-bench/verified/`, `/benchmarks/mmlu/pro/` | benchmark version |
| `/benchmarks/<slug>/<version>/#r-<tag>` | a result card, as an anchor of its row | result |
| `/datasets/` and `/datasets/<slug>/` | the dataset catalogue, and a dataset node or card | dataset |
| `/results/` | the results register: search and filter over all result cards, with download | catalogue |
| `/works/` and `/works/<slug>/` | the work catalogue, and a work node or card: `/works/attention-is-all-you-need/` | work |
| `/incidents/` and `/incidents/<source>-<id>/` | the incident catalogue, and an incident card: `/incidents/aiid-1625/` | incident |
| `/<policy>/<provision>/` | a provision card: `/eu-ai-act/article-50/`, `/nist-ai-rmf/govern-1-1/` | provision |
| `/policy/`, `/risks/`, `/systems/`, `/software/`, `/hardware/`, `/standards/` | the catalogues of 05.4 | catalogue |
| `/people/`, `/orgs/`, `/events/` | the catalogues of people, organizations and events | catalogue |
| `/timeline/` | the timeline of AI: all events, model releases, landmark works and policy dates, filterable (16.13) | catalogue |
| `/glossary/` and `/glossary/<letter>/` | the glossary (05.4) | catalogue |
| `/tasks/` | the task index (05.3) | index |
| `/exercises/` and `/exercises/<site>/<id>/` | the exercise catalogue, and an exercise card | exercise |
| `/compare/` and `/compare/<a>-vs-<b>/` | comparison pages (16.5): `/compare/lstm-vs-transformer/`, `/compare/ppo-vs-dpo/`, `/compare/llama-3-1-405b-vs-deepseek-v3/` | comparison |
| `/lineage/<slug>/` | the lineage view of a model family, a method or an idea (16.9) | view |
| `/calc/` and `/calc/<calculator>/` | the calculators (16.14): `/calc/training-flops/`, `/calc/kv-cache/` | tool |
| `/paths/` and `/paths/<path>/` | reading paths and courses (16.12) | path |
| `/search/` | search (17) | |
| `/coverage/`, `/gaps/`, `/universe/`, `/latest/`, `/matrix/` | the dashboards (16.15) | |
| `/about/`, `/about/evidence/`, `/about/badges/`, `/about/neutrality/`, `/contribute/`, `/spec/`, `/sources/` | the project, the meaning of the evidence labels (07), the badges (15.1), the neutrality policy (06.9), contributing, this spec, the source register | |
| `/t/<tag>` and `/t/<tag>@<release>` | the item by tag, node or card, and in a site release (20.6) | redirect |
| `/x/<id>` | an external id (17.4): `/x/arxiv:1706.03762`, `/x/doi:10.48550/arXiv.1706.03762`, `/x/hf:meta-llama/Llama-3.1-405B`, `/x/aiid:1625`, `/x/wikidata:Q25339462` | redirect |
| `<any page path>.md`, `<any page path>.json` | the Markdown twin and the JSON record (16.10): `/dropout.md`, `/models/llama.json`, `/benchmarks/mmlu/pro.json` | data |
| `/llms.txt`, `/<top>/llms.txt`, `/data/`, `/mcp`, `/.well-known/mcp`, `/feeds/`, `/sitemap.xml`, `/robots.txt` | the machine surface (16.10) | data |

**How a path is read.** The Worker and the build use the same function (`site.Resolve`, with golden tests), as in programming:

1. If the first segment is a reserved word, it is a site section, and the section's own rules read the rest (`/models/<family>/<slug>/`, `/benchmarks/<slug>/<version>/`).
2. If the first segment is a top area, the second, when present, is a sub-area.
3. If the first segment is a policy or standard that owns provisions, the second is a provision slug.
4. Otherwise the first segment is a global slug, and a second segment is a 404.

**Reserved words.** `registers/reserved.yaml` holds the list. Rule T04 (15.3) rejects a global slug on the list, and rejects a top-area slug that equals a policy slug.

```
about all api assets badges benchmarks calc compare contribute coverage data datasets
embed events evidence exercises feed feeds gaps glossary hardware help incidents index
latest lineage llms matrix mcp models neutrality orgs paths people policy random
releases results risks search sitemap software sources spec standards static status
systems t tasks timeline universe works x _cards _traces 404
```

**Canonical form.** Trailing slashes and lower-case slugs are canonical. Benchmark versions keep a lower-case slug of the publisher's name (`verified`, `pro`, `v1-1`). Provision slugs follow the official numbering (`article-50`, `annex-iii`, `section-4-2`). An alias in `registers/aliases.reg` redirects with 301, and so does a renamed model family (04.3).

## 16.3 Navigation

- **Header.** The site name, the search field (`/` or `⌘K`), the catalogue menu (Models, Benchmarks, Datasets, Results, Works, Policy, Incidents, Timeline, Glossary), the theme switch.
- **Breadcrumbs.** A node page shows its group and area path: D. Deep learning › Deep learning training › Regularization › Dropout. A model page shows the catalogue path: Models › Llama › Llama 3.1 405B. A provision shows its policy: EU AI Act › Article 50.
- **Sidebar** on wide screens: the sub-area list in reading order (05.7) on idea pages; the releases of the family on model pages; the versions on benchmark pages; the articles on policy pages. It folds under the head on narrow screens.
- **Previous and next** in the reading order of the sub-area, or of the path the reader follows (16.12).
- **"Elsewhere" links** to algorithm, math2 and programming (5.9) carry a small label that names the other site, and bridge cards in lists carry the mark "on algorithm.tamnd.com".

## 16.4 The node page

Each kind has one template that follows the sections of 06.3 and adds the parts that the build computes. The parts are the same across kinds, so the reader learns them once.

```
┌──────────────────────────────────────────────────────────────────────────────┐
│ D. Deep learning › Deep learning training › Regularization                   │
│                                                                              │
│ Dropout                                               method · core          │
│ During training, set each unit to zero with probability p and scale the      │
│ others by 1/(1−p), so the network cannot rely on any single unit.            │
│                                                                              │
│ ┌ Facts ────────────────────────────────────────────────────────────────────┐│
│ │ Introduced  2012, Hinton et al. (arXiv 1207.0580); JMLR 2014              ││
│ │ Purpose     regularization          Applies to  MLP, CNN, Transformer     ││
│ │ Cost        training: one mask per activation; inference: none           ││
│ │ Evidence    W4 · G3 · X3 numpy, pytorch · X2 jax · R2 · V2                ││
│ │ Read first  overfitting · feedforward network · regularization            ││
│ └───────────────────────────────────────────────────────────────────────────┘│
│                                                                              │
│ In plain words ...                                                           │
│ How it works ... (equations, derivation links)                               │
│ Evidence                                                                     │
│   Dropout lowers test error on MNIST MLPs.  [measured here] [replicated]    │
│   Dropout approximates a geometric ensemble. [proved, under assumptions]    │
│   Dropout helps less in large pretrained LMs. [contested]                   │
│ Code  [NumPy] [PyTorch] [JAX]   ▶ Run in the browser                         │
│ Experiment: mlp-mnist   5 seeds · gpu-small · R2   [chart with CI bands]     │
│ Visualization ...                                                            │
│ Variants and related ...   Pitfalls ...   History ...   Sources ...          │
│ ── History of this page ──────────────────────────────────────────────────── │
│ written by agent run 2027-03-02 (writer claude-sonnet-5-5, audit <family B>) │
│ checked: gate passed, citation check 14/14, audit score 92                   │
│ reviewed: not yet           ✎ Improve this page   ⚑ Report a problem        │
└──────────────────────────────────────────────────────────────────────────────┘
```

**The facts box** is at the top of every node page, in the first screen. It is computed from the front matter, the facts (04.9), the registers and the ladders, and never written by hand (T05).

| Kind | Facts box lines |
|---|---|
| concept, definition | kind and level; area; first named (year, work); the ladder digits; read first |
| method | introduced (year, work); purpose; applies to; cost at training and inference; hyperparameters with typical values; the ladder digits |
| architecture | family; inputs and outputs; parameter count formula; FLOPs per token; introduced by; notable models (count, link) |
| model | developer; released; architecture; parameters (total, active); training compute; training tokens; data; access; licence; context length; lineage (fine-tuned from, family); each number with its source and status mark (`estimated`, `disputed`) |
| task | input and output; modality; main metrics; main benchmarks; methods used (count) |
| benchmark | task; versions with dates and sizes; metrics; protocol; licence; contamination reports (count); saturation (computed, 10.8) |
| dataset | modality; size; source type; collection dates; licence; documentation; known issues |
| metric | measures; range; better (higher or lower); reference implementation and version; properties |
| theorem | statement in one line; hypotheses; Lean status (F level) |
| finding | the claim; first reported; replications and failures (counts); scale range; evidence label |
| argument, open-problem | the question or conclusion; positions (count); first stated |
| risk | source taxonomies; affected parts; mitigations (count); incidents (count, link) |
| policy | jurisdiction; type; status (computed from the dates); key dates; provisions (count); amended by |
| system, software, hardware | developer or vendor; years or release; type; key specs with sources |
| person, organization | roles or kind; years; areas; key works (count); no private facts (P01) |

**Evidence labels in prose.** Each claim sentence (07) ends with a small label chip: `proved`, `measured here`, `replicated`, `reported` or `contested`. The chip has a shape as well as a colour (a filled square, a filled circle, two circles, a hollow circle, a split circle), so it reads in monochrome. Hover or focus, or a tap on mobile, opens a popover with the claim's sources and locators, the experiment link, or the proof link. `/about/evidence/` defines each label. The chips can be turned off in the settings for plain reading; the label stays in the text view and the twin.

**Dated facts in prose.** A time-bound fact is rendered with its date: "the largest open-weights dense model by total parameters among our model cards (as of 2026-11-13)". The date links to the computed table that the fact comes from.

**History.** The last section of every node page is "History of this page", from the provenance records (04.8): each change with its date, its author (a GitHub login, or an agent run with the model ids and prompt versions), the auditor and the audit score, the checks that passed, and the reviewer when there is one, for the hash that they reviewed. It is the AI disclosure of the page (19). A long history folds after the 5 newest entries.

**Card entries.** A card renders as a row on its collection page (16.6), with its facts, its summary of at most 30 words, its sources, a stable anchor, and links to its `.json` and `.md`. Its URL redirects to that anchor, and the row is highlighted when reached that way.

## 16.5 Comparison pages

A comparison page shows two to five items of the same kind side by side. Pages exist for the registered pairs of `topics/comparisons.yaml` (Estimate: 400 pairs at M3, from the pairs that readers search for, 17.10). Any other set can be built in the page at `/compare/?items=a,b,c`, client-side from the JSON records.

| Kind | Rows |
|---|---|
| method, architecture | purpose, cost, inductive biases, typical use, hyperparameters, the results where both were evaluated in the same setting, the evidence labels of their main claims |
| model | every fact of the facts box, side by side, with sources; the results on the benchmark versions and settings where both have result cards; nothing else is compared |
| benchmark | task, size, metrics, protocol, saturation, contamination, licence |
| policy | jurisdiction, scope, obligations by topic (from provisions), dates |

Rules: results of the two items appear in one row only when they are comparable under 10.6 (same benchmark version, same setting, same protocol). A comparison page never has a "winner" row, a total score or a verdict in our voice (N01).

## 16.6 Catalogue pages

Each catalogue of 05.4 has the same layout:

```
┌ Models ─────────────────────────────────────────────── 24,612 cards · 812 pages ┐
│ [search in models…]                                Data as of 2026-11-13        │
│ Facets: developer ▾  country ▾  architecture ▾  params ▾  compute ▾  access ▾   │
│         licence ▾  modality ▾  released ▾  notable ☐                            │
│ View: [table] [timeline] [chart]                          ⤓ CSV  ⤓ Parquet  ⤓ JSON│
├─────────────────────────────────────────────────────────────────────────────────┤
│ Released    Model                 Developer   Params      Compute      Access    │
│ 2026-11-04  …                     …           7.0e10      ~2e24 (est)  open      │
│ …                                                                               │
└─────────────────────────────────────────────────────────────────────────────────┘
```

- **Table view.** Sorted by the default of 05.4. Each row is a card or a node; a node row has a "page" mark. Numeric facts show their status mark. Columns can be chosen.
- **Timeline view.** Items on a time axis, swimlanes by a facet (developer, country, access). It uses the `timeline` primitive (12.4).
- **Chart view.** The register charts of 12.9 for the current filter: compute against date, parameters against date, counts per year.
- **Facets** are those of 5.5, with counts. The filter state is in the URL query, so a filtered view can be linked.
- **Download.** The current filter as CSV, Parquet or JSON, from the catalogue's dataset in `ai-data` (16.10), with the licence of each column (19.2) and the "Data as of" date.
- **No script.** The table is in the HTML, paginated at 200 rows with page links; the facets are links to precomputed pages for the main facet values (one level deep: developer, year, access). Deeper filters need the script.
- **Pagination of large catalogues.** Works (about 200,000), results (about 150,000) and models (about 25,000) are split into static pages by a primary facet (works by area and year, results by benchmark version, models by family and year). The full set is only in the downloads and in the search index.
- **Family pages** (`/models/<family>/`) are nodes that hold the card rows of their releases and checkpoints, with the family's lineage tree (16.9) at the top.

## 16.7 Results tables

A benchmark version page (`/benchmarks/<slug>/<version>/`) has the results table, the AI form of a leaderboard. It is built from result cards (10.6) and never from a hand-written list.

```
┌ SWE-bench Verified · results ────────────────────────── Data as of 2026-11-13 ┐
│ Setting: [official: 1 attempt, any scaffold] [pass@5] [with hints] [all]      │
│ Evidence: ☑ reported  ☑ replicated  ☑ measured here      Show: ☐ disputed     │
├────────┬────────────────────┬────────┬────────────┬────────────┬───────────────┤
│ Score  │ Model              │ Date   │ Setting    │ Evidence   │ Source        │
├────────┼────────────────────┼────────┼────────────┼────────────┼───────────────┤
│ 74.4 % │ …                  │2026-09 │ own agent  │ reported   │ tech report §5│
│ 70.2 % │ …                  │2026-08 │ mini-agent │ replicated │ 2 sources     │
│ …      │                    │        │            │            │               │
├────────┴────────────────────┴────────┴────────────┴────────────┴───────────────┤
│ 312 results in this version. Settings that differ are not ranked together.    │
│ Best reported score in this setting rose from 0.4 % (2023-10) to 74.4 %.     │
│ [progress chart]  ⤓ CSV  ⤓ JSON   Saturation: no (10.8)                       │
└────────────────────────────────────────────────────────────────────────────────┘
```

Rules:

- The table opens on the official protocol setting of the benchmark version (10.3), and each other setting with results has its own tab (Default, Q-41). Only results that are comparable under 10.6 share a sorted table. The "all" tab groups by setting and does not sort across groups.
- The caption line "Best reported score in this setting" is computed over the open tab, with its date range, and is drawn as the best-so-far line of the progress chart (12.9, Default, Q-38).
- Each row shows the date of the source, the setting, the evidence label of the result (07, 10.6) and the source with its locator. A row of a `measured-here` result links to the harness run (10.4).
- The position of a row is a sort, not a rank: there is no rank column, no medal and no highlighting of the top row (N01, K04).
- The caption is written by the build from the data, with the date.
- Contamination reports (10.8.2) and disputes (04.9) show a mark on the row and a note under the table.

## 16.8 The contested-topic layout

Pages about contested questions (open-problem, argument, many finding and risk pages, and sections of other pages marked `contested`, 06.9) use one fixed layout:

```
┌ Do large language models have emergent abilities? ─────────── open problem ┐
│ The question, in plain words ...                                           │
│ Why it matters ...                                                         │
│ What would settle it ... (from `would_settle_it`)                          │
│                                                                            │
│ Positions (order: by first publication)                                    │
│ ┌ A. Abilities emerge sharply with scale ─┐ ┌ B. Sharpness is a metric ─────┐│
│ │ Summary ...                             │ │   artifact                    ││
│ │ Main evidence: 4 claims [reported ...]  │ │ Summary ...                   ││
│ │ Proponents (works): ...                 │ │ Main evidence: 3 claims ...   ││
│ │ Strongest reply from other positions .. │ │ Strongest reply ...           ││
│ └─────────────────────────────────────────┘ └───────────────────────────────┘│
│ Evidence table: claim × position (supports, against, neutral), with labels │
│ Timeline of the debate                                                     │
└────────────────────────────────────────────────────────────────────────────┘
```

- Positions are ordered by the date of their first source, never by their number of sources or their standing, and the order rule is printed.
- Each position has the same parts: a summary written in that position's own terms, its main evidence as claims with labels, its proponents as works (not as a count of people), and the strongest reply from the other positions.
- On narrow screens the positions stack in the same order.
- The site's own voice appears only in the question, the "why it matters" and the "what would settle it" sections, and these are checked by rule N01 for one-sidedness by the cross-family audit (15.4).

## 16.9 Lineage views

Lineage views draw the graph edges of 04.7 as trees and DAGs, at `/lineage/<slug>/` and at the top of family pages.

| View | Edges | Example |
|---|---|---|
| model family tree | `member_of`, `succeeded_by`, `fine_tuned_from`, `distilled_from`, `merged_from` | the Llama family with its releases and the notable fine-tunes and merges that have cards |
| weights lineage | `fine_tuned_from`, `distilled_from`, `merged_from` across families | from a base model to its derived models on the Hub, limited to cards by the scope rule (18.5) |
| idea lineage | `introduced_by`, `variant_of`, `generalizes`, `uses`, `responds_to`, with the works and dates | from the perceptron to backpropagation to deep nets; from attention in translation to the Transformer to its variants |
| architecture lineage | `variant_of`, `components` | the attention variants (multi-query, grouped-query, linear, sparse, flash as an implementation) |
| argument map | `responds_to` | the Chinese room, its replies and the counter-replies |
| citation neighbourhood | `cites` between works with cards | the works that a landmark work cites and that cite it, limited to cards |

- A lineage view is a layered DAG, time on the vertical axis by default. Nodes are clickable, edges are labelled by type, and the reader can choose the edge types.
- Undisclosed lineage is shown as unknown, never guessed. An edge with a source that the developer did not confirm is dashed and marked `reported`.
- With no script, the view is a nested list in time order.
- Large lineages (more than 300 nodes) are pruned to cards with node pages and shown with a "show all" that loads the JSON.

## 16.10 The data and agent surface

Agents are a main reader (02.5). Every page and card has a machine form, the site publishes its data as datasets, and an MCP server answers with the same data. This copies programming (2150p/16.10) with AI's datasets and tools.

**Markdown twins.** Every page has a twin at `<path>.md`, written by the build from the source, with front matter (tag, kind, URL, ladders, "data as of", licence) and the body. Claims carry their labels inline as `[evidence: replicated]`, and dated facts keep their dates. Card twins are generated from the card record by one fixed function shared by the build and the Worker, with golden tests. A request with `Accept: text/markdown` gets the twin (`Vary: Accept`). Twins are `noindex` with a canonical link to the HTML.

**JSON records.** `<path>.json` is the record of any item, node or card: the front matter or card fields, the facts with sources and status, the claims with their labels and source ids, the computed ladders, the edges in both directions, the result cards that cite the item (by id), and the sources. The schema is `schema/record.v1.json`, published at `/data/schema/`. A record never contains the prose, because the prose has another licence (19.2).

**llms.txt.** `/llms.txt` follows the llmstxt.org format and lists the groups, the top areas, the catalogues, the datasets and the MCP endpoint. Each top area has `/<top>/llms.txt` with its twins, and `/<top>/llms-full.txt` with the twins of its core nodes, at most 5 MB each (Default). There is no site-wide `llms-full.txt`.

**Datasets** at `/data/`, stored in R2 `ai-data`, versioned, with a changelog, and mirrored to Hugging Face as the dataset repository `tamnd/ai`. Each monthly release (20.6) is a tag.

| Dataset | Content | Licence | Format |
|---|---|---|---|
| `items` | every node and card record | CC0 where the sources allow it, otherwise per field (19.2) | JSONL and Parquet, sharded by kind |
| `models` | the model cards with all facts and sources | as `items` (Epoch AI derived fields keep CC BY 4.0 with attribution) | Parquet, CSV |
| `results` | all result cards with settings, sources and comparability keys | as `items` | Parquet, CSV |
| `claims` | every claim with its type, label, sources and the page it is on | CC0 (the claim text is a quotation of our prose, CC BY-SA 4.0) | JSONL |
| `graph` | all edges | CC0 | JSONL, GraphML |
| `works` | the work cards with ids (arXiv, DOI, OpenAlex) | CC0 | Parquet |
| `incidents`, `policy` | incident and policy cards with provisions and dates | per source | JSONL |
| `code` | every reference implementation with its tests and its X level | MIT | JSONL and a tarball per framework |
| `experiments` | every experiment config and `results.json` | MIT, CC0 for numbers | JSONL |
| `pages` | the Markdown twins of all nodes | CC BY-SA 4.0 (prose), MIT (code) | JSONL, sharded by top area |

**Feeds.** `/feeds/` has Atom feeds of new and changed nodes per top area, new model cards, new result cards per benchmark, new incidents and policy changes, written by the daily job (14.9).

**The MCP server** is at `https://ai.tamnd.com/mcp`, with discovery at `/.well-known/mcp`. It runs in the Worker `ai-site`, is read-only, needs no account, and is rate limited per IP (Default: 60 requests per minute). Protocol versions as in programming (2150p/16.10).

| Tool | Input | Output |
|---|---|---|
| `search` | a query; optional kind, area, level, evidence, date range, limit | ranked items with URL, kind, summary, ladders, from the index of 17 |
| `get_item` | a URL, slug, tag or external id (17.4); optional `format` | the twin or the record |
| `get_facts` | an item and optional fact ids | the facts with values, sources, status and `as_of` |
| `get_results` | a benchmark version, optional setting and model filters | comparable result cards with sources and dates, and the "data as of" date |
| `get_claims` | an item, optional label filter | its claims with labels and sources |
| `lineage` | an item and edge types | the lineage DAG as JSON |
| `calc` | a calculator id and its inputs | the result of 16.14 with the formula used |

Each tool result has the item URL and the "data as of" date, so an agent can cite the page and say how fresh the fact is.

## 16.11 Search

Search is document 17. The site part: the palette (`/`, `⌘K`), the `/search/` page, the static index shards in the Worker assets, the `/x/<id>` identifier route, and the "I need to…" form on `/tasks/` (17.6).

## 16.12 Reading paths and courses

| Path type | URL | How it is made |
|---|---|---|
| **To a target** | `/paths/to/<slug>/` | computed: the topological order of the prerequisites of one node (05.7), cut at what the reader marks as known; "from zero to the Transformer", "from zero to PPO" |
| **Courses** | `/paths/<course>/` | the curated courses of 05.7 (`topics/courses/*.yaml`), with section headings |
| **Curriculum paths** | `/paths/cs2023-ai/`, `/paths/acm-ccs-ml/` | from the crosswalks (5.8): the items mapped to each knowledge unit, in the outside system's order |
| **Area paths** | `/paths/area/<top>/` | the `intro` and `core` items of an area in reading order |
| **Code paths** | `/paths/from-scratch/<target>/` | the nodes whose NumPy references build up to a target (a tokenizer, an MLP, a Transformer, a small language model, a DQN agent), in the order where each step reuses the code of the steps before |

A path page shows its steps with their ladder digits, a time estimate per step (from the page length and the level), and the reader's progress in `localStorage`, with no account. The next-step strip appears on each page of the followed path. With no script, a path page is an ordered list.

## 16.13 The home page, area pages and the timeline

- **Home.** The one sentence of 02.1; the search field; the nine groups as a map with counts of nodes and cards and their coverage level; "Start here" paths by reader (02.5); the catalogues; the newest items of the daily job with dates; the coverage dial (18). No news, no "trending", no ranking.
- **Area pages.** The sub-areas in reading order; the core items; the task index of the area; the area's register charts (12.9); the crosswalk rows (5.8); the gaps of the area (18), each with "Help write this page".
- **Timeline.** `/timeline/` is all dated items (events, model releases, landmark works, policy dates, eras of `history-of-ai`) on one axis, with facets for kind and area, zoom from centuries to days, and a list view with no script.

## 16.14 Calculators

Calculators answer the engineer's questions of 02.5 with the formulas of the nodes, not with formulas of their own. Each calculator reads its formulas from the `shapes.yaml` of an architecture node (08.6) or from a `calc.yaml` of a method or concept node, and shows the formula and its source next to the result.

| Calculator | Route | Inputs | Output | Formula from |
|---|---|---|---|---|
| Parameter count | `/calc/params/` | an architecture and its size variables, or a model card | total and per block, embedding and non-embedding | `shapes.yaml` of the architecture |
| Training compute | `/calc/training-flops/` | parameters N (or active parameters for MoE), tokens D, or an architecture with sizes and a sequence length | FLOP, with the approximation C ≈ 6ND and the exact count from `shapes.yaml` side by side; GPU-hours at a chosen hardware card and utilization | the `training-compute` concept node; `shapes.yaml` |
| Training memory | `/calc/training-memory/` | parameters, precision, optimizer, batch, sequence, sharding (none, ZeRO 1 to 3, FSDP), activation checkpointing | memory per device for weights, gradients, optimizer state and activations | the `memory-optimization` method nodes |
| Inference memory | `/calc/inference-memory/` | parameters, weight precision, batch, context length | weights plus KV cache plus activations, per device | `shapes.yaml`; the `kv-cache` node |
| KV cache | `/calc/kv-cache/` | layers, KV heads, head dimension, context, batch, precision | bytes = 2 × layers × KV heads × head dim × context × batch × bytes per value | the `kv-cache` node; `grouped-query-attention` |
| Inference compute | `/calc/inference-flops/` | parameters, prompt and output tokens | FLOP ≈ 2N per token, plus the attention term from `shapes.yaml` | `shapes.yaml` |
| Tokens and cost | `/calc/tokens/` | text (in the browser), a tokenizer card, a price card | the token count with the in-browser tokenizer (12.8), and the cost at a dated price | the tokenizer's node; price facts with `as_of` (04.9) |
| Chinchilla-style allocation | `/calc/compute-optimal/` | a compute budget | N and D under a chosen published fit, with the fit's source and its uncertainty | the `scaling-laws` finding node, one option per cited fit |
| Benchmark sample size | `/calc/eval-ci/` | the number of items, an accuracy | the confidence interval of the score, and the smallest difference that a given number of items can detect | the `statistical-testing-for-ml` nodes |

Rules:

- A calculator never has a formula that is not on a node. Rule V14 (12.14) checks that the calculator and the architecture scene give the same numbers; the X tests of 08.6 check `shapes.yaml` against the code.
- Every result shows the formula, the assumptions (for example "6ND ignores the attention term and embedding lookups"), and links to the derivation.
- Prices and hardware numbers are facts with `as_of`, printed with their date.
- Calculators run in the page with no network; with no script, each calculator page shows the formulas and a worked example.

## 16.15 Dashboards

As in algorithm (2150/14.19) and programming: `/coverage/` (the coverage levels C0 to C6 by group and area, 18), `/gaps/` (the gap list with "Help write this page"), `/universe/` (outcome counts per source), `/latest/` (the newest changes and the daily job's report), `/matrix/` (the ladder matrix: nodes by W, G, X, R, F, V, D level, filterable by area). The evidence dashboard at `/about/evidence/` shows the share of claims by label for each area.

## 16.16 Visual design, interaction and mobile

The math2 and algorithm design applies (2150/14.10 to 14.12). Additions:

- The evidence chips (16.4) and fact status marks (`estimated`, `disputed`) have fixed shapes, in the legend of `/about/evidence/`.
- Data tables use tabular figures, right-aligned numbers, the unit in the header, and the scientific form for large numbers (`3.8e25`) with a plain-words tooltip ("38 septillion FLOP").
- Area hues tint headers only; evidence chips and chart series use the fixed colour-blind-safe palette of 12.4.1.
- On mobile, the facts box folds to its first three lines, tables scroll horizontally with a sticky first column, and the positions of a contested page stack.

## 16.17 Performance budgets

`aik site check` measures these budgets on a fixed sample of 120 pages: the 30 largest node pages, 20 model and family pages, 20 benchmark version pages, 20 catalogue pages, and 30 random pages. A page over budget fails the build.

| Budget | Limit |
|---|---|
| HTML of a node page, gzip | at most 80 KB (Estimate; the three framework tabs are in the HTML) |
| HTML of a catalogue or results page, gzip | at most 150 KB (200 rows) |
| HTML of any page, uncompressed | at most 900 KB |
| CSS, gzip | at most 30 KB, one file for the site |
| JS on a page with no scene, gzip | at most 20 KB (theme, palette loader, popovers) |
| JS on a page with a scene, gzip | at most 20 KB + the `@aik/viz` runtime + one scene (12.13) |
| Largest Contentful Paint, reference phone, fast 4G | at most 1.8 s at p75 |
| Cumulative Layout Shift | at most 0.05 |

Pyodide, ONNX Runtime Web and models load only when the reader asks (12.8), and are not in the budget.

## 16.18 Accessibility, SEO and security headers

- **Accessibility.** WCAG 2.2 AA, as in algorithm (2150/14.14). Evidence chips have text names for screen readers ("evidence: replicated"); tables have headers and captions; charts have text summaries and data tables (12.12).
- **SEO and social cards.** As in algorithm (2150/14.15), with schema.org types where they fit: `DefinedTerm` for definitions, `ScholarlyArticle` for works, `Dataset` for datasets and the published datasets, `SoftwareApplication` for software, `Person` and `Organization`, `Legislation` for policies. Model and result pages use `Dataset` for their data tables. Twins and records are `noindex`.
- **Security headers.** As in programming (2150p/16.18): immutable caching of hashed assets, short caching of HTML, a strict CSP with no third-party origins, `nosniff`, a strict referrer policy, COOP and COEP on notebook pages only (12.8), `'wasm-unsafe-eval'` where notebooks run, and `Access-Control-Allow-Origin: *` on `/data/`, `.json` and `.md`.

## 16.19 Help write this page

Every card and every node below W5 has a "Help write this page" or "Improve this page" link, as in programming (2150p/16.19).

| Action | Opens | Prefilled with |
|---|---|---|
| **Write this page** (cards) | a GitHub issue form `write-page.yml` in `tamnd/ai` | the card's tag, URL, kind, area, facts and sources, its scope reason (18.5), and the `aik promote <tag>` command for people who want to send a PR |
| **Edit on GitHub** (nodes) | the `node.md` or the open framework file in GitHub's editor | none; the DCO note is in the PR template (19) |
| **Report a problem** (any) | an issue form `report.yml` | the URL, the tag, the build commit, the section, and a type: wrong fact, wrong number, out of date, wrong evidence label, one-sided, broken code, missing item, privacy |
| **Dispute a result** (result rows) | an issue form `result.yml` | the result card id, its source and setting, and fields for the reader's source |

Reports get the label `reader-report`. The work queue (14.5) takes them as gap items with high priority, and the verifier role checks a "wrong fact", "wrong number" or "dispute" report against the raw snapshots before any writer changes the page. A "privacy" report goes to the takedown process of 19 and hides nothing automatically, but is answered within 3 days (Default). The links are plain links to GitHub, with no script and no account on our side.

## 16.20 Hosting

**Decision.** The site runs on the Cloudflare Worker `ai-site` with static assets and R2 from M0, as programming does. Algorithm started on GitHub Pages and had to move after it passed the file limits at about 3,500 nodes and 19,500 files (01.6). This site has more cards than algorithm has files before M1.

**The file budget** at the scale of 02.4 (all Estimate; recompute at M1 from the universe):

| Files | Count | Size | Where |
|---|---|---|---|
| node pages | 32,000 | 2.0 GB | assets for the core and the `intro` and `core` levels (about 12,000); R2 for the rest |
| area, path, course and calculator pages | 2,500 | 60 MB | assets |
| catalogue pages (paginated, facet pages one level deep) | 30,000 | 2.5 GB | assets for the first page of each catalogue and facet (about 3,000); R2 for the rest |
| benchmark version pages with results tables | 6,000 | 500 MB | assets for the core benchmarks (about 600); R2 for the rest |
| card indexes (`/_cards/<kind>/<shard>.json`) | 1,000 | 1.4 GB | R2 |
| node twins and records | 64,000 | 1.5 GB | R2 |
| `llms.txt` and `llms-full.txt` | 110 | 300 MB | R2 |
| scene bundles, traces (`/_traces/`), figures | 30,000 | 3 GB | assets for bundles; R2 for traces and frames |
| social images | 32,000 | 2.5 GB | R2 |
| browser runtimes and ONNX models | 400 | 3 GB | R2 |
| search shards, CSS, JS, fonts | 5,000 | 200 MB | assets |
| datasets (`/data/`) | 200 | 30 GB | R2 `ai-data`, and Hugging Face |
| **total served** | **about 205,000** | **about 47 GB** | |

**The outlets.**

| Outlet | Serves |
|---|---|
| **Workers static assets** of the Worker `ai-site` on `ai.tamnd.com` (Workers Paid) | the **hot set**, at most 95,000 files (the Paid limit is 100,000 files per version and 25 MiB per file, as checked by programming on 2026-11-09; recheck at M0): the shell, the core node pages, area and path pages, first catalogue pages, core benchmark pages, scene bundles, search shards |
| **R2 `ai-site`** behind the same Worker | every other file, at its URL path, cached at the edge |
| **R2 `ai-data`** | `/data/`, with CORS |
| **R2 `ai-raw`, `ai-runs`** | not served (13, 09) |
| **GitHub Pages mirror** | none in v1 (03.7, Q-12); the release archive (20.6) is the offline copy |

`registers/site-tiers.yaml` decides which files go to the assets, by path pattern and by the core list. `aik site check` fails when the hot set passes 95,000 files or any file passes 25 MiB.

**The Worker `ai-site`** lives in `ai-agent/worker/`, in TypeScript (Estimate: about 1,500 lines). On each request it tries, in order:

1. **Static assets**, matched before the script, so most requests cost no Worker CPU.
2. **Content negotiation**: `Accept: text/markdown` on a page path serves the twin.
3. **Special routes**: `/mcp` and `/.well-known/mcp` (16.10); `/data/` from `ai-data`; COOP and COEP on notebook routes.
4. **Redirects**: `/t/<tag>`, `/x/<id>` (from the id map of 17.4), aliases, card URLs to their collection anchors (from the card indexes), renamed families, case and slash folding. The redirect map is written by the build (Estimate: about 600,000 entries, sharded and loaded per prefix).
5. **R2**: read `ai-site/<path>` and cache at the edge.
6. **404**: a search for the words of the path, and "Report a missing item" (16.19).

**Cost** (Estimate; check at M0): Workers Paid at $5/month with 10 million requests included; R2 storage of about 50 GB for `ai-site` and `ai-data` at $0.015/GB-month, about $0.75/month; no egress fee. Below $15/month at 10 million page views per month, as in 03.7.

**DNS.** `ai.tamnd.com` is a Workers custom domain on the `tamnd.com` zone. Releases are served at `<release>.ai.tamnd.com` from the prefix `releases/<release>/` in `ai-site` (20.6).

## 16.21 Deploy and rollback

`aik site deploy -out build/site -commit <sha>` follows algorithm's `deploy.sh` and programming's deploy (2150p/16.21): a manifest with hashes and tiers, a diff against `_deploys/current.json`, a commit stamp in each page footer, backups of overwritten R2 objects, R2 first and assets second, edge purges for changed paths, then the **live check**: fetch fixed probe pages and check that they show `commit/<sha>`. The probes for this site are `/dropout/` (assets), one R2 node page, one card redirect (`/models/<family>/<checkpoint>/`), one `/x/arxiv:` redirect, one results page, `/llms.txt`, one `.md` twin and one `/data/` file. It retries every 30 s for up to 10 minutes. On success it writes `_deploys/current.json`; on failure it rolls back automatically and opens an issue labelled `deploy-failed`. `aik site deploy -rollback <sha>` restores on request.

The build server runs the deploy loop every 5 minutes, as for algorithm and math2, and does nothing when the commits of `tamnd/ai` and `tamnd/ai-agent` did not change. A merge to `main` is live in 15 to 30 minutes (03.5).
