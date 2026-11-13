# 07. Claims and evidence

This is the central idea of the site.
In algorithm, a page was trusted because its proofs were checked and its code was tested.
Most AI knowledge is not like that: it is empirical, it depends on scale and setting, it is reported by the people who built the system, and it ages.
So every factual sentence of a page is a **claim**: a typed object with a scope and sources, and a **computed evidence label** that says what kind of support it has.
The page reports; the build judges (06.1).

## 7.1 Why claims

- **A reader needs to know what kind of fact a sentence is.** "Dropout reduces overfitting" can be a measured effect in a 2014 paper on MNIST, a result that many labs replicated, a result that our own experiment shows at small scale, or a disputed claim at large scale. The prose looks the same in all four cases. The label shows the difference.
- **An agent writer overclaims.** The commonest defect of model-written AI text is a statement stronger than its source: a result at one scale stated as general, a developer's claim stated as fact, a correlation stated as a cause. Typed claims with scopes make overclaiming a checkable defect.
- **Evidence changes, the text should not have to.** A replication, a retraction or our own failed experiment changes the label at the next build, without an edit to the prose.
- **Claims are the unit of every check.** The citation check (7.5), the verifier (15.4), the experiments (09), the formal proofs (11) and the results register (10.6) all attach to claim ids.

## 7.2 Claim types

Each claim has one type.
The type decides which evidence kinds can support it and which labels it can have.

| Type | What it states | Example | Evidence it allows | Labels it can have |
|---|---|---|---|---|
| `definitional` | what a term means, or what a method, metric or law is | "BLEU is the geometric mean of modified n-gram precisions times a brevity penalty." | a source with a locator | none; it shows the source chip only |
| `mathematical` | an identity, a bound, a theorem, a count, a complexity | "With inverted dropout, the expected value of each unit at training time equals its value at test time." | a derivation (06.5), a Lean proof (11), a cited proof | `proved`, `reported` |
| `empirical` | an observed effect, a measured value, a comparison | "Adding dropout to a feedforward network on MNIST lowered the test error in Srivastava et al. (2014)." | works with results, experiments here (09), result cards (10.6) | `measured-here`, `replicated`, `reported`, `contested` |
| `historical` | who did what, when and where | "The term 'artificial intelligence' was used in the 1955 proposal for the Dartmouth workshop." | primary documents, histories, archives | `replicated` (two independent histories or a primary document), `reported`, `contested` |
| `attributive` | that a person, a lab or a body says, argues or reports something | "OpenAI reports that ..." ; "Searle (1980) argues that ..." | the source of the statement itself | `reported`; the claim is about the statement, never about its content |
| `normative-reported` | what a law, a standard or a policy requires, or what an ethical position holds | "Article 5 of the EU AI Act prohibits ..." | the official text (for law), the work of the proponents (for positions) | `reported`, `replicated` (official text plus official guidance) |
| `forecast` | a prediction or an expert estimate about the future | "In the survey of Grace et al. (2024), the median respondent gave ..." | the source of the forecast | `reported`; it is always attributive in form and dated (K07) |

Rules:

- A claim of type `attributive`, `normative-reported` or `forecast` never gets a label about the truth of what is said. "Searle argues X" can be `reported`; X itself is not labelled.
- An `empirical` claim needs a scope (7.8). A `mathematical` claim needs its assumptions.
- A claim that mixes two types is split into two claims.
- Definitional claims are not labelled because a label on a definition would only say "this is how the source defines it". The page shows the source, and the Usage section of a definition page shows how other sources differ (06.3).

## 7.3 `claims.yaml`

Each node directory has `claims.yaml` next to `node.md` (04.4).
The writer writes the claim records, and the extraction pass (7.6) checks them.
The build writes everything that is computed.

