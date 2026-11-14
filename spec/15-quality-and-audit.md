# 15. Quality and audit

This document says how the site stays correct: the badges a reader sees, the threats that the checks defend against, the catalogue of audit rules with their severities and waivers, the cross-family audit and the verifier, human review, sampling, checklists, corrections, and the numbers that show whether it all works.
It copies algorithm's 13 and programming's 15, and adds what an AI library needs: claims, dated facts, neutrality and the risks of a library that is mostly written by models about models.

## 15.1 Badges

A badge is a short computed statement about a page, shown in its evidence bar (16.4) and in catalogues.
No one writes a badge (T05). The `status` package (03.4) computes them from the ladders of 04.10 at each build, for the current hashes of the files.

| Badge | Computed when | Meaning on `/about/badges/` |
|---|---|---|
| **complete** | the node reaches the targets of its kind in `schema/targets.v1.yaml` (04.10) | every part this kind of page should have is there and checked |
| **reviewed** | W5 | a person who knows the subject reviewed this exact text |
| **grounded** | G3 | every claim was found in its stored source, and a second model judged each one supported |
| **proved** | F2 or above for the main theorem of a theorem node, or every mathematical claim of the page `proved` (07.4) | the mathematics of the page is checked by Lean or by an audited full derivation |
| **runs everywhere** | `numpy` and `pytorch` at X3 and `jax` at X2 (08.10) | the code is tested in the three tier-1 frameworks against one reference |
| **reproduced here** | R2 or above on every claim with a small-scale form (09.11) | our own experiments test the page's claims at small scale |
| **facts checked** | D2 for artifact kinds, D3 shows as "facts reviewed" (10.11) | the numbers of the facts box have sources that agree |
| **contested** | the page has a contested claim in its main section, or it is a contested question (06.9) | read the positions and their evidence |
| **review due** | a claim is past its `review_after` date or freshness window (07.9) | some facts may have changed |
| **correction** | an open S1 or S2 finding on the page (15.8) | a known error is being fixed |

Rules, copied from algorithm:

- **No red crosses.** A page shows what it has, never a list of failures. The missing parts are in the "what this page needs next" line of the evidence bar (06.10) and on the coverage dashboards (16.15).
- **Hashes, not dates.** A badge is true for a set of file hashes. A change to any file that the badge depends on removes the badge until the checks pass again; a review badge needs a new review (15.5). The page shows the hash and the date of the build that computed each badge.
- **Badges never say "best" or "most".** No badge ranks a page against another.
- `/about/badges/` explains every badge in plain words, and the `.json` twin of a page carries the badges with the hashes they were computed for.

## 15.2 The threat model

A library written mostly by models, about a field that changes every week, has its own ways to be wrong.
Each row names a failure, how it happens, and the checks that stop it.

