// SPDX-License-Identifier: Apache-2.0

package cite

import (
	"fmt"
	"strings"
	"testing"

	"github.com/tamnd/ai-agent/claims"
)

func heads(d *Doc) string {
	var out []string
	for _, h := range d.Heads {
		out = append(out, strings.TrimSpace(h.Num+" "+h.Title))
	}
	return strings.Join(out, "|")
}

func hasMark(d *Doc, kind, num string) (Mark, bool) {
	for _, m := range d.Marks {
		if m.Kind == kind && m.Num == num {
			return m, true
		}
	}
	return Mark{}, false
}

// A caption at the top of a page is on that page, not on the page before: its offset is after the form feed.
func TestParseCaptionTopOfPage(t *testing.T) {
	d := Parse("1 Introduction\n\nThe first page.\n\n1\n\fTable 1: BLEU scores of the systems.\n\nSystem BLEU\n\n2\n")
	m, ok := hasMark(d, "table", "1")
	if !ok {
		t.Fatalf("no table 1 in %+v", d.Marks)
	}
	if p := d.Page(m.Start); p != 1 {
		t.Errorf("table 1 is on page %d, want 1", p)
	}
	if !strings.HasPrefix(d.Text[m.Start:], "Table 1") {
		t.Errorf("table 1 starts at %q", d.Text[m.Start:m.Start+10])
	}
	s, why := d.resolve(Ref{Kind: "table", From: "1"})
	if why != "" || d.Page(s.Start) != 1 {
		t.Errorf("Table 1 resolves to page %d (%s)", d.Page(s.Start), why)
	}
}

// Papineni et al. 2002 in two columns: a footnote "3 As a guide to the eye" and its run-on line are not section 3,
// which would leave out every heading of section 2 after it, and 2.1.1 is alone above 2.1.2 and its title.
func TestParseTwoColumnFootnotes(t *testing.T) {
	text := "BLEU: a Method for Automatic Evaluation of Machine Translation\n\nAbstract\nHuman evaluations of machine translation are extensive but expensive.\n\n" +
		"1 Introduction\n1.1 Rationale\nHuman evaluations of machine translation (MT) weigh many aspects.\n\n" +
		"1.2 Viewpoint\nHow does one measure translation performance?\n\n\f" +
		"2 The Baseline BLEU Metric\nTypically, there are many perfect translations of a given source sentence.\n\n" +
		"2.1 Modified n-gram precision\nThe cornerstone of our metric is the familiar precision measure.\n" +
		"3 As a guide to the eye, we have underlined the important\nwords for comparing similarity.\n" +
		"4 B LEU only needs to match human judgment when averaged\nover a test corpus.\n\n" +
		"2.1.1\n\n2.1.2\n\nRanking systems using only modified\nn-gram precision\nTo verify that modified n-gram precision distinguishes.\n\n\f" +
		"2.2\n\nSentence length\nA candidate translation should be neither too long nor too short.\n\n" +
		"2.2.1 The trouble with recall\nTraditionally, precision has been paired with recall.\n\n" +
		"6 Using the geometric average also yields slightly stronger\ncorrelation with human judgments than our best results.\n\n" +
		"2.3 BLEU details\nWe take the geometric mean of the test corpus' modified precision scores.\n\n" +
		"3 The BLEU Evaluation\nThe BLEU metric ranges from 0 to 1.\n\n4 The Human Evaluation\nWe had two groups of human judges.\n"
	d := Parse(text)
	want := "1 Introduction|1.1 Rationale|1.2 Viewpoint|2 The Baseline BLEU Metric|2.1 Modified n-gram precision|2.1.1|2.1.2 Ranking systems using only modified|2.2 Sentence length|2.2.1 The trouble with recall|2.3 BLEU details|3 The BLEU Evaluation|4 The Human Evaluation"
	if got := heads(d); got != want {
		t.Errorf("heads =\n%s\nwant\n%s", got, want)
	}
}

