# 17. Search

Search is how most readers and agents enter the corpus.
AI search has five jobs: find an item by its name, its acronym or its alias; resolve an outside id (an arXiv number, a DOI, a Hub repo) to our item; answer "I need to…" with tasks and methods; find words in the full text; and report what people looked for and did not find, so that coverage can grow (18.6).

This document copies programming's search (2150p/17) and algorithm's (2150/15): a static, sharded index for the keystroke path, a palette, BM25F over named fields, ranking priors, an evaluation set that gates the build, and aggregated query logs.
It adds identifier search, the acronym register, the task-facet search, and a back end in the Worker for full text over cards and for agents.

## 17.1 Query types

| Type | Examples | Expected first result |
|---|---|---|
| name | `dropout`, `layer normalization`, `chinchilla scaling` | the node, or the card when there is no node |
| acronym | `GQA`, `RLHF`, `PPO`, `SAM`, `MoE`, `KV cache` | the expansion's node; for ambiguous acronyms, all senses (17.7) |
| model | `llama 3.1 405b`, `gpt-4o`, `qwen2.5 coder 32b` | the model card or the release node |
| model fact | `llama 3 70b parameters`, `gpt-3 training compute` | the model, with the fact shown in the result line |
| benchmark | `swe-bench verified`, `mmlu pro`, `imagenet top-1` | the benchmark version page |
| identifier | `1706.03762`, `doi:10.1038/nature16961`, `meta-llama/Llama-3.1-8B`, `aiid 1625`, `Q25339462` | the item with that id, or a "not in the corpus" result (17.4) |
| work | `attention is all you need`, `vaswani 2017` | the work card or node |
| person or organization | `hinton`, `fei-fei li`, `deepmind` | the person or organization card |
| policy | `eu ai act article 50`, `executive order 14110`, `nist ai rmf govern` | the provision card, else the policy |
| question | `why does batch norm help`, `how is the kv cache size computed` | the node that answers it; the section anchor when the full text matches a heading |
| task | `classify images with few labels`, `detect objects in lidar` | the task node; "I need to…" for the structured form (17.6) |
| formula or symbol | `softmax(QK^T/sqrt(d))`, `ELBO`, `6ND` | the node whose notation register (06.4) has the formula or symbol |
| neighbour item | `k-means`, `quicksort`, `kl divergence`, `numpy` | the bridge card, with the "on algorithm.tamnd.com" mark (5.9) |

## 17.2 Indexed entries and fields

**Entries** (Estimate at the scale of 02.4):

| Entries | Count | Indexed text |
|---|---|---|
| nodes | 32,000 | all fields, and the full text from stage S2 (17.11) |
| cards | 480,000 | title, aliases, ids, facts as text, the summary of at most 30 words |
| bridge cards | 3,000 | title, aliases, the owner site |
| sections of node pages | about 250,000 headings | the heading text, with the node as parent |
| known-missing ids | about 4 million (the `row-only` works of 18.5, and other universe rows) | ids only, in the id map (17.4) |
| **searchable items** | **about 515,000** | |

**Fields and weights** (BM25F; Default, tuned on the evaluation set of 17.10):

| Field | Weight | Source |
|---|---|---|
| `title` | 10 | the front matter or card title |
| `aliases` | 8 | aliases from front matter, cards and `registers/aliases.reg` |
| `acronyms` | 8 | `registers/acronyms.yaml` (17.7) |
| `ids` | exact only | `same_as` (04.5, 13.8), and the tag |
| `summary` | 4 | the one-sentence answer or the card summary |
| `facts` | 3 | the facts as text: "developer Meta", "released 2024-07", "parameters 405B" |
| `headings` | 2 | section headings of node pages |
| `notation` | 2 | the formulas and symbols of the node's notation register entries |
| `area` | 1 | the area path names |
| `body` | 1 | the prose of node pages (S2 and later) |

The keystroke index (17.5) holds only `title`, `aliases`, `acronyms`, `ids` and a short `summary`. The other fields are in the full-text back ends.

## 17.3 The palette

`/` or `⌘K` opens the palette on any page. It searches as the reader types, from the keystroke index, with no request to a server.

