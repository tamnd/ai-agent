// SPDX-License-Identifier: Apache-2.0

package harvest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/tamnd/ai-agent/cite"
)

// The register adapters read the registers of the content repository that name items (13.3, 5.7): the tables of
// contents of the canon works, the survey works and the courses. They make no request: the source is the repository,
// and each file read is stored as a snapshot like a fetched page, so a row cites the version of the register that
// named it. The catalog step (14.3, canon.py) reads the same registers with a model and gives the kinds; these rows
// give the names, the locators and the order, so the universe merge can count which works and courses name an item.
const (
	CanonFile    = "registers/canon.yaml"
	SurveysFile  = "registers/surveys.yaml"
	CoursesDir   = "topics/courses"
	maxRegister  = 32 << 20
	surveyPerSub = 5 // at most 5 surveys per sub-area (13.3)
)

// Local reads a file of the repository as a snapshot: it is stored in the raw store under the URL "repo:<rel>".
func (r *Run) Local(rel string) ([]byte, string, error) {
	b, err := os.ReadFile(filepath.Join(r.Root, filepath.FromSlash(rel)))
	if err != nil {
		return nil, "", err
	}
	if len(b) > maxRegister {
		return nil, "", fmt.Errorf("%s is larger than %d bytes", rel, maxRegister)
	}
	url := "repo:" + rel
	now := time.Now().UTC().Format(time.RFC3339)
	h, err := r.Store.Put(b, cite.Meta{URL: url, Fetched: now, ContentType: "application/yaml", Licence: r.Source.Class,
		Fetcher: r.adapterName()})
	if err != nil {
		return nil, "", err
	}
	r.use(Snapshot{URL: url, Fetched: now, Hash: "b3:" + h, Size: len(b), Class: r.Source.Class, Adapter: r.adapterName()})
	return b, "b3:" + h, nil
}

// canonSkip are the headings of a table of contents that name a part of the book, not an item.
var canonSkip = regexp.MustCompile(`(?i)^(introduction|overview|summary|conclusions?|concluding remarks|discussion|preface|` +
	`foreword|acknowledg(e)?ments|notation|exercises|problems|further reading|notes|bibliographical and historical notes|` +
	`bibliography|references|index|appendix|appendices|background|preliminaries|motivation|outlook|epilogue|prologue|` +
	`historical notes|chapter notes|review|what's next|looking ahead|the road ahead|examples?|definitions?|applications?|` +
	`extensions?|properties|terminology|the algorithm|(the )?basics|basic ideas?|basic concepts|computational issues|` +
	`interpretation|implementation|practical considerations|related work|other approaches)\.?$`)

// canonLead cuts the lead of a heading that frames an item: "Example: polynomial curve fitting" names the curve
// fitting, and "Case study: ..." the same.
var canonLead = regexp.MustCompile(`(?i)^(example|case study|application|worked example)s?:\s*`)

type canonReg struct {
	Version string `yaml:"version"`
	Works   []struct {
		ID    string `yaml:"id"`
		Title string `yaml:"title"`
		URL   string `yaml:"url"`
		TOC   []struct {
			Locator string `yaml:"locator"`
			Title   string `yaml:"title"`
			Page    int    `yaml:"page"`
			Note    string `yaml:"note"`
		} `yaml:"toc"`
	} `yaml:"works"`
}

// CanonRows gives a row for each named heading of the tables of contents of registers/canon.yaml. The id is
// "<work>:<locator>", the kind a guess (concept) that the catalog step corrects, and the depth of the locator is a
// field, so a chapter (1) and a subsection (1.2.3) can be told apart.
func CanonRows(b []byte, snap string) (rows []Row, skipped int, err error) {
	var reg canonReg
	if err := yaml.Unmarshal(b, &reg); err != nil {
		return nil, 0, &ErrSchema{"canon", CanonFile + " is not a map with works: " + err.Error()}
	}
	if len(reg.Works) == 0 {
		return nil, 0, &ErrSchema{"canon", CanonFile + " lists no work"}
	}
	for _, w := range reg.Works {
		if w.ID == "" {
			return nil, 0, &ErrSchema{"canon", CanonFile + ": a work has no id"}
		}
		for _, e := range w.TOC {
			name := clean(canonLead.ReplaceAllString(clean(e.Title), ""))
			if e.Locator == "" || name == "" || canonSkip.MatchString(name) {
				skipped++
				continue
			}
			row := Row{ID: w.ID + ":" + e.Locator, Name: name, Kind: "concept", URL: w.URL, Snapshot: snap,
				Labels: []string{w.ID + " " + w.Title}, Links: map[string][]string{"canon": {w.ID}},
				Fields: map[string]any{"work": w.ID, "locator": e.Locator, "depth": strings.Count(e.Locator, ".") + 1}}
			if e.Page > 0 {
				row.Fields["page"] = e.Page
			}
			if e.Note != "" {
				row.Fields["note"] = e.Note
			}
			if name != clean(e.Title) {
				row.Aliases = []string{clean(e.Title)}
			}
			rows = append(rows, row)
		}
	}
	return rows, skipped, nil
}