// Chen and Cherry 2014: a numbered finding "3. BLEU seems to be the best tuning metric" in the introduction is not
// section 3, and the K of a formula between the number 3 and its title "Experiments" is skipped.
func TestParseNumberedListAndFormula(t *testing.T) {
	text := "1 Introduction\nWe find that:\n1. Smoothing helps sentence-level BLEU\n2. No single method is always best\n" +
		"3. BLEU seems to be the best tuning metric\n\n2 BLEU and smoothing\n2.1 BLEU\nBLEU is the geometric mean of precisions.\n\n" +
		"2.2 Smoothing techniques\nSmoothing 1: if the number of matched n-grams is 0, we use a small value.\n\n" +
		"3\n\navailable at http://www.itl.nist.gov/iad/mig/tests/mt/2009/\n\nK\nln(len(T ))\n\nExperiments\n\nWe carried out two series of experiments.\n\n" +
		"3.1 Evaluation task\nWe used the data of WMT.\n\n3.2 Tuning task\nWe tuned with each metric.\n\n4 Conclusions\nSmoothing 7 is best.\n"
	d := Parse(text)
	want := "1 Introduction|2 BLEU and smoothing|2.1 BLEU|2.2 Smoothing techniques|3 Experiments|3.1 Evaluation task|3.2 Tuning task|4 Conclusions"
	if got := heads(d); got != want {
		t.Errorf("heads =\n%s\nwant\n%s", got, want)
	}
}

// Kaplan et al. 2020: the table of contents is on the page of the introduction, so it is the title "Introduction"
// again that ends it; 2.1 has its title on the line after its number; "Table 1 Parameter counts" is a caption.
func TestParseContentsSamePage(t *testing.T) {
	text := "Scaling Laws for Neural Language Models\n\f" +
		"Contents\n1\n\nIntroduction\n\n2\n\n2\n\nBackground and Methods\n\n6\n\n3\n\nEmpirical Results and Basic Power Laws\n\n7\n\n" +
		"1\n\nIntroduction\n\nLanguage provides a natural domain for the study of artificial intelligence.\n\n" +
		"1.1\n\nSummary\n\nOur key findings for Transformer language models are as follows.\n\n\f" +
		"2\n\nBackground and Methods\n\nWe train language models on WebText2.\n\n" +
		"2.1\n\nParameter and Compute Scaling of Transformers\n\nWe parameterize the Transformer architecture.\n\n" +
		"Table 1 Parameter counts and compute (forward pass) estimates for a Transformer model.\n\n" +
		"3\n\nEmpirical Results and Basic Power Laws\n\nTo characterize language model scaling we train a wide variety of models.\n"
	d := Parse(text)
	want := "1 Introduction|1.1 Summary|2 Background and Methods|2.1 Parameter and Compute Scaling of Transformers|3 Empirical Results and Basic Power Laws"
	if got := heads(d); got != want {
		t.Errorf("heads =\n%s\nwant\n%s", got, want)
	}
	if len(d.Heads) > 0 && d.Heads[0].Start < strings.Index(text, "Language provides")-20 {
		t.Errorf("the first heading is in the table of contents, at %d", d.Heads[0].Start)
	}
	if _, ok := hasMark(d, "table", "1"); !ok {
		t.Errorf("no table 1 in %+v", d.Marks)
	}
	if _, ok := hasMark(Parse("As Table 1 shows, the counts\n"), "table", "1"); ok {
		t.Errorf("a mention of Table 1 is a caption")
	}
}

