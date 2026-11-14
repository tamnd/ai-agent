// SPDX-License-Identifier: Apache-2.0

// Package harvest runs the adapters of spec 13.2: each one fetches a source with the User-Agent of aik, keeps the
// bytes it fetched in the raw store (13.6), and writes one inventory row per item that the source names (13.3).
package harvest

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"
)

// Source is a row of registers/sources.yaml (spec 13.10): who owns the source, its licence with the date it was
// checked, its class of 19.2, how the adapter reads it, and the credit line that the site shows.
type Source struct {
	ID       string `yaml:"id"`
	Name     string `yaml:"name"`
	URL      string `yaml:"url"`
	Owner    string `yaml:"owner"`
	Licence  string `yaml:"licence"`
	Checked  string `yaml:"checked"`
	Class    string `yaml:"class"`
	Access   string `yaml:"access"`
	Rate     string `yaml:"rate"`
	Schedule string `yaml:"schedule"`
	Standing string `yaml:"standing"`
	Facts    string `yaml:"facts"`
	Credit   string `yaml:"credit"`
	Note     string `yaml:"note,omitempty"`
}

// Sources are the sources that an adapter reads (13.2), the first five of M0 (spec 21.2, deliverable 6). The licences were read on the sources' own pages
// on the date in Checked; a person approves each class (13.10).
var Sources = []Source{
	{ //nolint:gosec // G101: the licence and the URL of a public source, not a credential
		ID: "epoch-ai", Name: "Epoch AI, Data on AI Models and Data on Machine Learning Hardware",
		URL: "https://epoch.ai/data/ai-models", Owner: "Epoch AI",
		Licence: "CC-BY-4.0", Checked: "2026-11-14", Class: "C",
		Access: "CSV downloads: all_ai_models.csv and ml_hardware.csv", Rate: "1 request per 2 s",
		Schedule: "weekly", Standing: "secondary-reference",
		Facts:  "secondary for parameters, training compute, dates and hardware specifications",
		Credit: "Data from Epoch AI, Data on AI Models (CC BY 4.0), https://epoch.ai/data/ai-models",
	},
	{ //nolint:gosec // G101: the licence and the URL of a public source, not a credential
		ID: "epoch-benchmarks", Name: "Epoch AI, Benchmarking Hub",
		URL: "https://epoch.ai/benchmarks", Owner: "Epoch AI",
		Licence: "CC-BY-4.0", Checked: "2026-11-14", Class: "C",
		Access: "benchmark_data.zip, one request: a CSV for each benchmark, benchmark_metadata.csv and model_metadata.csv",
		Rate:   "1 request per 2 s", Schedule: "weekly", Standing: "secondary-reference",
		Facts:  "results: Epoch AI's own runs, with the date, the standard error and the log; and the scores it copies from other leaderboards, with their source",
		Credit: "Epoch AI, 'Capabilities & benchmarking' (CC BY 4.0), https://epoch.ai/benchmarks",
		Note:   "Only the benchmarks that benchmark_metadata.csv maps to a file and a score column are read; the score keeps the hub's number and its scale (fraction or percent).",
	},
	{
		ID: "pwc-dump", Name: "Papers with Code, the final data dump",
		URL: "https://huggingface.co/datasets/pwc-archive/files", Owner: "Papers with Code (Meta AI); archived by the pwc-archive organization on Hugging Face",
		Licence: "CC-BY-SA-4.0", Checked: "2026-11-14", Class: "B",
		Access: "the JSON files of 2025-07-28 at a pinned revision of the Hugging Face dataset pwc-archive/files", Rate: "1 request per 2 s",
		Schedule: "once; the site closed in July 2025", Standing: "secondary-reference",
		Facts:  "ids, names and numbers only, rechecked against the primary papers (19.2)",
		Credit: "Papers with Code data dump (CC BY-SA 4.0), archived at https://huggingface.co/datasets/pwc-archive/files",
		Note:   "Methods whose names are advertising (the dump holds about 6,800 of them, such as customer service phone numbers) are skipped.",
	},
	{
		ID: "pwc-results", Name: "Papers with Code, the evaluation tables of the final data dump",
		URL: "https://huggingface.co/datasets/pwc-archive/files", Owner: "Papers with Code (Meta AI); archived by the pwc-archive organization on Hugging Face",
		Licence: "CC-BY-SA-4.0", Checked: "2026-11-14", Class: "B",
		Access: "jul-28-evaluation-tables.json.gz at the pinned revision of pwc-archive/files, one request", Rate: "1 request per 2 s",
		Schedule: "once; the site closed in July 2025", Standing: "secondary-reference",
		Facts:  "the model, dataset, metric and number of each row, with its paper; every number is checked against the paper (Q-35)",
		Credit: "Papers with Code data dump (CC BY-SA 4.0), archived at https://huggingface.co/datasets/pwc-archive/files",
		Note:   "Each value is a result row whose source is the paper and whose secondary source is the dump, flagged pwc-unverified until the citation check finds the number in the paper; a value with no percent sign has the unit number.",
	},
	{
		ID: "aiid", Name: "AI Incident Database",
		URL: "https://incidentdatabase.ai/research/snapshots/", Owner: "Responsible AI Collaborative",
		Licence: "CC-BY-SA-4.0", Checked: "2026-11-14", Class: "B",
		Access: "the weekly snapshot export (a MongoDB dump with CSV files)", Rate: "1 request per 2 s",
		Schedule: "weekly", Standing: "secondary-reference",
		Facts:  "incident ids, titles, dates and the entities that the database names; never the descriptions",
		Credit: "AI Incident Database, Responsible AI Collaborative (CC BY-SA 4.0), https://incidentdatabase.ai",
		Note:   "The snapshot's license.txt: the database is CC BY-SA 4.0 unless noted; report contents keep their own rights (Q-23).",
	},
	{
		ID: "arxiv", Name: "arXiv metadata",
		URL: "https://info.arxiv.org/help/api/", Owner: "Cornell University",
		Licence: "CC0-1.0", Checked: "2026-11-14", Class: "A",
		Access: "OAI-PMH (oaipmh.arxiv.org), one set per category; the robots.txt of export.arxiv.org, the host of the search API, disallows every path", Rate: "1 request per 3 s",
		Schedule: "daily", Standing: "preprint",
		Facts:  "titles, authors, dates, categories, DOIs and journal references; texts keep their own licences",
		Credit: "Thank you to arXiv for use of its open access interoperability.",
		Note:   "The metadata is CC0; the abstracts are not kept.",
	},
	{ //nolint:gosec // G101: the licence and the URL of a public source, not a credential
		ID: "wikidata", Name: "Wikidata",
		URL: "https://www.wikidata.org/", Owner: "Wikimedia Foundation",
		Licence: "CC0-1.0", Checked: "2026-11-14", Class: "A",
		Access: "SPARQL on the QLever endpoint of the University of Freiburg (qlever.dev/api/wikidata), one query per class of AI items; the robots.txt of query.wikidata.org disallows /sparql", Rate: "1 request per 2 s",
		Schedule: "weekly", Standing: "secondary-reference",
		Facts:  "QIDs, labels, aliases and the identifiers of other sources",
		Credit: "Wikidata (CC0)",
	},
	{ //nolint:gosec // G101: the licence and the URL of a public source, not a credential
		ID: "openalex", Name: "OpenAlex",
		URL: "https://openalex.org/", Owner: "OurResearch",
		Licence: "CC0-1.0", Checked: "2026-11-14", Class: "A",
		Access: "the REST API (api.openalex.org/works) with no key and no address; for each year of each part of crosswalk/openalex.yaml, the top 2% of works by citations (S-W1), by basic paging", Rate: "1 request per 1 s",
		Schedule: "weekly", Standing: "secondary-reference",
		Facts:  "work ids, titles, years, dates, authors, institutions, topics, DOIs and citation counts; never the abstracts",
		Credit: "OpenAlex (CC0), https://openalex.org",
		Note:   "Without a key the API gives a small daily budget, and cursor paging past the first page needs a key, so basic paging reads at most 10,000 works of a year. The API documentation asks agents to send a person's email address for a key; aik does not.",
	},
	{ //nolint:gosec // G101: the licence and the URL of a public source, not a credential
		ID: "huggingface", Name: "Hugging Face Hub",
		URL: "https://huggingface.co/", Owner: "Hugging Face",
		Licence: "facts only", Checked: "2026-11-14", Class: "D",
		Access: "the public API (huggingface.co/api/models and /api/datasets) with no token, sorted by downloads and by likes down to the thresholds of S-M2 and S-D1, 1,000 repositories a page; robots.txt allows all", Rate: "1 request per 1 s (the API allows 500 per 5 minutes)",
		Schedule: "weekly", Standing: "secondary-reference",
		Facts:  "repository ids, owners, downloads in the last 30 days, likes, tags, licence ids, base models, arXiv ids, commit hashes and dates; never the card text",
		Credit: "Hugging Face Hub, https://huggingface.co",
		Note:   "The metadata is facts (19.3); each repository's card and files keep their own licence, D by default.",
	},
	{ //nolint:gosec // G101: the licence and the URL of a public source, not a credential
		ID: "github", Name: "GitHub",
		URL: "https://github.com/", Owner: "GitHub",
		Licence: "facts only", Checked: "2026-11-14", Class: "D",
		Access: "the repository search of the REST API (api.github.com/search/repositories) with no token, one query per AI topic of crosswalk/github-topics.yaml with at least 5,000 stars (S-S1), split by stars past 1,000 results; api.github.com has no robots.txt", Rate: "1 request per 7 s (the API allows 10 searches a minute without a token)",
		Schedule: "weekly", Standing: "secondary-reference",
		Facts:  "repository names, ids, owners, stars, forks, AI topics, languages, licence ids and dates; never the descriptions or the READMEs",
		Credit: "GitHub, https://github.com",
		Note:   "The metadata is facts (19.3); the code and the READMEs keep their own licences.",
	},
	{ //nolint:gosec // G101: the licence and the URL of a public source, not a credential
		ID: "acl", Name: "ACL Anthology",
		URL: "https://aclanthology.org/", Owner: "the Association for Computational Linguistics",
		Licence: "CC-BY-4.0", Checked: "2026-11-14", Class: "C",
		Access: "the XML of the Anthology's repository (github.com/acl-org/acl-anthology, data/xml): the tree of data/xml by the GitHub API, then the file of each collection of the venues of crosswalk/acl.yaml from raw.githubusercontent.com; the volumes of other venues, such as the workshops in a conference's file, are not read", Rate: "1 request per 1 s",
		Schedule: "per conference", Standing: "secondary-reference",
		Facts:  "Anthology ids, titles, authors, venues, years, DOIs and awards; never the abstracts",
		Credit: "ACL Anthology (CC BY 4.0), https://aclanthology.org",
		Note:   "The metadata and the papers since 2016 are CC BY 4.0, the papers before 2016 CC BY-NC-SA 3.0; the code of the repository is Apache 2.0. The site lists awards only for some years.",
	},
	{ //nolint:gosec // G101: the licence and the URL of a public source, not a credential
		ID: "dblp", Name: "dblp computer science bibliography",
		URL: "https://dblp.org/", Owner: "Schloss Dagstuhl - Leibniz Center for Informatics",
		Licence: "CC0-1.0", Checked: "2026-11-14", Class: "A",
		Access: "the SPARQL endpoint sparql.dblp.org/sparql: for each venue stream of crosswalk/dblp.yaml, the papers (key, title, year, DOI, Wikidata item) and their authors in order, two queries of 10,000 papers a page; the robots.txt of dblp.org disallows /search, the search API, and its other pages answer robots with a challenge", Rate: "1 request per 10 s",
		Schedule: "weekly", Standing: "secondary-reference",
		Facts:  "dblp keys, titles, venues, years, DOIs, Wikidata items, author names and dblp person ids",
		Credit: "dblp computer science bibliography (CC0), https://dblp.org",
		Note:   "The README of dblp.org/xml/ releases the metadata under CC0 1.0, and under ODC-BY 1.0 as a second licence. The robots.txt of sparql.dblp.org asks a Crawl-delay of 10 s. dblp's homonym numbers (Hu Xu 0001) are dropped from the names and kept in the person ids.",
	},
	{
		ID: "federal-register", Name: "Federal Register",
		URL: "https://www.federalregister.gov/", Owner: "the Office of the Federal Register, US National Archives",
		Licence: "public domain (a work of the US federal government)", Checked: "2026-11-14", Class: "A",
		Access: "the documents API (federalregister.gov/api/v1/documents.json) with no key: the documents with the phrase \"artificial intelligence\", 1,000 a page, of which those with an AI title are rows; robots.txt closes only the search pages of the site", Rate: "1 request per 1 s",
		Schedule: "daily", Standing: "primary",
		Facts:  "document numbers, titles, types, executive order numbers, citations, agencies and the dates of publication, signing and effect; never the abstracts",
		Credit: "Federal Register, https://www.federalregister.gov",
		Note:   "The Federal Register covers the US federal government from 1994; the states and other countries come from their own gazettes (S-L1).",
	},
	{ //nolint:gosec // G101: the licence and the URL of a public source, not a credential
		ID: "eur-lex", Name: "EUR-Lex, through the Cellar of the Publications Office",
		URL: "https://eur-lex.europa.eu/", Owner: "the Publications Office of the European Union",
		Licence: "the reuse policy of the Commission (Decision 2011/833/EU), with acknowledgement; only the Official Journal text is authentic", Checked: "2026-11-15", Class: "C",
		Access: "the SPARQL endpoint of the Cellar (publications.europa.eu/webapi/rdf/sparql) with no key, one query for the works indexed under the EuroVoc concept artificial intelligence (3030); robots.txt of publications.europa.eu allows it, and eur-lex.europa.eu answers scripts with a challenge, so the adapter never fetches it", Rate: "1 request per 1 s",
		Schedule: "daily", Standing: "primary",
		Facts:  "CELEX numbers, ELI ids, resource types, English titles, the dates of the document, of entry into force, of application and of end of validity, and the CELEX numbers of the acts amended; never the text",
		Credit: "EUR-Lex, © European Union, https://eur-lex.europa.eu",
		Note:   "The EuroVoc index finds the acts about AI, not every act that has an AI provision; a curator adds the others to registers/watch.yaml (S-L1). Corrigenda, case law, national measures, parliamentary questions and notices are skipped.",
	},
	{ //nolint:gosec // G101: the licence and the URL of a public source, not a credential
		ID: "oecd-ai", Name: "OECD.AI policy database",
		URL: "https://oecd.ai/en/dashboards/policy-initiatives", Owner: "the OECD AI Policy Observatory",
		Licence: "facts only", Checked: "2026-11-14", Class: "D",
		Access: "the public API of the initiatives (api.oecd.ai/policy-initiatives/public) with no key, 100 a page; robots.txt of oecd.ai closes only French pages and api.oecd.ai has none", Rate: "1 request per 1 s",
		Schedule: "weekly", Standing: "secondary-reference",
		Facts:  "initiative ids, English and original names, types, categories, countries or organisations, binding or not, status, years, tags, sectors and websites; never the descriptions, and never the names or emails of the editors that the API also gives",
		Credit: "OECD.AI Policy Observatory, https://oecd.ai",
		Note:   "The site states no licence for the database, so it is D: names, ids, years and categories only. The initiatives are reported by the governments and organisations themselves.",
	},
	{ //nolint:gosec // G101: the licence and the URL of a public source, not a credential
		ID: "swe-bench", Name: "SWE-bench leaderboards",
		URL: "https://www.swebench.com/", Owner: "the SWE-bench team (Princeton and Stanford)",
		Licence: "CC-BY-NC-4.0", Checked: "2026-11-14", Class: "D",
		Access: "the data file of the site in its repository (github.com/SWE-bench/swe-bench.github.io, data/leaderboards.json) from raw.githubusercontent.com: every leaderboard in one request; robots.txt of swebench.com sets no content signal and disallows nothing", Rate: "1 request per 1 s",
		Schedule: "weekly", Standing: "secondary-reference",
		Facts:  "the system, the model, the score (% resolved), the date, the scaffold, the attempts and the submitter of each entry; never the logs or the trajectories",
		Credit: "SWE-bench, https://www.swebench.com",
		Note:   "The site is CC BY-NC 4.0, so it is D: names and numbers only. An entry run by the maintainers with mini-SWE-agent is independent of the model's developer; an entry that a developer submits for its own model is self-reported (10.6.1).",
	},
	{ //nolint:gosec // G101: the licence and the URL of a public source, not a credential
		ID: "open-llm-leaderboard", Name: "Open LLM Leaderboard v2, the archive of its results",
		URL: "https://huggingface.co/datasets/open-llm-leaderboard/contents", Owner: "Hugging Face",
		Licence: "none stated", Checked: "2026-11-14", Class: "D",
		Access: "the rows of the dataset open-llm-leaderboard/contents from the dataset server of Hugging Face, 100 rows a request (46 requests)", Rate: "1 request per 1 s",
		Schedule: "once; the leaderboard closed in March 2025", Standing: "secondary-reference",
		Facts:  "the raw score of each open model on IFEval, BBH, MATH Level 5, GPQA, MuSR and MMLU-PRO, with its precision, revision and submission date",
		Credit: "Open LLM Leaderboard (Hugging Face), https://huggingface.co/datasets/open-llm-leaderboard/contents",
		Note:   "The dataset states no licence, so it is D: names and numbers only. Hugging Face ran every model on one harness and protocol, so its results are independent of the developers. The score is the raw score of the harness; the leaderboard's normalized score is in the notes.",
	},
	{ //nolint:gosec // G101: the licence and the URL of a public source, not a credential
		ID: "lmarena", Name: "LMArena (arena.ai), the published leaderboards",
		URL: "https://huggingface.co/datasets/lmarena-ai/leaderboard-dataset", Owner: "LMArena",
		Licence: "CC-BY-4.0", Checked: "2026-11-14", Class: "C",
		Access: "the split latest of each config of the dataset lmarena-ai/leaderboard-dataset, from the rows endpoint of the dataset server of Hugging Face, 100 rows a request; the rows of the category overall are kept", Rate: "1 request per 1 s",
		Schedule: "weekly", Standing: "secondary-reference",
		Facts:  "the rating of each model in each arena, with its 95% interval, its number of battles, its rank, the developer and the date the leaderboard was published",
		Credit: "LMArena leaderboard data (CC BY 4.0), https://huggingface.co/datasets/lmarena-ai/leaderboard-dataset",
		Note:   "A rating is the preference of the voters of one arena over one period, comparable only inside one snapshot of one arena (10.10). The dataset's notes list the changes of method: Bradley-Terry in place of Elo from 2024-01-09, style control by default from 2025-05-16, re-weighting by frequency from 2025-07-23. The Agent Arena configs give an IPS score, not a rating, and are not read. ARC Prize is not a source: the terms of arcprize.org forbid access by a bot or a script and the aggregation of its content (read 2026-11-14), so its results come from Epoch AI's Benchmarking Hub.",
	},
	{ //nolint:gosec // G101: the licence and the URL of a public source, not a credential
		ID: "helm", Name: "HELM (Holistic Evaluation of Language Models), the published leaderboards",
		URL: "https://crfm.stanford.edu/helm/", Owner: "Stanford Center for Research on Foundation Models",
		Licence: "none stated for the results (the code is Apache-2.0)", Checked: "2026-11-15", Class: "D",
		Access: "the list of projects (crfm.stanford.edu/helm/project_metadata.json), then for the newest release of each project of crosswalk/helm.yaml its summary.json, schema.json, groups.json and the main table groups/<group>.json, from the public bucket storage.googleapis.com/crfm-helm-public: 4 requests a project; the bucket has no robots.txt", Rate: "1 request per 1 s",
		Schedule: "monthly; a project gets a new release a few times a year", Standing: "secondary-reference",
		Facts:  "the score of each model on each scenario of the main table of a project (Lite, Classic, Capabilities, MMLU, Safety, AIR-Bench, MedHELM, Finance, Long Context, Instruct), with its metric, the model's id in HELM, its developer and the date of the release",
		Credit: "HELM, Stanford CRFM, https://crfm.stanford.edu/helm/",
		Note:   "The results state no licence, so they are D: names and numbers only. CRFM ran every model on its own harness, prompts and settings, so the results are independent of the developers, but comparable only inside one release of one project. A score of a proportion (exact match, F1, accuracy, rates) is a fraction; a rating (Jury Score, WB Score, Instruct's 1 to 5 scales) or a text metric (BLEU, ROUGE) is a number. The mean win rate and the mean score are of the table and are not read.",
	},
}

