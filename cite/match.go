// SPDX-License-Identifier: Apache-2.0

package cite

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

// Norm writes text in the form that the checks compare: lower case, one space between words, and the dashes and
// minus signs of PDFs as "-".
func Norm(s string) string {
	s = strings.NewReplacer("−", "-", "–", "-", "—", "-", "­", "", "’", "'", "“", `"`, "”", `"`).Replace(s)
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}

// Number is a number of a claim or of a source, with the precision that it is written at.
type Number struct {
	Text  string  `json:"text"`
	Value float64 `json:"-"`
	Step  float64 `json:"-"` // the place of its last digit: 0.1 for "4.1", 1e6 for "65 million"
}

var (
	numRE = regexp.MustCompile(`(?i)(^|[^\w.])((?:[0-9]{1,3}(?:,[0-9]{3})+|[0-9]+)(?:\.[0-9]+)?|\.[0-9]+)(\s*(?:%|percent\b|per cent\b|thousand\b|million\b|billion\b|trillion\b|[kmbt]\b))?`)
	// mathRE finds inline math. Its numbers are read, but not its exponents and subscripts, which are symbols.
	mathRE = regexp.MustCompile(`\$[^$]*\$`)
	supRE  = regexp.MustCompile(`[_^](\{[^}]*\}|.)`)
)

var scales = map[string]float64{"%": 0.01, "percent": 0.01, "per cent": 0.01, "k": 1e3, "thousand": 1e3, "m": 1e6, "million": 1e6, "b": 1e9, "billion": 1e9, "t": 1e12, "trillion": 1e12}

// Numbers finds the numbers of a text with their precision. "28.4", "65 million", "4.5%" and "1,024" are numbers;
// the exponents and subscripts of inline math are not.
func Numbers(s string) []Number {
	s = mathRE.ReplaceAllStringFunc(s, func(m string) string {
		m = supRE.ReplaceAllString(m, " ")
		return strings.Trim(m, "$")
	})
	var out []Number
	for _, m := range numRE.FindAllStringSubmatch(s, -1) {
		digits := strings.ReplaceAll(m[2], ",", "")
		v, err := strconv.ParseFloat(digits, 64)
		if err != nil {
			continue
		}
		step := 1.0
		if i := strings.IndexByte(digits, '.'); i >= 0 {
			step = math.Pow(10, -float64(len(digits)-i-1))
		}
		scale, unit := 1.0, ""
		if w := strings.ToLower(strings.TrimSpace(m[3])); w != "" {
			// "1.5 M" and "1.5M" are a million; "M" after a big number is a unit, and only a lower-case k or a capital
			// M, B or T after a number counts, so that "5 m" (metres) is not read as five million.
			if raw := strings.TrimSpace(m[3]); len(raw) > 1 || raw == "%" || raw == "k" || raw == "M" || raw == "B" || raw == "T" {
				scale, unit = scales[w], m[3]
			}
		}
		out = append(out, Number{Text: strings.TrimSpace(m[2] + unit), Value: v * scale, Step: step * scale})
	}
	return out
}

// Found tells if the number n of a claim is in the numbers of a source: a number of the source that rounds to n at
// the precision of n. The page may round the source's number, as 4.1 for 4.08, but not state it more exactly.
func Found(n Number, in []Number) bool {
	for _, x := range in {
		if x.Step > n.Step*1.0000001 {
			continue // the source states it less exactly than the claim
		}
		if math.Abs(math.Round(x.Value/n.Step)*n.Step-n.Value) <= n.Step*1e-6 {
			return true
		}
	}
	return false
}

// stop are the words that a claim and its source share whatever they say.
var stop = map[string]bool{}

func init() {
	for _, w := range strings.Fields(`a about above after again against all also although among an and any are as at be because been
		before being below between both but by can could did do does doing done down during each either else even
		ever every few for from further had has have having he her here hers him his how however i if in into is it its
		itself just less like made make makes many may might more most much must my neither no nor not now of off often
		on once one only onto or other others our out over own per rather same she should since so some such than that
		the their theirs them then there these they this those though three through thus to too two under until up upon
		us use used uses using very via was we well were what when where whether which while who whom whose why will
		with within without would yet you your first second third paper papers authors author work works show shows
		shown showed report reports reported reporting state states stated find finds found section table figure page
		equation eq et al called call calls name named way ways part parts give gives given take takes taken new`) {
		stop[w] = true
	}
}

var wordRE = regexp.MustCompile(`[\p{L}][\p{L}'-]*`)

// Terms gives the content words of a claim, outside its math, each once and in order. The words of exempt, as the
// names of the authors of the cited work, are not key terms: "Srivastava et al. recommend" names the source.
func Terms(claim string, exempt ...string) []string {
	ex := map[string]bool{}
	for _, e := range exempt {
		for _, w := range wordRE.FindAllString(strings.ToLower(e), -1) {
			ex[stem(w)] = true
		}
	}
	return terms(claim, ex)
}

func terms(claim string, exempt map[string]bool) []string {
	claim = mathRE.ReplaceAllString(claim, " ")
	seen := map[string]bool{}
	var out []string
	for _, w := range wordRE.FindAllString(strings.ToLower(claim), -1) {
		for _, p := range strings.Split(strings.Trim(w, "'-"), "-") {
			p = strings.TrimSuffix(p, "'s")
			if len([]rune(p)) < 3 || stop[p] || stop[stem(p)] || exempt[stem(p)] || seen[stem(p)] {
				continue
			}
			seen[stem(p)] = true
			out = append(out, p)
		}
	}
	return out
}

// stem cuts the common endings of English words, so that "regularizes", "regularized" and "regularizing" meet.
func stem(w string) string {
	for _, suf := range []string{"izations", "ization", "isations", "isation", "ational", "nesses", "ments", "ings", "ness", "ment", "ies", "ing", "ers", "ed", "es", "er", "ly", "s"} {
		if !strings.HasSuffix(w, suf) || len(w)-len(suf) < 3 {
			continue
		}
		rest := strings.TrimSuffix(w, suf)
		switch {
		case strings.HasPrefix(suf, "iz"):
			rest += "iz" // "normalization", "normalized" and "normalizing" meet at "normaliz"
		case strings.HasPrefix(suf, "is"):
			rest += "is"
		case suf == "ies":
			rest += "y"
		case suf == "es" && !strings.HasSuffix(rest, "s") && !strings.HasSuffix(rest, "x") && !strings.HasSuffix(rest, "z") && !strings.HasSuffix(rest, "ch") && !strings.HasSuffix(rest, "sh"):
			rest += "e" // "pages" is "page", but "boxes" is "box"
		}
		return rest
	}
	return w
}

// Vocab is the set of word stems of a text.
type Vocab map[string]bool

// NewVocab reads the words of a text.
func NewVocab(text string) Vocab {
	v := Vocab{}
	for _, w := range wordRE.FindAllString(strings.ToLower(text), -1) {
		for _, p := range strings.Split(strings.Trim(w, "'-"), "-") {
			if p != "" {
				v[stem(p)] = true
			}
		}
	}
	return v
}

// Has tells if the text holds the word, by its stem, or by a shared start of seven letters for long words
// ("normalization" and "normalizing").
func (v Vocab) Has(w string) bool {
	s := stem(w)
	if v[s] {
		return true
	}
	if r := []rune(s); len(r) >= 8 {
		pre := string(r[:7])
		for k := range v {
			if strings.HasPrefix(k, pre) {
				return true
			}
		}
	}
	return false
}
