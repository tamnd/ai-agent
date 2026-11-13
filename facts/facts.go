// Package facts reads the fact records of a node (spec 04.9), computes the status of each fact, and lists the required facts of each kind (spec 10.11).
//
// A number that changes, or that sources disagree on, is never a bare field. It is a fact with a value, a unit, its sources and, when it is true only at a date, an as_of date. The build computes the status of a fact. Nobody writes it (rule T05).
package facts

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"
)

// File is the name of the fact file in a node directory.
const File = "facts.yaml"

// The kinds of a source. A primary source is the developer's paper, model card, documentation or official text. A secondary source is a database or a report that cites one, such as Epoch AI or the AI Index. An estimate is a value that an estimator computes when no primary source gives it.
const (
	Primary   = "primary"
	Secondary = "secondary"
	Estimate  = "estimate"
)

// The computed statuses of a fact (spec 04.9).
const (
	Agreed        = "agreed"         // a primary source and at least one other source give the same value
	PrimaryOnly   = "primary-only"   // only primary sources give the value
	SecondaryOnly = "secondary-only" // only secondary sources give the value
	Disputed      = "disputed"       // the sources differ beyond the tolerance
	Estimated     = "estimated"      // no primary source gives the value, and an estimator computes it
)

// Source is one source of a fact.
type Source struct {
	Src       string `yaml:"src" json:"src"`
	Loc       string `yaml:"loc,omitempty" json:"loc,omitempty"`
	Row       string `yaml:"row,omitempty" json:"row,omitempty"`
	Value     any    `yaml:"value" json:"value"`
	Kind      string `yaml:"kind" json:"kind"`
	Method    string `yaml:"method,omitempty" json:"method,omitempty"`
	Estimator string `yaml:"estimator,omitempty" json:"estimator,omitempty"`
	AsOf      string `yaml:"as_of,omitempty" json:"as_of,omitempty"`
}

// Fact is one fact record.
type Fact struct {
	ID    string `yaml:"id" json:"id"`
	Value any    `yaml:"value,omitempty" json:"value,omitempty"`
	Unit  string `yaml:"unit,omitempty" json:"unit,omitempty"`
	AsOf  string `yaml:"as_of,omitempty" json:"as_of,omitempty"`
	// NoPrimary says why no primary source exists, so rule D04 accepts a fact with no primary source.
	NoPrimary string   `yaml:"no_primary,omitempty" json:"no_primary,omitempty"`
	Note      string   `yaml:"note,omitempty" json:"note,omitempty"`
	Sources   []Source `yaml:"sources" json:"sources"`
	// Written holds a status that somebody wrote in the file. Rule T05 rejects it.
	Written string `yaml:"status,omitempty" json:"-"`
	// Status is computed by Compute.
	Status string `yaml:"-" json:"status"`
}

// Read reads the fact file of a node directory. It returns nil and no error when the file does not exist.
func Read(dir string) ([]Fact, error) {
	b, err := os.ReadFile(filepath.Join(dir, File))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Fact
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(&out); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%s: %w", File, err)
	}
	for i := range out {
		f := &out[i]
		f.Value = norm(f.Value)
		for j := range f.Sources {
			f.Sources[j].Value = norm(f.Sources[j].Value)
		}
		Compute(f)
	}
	return out, nil
}

// norm turns the values that YAML decodes into types that compare well: a timestamp becomes its date, and an integer becomes a float.
func norm(v any) any {
	switch x := v.(type) {
	case time.Time:
		if x.Hour() == 0 && x.Minute() == 0 && x.Second() == 0 {
			return x.Format("2006-01-02")
		}
		return x.Format(time.RFC3339)
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case uint64:
		return float64(x)
	case []any:
		out := make([]any, len(x))
		for i := range x {
			out[i] = norm(x[i])
		}
		return out
	}
	return v
}

// Compute sets the status of f from its sources, and its value from the best source when the file gives none.
func Compute(f *Fact) {
	f.Status = ""
	if len(f.Sources) == 0 {
		return
	}
	best := Best(f)
	if f.Value == nil && best != nil {
		f.Value = best.Value
	}
	kinds := map[string]int{}
	for _, s := range f.Sources {
		kinds[s.Kind]++
		if s.Value != nil && !Same(s.Value, f.Value) {
			f.Status = Disputed
		}
	}
	if f.Status == Disputed {
		return
	}
	switch {
	case kinds[Primary] > 0 && len(f.Sources) > kinds[Primary]:
		f.Status = Agreed
	case kinds[Primary] > 0:
		f.Status = PrimaryOnly
	case kinds[Estimate] > 0:
		f.Status = Estimated
	default:
		f.Status = SecondaryOnly
	}
}