```yaml
# claims.yaml of dropout
- id: c-reduces-overfitting
  type: empirical
  text: "Dropout lowered the test error of feedforward and convolutional networks on MNIST, CIFAR-10, SVHN and ImageNet in the experiments of Srivastava et al. (2014)."
  scope:
    models: "feedforward and convolutional networks of 2 to 8 layers"
    data: [mnist, cifar-10, svhn, imagenet-ilsvrc-2012]
    scale: "up to about 60 million parameters"
    setting: "supervised classification, SGD with momentum and max-norm"
  evidence:
    - {work: srivastava-2014-dropout-jmlr, role: original, locator: "Section 6, Table 2", stance: for}
    - {work: hinton-2012-improving-arxiv, role: original, locator: "Section 2", stance: for}
    - {work: krizhevsky-2012-imagenet-nips, role: replication, locator: "Section 4.2", stance: for, reuses: none}
  exp: [dropout-mnist-mlp]            # experiments of 09 that test it
  result: []                          # result card ids, when the claim is a benchmark comparison
- id: c-inverted-scaling
  type: mathematical
  text: "With inverted dropout, the expected value of each unit at training time equals its value at test time."
  assumes: ["units are dropped independently with probability p", "0 <= p < 1"]
  derivation: expectation-scaling     # derivations/expectation-scaling.md
  lean: null                          # a Lean name when one exists (11)
  evidence:
    - {work: srivastava-2014-dropout-jmlr, role: original, locator: "Section 10", stance: for}
- id: c-variance-shift-with-bn
  type: empirical
  text: "Placing dropout before batch normalization can shift the variance of activations between training and test, which Li et al. (2019) report lowers accuracy."
  scope: {models: "ResNet and DenseNet variants", data: [cifar-10, cifar-100], scale: "up to about 30 million parameters"}
  evidence:
    - {work: li-2019-disharmony-cvpr, role: original, locator: "Section 3 and Table 1", stance: for}
```

Fields written by people or agents:

| Field | Meaning |
|---|---|
| `id` | `c-` and a slug unique in the node, never reused after a claim is removed |
| `type` | one type of 7.2 |
| `text` | the claim in one sentence, as the page states it; the prose sentence may differ in wording but not in meaning (K01) |
| `scope` | for empirical claims: `models`, `data` (dataset or benchmark slugs), `scale` (parameters, data, compute), `setting`; what is not stated by the source is `unknown` |
| `assumes` | for mathematical claims |
| `evidence` | a list of sources, each with `work` (a work card, or a source id of `registers/sources.yaml`), `role` (`original`, `replication`, `reanalysis`, `review`, `meta-analysis`, `primary-document`, `official-text`, `official-guidance`, `statement`), `locator` (13.6), `stance` (`for`, `against`, `mixed`), and for replications `reuses` (`code`, `data`, `weights`, `none`) |
| `exp`, `derivation`, `lean`, `result` | links to the other kinds of evidence of the node |
| `as_of` | for claims that are true at a date (a licence term, the status of a law); required by K04 |
| `self_reported` | true when the source is the developer of the system that the claim is about (7.10) |
| `superseded_by`, `review_after` | 7.9 |

Anchors in the prose:

- A claim sentence in `node.md` ends with `{#c-<id>}` (06.2). One sentence can hold at most two claims (Default, Q-27). A claim can be anchored at most once per page.
- A page may show the claim of another node with `::claim{ref=<slug>#c-<id>}`. The claim is defined once, in the node that owns it; findings own their claims, and methods refer to them.
- A **claim sentence** is any sentence of the site voice that states a fact about the world and is not a definition of a term written in the same sentence; rule K02 finds unanchored ones (7.11).

Computed fields, in `build/claims.json` only, never in the source:

```json
{"id": "dropout#c-reduces-overfitting", "type": "empirical",
 "labels": ["replicated", "measured-here"], "headline": "replicated",
 "independent_for": 2, "against": 0,
 "cite": [{"work": "srivastava-2014-dropout-jmlr", "snap": "b3:7f2c…", "span": [48211, 48690], "check": "found", "verifier": "supported", "checked": "2026-11-12"}],
 "measured_here": {"exp": "dropout-mnist-mlp", "scope": "MLP 784-800-800-10 on MNIST, 5 seeds", "r": "R2"},
 "stale": false}
```

**Size (Estimate).** About 25 claims per node at W3 and about 32,000 nodes at M5 give about 800,000 claims, plus one claim per result card (10.6). `build/claims.json` is sharded by top area.

## 7.4 Evidence kinds and labels

**The labels.** Each label is computed from the evidence of the claim and the check results.