func harvestCanon(_ context.Context, r *Run, o Options) ([]Row, error) {
	b, snap, err := r.Local(CanonFile)
	if err != nil {
		return nil, fmt.Errorf("canon: %w", err)
	}
	rows, skipped, err := CanonRows(b, snap)
	if err != nil {
		return nil, err
	}
	r.Skip(skipped, "headings that name a part of a book (Introduction, Summary, Exercises, ...)")
	if len(rows) == 0 {
		r.Notes = append(r.Notes, "no canon work has a table of contents yet")
	}
	return keep(rows, o.Limit), nil
}

type surveyReg struct {
	Version string `yaml:"version"`
	Surveys []struct {
		ID      string   `yaml:"id"`
		Title   string   `yaml:"title"`
		Authors []string `yaml:"authors"`
		Date    string   `yaml:"date"`
		Area    string   `yaml:"area"`
		Arxiv   string   `yaml:"arxiv"`
		DOI     string   `yaml:"doi"`
		URL     string   `yaml:"url"`
		Reason  string   `yaml:"reason"`
	} `yaml:"surveys"`
}

var (
	arxivIDRE = regexp.MustCompile(`^\d{4}\.\d{4,5}$`)
	doiRE     = regexp.MustCompile(`^10\.\d{4,9}/\S+$`)
)

// SurveyRows gives a row of kind work for each survey of registers/surveys.yaml. A survey needs an id, a title, a
// date, a sub-area, an arXiv id or a DOI, and an editor's reason (13.3); the register is wrong, not the row, when one
// is missing, so the adapter stops. A sub-area with more than 5 surveys is a note.
func SurveyRows(b []byte, snap string, now time.Time) (rows []Row, notes []string, err error) {
	var reg surveyReg
	if err := yaml.Unmarshal(b, &reg); err != nil {
		return nil, nil, &ErrSchema{"surveys", SurveysFile + " is not a map with surveys: " + err.Error()}
	}
	if len(reg.Surveys) == 0 {
		return nil, nil, &ErrSchema{"surveys", SurveysFile + " lists no survey"}
	}
	per := map[string]int{}
	for _, s := range reg.Surveys {
		bad := func(what string) error {
			return &ErrSchema{"surveys", fmt.Sprintf("%s: the survey %q %s", SurveysFile, s.ID, what)}
		}
		switch {
		case s.ID == "" || s.Title == "":
			return nil, nil, bad("has no id or no title")
		case s.Area == "":
			return nil, nil, bad("has no sub-area")
		case s.Reason == "":
			return nil, nil, bad("has no reason (13.3)")
		case s.Arxiv == "" && s.DOI == "":
			return nil, nil, bad("has no arXiv id and no DOI")
		case s.Arxiv != "" && !arxivIDRE.MatchString(s.Arxiv):
			return nil, nil, bad("has an arXiv id that is not of the form 2303.18223")
		case s.DOI != "" && !doiRE.MatchString(s.DOI):
			return nil, nil, bad("has a DOI that does not start with 10.")
		}
		d, err := time.Parse("2006-01-02", s.Date)
		if err != nil {
			return nil, nil, bad("has no date YYYY-MM-DD")
		}
		per[s.Area]++
		row := Row{ID: s.ID, Name: clean(s.Title), Kind: "work", URL: s.URL, Snapshot: snap, SameAs: map[string]string{},
			Labels: []string{s.Area}, Links: map[string][]string{"area": {s.Area}},
			Fields: map[string]any{"area": s.Area, "date": s.Date, "reason": clean(s.Reason)}}
		if len(s.Authors) > 0 {
			row.Fields["authors"] = s.Authors
		}
		if s.Arxiv != "" {
			row.SameAs["arxiv"] = s.Arxiv
			if row.URL == "" {
				row.URL = "https://arxiv.org/abs/" + s.Arxiv
			}
		}
		if s.DOI != "" {
			row.SameAs["doi"] = strings.ToLower(s.DOI)
			if row.URL == "" {
				row.URL = "https://doi.org/" + s.DOI
			}
		}
		setFact(&row, "date", s.Date, "")
		// A survey older than three years is a candidate for replacement; the merge does not know the yearly count
		// of new works of the sub-area, so the age is a signal and the curator decides (13.3).
		setSignal(&row, "age_years", int(now.Sub(d).Hours()/24/365))
		rows = append(rows, row)
	}
	var areas []string
	for a, n := range per {
		if n > surveyPerSub {
			areas = append(areas, a+" ("+strconv.Itoa(n)+")")
		}
	}
	sort.Strings(areas)
	for _, a := range areas {
		notes = append(notes, "more than 5 surveys in the sub-area "+a+" (13.3)")
	}
	return rows, notes, nil
}

