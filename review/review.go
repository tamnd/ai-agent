// SPDX-License-Identifier: Apache-2.0

// Package review reads and checks the review records of spec 15.5: reviews/<tag>/<part>.yaml, a person's verdict on
// the exact bytes of the files of one part of a node, against a versioned checklist (15.7), by a reviewer of the
// register registers/reviewers.yaml (14.11) who is not an author of those files (4.8).
package review

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/zeebo/blake3"
	"go.yaml.in/yaml/v3"

	"github.com/tamnd/ai-agent/node"
)

// Dir is the directory of the review records, under the root of the content repository.
const Dir = "reviews"

// Register is the file of the reviewers, under the root of the content repository.
const Register = "registers/reviewers.yaml"

// Item is the answer to one question of the checklist. Check is the number of the question.
type Item struct {
	Check  string `yaml:"check"`
	Result string `yaml:"result"` // pass | fail | n/a
	Note   string `yaml:"note,omitempty"`
}

// Attest is the reviewer's statement that they read a source that the build cannot read, a paywalled one (7.5).
type Attest struct {
	Claim    string `yaml:"claim"`
	Work     string `yaml:"work"`
	Read     bool   `yaml:"read"`
	Supports bool   `yaml:"supports"`
}

// Record is a review record. Hashes maps each reviewed file to "b3:<hex>" of its bytes; the path is relative to the
// node directory, or to the root of the repository when it starts with "/" (a Lean module).
type Record struct {
	Tag          string            `yaml:"tag"`
	Part         string            `yaml:"part"`
	Hashes       map[string]string `yaml:"hashes"`
	Checklist    string            `yaml:"checklist"`
	Reviewer     string            `yaml:"reviewer"`
	Verdict      string            `yaml:"verdict"` // pass | changes
	Date         string            `yaml:"date"`
	Items        []Item            `yaml:"items"`
	Attest       []Attest          `yaml:"attest,omitempty"`
	Independence []string          `yaml:"independence,omitempty"`
	Interests    string            `yaml:"interests"`
	Basis        string            `yaml:"basis,omitempty"` // the record that a diff review renews
}

// Reviewer is a line of the register.
type Reviewer struct {
	ID     string   `yaml:"id"` // github:<login>
	Name   string   `yaml:"name"`
	Skills []string `yaml:"skills"`
	Key    string   `yaml:"key"` // the public key that signs the reviewer's commits
	Org    string   `yaml:"org,omitempty"`
}

// Hash is "b3:" and the BLAKE3 hash of the file, in hex.
func Hash(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	s := blake3.Sum256(b)
	return "b3:" + hex.EncodeToString(s[:]), nil
}

// Key is the name of a file in the hashes of a record of n: its path from the node directory, or "/" and its path from
// root when it is outside the node directory.
func Key(root string, n *node.Node, file string) string {
	rel := file
	ar, err1 := filepath.Abs(root)
	af, err2 := filepath.Abs(file)
	if err1 == nil && err2 == nil {
		if r, err := filepath.Rel(ar, af); err == nil {
			rel = r
		}
	}
	rel = filepath.ToSlash(rel)
	if n.Rel != "" && strings.HasPrefix(rel, n.Rel+"/") {
		return strings.TrimPrefix(rel, n.Rel+"/")
	}
	return "/" + rel
}

// Path is the file of a key of the hashes of a record of n.
func Path(root string, n *node.Node, key string) string {
	if strings.HasPrefix(key, "/") {
		return filepath.Join(root, filepath.FromSlash(key[1:]))
	}
	return filepath.Join(root, filepath.FromSlash(n.Rel), filepath.FromSlash(key))
}

// File is the path of the record of a part of the node with the tag.
func File(root, tag, part string) string {
	return filepath.Join(root, Dir, tag, filepath.FromSlash(part)+".yaml")
}

// Load reads a record. A missing record gives an error that wraps fs.ErrNotExist.
func Load(root, tag, part string) (*Record, error) {
	b, err := os.ReadFile(File(root, tag, part))
	if err != nil {
		return nil, err
	}
	var r Record
	if err := yaml.Unmarshal(b, &r); err != nil {
		return nil, fmt.Errorf("the review record does not parse: %w", err)
	}
	return &r, nil
}

// LoadRegister reads the reviewers. A missing register gives none.
func LoadRegister(root string) ([]Reviewer, error) {
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(Register)))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var rs []Reviewer
	if err := yaml.Unmarshal(b, &rs); err != nil {
		return nil, fmt.Errorf("%s does not parse: %w", Register, err)
	}
	return rs, nil
}

// TopArea is the first directory of the node under nodes/.
func TopArea(n *node.Node) string {
	a, _, _ := strings.Cut(strings.TrimPrefix(n.Rel, "nodes/"), "/")
	return a
}

// Skills are the skills of the register that can review the part of n; one of them is enough, except for an
// experiment, which needs both (15.5).
func Skills(n *node.Node, part string) (anyOf, allOf []string) {
	domain := "domain:" + TopArea(n)
	switch {
	case part == "text", part == "positions", strings.HasPrefix(part, "proof-"):
		return []string{domain}, nil
	case strings.HasPrefix(part, "impl-"):
		return []string{"code:" + strings.TrimPrefix(part, "impl-")}, nil
	case strings.HasPrefix(part, "exp/"), part == "experiment":
		return nil, []string{"exp", domain}
	case part == "lean-spec", part == "lean-statement":
		return []string{"lean"}, nil
	case part == "scene":
		return []string{"viz", domain}, nil
	case part == "facts":
		return []string{"data", "law:*"}, nil
	case part == "contract":
		return []string{"code:*", domain}, nil
	}
	return []string{domain}, nil
}

