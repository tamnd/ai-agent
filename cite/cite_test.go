// SPDX-License-Identifier: Apache-2.0

package cite

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/tamnd/ai-agent/claims"
)

func TestParseLocator(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want []Ref
	}{
		{"Section 3.2", []Ref{{Kind: "section", From: "3.2"}}},
		{"Sections 2 to 7", []Ref{{Kind: "section", From: "2", To: "7"}}},
		{"Sections 3 and 4", []Ref{{Kind: "section", From: "3"}, {Kind: "section", From: "4"}}},
		{"Section 6 and Tables 2 to 9", []Ref{{Kind: "section", From: "6"}, {Kind: "table", From: "2", To: "9"}}},
		{"Section 7.5, Figure 11", []Ref{{Kind: "section", From: "7.5"}, {Kind: "figure", From: "11"}}},
		{"Appendix A.4", []Ref{{Kind: "appendix", From: "A.4"}}},
		{"Eq. 4; Eqs. 5 to 6", []Ref{{Kind: "equation", From: "4"}, {Kind: "equation", From: "5", To: "6"}}},
		{`Table 2, row "Transformer (big)", column "EN-DE BLEU"`, []Ref{{Kind: "table", From: "2", Row: "Transformer (big)", Column: "EN-DE BLEU"}}},
		{`Section 5.4 "Residual Dropout"`, []Ref{{Kind: "section", From: "5.4", Title: "Residual Dropout"}}},
		{"Article 50(4)", []Ref{{Kind: "article", From: "50(4)"}}},
		{"Annex III", []Ref{{Kind: "annex", From: "III"}}},
		{"pp. 12 to 14", []Ref{{Kind: "page", From: "12", To: "14"}}},
		{"the abstract", []Ref{{Kind: "abstract"}}},
		{"the title page", []Ref{{Kind: "title-page"}}},
		{"Theorem 2.1", []Ref{{Kind: "theorem", From: "2.1"}}},
	} {
		got, err := ParseLocator(tc.in)
		if err != nil {
			t.Errorf("ParseLocator(%q): %v", tc.in, err)
			continue
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("ParseLocator(%q) = %+v, want %+v", tc.in, got, tc.want)
		}
	}
}

func TestParseLocatorRejects(t *testing.T) {
	for _, in := range []string{
		"",
		"NeurIPS 26, the sections on generalized linear models",
		"arXiv 1409.2329, Section 3",
		`row "x"`,
		`Section 3, row "x"`,
		"Section three",
		"Figure 1.2",
	} {
		if refs, err := ParseLocator(in); err == nil {
			t.Errorf("ParseLocator(%q) = %+v, want an error", in, refs)
		}
	}
}

func TestRefStringRoundTrip(t *testing.T) {
	for _, in := range []string{"Section 3.2", "Sections 2 to 7", "Appendix A.4", "Article 50(4)", "the abstract"} {
		refs, err := ParseLocator(in)
		if err != nil {
			t.Fatal(err)
		}
		back, err := ParseLocator(refs[0].String())
		if err != nil || !reflect.DeepEqual(back, refs) {
			t.Errorf("%q: String gives %q, which reads as %+v (%v)", in, refs[0].String(), back, err)
		}
	}
}

// paper is a small text in the form pdftotext gives: pages at form feeds, a printed page number at the foot, a table
// with numbers at the start of its lines, a caption, an equation, and a list of references.
const paper = "A Paper About Dropout\nAbstract\nWe study dropout on networks.\n\n" +
	"1 Introduction\nDropout drops units at random during training.\n\n" +
	"2 Method\nEach unit is kept with probability p.\n" +
	"y = f(Wx) / p   (1)\n\n" +
	"2.1 Scaling\nAt test time the weights are scaled by p.\n1\n\f" +
	"3 Results\nThe error falls from 1.6% to 1.25% on MNIST.\n" +
	"Table 1: Errors on MNIST.\n" +
	"4 1.60\n5 1.25\n\n" +
	"Appendix A. Practical Guide\nA.1 Rates\nUse a rate of 0.5 for hidden units.\n\n" +
	"References\n1 D. E. Rumelhart, G. E. Hinton, Learning representations (1986).\n" +
	"2 Gradient Methods Revisited, 2001.\n2\n"