func harvestSurveys(_ context.Context, r *Run, o Options) ([]Row, error) {
	b, snap, err := r.Local(SurveysFile)
	if err != nil {
		return nil, fmt.Errorf("surveys: %w", err)
	}
	rows, notes, err := SurveyRows(b, snap, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	r.Notes = append(r.Notes, notes...)
	return keep(rows, o.Limit), nil
}

type courseFile struct {
	ID       string `yaml:"id"`
	Title    string `yaml:"title"`
	Basis    string `yaml:"basis"`
	Sections []struct {
		Heading string `yaml:"heading"`
		Items   []struct {
			Name string `yaml:"name"`
			Kind string `yaml:"kind"`
			Node string `yaml:"node"`
		} `yaml:"items"`
	} `yaml:"sections"`
}

var courseIDRE = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// CourseRows gives a row for each item of a course of topics/courses (5.7), in the course's order: the id is
// "<course>:<section>.<item>", counted from 1, and the order is a field. An item names a node by its slug when the
// node exists, and by its name and kind before it does.
func CourseRows(b []byte, snap, file string) ([]Row, error) {
	var c courseFile
	if err := yaml.Unmarshal(b, &c); err != nil {
		return nil, &ErrSchema{"courses", file + " is not a course: " + err.Error()}
	}
	want := strings.TrimSuffix(filepath.Base(file), ".yaml")
	switch {
	case c.ID != want || !courseIDRE.MatchString(c.ID):
		return nil, &ErrSchema{"courses", fmt.Sprintf("%s: the id %q is not the file name %q", file, c.ID, want)}
	case c.Title == "" || c.Basis == "":
		return nil, &ErrSchema{"courses", file + ": a course needs a title and a basis (5.7)"}
	case len(c.Sections) == 0:
		return nil, &ErrSchema{"courses", file + " has no section"}
	}
	var rows []Row
	order := 0
	for i, s := range c.Sections {
		if s.Heading == "" {
			return nil, &ErrSchema{"courses", fmt.Sprintf("%s: section %d has no heading", file, i+1)}
		}
		for j, it := range s.Items {
			name := clean(it.Name)
			if name == "" {
				return nil, &ErrSchema{"courses", fmt.Sprintf("%s: item %d.%d has no name", file, i+1, j+1)}
			}
			order++
			kind := it.Kind
			if kind == "" {
				kind = "concept"
			}
			loc := strconv.Itoa(i+1) + "." + strconv.Itoa(j+1)
			row := Row{ID: c.ID + ":" + loc, Name: name, Kind: kind, Snapshot: snap,
				Labels: []string{c.ID + " " + c.Title}, Links: map[string][]string{"course": {c.ID}},
				Fields: map[string]any{"course": c.ID, "section": clean(s.Heading), "locator": loc, "order": order}}
			if it.Node != "" {
				row.Fields["node"] = it.Node
			}
			rows = append(rows, row)
		}
	}
	return rows, nil
}

func harvestCourses(_ context.Context, r *Run, o Options) ([]Row, error) {
	files, err := filepath.Glob(filepath.Join(r.Root, CoursesDir, "*.yaml"))
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("courses: %s has no course", CoursesDir)
	}
	sort.Strings(files)
	var rows []Row
	for _, f := range files {
		rel := CoursesDir + "/" + filepath.Base(f)
		b, snap, err := r.Local(rel)
		if err != nil {
			return nil, fmt.Errorf("courses: %w", err)
		}
		rs, err := CourseRows(b, snap, rel)
		if err != nil {
			return nil, err
		}
		rows = append(rows, keep(rs, o.Limit)...)
	}
	return rows, nil
}