func has(skills []string, want string) bool {
	for _, s := range skills {
		if s == want || strings.HasSuffix(want, ":*") && strings.HasPrefix(s, strings.TrimSuffix(want, "*")) {
			return true
		}
	}
	return false
}

// Authors are the logins of the authors of the files of the node with the tag, from prov/<tag>.jsonl; the file "*"
// is every file. An agent run is not a login.
func Authors(root, tag string) (map[string][]string, error) {
	f, err := os.Open(filepath.Join(root, "prov", tag+".jsonl"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	out := map[string][]string{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		var l struct{ Author, File string }
		if json.Unmarshal(sc.Bytes(), &l) != nil || l.Author == "" {
			continue
		}
		out[l.Author] = append(out[l.Author], l.File)
	}
	return out, sc.Err()
}

var questionRE = regexp.MustCompile(`^([0-9]+)\.\s`)

// Questions are the numbers of the questions of a checklist file: its lines "1. …", "2. …".
func Questions(path string) ([]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var qs []string
	for _, l := range strings.Split(string(b), "\n") {
		if m := questionRE.FindStringSubmatch(l); m != nil {
			qs = append(qs, m[1])
		}
	}
	return qs, nil
}

// ChecklistFile is the file of a checklist version, "text/method/v1" → checklists/text/method/v1.md.
func ChecklistFile(root, checklist string) string {
	return filepath.Join(root, "checklists", filepath.FromSlash(checklist)+".md")
}

var dateRE = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}$`)

// Check tells whether the record of the part of n counts for file (15.5), and why not. It does not check that the
// commit of the record is signed by the reviewer's key; CI does, as it has the history.
func Check(root string, n *node.Node, part, file string) (bool, string) {
	if n.Tag == "" {
		return false, "the node has no tag yet"
	}
	r, err := Load(root, n.Tag, part)
	if errors.Is(err, fs.ErrNotExist) {
		return false, "no review record " + filepath.ToSlash(filepath.Join(Dir, n.Tag, part)) + ".yaml"
	}
	if err != nil {
		return false, err.Error()
	}
	if r.Tag != n.Tag || r.Part != part {
		return false, fmt.Sprintf("the review record is for %s %s, not %s %s", r.Tag, r.Part, n.Tag, part)
	}
	if r.Verdict != "pass" {
		return false, "the review does not pass: its verdict is " + strconv.Quote(r.Verdict)
	}
	if !dateRE.MatchString(r.Date) {
		return false, "the review has no date"
	}
	// The files.
	key := Key(root, n, file)
	if _, ok := r.Hashes[key]; !ok {
		return false, "the review does not hash " + key
	}
	keys := make([]string, 0, len(r.Hashes))
	for k := range r.Hashes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		h, err := Hash(Path(root, n, k))
		if err != nil {
			return false, "the reviewed file " + k + " is gone"
		}
		if h != r.Hashes[k] {
			return false, "the review is for an older version of " + k
		}
	}
	// The checklist.
	if r.Checklist == "" {
		return false, "the review names no checklist"
	}
	qs, err := Questions(ChecklistFile(root, r.Checklist))
	if err != nil {
		return false, "the checklist " + r.Checklist + " does not exist"
	}
	got := map[string]string{}
	for _, it := range r.Items {
		got[it.Check] = it.Result
		if it.Result != "pass" && it.Result != "n/a" {
			return false, fmt.Sprintf("item %s of the checklist is %q, not pass or n/a", it.Check, it.Result)
		}
	}
	for _, q := range qs {
		if _, ok := got[q]; !ok {
			return false, "the review does not answer item " + q + " of " + r.Checklist
		}
	}
	// The reviewer.
	reg, err := LoadRegister(root)
	if err != nil {
		return false, err.Error()
	}
	i := slices.IndexFunc(reg, func(x Reviewer) bool { return x.ID == r.Reviewer })
	if i < 0 {
		return false, "the reviewer " + strconv.Quote(r.Reviewer) + " is not in " + Register
	}
	rv := reg[i]
	if rv.Key == "" {
		return false, "the reviewer " + r.Reviewer + " has no signing key in " + Register
	}
	anyOf, allOf := Skills(n, part)
	if len(anyOf) > 0 && !slices.ContainsFunc(anyOf, func(s string) bool { return has(rv.Skills, s) }) {
		return false, "the reviewer " + r.Reviewer + " has none of the skills " + strings.Join(anyOf, ", ") + " for " + part
	}
	for _, s := range allOf {
		if !has(rv.Skills, s) {
			return false, "the reviewer " + r.Reviewer + " lacks the skill " + s + " for " + part
		}
	}
	if r.Interests == "" {
		return false, "the review states no interests (\"none\" if there are none)"
	}
	// An author does not review their own files.
	authors, err := Authors(root, n.Tag)
	if err != nil {
		return false, err.Error()
	}
	login := strings.TrimPrefix(r.Reviewer, "github:")
	for _, f := range authors[login] {
		if f == "*" {
			return false, "the reviewer " + r.Reviewer + " is an author of the node (prov/" + n.Tag + ".jsonl)"
		}
		if _, ok := r.Hashes[strings.TrimPrefix(f, n.Rel+"/")]; ok {
			return false, "the reviewer " + r.Reviewer + " is an author of " + f + " (prov/" + n.Tag + ".jsonl)"
		}
	}
	return true, ""
}