// Best returns the source that wins (spec 04.9): the first primary source, else the first estimate, else the first source.
func Best(f *Fact) *Source {
	for _, k := range []string{Primary, Estimate, Secondary} {
		for i := range f.Sources {
			if f.Sources[i].Kind == k && f.Sources[i].Value != nil {
				return &f.Sources[i]
			}
		}
	}
	return nil
}

// Tolerance is the relative difference under which two numbers are the same value (spec 04.9: 1% for counts).
const Tolerance = 0.01

// Same tells if two values are the same within the tolerance. Numbers agree within 1%. Dates agree at the precision of the less precise one, so 2024-07 and 2024-07-23 agree. Text agrees without regard to case. Lists agree when they have the same items in any order.
func Same(a, b any) bool {
	if x, ok := number(a); ok {
		y, ok := number(b)
		if !ok {
			return false
		}
		if x == y {
			return true
		}
		return math.Abs(x-y) <= Tolerance*math.Max(math.Abs(x), math.Abs(y))
	}
	if la, ok := a.([]any); ok {
		lb, ok := b.([]any)
		if !ok || len(la) != len(lb) {
			return false
		}
		used := make([]bool, len(lb))
		for _, x := range la {
			found := false
			for j, y := range lb {
				if !used[j] && Same(x, y) {
					used[j], found = true, true
					break
				}
			}
			if !found {
				return false
			}
		}
		return true
	}
	sa, sb := fmt.Sprint(a), fmt.Sprint(b)
	if IsDate(sa) && IsDate(sb) {
		n := min(len(sa), len(sb))
		return sa[:n] == sb[:n]
	}
	return strings.EqualFold(strings.TrimSpace(sa), strings.TrimSpace(sb))
}

func number(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case int:
		return float64(x), true
	}
	return 0, false
}

// IsNumber tells if a value is a number.
func IsNumber(v any) bool {
	_, ok := number(v)
	return ok
}

var dateRe = regexp.MustCompile(`^\d{4}(-\d{2}(-\d{2})?)?$`)

// IsDate tells if s is a date of the form YYYY, YYYY-MM or YYYY-MM-DD.
func IsDate(s string) bool {
	return dateRe.MatchString(s)
}