| Threat | How it happens | Defences |
|---|---|---|
| invented paper or source | the writer cites a paper that does not exist, or a real arXiv id with the wrong title | G04 resolves every identifier and matches title and authors; the citation check needs a snapshot (07.5); an invented source is a blocker in the audit (14.6) |
| invented or wrong attribution | "introduced by X in 2015" when it was Y in 2013 | K12 for "first" claims; historical claims need two sources (07.2); the audit's facts check; sampling of History sections (15.6) |
| wrong numbers | a parameter count, a score, a date or a FLOP count that the source does not give | D01 (no bare number), the citation check finds every number in its span (07.5), M08 (results not found are hidden), D05 (disputed facts show both values) |
| mixed-up model versions | the score of one release written for another; Llama 2 facts on the Llama 3 page | result cards name the exact model version and benchmark version (M01, M02); the citation check matches table rows and columns (13.6) |
| wrong benchmark setting | a 5-shot score compared with a 0-shot one | settings are hard keys (10.5), M03 (no guessed setting), M09 (one comparable group per column) |
| stale "state of the art" | text that was true on the day of writing and false a month later | K04 and S07 ban the words; superlatives come from computed facts with dates (06.7); `as_of`, freshness windows and `review_after` (07.9); the daily job (14.9) |
| overclaiming | a result at one scale written as general; a correlation written as a cause; a developer's claim written as fact | claim scopes (07.8), K05, K06, `self_reported` (07.10), the audit's "no claim states more than its source shows" |
| a small experiment presented as the paper's result | "we reproduced dropout" for an MLP on MNIST | the computed scoped claim (07.4), K06, R03 and R04 |
| fake tests | the code tested against itself, or tests weakened to pass | X02 (agreement with the NumPy reference), X04 (finite differences), X05 (official vectors), X06 (mutation testing), the fixer prompt forbids weakening tests (14.6) |
| wrong derivation | a step that does not follow, a dropped term | numbered justified steps and numeric checks (06.5, S11); Lean where it applies (11); the audit's derivations check |
| one-sided contested topics | the writer's model takes the side its training data favours | N01 to N06, the symmetric form of 06.9, the neutrality check of the audit (15.4), sampling of contested pages (15.6), reviewers who can state each position (Q-19) |
| hype and anthropomorphism | "revolutionary", "the model understands" | S03, S14, the banned word list (06.2) |
| model self-favour | a writer from one lab's model family writing about that lab's models | the auditor and verifier are of another family (15.4); for model and organization pages of a lab, neither the writer nor the auditor is of that lab's family (Default, 15.4) |
| private information about people | a home town, a family member, a health detail of a researcher | P01 to P08, the person field allow-list (19.6), sampling of person pages |
| copied text | an abstract, a model card or a Wikipedia paragraph pasted with light edits | the copy check (13.7, C01), spans stored as offsets only (07.5), C06 for card text |
| hazardous content | a working jailbreak, test items of a dangerous capability evaluation, weapons uplift | C13, C14, `hazard: review` pages need a person review before they publish (19.8) |
| harvest errors at scale | a broken adapter writes thousands of wrong cards | adapters have tests on recorded responses (13.2); large changes need a person (13.4); D rules; sampling of new cards (15.6) |
| silent drift of the checks | a check that passes everything after a change of a tool or a model | the checks have their own tests; the audit of the audit (15.4); the canary pages (15.9) |

## 15.3 The rule catalogue

All rules have an id of a family letter and two digits.
`aik check` runs them; `schema/rules.v1.yaml` holds each rule's id, family, text, severity, waivable flag and the document that defines it.
An id is never reused for another meaning; a removed rule keeps its id, marked retired.

### Severities

As in algorithm's 13:

| Severity | Effect |
|---|---|
| **block** | the change cannot merge (the gate and CI fail); a block rule on published content takes the part off the site at the next deploy |
| **level (Wn)** | the page cannot reach level Wn (or the named level of another ladder) while the rule fails; lower levels stay |
| **warn** | shown in the report of the change and on the coverage dashboards; never blocks |
| **sample** | checked on samples (15.6), not on each change; a confirmed defect becomes a finding (15.8) |

### The families

| Family | Name | Defined in | Ids |
|---|---|---|---|
| T | structure | 15.3 (below) | T01 to T16 |
| E | edges | 15.3 (below), with the edge list of 04.7 | E01 to E14 |
| S | style and sections | 06.13 | S01 to S16 |
| G | grounding and citations | 07.11 | G01 to G10 |
| K | claims and time | 07.11 | K01 to K14 |
| X | implementations | 08.15 | X01 to X20 |
| R | experiments | 09.13 | R01 to R14 |
| M | metrics and results | 10.13 | M01 to M14 |
| F | formal | 11 | F01 and on; F03 is fixed: no `sorry`, `admit`, `native_decide` or new axiom |
| V | visualization | 12.14 | V01 to V16 |
| D | data facts | 10.13 | D01 to D10 |
| C | copying, licences and content limits | 19.13 | C01 to C15 |
| P | people and privacy | 19.13 | P01 to P08 |
| N | neutrality | 07.11 | N01 to N06 |
| L | links | 15.3 (below) | L01 to L07 |

### T: structure

