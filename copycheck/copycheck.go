// SPDX-License-Identifier: Apache-2.0

// Package copycheck is the copy check of spec 13.7 and rule C01 (19.13): it finds the prose of a page that copies a
// source, and the code that copies a file of the code corpora.
//
// The prose check has two tests. The shingle test takes the word 8-grams of each section of the page and of each
// source, and a section that shares more than [SectionShare] of its 8-grams with one source, and at least [MinShared]
// of them, fails. Only the 8-grams of the page with [Telling] or more plain words count: a name, a number or a stop word
// is not plain, so that a list of authors, the title of a law or a row of figures, which a page cannot say otherwise,
// is not taken for copied prose. The alignment test looks for close paraphrase in the same order: a sentence of the page is close to
// a sentence of a source when [CloseShare] or more of its content words are in it, and a run of [AlignRun] or more
// consecutive sentences of the page, each close to a sentence of one source that comes after the last one by at most
// [AlignGap] sentences, fails.
//
// The code check fingerprints each file by winnowing (Schleimer, Wilkerson and Aiken, 2003) after it renames every
// identifier, number and string to one token each, and a file that has [CodeShare] or more of its fingerprints in one
// file of the corpus fails. The lines of the idiom allow-list, such as optimizer.zero_grad(), do not count.
//
// Equations are facts and are not checked (13.7): a formula ends a run of words, so that no 8-gram spans it. A quotation
// in the page is a citation and is not checked either. The check uses no network and no model, the same input gives
// the same report, and the report never holds text of a source, only where the page is close to it.
package copycheck

import (
	"cmp"
	"hash/fnv"
	"regexp"
	"slices"
	"strings"
	"unicode"
)

// The thresholds of rule C01 (19.13), and the guards of this package. Version names them in a report.
const (
	Shingle      = 8    // the words of a shingle
	SectionShare = 0.05 // the share of the shingles of a section that one source may have
	MinShared    = 3    // the shared shingles below which a short section passes: one common phrase is not a copy
	Telling      = 3    // the plain words a shingle of the page needs to count
	AlignRun     = 3    // the consecutive close sentences that make a paraphrase in the same order
	AlignGap     = 3    // the most sentences of the source between two sentences of a run
	CloseShare   = 0.7  // the share of the content words of a sentence that make it close to a sentence of a source
	CloseMin     = 6    // the content words a sentence needs for the alignment test
	CodeShare    = 0.30 // the share of the fingerprints of a file that one file of the corpus may have
	CodeGram     = 12   // the tokens of a k-gram of the winnowing
	CodeWindow   = 8    // the k-grams of a window: a common run of CodeGram+CodeWindow-1 tokens is always found
	CodeMin      = 10   // the fingerprints a file needs for the code test, about 50 tokens
)

// Version is the version of the check and its thresholds.
const Version = "copycheck/1"

// Section is a part of the prose of a page: the lead, a section under its heading, or a derivation.
type Section struct {
	Name string // the heading, "lead", or derivations/<id>
	File string // the file, relative to the content root
	Line int    // the line of the file where the text starts
	Text string // the Markdown
}

// Source is one text of a corpus: a stored snapshot of a cited work, a Wikipedia article, a canon work or a code file.
type Source struct {
	Name string // the work key, or another name of the text
	URI  string // where the text is, when it has an address
	Text string
}

// Finding is a part of a page or of a file that is too close to a source.
type Finding struct {
	Test    string  `json:"test"` // shingles, aligned or code
	File    string  `json:"file"`
	Line    int     `json:"line"`              // the first line, in File
	End     int     `json:"end,omitempty"`     // the last line of an aligned run
	Section string  `json:"section,omitempty"` // the section of the prose
	Source  string  `json:"source"`
	URI     string  `json:"uri,omitempty"`
	Share   float64 `json:"share"`            // the shared share of the shingles or the fingerprints, or the least closeness of a run
	Shared  int     `json:"shared,omitempty"` // the shared shingles or fingerprints, or the sentences of a run
	Of      int     `json:"of,omitempty"`     // the shingles or fingerprints of the part
	Runs    []Run   `json:"runs,omitempty"`   // the runs of words of the page that the source has too
}

