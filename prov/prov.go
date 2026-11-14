// SPDX-License-Identifier: Apache-2.0

// Package prov reads, checks and appends the provenance of the nodes of tamnd/ai (spec 04.8). Each node has a log,
// prov/<tag>.jsonl, with a line for each change to a file of its directory: the date, the file, the operation, the hash
// of the file after the change, the author (a GitHub login, or an agent run with its models and prompt version), the
// auditor, the reviewer and the sources that the change used. The file "*" is the whole node, for the line that
// created it. The page shows the log as its "History" (16.4), and review reads it to keep authors from reviewing their
// own files (15.5).
package prov

import (
	"bufio"
	"bytes"
	"crypto/sha256"
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
	"strings"
	"time"
)

// Dir is the directory of the logs in the content repository.
const Dir = "prov"

// Whole is the file of a line about the whole node.
const Whole = "*"

// The operations of a line.
const (
	Create     = "create"     // the node or the file is new
	Edit       = "edit"       // the file has a new content
	Remove     = "remove"     // the file is gone
	Confirm    = "confirm"    // a person read the file as it is and approved it
	Quarantine = "quarantine" // a person hid the file until a person clears it (15.7)
)

// Ops are the operations, in the order above.
var Ops = []string{Create, Edit, Remove, Confirm, Quarantine}

// Author is who made a change: a person by a GitHub login, or an agent by its role, models, prompt version and run.
type Author struct {
	Login         string   // a GitHub login, for a person
	Agent         string   // the role of the agent, for example writer
	Models        []string // the models of the run, backend:model
	Harness       string   // the program that wrote the line, for example ai-agent/work/prov.py
	Prompt        string   // the prompt version, role/v<n>
	Run           string   // the id of the agent run
	ModelsVersion string   // the version of registers/models.yaml
}

type agentWire struct {
	Agent         string   `json:"agent"`
	Models        []string `json:"models,omitempty"`
	Harness       string   `json:"harness,omitempty"`
	Prompt        string   `json:"prompt,omitempty"`
	Run           string   `json:"run,omitempty"`
	ModelsVersion string   `json:"models_version,omitempty"`
}

// MarshalJSON writes a person as the login, and an agent as an object.
func (a Author) MarshalJSON() ([]byte, error) {
	if a.Agent == "" {
		return json.Marshal(a.Login)
	}
	return json.Marshal(agentWire{a.Agent, a.Models, a.Harness, a.Prompt, a.Run, a.ModelsVersion})
}

// UnmarshalJSON reads a login or an agent object.
func (a *Author) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) > 0 && b[0] == '"' {
		*a = Author{}
		return json.Unmarshal(b, &a.Login)
	}
	if string(b) == "null" {
		*a = Author{}
		return nil
	}
	var w agentWire
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	*a = Author{Agent: w.Agent, Models: w.Models, Harness: w.Harness, Prompt: w.Prompt, Run: w.Run, ModelsVersion: w.ModelsVersion}
	return nil
}

// String is the login, or the agent with its models and run.
func (a Author) String() string {
	if a.Agent == "" {
		return a.Login
	}
	s := "agent " + a.Agent
	if a.Prompt != "" {
		s += " (" + a.Prompt + ")"
	}
	if len(a.Models) > 0 {
		s += " with " + strings.Join(a.Models, ", ")
	}
	if a.Run != "" {
		s += ", run " + a.Run
	}
	return s
}

// Empty tells if the author names no one.
func (a Author) Empty() bool { return a.Login == "" && a.Agent == "" }

// Line is one line of a log.
type Line struct {
	Date     string   `json:"date"` // YYYY-MM-DD
	File     string   `json:"file"` // the path in the node directory, with slashes, or "*"
	Op       string   `json:"op"`
	Hash     string   `json:"hash,omitempty"` // sha256:<hex> of the file after the change
	Author   Author   `json:"author"`
	Audit    string   `json:"audit,omitempty"`    // the auditor, backend:model, for a change of an agent
	Reviewer *string  `json:"reviewer,omitempty"` // the GitHub login of the reviewer, or null
	Note     string   `json:"note,omitempty"`
	Sources  []string `json:"sources"` // the source ids that the change used
	Num      int      `json:"-"`       // the line number in the log
}

// Path is the log of a tag.
func Path(root, tag string) string { return filepath.Join(root, Dir, tag+".jsonl") }

// Read reads the log of a tag. It gives nil and no error when the node has no log.
func Read(root, tag string) ([]Line, error) {
	b, err := os.ReadFile(Path(root, tag))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return Parse(b, Dir+"/"+tag+".jsonl")
}

// Parse reads the lines of a log; name is the log in errors.
func Parse(b []byte, name string) ([]Line, error) {
	var out []Line
	sc := bufio.NewScanner(bytes.NewReader(b))
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for n := 1; sc.Scan(); n++ {
		t := bytes.TrimSpace(sc.Bytes())
		if len(t) == 0 {
			continue
		}
		var l Line
		if err := json.Unmarshal(t, &l); err != nil {
			return nil, fmt.Errorf("%s:%d: %w", name, n, err)
		}
		l.Num = n
		out = append(out, l)
	}
	return out, sc.Err()
}

