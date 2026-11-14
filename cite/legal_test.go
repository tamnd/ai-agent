// SPDX-License-Identifier: Apache-2.0

package cite

import (
	"strings"
	"testing"

	"github.com/tamnd/ai-agent/claims"
)

// regulation is a short act laid out as the Official Journal on the web is after extraction: a point's letter and a
// recital's number on a line of their own, articles cited in sentences, and annexes after the final formula.
const regulation = `REGULATION (EU) 2024/1689 OF THE EUROPEAN PARLIAMENT AND OF THE COUNCIL
of 13 June 2024
laying down harmonised rules on artificial intelligence

Whereas:

(1)
The purpose of this Regulation is to improve the functioning of the internal market.

(2)
The rules on literacy are set out in
Article 4
of this Regulation, and apply to providers and deployers.

HAVE ADOPTED THIS REGULATION:

CHAPTER I
GENERAL PROVISIONS

Article 1
Subject matter

1. The purpose of this Regulation is to improve the functioning of the internal market.

Article 2
Scope

This Regulation applies to providers placing on the market AI systems referred to in Annex III.

CHAPTER II
FINAL PROVISIONS

Article 3
Entry into force and application

This Regulation shall enter into force on the twentieth day following that of its publication.

It shall apply from 2 August 2026.

However:

(a)
Chapters I and II shall apply from 2 February 2025;

(c)
Article 6(1) and the corresponding obligations in this Regulation shall apply from 2 August 2027.

This Regulation shall be binding in its entirety and directly applicable in all Member States.

Done at Brussels, 13 June 2024.

ANNEX I
List of Union harmonisation legislation

Section A. List based on the New Legislative Framework

1.
Directive 2006/42/EC on machinery.

ANNEX III
High-risk AI systems referred to in Article 6(2)

1.
Biometrics, in so far as their use is permitted.

2.
Critical infrastructure.
`

// amending is an act that amends another: its Article 1 quotes the articles that it replaces and inserts, and the
// quoted "Article 75b" is not an article of the act.
const amending = `REGULATION (EU) 2026/1744 OF THE EUROPEAN PARLIAMENT AND OF THE COUNCIL
amending Regulation (EU) 2024/1689

Whereas:

(1)
Article 4 of Regulation (EU) 2024/1689 should be simplified.

(2)
The obligation in
Article 4
should become an obligation of the Member States.

HAVE ADOPTED THIS REGULATION:

Article 1
Amendments to Regulation (EU) 2024/1689

Regulation (EU) 2024/1689 is amended as follows:

(1)
Article 4 is replaced by the following:
‘Article 4
AI literacy
The Commission and the Member States shall encourage literacy.’;

(2)
the following articles are inserted:
‘Article 75b
Supervision of general-purpose models
The AI Office shall supervise.
Article 75c
Cooperation
The authorities shall cooperate.’;

(n)
Article 99 is amended as follows:
paragraph 6a is inserted.

Article 2
Amendments to Regulation (EU) 2019/2144
Article 6 of Regulation (EU) 2019/2144 is deleted.

Article 3
Amendments to Regulation (EU) 2018/858
Article 7 of Regulation (EU) 2018/858 is deleted.

Article 4
Entry into force
This Regulation shall enter into force on the third day following that of its publication.

This Regulation shall be binding in its entirety and directly applicable in all Member States.
`

func resolve(t *testing.T, d *Doc, loc string) string {
	t.Helper()
	refs, err := ParseLocator(loc)
	if err != nil {
		t.Fatalf("ParseLocator(%q): %v", loc, err)
	}
	spans, fails := Resolve(d, refs)
	if len(fails) > 0 {
		t.Fatalf("%s: %v", loc, fails)
	}
	return d.Text[spans[0].Start:spans[0].End]
}

// An annex is found by its heading in capitals, and runs to the next annex: its sections and points are its own.
func TestResolveAnnexInCapitals(t *testing.T) {
	d := Parse(regulation)
	got := resolve(t, d, "Annex III")
	if !strings.HasPrefix(got, "ANNEX III\nHigh-risk") || !strings.Contains(got, "Critical infrastructure") {
		t.Errorf("Annex III = %q", got)
	}
	got = resolve(t, d, "Annex I")
	if !strings.Contains(got, "Directive 2006/42/EC") || strings.Contains(got, "ANNEX III") {
		t.Errorf("Annex I = %q", got)
	}
}