var (
	marker   = regexp.MustCompile(`\{#[^}\s]*\}`)
	wiki     = regexp.MustCompile(`\[\[(?:[a-z0-9]+:)?([^\]|]+)(?:\|([^\]]+))?\]\]`)
	mdLink   = regexp.MustCompile(`!?\[([^\]]*)\]\([^)]*\)`)
	display  = regexp.MustCompile(`(?s)\$\$.*?\$\$`)
	inline   = regexp.MustCompile(`\$[^$\n]+\$`)
	code     = regexp.MustCompile("`[^`\n]*`")
	quoted   = regexp.MustCompile(`"[^"\n]*"|“[^”\n]*”`)
	hyphened = regexp.MustCompile(`(\pL)-\n(\pL)`)
)

// brk is the word that ends a run of words: no shingle spans it.
const brk = "\x00"

// clean takes the Markdown out of a line of the page: the claim markers, the links (their labels stay), the formulas,
// the inline code and the quotations, which end a run of words.
func clean(s string) string {
	s = marker.ReplaceAllString(s, " ")
	s = wiki.ReplaceAllStringFunc(s, func(m string) string {
		g := wiki.FindStringSubmatch(m)
		if g[2] != "" {
			return g[2]
		}
		return strings.ReplaceAll(g[1], "-", " ")
	})
	s = mdLink.ReplaceAllString(s, "$1")
	for _, re := range []*regexp.Regexp{display, inline, code, quoted} {
		s = re.ReplaceAllString(s, " "+brk+" ")
	}
	return s
}

// words gives the words of a text in lower case: the runs of letters and digits, with brk where a run of words ends.
func words(s string) []string {
	ws, _ := plainWords(s)
	return ws
}

// plainWords gives the words of a text in lower case, and for each word whether it is plain: a content word that is
// not a name or a number. A word with a capital letter is a name, unless it starts a sentence and only its first letter
// is a capital.
func plainWords(s string) (out []string, plain []bool) {
	var w []rune
	start := true // the next word starts a sentence
	flush := func() {
		if len(w) == 0 {
			return
		}
		lw := strings.ToLower(string(w))
		upper := 0
		for i, r := range w {
			if unicode.IsUpper(r) && (i > 0 || !start) {
				upper++
			}
		}
		out = append(out, lw)
		plain = append(plain, upper == 0 && len(w) >= 3 && !stop[lw] && !numberWord[lw] && strings.IndexFunc(lw, unicode.IsDigit) < 0)
		w = w[:0]
		start = false
	}
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			w = append(w, r)
		case r == 0:
			flush()
			if len(out) > 0 && out[len(out)-1] != brk {
				out = append(out, brk)
				plain = append(plain, false)
			}
		default:
			flush()
			if strings.ContainsRune(".?!", r) {
				start = true
			}
		}
	}
	flush()
	return out, plain
}

// shingles gives the hashes of the word 8-grams of the words, none across brk.
func shingles(ws []string) map[uint64]bool {
	out := map[uint64]bool{}
	run := 0
	for i, w := range ws {
		if w == brk {
			run = 0
			continue
		}
		run++
		if run >= Shingle {
			out[hash(ws[i-Shingle+1:i+1])] = true
		}
	}
	return out
}

func hash(ws []string) uint64 {
	h := fnv.New64a()
	for _, w := range ws {
		_, _ = h.Write([]byte(w))
		_, _ = h.Write([]byte{' '})
	}
	return h.Sum64()
}

// sourceText joins the words that a PDF split at the end of a line.
func sourceText(s string) string { return hyphened.ReplaceAllString(s, "$1$2") }

// stop are the words that say nothing of the content of a sentence.
var stop = map[string]bool{}

func init() {
	for _, w := range strings.Fields(`a about above after again against all also although am an and any are as at be because
		been before being below between both but by can could did do does doing done down during each either even every
		few for from further had has have having he her here hers him his how however i if in into is it its itself just
		less many may me might more most much must my neither no nor not now of off often on once one only or other our
		out over own per same she should since so some such than that the their them then there these they this those
		though through thus to too two under until up upon us very was we well were what when where whether which while
		who whom whose why will with within without would yet you your`) {
		stop[w] = true
	}
	for _, w := range strings.Fields(`zero three four five six seven eight nine ten eleven twelve twenty thirty forty fifty
		hundred thousand million billion trillion first second third half percent times`) {
		numberWord[w] = true
	}
}

// numberWord are the words of numbers and units, which are facts like the numbers.
var numberWord = map[string]bool{}

