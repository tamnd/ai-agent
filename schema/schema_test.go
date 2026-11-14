// SPDX-License-Identifier: Apache-2.0

package schema

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"testing"
)

var update = flag.Bool("update", false, "write the schema files from the generator")

// Golden compares a schema with its file, or writes the file with -update. The site package uses it for record.v1.json.
func Golden(t *testing.T, file string, s map[string]any, upd bool) {
	t.Helper()
	b, err := Encode(s)
	if err != nil {
		t.Fatal(err)
	}
	if upd {
		if err := os.WriteFile(file, b, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	old, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(old, b) {
		t.Errorf("%s is not what the generator makes; run go test ./schema ./site -update", file)
	}
}

// GoldenBytes compares a generated file with its copy, or writes the copy with -update.
func GoldenBytes(t *testing.T, file string, b []byte, upd bool) {
	t.Helper()
	if upd {
		if err := os.WriteFile(file, b, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	old, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(old, b) {
		t.Errorf("%s is not what the generator makes; run go test ./schema ./site -update", file)
	}
}

func TestFiles(t *testing.T) {
	fy, err := FactsYAML()
	if err != nil {
		t.Fatal(err)
	}
	GoldenBytes(t, "facts.v1.yaml", fy, *update)
	GoldenBytes(t, "edges.md", EdgesMarkdown(), *update)
	for file, v := range map[string]struct {
		pkg string
		v   any
	}{"levels.v1.yaml": {"packages status and coverage", Levels()}, "targets.v1.yaml": {"package coverage", Targets()}} {
		b, err := encodeYAML(v.pkg, v.v)
		if err != nil {
			t.Fatal(err)
		}
		GoldenBytes(t, file, b, *update)
	}
	Golden(t, "node.v1.json", Node(), *update)
	Golden(t, "claims.v1.json", Claims(), *update)
	Golden(t, "card.v1.json", Card(), *update)
	Golden(t, "experiment.v1.json", Experiment(), *update)
	Golden(t, "expect.v1.json", Expect(), *update)
}

func TestNode(t *testing.T) {
	s := Node()
	props := s["properties"].(map[string]any)
	for _, k := range []string{"tag", "kind", "slug", "name", "area", "level", "summary", "solves", "same_as"} {
		if _, ok := props[k]; !ok {
			t.Errorf("node.v1 has no %s", k)
		}
	}
	if _, ok := props["fields"]; ok {
		t.Error("node.v1 has the Go field Fields, which is not a key of the file")
	}
	req, _ := s["required"].([]string)
	for _, k := range []string{"tag", "kind", "slug", "name", "area", "level", "summary"} {
		found := false
		for _, r := range req {
			found = found || r == k
		}
		if !found {
			t.Errorf("%s is not required", k)
		}
	}
	b, _ := json.Marshal(s)
	for _, want := range []string{`"const":"argument"`, `"premises":{}`, `"unevaluatedProperties":false`} {
		if !bytes.Contains(b, []byte(want)) {
			t.Errorf("node.v1 lacks %s", want)
		}
	}
}

func TestClaims(t *testing.T) {
	b, _ := json.Marshal(Claims())
	if bytes.Contains(b, []byte(`"labels"`)) {
		t.Error("claims.v1 has the computed labels")
	}
	for _, want := range []string{`"normative-reported"`, `"meta-analysis"`, `"against"`} {
		if !bytes.Contains(b, []byte(want)) {
			t.Errorf("claims.v1 lacks %s", want)
		}
	}
}

func TestCard(t *testing.T) {
	b, _ := json.Marshal(Card())
	for _, want := range []string{`"required":["v","src"]`, `"const":"result"`, `"checkpoint"`, `"provision"`, `"unevaluatedProperties":{"$ref":"#/$defs/field"}`} {
		if !bytes.Contains(b, []byte(want)) {
			t.Errorf("card.v1 lacks %s", want)
		}
	}
}