```
┌──────────────────────────────────────────────────────────────────────────────┐
│ 🔍 gqa                                                          esc to close │
├──────────────────────────────────────────────────────────────────────────────┤
│ Methods                                                                      │
│ ▸ Grouped-query attention              method · core · D. Deep learning      │
│   Share each key and value head among a group of query heads.                │
│ Works                                                                        │
│   GQA: Training generalized multi-query transformer models … (2023)  work    │
│ Benchmarks                                                                   │
│   GQA (visual question answering)     benchmark · 2019                       │
│ Calculators                                                                  │
│   KV cache size                       /calc/kv-cache/                        │
├──────────────────────────────────────────────────────────────────────────────┤
│ ↑↓ move  ↵ open  ⇥ filter by kind  "m:" models  "b:" benchmarks  "#" ids      │
└──────────────────────────────────────────────────────────────────────────────┘
```

- Results are grouped by kind, at most 3 per group and 12 in total, with the group order set by the ranking of each group's first result.
- Each result line shows the title, the kind, the level when it is a node, the area, and one line: the summary, or the matched fact for a model fact query ("parameters: 405B, primary-only").
- **Prefixes** filter by kind: `m:` models, `b:` benchmarks, `d:` datasets, `w:` works, `p:` people and organizations, `l:` policy and provisions, `i:` incidents, `t:` tasks, `c:` calculators. `#` forces identifier mode.
- **Level view.** As in algorithm (2150/15): a reader who chose a level in the settings (intro, core, advanced) sees nodes of that level ranked higher, and nothing is hidden.
- Enter on the first result opens it; the full results page is `/search/?q=…`.
- With no script, the header search field is a form that sends `GET /search/?q=…`, and the Worker answers with a results page built from the same index (17.11).

## 17.4 Identifier search

Every outside id in `same_as` (13.8) and every tag resolves.
The build writes the **id map**: one row per id, from a normalized id to a tag and a URL, or to a universe row with its outcome (18.3) when the id names an item that has no card.

| Id | Patterns recognized (normalized form) | Example | Resolves to |
|---|---|---|---|
| tag | `[0-9A-HJKMNP-TV-Z]{6}`, also `ai://TAG` | `5TQH2W` | any node or card |
| arXiv, new form | `\d{4}\.\d{4,5}(v\d+)?`, with optional `arxiv:` or an `arxiv.org/abs/` or `/pdf/` URL | `1706.03762`, `2307.09288v2` | the work; the version is dropped for lookup and shown |
| arXiv, old form | `[a-z-]+(\.[A-Z]{2})?/\d{7}(v\d+)?` | `cs/0112017`, `math.ST/0406077` | the work |
| DOI | `10\.\d{4,9}/\S+`, also a `doi.org` URL, lower-cased | `10.1038/nature16961` | the work; arXiv DOIs (`10.48550/arXiv.…`) map to the arXiv id |
| OpenAlex | `[WAIST]\d{4,}`, also an `openalex.org` URL | `W2626778328` | work, person, organization, or a crosswalk row |
| Semantic Scholar | `s2:` and the corpus id, or a `semanticscholar.org` URL | | the work |
| DBLP | a DBLP key, or a `dblp.org` URL | `conf/nips/VaswaniSPUJGKP17` | the work, or a person |
| ACL Anthology | `\d{4}\.[a-z-]+\.\d+`, or `[A-Z]\d{2}-\d{4}` | `2023.acl-long.1`, `P19-1001` | the work |
| OpenReview | `openreview:` and a forum id, or an `openreview.net/forum?id=` URL | | the work |
| Hugging Face | `org/name`, with optional `hf:`, `datasets/`, or a `huggingface.co` URL; case-insensitive match | `meta-llama/Llama-3.1-405B` | the model or dataset card; a Hub repo without a card resolves to its universe row |
| GitHub | a `github.com/org/repo` URL | `github.com/karpathy/nanoGPT` | the software card, or the work whose code it is |
| Wikidata | `Q\d+`, or a `wikidata.org` URL | `Q25339462` | any item |
| ORCID | `\d{4}-\d{4}-\d{4}-\d{3}[\dX]` | | the person card |
| ROR | `0[a-z0-9]{8}`, with `ror:` or a `ror.org` URL | | the organization card |
| AIID | `aiid` and a number, or an `incidentdatabase.ai/cite/` URL | `aiid 1625` | the incident card |
| Epoch AI | the model name as in the database, with `epoch:` | `epoch:Llama 3.1-405B` | the model card |
| CELEX, ELI | `\d{5}[A-Z]\d{4}`, or an `eur-lex.europa.eu` or ELI URL | `32024R1689` | the policy |
| legal reference | a policy name or alias with an article, annex, section or recital number | `ai act art. 50`, `eo 14110 sec 4.2` | the provision card |
| standard | `ISO/IEC \d+(-\d+)?(:\d{4})?`, `IEEE \d+` | `ISO/IEC 42001:2023` | the standard |
| SPDX | an SPDX licence id | `Apache-2.0` | the licence page of `/about/` (19.7) |

