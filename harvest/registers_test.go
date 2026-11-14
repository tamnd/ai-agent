// SPDX-License-Identifier: Apache-2.0

package harvest

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tamnd/ai-agent/cite"
)

func TestCanonRows(t *testing.T) {
	rows, skipped, err := CanonRows(fixture(t, "canon.yaml"), "b3:x")
	if err != nil {
		t.Fatal(err)
	}
	if skipped != 3 || len(rows) != 3 {
		t.Fatalf("got %d rows and %d skipped, want 3 and 3: %+v", len(rows), skipped, rows)
	}
	r := rows[0]
	if r.ID != "bishop-2006-prml:1.1" || r.Name != "Polynomial Curve Fitting" || r.Kind != "concept" ||
		len(r.Aliases) != 1 || r.Aliases[0] != "Example: Polynomial Curve Fitting" || r.Fields["page"] != 4 || r.Fields["depth"] != 2 {
		t.Errorf("the first row is %+v", r)
	}
	if rows[1].Name != "The Gaussian distribution" || rows[1].Fields["depth"] != 3 {
		t.Errorf("the second row is %+v", rows[1])
	}
	if d := rows[2]; d.ID != "goodfellow-2016-deep-learning:7.12" || d.Links["canon"][0] != "goodfellow-2016-deep-learning" || d.Fields["page"] != nil {
		t.Errorf("the dropout row is %+v", d)
	}
	var e *ErrSchema
	if _, _, err := CanonRows([]byte("version: x\n"), ""); !errors.As(err, &e) {
		t.Errorf("a register with no works: %v", err)
	}
}

func TestSurveyRows(t *testing.T) {
	now := time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC)
	rows, notes, err := SurveyRows(fixture(t, "surveys.yaml"), "b3:x", now)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || len(notes) != 0 {
		t.Fatalf("got %d rows and notes %v", len(rows), notes)
	}
	if r := rows[0]; r.Kind != "work" || r.SameAs["arxiv"] != "2303.18223" || r.URL != "https://arxiv.org/abs/2303.18223" ||
		r.Signals["age_years"] != 3 || r.Fields["area"] != "large-language-models" {
		t.Errorf("the first row is %+v", r)
	}
	if r := rows[1]; r.SameAs["doi"] != "10.1145/3560815" || r.URL != "https://doi.org/10.1145/3560815" {
		t.Errorf("the second row is %+v", r)
	}
	for _, bad := range []string{
		"surveys: [{id: a, title: T, date: 2020-01-01, area: x, arxiv: '2001.00001'}]",            // no reason
		"surveys: [{id: a, title: T, date: 2020-01-01, area: x, reason: r}]",                      // no id
		"surveys: [{id: a, title: T, date: 2020, area: x, arxiv: '2001.00001', reason: r}]",       // a bad date
		"surveys: [{id: a, title: T, date: 2020-01-01, area: x, arxiv: 'cs/0101001', reason: r}]", // an old arXiv id
	} {
		var e *ErrSchema
		if _, _, err := SurveyRows([]byte(bad), "", now); !errors.As(err, &e) {
			t.Errorf("%s: %v", bad, err)
		}
	}
	six := "surveys:\n"
	for i := range 6 {
		six += "  - {id: s" + string(rune('a'+i)) + ", title: T, date: 2024-01-01, area: agents, arxiv: '2401.0000" + string(rune('1'+i)) + "', reason: r}\n"
	}
	if _, notes, err := SurveyRows([]byte(six), "", now); err != nil || len(notes) != 1 || !strings.Contains(notes[0], "agents (6)") {
		t.Errorf("six surveys in one sub-area: %v %v", notes, err)
	}
}

func TestCourseRows(t *testing.T) {
	rows, err := CourseRows(fixture(t, "course.yaml"), "b3:x", "topics/courses/intro-ai.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("got %d rows", len(rows))
	}
	if r := rows[0]; r.ID != "intro-ai:1.1" || r.Fields["node"] != "rational-agent" || r.Fields["order"] != 1 || r.Kind != "concept" {
		t.Errorf("the first row is %+v", r)
	}
	if r := rows[2]; r.ID != "intro-ai:2.1" || r.Kind != "method" || r.Fields["section"] != "Search" || r.Fields["order"] != 3 {
		t.Errorf("the last row is %+v", r)
	}
	var e *ErrSchema
	if _, err := CourseRows(fixture(t, "course.yaml"), "", "topics/courses/other.yaml"); !errors.As(err, &e) {
		t.Errorf("a course whose id is not its file name: %v", err)
	}
}

// TestRegisterRun runs the three register adapters on a repository and checks that each file read is a snapshot.
func TestRegisterRun(t *testing.T) {
	root := t.TempDir()
	for rel, name := range map[string]string{CanonFile: "canon.yaml", SurveysFile: "surveys.yaml", CoursesDir + "/intro-ai.yaml": "course.yaml"} {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, fixture(t, name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for id, want := range map[string]int{"canon": 3, "surveys": 2, "courses": 3} {
		r := &Run{Root: root, Source: *SourceByID(id), Store: cite.Store{Dir: filepath.Join(root, ".raw")}, Date: "2026-11-15", Version: "v0.0.1"}
		rows, err := Adapters[id].Run(context.Background(), r, Options{})
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		rep, err := r.Finish(rows, Options{})
		if err != nil {
			t.Fatal(err)
		}
		if rep.Rows != want || len(rep.Snapshots) != 1 || rep.Requests != 0 {
			t.Errorf("%s: %+v", id, rep)
		}
		m, err := ReadManifest(root, id)
		if err != nil || len(m) != 1 || !strings.HasPrefix(m[0].URL, "repo:") {
			t.Errorf("%s: the manifest is %+v, %v", id, m, err)
		}
	}
	empty := &Run{Root: t.TempDir(), Source: *SourceByID("courses"), Store: cite.Store{Dir: t.TempDir()}}
	if _, err := harvestCourses(context.Background(), empty, Options{}); err == nil {
		t.Error("a repository with no course gave no error")
	}
}