var idRe = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z0-9_]+)*$`)

// ValidID tells if id has the form of a fact id, such as params.total or training.compute_flop.
func ValidID(id string) bool {
	return idRe.MatchString(id)
}

// Units are the units of schema/units.yaml (rule D09). A number has one of them. Text, dates and lists have none.
var Units = []string{"parameters", "tokens", "FLOP", "FLOP/s", "bytes", "bytes/s", "items", "W", "USD", "seconds", "votes"}

// Required lists the required facts of each fact kind (spec 10.11). A required id is present when a fact has the id or the id is a prefix of its id, so peak_flops.fp16 gives peak_flops.
var Required = map[string][]string{
	"model":     {"developer", "released", "architecture", "params.total", "access", "licence", "modalities"},
	"dataset":   {"size.items", "modality", "source_type", "collected", "licence", "access", "docs"},
	"benchmark": {"released", "size.items", "splits", "metric", "protocol", "licence", "data_hash"},
	"hardware":  {"vendor", "released", "type", "peak_flops", "memory.bytes", "memory.bandwidth", "power"},
	"policy":    {"jurisdiction", "type", "adopted", "in_force", "text"},
	"system":    {"developer", "years", "domain", "status"},
}

// Kinds are the kinds that carry facts and have the D ladder, in a stable order.
var Kinds = func() []string {
	out := make([]string, 0, len(Required))
	for k := range Required {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}()

// HasFacts tells if a kind carries facts and has the D ladder.
func HasFacts(kind string) bool {
	_, ok := Required[kind]
	return ok
}

// Covers tells if the fact id gives the required id.
func Covers(id, required string) bool {
	return id == required || strings.HasPrefix(id, required+".")
}

// Missing returns the required facts of the kind that fs does not give, in the order of Required.
func Missing(kind string, fs []Fact) []string {
	var out []string
	for _, r := range Required[kind] {
		if !slices.ContainsFunc(fs, func(f Fact) bool { return Covers(f.ID, r) }) {
			out = append(out, r)
		}
	}
	return out
}

// timeBound are the fact ids that are true only at a date (rule D07). A fact with one of these ids, or with an id under one of them, needs as_of.
var timeBound = []string{"access", "licence", "status", "price", "users", "downloads", "likes", "stars", "rank"}

// TimeBound tells if the fact is true only at a date, so it needs as_of.
func TimeBound(id string) bool {
	return slices.ContainsFunc(timeBound, func(t string) bool { return Covers(id, t) })
}

// AsOf returns the date of a fact: its own as_of, or else the latest as_of of its sources.
func AsOf(f Fact) string {
	if f.AsOf != "" {
		return f.AsOf
	}
	latest := ""
	for _, s := range f.Sources {
		if s.AsOf > latest {
			latest = s.AsOf
		}
	}
	return latest
}

// Format writes a value for a reader: large numbers in words or in powers of ten, lists with commas.
func Format(v any, unit string) string {
	if x, ok := number(v); ok {
		s := formatNumber(x)
		if unit != "" {
			s += " " + unit
		}
		return s
	}
	if l, ok := v.([]any); ok {
		parts := make([]string, len(l))
		for i, x := range l {
			parts[i] = Format(x, "")
		}
		return strings.Join(parts, ", ")
	}
	return fmt.Sprint(v)
}

func formatNumber(x float64) string {
	a := math.Abs(x)
	switch {
	case a >= 1e15:
		e := math.Floor(math.Log10(a))
		m := x / math.Pow(10, e)
		return strconv.FormatFloat(m, 'g', 3, 64) + " × 10^" + strconv.Itoa(int(e))
	case a >= 1e12:
		return strconv.FormatFloat(x/1e12, 'g', 4, 64) + " trillion"
	case a >= 1e9:
		return strconv.FormatFloat(x/1e9, 'g', 4, 64) + " billion"
	case a >= 1e6:
		return strconv.FormatFloat(x/1e6, 'g', 4, 64) + " million"
	case a == math.Trunc(a):
		return group(strconv.FormatFloat(x, 'f', 0, 64))
	}
	return strconv.FormatFloat(x, 'g', 6, 64)
}

// group puts commas between groups of three digits.
func group(s string) string {
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}

// labels are the names of the common fact ids.
var labels = map[string]string{ //nolint:gosec // G101 reads the fact id training.tokens as a credential; these are display names.
	"developer": "Developer", "released": "Release date", "architecture": "Architecture",
	"params.total": "Parameters (total)", "params.active": "Parameters (active)",
	"access": "Access", "licence": "Licence", "modalities": "Modalities", "modality": "Modality",
	"training.compute_flop": "Training compute", "training.tokens": "Training tokens",
	"context_length": "Context length", "training.data": "Training data",
	"size.items": "Size (items)", "size.tokens": "Size (tokens)", "size.bytes": "Size (bytes)",
	"source_type": "Source type", "collected": "Collection dates", "docs": "Documentation",
	"splits": "Splits", "metric": "Metric", "protocol": "Protocol", "data_hash": "Data hash",
	"vendor": "Vendor", "type": "Type", "peak_flops": "Peak FLOP/s", "memory.bytes": "Memory",
	"memory.bandwidth": "Memory bandwidth", "power": "Power",
	"jurisdiction": "Jurisdiction", "adopted": "Adopted", "in_force": "In force", "text": "Official text",
	"years": "Years", "domain": "Domain", "status": "Status",
}

// Label turns a fact id into a name for a reader. A common id has a fixed name, such as "Parameters (total)" for params.total. A sub-id of a common id adds its last part, so peak_flops.fp16 becomes "Peak FLOP/s (fp16)". Another id becomes words: context_window becomes "Context window".
func Label(id string) string {
	if l, ok := labels[id]; ok {
		return l
	}
	if i := strings.LastIndexByte(id, '.'); i > 0 {
		if l, ok := labels[id[:i]]; ok {
			return l + " (" + strings.ReplaceAll(id[i+1:], "_", " ") + ")"
		}
	}
	s := strings.NewReplacer(".", " ", "_", " ").Replace(id)
	r, n := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError {
		return s
	}
	return string(unicode.ToUpper(r)) + s[n:]
}