func TestParse(t *testing.T) {
	d := Parse(paper)
	if len(d.Pages) != 2 {
		t.Fatalf("pages = %v, want 2", d.Pages)
	}
	var heads []string
	for _, h := range d.Heads {
		heads = append(heads, h.Num+" "+h.Title)
	}
	want := []string{"1 Introduction", "2 Method", "2.1 Scaling", "3 Results", "A Practical Guide", "A.1 Rates"}
	if !reflect.DeepEqual(heads, want) {
		t.Errorf("heads = %q, want %q", heads, want)
	}
	if d.Abstract == nil || !strings.Contains(d.Text[d.Abstract.Start:d.Abstract.End], "We study dropout") ||
		strings.Contains(d.Text[d.Abstract.Start:d.Abstract.End], "Introduction") {
		t.Errorf("abstract = %+v", d.Abstract)
	}
	method, _ := d.head("2")
	if txt := d.Text[method.Start:method.End]; !strings.Contains(txt, "scaled by p") || strings.Contains(txt, "Results") {
		t.Errorf("section 2 holds %q", txt)
	}
	var marks []string
	for _, m := range d.Marks {
		marks = append(marks, m.Kind+" "+m.Num)
	}
	for _, w := range []string{"equation 1", "table 1"} {
		if !strings.Contains(strings.Join(marks, ","), w) {
			t.Errorf("marks = %q, want %s", marks, w)
		}
	}
}

func TestParseHeadingOnNextLine(t *testing.T) {
	d := Parse("1\nIntroduction\nText.\n\n1.1\n0.1 0.2 0.3\nAblations\nMore text.\n\n7\nA line of text.\n")
	var heads []string
	for _, h := range d.Heads {
		heads = append(heads, h.Num+" "+h.Title)
	}
	// A row of numbers between 1.1 and its title is skipped; 7 is not a heading, as its number does not follow and
	// the line after it is a sentence.
	if want := []string{"1 Introduction", "1.1 Ablations"}; !reflect.DeepEqual(heads, want) {
		t.Errorf("heads = %q, want %q", heads, want)
	}
}

func TestResolve(t *testing.T) {
	d := Parse(paper)
	for _, tc := range []struct {
		loc, holds, lacks string
	}{
		{"Section 2.1", "scaled by p", "Results"},
		{"Sections 1 to 2", "drops units", "Results"},
		{"Appendix A.1", "rate of 0.5", "References"},
		{"Table 1", "1.25", ""},
		{"Eq. 1", "kept with probability", ""},
		{"p. 2", "MNIST", "Introduction"},
		{"the abstract", "We study", "drops units"},
		{`Section 2 "Method"`, "Method", ""},
	} {
		refs, err := ParseLocator(tc.loc)
		if err != nil {
			t.Fatal(err)
		}
		spans, fails := Resolve(d, refs)
		if len(fails) > 0 {
			t.Errorf("%s: %v", tc.loc, fails)
			continue
		}
		var txt string
		for _, s := range spans {
			txt += d.Text[s.Start:s.End]
		}
		if !strings.Contains(txt, tc.holds) || (tc.lacks != "" && strings.Contains(txt, tc.lacks)) {
			t.Errorf("%s resolves to %q", tc.loc, txt)
		}
	}
	for _, loc := range []string{"Section 9", "Table 4", `Section 2 "Results"`, "Appendix B", "Theorem 1"} {
		refs, _ := ParseLocator(loc)
		if _, fails := Resolve(d, refs); len(fails) == 0 {
			t.Errorf("%s resolves, but the text has no such place", loc)
		}
	}
}

