// SPDX-License-Identifier: Apache-2.0

// Package universe builds the universe register of spec 18.3 from the harvested inventories, and writes the cards
// of its card outcomes (aik universe merge, aik card sync).
package universe

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Dir is the directory of the register: one shard per top area, and _.jsonl for the items with no area yet.
const Dir = "registers/universe"

// Unplaced is the shard of the items that have no area yet.
const Unplaced = "_"

// Item is one row of the universe register.
type Item struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Kind      string            `json:"kind"`
	Area      string            `json:"area,omitempty"`
	Aliases   []string          `json:"aliases,omitempty"`
	SameAs    map[string]string `json:"same_as,omitempty"`
	Sources   []string          `json:"sources,omitempty"`
	Signals   map[string]any    `json:"signals,omitempty"`
	Outcome   string            `json:"outcome"`
	Target    string            `json:"target,omitempty"`
	Reason    string            `json:"reason,omitempty"`
	DecidedBy string            `json:"decided_by,omitempty"`
	Decided   string            `json:"decided,omitempty"`
	Core      bool              `json:"core,omitempty"`
	// Added is the date of the merge that made the item; a pending item ages from it (18.7).
	Added string `json:"added,omitempty"`
}

// Outcomes are the outcomes of 18.3.
var Outcomes = []string{"node", "card", "alias", "part", "bridge", "row-only", "out-of-scope", "merged", "pending"}

// Shard is the shard of an item: the top area of its area, or _.
func (it *Item) Shard() string {
	if top, _, _ := strings.Cut(it.Area, "/"); top != "" {
		return top
	}
	return Unplaced
}

// Read reads every shard of the register.
func Read(root string) ([]*Item, error) {
	paths, err := filepath.Glob(filepath.Join(root, Dir, "*.jsonl"))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	var out []*Item
	for _, p := range paths {
		f, err := os.Open(p)
		if err != nil {
			return nil, err
		}
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
		for n := 1; sc.Scan(); n++ {
			b := bytes.TrimSpace(sc.Bytes())
			if len(b) == 0 {
				continue
			}
			it := &Item{}
			if err := json.Unmarshal(b, it); err != nil {
				_ = f.Close()
				return nil, fmt.Errorf("%s:%d: %w", p, n, err)
			}
			out = append(out, it)
		}
		err = sc.Err()
		_ = f.Close()
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// Write writes the register: each shard sorted by id. A shard that has no items any more is removed.
func Write(root string, items []*Item) error {
	shards := map[string][]*Item{}
	for _, it := range items {
		shards[it.Shard()] = append(shards[it.Shard()], it)
	}
	dir := filepath.Join(root, Dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	old, _ := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	for _, p := range old {
		if _, ok := shards[strings.TrimSuffix(filepath.Base(p), ".jsonl")]; !ok {
			if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return err
			}
		}
	}
	for name, its := range shards {
		sort.Slice(its, func(i, j int) bool { return its[i].ID < its[j].ID })
		var buf bytes.Buffer
		e := json.NewEncoder(&buf)
		e.SetEscapeHTML(false)
		for _, it := range its {
			if err := e.Encode(it); err != nil {
				return err
			}
		}
		if err := os.WriteFile(filepath.Join(dir, name+".jsonl"), buf.Bytes(), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// Rule is a scope rule of registers/scope.yaml (18.5).
type Rule struct {
	ID         string         `yaml:"id"`
	Kind       string         `yaml:"kind"`
	Outcome    string         `yaml:"outcome"`
	Rule       string         `yaml:"rule"`
	Source     string         `yaml:"source"`
	Thresholds map[string]any `yaml:"thresholds"`
	Default    string         `yaml:"default"`
}

// ScopeFile is the register of the scope rules.
const ScopeFile = "registers/scope.yaml"

// ReadRules reads the scope rules. A tree with no register has no rules.
func ReadRules(root string) ([]Rule, error) {
	b, err := os.ReadFile(filepath.Join(root, ScopeFile))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var rules []Rule
	if err := yaml.Unmarshal(b, &rules); err != nil {
		return nil, fmt.Errorf("%s: %w", ScopeFile, err)
	}
	return rules, nil
}

// Unconditional tells if the rule decides from the source alone: "each model of Epoch AI is a card". A rule with
// thresholds decides by Meets, and a rule with neither a source nor thresholds needs judgement.
func (r Rule) Unconditional() bool {
	return r.Source != "" && len(r.Thresholds) == 0 && r.Outcome != ""
}

// Meets tells if the signals meet one of the thresholds of the rule: S-M2 makes a card of a model with 10,000
// downloads or 100 likes. A signal that is missing or not a number meets nothing.
func (r Rule) Meets(signals map[string]any) bool {
	for k, t := range r.Thresholds {
		want, ok1 := number(t)
		got, ok2 := number(signals[k])
		if ok1 && ok2 && got >= want {
			return true
		}
	}
	return false
}

func number(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case float64:
		return n, true
	case uint64:
		return float64(n), true
	}
	return 0, false
}

// URI is the target of an item that names a tag: ai://5TQH2W, or ai://NEW-dropout before aik mint (04.2).
func URI(tag string) string { return "ai://" + tag }

// TagOf is the tag of a target: ai://5TQH2W#part and 5TQH2W are both 5TQH2W.
func TagOf(target string) string {
	t := strings.TrimPrefix(target, "ai://")
	t, _, _ = strings.Cut(t, "#")
	t, _, _ = strings.Cut(t, "@")
	return t
}

// Retarget points the items at the new tags after aik mint: an item whose target is an old tag takes the new one. A
// tree with no register has nothing to do.
func Retarget(root string, tags map[string]string) error {
	if len(tags) == 0 {
		return nil
	}
	items, err := Read(root)
	if err != nil {
		return err
	}
	changed := false
	for _, it := range items {
		if t, ok := tags[TagOf(it.Target)]; ok && it.Target != "" {
			it.Target, changed = URI(t), true
		}
	}
	if !changed {
		return nil
	}
	return Write(root, items)
}
