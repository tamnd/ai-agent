// SPDX-License-Identifier: Apache-2.0

package check

import (
	"regexp"
	"strings"

	"github.com/tamnd/ai-agent/node"
)

// rankWords are the words of rule K04 (spec 6.7). A page states a rank or a superlative about models only through a computed fact with a date.
var rankWords = []struct {
	re   *regexp.Regexp
	hint string
}{
	{regexp.MustCompile(`(?i)\bstate[- ]of[- ]the[- ]art\b`), `"state of the art"`},
	{regexp.MustCompile(`\bSOTA\b`), `"SOTA"`},
	{regexp.MustCompile(`(?i)\bcurrently\b`), `"currently"`},
	{regexp.MustCompile(`(?i)\b(?:the|a|an)\s+(?:largest|biggest|best|most\s+capable|most\s+powerful|most\s+advanced|strongest|top|leading)(?:[- ][\w-]+){0,3}?\s+(?:models?|LLMs?|labs?|laborator(?:y|ies)|compan(?:y|ies)|systems?|datasets?|corpus|corpora|benchmarks?|scores?|chatbots?|networks?|AIs?)\b`), "a rank or a superlative about a model, a lab, a score or a dataset"},
	{regexp.MustCompile(`(?i)\bleading\s+(?:models?|LLMs?|labs?|laborator(?:y|ies)|compan(?:y|ies)|systems?|AI)\b`), `"leading"`},
	{regexp.MustCompile(`(?i)\bfrontier\b`), `"frontier" as a rank; write [[frontier-model|frontier model]] for the defined term`},
}

var (
	voiceLinkRE  = regexp.MustCompile(`\[\[[^\]]*\]\]|\[[^\]]*\]\([^)]*\)`)
	voiceQuoteRE = regexp.MustCompile(`"[^"]*"|“[^”]*”`)
	voiceMathRE  = regexp.MustCompile(`\$\$[^$]*\$\$|\$[^$]*\$`)
	frontierOK   = regexp.MustCompile(`(?i)\b(?:pareto|efficient|production[- ]possibility|search|wave)[- ]frontier\b`)
)

// voice checks the prose of the site voice for the words of rule K04 (spec 6.7 and 07.11). It skips code, math, quotes and block quotes, and the text of links, so that a link to the defined term "frontier model" passes.
func (r *run) voice(n *node.Node) {
	p := n.Rel + "/node.md"
	fence := ""
	for i, l := range strings.Split(n.Body, "\n") {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
			if fence == "" {
				fence = t[:3]
			} else if strings.HasPrefix(t, fence) {
				fence = ""
			}
			continue
		}
		if fence != "" || strings.HasPrefix(t, ">") || strings.HasPrefix(t, "::") {
			continue
		}
		l = codeSpanRE.ReplaceAllString(l, " ")
		l = voiceMathRE.ReplaceAllString(l, " ")
		l = voiceQuoteRE.ReplaceAllString(l, " ")
		l = voiceLinkRE.ReplaceAllStringFunc(l, linkText)
		l = frontierOK.ReplaceAllString(l, " ")
		for _, w := range rankWords {
			if w.re.MatchString(l) {
				r.add("K04", Level, p, n.BodyLine+i, "the site voice does not use %s; a rank comes from a computed fact with a date (6.7)", w.hint)
				break
			}
		}
	}
}

var codeSpanRE = regexp.MustCompile("`[^`]*`")

// linkText keeps the text of a link, except the defined term "frontier model", which K04 allows in a link.
func linkText(m string) string {
	text := m
	switch {
	case strings.HasPrefix(m, "[["):
		text = strings.TrimSuffix(strings.TrimPrefix(m, "[["), "]]")
		if _, after, ok := strings.Cut(text, "|"); ok {
			text = after
		} else {
			text = strings.ReplaceAll(text, "-", " ")
		}
	default:
		text = m[1:strings.Index(m, "](")]
	}
	if strings.Contains(strings.ToLower(text), "frontier model") {
		return " "
	}
	return text
}