// An article cited in a sentence, even alone on its line, is not the heading of the article: a heading is "Article N"
// on a line of its own, in the run of numbers of the act.
func TestArticleHeadingIsNotAReference(t *testing.T) {
	d := Parse(regulation)
	got := resolve(t, d, "Article 3")
	if !strings.HasPrefix(got, "Article 3\nEntry into force") || !strings.Contains(got, "(c)\nArticle 6(1) and the corresponding obligations") {
		t.Errorf("Article 3 = %q", got)
	}
	if !strings.Contains(got, "shall be binding in its entirety") || strings.Contains(got, "Done at") {
		t.Errorf("Article 3, the last article, should hold the binding formula and end at the final formula: %q", got)
	}

	d = Parse(amending)
	got = resolve(t, d, "Article 4")
	if !strings.HasPrefix(got, "Article 4\nEntry into force") {
		t.Errorf("Article 4 of the amending act = %q", got)
	}
	got = resolve(t, d, "Article 1")
	for _, want := range []string{"‘Article 75b", "Article 75c\nCooperation", "(n)\nArticle 99 is amended", "paragraph 6a is inserted."} {
		if !strings.Contains(got, want) {
			t.Errorf("Article 1 of the amending act lacks %q: %q", want, got)
		}
	}
	if strings.Contains(got, "Article 2\nAmendments") {
		t.Errorf("Article 1 of the amending act runs into Article 2: %q", got)
	}
	refs, _ := ParseLocator("Article 75b")
	if _, fails := Resolve(d, refs); len(fails) == 0 {
		t.Error("Article 75b, which Article 1 inserts in another act, resolves in the amending act")
	}
}

// A span ends at the next heading of its level or above: a recital at the next recital or at the enacting formula,
// an article at the next article, a division or the final formula.
func TestProvisionEnds(t *testing.T) {
	d := Parse(regulation)
	for loc, want := range map[string]string{
		"Recital 1": "(1)\nThe purpose of this Regulation is to improve the functioning of the internal market.",
		"Recital 2": "(2)\nThe rules on literacy are set out in\nArticle 4\nof this Regulation, and apply to providers and deployers.",
		"Article 1": "Article 1\nSubject matter\n\n1. The purpose of this Regulation is to improve the functioning of the internal market.",
		"Article 2": "Article 2\nScope\n\nThis Regulation applies to providers placing on the market AI systems referred to in Annex III.",
	} {
		if got := strings.TrimSpace(resolve(t, d, loc)); got != want {
			t.Errorf("%s = %q, want %q", loc, got, want)
		}
	}
	// A paragraph of an article is read with the article as its section.
	if h := d.section(strings.Index(d.Text, "It shall apply from")); h == nil || h.Num != "Article 3" {
		t.Errorf("the section of a paragraph of Article 3 is %+v", h)
	}
	if h := d.section(strings.Index(d.Text, "The rules on literacy")); h == nil || h.Num != "Recital 2" {
		t.Errorf("the section of Recital 2 is %+v", h)
	}
}

// euPage is a page of the Official Journal on the web, with its markup: no headings in h tags, the articles in classes,
// a superscript power, notes and runs of white space.
const euPage = `<html><head><title>Regulation</title></head><body>
<nav>Home    Menu</nav>
<p class="oj-doc-ti">REGULATION (EU) 2024/1689 OF THE EUROPEAN PARLIAMENT AND OF THE COUNCIL</p>
<p class="oj-normal">laying    down
        harmonised rules on artificial intelligence</p>
<p class="oj-normal">Having regard to the opinion of the European Economic and Social Committee (<a href="#ntr1" id="ntc1">(<span class="oj-super oj-note-tag">1</span>)</a>),</p>
<p class="oj-normal">Whereas:</p>
<table><tr><td><p class="oj-normal">(1)</p></td><td><p class="oj-normal">The purpose      of this Regulation is to improve the internal market.</p></td></tr></table>
<table><tr><td><p class="oj-normal">(2)</p></td><td><p class="oj-normal">A model trained with more than 10<span class="oj-super">25</span> floating point operations has high-impact capabilities.</p></td></tr></table>
<p class="oj-normal">HAVE ADOPTED THIS REGULATION:</p>
<p class="oj-ti-art">Article 1</p>
<p class="oj-sti-art">Subject matter</p>
<p class="oj-normal">1.   The purpose of this Regulation is to improve the internal market.</p>
<p class="oj-ti-art">Article 2</p>
<p class="oj-sti-art">Entry into force</p>
<p class="oj-normal">This Regulation shall apply from 2 August 2026.</p>
<p class="oj-normal">This Regulation shall be binding in its entirety and directly applicable in all Member States.</p>
<p class="oj-note"><a id="ntr1" href="#ntc1">(<span class="oj-super">1</span>)</a> OJ C 517, 22.12.2021, p. 56.</p>
<p class="oj-note"><a id="ntr2" href="#ntc2">(<span class="oj-super">2</span>)</a> OJ C 97, 28.2.2022, p. 60.</p>
<p class="oj-doc-ti">ANNEX III</p>
<p class="oj-doc-sti">High-risk AI systems referred to in Article 6(2)</p>
<p class="oj-normal">1.   Biometrics.</p>
</body></html>`