func TestResolveProvision(t *testing.T) {
	d := Parse("Article 49\nOld text.\nArticle 50\nTransparency\n1. Providers shall inform.\n4. Deployers shall disclose.\nArticle 51\nNext.\n")
	refs, _ := ParseLocator("Article 50(4)")
	spans, fails := Resolve(d, refs)
	if len(fails) > 0 {
		t.Fatal(fails)
	}
	if txt := d.Text[spans[0].Start:spans[0].End]; !strings.Contains(txt, "Deployers") || strings.Contains(txt, "Providers") || strings.Contains(txt, "Next") {
		t.Errorf("Article 50(4) = %q", txt)
	}
}

func TestNumbers(t *testing.T) {
	got := map[string]float64{}
	for _, n := range Numbers("It has 65 million weights, 4.5% error, 1,024 units, 28.4 BLEU, $x^2$ and 3.5M tokens; 5 m away.") {
		got[n.Text] = n.Value
	}
	for text, v := range map[string]float64{"65 million": 65e6, "4.5%": 0.045, "1,024": 1024, "28.4": 28.4, "3.5M": 3.5e6} {
		if g, ok := got[text]; !ok || g < v*0.999999 || g > v*1.000001 {
			t.Errorf("Numbers: %s = %v (%v), want %v", text, g, ok, v)
		}
	}
	if _, ok := got["2"]; ok {
		t.Error("Numbers reads the exponent of $x^2$")
	}
	if _, ok := got["5 m"]; ok {
		t.Error(`Numbers reads "5 m" as five million`)
	}
}

func TestFound(t *testing.T) {
	src := Numbers("errors of 4.08 and 1.25, 213 thousand steps")
	for _, tc := range []struct {
		claim string
		ok    bool
	}{
		{"4.1", true},    // the page rounds the source
		{"4.08", true},   // as exact as the source
		{"4.080", false}, // more exact than the source
		{"1.3", true},    // 1.25 rounds half to even or up; math.Round gives 1.3
		{"4.2", false},   // a different number
		{"213 thousand", true},
	} {
		n := Numbers(tc.claim)
		if len(n) != 1 {
			t.Fatalf("Numbers(%q) = %v", tc.claim, n)
		}
		if got := Found(n[0], src); got != tc.ok {
			t.Errorf("Found(%s) = %v, want %v", tc.claim, got, tc.ok)
		}
	}
}

func TestTerms(t *testing.T) {
	got := Terms("Srivastava et al. recommend a dropout rate of 0.5 for hidden units in the paper.", "Nitish Srivastava")
	want := []string{"recommend", "dropout", "rate", "hidden", "units"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Terms = %q, want %q", got, want)
	}
	if got := Terms("The $\\frac{1}{p}$ factor scales units, unit and units."); !reflect.DeepEqual(got, []string{"factor", "scales", "units"}) {
		t.Errorf("Terms keeps the math or a word twice: %q", got)
	}
}

