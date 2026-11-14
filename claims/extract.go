// SPDX-License-Identifier: Apache-2.0

package claims

import (
	"regexp"
	"strings"
)

// The classes of a sentence of the extraction pass (07.6).
const (
	ClassClaim      = "claim"
	ClassDefinition = "definition-in-place"
	ClassStructure  = "structure"
	ClassQuote      = "quote"
)

// Sentence is one sentence of the prose of a node, as the cheap part of the extraction pass (07.6) sees it.
type Sentence struct {
	Line    int      `json:"line"`
	Text    string   `json:"text"`
	Class   string   `json:"class"`
	Anchors []string `json:"anchors,omitempty"`
	// Signals are the marks of a claim about the world that the sentence has: a year, a citation, a number outside
	// math, a reporting or comparing verb. An unanchored claim sentence with a signal is a candidate for rule K02.
	Signals []string `json:"signals,omitempty"`
}

var (
	// A sentence ends at a stop, then its anchors, then a space and a capital.
	sentEndRe  = regexp.MustCompile(`([.!?])((?:\s*\{#c-[^}\s]*\})*)\s+([A-Z"“(\[])`)
	abbrevRe   = regexp.MustCompile(`(?i)(?:\bet al|\be\.g|\bi\.e|\bvs|\bcf|\bfig|\beq|\bsec|\bno|\bvol|\bpp|\bch|\bapprox|\bca|\bdr|\bmr|\bms|\bst|\bjr|\bsr|\bu\.s|\b[A-Z])$`)
	headingRe  = regexp.MustCompile(`^#{1,6}\s`)
	listRe     = regexp.MustCompile(`^(?:[-*+]|\d+[.)])\s+`)
	mathRe     = regexp.MustCompile(`\$\$[^$]*\$\$|\$[^$]*\$`)
	codeRe     = regexp.MustCompile("`[^`]*`")
	linkRe     = regexp.MustCompile(`\[\[([^\]|]*)\|?([^\]]*)\]\]|\[([^\]]*)\]\([^)]*\)`)
	commentRe  = regexp.MustCompile(`<!--.*?-->`)
	boldRe     = regexp.MustCompile(`\*\*[^*]+\*\*`)
	structRe   = regexp.MustCompile(`(?i)^(?:this|the next|the following|the rest of this|the last|each|later) (?:page|section|sections|node|part|chapter|table|figure|list)\b|^(?:see|read|compare) |^(?:for (?:a|the) (?:proof|derivation|history|details?)\b.*\bsee\b)`)
	quotedRe   = regexp.MustCompile(`^["“].*["”][.,]?$`)
	yearRe     = regexp.MustCompile(`\b(?:1[6-9]\d\d|20\d\d)\b`)
	citeRe     = regexp.MustCompile(`\bet al\.|\(\d{4}[a-z]?\)`)
	numberRe   = regexp.MustCompile(`\b\d+(?:[.,]\d+)?\s*(?:%|percent|per cent|points?|×|x\b|times\b|fold\b|[kKMBT]\b|billion|million|thousand)|\b\d+(?:\.\d+)\b`)
	verbRe     = regexp.MustCompile(`(?i)\b(?:reports?|reported|finds?|found|shows?|showed|shown|demonstrat\w*|measur\w*|observ\w*|outperform\w*|improv\w*|reduc\w*|increas\w*|beats?|achiev\w*|surpass\w*|exceed\w*|faster|slower|better|worse|first|introduc\w*|propos\w*|invent\w*|discover\w*)\b`)
	anchorInRe = regexp.MustCompile(`\{#(c-[^}\s]*)\}`)
)

// Extract splits the prose of a node into sentences, from line first (the line of the body in node.md), and classes
// each one. It skips headings, code, display math, tables, directives and comments; the text of a link is kept.
func Extract(body string, first int) []Sentence {
	var out []Sentence
	fence, math, callout := "", false, false
	for i, l := range strings.Split(body, "\n") {
		t := strings.TrimSpace(l)
		switch {
		case strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~"):
			if fence == "" {
				fence = t[:3]
			} else if strings.HasPrefix(t, fence) {
				fence = ""
			}
			continue
		case fence != "":
			continue
		case math:
			// The display ends at the line with the closing $$, which may have a label after it.
			if strings.Contains(t, "$$") {
				math = false
			}
			continue
		case strings.HasPrefix(t, "$$"):
			// A display on one line, as $$ x $$ {#eq-x}, or the start of one.
			math = strings.Count(t, "$$") == 1
			continue
		}
		if t == "" {
			callout = false
			continue
		}
		if headingRe.MatchString(t) || strings.HasPrefix(t, "|") || strings.HasPrefix(t, "::") || strings.HasPrefix(t, "<") {
			continue
		}
		// A block quote is the voice of its source; a callout, which starts with a line such as > [!note], is the voice
		// of the site.
		quote := false
		if strings.HasPrefix(t, ">") {
			t = strings.TrimSpace(strings.TrimLeft(t, ">"))
			if strings.HasPrefix(t, "[!") {
				callout = true
				if _, rest, ok := strings.Cut(t, "]"); ok {
					t = strings.TrimSpace(rest)
				}
			}
			quote = !callout
		} else {
			callout = false
		}
		t = listRe.ReplaceAllString(t, "")
		t = commentRe.ReplaceAllString(t, "")
		for _, s := range split(t) {
			out = append(out, classify(s, first+i, quote))
		}
	}
	return out
}