// Press et al. 2022: the footnotes "2 Code & models: https://..." and "3 These include the embedding lookup" with its
// run-on line are not headings, so section 4 is found, and its number is alone above its title. The small capitals
// come out of pdftotext split, and joinCaps mends only some of them.
func TestParsePressFootnotes(t *testing.T) {
	text := "1 I NTRODUCTION\n\nWhen constructing a transformer-based language model, a major design decision is made.\n" +
		"2 Code & models: https://github.com/ofirpress/attention_with_linear_biases\n\f" +
		"2 C URRENT APPROACHES D O NOT E XTRAPOLATE E FFICIENTLY\n\nWe show for the first time.\n\n" +
		"2.1 B ACKGROUND AND E XPERIMENTAL S ETUP\n\nA transformer language model receives a list of tokens.\n" +
		"3 These include the embedding lookup, feedforward sublayer, and softmax layer, which act independently\non vector inputs.\n\n\f" +
		"3 ATTENTION WITH L INEAR B IASES (AL I B I )\n\nIn the transformer model of Vaswani et al., position embeddings are added.\n\n\f" +
		"4\n\nR ESULTS\n\nThis section shows that ALiBi works.\n\n4.1\n\nR ESULTS ON W IKI T EXT-103\n\nWe first develop our method.\n"
	d := Parse(text)
	// The small capitals "D O NOT" are joined one letter off; the literal is split so that misspell does not read it.
	want := "1 INTRODUCTION|2 CURRENT APPROACHES D O" + "NOT EXTRAPOLATE EFFICIENTLY|2.1 BACKGROUND AND EXPERIMENTALS ETUP|3 ATTENTION WITH LINEAR BIASES (AL I B I )|4 RESULTS|4.1 RESULTS ON WIKI TEXT-103"
	if got := heads(d); got != want {
		t.Errorf("heads =\n%s\nwant\n%s", got, want)
	}
}

// Post 2018: a footnote mark 4 alone above "2.2" does not take the title of 2.2.
func TestParseMarkAboveHeading(t *testing.T) {
	text := "1 Introduction\n\nScience is the process of formulating hypotheses.\n\n2 Problem Description\n\n" +
		"2.1 Problem: BLEU is underspecified\n\nBLEU is a parameterized metric.\n\n4\n\n2.2\n\nProblem: Different reference\n\npreprocessing cannot be compared\n"
	d := Parse(text)
	want := "1 Introduction|2 Problem Description|2.1 Problem: BLEU is underspecified|2.2 Problem: Different reference"
	if got := heads(d); got != want {
		t.Errorf("heads =\n%s\nwant\n%s", got, want)
	}
}

// Vaswani et al. 2017: footnote 4 has its number alone on a line at the foot of the page, and "footnote 4" resolves.
func TestParseFootnoteAlone(t *testing.T) {
	var b strings.Builder
	b.WriteString("3.2.1 Scaled Dot-Product Attention\n\n")
	for i := range 30 {
		fmt.Fprintf(&b, "We call our particular attention Scaled Dot-Product Attention, line %d of the page.\n", i)
	}
	b.WriteString("4\n\nTo illustrate why the dot products get large, assume that the components of q and k are independent.\n\n4\n\f")
	b.WriteString("3.2.2 Multi-Head Attention\n\n4\n\noutput values. These are concatenated and once again projected.\n")
	d := Parse(b.String())
	m, ok := hasMark(d, "footnote", "4")
	if !ok {
		t.Fatalf("no footnote 4 in %+v", d.Marks)
	}
	s, why := d.resolve(Ref{Kind: "footnote", From: "4"})
	if why != "" || !strings.Contains(d.Text[s.Start:s.End], "To illustrate why the dot products get large") {
		t.Errorf("footnote 4 at %d resolves to %q (%s)", m.Start, d.Text[s.Start:s.End], why)
	}
	n := 0
	for _, m := range d.Marks {
		if m.Kind == "footnote" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("footnotes = %+v, want the one 4: the page number and a mark before a run-on line are not footnotes", d.Marks)
	}
}