**Behaviour.**

- A query that matches an id pattern is looked up in the id map first. An exact hit opens the item, and the palette shows it as the only result with the id beside it.
- An id that the map knows as a universe row with no card shows a "Known, not in the corpus" result with its outcome (`row-only` with the scope rule of 18.5 that excluded it, `pending`, or `out-of-scope` with its reason), the outside link, and "Report a missing item" (16.19).
- An id that matches a pattern but is not in the map shows "Not known", the outside link built from the pattern, and the report link. The miss is logged as an id miss (17.9).
- `/x/<scheme>:<id>` (16.2) uses the same map in the Worker and redirects with 302 to the item, or to the "known, not in the corpus" page.
- **Size** (Estimate): about 6 million ids (the 515,000 items with an average of 3 ids, plus the `row-only` rows). The id map for the browser has only the ids of items with a card or node (about 1.6 million, sharded by a hash of the normalized id into 1,024 shards of about 12 KB gzip each). The Worker holds the full map, sharded the same way, in R2.

## 17.5 The static index

The keystroke path runs in the browser, from static shards in the Worker assets (16.20), as in programming.

| Part | Content | Size (Estimate) |
|---|---|---|
| `search/meta.json` | the shard list, the build commit, the field weights, the acronym register, the normalization tables | 60 KB gzip |
| `search/core.json` | the entries of the about 12,000 nodes of level `intro` and `core` and the 2,000 most searched cards (17.9), with all keystroke fields | 250 KB gzip, loaded when the palette opens |
| `search/p/<xx>.json` | prefix shards: every entry under the first two characters of each normalized title, alias and acronym token, with tag, kind, title, URL, level, area, a 100-character summary and the prior (17.8) | about 1,300 shards, 30 KB gzip on average, at most 120 KB; about 40 MB in total |
| `search/id/<hhh>.json` | the browser id map (17.4) | 1,024 shards, 12 MB in total |
| `search/x/` | the S2 full-text index of node pages (17.11) | as built by Pagefind |

- The palette loads `meta.json` and `core.json` when it opens, then the one prefix shard of the first two characters typed. A query of one character searches `core.json` only.
- Prefix shards that pass 120 KB are split by the third character. `aik site check` fails when a shard passes 200 KB.
- The shards are hashed in their names and cached as immutable assets. A deploy that changes an entry changes only the shards it touches.
- Card entries carry no prose, only the fields of the card record, so the index has the licence of the records (19.2).

## 17.6 "I need to…"

The task form is on `/tasks/` and in the palette (as the command "I need to…"). It answers a practical question with tasks, then methods and models, using the task facets of 05.3.