// split cuts a line into sentences at a stop followed by a space and a capital, but not after an abbreviation or an
// initial, and not inside math or code.
func split(line string) []string {
	// Hide the stops inside math, code and links, so that they do not end a sentence.
	masked := []byte(line)
	for _, re := range []*regexp.Regexp{mathRe, codeRe, linkRe} {
		for _, m := range re.FindAllStringIndex(line, -1) {
			for j := m[0]; j < m[1]; j++ {
				if masked[j] == '.' || masked[j] == '!' || masked[j] == '?' {
					masked[j] = '_'
				}
			}
		}
	}
	var out []string
	start := 0
	for _, m := range sentEndRe.FindAllSubmatchIndex(masked, -1) {
		stop, end := m[2], m[5] // the stop, and the end of the anchors after it
		if abbrevRe.MatchString(line[start:stop]) {
			continue
		}
		if s := strings.TrimSpace(line[start:end]); s != "" {
			out = append(out, s)
		}
		start = m[6]
	}
	if s := strings.TrimSpace(line[start:]); s != "" {
		out = append(out, s)
	}
	return out
}

func classify(s string, line int, quote bool) Sentence {
	x := Sentence{Line: line, Text: s}
	for _, m := range anchorInRe.FindAllStringSubmatch(s, -1) {
		x.Anchors = append(x.Anchors, m[1])
	}
	plain := Plain(s)
	switch {
	case quote || quotedRe.MatchString(plain):
		x.Class = ClassQuote
	case len(x.Anchors) == 0 && structRe.MatchString(plain):
		x.Class = ClassStructure
	case len(x.Anchors) == 0 && boldRe.MatchString(s):
		x.Class = ClassDefinition
	default:
		x.Class = ClassClaim
	}
	if x.Class != ClassClaim {
		return x
	}
	// The signals are looked for outside math and code, where a number is notation.
	bare := codeRe.ReplaceAllString(mathRe.ReplaceAllString(s, " "), " ")
	bare = linkRe.ReplaceAllString(bare, "$2$3")
	if yearRe.MatchString(bare) {
		x.Signals = append(x.Signals, "year")
	}
	if citeRe.MatchString(bare) {
		x.Signals = append(x.Signals, "citation")
	}
	if numberRe.MatchString(yearRe.ReplaceAllString(bare, " ")) {
		x.Signals = append(x.Signals, "number")
	}
	if verbRe.MatchString(bare) {
		x.Signals = append(x.Signals, "verb")
	}
	return x
}

// Plain is the text of a sentence with the anchors and the markup of links, bold and code removed.
func Plain(s string) string {
	s = anchorInRe.ReplaceAllString(s, "")
	s = linkRe.ReplaceAllStringFunc(s, func(m string) string {
		sm := linkRe.FindStringSubmatch(m)
		switch {
		case sm[3] != "":
			return sm[3]
		case sm[2] != "":
			return sm[2]
		default:
			return strings.ReplaceAll(sm[1], "-", " ")
		}
	})
	s = strings.ReplaceAll(s, "**", "")
	s = strings.ReplaceAll(s, "`", "")
	return strings.TrimSpace(s)
}

// Candidate tells if an unanchored sentence is a claim sentence with a mark of a claim about the world, so that rule
// K02 asks for an anchor or the model pass decides it.
func (s Sentence) Candidate() bool {
	return s.Class == ClassClaim && len(s.Anchors) == 0 && len(s.Signals) > 0
}