// content gives the stems of the content words of a run of words: no stop word, no number and no word of one or two
// letters.
func content(ws []string) map[string]bool {
	out := map[string]bool{}
	for _, w := range ws {
		if w == brk || len(w) < 3 || stop[w] || strings.IndexFunc(w, func(r rune) bool { return !unicode.IsDigit(r) }) < 0 {
			continue
		}
		out[stem(w)] = true
	}
	return out
}

// stem takes the common endings off a word, so that dropped and drop, or units and unit, are one word.
func stem(w string) string {
	for _, suf := range []string{"ingly", "edly", "ing", "ed", "ly", "es", "s"} {
		if !strings.HasSuffix(w, suf) || len(w)-len(suf) < 4 || suf == "s" && strings.HasSuffix(w, "ss") {
			continue
		}
		w = w[:len(w)-len(suf)]
		if n := len(w); w[n-1] == w[n-2] && !strings.ContainsRune("lsz", rune(w[n-1])) {
			w = w[:n-1]
		}
		return w
	}
	return w
}

// lines gives the words of the lines of a section, with brk at the end of each line: a sentence of the page is a line
// (6.2), and no shingle spans two of them.
// It also gives the index of the line of each word, and which words are plain.
func lines(text string) (ws []string, at []int, plain []bool) {
	for i, l := range strings.Split(text, "\n") {
		if lw, lp := plainWords(clean(l)); len(lw) > 0 {
			ws = append(ws, lw...)
			ws = append(ws, brk)
			plain = append(plain, lp...)
			plain = append(plain, false)
			for range len(lw) + 1 {
				at = append(at, i)
			}
		}
	}
	return ws, at, plain
}

// telling gives the hashes of the shingles of the page that have Telling or more plain words.
func telling(ws []string, plain []bool) map[uint64]bool {
	out := map[uint64]bool{}
	run := 0
	for i, w := range ws {
		if w == brk {
			run = 0
			continue
		}
		run++
		if run < Shingle {
			continue
		}
		n := 0
		for j := i - Shingle + 1; j <= i; j++ {
			if plain[j] {
				n++
			}
		}
		if n >= Telling {
			out[hash(ws[i-Shingle+1:i+1])] = true
		}
	}
	return out
}

// Run is a run of words of the page that a source has too, as the page writes it, with the line where it starts.
type Run struct {
	Line int    `json:"line"`
	Text string `json:"text"`
}

// runs gives the longest runs of words of the page that the shingles of the source cover, of the shingles of the
// page in sh.
func runs(sec Section, ws []string, at []int, sh, src map[uint64]bool) []Run {
	cover := make([]bool, len(ws))
	n := 0
	for i, w := range ws {
		if w == brk {
			n = 0
			continue
		}
		n++
		if n < Shingle {
			continue
		}
		if h := hash(ws[i-Shingle+1 : i+1]); sh[h] && src[h] {
			for j := i - Shingle + 1; j <= i; j++ {
				cover[j] = true
			}
		}
	}
	var out []Run
	for i := 0; i < len(ws); {
		if !cover[i] {
			i++
			continue
		}
		j := i
		for j < len(ws) && cover[j] {
			j++
		}
		out = append(out, Run{Line: sec.Line + at[i], Text: strings.Join(ws[i:j], " ")})
		i = j
	}
	return out
}

// Sections splits a body into the lead and its sections at the headings of level 2, leaving out the code fences.
// line is the line of the file where the body starts.
func Sections(file, body string, line int) []Section {
	var out []Section
	cur := Section{Name: "lead", File: file, Line: line}
	var b strings.Builder
	fence := false
	push := func() {
		if strings.TrimSpace(b.String()) != "" {
			cur.Text = b.String()
			out = append(out, cur)
		}
		b.Reset()
	}
	for i, l := range strings.Split(body, "\n") {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
			fence = !fence
			b.WriteString("\n")
			continue
		}
		if !fence && strings.HasPrefix(l, "## ") {
			push()
			cur = Section{Name: strings.TrimSpace(l[3:]), File: file, Line: line + i + 1}
			continue
		}
		if fence {
			l = ""
		}
		b.WriteString(l)
		b.WriteString("\n")
	}
	push()
	return out
}

// sentence is a sentence of a section, one line of the page (6.2), with its content words.
type sentence struct {
	line  int
	words map[string]bool
}

// sentences gives the sentences of a section that have CloseMin content words or more.
func sentences(s Section) []sentence {
	var out []sentence
	for i, l := range strings.Split(s.Text, "\n") {
		if w := content(words(clean(l))); len(w) >= CloseMin {
			out = append(out, sentence{line: s.Line + i, words: w})
		}
	}
	return out
}