```
┌ I need to… ──────────────────────────────────────────────────────────────────┐
│ [ find objects in drone video with only a few labelled frames            ]   │
│ Understood as:  modality video ▾   output boxes ▾   supervision weak ▾        │
│                 setting offline ▾   compute any ▾                              │
├──────────────────────────────────────────────────────────────────────────────┤
│ Tasks                                                                         │
│   Video object detection        video → boxes + labels   mAP   ImageNet VID   │
│   Few-shot object detection     image → boxes            nAP   COCO few-shot  │
│ Methods used for these tasks (by evidence, then by number of results)         │
│   Semi-supervised detection (teacher-student)   reported, 41 results          │
│   Open-vocabulary detectors                     reported, 120 results         │
│   Fine-tuning a pretrained detector             replicated, measured here     │
│ Models with results on these benchmarks   ⟶ results tables                    │
│ Read first: object detection · transfer learning · semi-supervised learning   │
└──────────────────────────────────────────────────────────────────────────────┘
```

- **Parsing.** The free text is mapped to facet values by a fixed, tested rule set (`registers/task-words.yaml`: words and phrases to facet values, such as "few labelled" to supervision `weak`, "lidar" to modality `3D`, "rank" to output `ranking`). The reader sees the parsed facets as dropdowns and can change each of them. No language model runs in the browser or behind the form (Default). A text that maps to no facet falls back to the plain search of 17.3, restricted to tasks.
- **Matching.** Tasks match when each chosen facet value is in their keys (05.3). They are ranked by the number of matching facets, then by the plain text score of the remaining words against the task fields.
- **Methods.** For the matched tasks, the form lists the method and architecture nodes linked to them by the edges of 04.7 (`solves`, and `evaluated_on` through the result cards of their models). Each has its evidence summary and its result count. The order rule is printed, and it never uses a score of one method against another: "best method" is never computed or written (K04, N01).
- **Models and results.** The models with result cards on the main benchmarks of the matched tasks, each linked to the results table (16.7), never listed in a ranked order on this form.
- **Compute.** The `compute_hint` facet filters by the compute class of the common methods, and links the calculators (16.14).
- **Agents.** The MCP `search` tool accepts the facets as parameters (17.12).

## 17.7 Normalization, acronyms and synonyms

The same normalizer runs in the build, the browser and the Worker (one TypeScript module with golden tests, compiled for both; the Go build calls it through the test vectors to check that its own port agrees).

- Unicode NFKC, case folding, accents removed for matching and kept for display.
- **Greek and math.** Greek letters fold to their names (`α` and `alpha`, `ε-greedy` and `epsilon greedy`), `^T` and `ᵀ` to `T`, `√` to `sqrt`, `·` and `×` to `*`, and the notation register (06.4) adds the spoken form of each symbol.
- **Hyphens, spaces and joins.** `pre-training`, `pre training` and `pretraining` match each other; so do `fine-tuning`, `finetuning` and `fine tuning`; `word2vec` and `word 2 vec`.
- **Spelling.** British and American forms match (`regularisation`, `normalise`, `modelling`).
- **Numbers and sizes.** `gpt4`, `gpt 4` and `gpt-4` match; `70b` and `70 b` match a parameter fact of 70 billion; `3.5`, `3-5` and `3_5` match in model names; `1e25` and `10^25` match in compute facts.
- **Stemming.** Light: plural and `-ing` forms only, with a stop list for terms where the plural is another item (`attention` and `attentions` are one; `transformers` the library and `transformer` the architecture are not, and both are shown).
- **Typos.** Edit distance 1 for tokens of 5 to 8 characters and 2 for longer tokens, applied only when the exact and prefix searches find fewer than 3 results, and never to ids.

**The acronym register** is `registers/acronyms.yaml`, written by the cataloguer (14.2) from the aliases of the items and the harvest (Estimate: 9,000 acronyms at M3). AI reuses acronyms often, so each entry lists all its senses:

```yaml
SAM:
  - {target: segment-anything, note: "Segment Anything Model, 2023"}
  - {target: sharpness-aware-minimization, note: "optimizer, 2020"}
PPO:
  - {target: proximal-policy-optimization}
RAG:
  - {target: retrieval-augmented-generation}
GQA:
  - {target: grouped-query-attention}
  - {target: benchmarks/gqa, note: "visual question answering benchmark, 2019"}
```

An acronym with one sense ranks its target first. An acronym with more than one sense shows all senses at the top, in the order of their priors (17.8), with the note.

