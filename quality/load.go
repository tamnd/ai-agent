// SPDX-License-Identifier: Apache-2.0

package quality

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/tamnd/ai-agent/audit"
)

// LoadAudits reads the verdicts of the audits of each item from the state files of the writer.
func LoadAudits(dir string) (map[string][]string, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	out := map[string][]string{}
	for _, p := range files {
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		var st struct {
			Slug   string `json:"slug"`
			Audits []struct {
				Verdict string `json:"verdict"`
			} `json:"audits"`
		}
		if err := json.Unmarshal(b, &st); err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		if len(st.Audits) == 0 {
			continue
		}
		if st.Slug == "" {
			st.Slug = strings.TrimSuffix(filepath.Base(p), ".json")
		}
		for _, a := range st.Audits {
			out[st.Slug] = append(out[st.Slug], a.Verdict)
		}
	}
	return out, nil
}

// LoadLinks reads the link check of an audit report; a missing report, or one without the check, gives nil.
func LoadLinks(p string) (*audit.Links, error) {
	b, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var r audit.Report
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, fmt.Errorf("%s: %w", p, err)
	}
	return r.Links, nil
}

// LoadWaivers reads registers/waivers.yaml, a list of waivers or a map with the list under waivers. A missing register
// gives nil.
func LoadWaivers(root string) ([]Waiver, error) {
	p := filepath.Join(root, "registers", "waivers.yaml")
	b, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	ws := []Waiver{}
	if err := yaml.Unmarshal(b, &ws); err == nil {
		return ws, nil
	}
	var m struct {
		Waivers []Waiver `yaml:"waivers"`
	}
	if err := yaml.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("%s: %w", p, err)
	}
	if m.Waivers == nil {
		m.Waivers = []Waiver{}
	}
	return m.Waivers, nil
}