// ref is a source with its shingles and its sentences, indexed by content word.
type ref struct {
	src   Source
	shing map[uint64]bool
	sents []map[string]bool
	index map[string][]int
}

var sentenceEnd = regexp.MustCompile(`[.?!]["”')\]]?\s+`)

func newRef(s Source) *ref {
	t := sourceText(s.Text)
	r := &ref{src: s, shing: shingles(words(t)), index: map[string][]int{}}
	for _, para := range strings.Split(t, "\n\n") {
		para = strings.Join(strings.Fields(para), " ")
		for _, sn := range sentenceEnd.Split(para, -1) {
			w := content(words(sn))
			if len(w) < 3 {
				continue
			}
			k := len(r.sents)
			r.sents = append(r.sents, w)
			for x := range w {
				r.index[x] = append(r.index[x], k)
			}
		}
	}
	return r
}

// close gives the sentences of the source that are close to a sentence of the page, in their order, with their
// closeness.
func (r *ref) close(s sentence) (ids []int, shares []float64) {
	count := map[int]int{}
	for w := range s.words {
		for _, k := range r.index[w] {
			count[k]++
		}
	}
	for k, n := range count {
		if share := float64(n) / float64(len(s.words)); share >= CloseShare {
			ids = append(ids, k)
		}
	}
	slices.Sort(ids)
	for _, k := range ids {
		shares = append(shares, float64(count[k])/float64(len(s.words)))
	}
	return ids, shares
}

// Prose runs the shingle and the alignment tests on the sections against the sources. The findings come in the order
// of the sections, then of the sources.
func Prose(sections []Section, sources []Source) []Finding {
	refs := make([]*ref, len(sources))
	for i, s := range sources {
		refs[i] = newRef(s)
	}
	var out []Finding
	for _, sec := range sections {
		ws, at, plain := lines(sec.Text)
		sh := telling(ws, plain)
		sents := sentences(sec)
		for _, r := range refs {
			shared := 0
			for h := range sh {
				if r.shing[h] {
					shared++
				}
			}
			if shared >= MinShared && float64(shared) > SectionShare*float64(len(sh)) {
				out = append(out, Finding{Test: "shingles", File: sec.File, Line: sec.Line, Section: sec.Name, Source: r.src.Name, URI: r.src.URI, Share: float64(shared) / float64(len(sh)), Shared: shared, Of: len(sh), Runs: runs(sec, ws, at, sh, r.shing)})
			}
			out = append(out, r.aligned(sec, sents)...)
		}
	}
	return out
}

// aligned finds the runs of AlignRun or more consecutive sentences of a section that are close to sentences of the
// source in the same order. A block of sentences that are each close to some sentence of the source gives one finding,
// for its longest run.
func (r *ref) aligned(sec Section, sents []sentence) []Finding {
	type link struct {
		at, run int     // the sentence of the source, and the length of the run that ends with it
		least   float64 // the least closeness of the run
	}
	var out []Finding
	var prev []link
	best, end := link{}, 0
	flush := func() {
		if best.run >= AlignRun {
			out = append(out, Finding{Test: "aligned", File: sec.File, Line: sents[end-best.run+1].line, End: sents[end].line, Section: sec.Name, Source: r.src.Name, URI: r.src.URI, Share: best.least, Shared: best.run})
		}
		best = link{}
	}
	for i, s := range sents {
		ids, shares := r.close(s)
		cur := make([]link, 0, len(ids))
		for j, k := range ids {
			l := link{at: k, run: 1, least: shares[j]}
			for _, p := range prev {
				if p.at < k && k-p.at <= AlignGap+1 && p.run+1 > l.run {
					l = link{at: k, run: p.run + 1, least: min(p.least, shares[j])}
				}
			}
			cur = append(cur, l)
		}
		if len(cur) == 0 {
			flush()
		}
		for _, l := range cur {
			if l.run > best.run || l.run == best.run && l.least > best.least {
				best, end = l, i
			}
		}
		prev = cur
	}
	flush()
	return out
}

// Sort puts findings in the order of their file, line, test and source.
func Sort(fs []Finding) {
	slices.SortStableFunc(fs, func(a, b Finding) int {
		return cmp.Or(cmp.Compare(a.File, b.File), cmp.Compare(a.Line, b.Line), cmp.Compare(a.Test, b.Test), cmp.Compare(a.Source, b.Source))
	})
}