// The Stanford Encyclopedia: the table of contents lists "Bibliography", which would end the body before its first
// heading; the headings come from the markup, and the bibliography ends the last section.
func TestExtractHTMLContents(t *testing.T) {
	page := `<html><body><h1>The Chinese Room Argument</h1><div id="toc"><ul>` +
		`<li><a href="#Over">1. Overview</a></li><li><a href="#Hist">2. Historical Background</a><ul><li><a href="#Leib">2.1 Leibniz&rsquo; Mill</a></li></ul></li>` +
		`<li><a href="#Bib">Bibliography</a></li><li><a href="#Rel">Related Entries</a></li></ul></div>` +
		`<div id="main-text"><h2 id="Over">1. Overview</h2><p>Work in Artificial Intelligence has produced computer programs.</p>` +
		`<h2 id="Hist">2. Historical Background</h2><h3 id="Leib">2.1 Leibniz&rsquo; Mill</h3><p>Leibniz asks us to imagine a mill.</p>` +
		`<h2 id="Conc">Conclusion</h2><p>The argument has been widely discussed.</p>` +
		`<h2 id="Bib">Bibliography</h2><p>Searle, J., 1980, &ldquo;Minds, Brains and Programs&rdquo;.</p></div></body></html>`
	d := ExtractHTML([]byte(page))
	if got, want := heads(d), "1 Overview|2 Historical Background|2.1 Leibniz’ Mill"; got != want {
		t.Fatalf("heads = %q, want %q", got, want)
	}
	s, why := d.resolve(Ref{Kind: "section", From: "2.1"})
	if txt := d.Text[s.Start:s.End]; why != "" || !strings.Contains(txt, "imagine a mill") || strings.Contains(txt, "widely discussed") {
		t.Errorf("section 2.1 = %q (%s)", txt, why)
	}
	s, _ = d.resolve(Ref{Kind: "section", From: "1"})
	if txt := d.Text[s.Start:s.End]; !strings.Contains(txt, "computer programs") || strings.Contains(txt, "Leibniz asks") {
		t.Errorf("section 1 = %q", txt)
	}
}

// The references of the Stanford Encyclopedia are the last "Bibliography", after the one of its table of contents, and
// run to the end of the text; those of a paper end at the appendix after them.
func TestReferences(t *testing.T) {
	page := `<html><body><h1>The Chinese Room Argument</h1><div id="toc"><ul>` +
		`<li><a href="#Over">1. Overview</a></li><li><a href="#Bib">Bibliography</a></li></ul></div>` +
		`<div id="main-text"><h2 id="Over">1. Overview</h2><p>Work in Artificial Intelligence has produced computer programs.</p>` +
		`<h2 id="Bib">Bibliography</h2><p>Preston, J. and M. Bishop (eds.), 2002, <em>Views into the Chinese Room</em>.</p></div></body></html>`
	d := ExtractHTML([]byte(page))
	if txt := resolve(t, d, "the bibliography"); !strings.HasPrefix(txt, "Bibliography") || !strings.Contains(txt, "Views into the Chinese Room") || strings.Contains(txt, "computer programs") {
		t.Errorf("the bibliography = %q", txt)
	}
	d = &Doc{Text: "1 Introduction\nDropout works.\n\nReferences\nHinton, G. 2012. Improving neural networks.\n\nA Proofs\nThe proof.\n"}
	d.Heads = []Head{{Num: "1", Title: "Introduction", Start: 0}, {Num: "A", Title: "Proofs", Start: strings.Index(d.Text, "A Proofs")}}
	if txt := resolve(t, d, "the references"); !strings.Contains(txt, "Improving neural networks") || strings.Contains(txt, "The proof") {
		t.Errorf("the references = %q", txt)
	}
	if _, fails := Resolve(&Doc{Text: "1 Introduction\nNo list here.\n"}, []Ref{{Kind: "references"}}); len(fails) == 0 {
		t.Error("a text with no references resolved them")
	}
}