// The phrases of rule K05 (07.11, 06.7): the prose does not state the strength of the evidence, says "shows" or
// "proves" only of a proved claim or a proof, and names the source of a self-report.
var (
	strengthRe = regexp.MustCompile(`(?i)\b(?:well[- ]established|(?:is|are|it is|it's) well[- ]known|well[- ]known that|widely (?:accepted|believed|known)|(?:it )?is (?:now )?known that|it is (?:a )?(?:fact|certain|clear) that|(?:has|have) been proven|(?:is|are) proven|proven to|beyond (?:doubt|question)|undeniabl[ey]|indisputabl[ey]|conclusively|definitively)\b`)
	// showsRe is "shows" or "demonstrates" with a clause, the verb of a finding; "shows a peak" or "a photograph shows" is
	// not one.
	showsRe = regexp.MustCompile(`(?i)\b(?:shows?|showed|shown|demonstrat(?:es|ed|e))\s+(?:[\w,'’-]+\s+){0,2}?(?:that|how)\b`)
	// whRe is a word of the gap that starts another clause, as in "shows what that changes".
	whRe     = regexp.MustCompile(`(?i)\b(?:what|which|where|when|why|who|no)\b`)
	provesRe = regexp.MustCompile(`(?i)\b(?:proves?|proved|proven)\b`)
	// displayRe is "shows" in the sense of displays, with a table, a figure or a part of the page as its subject.
	displayRe = regexp.MustCompile(`(?i)\b(?:table|figure|fig\.|chart|plot|graph|diagram|scene|page|fact|row|column|list|section|example|output|trace|log|map|panel|card|widget|code|listing|it|this)\s+(?:\w+\s+){0,2}?shows?\b`)
	proofRe   = regexp.MustCompile(`(?i)\b(?:proof|theorem|lemma|derivation|derives?|derived|corollary|induction|by construction|in theory|theoretically|analytically)\b`)
	quoteRe   = regexp.MustCompile(`"[^"]*"|“[^”]*”`)
	attribRe  = regexp.MustCompile(`(?i)\b(?:reports?|reported|according to|states?|stated|says?|said|claims?|claimed|announc\w*|writes?|wrote|describes?|described|estimates?|estimated|lists?|listed|publish\w*|its (?:paper|report|model card|blog|announcement))\b`)
)

// Phrase is a use of a phrase of rule K05 in a sentence.
type Phrase struct {
	Line int    `json:"line"`
	Text string `json:"text"`
	Msg  string `json:"msg"`
}

// Phrases finds the uses of the phrases of rule K05 in the sentences, with the claims of the node by id. A quote is the
// voice of its source, not of the site, so it is skipped.
func Phrases(ss []Sentence, byID map[string]*Claim) []Phrase {
	var out []Phrase
	for _, s := range ss {
		if s.Class == ClassQuote {
			continue
		}
		bare := Plain(codeRe.ReplaceAllString(mathRe.ReplaceAllString(s.Text, " "), " "))
		bare = quoteRe.ReplaceAllString(bare, " ")
		if m := strengthRe.FindString(bare); m != "" {
			out = append(out, Phrase{s.Line, s.Text, "the prose states the strength of the evidence with \"" + m + "\"; the label of the claim says it (07.11)"})
		}
		// A theorem may be shown or proved. With an anchor the type of the claim decides; with none, "shows that" is the
		// verb of a finding and "proves" is taken as the verb of a proof.
		// An attributive claim says who stated or proved a result, and a definitional one what a term means; neither is
		// a finding, so only an empirical claim makes "proves" wrong.
		exempt, empirical := len(s.Anchors) > 0, false
		for _, a := range s.Anchors {
			c := byID[a]
			if c == nil || c.Type != Mathematical && c.Type != Definitional {
				exempt = false
			}
			if c != nil && c.Type == Empirical {
				empirical = true
			}
		}
		if m := showsRe.FindString(bare); m != "" && !whRe.MatchString(strings.Join(strings.Fields(m)[1:], " ")) && !exempt && !proofRe.MatchString(bare) && !displayRe.MatchString(bare) {
			out = append(out, Phrase{s.Line, s.Text, "\"" + strings.Fields(m)[0] + "\" is for a proved claim or a proof; for a theorem write proves, and for empirical work reports, finds, measures or observes (06.7)"})
		} else if m := provesRe.FindString(bare); m != "" && empirical {
			out = append(out, Phrase{s.Line, s.Text, "\"" + m + "\" is for a proved claim, and the claim here is not mathematical; write reports, finds, measures or observes (06.7)"})
		}
		for _, a := range s.Anchors {
			if c := byID[a]; c != nil && c.SelfReported && !attribRe.MatchString(bare) {
				out = append(out, Phrase{s.Line, s.Text, "the claim " + a + " is a self-report, so the sentence names its source, as \"Meta reports\" or \"according to the technical report\" (07.10)"})
				break
			}
		}
	}
	return out
}
