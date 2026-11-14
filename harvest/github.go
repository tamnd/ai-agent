// SPDX-License-Identifier: Apache-2.0

package harvest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// GitHubSearch is the repository search of the GitHub API. Without a token it allows 10 searches a minute and gives at
// most 1,000 results of a query, so a query with more is split by stars.
const GitHubSearch = "https://api.github.com/search/repositories"

// GitHubStars is the threshold of S-S1 (18.5): a repository with an AI topic and this many stars is a card.
const GitHubStars = 5000

// GitHubTopicsFile lists the topics of GitHub that are AI; without it the adapter reads GitHubTopics.
const GitHubTopicsFile = "crosswalk/github-topics.yaml"

// GitHubTopics are the default AI topics.
var GitHubTopics = []string{
	"artificial-intelligence", "ai", "machine-learning", "deep-learning", "neural-network", "neural-networks",
	"reinforcement-learning", "computer-vision", "nlp", "natural-language-processing", "llm", "llms",
	"large-language-models", "generative-ai", "transformers", "transformer", "pytorch", "tensorflow", "jax", "keras",
	"scikit-learn", "huggingface", "diffusion-models", "stable-diffusion", "gpt", "chatgpt", "openai", "rag",
	"ai-agents", "agents", "autonomous-agents", "multi-agent-systems", "langchain", "llama", "ollama", "mlops",
	"llmops", "llm-inference", "model-serving", "speech-recognition", "text-to-speech", "object-detection",
	"image-generation", "multimodal", "embeddings", "vector-database", "prompt-engineering",
	"model-context-protocol", "mcp", "ai-safety", "interpretability", "federated-learning", "automl",
}

// gitHubMaxResults is the most results that a search gives.
const gitHubMaxResults = 1000

var topicNameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,49}$`)

// gitHubTopics reads the topics file, or gives the default topics.
func gitHubTopics(root string) ([]string, error) {
	b, err := os.ReadFile(filepath.Join(root, GitHubTopicsFile))
	if os.IsNotExist(err) {
		return GitHubTopics, nil
	}
	if err != nil {
		return nil, err
	}
	var doc struct {
		Topics []string `yaml:"topics"`
	}
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return nil, &ErrSchema{"github", GitHubTopicsFile + " is not a map with topics: " + err.Error()}
	}
	var out []string
	for _, t := range doc.Topics {
		if !topicNameRE.MatchString(t) {
			return nil, &ErrSchema{"github", GitHubTopicsFile + ": " + strconv.Quote(t) + " is not a topic"}
		}
		out = append(out, t)
	}
	if len(out) == 0 {
		return nil, &ErrSchema{"github", GitHubTopicsFile + " names no topic"}
	}
	return out, nil
}

// Stars is a range of stars; Hi 0 has no top.
type Stars struct{ Lo, Hi int }

func (s Stars) String() string {
	if s.Hi == 0 {
		return ">=" + strconv.Itoa(s.Lo)
	}
	return strconv.Itoa(s.Lo) + ".." + strconv.Itoa(s.Hi)
}

// Split halves the range: an open range at twice its bottom, a closed one at its middle. ok is false when the range
// is one number.
func (s Stars) Split() (Stars, Stars, bool) {
	if s.Hi == 0 {
		return Stars{s.Lo, 2*s.Lo - 1}, Stars{2 * s.Lo, 0}, true
	}
	if s.Lo >= s.Hi {
		return s, s, false
	}
	m := (s.Lo + s.Hi) / 2
	return Stars{s.Lo, m}, Stars{m + 1, s.Hi}, true
}

// GitHubURL is page n (from 1) of the repositories of the topic in the range of stars, the most starred first.
func GitHubURL(topic string, s Stars, page int) string {
	return GitHubSearch + "?" + url.Values{
		"q":        {"topic:" + topic + " stars:" + s.String() + " fork:false"},
		"sort":     {"stars"},
		"order":    {"desc"},
		"per_page": {"100"},
		"page":     {strconv.Itoa(page)},
	}.Encode()
}

// GitHubRepo is a repository of a search page.
type GitHubRepo struct {
	ID       int64    `json:"id"`
	FullName string   `json:"full_name"`
	Stars    int      `json:"stargazers_count"`
	Forks    int      `json:"forks_count"`
	Topics   []string `json:"topics"`
	Language string   `json:"language"`
	Created  string   `json:"created_at"`
	Pushed   string   `json:"pushed_at"`
	Archived bool     `json:"archived"`
	Fork     bool     `json:"fork"`
	Licence  *struct {
		SPDX string `json:"spdx_id"`
	} `json:"license"`
	Owner struct {
		Login string `json:"login"`
		Type  string `json:"type"`
	} `json:"owner"`
}

// GitHubPage reads a search page: the total of the query and the repositories of the page.
func GitHubPage(b []byte) (int, []GitHubRepo, error) {
	var r struct {
		Total      *int         `json:"total_count"`
		Incomplete bool         `json:"incomplete_results"`
		Items      []GitHubRepo `json:"items"`
		Message    string       `json:"message"`
	}
	if err := json.Unmarshal(b, &r); err != nil {
		return 0, nil, &ErrSchema{"github", "the page is not JSON: " + err.Error()}
	}
	if r.Total == nil {
		return 0, nil, &ErrSchema{"github", "the page has no total_count: " + r.Message}
	}
	for _, it := range r.Items {
		if it.ID == 0 || it.FullName == "" {
			return 0, nil, &ErrSchema{"github", "a repository of the page has no id or no full_name"}
		}
	}
	return *r.Total, r.Items, nil
}

// GitHubRow is the row of a repository. The topics of the row are its AI topics.
func GitHubRow(g GitHubRepo, ai map[string]bool, snap string) Row {
	row := Row{ID: g.FullName, Name: g.FullName, Kind: "software", URL: "https://github.com/" + g.FullName,
		SameAs:   map[string]string{"github": g.FullName, "github_id": strconv.FormatInt(g.ID, 10)},
		Fields:   map[string]any{},
		Links:    map[string][]string{"owner": {g.Owner.Login}},
		Snapshot: snap}
	if g.Language != "" {
		row.Fields["language"] = g.Language
	}
	if g.Licence != nil && g.Licence.SPDX != "" && g.Licence.SPDX != "NOASSERTION" {
		row.Fields["licence"] = g.Licence.SPDX
	}
	if g.Archived {
		row.Fields["archived"] = true
	}
	if g.Owner.Type != "" {
		row.Fields["owner_type"] = strings.ToLower(g.Owner.Type)
	}
	for _, t := range g.Topics {
		if ai[t] {
			row.Labels = append(row.Labels, t)
		}
	}
	sort.Strings(row.Labels)
	setFact(&row, "created", date(g.Created), "created_at")
	setSignal(&row, "stars", g.Stars)
	setSignal(&row, "forks", g.Forks)
	setSignal(&row, "pushed", date(g.Pushed))
	return row
}

// harvestGitHub reads, for each AI topic, the repositories with at least GitHubStars stars, the most starred first,
// splitting a range of stars when the search has more results than it gives. -limit keeps at most that many
// repositories of each topic.
func harvestGitHub(ctx context.Context, r *Run, o Options) ([]Row, error) {
	topics, err := gitHubTopics(r.Root)
	if err != nil {
		return nil, err
	}
	ai := map[string]bool{}
	for _, t := range topics {
		ai[t] = true
	}
	rows := map[string]Row{}
	for _, t := range topics {
		kept := 0
		ranges := []Stars{{GitHubStars, 0}}
		for len(ranges) > 0 && (o.Limit == 0 || kept < o.Limit) {
			s := ranges[0]
			ranges = ranges[1:]
			for page := 1; ; page++ {
				b, snap, err := r.Get(ctx, GitHubURL(t, s, page), 16<<20)
				if err != nil {
					return nil, fmt.Errorf("github topic %s stars %s: %w", t, s, err)
				}
				total, items, err := GitHubPage(b)
				if err != nil {
					return nil, err
				}
				if page == 1 && total > gitHubMaxResults {
					if lo, hi, ok := s.Split(); ok {
						// The top half first, so the most starred come first under -limit.
						ranges = append([]Stars{hi, lo}, ranges...)
						break
					}
				}
				for _, g := range items {
					if o.Limit > 0 && kept >= o.Limit {
						break
					}
					kept++
					if _, ok := rows[g.FullName]; !ok && !g.Fork {
						rows[g.FullName] = GitHubRow(g, ai, snap)
					}
				}
				if len(items) < 100 || page*100 >= min(total, gitHubMaxResults) || o.Limit > 0 && kept >= o.Limit {
					break
				}
			}
		}
	}
	out := make([]Row, 0, len(rows))
	for _, row := range rows {
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}