**Synonyms.** `registers/aliases.reg` holds the aliases that are names of the same item (`backprop`, `backpropagation`; `LayerNorm`, `layer normalization`). Names that are close but not the same item are not synonyms: they are separate items, and the disambiguation rules of 5.11 apply.

## 17.8 Ranking

The score of an entry is its BM25F score times its prior, after the exact rules:

1. An exact id hit (17.4) is first.
2. An exact match of the whole query with a title, alias or acronym is above every partial match. When several items match exactly, the prior orders them.
3. Otherwise, BM25F × prior.

The **prior** is a product of factors, computed by the build and stored in each entry (Default values, tuned on the evaluation set):

| Factor | Value |
|---|---|
| core list (5.10) | 1.5 |
| node with a page | 1.3; a card is 1.0 |
| level matches the reader's level setting (17.3) | 1.2 |
| writing level W4 or W5 | 1.1 |
| grounding level G2 or G3 | 1.1 |
| notable model (the Epoch AI "notable" flag or the scope rule of 18.5) | 1.2 |
| recency, for models and benchmark versions only | from 1.0 for the oldest to 1.15 for the newest, linear in the release date within the family; never applied across families |
| demand (17.9): searched at least 50 times in the last 90 days | 1.1 |
| bridge card | 0.6, unless the query is a neighbour prefix (`al:`, `m2:`, `pg:`) |
| superseded version (a benchmark version or a model checkpoint with `succeeded_by`) | 0.8 |

Rules:

- No factor uses the developer, the country, the lab or the author of an item, and no factor uses a score from the results register. Search never ranks models by their results (N01).
- Click data is not a factor. The demand factor counts searches that named the item, not clicks, so the ranking does not feed on itself.
- The prior table is in `registers/search.yaml`, and a change to it must pass the evaluation gates (17.10).

## 17.9 Query logs and gaps

The Worker logs search for one purpose: to find what is missing or hard to find.
The logs follow programming's rules (2150p/17), with the defaults of Q-42.

- **What is logged.** The normalized query, the kind filter, the number of results, whether a result was opened and its rank, and the day. Never an IP address, a cookie, a user agent, or the full URL of the page the reader was on.
- **Sessions.** A session is a random id that the palette makes in `sessionStorage` and sends with the query. It is never stored with the query: the Worker keeps a count of distinct session ids per query and day, salted daily, and drops the salt at the end of the day.
- **The 5-session rule.** A query string is kept in the aggregate only when at least 5 distinct sessions searched it in the window of 30 days. Rarer queries are counted but their text is dropped. Id misses (17.4) are kept from 1 session, because an id is not personal text, after the id normalizer confirms that the query is only an id.
- **Opt-out.** A browser that sends `Sec-GPC: 1` or `DNT: 1` is not logged. The settings have a "Do not log my searches" switch, kept in `localStorage`, that the palette sends as a header.
- **Retention.** Daily counts for 30 days in D1 (17.11); then only the monthly aggregates of the queries that passed the 5-session rule, published as `search-gaps` in `ai-data` (16.10).
- **Gaps.** Each week, the `gaps` job of 14.9 turns the aggregate into work:

| Signal | Meaning | Action |
|---|---|---|
| zero results, at least 5 sessions | something people look for that the corpus does not name | a `pending` universe row when the query names an item (18.3); otherwise a new alias candidate for the cataloguer |
| results but no click, at least 10 sessions | the item exists but its name or summary does not match how people say it | an alias or summary review for the curator |
| an id miss | an artifact we do not know | a lookup by the harvester (13), then a universe row with an outcome |
| a click on a low result (rank 5 or below), at least 10 sessions | the ranking is off for this query | an entry in the evaluation set (17.10) with the clicked item as the expected result, after review |
| a frequent query that names a card | readers want this item | a demand signal for promotion (18.6) |

These are the search misses of 18.6, and the coverage report counts them.

## 17.10 Evaluation

`registers/search-eval.yaml` is the evaluation set. `aik search eval` runs it against the built index, in the same code as the browser, and the build fails when a gate fails.

**The set** (Estimate: 4,000 queries at M3):

