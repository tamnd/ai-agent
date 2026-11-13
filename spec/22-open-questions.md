# 22. Open questions

Each decision that is not settled has an id, the options, the default that the spec uses until it is decided, and the latest date by which it must be decided.
Elsewhere in the spec, "Default" points here.
A decision is recorded by changing its row to "Decided" with the date and a link to the pull request, and by updating the documents that use it.

## 22.1 Questions of the backbone and of 14, 18, 20 and 21

| Id | Question | Options | Default | Decide by |
|---|---|---|---|---|
| Q-01 | The name of the binary | `aik`; `ai`; `aikb` | `aik`, because `ai` collides with many tools and shell aliases, and a short distinct name is easy to type, as `algo` was | M0 (2026-12) |
| Q-02 | The URI scheme | `ai://`; `aik://`; `tamnd-ai://` | `ai://`, distinct from `al://`, `m2://` and `pg://` (04.3) | M0 |
| Q-03 | Where cards live | in `tamnd/ai/cards/` in git (20.3); a separate repository `tamnd/ai-cards`; only in R2 as Parquet with manifests in git | in `tamnd/ai`, sharded under 5 MB, without LFS, so that one PR can change a node and its cards; revisit if the repository passes 6 GB (20.3) | M1 (2027-03) |
| Q-04 | The notable-model rule for model nodes | Epoch notable or frontier, or introduced a method that has a node, or named by canon or frontier sources (18.5 S-M4); only Epoch notable; by a downloads threshold | S-M4 as written; about 2,000 model nodes, mostly families | M1 |
| Q-05 | Port `work/` from Python to Go | keep Python; port to Go; port only the scripts that CI runs | keep Python; a port is allowed when it passes the same tests (03.1) | M2 (2027-07) |
| Q-06 | The Hub threshold for model and dataset cards | 10,000 downloads in 30 days or 100 likes (models), 1,000 or 50 (datasets); a top-N by downloads; all repositories with a model card | the thresholds of S-M2 and S-D1, base and instruction models only, at most about 20,000 model repositories; M1 measures the count | M1 |
| Q-07 | The threshold for work cards | top 2% by citations per year and subfield (S-W1); top 5%; every work with a venue | top 2%, plus every work cited by our items (S-W2) and every award paper (S-W3); about 200,000 cards | M1 |
| Q-08 | What an audit does when the other model family is unavailable | wait; use the strongest model of the same family; use a third family | wait, unless the writer was of the other family; add a third family as a fallback when one is pinned in `models.yaml` (14.4) | M0 |
| Q-09 | Server-side execution for readers | none in v1; a sandbox for NumPy only; a sandbox for all tier-1 frameworks | none in v1; readers run code and small models in the browser (12.8) | M4 (2028-06) |
| Q-10 | Auto-merge of daily-job PRs | merge when `aik check` passes and no fact changed past its tolerance; always a person; never auto-merge for policy cards | auto-merge as stated in 14.9, except policy and provision cards, which need a person for the first harvest of a jurisdiction | M1 |
| Q-11 | Publish agent transcripts | not public (14.7); public for nodes whose sources are all of open classes; public after redaction of source snapshots | not public; run records (`prov/runs/`) are public, and transcripts are kept in `ai-runs` for audit | M2 |
| Q-12 | A GitHub Pages mirror | none; a Pages mirror of the core; the release archive as the offline copy | none in v1; the release archive and the release sites `<YYYY-MM>.ai.tamnd.com` (20.6) | M0 |
| Q-13 | The core rule | two or more canon works, plus the frontier core (5.10); three canon works; editor curation only | two or more of the 19 canon works, plus the frontier core recomputed yearly; approved by editors at M1 | M1 |
| Q-14 | Person nodes for living people | only with a reviewer's approval and public professional facts; cards only; no living people | cards for all of S-P1 with public professional facts only; nodes only with a reviewer's approval (18.5 S-P2) | M1 |
| Q-15 | Where course progress is stored | the reader's browser only; an account; none | the browser only, as in programming (5.7) | M2 |
| Q-16 | Corrections and pages from developers about their own models and systems | accept as PRs with a disclosed affiliation, reviewed by an independent reviewer; accept facts only; refuse | accept facts with a primary source and a disclosed affiliation (`prov` records it); prose about the developer's own system needs an independent reviewer; positions on contested topics from developers count as one source, never as the site's voice | M2 |
| Q-17 | Languages other than English | English only until M5; machine translation of the core; human translation | English only until M5, then decide (21.7) | M5 (2028-12) |
| Q-18 | Where the `row-only` universe rows live | Parquet in R2 `ai-data` with manifests in git (20.3); all in git; not kept | Parquet in R2, so that git holds only rows with other outcomes; search finds works by id from the Parquet index (17) | M1 |
| Q-19 | Review of contested topics | one reviewer with a disclosure; two reviewers of different positions; an editor panel | a reviewer who is a proponent of one position on record discloses it and cannot be the only reviewer (06.9, 15.5); the position list is approved by the area's editors | M2 |
| Q-20 | The two-model rule in the catalog phase | two catalog runs of different families must agree, and single-run items go to the curator with a flag (14.3); one run; three runs | two runs of different families, as programming did | M1 |

## 22.2 Questions from other documents

These ids are used by 06 to 13 and 15 to 19.
Each row gives the question as its document states it; the document holds the detail.

