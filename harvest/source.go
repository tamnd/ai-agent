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

// Sources are the five sources of M0 (spec 21.2, deliverable 6). The licences were read on the sources' own pages
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
