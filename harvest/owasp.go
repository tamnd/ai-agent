// SPDX-License-Identifier: Apache-2.0

package harvest

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// OWASPRepo is the repository of the OWASP Top 10 for LLM Applications, whose LICENSE.md is CC BY-SA 4.0.
const OWASPRepo = "OWASP/www-project-top-10-for-large-language-model-applications"

// OWASPRelease is one release of the list: the directory of its entries, the year of its ids and its same_as key. Each
// release has its own key, so that an entry kept under one name across releases (LLM01:2023 and LLM01:2025 Prompt
// Injection, LLM06:2023 and LLM02:2025 Sensitive Information Disclosure) joins into one risk in the universe, while two
// entries of one release never do. An entry renamed between releases (Supply-Chain Vulnerabilities, Supply Chain) is
// not joined: whether it is the same risk is the curator's call.
type OWASPRelease struct {
	Dir, Year, Version, Key string
}

// OWASPReleases are the releases read, newest first. Version 1.1 (2023) is the last of the first list; the earlier
// drafts under Archive are not releases.
var OWASPReleases = []OWASPRelease{
	{Dir: "2_0_vulns", Year: "2025", Version: "2025", Key: "owasp_llm_2025"},
	{Dir: "Archive/1_1_vulns", Year: "2023", Version: "1.1", Key: "owasp_llm_2023"},
}

// OWASPListURL is the GitHub contents API listing of a release directory.
func OWASPListURL(dir string) string {
	return "https://api.github.com/repos/" + OWASPRepo + "/contents/" + strings.ReplaceAll(dir, " ", "%20")
}

// OWASPRawURL is the raw file of an entry.
func OWASPRawURL(p string) string {
	return "https://raw.githubusercontent.com/" + OWASPRepo + "/main/" + p
}

func owaspBlobURL(p string) string { return "https://github.com/" + OWASPRepo + "/blob/main/" + p }

var (
	owaspFileRE    = regexp.MustCompile(`^LLM(\d\d)_[A-Za-z]+\.md$`)
	owaspHeadingRE = regexp.MustCompile(`^#{1,3}\s+LLM(\d\d)(?::(\d{4}))?:?\s+(.+?)\s*#*\s*$`)
)

// OWASPFiles reads a contents API listing and returns the entry files, LLM01 to LLM10 in order; the preface
// (LLM00), the template and the subdirectories are not entries.
func OWASPFiles(b []byte) ([]string, error) {
	var list []struct {
		Name string `json:"name"`
		Path string `json:"path"`
		Type string `json:"type"`
	}
	if err := json.Unmarshal(b, &list); err != nil {
		return nil, &ErrSchema{Source: "owasp-llm", What: "the directory listing is not a JSON array: " + err.Error()}
	}
	var out []string
	for _, f := range list {
		m := owaspFileRE.FindStringSubmatch(f.Name)
		if f.Type != "file" || m == nil || m[1] == "00" {
			continue
		}
		out = append(out, f.Path)
	}
	sort.Strings(out)
	if len(out) == 0 {
		return nil, &ErrSchema{Source: "owasp-llm", What: "the directory lists no LLMnn_*.md entry"}
	}
	return out, nil
}

// OWASPRow reads the heading of an entry: "## LLM01:2025 Prompt Injection" in 2025, "## LLM01: Prompt Injection" in
// 1.1. Only the id and the name are kept, never the text (spec 13: names and ids only).
func OWASPRow(rel OWASPRelease, p string, b []byte, snap string) (Row, error) {
	file := owaspFileRE.FindStringSubmatch(p[strings.LastIndex(p, "/")+1:])
	if file == nil {
		return Row{}, &ErrSchema{Source: "owasp-llm", What: p + " is not an LLMnn_*.md entry"}
	}
	for _, line := range strings.SplitN(string(b), "\n", 20) {
		m := owaspHeadingRE.FindStringSubmatch(strings.TrimSpace(strings.TrimPrefix(line, "\ufeff")))
		if m == nil {
			continue
		}
		if m[1] != file[1] || (m[2] != "" && m[2] != rel.Year) {
			return Row{}, &ErrSchema{Source: "owasp-llm", What: fmt.Sprintf("%s is headed LLM%s:%s", p, m[1], m[2])}
		}
		id := "LLM" + m[1] + ":" + rel.Year
		rank, _ := strconv.Atoi(m[1])
		r := Row{
			Source: "owasp-llm", ID: id, Name: clean(m[3]), Kind: "risk", URL: owaspBlobURL(p), Snapshot: snap,
			SameAs: map[string]string{rel.Key: id},
		}
		loc := "the heading of " + p
		setFact(&r, "release", rel.Version, loc)
		setFact(&r, "rank", rank, loc)
		return r, nil
	}
	return Row{}, &ErrSchema{Source: "owasp-llm", What: p + " has no LLMnn heading"}
}

func harvestOWASP(ctx context.Context, r *Run, o Options) ([]Row, error) {
	var rows []Row
	for _, rel := range OWASPReleases {
		b, _, err := r.Get(ctx, OWASPListURL(rel.Dir), 1<<20)
		if err != nil {
			return nil, fmt.Errorf("owasp-llm: %w", err)
		}
		files, err := OWASPFiles(b)
		if err != nil {
			return nil, err
		}
		if len(files) != 10 {
			r.Notes = append(r.Notes, fmt.Sprintf("release %s has %d entries, not 10", rel.Version, len(files)))
		}
		for _, p := range files {
			b, snap, err := r.Get(ctx, OWASPRawURL(p), 1<<20)
			if err != nil {
				return nil, fmt.Errorf("owasp-llm: %w", err)
			}
			row, err := OWASPRow(rel, p, b, snap)
			if err != nil {
				return nil, err
			}
			rows = append(rows, row)
		}
	}
	return keep(rows, o.Limit), nil
}