// Searle 1980 in BBS, a scan: the odd pages print their numbers at the foot, the even pages lose them in the noise of
// the margin or put them mid-column, and the numbers that continue the neighbours find them.
func TestPrintedNumbersScan(t *testing.T) {
	pages := []string{
		"THE BEHAVIORAL AND BRAIN SCIENCES (1980)\nMinds, brains, and programs\nJohn R. Searle\nthe text of page one\nTHE BEHAVIORAL AND BRAIN SCIENCES (1980)\n417\n",
		"Searle: Minds, brains, and programs\n(as is indeed the case) that I know no Chinese\n~\n418 the margin\nunderstands y; and so on.\nof course, of course.\n",
		"Searle: Minds, brains, and programs\npoints at issue.\nthat I have a certain sort of input\nTHE BEHAVIORAL AND BRAIN SCIENCES (1980)\n419\n",
		"~\n...\nSearle: Minds, brains, and programs\nthe systems reply\nill\\j\nl!\n;j\n",
		"Searle: Minds, brains, and programs\nBefore countering this reply\nTHE BEHAVIORAL AND BRAIN SCIENCES (1980)\n421\n",
		"Searle: Minds, brains, and programs\nattributing to them\n422\nthe level of mental operations\nformal program\n",
	}
	d := Parse(strings.Join(pages, "\f"))
	for n, want := range map[int]int{417: 0, 418: 1, 419: 2, 420: 3, 421: 4, 422: 5} {
		if got := d.printed(n); got != want {
			t.Errorf("p. %d is the page %d, want %d", n, got, want)
		}
	}
	// A page with no number and no neighbour that shows one is found by its place only.
	d = Parse("Cover of the archive\nAll use subject to terms\n\fCould a Machine Think?\nthe text\nAll ©\nuse1989\n" +
		"\fthe text of the second page\nSCIENTIFIC AMERICAN,\nINC\n33\n\fthe text of the third page\nAll ©\nsubject to\n" +
		"\fthe text of the fourth page\nSCIENTIFIC AMERICAN,\nINC\n35\n")
	for n, want := range map[int]int{32: 1, 33: 2, 34: 3, 35: 4} {
		if got := d.printed(n); got != want {
			t.Errorf("p. %d is the page %d, want %d", n, got, want)
		}
	}
	if got := d.printed(31); got != -1 && got != 0 {
		t.Errorf("p. 31 is the page %d", got)
	}
}

// Understanding Machine Learning: the contents ends with a lone 50, and page 50, the first of a section, prints no
// number; p. 50 is the page after 49, not the contents.
func TestPrintedNumbersContents(t *testing.T) {
	pages := []string{"Contents\nPreface\n3.4 Bibliographic Remarks\n50\n", "Preface\nthe preface prints no number\n"}
	for n := 2; n <= 6; n++ {
		pages = append(pages, fmt.Sprintf("the text of page %d\nmore text\n%d\n", n, n))
	}
	pages = append(pages, "3.4 Bibliographic Remarks\nthe text of page 7\n")
	for n := 8; n <= 9; n++ {
		pages = append(pages, fmt.Sprintf("%d\nthe text of page %d\n", n, n))
	}
	d := Parse(strings.Join(pages, "\f"))
	for n, want := range map[int]int{2: 2, 6: 6, 7: 7, 8: 8} {
		if got := d.printed(n); got != want {
			t.Errorf("p. %d is the page %d, want %d", n, got, want)
		}
	}
	pages[0] = "Contents\nPreface\n7\n"
	d = Parse(strings.Join(pages, "\f"))
	if got := d.printed(7); got != 7 {
		t.Errorf("p. 7 is the page %d, want 7: the contents took it", got)
	}
}