| Source | Count | Expected result |
|---|---|---|
| titles and aliases of core nodes, typed with a typo or a case change | 1,000 | the node at rank 1 |
| acronyms with one sense, and with several | 800 | the target at rank 1, or all senses in the top 5 |
| ids of each scheme of 17.4 | 600 | the item, exactly |
| model and benchmark names with versions and sizes | 600 | the exact release or version at rank 1 |
| questions and tasks written by the evaluator role and reviewed | 700 | the answering node in the top 3 |
| queries from the logs (17.9), reviewed | 300 | the clicked item in the top 3 |

**Gates** (Default):

| Gate | Threshold |
|---|---|
| exact title, alias and id queries: success at 1 | at least 0.99 |
| acronym queries: success at 1 for one sense; all senses in the top 5 for several | at least 0.95 |
| model and benchmark version queries: success at 1 | at least 0.95 |
| question and task queries: success at 3 | at least 0.80 |
| the whole set: mean reciprocal rank at 10 | at least 0.85, and no drop of more than 0.01 from the last release |
| keystroke latency on the reference phone, shard in cache | at most 30 ms at p95 |
| first results after the palette opens, cold, fast 4G | at most 400 ms at p75 |
| palette transfer before the first result | at most 350 KB gzip (`meta.json`, `core.json`, one prefix shard) |

A query that fails is reported with the expected item, the actual top 5 and their scores, so that a curator can fix an alias, a summary or the prior table.

## 17.11 Stages and back end

| Stage | When | What | Where |
|---|---|---|---|
| **S1: keystroke** | M0 | the static index of 17.5, the palette, the id map, the no-script results page served by the Worker from the same shards | Worker assets; the Worker for `/search/` and `/x/`; the query logs of 17.9 in the D1 database `ai-search` |
| **S2: node full text** | M1 | Pagefind over the HTML of the node pages, loaded on the results page and when the palette has fewer than 3 keystroke results | Worker assets (`search/x/`), about 2,500 files (Estimate) |
| **S3: full text and agents** | M3 | D1 with FTS5 over all entries with all fields of 17.2, the card text and the twins; the `search` MCP tool and `GET /search.json` | the Worker `ai-site`, the D1 database `ai-search` (Estimate: 4 GB, inside the 10 GB limit of a D1 database; check at M2) |
| **S4: related** | after M3, if Q-40 is settled yes | vector search over node summaries and sections with a pinned open-weights embedding model; shown only as "Related" under the lexical results, never mixed into them | Cloudflare Vectorize, with the embeddings built by the build server and the model id in `registers/models.yaml` |

- The keystroke path stays static at every stage (Default, Q-39): the palette never waits for the Worker. S3 adds results under the keystroke results when the reader asks for "all results" or presses enter.
- S3 is rebuilt from the build output on each deploy, as a new D1 database that the Worker switches to after the live check (16.21), so a failed build never leaves a half-written index.
- **Cost** (Estimate; check at M3): D1 storage of 4 GB and about 2 million reads a day, within the Workers Paid allowances or a few dollars a month; Vectorize for about 300,000 vectors of 768 dimensions, below $2 a month.

## 17.12 Search for agents

Agents use the same index and the same ranking as people, through three doors:

- **MCP** (16.10): `search` takes `query`, and optional `kind`, `area`, `level`, `evidence`, `modality`, `output_type`, `supervision`, `setting`, `compute_hint`, `as_of_after`, `limit` (at most 50). `get_item` accepts any id of 17.4, so an agent with an arXiv number or a Hub repo gets our item, or the "known, not in the corpus" record with its outcome.
- **HTTP**: `GET /search.json?q=…&kind=…` with the same parameters, from S3 (before S3, the Worker answers from the keystroke shards, with the fields of 17.5 only). CORS is open; the rate limit is that of the MCP server (16.10).
- **Files**: the shards of 17.5 and the `items` dataset (16.10), for agents that index the corpus themselves.

Each result has the URL, the tag, the kind, the title, the summary, the ladder digits, the evidence summary for nodes, the "data as of" date, and the URLs of the `.md` and `.json` twins. Agent queries are logged under the rules of 17.9, with the door (MCP, HTTP) kept as a category and never the user agent.
