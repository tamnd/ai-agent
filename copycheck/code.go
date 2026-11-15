// SPDX-License-Identifier: Apache-2.0

package copycheck

import (
	"regexp"
	"strings"
	"unicode"
)

// Idioms are the lines that every program of the kind has, which the code test leaves out (13.7): a training step,
// the skeleton of a module, the imports. A line counts when it matches one of them after its white space is trimmed.
var Idioms = []*regexp.Regexp{
	regexp.MustCompile(`^(import|from)\s`),
	regexp.MustCompile(`^using\s`),
	regexp.MustCompile(`^package\s`),
	regexp.MustCompile(`^\w+\.zero_grad\(\)$`),
	regexp.MustCompile(`^\w+\.backward\(\)$`),
	regexp.MustCompile(`^\w+\.step\(\)$`),
	regexp.MustCompile(`^class \w+\((nn|torch\.nn)\.Module\):$`),
	regexp.MustCompile(`^def __init__\(self[^)]*\):$`),
	regexp.MustCompile(`^super\(\)\.__init__\(\)$`),
	regexp.MustCompile(`^def forward\(self[^)]*\):$`),
	regexp.MustCompile(`^if __name__ == ["']__main__["']:$`),
	regexp.MustCompile(`^(model|self)\.(train|eval)\(\)$`),
	regexp.MustCompile(`^with torch\.no_grad\(\):$`),
}

// keywords are the words of the languages of 08.1 that the code test keeps; every other name becomes one token.
var keywords = map[string]bool{}

func init() {
	for _, w := range strings.Fields(`and as assert async await break case catch class const continue def default defer del
		do elif else end except false False finally fn for func function global go if impl import in is lambda let local
		loop match module mut nil None nonlocal not or pass pub raise range return self struct switch true True try type
		var where while with yield begin quote macro mutable abstract primitive export`) {
		keywords[w] = true
	}
}

// tokens gives the tokens of a file: a keyword or a sign as it is, any other name as "v", a number as "n" and a string
// as "s". It leaves out comments and the lines of the idioms.
func tokens(src string) []string {
	var out []string
	for _, line := range strings.Split(src, "\n") {
		t := strings.TrimSpace(line)
		if t == "" || idiom(t) {
			continue
		}
		rs := []rune(line)
		for i := 0; i < len(rs); {
			r := rs[i]
			switch {
			case unicode.IsSpace(r):
				i++
			case r == '#' || r == '/' && i+1 < len(rs) && rs[i+1] == '/':
				i = len(rs)
			case r == '"' || r == '\'' || r == '`':
				j := i + 1
				for j < len(rs) && rs[j] != r {
					if rs[j] == '\\' {
						j++
					}
					j++
				}
				out = append(out, "s")
				i = j + 1
			case unicode.IsDigit(r):
				j := i
				for j < len(rs) && (unicode.IsDigit(rs[j]) || unicode.IsLetter(rs[j]) || rs[j] == '.' || rs[j] == '_') {
					j++
				}
				out = append(out, "n")
				i = j
			case unicode.IsLetter(r) || r == '_':
				j := i
				for j < len(rs) && (unicode.IsLetter(rs[j]) || unicode.IsDigit(rs[j]) || rs[j] == '_') {
					j++
				}
				if w := string(rs[i:j]); keywords[w] {
					out = append(out, w)
				} else {
					out = append(out, "v")
				}
				i = j
			default:
				out = append(out, string(r))
				i++
			}
		}
	}
	return out
}

func idiom(line string) bool {
	for _, re := range Idioms {
		if re.MatchString(line) {
			return true
		}
	}
	return false
}

// Fingerprint gives the winnowed fingerprints of a file: the least hash of each window of CodeWindow k-grams of
// CodeGram tokens.
func Fingerprint(src string) map[uint64]bool {
	ts := tokens(src)
	var hs []uint64
	for i := 0; i+CodeGram <= len(ts); i++ {
		hs = append(hs, hash(ts[i:i+CodeGram]))
	}
	out := map[uint64]bool{}
	if len(hs) > 0 && len(hs) < CodeWindow {
		out[minOf(hs)] = true
	}
	for i := 0; i+CodeWindow <= len(hs); i++ {
		out[minOf(hs[i:i+CodeWindow])] = true
	}
	return out
}

func minOf(hs []uint64) uint64 {
	m := hs[0]
	for _, h := range hs[1:] {
		m = min(m, h)
	}
	return m
}

// File is a code file of a page, with its path relative to the content root.
type File struct {
	Path string
	Text string
}

// Code runs the code test on the files against the files of the corpus. A file with fewer than CodeMin fingerprints
// is not tested: it is too short to tell a copy from an idiom.
func Code(files []File, corpus []Source) []Finding {
	fps := make([]map[uint64]bool, len(corpus))
	for i, s := range corpus {
		fps[i] = Fingerprint(s.Text)
	}
	var out []Finding
	for _, f := range files {
		fp := Fingerprint(f.Text)
		if len(fp) < CodeMin {
			continue
		}
		for i, s := range corpus {
			shared := 0
			for h := range fp {
				if fps[i][h] {
					shared++
				}
			}
			if share := float64(shared) / float64(len(fp)); share >= CodeShare {
				out = append(out, Finding{Test: "code", File: f.Path, Line: 1, Source: s.Name, URI: s.URI, Share: share, Shared: shared, Of: len(fp)})
			}
		}
	}
	return out
}
