# 13. Sources and harvest

Most facts about AI artifacts already exist in structured form: Epoch AI's model database, the Hugging Face Hub API, OpenAlex, arXiv, DBLP, Wikidata, the Papers with Code dump, the AI Incident Database, the official journals of law.
The harvester turns them into inventory rows, universe items and cards without a model, or with a small model for extraction only.
This document defines the sources, the adapters, the inventory, the canon, the schedules, the licence classes, the raw snapshots and the citation check that ties every citation to a stored copy, the copy check, and the identifiers.
It follows 2150/11 (algorithm) and 2150p/13 (programming).

## 13.1 Principles

1. **Harvest before you write.** A card made from a structured source is cheaper and more correct than a page that a model writes from memory. The catalog step (14.3) starts from the harvest, not from a model's list.
2. **Every fact has a snapshot.** The harvester stores the raw bytes of every response in R2 `ai-raw`, keyed by BLAKE3 (13.6). A card's `harvest` record (04.6) names the snapshot. The citation check reads the snapshot, not the live web.
3. **Primary sources win.** For each fact the harvester records whether the source is primary (the developer's paper, model card, documentation, the official text of a law) or secondary (Epoch AI, the AI Index, Wikipedia, news). The fact model of 04.9 merges them.
4. **Licence decides what we keep.** Each source has a licence class (13.5, 19.2). A class D source gives names, ids, numbers and dates only, never text.
5. **Be a good citizen.** Adapters use official APIs and dumps first, respect `robots.txt`, rate limits and machine-readable text and data mining reservations (19.4), identify themselves with a generic project User-Agent (`aik/<version> (+https://github.com/tamnd/ai-agent)`) that names the project page, never a personal address, and cache. A source that blocks automated access is class E and is not scraped around.
6. **Self-reported is a label, not a defect.** Most AI numbers are reported by the people who made the artifact. The harvester keeps them and marks them `self-reported`; the evidence labels of 07 and the comparability rules of 10.6 decide how they are shown.

## 13.2 Sources and adapters

Each adapter lives in `ai-agent/harvest/<source>/` (20) and writes `registers/inventory/<source>.jsonl`, proposals for `registers/universe/` (18.3), and card updates under `cards/`.
Many sources already have a `*-cli` tool in the author's workspace; the adapter wraps it, and the adapter's output schema is the contract, so a tool can be replaced without changing the cards.

### Scholarly record

| Source | Gives | Access (adapter) | Class | Schedule |
|---|---|---|---|---|
| arXiv | new and updated papers in the categories of 5.8, versions (`v1`, `v2`), abstracts, licences, PDFs and source for cited papers | OAI-PMH at `oaipmh.arxiv.org` (the `arXiv` metadata format, one set per category; `export.arxiv.org` disallows robots); bulk metadata dump | metadata A; full text per paper licence, default D (19.3) | daily |
| OpenAlex | works, authors, institutions, venues, topics, citation counts, subfield 1702 and neighbours | API and snapshot (`openalex-cli`) | A | weekly; snapshot monthly |
| Semantic Scholar | works, citation contexts, influential citations, TLDR flags (not text), author ids | API and datasets (`semanticscholar-cli`) | facts; datasets ODC-BY (checked at M1) | weekly |
| DBLP | venues, proceedings, authors, keys; the cleanest venue record | dumps and API (`dblp-cli`) | A | weekly |
| Crossref, DataCite, Unpaywall | DOIs, publication metadata, open-access locations | APIs (`crossref-cli`, `datacite-cli`, `unpaywall-cli`) | facts | on demand |
| OpenReview | ICLR, NeurIPS, TMLR and other submissions, decisions, reviews, versions | API (`openreview-cli`) | metadata facts; text D | per conference cycle |
| ACL Anthology | NLP papers, venues, awards | data repository (`aclanthology-cli`) | papers since 2016 CC BY 4.0 (C); metadata C | per conference |
| CVF open access, NeurIPS proceedings, PMLR, JMLR, AAAI, IJCAI | papers of the main venues, awards | sites (`cvf-cli`, `neurips-cli`, others) | metadata facts; text D unless the paper states a licence | per conference |
| ACM DL, IEEE Xplore, Springer, SSRN, NBER | metadata of papers outside arXiv, economics and policy papers | APIs where allowed (`acmdl-cli`, `ieee-cli`, `springer-cli`, `ssrn-cli`, `nber-cli`) | facts; text D | on demand |
| OpenAIRE, ORCID, ROR | funders, author ids, institution ids | APIs (`openaire-cli`, `orcid-cli`, `ror-cli`) | A (ORCID public data CC0, ROR CC0) | weekly |
| Connected Papers, paper review and digest sites | related-work signals for the curate step | sites (`connectedpapers-cli`, `paperreview-cli`, `morningpaper-cli`) | signals only, never cited for facts | on demand |

### Models, datasets, software and hardware

| Source | Gives | Access (adapter) | Class | Schedule |
|---|---|---|---|---|
| Epoch AI data | models with developers, dates, parameters, training compute, data size, hardware, cost estimates, notability; hardware; benchmark results it runs | CSV downloads | C (CC BY 4.0) | daily (the database changes most days) |
| Hugging Face Hub | model and dataset repositories, downloads, likes, tags, licences, base models, `model-index` results, commit hashes, model cards; Spaces; Trending Papers | API (`huggingface-cli`, `hf-cli`) | metadata facts; card text per repository licence, default D | daily for the head and for new repositories of tracked organizations; weekly for the rest |
| Papers with Code data dump | tasks, methods, datasets, papers with code links, evaluation tables (historical results to 2025-07) | GitHub repository `paperswithcode/paperswithcode-data` (`pwc-cli`) | B (CC BY-SA 4.0): ids, names and numbers only into cards (19.2) | once at M0; never updated |
| Kaggle | datasets, competitions, leaderboards of finished competitions, notebooks as exercise links | API (`kaggle-cli`) | facts; text D | weekly |
| UCI ML Repository, OpenML, Zenodo | classic and small datasets with licences and DOIs | APIs (`uci-cli`, `zenodo-cli`) | per dataset | monthly |
| lab and developer pages | model cards, release posts, technical reports, system cards, licences, usage policies | fetched on citation, snapshotted (13.6) | primary sources; text D unless licensed | on citation and on release (14.9) |
| GitHub | repositories of AI software and of papers' code, releases, licences, stars | API (`github-cli`) | facts; code per repository licence, never copied (C rules) | weekly |
| PyPI, conda, Docker Hub | releases of AI software | APIs (`pypi-cli`, `anaconda-cli`, `dockerhub-cli`) | facts | daily for tracked packages |
| Weights & Biases public reports | public training runs cited by works | API (`wandb-cli`) | facts | on citation |
| vendor hardware pages and datasheets | specs of GPUs, TPUs and accelerators | fetched on citation | primary; facts | on release |

### Knowledge bases and teaching

| Source | Gives | Access (adapter) | Class | Schedule |
|---|---|---|---|---|
| Wikidata | QIDs and statements for concepts, models, software, people, organizations, events, awards | dumps; for a limited run, SPARQL on a public mirror whose `robots.txt` allows it (QLever), since `query.wikidata.org` disallows robots | A | weekly |
| Wikipedia (English) | outlines, glossaries, timelines, category trees, article titles, infobox facts, redirects (aliases) | API and dumps (`wikipedia-cli`) | B: titles, ids and facts only; never text into cards or code | weekly |
| Wikimedia Commons | photos of people and historical systems with free licences | API (`commons-cli`) | per file: CC0, CC BY, CC BY-SA, public domain only | on demand |
| Britannica, Stanford Encyclopedia of Philosophy, PhilPapers | names, categories and citations for history and philosophy | sites (`britannica-cli`, `philpapers-cli`) | D: names, ids and citations only | monthly |
| Distill, Transformer Circuits, The Gradient, interpretability and lab blogs | explanations, figures, findings | sites (`distill-cli`, `thegradient-cli`, `interpretableml-cli`) | Distill C (CC BY 4.0); others per post, default D | weekly |
| personal explainers (Lilian Weng, Jay Alammar, Andrej Karpathy, Two Minute Papers) | coverage signals and citations | sites (`lilianweng-cli`, `jayalammar-cli`, `karpathy-cli`, `twominutepapers-cli`) | D unless stated; signals and citations only | weekly |
| textbooks with free online versions (D2L, the Deep Learning book, Prince UDL, SLP3, Interpretable ML, Neural Networks and Deep Learning) | tables of contents and index terms for the canon (5.10), section locators | sites (`deeplearningbook-cli`, `neuralnetworksdl-cli`, `interpretableml-cli`, others) and by hand | D for text except where licensed (D2L text CC BY-SA 4.0 is B); tables of contents are facts | per edition |
| course sites (CS229, CS231n, fast.ai, MIT OCW, Coursera, edX, Class Central) | syllabi orders for the courses of 5.7; exercises | sites (`cs229-cli`, `cs231n-cli`, `fastai-cli`, `ocwmit-cli`, `coursera-cli`, `edx-cli`, `classcentral-cli`) | names and orders only (19.3); OCW CC BY-NC-SA is D for us | per term |
| EleutherAI, AI2 and other open research groups | evaluation harnesses, datasets, model releases | sites and GitHub (`eleutherai-cli`, `allenbrain-cli`) | per artifact | weekly |
| scikit-learn and ML library documentation, `mlxtend` | API facts for software cards, default hyperparameters | sites (`mlxtend-cli`, `pydocs-cli`, `rtd-cli`) | facts; text per licence (scikit-learn BSD-3 is C) | per release |

### Safety, incidents and policy

| Source | Gives | Access (adapter) | Class | Schedule |
|---|---|---|---|---|
| AI Incident Database | incidents, dates, reports (links to articles), classifications | snapshot exports (the backups listed on its research snapshots page); for a limited run, the page-data JSON of each incident | facts; text per its terms (checked at M1, Q-23) | daily |
| OECD.AI | policy initiatives by country, the AI incidents monitor, principles | site and data downloads | per dataset (checked at M1) | weekly |
| MIT AI Risk Repository | risks with causal and domain taxonomies, source documents | spreadsheet download | C (CC BY 4.0, checked at M1) | monthly |
| MITRE ATLAS | tactics, techniques, mitigations, case studies of attacks on AI | data repository | C (checked at M1) | monthly |
| OWASP Top 10 for LLM Applications and the GenAI Security Project | risk classes and mitigations | site (`owasp-cli`) | B (CC BY-SA 4.0): names and ids only | per release |
| NVD and OSV | CVEs in AI software | APIs (`nvd-cli`, `osv-cli`) | A | daily for tracked software |
| Alignment Forum, LessWrong | alignment research posts, concepts and debates | GraphQL API (`alignmentforum-cli`, `lesswrong-cli`) | per post licence (checked at M1); default D | weekly |
| EUR-Lex | the EU AI Act, the Digital Omnibus on AI, implementing and delegated acts, guidelines, harmonized standards references; CELEX numbers and ELI ids | API and SPARQL | C (EU reuse policy, Decision 2011/833/EU, with attribution) | daily (the official journal) |
| US Federal Register, Congress.gov, GovInfo | executive orders, agency rules, bills, hearings | APIs (`federalregister-cli`, `congress-cli`, `govinfo-cli`) | A (US federal works) | daily |
| NIST | the AI RMF, NIST AI 600-1, AI 100 series, terms | site | A | weekly |
| UK legislation and Parliament | acts, bills, the AI Security Institute's publications | APIs (`ukparliament-cli`) | C (Open Government Licence v3.0) | weekly |
| other national registers (China's CAC and NPC texts, Korea, Japan, Canada, Brazil, India and others) | national AI laws and rules | official sites; translations from licensed sources only | per jurisdiction (19.3) | weekly |
| Council of Europe, UN, OECD, G7, standards bodies (ISO/IEC JTC 1/SC 42, IEEE, CEN-CENELEC JTC 21, ETSI, C2PA) | treaties, resolutions, standard titles, numbers, status and dates | sites | treaties and resolutions C or A; standards text D (names and numbers only) | monthly |
| court records | AI copyright and liability cases | `courtlistener-cli` and official court sites | A for US federal opinions; others per court | weekly |

### Community and signals

| Source | Gives | Access (adapter) | Class | Schedule |
|---|---|---|---|---|
| Stack Overflow, Reddit, Hacker News | the questions people ask (search terms, the "missing" signal of 18.6) | APIs (`stackoverflow-cli`, `reddit-cli`, `hackernews-cli`) | signals only; never cited for facts | weekly |
| Our World in Data, the AI Index data | yearly series (investment, compute, publications) | downloads (`owid-cli`) | OWID C (CC BY 4.0); AI Index data per its licence (CC BY-ND 4.0 for the report) | yearly and on update |

### Neighbours

| Source | Gives | Access | Class | Schedule |
|---|---|---|---|---|
| algorithm.tamnd.com, programming.tamnd.com, math2 | bridge cards: tag, slug, name, summary, URL, kind (5.9) | the `.json` twins of each site | A (CC0 metadata, our own) | at each release of the neighbour |

## 13.3 The inventory, the canon and the surveys

**The inventory.** Each adapter writes inventory rows: one per named item that the source lists, with the source id, the source's own id, the name, the kind as guessed by the adapter, the URL, the snapshot hash and the date.
`aik universe merge` (18.3) clusters rows that name the same item (by shared ids first: DOI, arXiv id, QID, Hub repo id, Epoch name; then by normalized name and kind) and proposes universe items with an outcome.
The inventory is the input of the catalog and curate steps (14.3): a sub-area's catalog run sees every inventory row placed in it, so the model fills gaps and does not invent the list.

**The canon.** `registers/canon.yaml` lists the canon works of 5.10, each with its ISBN or DOI, edition, the parts we index, and its table of contents and index terms (names and locators only, 19.3).
It is used as a checklist (the canon match step and the core flag, 14.3, 5.10), and as secondary sources with locators.
It is never copied.
Each area also has a **secondary canon** of works that the core rule does not count but the curate step reads for coverage, for example *Neural Network Methods for Natural Language Processing* (Goldberg), *Mathematics for Machine Learning* (Deisenroth, Faisal and Ong), *Algorithms for Decision Making* (Kochenderfer, Wheeler and Wray), *Reinforcement Learning: Theory and Algorithms* (Agarwal et al.), *Bandit Algorithms* (Lattimore and Szepesvári), *Elements of Causal Inference* (Peters, Janzing and Schölkopf), *Fairness and Machine Learning* (Barocas, Hardt and Narayanan), *Interpretable Machine Learning* (Molnar), *Artificial Intelligence: Foundations of Computational Agents* (Poole and Mackworth), *Machines Who Think* (McCorduck) and *The Quest for Artificial Intelligence* (Nilsson).

**The surveys.** `registers/surveys.yaml` lists survey works by sub-area (for example a survey of parameter-efficient fine-tuning, of diffusion models, of LLM agents), at most 5 per sub-area, each with a date.
Surveys feed the frontier core (5.10) and the catalog step for items younger than the canon.
A survey older than three years in a sub-area with more than 50 new works a year is flagged for replacement.

Rule: a canon or survey entry needs an editor's reason. The canon is a checklist, not a ranking.

## 13.4 Harvest runs, schedules and politeness

- **Runs.** `aik harvest <source>` runs one adapter. A run is a function of the source's state on that date: it writes snapshots, inventory rows and card updates, and a run record (`reports/harvest/<source>/<date>.json`) with counts of new, changed, removed, unchanged and skipped rows, the requests and the snapshots.
  A run with `-limit` or `-since` is partial: it adds and updates rows but removes none, so a sample never deletes the inventory.
- **Pull requests.** One pull request per source per run, as in programming. A run whose changes pass the D and M rules (15.3) and touch only cards may merge without a person (Default); a run that removes cards or changes more than 5% of a source's cards waits for a person.
- **Schedules.** As in the tables of 13.2. The daily job (14.9) runs the daily sources in one batch at 02:00 UTC.
- **Limits.** Each adapter has a rate limit in `registers/sources.yaml`, at or below the source's published limit, and a monthly request budget. Bulk dumps are preferred whenever they exist (OpenAlex, DBLP, Wikidata, arXiv metadata, Papers with Code).
- **Failures.** A source that fails three runs in a row raises an issue. A source whose schema changes fails the adapter's schema check and stops, so a changed source never writes bad cards.

## 13.5 Licence classes

The classes are defined in 19.2, and each source's class is in 19.3; this is the summary the harvester uses.
Every adapter writes the class into each snapshot and into each card's `harvest` record.

| Class | Licence of the source | What the harvester keeps | Examples |
|---|---|---|---|
| A | public domain, CC0 | anything; data may enter the CC0 zone | Wikidata, OpenAlex, DBLP, arXiv metadata, ORCID, ROR, US federal works, NIST |
| B | share-alike (CC BY-SA) | ids, names and numbers into cards; prose only into the prose zone with attribution, never into code or card text | Wikipedia, the Papers with Code dump, OWASP, D2L text |
| C | permissive or attribution (CC BY, OGL, the EU reuse policy, MIT, Apache-2.0, BSD) | facts, structure, short quotes and adapted prose with attribution; card fields under the attribution of 19.2 | Epoch AI, the ACL Anthology, Distill, the MIT AI Risk Repository, EUR-Lex, UK legislation |
| D | all rights reserved, non-commercial, no-derivatives, or unclear | names, ids, numbers, dates, section and table locators, and quotes under G04 only | most papers, textbooks, ISO and IEEE standards text, the AI Index report text, lab blogs, model card text, SEP, course materials |
| E | forbidden to harvest | nothing by machine; a person may cite it | any source that blocks bots or asks us to stop |

## 13.6 Raw snapshots and the citation check

**Snapshots.**

- Every adapter run writes its raw responses to `ai-raw/<source>/<date>/<blake3>`, with the URL, the time, the HTTP headers, the licence class and the adapter version.
- The repository keeps only the manifests (`harvest/manifests/<source>.jsonl`: URL, time, hash), never the raw bytes.
- When a node cites a work, the citation check fetches the cited version once and keeps it: the arXiv version named by the citation (`2307.09288v2`, never the moving latest), the PDF from the venue or an open-access location found by Unpaywall, the model card at a Hub commit hash, the web page as captured, the legal text by its CELEX number and consolidated version date.
- **Extracted text.** Each PDF snapshot gets a text extraction with layout: pages, section headings, numbered equations, tables (cells with row and column labels), figure captions and footnotes, stored next to the PDF as `<blake3>.doc.json` with the extractor's name and version. The LaTeX source from arXiv is used when it is available, because it gives exact equations and table cells. Equations are kept as LaTeX for the derivation check of 06.5.
- **Locators.** A citation's `locator` names a place that the extraction can find: `Section 3.2`, `Eq. 4`, `Table 3, row "Llama 3.1 405B", column "MMLU (5-shot)"`, `p. 12`, `Article 50(4)`, `Appendix B.1`. The grammar of locators is in 13.6.1.
- Snapshots of class D sources are kept for checking only, and they are never published. Their size is the main cost of `ai-raw` (03.3). Whether to store the PDFs of all work cards or only of cited works is Q-21.

**The citation check.** This is the check behind G05 (15.3) and the verifier (15.4):

1. Resolve the source to a snapshot at the cited version.
2. Check that the locator exists in the snapshot's extraction: the section, the equation number, the table and its row and column, the page, the article and paragraph.
3. For a number claim (a score, a parameter count, a date), check that the number appears at the located place, with the stated precision; for a table cell, check the row and column labels as well.
4. For an equation claim, check that the cited equation and the page's equation are the same up to the renaming of symbols in the notation register (06.4), with a symbolic comparison where the extraction allows it, and a model judgement otherwise, recorded as `model-checked`.
5. For a text claim, check that the claim's key terms appear in the located section, and ask the verifier (a model of another family than the writer, 14.2) whether the located passage supports the claim. The verifier sees only the claim and the passage.
6. For a class D source that we cannot snapshot (a paywalled standard), check the clause number against the published table of contents and mark the citation `toc-checked`.

A citation that fails any step fails G05 and goes to the fixer (14.2).
The outcome of each step is stored with the claim (07.5), so that a page can show "checked against the source on <date>".

**Where the check keeps its work.**

- **What to fetch.** The work card says it, in the field `cited`: `{version, url, format}`, where `version` is the version that the page cites (`arXiv 1706.03762v7`, `JMLR 15(56)`), `url` is where that exact version is, and `format` is `pdf`, `html` or `none` (a work that the check cannot fetch, as in step 6). The card's `authors` are not key terms of a claim, so that "Srivastava et al. recommend" does not need the source to hold the names.
- **Snapshots.** `aik cite` fetches each cited version once, with the user agent `aik/<version> (+https://github.com/tamnd/ai-agent)` and at most one request each three seconds to arXiv, and keeps it in the raw store under its BLAKE3 hash (`ai-raw`; locally `$AIK_RAW`). `manifests/works.jsonl` in the content repository has one line per work: the work, the version, the URL, the hash, the size, the fetch time and the extractor.
- **Records.** `prov/cite/<tag>.jsonl` has one line per source of each claim of the node: the claim, the work, the locator, the cache key of 7.5, the snapshot hash, the spans as offsets, the outcome of each step, and the verifier's verdict when there is one. The records are committed, so the build computes G2 and G3 without the raw store. A record whose key does not match the claim, the locator and the snapshot as they are now is stale and counts for nothing.
- **Key terms.** A source passes step 5 when its span or the section of its span holds at least 75% of the key terms of the claim (Default). The key terms are the content words of the claim outside its math, after stemming, without the names of the authors of the work. The check finds a passage about the claim; the verifier decides whether the passage supports it.
- **Writers** read the places that the extraction found with `aik cite show <work>`, and the text of a place with `aik cite show <work> "<locator>"`.

### 13.6.1 The locator grammar

A locator is a list of places separated by commas, semicolons or "and".
Each place is a kind and a number.

| Kind | Written | Number |
|---|---|---|
| section, chapter | `Section 3.2`, `Sections 2 to 7`, `§ 4`, `Chapter 5` | `3`, `3.2`, `3.2.1` |
| appendix | `Appendix A.4` | `A`, `A.4` |
| equation | `Eq. 4`, `Eqs. 5 to 7`, `Equation 3.1` | `4`, `3.1`, `4a`, `A.2` |
| table, figure, algorithm | `Table 3`, `Fig. 2`, `Figure 11`, `Algorithm 1` | `3`, `A2` |
| theorem, lemma, definition, proposition, corollary | `Theorem 2.1` | `2`, `2.1` |
| footnote | `Footnote 3` | `3` |
| page | `p. 12`, `pp. 12 to 14` | the number printed on the page |
| article, recital, annex | `Article 50(4)`, `Recital 27`, `Annex III` | `50`, `50(4)`, `50(4)(a)`, `III` |
| abstract, title page | `the abstract`, `the title page` | none |

- A plural kind takes a range with "to". A bare number continues the kind before it: `Sections 3 and 4`, `Tables 2, 3`.
- A table takes the qualifiers `row "<label>"` and `column "<label>"`, which must appear on the located pages.
- A quoted heading after a place names it, and the place must hold it: `Section 5.4 "Residual Dropout"`. It guards against a renumbered version.
- The venue, the arXiv id and the version are not part of a locator; they belong to the work card. `NeurIPS 26, the sections on GLMs` and `arXiv 1409.2329, Section 3` are not locators.

How a place resolves in the extraction:

- **Sections, chapters and appendices** resolve to the text from their heading to the next heading at their depth or above, or to the list of references. A range runs from the start of its first place to the end of its last. Headings are found in order (3.2 after 3.1, 4 after 3.2.1), so that a number at the start of a line of a table or of a list of references is not taken for one.
- **Tables, figures and algorithms** resolve to the pages that hold their caption, because the extraction does not keep the cells of a table next to its caption.
- **Equations, footnotes and labelled statements** resolve to the text around their mark within its page, so that an equation is read with the sentences that introduce it. A footnote that the markup of a web page marks as a note (the notes of the Official Journal) resolves to the note itself.
- **Pages** resolve by the number printed on the page, so that an article that starts at page 1929 is cited by its printed numbers, and otherwise by their place in the file.
- **Articles** resolve to the text up to the next article; a paragraph, to the text from its number to the next one.
- **The provisions of a legal text** end at the next heading of their level or above. A recital ends at the next recital, or at the enacting formula ("HAVE ADOPTED THIS REGULATION:") after the last one. An article ends at the next article, at the next chapter or section, or at the final formula ("This Regulation shall be binding…") after the last one. An annex ends at the next annex, as its parts and points are its own. The heading of an article is "Article N" on a line of its own, with its title on the next line, and the headings of an act are the run of numbers that starts at Article 1, so that a reference ("Article 6(1) and the corresponding obligations…") or an article that an amending act quotes ("Article 75b", inside its Article 1) is not taken for one. An annex is found by "ANNEX III" or "Annex III" on a line of its own after the last article, and a recital by its number in brackets between "Whereas:" and the enacting formula. A span inside a provision is read with the provision as its section.
- **The title page** of a web page is its opening block, the text before its first heading or provision, cut at 4,000 bytes. Of a PDF it is its first page.

The text of a web page keeps its white space as a browser shows it, one space for a run of spaces and new lines and one new line at the end of a block. A superscript after a digit is written as a power, so that `10<sup>25</sup>` is `10^25`, and the number check reads `10^25`, `$10^{25}$` and `10²⁵` as 1e25. The text of a PDF loses its superscripts, so a power of a claim is also found there as its base and its exponent.

## 13.7 The copy check

The copy check is the `copycheck` package from math2-agent, extended for AI sources.

- **Corpora.** The check compares each change with the snapshots of every source that the change cites, with the text of the canon works that the project holds lawfully (for checking only), with the Wikipedia articles of the item, and with the code corpora: the official repositories of papers' code that the node links, the framework tutorials (PyTorch, JAX, Flax, Hugging Face docs), D2L's code, and popular from-scratch repositories (for example minimal GPT and nanoGPT-style implementations).
- **Prose.** Shingled word n-gram overlap (n = 8) against each corpus, and a sentence-alignment test that finds close paraphrase in the same order (19.5). The thresholds are in 19.13 (C01).
- **Code.** Token-level winnowing after renaming identifiers, against the code corpora. Standard idioms (a training loop, `optimizer.zero_grad()`, a `torch.nn.Module` skeleton) are on an allow-list and do not count.
- **Figures.** Perceptual hashes of every figure against the figures extracted from cited PDFs. A redrawn figure that copies the layout and data of a source figure needs a source with a class that allows it, and the caption must say "after <source>".
- **Equations** are facts and are not copy-checked. The prose that derives them is.
- The copy check runs in F2 on every change and in the audit (15.4). A failure blocks the merge.

## 13.8 Identifiers and resolution

Every outside id that an item has goes into `same_as` (04.5), and search resolves each of them (17).

| Id | Form | Example | Resolves to |
|---|---|---|---|
| arXiv | `YYMM.NNNNN` with optional version | `1706.03762`, `2307.09288v2` | work card |
| DOI | `10.x/…` | `10.1038/nature16961` | work card |
| OpenAlex | `W…`, `A…`, `I…`, `T…` | `W2626778328` | work, person, organization, topic crosswalk |
| Semantic Scholar | corpus id | | work card |
| DBLP | key | `conf/nips/VaswaniSPUJGKP17` | work card; person key to person card |
| Wikidata | QID | `Q25339462` | any item |
| Hugging Face | `org/name` and a commit hash | `meta-llama/Llama-3.1-405B` | model or dataset card |
| Epoch AI | model name as in the database | `Llama 3.1-405B` | model card |
| AIID | incident number | `1625` | incident card |
| ORCID, ROR | person and organization ids | | person and organization cards |
| CELEX, ELI | EU legal acts | `32024R1689` | policy and provision cards |
| SPDX | licence id | `Apache-2.0` | licence facts (19.7) |

Rules: an id belongs to exactly one item (rule E14); two items that claim the same id are merged or one of them is wrong, and the harvester stops with a conflict report.

## 13.9 When sources disagree

The fact model of 04.9 decides: primary over secondary, disagreements beyond the tolerance shown as `disputed` with both sources, estimates marked as estimates with their estimator.
The harvester never chooses a value silently.
For results, 10.6 adds the comparability rules: two reports of the "same" score with different settings are two result cards, not a disagreement.

## 13.10 Adding a source

A new source is a pull request that adds a row to `registers/sources.yaml` (id, name, URL, owner, licence with the date it was checked, class, access method, rate limit, schedule, primary or secondary for which facts), an adapter with a schema test and a recorded fixture, and an entry in the crosswalk list of 5.8 if the source has its own ids.
A person approves the class.