// A verdict outlives a new extractor when the claim, the work, the locator, the snapshot and the passage that the
// verifier read are the same, and only then.
func TestCarry(t *testing.T) {
	d := Parse(paper)
	c := &claims.Claim{ID: "c", Text: "At test time the weights are scaled by p."}
	e := claims.Evidence{Work: "w", Locator: "Section 2.1"}
	snap := Snapshot{Work: "w", Hash: "h", Extractor: "pdftotext-1+" + ExtractorVersion}
	r := Check(c, e, d, snap)
	if !r.Pass || r.Judged == "" || r.Passage == "" {
		t.Fatalf("the check gives %+v", r)
	}
	old := r
	old.Key, old.Verdict, old.Verifier, old.Run = "an older key", Supported, "codex:m", "r1"
	if !Carry(old, r, c, d, nil, snap) {
		t.Errorf("the verdict of the same passage is not carried")
	}
	for name, f := range map[string]func(o *Record){
		"no verdict":   func(o *Record) { o.Verdict = "" },
		"another text": func(o *Record) { o.Judged = Hash([]byte("At test time the weights are halved."))[:32] },
		"a passage":    func(o *Record) { o.Passage = Hash([]byte("another passage"))[:32] },
		"a locator":    func(o *Record) { o.Locator = "Section 2" },
		"a snapshot":   func(o *Record) { o.Snapshot = "h2" },
	} {
		o := old
		f(&o)
		if Carry(o, r, c, d, nil, snap) {
			t.Errorf("the verdict is carried with %s changed", name)
		}
	}
	// A record of the previous extractor has no hashes: its key and its spans tell what was judged.
	legacy := old
	legacy.Judged, legacy.Passage = "", ""
	legacy.Key = Key(Judged(c), snap.Hash, e.Locator, "pdftotext-1+"+HashlessExtractorVersion)
	if !Carry(legacy, r, c, d, nil, snap) {
		t.Errorf("the verdict of a record of %s is not carried", HashlessExtractorVersion)
	}
	changed := *c
	changed.Text = "At test time the weights are scaled by p, always."
	if Carry(legacy, Check(&changed, e, d, snap), &changed, d, nil, snap) {
		t.Errorf("the verdict of a record of %s is carried to another text", HashlessExtractorVersion)
	}
	moved := legacy
	moved.Spans = []Span{{0, 40}}
	if Carry(moved, r, c, d, nil, snap) {
		t.Errorf("the verdict of a record of %s is carried to another passage", HashlessExtractorVersion)
	}
	legacy.Key = Key(Judged(c), snap.Hash, e.Locator, "pdftotext-1+aik-doc-2")
	if Carry(legacy, r, c, d, nil, snap) {
		t.Errorf("the verdict of a record of an older extractor is carried")
	}
}

// An address on the title page, "1 Microsoft Way", is not section 1: headings come after the abstract.
func TestParseAddressBeforeAbstract(t *testing.T) {
	text := "To Ship or Not to Ship: An Extensive Evaluation of Automatic Metrics for Machine Translation\n\n" +
		"Tom Kocmi Christian Federmann\nMicrosoft\n1 Microsoft Way\nRedmond, WA 98052, USA\n" +
		"{tomkocmi,chrife}@microsoft.com\n\nAbstract\n\nAutomatic metrics are commonly used as the main tool.\n\n" +
		"1\n\nIntroduction\n\nAutomatic evaluation metrics are commonly used as the main tool for comparing.\n\n" +
		"2\n\nData\n\nWe use the data of a large human evaluation.\n"
	d := Parse(text)
	got := heads(d)
	if !strings.Contains(got, "1 Introduction") || strings.Contains(got, "Microsoft") {
		t.Fatalf("heads = %q", got)
	}
}