| Id | Question | Options | Default | Decide by |
|---|---|---|---|---|
| Q-21 | Which PDFs the raw store keeps (13.6) | the PDFs of all work cards; only of cited works; only extracted text | only works that a node, card, result or fact cites, plus their extracted text; this is the main cost of `ai-raw` | M1 |
| Q-22 | The database right and card fields from sources with database rights (19.2) | facts and names with attribution, insubstantial extraction per source; licence per field; ask each source | the rules of 19.2: facts and names, per-field licences, never a substantial extraction from a protected database without permission | M1 |
| Q-23 | The terms of the AI Incident Database snapshots and other sources marked "checked at M1" (13.2, 19.3) | C (reuse with attribution and share-alike); D (facts only) | D by default until the check; C when the terms allow | M1 |
| Q-24 | Hazardous content in security and safety areas (19) | `hazard: review` and a person review before publishing; no code for attacks; no restriction | `hazard: review` on pages under `robustness-security-and-privacy` and `ai-safety-and-alignment` that describe attacks | M1 |
| Q-25 | The disclosure of AI-generated text (19) | per-page labels and a site statement written to meet the EU AI Act Article 50 disclosure; a site statement only | per-page labels, a site statement, and the same label in the `.md` and `.json` twins, whether or not Article 50 applies | M0 |
| Q-26 | The headline evidence label when a claim has several (07.4) | a fixed order beginning with `proved`; the strongest by evidence; show all | the fixed order of 07.4 | M1 |
| Q-27 | How many claims one sentence may hold (07.3) | one; at most two; any | at most two | M1 |
| Q-28 | Whether a citation that the check can only match by table of contents counts for G2 (07.5) | only with a reviewer's attestation; never; always | only with a reviewer's attestation in a review record | M2 |
| Q-29 | Whether reviewers who work for a developer may review its pages (15.5) | yes, with interests stated; facts only; never | facts (D3) only, with interests stated; never W5 or contested sections | M2 |
| Q-30 | When a position has a place on a contested page (06.9, 07.7) | the source classes of 07.4 or a named official body, and editors can add one with a reason; a citation count; editors only | as 07.7: three or more serious sources, or one named official body; editors can add a position with a recorded reason | M2 |
| Q-31 | Which models the benchmark harness may run for `measured-here` results (10.4) | open models up to 14 billion parameters at 16 bits and up to 70 billion at 4 bits; closed models through paid APIs; none | open models within the limits of 10.4; never closed models or paid APIs (rule M06) | M3 (2027-12) |
| Q-32 | The GPU pool provider and multi-GPU experiments (09.4) | one on-demand cloud provider; several; owned hardware; multi-GPU in scope | one on-demand provider, revisited at M3; multi-GPU and multi-node out of scope in v1 | M3 |
| Q-33 | The default seed counts by budget class (09.3) | 10, 5 and 3 for `cpu-small`, `gpu-small` and `gpu-medium`; from a power analysis only | 10, 5 and 3, with `aik exp power` raising them when a pilot shows too little power | M2 |
| Q-34 | The Lean and Mathlib pins (11.2, 11.11) | shared with algorithm and bumped together monthly; independent | Lean v4.35.0 and Mathlib at the matching tag, shared with algorithm, bumped together | M0 |
| Q-35 | Papers with Code dump rows whose number the citation check cannot find (10.6) | keep as `pwc-unverified` and hide from tables; drop; show with a mark | keep as `pwc-unverified`, not shown in tables | M1 |
| Q-36 | The scope of tier-2 frameworks (08.2) | the core architectures and metrics from M3; all nodes with code; none in v1 | the core architectures and metrics from M3, about 400 nodes | M3 |
| Q-37 | The size limit of in-browser models (12.8) | 50 MB by default and up to 300 MB behind an explicit download; 100 MB; none | 50 MB, and up to 300 MB behind "Download" | M2 |
| Q-38 | The "best reported score" line of results tables and charts (12.9, 16.7) | computed over the open setting tab, with its date range; not shown | computed best-so-far over the open tab, with its date range | M2 |
| Q-39 | The search back end (17.11) | a static sharded index only; a Worker with D1 FTS5 (`ai-search`) for full text; both | a static sharded index for the keystroke path, plus a Worker with D1 FTS5 for full text and agents from M3 | M1 |
| Q-40 | Semantic search with embeddings (17.11) | off; on from the start; on at M3 | off until M3, then a pinned open-weights embedding model, with results shown only as "Related" | M3 |
| Q-41 | Which setting a results table opens on (16.7) | the official protocol setting of the benchmark version, with a tab for each other setting; all settings mixed | the official protocol setting, other settings as tabs; only comparable results share a sorted table (10.6) | M2 |
| Q-42 | Logging of search misses (17.9) | aggregated counts with a 5-session threshold, honouring GPC and DNT, with an opt-out switch; no logging; full query logs | aggregated, 5-session threshold, GPC and DNT honoured, opt-out switch | M1 |

## 22.3 How a question is decided

- An editor opens a PR that changes the row to "Decided: <option>, <date>" and updates every document that uses the id (`grep Q-NN`).
- A decision that changes a schema or a register also bumps its version (20.1).
- A question that passes its "decide by" date without a decision keeps its default, and the release notes say so.
- A new question gets the next free id and a "decide by" date no later than the next milestone.