// SourceByID gives the source of an id, or nil.
func SourceByID(id string) *Source {
	for i := range Sources {
		if Sources[i].ID == id {
			return &Sources[i]
		}
	}
	return nil
}

// SourcesFile is the register of sources in the content repository.
const SourcesFile = "registers/sources.yaml"

const sourcesHeader = `# The sources of spec 13.2: each one with its owner, its licence and the date it was checked, its class of 19.2,
# how the adapter reads it, its rate limit and schedule, and the credit line that the site shows (13.10).
# A person approves the class of each source. aik harvest adds the row of its source when the row is missing,
# and never changes a row that is here.
`

// Register adds the row of src to registers/sources.yaml when the file has no row with its id. A row that is there
// is left as a person wrote it. It tells if it added the row.
func Register(root string, src Source) (bool, error) {
	path := filepath.Join(root, SourcesFile)
	b, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}
	var rows []Source
	if err := yaml.Unmarshal(b, &rows); err != nil {
		return false, fmt.Errorf("%s: %w", SourcesFile, err)
	}
	for _, r := range rows {
		if r.ID == src.ID {
			return false, nil
		}
	}
	if len(b) == 0 {
		b = []byte(sourcesHeader)
	}
	if !bytes.HasSuffix(b, []byte("\n")) {
		b = append(b, '\n')
	}
	row, err := yaml.Marshal([]Source{src})
	if err != nil {
		return false, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	return true, os.WriteFile(path, append(b, row...), 0o644)
}