func TestStem(t *testing.T) {
	for in, want := range map[string]string{
		"regularizes": "regulariz", "regularized": "regulariz", "pages": "page", "boxes": "box",
		"studies": "study", "training": "train", "units": "unit", "normalization": "normaliz",
	} {
		if got := stem(in); got != want {
			t.Errorf("stem(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestVocabHas(t *testing.T) {
	v := NewVocab("Batch normalizing and the scaled weights; well-known units.")
	for _, w := range []string{"normalization", "scaling", "weight", "known", "unit"} {
		if !v.Has(w) {
			t.Errorf("Has(%q) = false", w)
		}
	}
	for _, w := range []string{"dropout", "batches2"} {
		if v.Has(w) {
			t.Errorf("Has(%q) = true", w)
		}
	}
}

func TestCheck(t *testing.T) {
	d := Parse(paper)
	snap := Snapshot{Work: "w", Hash: "abc", Extractor: "x+" + ExtractorVersion}
	for _, tc := range []struct {
		text, loc string
		pass      bool
		why       string
	}{
		{"The MNIST error falls from 1.6% to 1.25%.", "Section 3", true, ""},
		{"The MNIST error falls from 1.6% to 1.20%.", "Section 3", false, "the numbers 1.20%"},
		{"At test time the weights are scaled by p.", "Section 2.1", true, ""},
		{"Recurrent connections need variational masks across timesteps.", "Section 2.1", false, "key terms"},
		{"The weights are scaled.", "Section 8", false, "does not resolve"},
		{"The weights are scaled.", "NeurIPS 26", false, "does not resolve"},
	} {
		c := &claims.Claim{ID: "c", Text: tc.text}
		r := Check(c, claims.Evidence{Work: "w", Locator: tc.loc}, d, snap)
		if r.Pass != tc.pass {
			t.Errorf("%q at %s: pass = %v, want %v (%s)", tc.text, tc.loc, r.Pass, tc.pass, r.Why())
		}
		if !strings.Contains(r.Why(), tc.why) {
			t.Errorf("%q at %s: Why = %q, want it to say %q", tc.text, tc.loc, r.Why(), tc.why)
		}
		if r.Key != Key(tc.text, "abc", tc.loc, snap.Extractor) {
			t.Errorf("the key of the record is not Key of its inputs")
		}
	}
}

func TestKeyChanges(t *testing.T) {
	base := Key("claim", "snap", "Section 1", "e")
	for _, k := range []string{Key("claim.", "snap", "Section 1", "e"), Key("claim", "snap2", "Section 1", "e"),
		Key("claim", "snap", "Section 2", "e"), Key("claim", "snap", "Section 1", "e2")} {
		if k == base {
			t.Error("a change of an input does not change the key")
		}
	}
}

func TestIndexRoundTrip(t *testing.T) {
	root := t.TempDir()
	snap := Snapshot{Work: "w", Version: "v1", Hash: "abc", Extractor: "x+" + ExtractorVersion}
	c := &claims.Claim{ID: "c1", Text: "The weights are scaled by p."}
	e := claims.Evidence{Work: "w", Locator: "Section 2.1"}
	r := Check(c, e, Parse(paper), snap)
	if !r.Pass {
		t.Fatal(r.Why())
	}
	if err := WriteManifest(root, map[string]Snapshot{"w": snap}); err != nil {
		t.Fatal(err)
	}
	if err := WriteRecords(root, "dropout", []Record{r}); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(root, RecordDir, "dropout.jsonl"))
	if bytes.Contains(b, []byte("scaled by p")) {
		t.Error("a record holds text of the source (C01)")
	}
	ix, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if !ix.Grounded("dropout", c, e) {
		t.Error("the record is not read back as grounded")
	}
	c2 := *c
	c2.Text = "The weights are scaled by q."
	if ix.Grounded("dropout", &c2, e) {
		t.Error("a record of an older claim text counts")
	}
	if ix.Grounded("dropout", c, claims.Evidence{Work: "w", Locator: "Section 2"}) {
		t.Error("a record of another locator counts")
	}
	var nilIx *Index
	if nilIx.Grounded("dropout", c, e) {
		t.Error("a nil index grounds a claim")
	}
	empty, err := Load(t.TempDir())
	if err != nil || len(empty.Snapshots) != 0 {
		t.Errorf("Load of an empty root = %+v, %v", empty, err)
	}
}

func TestStore(t *testing.T) {
	s := Store{t.TempDir()}
	h, err := s.Put([]byte("%PDF-1.4 body"), Meta{URL: "https://example.org/a.pdf"})
	if err != nil {
		t.Fatal(err)
	}
	if h != Hash([]byte("%PDF-1.4 body")) || !s.Has(h) {
		t.Fatalf("Put = %s", h)
	}
	b, err := s.Get(h)
	if err != nil || string(b) != "%PDF-1.4 body" {
		t.Fatalf("Get = %q, %v", b, err)
	}
	if s.Doc(h) != nil {
		t.Error("Doc of an object with no extraction is not nil")
	}
	d := Parse(paper)
	d.Extractor = "pdftotext-1+" + ExtractorVersion
	if err := s.PutDoc(h, d); err != nil {
		t.Fatal(err)
	}
	if got := s.Doc(h); got == nil || len(got.Heads) != len(d.Heads) {
		t.Error("PutDoc then Doc does not give the doc")
	}
	d.Extractor = "pdftotext-1+aik-doc-0"
	_ = s.PutDoc(h, d)
	if s.Doc(h) != nil {
		t.Error("Doc gives an extraction by an older extractor")
	}
	if err := os.WriteFile(s.path(h, ""), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(h); err == nil {
		t.Error("Get gives an object whose bytes do not have its hash")
	}
	if _, err := Extract(t.Context(), []byte("<html>login</html>"), "pdf"); !errors.Is(err, ErrNotPDF) {
		t.Errorf("Extract of HTML as a PDF = %v", err)
	}
}

func TestExtractHTML(t *testing.T) {
	d := ExtractHTML([]byte(`<html><nav>menu</nav><h2>1 Introduction</h2><p>Text here.</p><script>x()</script><h2>2 Method</h2><p>More.</p></html>`))
	if strings.Contains(d.Text, "menu") || strings.Contains(d.Text, "x()") {
		t.Errorf("text = %q", d.Text)
	}
	if len(d.Heads) != 2 {
		t.Errorf("heads = %+v", d.Heads)
	}
}

func TestMerge(t *testing.T) {
	got := Merge([]Span{{10, 20}, {0, 5}, {15, 30}, {30, 31}})
	if want := []Span{{0, 5}, {10, 31}}; !reflect.DeepEqual(got, want) {
		t.Errorf("Merge = %v, want %v", got, want)
	}
}

func TestPassage(t *testing.T) {
	d := Parse(paper)
	r := Check(&claims.Claim{ID: "c", Text: "At test time the weights are scaled by p."}, claims.Evidence{Work: "w", Locator: "Section 2.1"}, d, Snapshot{Hash: "h"})
	if !r.Pass {
		t.Fatalf("the check fails: %s", r.Why())
	}
	p, sec := Passage(d, r.Spans)
	if !strings.Contains(p, "scaled by p") {
		t.Errorf("the passage %q does not hold the span", p)
	}
	if sec != "" && !strings.Contains(sec, "scaled by p") {
		t.Errorf("the section %q does not hold the span", sec)
	}
	if p, sec := Passage(d, []Span{{Start: -1, End: 5}, {Start: 10, End: len(d.Text) + 1}}); p != "" || sec != "" {
		t.Errorf("spans outside the text give %q, %q", p, sec)
	}
	for _, v := range []string{Supported, Unsupported, Contradicted, Unreadable} {
		if !slices.Contains(Verdicts, v) {
			t.Errorf("Verdicts has no %q", v)
		}
	}
}

func TestParseContents(t *testing.T) {
	text := "Title\n\nContents\n1 Introduction\n\n3\n\n2 Method\n2.1 Setup . . . . . . 5\n3 Results\n9\n\f" +
		"1\n\nIntroduction\n\nWe study a thing.\n\n2\n\nMethod\n\nThe method.\n\n2.1\n\nSetup\n\nThe setup.\n\n" +
		"1.0\n\n5.02\n\n3 Results\n\nAvg. Test RMSE\n\n4 Avg. Test RMSE and Std. Errors\n\n4 X = apx\n\n4 R EGULARIZING RNN S WITH LSTM C ELLS\n"
	d := Parse(text)
	var got []string
	for _, h := range d.Heads {
		got = append(got, h.Num+" "+h.Title)
	}
	want := []string{"1 Introduction", "2 Method", "2.1 Setup", "3 Results", "4 REGULARIZING RNNS WITH LSTM CELLS"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("heads = %q, want %q", got, want)
	}
	if len(d.Heads) > 0 && d.Heads[0].Start < strings.IndexByte(text, '\f') {
		t.Errorf("the first heading is in the table of contents, at %d", d.Heads[0].Start)
	}
}
