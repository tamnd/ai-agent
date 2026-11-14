# 19. Legal and licensing

This document says under which licence each part of the corpus is published, what the project may take from each source, how text and data mining reservations are respected, how quotes and paraphrase are limited, how people are protected, how model and dataset licences are recorded, what the project never hosts or publishes, how agent-written content is labelled, and how takedowns work.
It follows 2150/17 (algorithm) and 2150p/19 (programming), and changes them where AI forces a change: more of the facts are about living people, more of the sources are papers under all-rights-reserved terms, the corpus describes dual-use techniques, and the subject is itself regulated (the EU AI Act's transparency rules apply to AI-generated text).

This is a design, not legal advice. The points marked "Default" are decisions that a lawyer's check may change (22). Every licence fact in 19.3 says whether it was checked, and when.

**Principles.**

1. **Write fresh.** As in algorithm and programming, prose, code and figures are written fresh. Sources are read, closed, and then the writer writes.
2. **Facts are free.** Names, dates, parameter counts, benchmark scores, ids, article numbers, equations and the structure of a field are facts. The project takes facts from any source with a citation, unless the source's terms forbid automated access (class E).
3. **Code stays MIT.** Readers copy implementations and experiments into their own work. No implementation, test or experiment may come from a share-alike, non-commercial or all-rights-reserved source (C05).
4. **Data stays reusable.** Cards and registers are published as datasets. Their text fields are written fresh or come from class A sources (C06). Fields whose source asks for attribution carry it, per field (19.2).
5. **Never host what we only describe.** Weights, large datasets, harmful content and other people's papers stay where they are published. The site links and pins hashes (19.8).
6. **Show the evidence.** Every page lists its sources with their classes, and `/sources/` lists every source with its licence and what the project took.

## 19.1 Licence zones

Each file of `tamnd/ai` is in one zone, set by its path in `REUSE.toml`, as in algorithm 17.1.

| Zone | Paths | Licence | Why |
|---|---|---|---|
| prose | `nodes/**/node.md` (body), `nodes/**/derivations/**`, `topics/**/*.md` | CC BY-SA 4.0 | the licence of Wikipedia and D2L text, so class B text can be adapted with attribution; as algorithm and math2 |
| figures | `nodes/**/figures/**`, `nodes/**/exp/**/figures/**`, frames and images made from scenes | CC BY-SA 4.0 | they explain the prose and travel with it |
| code | `nodes/**/impl/**`, `nodes/**/exp/**` (except figures and `results.json`), `nodes/**/viz/**`, `nodes/**/contract.yaml`, `nodes/**/shapes.yaml`, `nodes/**/vectors/**`, `nodes/**/cases/**` | MIT | readers copy code into closed programs |
| Lean | `lean/**` | Apache-2.0 | the licence of Mathlib, so code can move both ways (11) |
| data | front matter (the YAML block of each `node.md`), `cards/**`, `nodes/**/facts.yaml`, `nodes/**/releases.yaml`, `nodes/**/claims.yaml`, `nodes/**/exp/**/results.json`, `registers/**`, `topics/*.yaml`, `crosswalk/**`, `prov/**`, `schema/**`, `manifests/**`, `reports/**` | CC0 1.0, except card and fact fields marked with a source licence (19.2) | facts about AI should be free to reuse, as Wikidata's are |
| toolchain | all of `tamnd/ai-agent` | Apache-2.0; `@aik/viz` MIT; the Worker MIT | as algorithm-agent |

**Mixed files.** `node.md` is marked `CC-BY-SA-4.0 AND CC0-1.0`, and `LICENSES/README.md` says that the YAML block between the first two `---` lines is CC0 and the rest is CC BY-SA 4.0, as in algorithm.
Code blocks inside prose (a short illustration in a derivation) are MIT, as in programming's mixed files.

**Inbound equals outbound.** A contribution is under the licence of its zone. Contributors sign off each commit with the Developer Certificate of Origin. There is no CLA (Default).

**On the site.** Each page footer shows "Text CC BY-SA 4.0 · Code MIT · Lean Apache-2.0 · Data CC0 (with field credits)". Each implementation's raw file starts with one SPDX line.

## 19.2 Source classes and card fields

Each source in `registers/sources.yaml` has a class, as in algorithm and programming:

| Class | Licence of the source | May take |
|---|---|---|
| A | public domain, CC0 | anything, with a credit line as courtesy; data may enter the CC0 zone |
| B | share-alike (CC BY-SA, any version that allows CC BY-SA 4.0) | ids, names and numbers into cards; prose adapted into the prose zone with attribution; never into code (C05) or card text (C06) |
| C | permissive or attribution: CC BY, the UK Open Government Licence, the EU reuse policy (Decision 2011/833/EU), MIT, Apache-2.0, BSD | facts and structure; short quotes and adapted prose with attribution; card fields with the attribution kept per field |
| D | all rights reserved, non-commercial (NC), no-derivatives (ND), GFDL, terms for personal use, or unclear terms | facts, names, ids, numbers, dates, locators, and quotes under 19.5 only; never text, figures, tables reproduced as tables, exercises or code |
| E | forbidden to harvest: terms that forbid automated access, or the owner asked us to stop | nothing by machine; a person may cite it |

NC and ND sources are class D because the project's licences allow commercial use and derivatives.
GFDL sources are class D for the reasons of algorithm 17.2.

**Card fields.** Facts in themselves are not protected by copyright, but a database can be (the EU database right protects a substantial extraction from a database whose making needed a substantial investment).
So the cards follow these rules (Default, Q-22):

- A card field whose value came from a class A source, or from a primary source as a plain fact (a parameter count in a paper, a date in an announcement), is CC0.
- A card field whose value came only from a class C database (Epoch AI's compute estimates, the MIT AI Risk Repository's classification) keeps that source's licence and attribution in the field's `src` (04.6), and the dataset exports (16.10) carry a `licence` column per field and a NOTICE file that lists each such source and its credit line. The site shows "Data from Epoch AI (CC BY 4.0)" next to such values.
- No card takes a substantial part of a class D database: from such a source only facts that a node cites, one at a time, with a locator.
- The project does not take a whole class B database into cards: from the Papers with Code dump it keeps ids, names and numbers, rechecked against the primary papers before they are shown (10.6).

## 19.3 Per-source rules

Each row was checked on the date in `registers/sources.yaml`; "checked at M1" marks a summary from general knowledge that a person must verify before the source is used beyond facts.

| Source | Licence | Class | What we take |
|---|---|---|---|
| Wikidata | CC0 | A | anything |
| OpenAlex, DBLP, ORCID public data, ROR | CC0 | A | anything |
| arXiv metadata | CC0 | A | metadata; full text per paper (below) |
| arXiv papers | the arXiv non-exclusive licence by default; some CC BY, CC BY-SA, CC BY-NC-SA, CC0 | D by default; per paper otherwise | facts, equations, locators, quotes under 19.5; figures only from CC BY, CC BY-SA or CC0 papers, with credit |
| papers at venues (CVF, NeurIPS, PMLR, AAAI, IJCAI, ACM, IEEE, Springer) | authors' or publisher's copyright unless stated | D | facts, equations, locators, short quotes |
| ACL Anthology papers since 2016 | CC BY 4.0 | C | facts; figures with credit; prose adapted with attribution (we still write fresh) |
| Semantic Scholar datasets | per dataset, "such as CC BY-NC or ODC-BY", under the API License Agreement, which asks for the attribution "Semantic Scholar" (checked 2026-11-15); closed to the harvester (13.2), so not used | C or D | facts with attribution, if it opens |
| Wikipedia | CC BY-SA 4.0 | B | titles, ids, redirects, infobox facts; prose only into the prose zone with attribution, and by default not at all |
| Wikimedia Commons | per file | per file | photos only under CC0, public domain, CC BY or CC BY-SA, with credit (C04) |
| Papers with Code dump | CC BY-SA 4.0 | B | ids, names, numbers; never descriptions into cards |
| Epoch AI data | CC BY 4.0 | C | facts with the per-field credit of 19.2 |
| Hugging Face Hub metadata | facts | facts | ids, downloads, likes, tags, licence ids, base models, commit hashes |
| model cards and dataset cards | per repository; most have no stated licence for the card text | D by default | facts, locators, short quotes |
| AI Incident Database | CC BY-SA 4.0 unless noted, by the license.txt of its snapshots (checked 2026-11-14); the contents of the reports keep their own rights | B | incident ids, dates, titles of reports as citations, classifications as facts; links to the reports; never the text of news reports |
| MIT AI Risk Repository | CC BY 4.0 (checked 2026-11-15; each sheet says "This work is licensed under CC BY 4.0") | C | risk names, taxonomy ids, causal codes, source documents as citations |
| MITRE ATLAS | Apache-2.0 (checked 2026-11-15) | C | ids, names, technique structure, maturity, dates; never the actor of a case study |
| OWASP Top 10 for LLM Applications | CC BY-SA 4.0 (checked 2026-11-15; LICENSE.md reads "This work is licensed under a Creative Commons Attribution-ShareAlike 4.0 International License") | B | ids and names |
| Stanford AI Index | report CC BY-ND 4.0; data per download | D for text and figures; facts free | numbers with locators |
| OECD.AI | no licence stated for the policy database (checked 2026-11-14) | D | names, ids, years and types; never the descriptions or the editors' names and emails |
| EUR-Lex | EU reuse policy, Decision 2011/833/EU, with attribution; only the Official Journal text is authentic | C | the text of articles may be quoted for exact wording under 19.5, with the CELEX number; provisions are summarized fresh |
| US federal works (Federal Register, Congress.gov, NIST) | public domain in the US | A | anything |
| UK legislation | Open Government Licence v3.0 | C | quotes and facts with attribution |
| Chinese laws and regulations | official texts of laws and regulations are not protected by copyright in China (Copyright Law, Article 5); translations are protected | A for the official Chinese text; translations per translator | facts from the official text; quotes in English only from licensed translations with credit |
| other national legal texts | per jurisdiction (checked when the first node needs it) | per row | facts and exact quotes under 19.5 |
| ISO/IEC, IEEE, CEN-CENELEC, ETSI standards | sold, all rights reserved | D | **names only**: titles, numbers, dates, status, clause numbers from public tables of contents; never definitions or text, not even short quotes of definitions (C08) |
| textbooks of the canon (5.10) and the secondary canon | copyright of authors and publishers; some free to read online, which is not a licence | D (D2L text CC BY-SA 4.0 is B) | **names only** for tables of contents and index terms; locators; short quotes under 19.5 |
| syllabi and course sites (CS229, CS231n, MIT OCW CC BY-NC-SA, Coursera, edX) | per site | D | **names and orders only** for the courses of 5.7; exercise links |
| Stanford Encyclopedia of Philosophy, Britannica, PhilPapers | all rights reserved | D | **names only**: entry titles, categories, citations |
| Distill | CC BY 4.0 | C | facts; figures with credit (we still prefer our own scenes) |
| lab blogs, Transformer Circuits, personal blogs | all rights reserved unless stated | D | facts, locators, short quotes |
| Alignment Forum, LessWrong | none stated for the sites (checked 2026-11-15: the FAQ and /api/SKILL.md give none, /termsOfUse is a 404); a post that states its own licence is under it | D | facts and citations |
| Stack Overflow, Reddit, Hacker News | CC BY-SA for SO; others per terms | signals | search terms and counts only |
| GitHub repositories of papers' code | per repository | per licence | never copied into our code (C05); links and pinned commits only |
| algorithm, programming, math2 | our own | A for metadata | bridge cards |

## 19.4 Text and data mining, robots and opt-outs

- The harvest stores copies for three purposes only: provenance, the citation check and the copy check (13.6). It does not train models on them, and the snapshots are never published.
- In the EU, text and data mining for purposes other than research by research organizations is allowed under Article 4 of Directive (EU) 2019/790 unless the rights holder has reserved it in a machine-readable way. The harvester honours `robots.txt`, the TDM Reservation Protocol (`tdmrep.json` and the `tdm-reservation` header), and `noai`-style meta tags: when a source reserves text and data mining, the harvester keeps only the metadata and the facts that a citation needs, and a person checks the cited passage by hand (the citation is marked `person-checked`) (Default).
- When a source asks us to stop, it becomes class E (19.12).
- **Our own site.** The site's content is published for reuse, including by AI systems, under the licences of 19.1. It serves `llms.txt` and allows AI crawlers in `robots.txt`, and its datasets carry their licences (16.10).

## 19.5 Quotes and paraphrase

- **Quotes.** A quote is at most 50 words, marked as a quote, with the source and the locator, from a class that allows it (A, B or C, or D for a short quote with a clear purpose, such as the exact words of a definition in a law or the exact claim of a paper whose wording is debated). A page has at most 3 quotes from one class D source (rule G04, 15.3). Standards' definitions are never quoted (C08).
- **Legal text.** The exact words of a provision matter. A provision card links the official text and summarizes it fresh. The page may quote a sentence of a law under the rule above, with the article and paragraph. It may not reproduce an annex or a list of a law beyond 50 words; it summarizes and links.
- **No close paraphrase.** A section that follows one source sentence by sentence, in the same order with the words changed, is a copy even if no sentence matches. The audit checks for it, and the copy check measures it (13.7).
- **Structure.** The order of a textbook or a survey is a fact about that source and may inform the tree, but our sections follow our own templates (06.3).
- **Tables of results.** A table of numbers from a paper is a set of facts. The project takes the numbers as result cards with the paper as source (10.6), and renders its own tables from the register. It never reproduces a paper's table as an image or with the paper's layout and notes.
- **Equations.** Equations are facts and are written in our notation (06.4). A derivation follows our own steps, and when it follows a source's steps closely, the page says "following <source>, Section x".
- **Code.** Common idioms are not protected. What counts as copying is the same sequence of non-obvious choices in a whole implementation, the same names and comments, or the same worked example with the same data.

## 19.6 People and privacy

AI pages name many living people: researchers, engineers, founders, policy makers, critics, and sometimes the people harmed in incidents.
The rules follow the spirit of Wikipedia's policy on biographies of living persons, and the GDPR's principles of purpose limitation and data minimization.

**Who gets a card or a node.** A person card exists only if the person is notable for AI by at least one of (rule P03):

- the person is an author of a canon work (5.10) or of a work that introduced an item with a node;
- the person is named in the `history.people` of a node, with a source;
- the person received a major award of the field (the Turing Award for AI work, the IJCAI Computers and Thought Award and others in `registers/awards.yaml`) or holds or held a public role that a node needs (a regulator, a minister who signed an AI law, the head of a lab that a node is about);
- the person has a Wikidata item with an English Wikipedia article about them that mentions their AI work.

Authors of work cards appear as names in the bibliographic record (as on DBLP), without a person card unless a rule above applies.
A person node (a page) needs the core or the notable rule of 18.5.

**What a person card or node may hold (rule P01).** Only public facts about published work and public roles, each with a reliable source:

- the name as the person publishes it, aliases, years of birth and death when published by a reliable source (never the full date of birth of a living person);
- affiliations and public roles with years;
- the items they made, named or shaped, and their works;
- awards;
- `same_as` ids (Wikidata, ORCID, DBLP, OpenAlex);
- a photo under C04, only if a free one exists.

Never: contact details, home location, family, health, religion, political views not part of their public AI role, private life, salary or wealth (except what the person or their company discloses as part of a public role and a reliable source reports), social-media posts that are not about their published work, or speculation about motives.

**Other rules.**

- A person node about a living person needs a person review (W5) before it shows anything beyond the card (rule P02, Default).
- Criticism of a living person is allowed only as criticism of published work or of public acts in a public role, from reliable sources, attributed, in neutral words (rule P06; N01 also applies).
- Statements about predictions that people made (timelines, risk estimates) are quoted or summarized with the source and the date, never as the site's judgement of the person (rule P07).
- **Incidents.** An incident card names organizations and systems. It names a private individual (a victim, a bystander, an employee) only when the person is a public figure in the matter, at least two reliable sources name them, and the name is needed to identify the incident. Minors are never named. The card never describes the harm in more detail than a reader needs to classify it (rule P04).
- **Synthetic media.** The site never shows AI-generated images, audio or video of a real person. Pages on deepfakes and synthetic media use synthetic subjects or subjects who consented in a public, licensed dataset (rule P05).
- **Recently dead.** A person who died less than two years ago is treated as living (rule P08).
- **Requests.** A person who is the subject of a card or a page can ask for a correction, or for the page to be cut back to the card; the default answer is yes (19.12).

**Readers.** The site sets no cookies and loads nothing from a third party (16). Preferences, course progress and notebook state live in the reader's browser only. The in-browser notebooks never send code or data to our servers (12.8). Page view counts come from Cloudflare's aggregated logs by path. `/about/privacy/` says all of this in plain words.

**Contributors.** Names and e-mail addresses are in git history, as in any public repository. Provenance logs store the GitHub login only.

## 19.7 Model, dataset and software licences as facts

AI artifacts carry many licences that are not standard open source licences: the Llama community licences, the Gemma terms of use, the OpenRAIL family (CreativeML OpenRAIL-M, BigScience OpenRAIL-M), research-only licences, API terms, and datasets with a mix of licences per part.

- **A licence is a fact.** Each model, dataset and software card has `licence`: an SPDX id when the licence is on the SPDX list, otherwise a licence card (`registers/licences.yaml`) with the licence's name, version, URL, a snapshot hash, the date checked, and a fixed set of fact fields (commercial use allowed: yes, no or with conditions; a user threshold, such as the 700 million monthly active users clause of the Llama licences; field-of-use restrictions; attribution; share-alike; whether outputs may train other models; whether it is approved by the OSI).
- **No legal advice.** A licence card states what the text says, with the clause, and does not say whether a use is lawful. The site never says "you may" or "you may not" in its own voice; it says "clause 2 of the licence says …" (rule C12).
- **"Open source" is a term with a definition.** The site uses "open source" for a model only when the model meets the OSI Open Source AI Definition 1.0 as stated by the OSI or by a reliable source, and otherwise uses the neutral terms of 04.5: `open-weights`, `open-source`, `api`, `closed` (rule C12). The `access` field is a fact with a source.
- **Datasets.** A dataset's licence field lists the licence of each part when the parts differ, and `known_issues` records published problems with consent, copyright or harmful content, with sources (04.5).
- **Changes.** Licences of models change after release. The licence fact has `as_of`, and the daily job (14.9) watches the licence files of tracked repositories.

## 19.8 What the project never hosts or publishes

- **Weights.** The project never hosts model weights of others. Experiments (09) train small models of our own; their checkpoints stay in `ai-runs` and are published only when they are small, trained on data whose licence allows it, and useful to a reader (Default).
- **Datasets.** The project mirrors a dataset in `ai-data` (09.5) only when its licence allows redistribution, and it never mirrors web-scraped image or video datasets, or datasets with known reports of harmful or illegal content, even when their licence would allow it (rule C10).
- **Other people's papers.** PDFs are snapshots for checking only (13.6). The site links to the open version (arXiv, the venue, Unpaywall).
- **Illegal and abusive content.** No child sexual abuse material, no non-consensual intimate imagery, no content that the Digital Omnibus prohibition covers, in any form, by link or by description beyond what is needed to name a risk or an incident (rule C13). A harvested item that may contain or link such content is quarantined, never fetched further, and reported through the proper channel by a person.
- **Hazardous instructions.** Risk, security and safety pages teach classes of attacks and failures so that readers can recognize and prevent them. They follow these limits (rule C14):
  - no working jailbreak or prompt-injection strings aimed at deployed models; examples are abstracted ("a role-play framing that asks the model to ignore its instructions"), or are the published examples of a paper against a model that is retired or patched, with the citation;
  - no content that gives meaningful uplift toward biological, chemical, radiological, nuclear or cyber weapons, or toward attacks on critical infrastructure; dangerous capability evaluations are described by their design and results, never by their test items;
  - no malware, no exploit code for real systems, no scanners for targets; attack demos (an adversarial example on MNIST, a membership inference attack on a model we trained) run only on our own toy models and data in the sandbox (08.9);
  - pages under `robustness-security-and-privacy` and `ai-safety-and-alignment` that describe attacks are marked `hazard: review` and need a person review before they publish (Default, Q-24).
- **Responsible disclosure.** If a writer or an agent finds what looks like a real, unreported vulnerability in a deployed AI system or AI software while working, the run stops that item and a person reports it privately through the vendor's channel. Nothing about it is published until it is fixed or public, as in programming 19.7.

## 19.9 Content written by agents

- **Provenance.** Each change has a provenance record (04.8) with the author: a GitHub login or an agent run id, with the model ids, the prompt versions and the sources given.
- **Disclosure on pages.** Each page is labelled by how it was made, from the provenance and review records: "Written by AI, checked by the build" (W4 without review), "Reviewed by <name>" (W5), or "Written by <name>". The site has a statement on `/about/ai/` that explains the pipeline (14) and the checks (15).
- **The EU AI Act.** Article 50(4) of Regulation (EU) 2024/1689, which applies from 2026-08-02, asks deployers of an AI system that generates text published to inform the public on matters of public interest to disclose that the text is AI-generated, unless the text has undergone human review or editorial control and a person holds editorial responsibility. The project's per-page labels and the site statement are written to meet this disclosure whether or not it applies to the project (Default, Q-25). The `.md` and `.json` twins carry the same label in a field, so that agents that reuse the text see it.
- **Licence of agent output.** Agent output is published under the licence of its zone, with "The ai contributors" as the copyright holder, as in algorithm 17.4.
- **Upstream sites.** The project does not post AI-written text to Wikipedia, Wikidata, Stack Overflow, forums or any upstream site. An error found upstream is reported by a person.
- **Model terms.** The project uses model providers whose terms give the user the rights in the output, recorded in `registers/models.yaml` (14.4). Provider terms that restrict using outputs to train competing models bind the project, which does not train such models; the published content is under the licences of 19.1 for everyone.

## 19.10 Trademarks

- **Nominative use only.** Names of models, labs, companies and products are used to identify them, in the form their owners write them (5.11). The site does not use logos or brand colours (Default: no logos at all).
- **No sign of endorsement.** The About page and the footer say that the site is independent and not endorsed by any lab, company or body that it describes.
- Model and product names that are trademarks (GPT, Claude, Gemini, Llama, and others) are listed on `/about/legal/` with their owners where the owner asks for it.

## 19.11 Corrections of facts

Facts about artifacts and organizations are often disputed by their makers.
A correction from a developer or a rights holder is handled as any source: if it comes with a primary source, the fact model records it (04.9); if not, the request is recorded and the fact stays with its sources, with a note that the developer disputes it (Default).

## 19.12 Takedown, corrections and contact

**How to report.** Each page has "Report a problem" (16), which opens a GitHub issue with the URL filled in and a choice: wrong, outdated, unclear, copied, rights, person, harmful, security, other. A rights holder or a person who does not want a public issue writes to the address on `/about/legal/` (Default: an alias that forwards to the maintainers). Reports marked harmful or person are never public issues; the form routes them to the alias.

| Report | Within | Action |
|---|---|---|
| a claim that text, code or a figure was copied | 2 working days | a person runs the copy check against the named source; if the claim holds, the part is quarantined (it leaves the site at the next deploy), and a fresh version is queued with the source added to the corpus |
| a DMCA notice or the like | 2 working days | the same, following the notice's procedure; the file leaves the next release archive; the notice is kept in `docs/legal/notices/` without personal details |
| a source asks us to stop harvesting | 2 working days | the source becomes class E; its adapter stops; its snapshots are deleted from `ai-raw` unless needed as evidence of a dispute; facts in cards stay, with the citation marked "source withdrawn" where the source asks |
| a request about a living person | 5 working days | the card or page is fixed or cut back (19.6) |
| a report of harmful or hazardous content | 1 working day | the part is quarantined at once by a person; then reviewed against 19.8 |
| a trademark complaint | 5 working days | a person checks the use against 19.10 |
| a licence fact or a developer's correction | 5 working days | as 19.11 |
| a security report about the site or the toolchain | per `SECURITY.md` | private handling, then a fix and a note |
| an error in content | no deadline | the work queue with a high weight (14.5); the reader is credited if they wish |

Removed content stays in git history, which is public. When a court order or a valid notice requires it, or when content falls under 19.8's illegal content rule, the maintainers rewrite history for that path only and publish a note that they did.

## 19.13 Rules

Listed in 15.3 with the other rule families.

**C: copying, licences and content limits.**

| Rule | Checks |
|---|---|
| C01 | no copied text, code or figure from a source whose licence forbids it: the copy check (13.7) passes; for prose, no section shares more than 5% of its 8-word shingles with any one source and no aligned run of 3 or more paraphrased sentences; for code, winnowing similarity below 30% against every file of the code corpora outside the idiom allow-list; for figures, no perceptual hash match with a source figure unless the source's class allows it and the caption says "after <source>" |
| C02 | `reuse lint` passes: every file has a licence from `REUSE.toml` |
| C03 | every source with `adapted: true` is class A, B or C, and the page footer carries the attribution that the licence asks |
| C04 | a photo on a person, system or hardware page is from Wikimedia Commons under CC0, public domain, CC BY or CC BY-SA, with a credit line; otherwise there is no photo |
| C05 | no file in the MIT zone (implementations, tests, experiments, scenes, contracts) has its origin in a class B or D source in `prov/` |
| C06 | no text field of a card or a register (`summary`, `title`, `note`) has its origin in a class B, C or D source; card text is written fresh or comes from a class A source |
| C07 | every cited source has a row in `registers/sources.yaml` with a licence, a class and the date the licence was checked; every card field from a class C database carries its source licence (19.2) |
| C08 | no text from a standard (ISO/IEC, IEEE, CEN-CENELEC, ETSI) beyond its title, number, date, status and clause numbers; no definition of a standard quoted |
| C09 | no exercise card or page has a run of more than 12 words that matches the captured statement of the exercise's site |
| C10 | no dataset in `ai-data` without a licence that allows redistribution in `registers/licences.yaml`, and no web-scraped image or video dataset, and no dataset with a known report of illegal content |
| C11 | no model weights of others in any store; every model link points to the publisher's location with a pinned revision |
| C12 | every model, dataset and software card has a licence field (SPDX id or licence card); no page says "you may" or "you may not" about a licence in the site's own voice; "open source" for a model only with a source for OSI conformance |
| C13 | no child sexual abuse material, non-consensual intimate imagery, or other illegal content in any form, by link or by description beyond naming the risk or the incident; a flagged item is quarantined |
| C14 | a page marked `hazard: review` has a person review of this hash before it publishes; no page contains a working jailbreak or injection string aimed at a deployed model, test items of a dangerous capability evaluation, or weapons uplift (19.8) |
| C15 | the page and its `.md` and `.json` twins carry the AI disclosure label of 19.9 |

**P: people and privacy.**

| Rule | Checks |
|---|---|
| P01 | no private fact about a living person: a person card or node has no field outside the allowed list of 19.6, and the prose of any page has no such fact (the audit checks sentences that mention a person) |
| P02 | a person node about a living person shows only its card until a person review (W5) of this hash exists |
| P03 | every person card meets one of the notability rules of 19.6, with the rule recorded |
| P04 | an incident card or page names no private individual unless the conditions of 19.6 hold, and never a minor |
| P05 | no AI-generated image, audio or video of a real person on any page |
| P06 | criticism of a living person is about published work or public acts, attributed to a reliable source, in neutral words |
| P07 | a statement of a person's prediction or estimate has the source and the date |
| P08 | a person who died less than two years ago is checked as living |
