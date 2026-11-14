// SPDX-License-Identifier: Apache-2.0

package harvest

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// ACLTree lists the files of data/xml of the ACL Anthology's repository, one file a collection. It is one request of
// the GitHub API, which allows 60 an hour without a token.
const ACLTree = "https://api.github.com/repos/acl-org/acl-anthology/git/trees/master:data/xml"

// ACLRaw is where the files of data/xml are.
const ACLRaw = "https://raw.githubusercontent.com/acl-org/acl-anthology/master/data/xml/"

// ACLVenuesFile lists the venues of the Anthology to read; without it the adapter reads ACLVenues.
const ACLVenuesFile = "crosswalk/acl.yaml"

// ACLVenues are the main venues of the Anthology: the conferences of the ACL and its chapters, Findings, the two
// journals, CoNLL and COLING. Workshops are not read.
var ACLVenues = []string{"acl", "emnlp", "naacl", "eacl", "aacl", "findings", "tacl", "cl", "conll", "coling"}

// aclOldPrefix is the venue of the letter of an old collection id (P19 is ACL 2019); the volumes of such a file are
// still read by their own venues, since a file of a conference also has its workshops.
var aclOldPrefix = map[string]string{"P": "acl", "D": "emnlp", "N": "naacl", "E": "eacl", "Q": "tacl", "J": "cl",
	"K": "conll", "C": "coling"}