func TestExtractLegalHTML(t *testing.T) {
	d := ExtractHTML([]byte(euPage))
	if strings.Contains(d.Text, "  ") || strings.Contains(d.Text, "\n\n\n") || strings.Contains(d.Text, " \n") {
		t.Errorf("the text keeps runs of white space: %q", d.Text)
	}
	// The title page is the opening block, before the first heading.
	title := resolve(t, d, "the title page")
	if !strings.Contains(title, "laying down harmonised rules") || !strings.Contains(title, "Whereas:") || strings.Contains(title, "The purpose") {
		t.Errorf("the title page = %q", title)
	}
	if strings.Contains(title, "Home") {
		t.Errorf("the title page holds the menu: %q", title)
	}
	// A superscript after a digit is a power.
	rec := resolve(t, d, "Recital 2")
	if !strings.Contains(rec, "more than 10^25 floating point") {
		t.Errorf("Recital 2 = %q", rec)
	}
	found := false
	for _, n := range Numbers(rec) {
		found = found || n.Value == 1e25
	}
	if !found {
		t.Errorf("the numbers of Recital 2 are %+v, with no 1e25", Numbers(rec))
	}
	// A note of the markup resolves to itself; its tag in the text is not a power.
	if fn := strings.TrimSpace(resolve(t, d, "Footnote 1")); !strings.Contains(fn, "OJ C 517") || strings.Contains(fn, "OJ C 97") || strings.Contains(fn, "shall be binding") {
		t.Errorf("Footnote 1 = %q", fn)
	}
	if strings.Contains(d.Text, "^1") {
		t.Errorf("a note tag is read as a power: %q", d.Text)
	}
	if got := resolve(t, d, "Article 2"); !strings.Contains(got, "2 August 2026") || !strings.Contains(got, "binding") || strings.Contains(got, "OJ C 517") {
		t.Errorf("Article 2 = %q", got)
	}
	if got := resolve(t, d, "Annex III"); !strings.Contains(got, "Biometrics") {
		t.Errorf("Annex III = %q", got)
	}
}

func TestNumbersPowers(t *testing.T) {
	for in, want := range map[string]float64{
		"more than 10^25 operations": 1e25,
		"$10^{25}$ FLOPs":            1e25,
		"10²⁵ FLOPs":                 1e25,
		"a rate of 3 × 10^-4":        3e-4,
		"$3 \\times 10^{-4}$":        3e-4,
		"2^10 entries":               1024,
	} {
		ns := Numbers(in)
		if len(ns) != 1 || ns[0].Value < want*0.999999 || ns[0].Value > want*1.000001 {
			t.Errorf("Numbers(%q) = %+v, want %v", in, ns, want)
		}
	}
	if ns := Numbers("$x^2$ and $2^n$"); len(ns) != 1 || ns[0].Value != 2 {
		t.Errorf("Numbers reads a symbol's exponent: %+v", ns)
	}
	claim := Numbers("10^25")[0]
	if !Found(claim, Numbers("more than 10^25 floating point operations")) {
		t.Error("10^25 is not found in 10^25")
	}
	if !Found(claim, Numbers("more than 1025 floating point operations")) {
		t.Error("10^25 is not found in the text of a PDF, which lost the superscript")
	}
	if Found(claim, Numbers("more than 10^24 floating point operations")) {
		t.Error("10^25 is found in 10^24")
	}
}

// A verdict outlives an extractor that only joins runs of white space: the words that the verifier read are the same.
func TestCarryWhiteSpace(t *testing.T) {
	before := Parse("Article 1\nSubject matter\n\n1.   The   purpose of this Regulation is to improve the market.\n\nArticle 2\nScope\nIt applies.\n")
	after := Parse("Article 1\nSubject matter\n\n1. The purpose of this Regulation is to improve the market.\n\nArticle 2\nScope\nIt applies.\n")
	c := &claims.Claim{ID: "c", Text: "The purpose of the Regulation is to improve the market."}
	e := claims.Evidence{Work: "w", Locator: "Article 1(1)"}
	snap := Snapshot{Work: "w", Hash: "h", Extractor: "html+" + ExtractorVersion}
	old := Check(c, e, before, snap)
	r := Check(c, e, after, snap)
	if !old.Pass || !r.Pass || old.Passage == r.Passage {
		t.Fatalf("the checks give %+v and %+v", old, r)
	}
	old.Key, old.Verdict = "an older key", Supported
	if !Carry(old, r, c, after, before, snap) {
		t.Error("the verdict is not carried across a change of white space")
	}
	if Carry(old, r, c, after, nil, snap) {
		t.Error("the verdict is carried when the old text is not at hand")
	}
	moved := Check(c, claims.Evidence{Work: "w", Locator: "Article 1"}, after, snap)
	moved.Locator = old.Locator
	if Carry(old, moved, c, after, before, snap) {
		t.Error("the verdict is carried to another passage")
	}
}