| Rule | Checks | Severity |
|---|---|---|
| T01 | front matter, cards and registers are valid against their schemas in `schema/` | block |
| T02 | every item has a minted tag; no `NEW-` placeholder id on `main` | block |
| T03 | a slug is unique in its namespace (global, or under its catalogue route, 16.2) | block |
| T04 | no global slug is a reserved word of `registers/reserved.yaml`, a language id (except the software page of that language or framework, as `/pytorch/`, since a language's page is at `/languages/<id>/`) or the id of a top area (a sub-area is at `/<top>/<sub>/` and does not clash), and no top-area slug equals a policy slug | block |
| T05 | no hand-written status, level, badge, rank or evidence label in front matter, cards or prose | block |
| T06 | fields that do not apply to the kind are absent, not empty (04.5) | block |
| T07 | the `level` of a node is sampled against its canon and its prerequisites; an item less than 2 years old starts at `research` | sample |
| T08 | a node's directory is under its primary area | block |
| T09 | a sub-area with more than 200 nodes is split; a sub-area with fewer than 5 nodes after M3 is merged | warn |
| T10 | the second-page test across the border with a neighbour (5.9): the AI page has at least 3 sourced facts that the neighbour's page lacks | level (W2) |
| T11 | each card shard is at most 5 MB and sorted by tag | block |
| T12 | a tag is a card or a node, never both | block |
| T13 | a name shared by several items has a disambiguation entry | warn |
| T14 | a renamed slug is in `aliases.reg`, so old URLs redirect | block |
| T15 | a card-only kind (result, incident, work, exercise, provision, event) has no node directory unless it is listed in `registers/promoted.yaml` | block |
| T16 | every changed file and card row has a provenance record (04.8) | block |

### E: edges

| Rule | Checks | Severity |
|---|---|---|
| E01 | every slug in an edge field exists, as a node, a card or a bridge card | block |
| E02 | the target kinds of each edge match the edge list of 04.7 | block |
| E03 | the `prereqs` graph has no cycle | block |
| E04 | no cycle in `variant_of`, `generalizes`, `fine_tuned_from` or `succeeded_by` | block |
| E05 | `solves` targets are tasks | block |
| E06 | `evaluated_on` is derived from result cards only, and targets benchmark versions | block |
| E07 | `architecture` targets are architectures; lineage edges (`fine_tuned_from`, `distilled_from`, `merged_from`) join models | block |
| E08 | at most 8 `see_also` entries | warn |
| E09 | each core node has a path of `prereqs` back to `ai-foundations`, `math-for-ai` or `deep-learning-foundations` | level (W3) |
| E10 | a `related_to` edge has a note of one sentence | level (W2) |
| E11 | no reverse edge is written by hand; the build derives them | block |
| E12 | neighbour targets (`al:`, `m2:`, `pg:`) resolve in the neighbour's published index | level (W2) |
| E13 | `evidence_for` and `evidence_against` edges carry a claim id and a role (07.3) | block |
| E14 | `mitigates` and `regulates` edges carry a source with a locator | level (W2) |

### L: links

| Rule | Checks | Severity |
|---|---|---|
| L01 | every internal link (`[[...]]`, routes, anchors) resolves in the build | level (W2) |
| L02 | every neighbour link resolves through a bridge card (5.9) | level (W2) |
| L03 | external links are checked weekly; a link that fails twice uses its archived capture, and the page shows "archived" | warn |
| L04 | no link to a domain on the deny list (`registers/sources.yaml`, G08) | block |
| L05 | a work is linked by its canonical DOI or arXiv URL, not by a mirror | warn |
| L06 | every `#c-`, `#eq-` and section anchor that a link names exists | level (W2) |
| L07 | links to the Hugging Face Hub and to code repositories pin a revision | level (W3) |

### Severities of the other families

The texts of these rules are in the documents named above. Their severities:

| Severity | Rules |
|---|---|
| block | S01, S10 (new cards), S16; G01, G04, G08; K01 (anchor), K03, K06, K08; X03, X07, X08, X18; R02, R05, R06, R10; M01, M02, M03, M06, M07, M08; F03; V01, V13; D01, D02, D05, D08; C01 to C15 except C04 and C07 (level W1); P01, P02, P04, P05; N01 (in the audit) |
| level | S03, S04, S05, K04, G02 (W1); S07, S14, K05, K07, K09, G03, G09, N02, N04, E10, E12, E14, L01, L02, L06, T10 (W2); S08, S09, S11, S12, K02, K10, K11, K12, G05, G06, G10, N03, N06, E09, L07 (W3); N01, N05 (W4); X01, X02, X04, X05, X09, X11, X12, X15, X19, X20 (the X level of 08.10); X06, X10, X13, X16, X17 (X3); R01, R03, R04, R07, R08, R09, R11, R12, R13, R14 (the R level of 09.11); M04, M05, M09 to M14 (D2); D03, D04, D06, D07, D09, D10 (the D level of 10.11); V02 to V12, V14 to V16 (the V level of 12.15); C04, C07 (W1); P03, P06, P07, P08 (W1) |
| warn | S02, S06, S13, S15; K13 (level W4 after 30 days); G07; T09, T13; E08; L03, L05; X14 |
| sample | T07, K14; and every rule of the audit (15.4) is also sampled (15.6) |

The F rules are listed in 11 with their levels; F03 blocks everywhere.

### Waivers

A rule can be waived for one item when the rule is wrong for it, as in algorithm.

- A waiver is a row of `registers/waivers.yaml`: the rule, the item tag, the files and hashes, a reason, the person who grants it (an editor of the area, 15.10), and an end date at most one year away.
- An expired waiver stops counting; the rule applies again at the next build, and the daily job lists waivers that end within 30 days.
- **Never waivable:** T05, G04, G08, K04, K06, K08, R05, X07, X18, F03, C01, C05, C10, C11, C13, C14, P01, P04, P05 and N01. These rules protect readers, people and rights holders, or they keep the computed labels honest.
- A page with a waiver shows it in "History of this page", with the reason.
- The count of active waivers per rule is on the quality dashboard (15.9); a rule with more than 20 active waivers is reviewed by the editors at the next milestone.

## 15.4 The cross-family audit and the verifier

### The audit

The audit is algorithm-work's `audit.py`, copied (14.1, 14.6).
Its prompt is `work/prompts/audit/v1.md`; its order of checks and its JSON are those of 14.6.
This section adds what the audit must cover for AI pages and how its output becomes levels.

- **Families.** The writer of a node is a model of family A. The auditor, the verifier and the extraction pass are of a family B that is not the family of any writer of any file of the node (`auditor_for` in `work/writer.py`, 14.2). The families and their models are in `registers/models.yaml` (14.4).
- **Lab conflict.** For a model, system, organization or person page about a lab that makes a model family on the ladder, neither the writer nor the auditor is of that lab's family (Default). When the ladder has no other family available, the page waits.
- **Inputs.** Every file of the node directory, the gold pages of its kind (06.12), the canon refs of the item (13.3), the claims with their computed labels and citation results, and the facts of its cards. The auditor edits nothing.
- **Checks**, in the order of 14.6: facts and claims; derivations; code and experiments; completeness, including the **canon coverage check**; neutrality and dates; style.
- **The canon coverage check.** The auditor gets the list of subtopics that the canon works name for the item: the index terms and the section titles of its locators in `registers/canon.yaml`, and the sections of the gold page of the kind. Each subtopic that the page does not cover is a `major` issue with `rule: "canon"`, or the auditor says why the subtopic belongs on another page and names it. W4 needs no such issue (04.10).
- **The neutrality check.** For pages with a position block or a contested claim, the auditor answers four questions in the issues: is any position stated in the site voice; does each position have its strongest form with a proponent's source; do the positions have the same parts and the same tone; is any serious position of the literature missing (with sources). A yes to the first or a missing position is a `blocker`.
- **Output.** The JSON of 14.6, with two optional fields per issue: `rule` (the rule id the issue is about, when there is one, or `canon`) and `claim` (a claim id).
  ```json
  {"verdict": "fail", "score": 6,
   "issues": [{"severity": "major", "file": "node.md", "quote": "Dropout always improves generalization",
               "problem": "overclaim: the sources show lower test error for the networks and data of their experiments only",
               "fix": "Dropout lowered the test error in the experiments of Srivastava et al. (2014) on MNIST, CIFAR-10, SVHN and ImageNet.",
               "rule": "K05", "claim": "c-reduces-overfitting"}]}
  ```
- **Verdict.** `fail` on any blocker or major issue, or when the score is below 8, as in algorithm.
- **Rounds.** The counters of 14.6: two failed audits climb a rung of the model ladder, four end the item as `failed` for the person queue. A pass with only minor issues gets one `fix-minor` round and the gate again.
- **Binding to hashes.** An audit record (`state/audits/<slug>.json`, and its copy in the provenance record) names the hashes of every file it read. W4 counts only while those hashes are current. A change of a file re-runs the audit for that node at the next write.

### The verifier

The verifier runs after the audit passes, for nodes going to W3 or above (14.6).
It sees, for each claim, only the claim text, its type and scope, and the located span with its section from the snapshot (07.5).
It does not see the page, so it cannot be persuaded by it.

```json
{"claim": "dropout#c-reduces-overfitting", "work": "srivastava-2014-dropout-jmlr",
 "verdict": "supported" | "unsupported" | "contradicted" | "unreadable",
 "span": [48211, 48690], "note": "Table 2 reports lower test error with dropout for every listed architecture."}
```

- `supported`: the span says what the claim says, within its scope.
- `unsupported`: the span does not say it, or says less (a narrower scope, a weaker effect). The claim fails G3, and the fixer must narrow the claim or find a better locator.
- `contradicted`: the span says the opposite, or a different number. This is a blocker; the node goes back to `fix-audit` with the note as the issue (14.6).
- `unreadable`: the span is garbled by extraction (a broken table, a scanned page). The claim needs a person's attestation or another source.
- The verifier's verdicts feed G3 and the citation records; they never set an evidence label by themselves (07.4).
- **How it runs.** `work/verify.py <slug>` takes the claims whose record passed the citation check and has no verdict (`aik cite passages`), sends them with their passages to the verifier in one prompt (`work/prompts/verify/v1.md`), and writes each verdict into the record with `aik cite verdict`, with the verifier and the run id. The verifier is `verifier` of `registers/models.yaml`, or `auditor_for` the writers when that model is of a writer's family; with no writer known, the node waits. A record's verdict counts only while its key is current: a change of the claim, the locator or the snapshot drops it (13.6). A change of the extractor or the checker drops it too, unless the passage and section that the verifier read are the same: `aik cite` then carries the verdict, the verifier and the run to the new record, since the verifier would judge the same text (07.5 Cache). It reports how many verdicts it carried.
- **What is kept.** The citation record keeps the verdict, the verifier and the run. The verifier's quote (at most 25 words of the span) and its note go to the run record `prov/runs/<run>.yaml` (14.7), not to the repository's pages, so no text of a source is published beyond a short quote (C01).

### The audit of the audit

Model judges drift with model versions, and an auditor that passes everything looks the same as a good page.

- **Quarterly checks.** Each quarter, people check 100 random audit verdicts and 200 random verifier verdicts, stratified by kind and by model family. For each, a person records agree or disagree with a reason. The disagreement rate, with its Wilson interval, is on the quality dashboard (15.9).
- **Canary pages.** `registers/canaries.yaml` holds 30 node directories with planted defects: an invented source, a wrong parameter count, a position in the site voice, a gradient sign error, a test that checks the code against itself, a copied paragraph. Each new auditor or verifier model, and each new prompt version, must catch at least 90% of the planted defects before it is used (14.4). The canaries are never published and are excluded from training data that the project releases.
- **Agreement between families.** On 2% of nodes, a second auditor of a third family audits the same hashes. The rate at which the two disagree on the verdict is tracked; above 15% for a kind, the editors review the prompt for that kind.

## 15.5 Human review and provenance display

### Review records

As in algorithm: a review is a file `reviews/<tag>/<part>.yaml`, in a signed commit by the reviewer.

```yaml
tag: 5TQH2W
part: text                   # text | positions | facts | contract | impl-<framework> | exp/<slug> | scene | lean-spec | lean-statement | proof-<id>
hashes: {node.md: b3:91ac…, claims.yaml: b3:22f0…, derivations/expectation-scaling.md: b3:4b7d…}
checklist: text/method/v1    # 15.7
reviewer: "github:alice"
verdict: pass                # pass | changes
date: 2027-04-02
items:                       # one per question of the checklist, by its number
  - {check: "1", result: pass}
  - {check: "8", result: pass, note: "checked Hinton 2012 arXiv v1 date"}
attest:                      # optional: paywalled sources read (07.5) and independence (07.4)
  - {claim: c-variance-shift-with-bn, work: li-2019-disharmony-cvpr, read: true, supports: true}
independence: []
interests: "none"            # the reviewer's statement of interests for this item
```

- `hashes` holds the BLAKE3 hash of every file of the part, by its path from the node directory, or from the root of the repository with a leading "/" for a file outside it (`/lean/AI/Dropout.lean`). The files of a part: `text` and `positions`, node.md, claims.yaml and derivations/; `facts`, facts.yaml; `contract`, contract.yaml; `impl-<framework>`, impl/<framework>/; `exp/<slug>`, the experiment's directory; `scene`, viz/; `lean-spec` and `lean-statement`, the Lean module; `proof-<id>`, the proof or derivation with that id.
- `aik review <slug> <part> -reviewer github:<login> -w` writes the record to fill in: the current hashes, the newest checklist of the part and the kind, and one item per question. `aik review <slug> <part> -check` tells whether the record counts, and why not.

| Part | Raises | Reviewer skill in `registers/reviewers.yaml` |
|---|---|---|
| `text` | W5 | `domain:<top area>` |
| `impl-<framework>` | X4 for that framework | `code:<framework>` |
| `exp/<slug>` | R4 | `exp`, and `domain:<top area>` |
| `lean-spec` | F1 of an algorithm (the spec matches the page) | `lean` |
| `lean-statement` | F1 of a theorem (the statement matches the page, 11.7) | `lean` |
| `proof-<id>` | H3 (the page proof is checked) | `domain:<top area>` |
| `contract` | the contract of an interface (09) | `code:<any framework>`, or `domain:<top area>` |
| `scene` | V3 | `viz` or `domain:<top area>` |
| `facts` | D3 | `data`, or `law:<jurisdiction>` for policy and provision facts |
| `positions` | the neutrality part of W5 on contested pages | `domain:<top area>`, and the conditions below |

A review counts only when all of these hold:

- The reviewer is not an author of any reviewed hash (by provenance, 04.8).
- The reviewer has the skill for the part in `registers/reviewers.yaml` (14.11), with a signing key there. The commit of the record is signed with that key; CI checks the signature, as it has the history, and the build checks the rest.
- The verdict is `pass`, every question of the checklist file has an item that is `pass` or `n/a`, the hashes are the current hashes of the files, and `interests` is stated ("none" when there are none).
- **Conflicts of interest.** A reviewer employed by, or paid by, the developer of a model, system or dataset, or the body that made a policy, cannot give W5 to its page or review its contested sections. They may review `facts` (D3) with their interest stated in `interests`, because they often know the facts best, and the page shows "facts reviewed by a reviewer from <organization>" (Default, Q-29).
- **Contested pages.** A reviewer who has published for one position must say so in `interests`. A `positions` review then needs a second reviewer who holds another position or none (Q-19).
- **Staleness.** A changed hash makes the record stale. The record stays in the history, and the page loses the level until a new review. A reviewer can renew a stale review with a diff review of the changed lines only, recorded with `basis: <old record>`.

### The provenance display

Each node page ends with "History of this page" (16.4), from the provenance and review records:

- who wrote each part (a GitHub login, or an agent run with its model ids and prompt versions, 14.7);
- what checked it: the gate, the citation check ("14 of 14 claims found in their sources"), the verifier, the audit with its score and its auditor family, the tests per framework, the experiments;
- who reviewed which part, for which hash, on which date, with the interests they stated;
- the active waivers, with reasons;
- the corrections of the page (15.8).

The disclosure line of 19.9 ("Written by AI, checked by the build", "Reviewed by <name>", "Written by <name>") is computed from the same records, and the `.md` and `.json` twins carry it (C15).

## 15.6 Sampling

Checks of each change catch what can be computed. Sampling catches the rest, as in algorithm.

- **Weekly**, seeded by the date (`aik sample -week 2027-W14`), so that anyone can recompute the sample.
- **Stratified by risk.** The sample first takes pages from these strata, at these rates of the pages changed that week, then 2% of all other published pages at random:

| Stratum | Rate | Why |
|---|---|---|
| gold pages (06.12) | every changed one | the quality bar of the writer and the auditor |
| core nodes (5.10) | 10% | the pages most readers read |
| contested pages and pages with a position block | 15% | the risk of one-sidedness |
| policy and provision pages | 15% | the risk of wrong legal facts with real effects |
| person pages and pages that name living people | 15% | the risk to people (19.6) |
| model pages of releases less than 6 months old | 10% | facts change fast and are mostly self-reported |
| new cards from a harvest run | 1%, at least 50 per source | the risk of an adapter error at scale |
| pages written by a model new on the ladder | 10% for its first 1,000 pages | the risk of a new model's failure modes |

- **The draw.** `aik sample` reads the changed pages from git, between the last commit before the Monday of the week and the last commit before the next Monday (UTC). A card changed when its line changed and is new when no removed line has its tag, so a card that moved shard is not new. In each stratum the pages are sorted by sha256(week, stratum, id) and the first ones are taken: the rate of the stratum, rounded up, and at least its minimum. The new cards are drawn per harvest source. A page that two strata take is listed once, under the first. The stratum of a new model on the ladder and the doubling of rates are not computed yet: the writer of a page and the defect records are not recorded.
- **What a sampler does.** A person, or for the first pass a model of a family other than the writer's and the auditor's, runs the checklist of the kind (15.7) and the sampled rules (T07, K14). A model's finding is confirmed by a person before it counts.
- **Defect classes**, as in algorithm: `critical` (a false claim, an invented source or attribution, a private fact, copied text, a position in the site voice, hazardous content), `major` (an overclaim, a missing required part, a misleading explanation, a wrong date), `minor` (style, a weak example, a missing link).
- **Rates.** Each stratum's defect rate per class is published with a Wilson 95% interval. A stratum whose upper bound for critical defects is above 1%, or for major defects above 5%, doubles its sampling rate until two weeks are under the targets, and the editors look for the common cause (a prompt, a model, an adapter).

## 15.7 Checklists

Checklists are versioned files `checklists/<part>/<kind>/v<n>.md`, used by reviewers, samplers and the reviewer-assistant (14.2).
A review record names its checklist version.
Each item is a yes or no question with the place to look.

The text checklist for a method page (`checklists/text/method/v1.md`), as an example:

1. The lead says what the method is and what it is for, in our words.
2. "In plain words" is right and needs no notation.
3. The equations of "The method" are those of the original source, up to the notation map, and the procedure is complete.
4. Every derivation step follows; the numeric check tests the stated result.
5. The worked example is right by hand.
6. Every claim has the right type and scope, and no prose sentence states more than its claim.
7. The sources of each claim say what the claim says (check three claims at random against the source).
8. The History names the right people, works and years; "first" claims are justified.
9. Limitations, variants and pitfalls include those of the canon and of the main review articles.
10. Hyperparameter ranges and framework defaults are right for the pinned versions.
11. Nothing is in the site voice that a source should say; nothing ranks or dates itself.
12. No text reads as copied from a source you know.

Other checklists: `text/<kind>` for every kind with a page; `positions/v1` (the four neutrality questions of 15.4, and for each position, "would a proponent accept this statement of it?"); `facts/<kind>`; `impl/<framework>`; `experiment/v1`; `lean-spec/v1`; `scene/v1`; and `sample/<kind>` for samplers, a shorter list.

## 15.8 Findings and corrections

A defect that a reader report (19.12), a sample, a reviewer, the audit of the audit or a daily check finds is a **finding**: an issue with the label `finding` and a severity.

| Severity | Examples | Response |
|---|---|---|
| **S1** | a false claim in a main section, an invented source, a private fact about a person, copied text, hazardous content, a position in the site voice on a contested page | the affected part is hidden within 24 hours (it leaves the site at the next deploy, or at once by a person with `aik quarantine`, which writes a `quarantine` line in the provenance log of the node (04.8) that the next build obeys); a fix is queued at the top weight; the page shows a "correction" badge |
| **S2** | an overclaim, a wrong number in a fact or a result, a wrong date, a missing serious position, a retracted source still used | fixed within 7 days; the page shows the badge until then |
| **S3** | a weak explanation, a missing variant, a style defect | the work queue at normal weight |

- **The corrections log.** Every S1 and S2 finding, once fixed, gets a line in `docs/corrections.md`: the date, the page, what was wrong, what changed, and how it was found. The site shows it at `/about/corrections/`, and each page lists its own corrections in "History of this page".
- **Root cause.** Each S1 finding gets a short note of its cause (a prompt, a model, an adapter, a check that should have caught it) and, where it can, a new rule, a new canary (15.4) or a new test, so that the same defect is caught next time.
- **Fan-out.** When a finding's cause is shared (one adapter, one prompt version, one model on the ladder), the daily job lists every item made by the same cause, and they are sampled at 20% until the rate is under target.
- **Credit.** A reader who reported a finding is credited in the log if they wish (19.12).

## 15.9 Metrics and targets

`aik report quality` computes these each week and publishes them on the quality dashboard at `/about/quality/`, next to the dashboards of 16.15.

| Metric | Target at M3 | Target at M5 |
|---|---|---|
| critical defect rate in the sample, upper Wilson bound | under 1% | under 0.5% |
| major defect rate in the sample, upper Wilson bound | under 5% | under 3% |
| share of published claims that pass the citation check (G2) | 95% | 99% |
| share of claims on W3 pages with the verifier's `supported` (G3) | 90% | 97% |
| share of claims with a headline label above `reported` on core nodes | 30% | 45% |
| audit first-pass rate (pass with no fix round) | tracked | tracked |
| person disagreement with audit verdicts (15.4) | under 10% | under 5% |
| canary catch rate of each auditor and verifier model | 90% | 95% |
| S1 findings hidden within 24 hours | 100% | 100% |
| S2 findings fixed within 7 days | 90% | 95% |
| reviewed nodes (W5) among core nodes | 20% | 60% |
| active waivers per 1,000 nodes | under 10 | under 5 |
| claims past `review_after` or their freshness window | under 2% | under 1% |
| external links that fail and have no archive | under 1% | under 0.5% |

The targets are also exit criteria of the milestones (21).
A metric that is off target for two weeks opens an issue for the editors.

## 15.10 Who decides

| Decision | Who | How it is recorded |
|---|---|---|
| a rule's text, family and severity | the maintainers, after a public issue with at least 7 days for comments | `schema/rules.v1.yaml`, a version bump |
| a waiver for one item | an editor of the item's top area | `registers/waivers.yaml` |
| the positions of a contested question, and a position added by an editor | the editors of the area, with reviewers who can state each position | `positions.yaml` of the node, `registers/positions.yaml` |
| the gold pages and their changes | the editors, with a review record | `registers/gold.yaml` |
| the canon and the canaries | the editors | `registers/canon.yaml`, `registers/canaries.yaml` |
| the model ladder and the auditor families | the maintainers, after the canary test | `registers/models.yaml` (14.4) |
| a finding's severity | the person who triages it; S1 can be set by any reviewer | the issue label |
| removal of content for legal or safety reasons | the maintainers (19.12) | the quarantine record and the corrections log |
| a reviewer's skills and keys | the editors of the area | `registers/reviewers.yaml` |

Disagreements between an editor and a reviewer go to the maintainers, whose decision is recorded in the issue.
No decision of this table can be made by an agent; agents propose, people decide.