| Label | Computed when | What the reader sees |
|---|---|---|
| `proved` | a Lean proof at F2 or above (11) states the claim, or a `full` derivation (06.5) derives it and passed the cross-family audit with no blocker or major issue, or a cited proof in a peer-reviewed source has been checked by a reviewer (15.5) | "proved", with a link to the proof |
| `measured-here` | an experiment of the node tests the claim and is at R2 or above, with its test passed (09), or our harness produced the result (10.4) | "measured here", always followed by the scope of the experiment (K06) |
| `replicated` | at least two independent sources support the claim within its scope, both grounded at G2 (7.5); for result cards, as in 10.6 | "replicated", with the count of independent sources |
| `reported` | at least one source supports it at G2, and the other labels do not apply | "reported", with the source |
| `contested` | standing evidence against the claim within its scope: an independent source with `stance: against` at G2, a failed replication, an experiment of ours at R2 that fails its test, or a disputed fact (04.9) that the claim depends on | "contested", with the sources on each side |

A claim with no grounded source has no label and fails K02 or G05.

**Labels form a set.** A claim can be `replicated` and `measured-here` at once, or `reported` and `contested`.
The chip shows one **headline** label, by this order (Default, Q-26):

1. `proved`, which nothing overrides, because a proof of a mathematical claim cannot be contested by evidence; a claimed counterexample is a bug report on the proof and fails the proof's check instead.
2. `contested`, which overrides every other label of an empirical, historical or normative-reported claim.
3. `measured-here`.
4. `replicated`.
5. `reported`.

The popover shows the full set (16.4). The order is printed on `/about/evidence/`.

**`measured-here` never upgrades the paper's claim.** An experiment at small scale gives `measured-here` to the claim at the experiment's scope.
If the experiment's scope is smaller than the claim's (a 2-layer network for a claim about deep networks), the build does not give the label to the claim; it creates a computed **scoped claim** next to it ("measured here for an MLP on MNIST") that carries the label, and the page shows both (K06, 7.8).

**Independence.** Two supporting sources are independent when all of these hold:

- No author is shared, by OpenAlex, ORCID or DBLP author id, or by name match when no id exists.
- No primary institution is shared (OpenAlex institution ids), except for universities with more than 500 authors in the area, where the lab or group is compared instead.
- The replication does not reuse the original's weights; when it reuses its code or data (`reuses`), it counts as independent for the effect but the popover says what it reused.
- Neither is a review, a textbook or a meta-analysis of the other. Reviews and meta-analyses are shown, but they count as one source each, never as the count of their inputs.
- For result cards: as in 10.6 (another lab's paper, our harness, or an independent leaderboard whose maintainer ran the model).

A reviewer can assert or deny independence in a review record (15.5), for example when two labs share data under a contract that the ids do not show.

**Source standing.** `registers/sources.yaml` gives each source, and each work card gives each work, a `standing`:

| Standing | Examples | Counts for G2 | Counts as an independent source for `replicated` |
|---|---|---|---|
| `peer-reviewed` | journal articles, conference papers in the proceedings (NeurIPS, ICML, ICLR, ACL, CVPR, AAAI, JMLR, TMLR) | yes | yes |
| `preprint` | arXiv, OpenReview submissions not accepted | yes | yes |
| `official` | legal texts, official guidance, standards bodies' documents, official statistics | yes | yes, for normative-reported claims |
| `developer` | technical reports, model cards and system cards by the developer, official blogs of a lab | yes | no, for claims about the developer's own systems (7.10) |
| `secondary-reference` | textbooks, encyclopedias, Wikipedia, surveys | yes, but never the only source for G2 of an empirical or historical claim that has a primary source (G06, G10) | no |
| `journalism` | news reports | yes, for incidents and dated events only | yes, for incidents (two outlets with independent reporting) |
| `community` | blog posts, forums, social media posts | only for attributive claims about the post itself | no |

A work that the build finds in Retraction Watch or with a Crossref retraction notice gets `standing: retracted` (7.9).

## 7.5 The citation check

The citation check proves that a source says what the page says it says.
The mechanism is in 13.6; this section says how its results become levels and labels.

- **Snapshots.** Each source of each claim resolves to a snapshot in `ai-raw` with a BLAKE3 hash (`snap:`). arXiv citations pin the version (G09). A source cited without a version gets the version that was current when the citation was written, and the build records it.
- **Locators.** Each evidence entry has a locator in the grammar of 13.6.1. The check resolves the locator to a text span of the extraction (`span: [start, end]` in characters). A locator that does not resolve fails G03.
- **Numbers.** Numbers in the claim are normalized (thousands separators, percent and fraction forms, scientific notation, "B" and "billion", "k" and "thousand") and must be found in the span at the stated precision. Rounding: the page may round the source's number but never state more digits than the source has (K11).
- **Key terms.** The content words of the claim, after the notation map of 06.4, must appear in the span or its section: at least 75% of them, without the names of the authors of the work (Default, 13.6). This is the step that finds the passage; the verifier judges it.
- **The verifier** (15.4) reads only the claim and the span with its section, and gives `supported`, `unsupported`, `contradicted` or `unreadable`. G3 needs `supported` for every claim of the page.
- **Records.** The outcome of each step is in `prov/cite/<tag>.jsonl`, and the snapshots are in `manifests/works.jsonl` (13.6).
- **Cache.** The result of each check is cached by the hash of the claim text, the snapshot hash, the locator and the versions of the text extractor (13.6) and the checker. A claim whose cache key is unchanged is not checked again.
- **No copied text.** The build stores spans as offsets into the snapshot, never as copied text, so `build/claims.json` holds no source text (C01). The verifier's short quote is kept in the run record only (14.7).
- **Sources we cannot snapshot.** A source behind a paywall, or a class D source that we may not store, is `toc-checked` at best (13.6). Such a citation counts for G2 only with a reviewer's attestation in a review record that the reviewer read the passage and that it supports the claim (Default, Q-28). Without it, the citation is shown but the claim needs another source to reach G2.

**Grounding level of a page.** G1 needs every claim to have a source with a locator in the right form (G01 to G04). G2 needs every claim to pass the citation check on at least one source (G05). G3 needs the verifier's `supported` on every claim, and no claim `contradicted` (04.10).

## 7.6 Extraction

The writer writes `claims.yaml` with the page, because the writer knows what it meant to claim.
Then an **extraction pass**, run by the verifier role (14.2) and so by a model of another family than the writer, reads `node.md` and finds every claim sentence:

1. It splits the page into sentences and marks each one as `claim`, `definition-in-place`, `structure` (a sentence that only says what the page contains), or `quote`.
2. It matches each claim sentence to a claim of `claims.yaml` by meaning; a match with a different meaning or scope is reported as a K01 issue, and a claim sentence with no match is a K02 issue.
3. It proposes a type and a scope for each claim and compares them with the writer's; a difference goes to the auditor as an issue.
4. It writes nothing to `claims.yaml`. The fixer (14.2) changes the files.

`aik claims extract <slug>` runs the cheap part (sentence split, anchor match, the forbidden phrases of K04 and K05) in the gate (14.6); the model part runs once, before the audit.

## 7.7 Contested claims and contested questions

There are two kinds of disagreement, and the site treats them differently.

- **A contested claim** is an empirical, historical or normative-reported claim with evidence on both sides (7.4). It is labelled `contested`, and the popover shows the sources on each side with their standing and independence. The prose states the claim with its scope and attribution ("Wei et al. (2022) report ... ; Schaeffer et al. (2023) argue that the effect disappears under continuous metrics"), never which side is right (N01).
- **A contested question** is an open question on which serious positions disagree (6.9): an open-problem or argument node, or a section with a position block. Its positions are written in the symmetric form of 06.9 and laid out as in 16.8.

**Serious positions.** A position has a place on a page when one of these holds (Default, Q-30):

- at least three sources by distinct authors, with standing `peer-reviewed`, `preprint` or `official`, or books from academic publishers, hold it; or
- a named official body holds it (a national academy, a standards body, a court, a legislature, a regulator); or
- an editor of the area adds it with a recorded reason in `registers/positions.yaml`, for example a position that is new but held by researchers of the area.

A position that meets none of these is not on the page, and a position that meets one is never left out.
Positions on a page and the sources that hold them are in `positions.yaml` of the node:

```yaml
question: "Are emergent abilities of large language models a property of the models or of the metrics used to measure them?"
positions:
  - id: p-model-property
    claim: "Some abilities appear abruptly with scale, and this is a property of the models."
    held_by: [wei-2022-emergent-tmlr, ganguli-2022-predictability-facct, srivastava-2022-bigbench]
    relies_on: [c-bigbench-jumps, c-multistep-arithmetic]
  - id: p-metric-artefact
    claim: "The abrupt jumps come from discontinuous metrics, and continuous metrics show smooth improvement."
    held_by: [schaeffer-2023-mirage-neurips, lu-2023-emergent-icl, du-2024-loss-perspective]
    relies_on: [c-continuous-metrics-smooth]
order: by-independent-sources     # 06.9, item 4
```

## 7.8 Scope and transfer

An empirical result holds where it was measured.
The scope of a claim says where that is, and the page never states the claim more widely than its scope.

- **Scope fields.** `models` (family, size range, architecture), `data` (dataset and benchmark slugs, languages, modalities), `scale` (parameters, training tokens, compute), `setting` (training regime, fine-tuning or pretraining, the benchmark setting of 10.5).
- **Scope in prose.** The prose sentence of an empirical claim names the scope, or the scope is clear from the same paragraph (the audit judges this; an overclaim is a major issue, 14.6).
- **Transfer is a claim.** "This holds for large language models too" is a new claim with its own sources, never an inference of the site.
- **Scoped claims of our experiments.** When an experiment of 09 tests a claim at a smaller scale, the computed scoped claim (7.4) reads: "Measured here: <effect> for <model> on <data>, <n> seeds, <test> (R<level>)." It is written by the build from `experiment.yaml`, so it cannot overclaim.
- **Failures to transfer.** A source that tests a claim outside its scope and finds that it fails is `stance: against` for a scoped claim that the build creates for the new scope, not for the original claim. The page shows both scopes, so a reader sees where the effect holds and where it does not.

## 7.9 Aging, supersession and retraction

- **`as_of`.** A claim that is true at a date (the terms of a licence, the status of a law, a count of downloads) carries `as_of`. The renderer prints it. A claim with `as_of` older than its kind's freshness window (`schema/freshness.yaml`, for example 90 days for licence terms of models and 30 days for the status of a bill) is flagged by the daily job (14.9) for a refresh (K13).
- **`review_after`.** Any claim can carry a date after which it must be reviewed, for example a claim about a benchmark that is near saturation. After the date, the claim is listed in the work queue (14.5), and its chip shows "review due" until a writer or reviewer renews it (K13).
- **Supersession.** When a later work corrects a claim (an erratum, a corrected number in a v2, a later paper by the same authors), the old claim gets `superseded_by: <claim id>` and the new claim takes its place on the page. The old claim stays in `claims.yaml` and in History sections, never in the main text (K09).
- **Retraction.** The daily job checks every cited work against Crossref's retraction and correction notices and the Retraction Watch database. A retracted source stops counting for any label; the claim is re-labelled at the next build; a claim whose only source is retracted is hidden from the main text, shown in History with a note, and sent to the work queue at the top weight (K08, 15.8).
- **arXiv revisions.** A new arXiv version of a cited work does not change the citation (it is pinned, G09). The daily job lists citations whose work has a newer version, and the extraction pass compares the cited span with the new version; a change at the span opens a work item.

## 7.10 Frontier systems and developer reports

Most facts about the largest models come from their developers, and many are not documented at all.

- **Self-reports are attributed.** A claim whose only sources are by the developer of the system it is about has `self_reported: true`, can be at most `reported`, and its prose names the developer as the source ("Meta reports ...", "according to the GPT-4 technical report") (K05, G06).
- **Self-reports do not replicate each other.** A model card, a technical report and a blog post by the same developer are one source for independence (7.4).
- **Undocumented is a fact.** When the developer does not disclose a fact (parameter count, training data, compute), the fact record has no value, a note "not disclosed by the developer" and the date of the check (04.9), and estimates by others (Epoch AI, for example) are `estimated` facts with their estimator and method (D06). The page never writes an estimate as a value without its estimator.
- **Capability claims.** Claims that a system can do a task ("reaches expert level", "can reason") are `attributive` when they come from the developer, and become `empirical` claims only with result cards from benchmarks with versions and settings (10.6).
- **Safety evaluations.** Results of a developer's own safety evaluations are `self_reported` and attributive. Third-party evaluations (government AI safety institutes, independent evaluators) are independent sources and name their access conditions (pre-release access, API, weights).
- **Leaks and rumours.** Facts from leaks, anonymous sources or social media are not used for any fact or claim, except as `attributive` claims in journalism-sourced incident pages, and never about private individuals (19.6).

## 7.11 The rules K, G and N

These rules are listed in 15.3 with their severities.

**K: claims and time.**

| Rule | Checks | Severity |
|---|---|---|
| K01 | every `{#c-...}` anchor names a claim of `claims.yaml` (or a claim of another node by reference), and the sentence matches the claim's text in meaning and scope (the extraction pass, 7.6) | block (anchor); level W3 (meaning) |
| K02 | every claim sentence of the site voice is anchored; definitions in place and structural sentences are exempt | level (W3) |
| K03 | the evidence of each claim is of a kind that its type allows (7.2); `attributive`, `normative-reported` and `forecast` claims are not labelled on their content | block |
| K04 | no "state of the art", "SOTA", "currently", superlative or rank about models, labs, scores or datasets in the site voice; no time-bound fact without `as_of` (06.7) | level (W1) |
| K05 | the prose does not state the strength of evidence ("well established", "proven", "it is known that") and uses "shows" or "proves" only for `proved` claims; self-reports are attributed | level (W2) |
| K06 | a `measured-here` label is shown with the scope of the experiment, and never on a claim whose scope is larger than the experiment's | block |
| K07 | a `forecast` claim is attributed in the prose and carries the date of the forecast | level (W2) |
| K08 | no label counts a retracted source; a claim whose only source is retracted is not in the main text | block |
| K09 | a superseded claim is not in the main text | level (W2) |
| K10 | an empirical claim that compares scores on a benchmark points to result cards (`result:`), not only to a work | level (W3) |
| K11 | numbers in claims carry units and no more precision than the source | level (W3) |
| K12 | a claim that something was "first" is `historical`, has two independent sources or a primary document plus a history, and names the sense of "first" | level (W3) |
| K13 | no claim is past its `review_after` date or its freshness window without a renewal | warn; level W4 after 30 days |
| K14 | claims of two pages that state contradicting values or effects in overlapping scopes are reported (sampled weekly by the extraction pass over claims that share a subject) | sample |

**G: grounding and citations.**

| Rule | Checks | Severity |
|---|---|---|
| G01 | every source is in the right form: a work card slug, or a source id of `registers/sources.yaml` | block |
| G02 | every claim and every fact has at least one source | level (W1) |
| G03 | every evidence entry has a locator in the grammar of 13.6.1, and the locator resolves in the snapshot | level (W2) |
| G04 | the cited work exists: its DOI, arXiv id, CELEX number, Hub commit or URL resolves, and its title and authors match the card (an invented source is a blocker in the audit, 14.6) | block |
| G05 | the citation check passes for at least one source of each claim (7.5) | level (W3) |
| G06 | a `secondary-reference` or `developer` source is not the only source of a claim that a primary or independent source exists for | level (W3) |
| G07 | a work published in a venue is cited by its version of record, with the preprint as a second identifier | warn |
| G08 | no source on the deny list of `registers/sources.yaml` (content farms, paper mills, sites flagged for fabricated content) | block |
| G09 | arXiv citations pin a version; Hub citations pin a commit; web citations have a capture | level (W2) |
| G10 | Wikipedia is never the only source for G2 | level (W3) |

**N: neutrality.**

| Rule | Checks | Severity |
|---|---|---|
| N01 | no position on a contested question or a contested claim in the site's own voice (the audit, 15.4) | block in the audit; level (W4) |
| N02 | every view on the page has a named holder and a source; no "some say", "many believe", "critics argue" without names | level (W2) |
| N03 | each position of a contested question is stated with at least one source by its proponents, and the page has at least two positions | level (W3) |
| N04 | no loaded label of `registers/loaded-words.yaml` outside a quote or a self-described name | level (W2) |
| N05 | positions have the same parts in the same order, and no position has an adjective of praise or blame that another lacks | level (W4) |
| N06 | an expert survey is reported with its sample, response rate, question wording and date | level (W3) |

## 7.12 What the reader sees

- **Chips.** Each anchored sentence ends with the chip of its headline label (16.4). Definitional claims show a source chip only.
- **The popover** shows the full label set, the scope, the sources with their standing, roles, locators and the date of the citation check, the independence of the sources, the experiment and its scope, the proof, and "review due" when it applies.
- **The Evidence section** of methods, architectures, findings and risks (06.3) lists the claims of the node in a table: the claim, its type, its headline label, the count of independent supporting and opposing sources, and the scope. It can be sorted and filtered.
- **The evidence share.** The evidence bar of a page shows the share of its claims with a headline label above `reported` (04.10), for example "Claims: 41% replicated or better".
- **Filters.** A reader can hide every sentence below a chosen label ("show only replicated or better"), which greys the other sentences without changing the layout.
- **The twins.** The `.md` and `.json` twins of a page (16.10) carry the claims with their ids, types, labels and sources, so that agents that read the site can quote a claim with its evidence.
- **Claim links.** `/<slug>/#c-<id>` links to a claim and opens its popover; the full record of the claim is in the `.json` twin of the page (16.10). There is no separate claim route.