var (
	aclNewFileRE = regexp.MustCompile(`^(\d{4})\.([a-z0-9-]+)\.xml$`)
	aclOldFileRE = regexp.MustCompile(`^([A-Z])(\d\d)\.xml$`)
	venueNameRE  = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,39}$`)
	// mainAwardRE is an award of S-W3: a best paper, an outstanding paper or a test-of-time award of any track, not a
	// runner-up, an area chair's highlight or a people's choice.
	mainAwardRE = regexp.MustCompile(`(?i)\b(best|outstanding|test[- ]of[- ]time)\b`)
	runnerUpRE  = regexp.MustCompile(`(?i)runner[- ]up|honou?rable mention`)
)

// MainAward tells if an award of a venue is one of S-W3 (18.5).
func MainAward(name string) bool {
	return mainAwardRE.MatchString(name) && !runnerUpRE.MatchString(name)
}

// aclVenues reads the venues file, or gives the default venues.
func aclVenues(root string) (map[string]bool, error) {
	list := ACLVenues
	b, err := os.ReadFile(filepath.Join(root, ACLVenuesFile))
	switch {
	case os.IsNotExist(err):
	case err != nil:
		return nil, err
	default:
		var doc struct {
			Venues []string `yaml:"venues"`
		}
		if err := yaml.Unmarshal(b, &doc); err != nil {
			return nil, &ErrSchema{"acl", ACLVenuesFile + " is not a map with venues: " + err.Error()}
		}
		for _, v := range doc.Venues {
			if !venueNameRE.MatchString(v) {
				return nil, &ErrSchema{"acl", ACLVenuesFile + ": " + strconv.Quote(v) + " is not a venue id"}
			}
		}
		if len(doc.Venues) == 0 {
			return nil, &ErrSchema{"acl", ACLVenuesFile + " names no venue"}
		}
		list = doc.Venues
	}
	out := map[string]bool{}
	for _, v := range list {
		out[v] = true
	}
	return out, nil
}

// ACLFile is a file of data/xml that may hold volumes of a venue, with the year of its collection.
type ACLFile struct {
	Name string
	Year int
}

// ACLFiles reads the tree of data/xml and gives the files of the venues, the newest first.
func ACLFiles(b []byte, venues map[string]bool) ([]ACLFile, error) {
	var t struct {
		Truncated bool `json:"truncated"`
		Tree      []struct {
			Path string `json:"path"`
			Type string `json:"type"`
		} `json:"tree"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(b, &t); err != nil {
		return nil, &ErrSchema{"acl", "the tree is not JSON: " + err.Error()}
	}
	if t.Tree == nil {
		return nil, &ErrSchema{"acl", "the answer has no tree: " + t.Message}
	}
	if t.Truncated {
		return nil, &ErrSchema{"acl", "the tree of data/xml is truncated"}
	}
	var out []ACLFile
	for _, e := range t.Tree {
		if e.Type != "blob" {
			continue
		}
		if m := aclNewFileRE.FindStringSubmatch(e.Path); m != nil && venues[m[2]] {
			y, _ := strconv.Atoi(m[1])
			out = append(out, ACLFile{e.Path, y})
		} else if m := aclOldFileRE.FindStringSubmatch(e.Path); m != nil && venues[aclOldPrefix[m[1]]] {
			y, _ := strconv.Atoi(m[2])
			if y >= 50 {
				y += 1900
			} else {
				y += 2000
			}
			out = append(out, ACLFile{e.Path, y})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Year != out[j].Year {
			return out[i].Year > out[j].Year
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

// aclText is an element whose text is all its character data, without the markup inside it (<fixed-case>, <i>,
// <tex-math>).
type aclText string

func (t *aclText) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var b strings.Builder
	depth := 1
	for depth > 0 {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch x := tok.(type) {
		case xml.StartElement:
			depth++
		case xml.EndElement:
			depth--
		case xml.CharData:
			b.Write(x)
		}
	}
	*t = aclText(clean(b.String()))
	return nil
}

type aclPerson struct {
	ID    string  `xml:"id,attr"`
	First aclText `xml:"first"`
	Last  aclText `xml:"last"`
}

func (p aclPerson) name() string { return clean(string(p.First) + " " + string(p.Last)) }

// ACLPaper is a paper of a volume.
type ACLPaper struct {
	ID      string      `xml:"id,attr"`
	Title   aclText     `xml:"title"`
	Authors []aclPerson `xml:"author"`
	DOI     string      `xml:"doi"`
	Awards  []struct {
		Name string `xml:"name,attr"`
		Text string `xml:",chardata"`
	} `xml:"award"`
}

// ACLVolume is a volume of a collection.
type ACLVolume struct {
	ID   string `xml:"id,attr"`
	Type string `xml:"type,attr"`
	Meta struct {
		Booktitle aclText  `xml:"booktitle"`
		Year      string   `xml:"year"`
		Month     string   `xml:"month"`
		Venues    []string `xml:"venue"`
	} `xml:"meta"`
	Papers []ACLPaper `xml:"paper"`
}

// ACLCollection reads a file of data/xml.
func ACLCollection(b []byte) (string, []ACLVolume, error) {
	var c struct {
		ID      string      `xml:"id,attr"`
		Volumes []ACLVolume `xml:"volume"`
	}
	if err := xml.NewDecoder(bytes.NewReader(b)).Decode(&c); err != nil {
		return "", nil, &ErrSchema{"acl", "the file is not a collection: " + err.Error()}
	}
	if c.ID == "" {
		return "", nil, &ErrSchema{"acl", "the collection has no id"}
	}
	return c.ID, c.Volumes, nil
}

// ACLID is the Anthology id of a paper: "2023.acl-long.1" in a new collection, "P19-1078" in an old one, whose
// volume and paper numbers fill four digits.
func ACLID(collection, volume, paper string) string {
	if aclOldFileRE.MatchString(collection + ".xml") {
		pad := 4 - len(volume)
		for len(paper) < pad {
			paper = "0" + paper
		}
		return collection + "-" + volume + paper
	}
	return collection + "-" + volume + "." + paper
}

// ACLRows are the rows of the papers of the volumes of the venues. The venue of a row is the first venue of its
// volume that is read; its labels are its awards, and its signal awards counts those of S-W3.
func ACLRows(collection string, vols []ACLVolume, venues map[string]bool, snap string) []Row {
	var out []Row
	for _, v := range vols {
		venue := ""
		for _, x := range v.Meta.Venues {
			if venues[strings.TrimSpace(x)] {
				venue = strings.TrimSpace(x)
				break
			}
		}
		if venue == "" {
			continue
		}
		for _, p := range v.Papers {
			name := string(p.Title)
			if p.ID == "" || p.ID == "0" || name == "" {
				continue
			}
			id := ACLID(collection, v.ID, p.ID)
			row := Row{ID: id, Name: name, Kind: "work", URL: "https://aclanthology.org/" + id + "/",
				SameAs: map[string]string{"acl": id}, Snapshot: snap,
				Fields: map[string]any{"venue": venue}}
			if d := strings.ToLower(strings.TrimSpace(p.DOI)); d != "" {
				row.SameAs["doi"] = d
			}
			if bt := string(v.Meta.Booktitle); bt != "" {
				row.Fields["volume"] = bt
			}
			if y := strings.TrimSpace(v.Meta.Year); y != "" {
				row.Fields["year"] = y
			}
			var authors, ids []string
			for _, a := range p.Authors {
				if n := a.name(); n != "" {
					authors = append(authors, n)
				}
				if a.ID != "" {
					ids = append(ids, a.ID)
				}
			}
			if len(authors) > 0 {
				row.Fields["authors"] = authors
			}
			if len(ids) > 0 {
				row.Links = map[string][]string{"author": ids}
			}
			for _, a := range p.Awards {
				n := clean(a.Name)
				if n == "" {
					n = clean(a.Text)
				}
				if n != "" {
					row.Labels = append(row.Labels, n)
				}
			}
			sort.Strings(row.Labels)
			main := 0
			for _, l := range row.Labels {
				if MainAward(l) {
					main++
				}
			}
			if main > 0 {
				setSignal(&row, "awards", main)
			}
			out = append(out, row)
		}
	}
	return out
}

// harvestACL reads the collections of the venues of the ACL Anthology from its repository, the newest first: a row
// for each paper, with its DOI and its awards. -since keeps the collections of that year and later; -limit keeps at
// most that many papers.
func harvestACL(ctx context.Context, r *Run, o Options) ([]Row, error) {
	venues, err := aclVenues(r.Root)
	if err != nil {
		return nil, err
	}
	b, _, err := r.Get(ctx, ACLTree, 8<<20)
	if err != nil {
		return nil, fmt.Errorf("acl tree: %w", err)
	}
	files, err := ACLFiles(b, venues)
	if err != nil {
		return nil, err
	}
	since := 0
	if len(o.Since) >= 4 {
		since, _ = strconv.Atoi(o.Since[:4])
	}
	var out []Row
	for _, f := range files {
		if f.Year < since {
			continue
		}
		b, snap, err := r.Get(ctx, ACLRaw+f.Name, 64<<20)
		if err != nil {
			return nil, fmt.Errorf("acl %s: %w", f.Name, err)
		}
		id, vols, err := ACLCollection(b)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f.Name, err)
		}
		out = append(out, ACLRows(id, vols, venues, snap)...)
		if o.Limit > 0 && len(out) >= o.Limit {
			break
		}
	}
	out = keep(out, o.Limit)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}
