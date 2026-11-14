// SPDX-License-Identifier: Apache-2.0

package prov

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The lines that aik mint and work/prov.py write.
const logText = `{"author":"tamnd","date":"2026-11-13","file":"*","op":"create","sources":[]}
{"date":"2026-11-14","file":"node.md","op":"edit","hash":"sha256:` + "%s" + `","author":{"agent":"writer","models":["claude:claude-opus-5-5"],"harness":"ai-agent/work/prov.py","prompt":"writer/v1","run":"s-20261114-191859-19015","models_version":"m-2026.11"},"audit":"codex:gpt-6-sol","reviewer":null,"note":"gold page","sources":[]}
{"date":"2026-11-14","file":"old.md","op":"edit","hash":"sha256:` + "%s" + `","author":"tamnd","sources":["arxiv:1207.0580"]}
`

func TestLog(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "nodes", "a", "b", "dropout")
	if err := os.MkdirAll(filepath.Join(dir, "impl", "__pycache__"), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(p, s string) {
		if err := os.WriteFile(filepath.Join(dir, p), []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("node.md", "body")
	write("claims.yaml", "claims")
	write("impl/__pycache__/x.pyc", "cache")
	nodeHash := strings.TrimPrefix(Hash([]byte("body")), "sha256:")
	text := strings.Replace(strings.Replace(logText, "%s", nodeHash, 1), "%s", strings.Repeat("0", 64), 1)
	if err := os.MkdirAll(filepath.Join(root, Dir), 0o755); err != nil {
		t.Fatal(err)
	}
	// A log whose last line has no newline, as an editor may leave it.
	if err := os.WriteFile(Path(root, "H0HPFC"), []byte(strings.TrimSuffix(text, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	ls, err := Read(root, "H0HPFC")
	if err != nil || len(ls) != 3 {
		t.Fatalf("%d lines, %v", len(ls), err)
	}
	if ls[0].Author.Login != "tamnd" || ls[1].Author.Agent != "writer" || ls[1].Author.Run != "s-20261114-191859-19015" ||
		ls[1].Reviewer != nil || ls[2].Num != 3 {
		t.Errorf("the lines %+v", ls)
	}
	if ps := Check(ls); len(ps) != 0 {
		t.Errorf("the lines of mint and prov.py have problems: %v", ps)
	}

	files, err := Files(dir)
	if err != nil || len(files) != 2 {
		t.Fatalf("files %v %v (no Python cache)", files, err)
	}
	got := map[string]string{}
	for _, s := range States(files, ls) {
		got[s.File] = s.State
	}
	want := map[string]string{"node.md": Recorded, "claims.yaml": Created, "old.md": Removed}
	if len(got) != len(want) {
		t.Errorf("states %v", got)
	}
	for f, s := range want {
		if got[f] != s {
			t.Errorf("%s is %s, want %s", f, got[f], s)
		}
	}
	if s := States(files, nil); s[0].State != Unrecorded {
		t.Errorf("a file with no log is %s", s[0].State)
	}

	r := "a-reviewer"
	add := Changes(files, ls, Line{Date: "2026-11-15", Author: Author{Login: "tamnd"}, Reviewer: &r, Sources: []string{"doi:10.1"}})
	if len(add) != 2 || add[0].File != "claims.yaml" || add[0].Op != Edit || add[0].Hash != Hash([]byte("claims")) ||
		add[1].File != "old.md" || add[1].Op != Remove || add[1].Hash != "" {
		t.Fatalf("changes %+v", add)
	}
	if ps := Check(add); len(ps) != 0 {
		t.Errorf("the new lines have problems: %v", ps)
	}
	if err := Append(root, "H0HPFC", add); err != nil {
		t.Fatal(err)
	}
	ls, err = Read(root, "H0HPFC")
	if err != nil || len(ls) != 5 {
		t.Fatalf("after append: %d lines, %v", len(ls), err)
	}
	if again := Changes(files, ls, Line{Date: "2026-11-15", Author: Author{Login: "tamnd"}}); len(again) != 0 {
		t.Errorf("a second record adds %+v", again)
	}
	b, _ := os.ReadFile(Path(root, "H0HPFC"))
	last := strings.Split(strings.TrimSpace(string(b)), "\n")[4]
	if last != `{"date":"2026-11-15","file":"old.md","op":"remove","author":"tamnd","reviewer":"a-reviewer","sources":["doi:10.1"]}` {
		t.Errorf("the line written %s", last)
	}
	if l, err := Read(root, "NOPE"); l != nil || err != nil {
		t.Errorf("a missing log: %v %v", l, err)
	}
}

func TestCheck(t *testing.T) {
	var ls []Line
	for _, s := range []string{
		`{"date":"2026-13-01","file":"*","op":"create","author":"tamnd","sources":[]}`,
		`{"date":"2026-11-01","file":"../x","op":"edit","hash":"sha256:abc","author":"tamnd","sources":[]}`,
		`{"date":"2026-11-01","file":"node.md","op":"rewrite","author":null,"sources":[]}`,
		`{"date":"2026-11-01","file":"node.md","op":"edit","author":{"agent":"writer"},"sources":[]}`,
		`{"date":"2026-11-01","file":"node.md","op":"quarantine","author":{"agent":"writer","models":["m"],"run":"r"},"sources":[]}`,
		`{"date":"2026-11-01","file":"node.md","op":"create","author":"not a login","sources":[]}`,
	} {
		var l Line
		if err := json.Unmarshal([]byte(s), &l); err != nil {
			t.Fatal(err)
		}
		l.Num = len(ls) + 1
		ls = append(ls, l)
	}
	ps := Check(ls)
	var msgs []string
	for _, p := range ps {
		msgs = append(msgs, p.Msg)
	}
	all := strings.Join(msgs, "\n")
	for _, want := range []string{
		`the date "2026-13-01" is not a day`,
		`the file "../x" is not a path inside the node directory`,
		`the hash "sha256:abc" is not sha256:<64 hex digits>`,
		`the operation "rewrite" is not one of`,
		`the line has no author`,
		`the agent writer has no models or no run id`,
		`an edit line has no hash`,
		`the change of an agent has no auditor`,
		`only a person can quarantine a file`,
		`the author "not a login" is not a GitHub login`,
	} {
		if !strings.Contains(all, want) {
			t.Errorf("no problem %q in:\n%s", want, all)
		}
	}
	a := Author{Agent: "writer", Models: []string{"claude:x"}, Prompt: "writer/v1", Run: "r1"}
	if a.String() != "agent writer (writer/v1) with claude:x, run r1" || (Author{Login: "tamnd"}).String() != "tamnd" {
		t.Errorf("String %q", a.String())
	}
}