// Append adds lines to the log of a tag, making the log when there is none.
func Append(root, tag string, ls []Line) error {
	if len(ls) == 0 {
		return nil
	}
	var buf bytes.Buffer
	for _, l := range ls {
		if l.Sources == nil {
			l.Sources = []string{}
		}
		b, err := json.Marshal(l)
		if err != nil {
			return err
		}
		buf.Write(b)
		buf.WriteByte('\n')
	}
	p := Path(root, tag)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	if old, err := os.ReadFile(p); err == nil && len(old) > 0 && old[len(old)-1] != '\n' {
		buf = *bytes.NewBuffer(append([]byte{'\n'}, buf.Bytes()...))
	}
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644) //nolint:gosec // the provenance logs are public files of the content repo
	if err != nil {
		return err
	}
	if _, err := f.Write(buf.Bytes()); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// Hash is the hash of a content as a line writes it.
func Hash(b []byte) string {
	h := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(h[:])
}

// Files are the hashes of the files of a node directory, by their path with slashes. Python caches are not files of
// the node.
func Files(dir string) (map[string]string, error) {
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "__pycache__" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(p, ".pyc") || d.Name() == ".DS_Store" {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(rel)] = Hash(b)
		return nil
	})
	return out, err
}

// Latest is the last line of each file of a log, "*" included.
func Latest(ls []Line) map[string]Line {
	out := map[string]Line{}
	for _, l := range ls {
		out[l.File] = l
	}
	return out
}

// State is how a file of a node stands against its log.
type State struct {
	File  string `json:"file"`
	Hash  string `json:"hash,omitempty"` // the hash now, or "" for a file that is gone
	State string `json:"state"`          // recorded, changed, unrecorded, created or removed
	Last  *Line  `json:"last,omitempty"` // the last line of the file
}

// The states of a file.
const (
	Recorded   = "recorded"   // the last line of the file has its hash
	Changed    = "changed"    // the file changed after its last line
	Unrecorded = "unrecorded" // no line names the file, and no line made the node
	Created    = "created"    // no line names the file, but a line "*" made the node, with no hashes
	Removed    = "removed"    // a line names the file, which is gone, and no line records the removal
)

// States compares the files of a node directory with its log, in the order of the paths.
func States(files map[string]string, ls []Line) []State {
	last := Latest(ls)
	_, whole := last[Whole]
	var out []State
	for f, h := range files {
		s := State{File: f, Hash: h}
		l, ok := last[f]
		switch {
		case ok && l.Hash == h && l.Op != Remove:
			s.State = Recorded
		case ok:
			s.State = Changed
		case whole:
			s.State = Created
		default:
			s.State = Unrecorded
		}
		if ok {
			s.Last = &l
		}
		out = append(out, s)
	}
	for f, l := range last {
		if _, ok := files[f]; ok || f == Whole || l.Op == Remove {
			continue
		}
		l := l
		out = append(out, State{File: f, State: Removed, Last: &l})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].File < out[j].File })
	return out
}

// Changes are the lines that bring a log up to date with the files: an edit for each file that changed or that no
// line names, and a remove for each file that is gone. tmpl gives the date, the author and the rest of each line.
func Changes(files map[string]string, ls []Line, tmpl Line) []Line {
	var out []Line
	for _, s := range States(files, ls) {
		if s.State == Recorded {
			continue
		}
		l := tmpl
		l.File, l.Hash, l.Op, l.Num = s.File, s.Hash, Edit, 0
		if s.State == Removed {
			l.Op = Remove
		}
		l.Sources = slices.Clone(tmpl.Sources)
		out = append(out, l)
	}
	return out
}

var (
	dateRE  = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	hashRE  = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	loginRE = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,38})$`)
)

// Problem is a line of a log that does not follow 04.8.
type Problem struct {
	Line int    `json:"line"`
	Msg  string `json:"msg"`
}

// Check gives the problems of the lines of a log: a missing or bad date, file, operation, hash or author.
func Check(ls []Line) []Problem {
	var out []Problem
	add := func(l Line, f string, a ...any) { out = append(out, Problem{l.Num, fmt.Sprintf(f, a...)}) }
	for _, l := range ls {
		if !dateRE.MatchString(l.Date) {
			add(l, "the date %q is not YYYY-MM-DD", l.Date)
		} else if _, err := time.Parse("2006-01-02", l.Date); err != nil {
			add(l, "the date %q is not a day", l.Date)
		}
		switch {
		case l.File == "":
			add(l, "the line names no file")
		case l.File != Whole && (strings.HasPrefix(l.File, "/") || strings.Contains(l.File, "\\") || slices.Contains(strings.Split(l.File, "/"), "..")):
			add(l, "the file %q is not a path inside the node directory", l.File)
		}
		if !slices.Contains(Ops, l.Op) {
			add(l, "the operation %q is not one of %s", l.Op, strings.Join(Ops, ", "))
		}
		if l.Hash != "" && !hashRE.MatchString(l.Hash) {
			add(l, "the hash %q is not sha256:<64 hex digits>", l.Hash)
		}
		if l.Hash == "" && (l.Op == Edit || l.Op == Confirm) {
			add(l, "an %s line has no hash", l.Op)
		}
		switch {
		case l.Author.Empty():
			add(l, "the line has no author")
		case l.Author.Agent != "" && (len(l.Author.Models) == 0 || l.Author.Run == ""):
			add(l, "the agent %s has no models or no run id", l.Author.Agent)
		case l.Author.Agent == "" && !loginRE.MatchString(l.Author.Login):
			add(l, "the author %q is not a GitHub login", l.Author.Login)
		}
		if l.Author.Agent != "" && l.Audit == "" && l.Op == Edit {
			add(l, "the change of an agent has no auditor")
		}
		if (l.Op == Confirm || l.Op == Quarantine) && l.Author.Agent != "" {
			add(l, "only a person can %s a file", l.Op)
		}
	}
	return out
}