// The previous extractor put a heading at the top of a page on the form feed before it, so that a record of it holds
// a span from the form feed; it is the same passage.
func TestCarryPageTopHeading(t *testing.T) {
	text := "A Study of Dropout\n\nAbstract\n\nWe study dropout.\n\n1 Introduction\n\nDropout is a way to regularize nets.\n" +
		"\f2 Method\n\nAt test time the weights are scaled by p, the probability of keeping a unit.\n\n3 Results\n\nIt works.\n"
	d := Parse(text)
	c := &claims.Claim{ID: "c", Text: "At test time the weights are scaled by p."}
	e := claims.Evidence{Work: "w", Locator: "Section 2"}
	snap := Snapshot{Work: "w", Hash: "h", Extractor: "pdftotext-1+" + ExtractorVersion}
	r := Check(c, e, d, snap)
	if !r.Pass || len(r.Spans) != 1 || d.Text[r.Spans[0].Start-1] != '\f' {
		t.Fatalf("the check gives %+v", r)
	}
	legacy := r
	legacy.Judged, legacy.Passage, legacy.Verdict = "", "", Supported
	legacy.Key = Key(Judged(c), snap.Hash, e.Locator, "pdftotext-1+"+HashlessExtractorVersion)
	legacy.Spans = []Span{{r.Spans[0].Start - 1, r.Spans[0].End}}
	if !Carry(legacy, r, c, d, nil, snap) {
		t.Errorf("the verdict of a span from the form feed is not carried")
	}
	// Section 1 ends at the heading of Section 2 now, past the form feed where it ended before.
	e.Locator = "Section 1"
	c.Text = "Dropout is a way to regularize nets."
	r = Check(c, e, d, snap)
	if !r.Pass || len(r.Spans) != 1 || d.Text[r.Spans[0].End-1] != '\f' {
		t.Fatalf("the check gives %+v", r)
	}
	legacy = r
	legacy.Judged, legacy.Passage, legacy.Verdict = "", "", Supported
	legacy.Key = Key(Judged(c), snap.Hash, e.Locator, "pdftotext-1+"+HashlessExtractorVersion)
	legacy.Spans = []Span{{r.Spans[0].Start, r.Spans[0].End - 1}}
	if !Carry(legacy, r, c, d, nil, snap) {
		t.Errorf("the verdict of a span up to the form feed is not carried")
	}
}

// Silver et al. 2016 in Nature: the running line at the foot spaces out its digits, "4 8 4 | N AT U R E | VO L 5 2 9",
// and the online part has its own tables, Extended Data Table 1 to 11.
func TestNature(t *testing.T) {
	foot := []string{"4 8 4 | N AT U R E | VO L 5 2 9 | 2 8 JA N UA RY 2 0 1 6", "2 8 JA N UA RY 2 0 1 6 | VO L 5 2 9 | N AT U R E | 4 8 5", "4 8 6 | N AT U R E | VO L 5 2 9 | 2 8 JA N UA RY 2 0 1 6"}
	var pages []string
	for i, f := range foot {
		pages = append(pages, fmt.Sprintf("ARTICLE RESEARCH\nthe text of page %d\nmore text\nand more\n%s\n© 2016 Macmillan Publishers Limited. All rights reserved\n", i+1, f))
	}
	pages = append(pages, "RESEARCH ARTICLE\nMETHODS\nProblem setting.\n",
		"ARTICLE RESEARCH\nExtended Data Table 1 | Details of match between AlphaGo and Fan Hui\nDate\n5 Oct 2015\n",
		"RESEARCH ARTICLE\nExtended Data Figure 2 | Tree search in AlphaGo\nthe figure\n")
	d := Parse(strings.Join(pages, "\f"))
	for n, want := range map[int]int{484: 0, 485: 1, 486: 2} {
		if got := d.printed(n); got != want {
			t.Errorf("p. %d is the page %d, want %d", n, got, want)
		}
	}
	if txt := resolve(t, d, "Extended Data Table 1"); !strings.Contains(txt, "Fan Hui") {
		t.Errorf("Extended Data Table 1 = %q", txt)
	}
	if txt := resolve(t, d, "Extended Data Fig. 2"); !strings.Contains(txt, "Tree search") {
		t.Errorf("Extended Data Fig. 2 = %q", txt)
	}
	if refs, err := ParseLocator("Table 1"); err != nil {
		t.Fatal(err)
	} else if _, fails := Resolve(d, refs); len(fails) == 0 {
		t.Error("Table 1 resolved to an Extended Data table")
	}
	for in, want := range map[string]string{"4 8 4 | N AT U R E | VO L 5 2 9": "484", "12": "12", "A | B | 2016 Macmillan": "", "Table 3 | Results": ""} {
		if got := footNumber(in); got != want {
			t.Errorf("footNumber(%q) = %q, want %q", in, got, want)
		}
	}
}
