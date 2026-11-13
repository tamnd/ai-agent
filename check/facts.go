package check

import (
	"slices"
	"strings"

	"github.com/tamnd/ai-agent/facts"
	"github.com/tamnd/ai-agent/node"
)

// facts checks the fact records of a node against the rules D of spec 10.13 and the rules T01 and T05.
func (r *run) facts(n *node.Node) {
	p := n.Rel + "/" + facts.File
	seen := map[string]bool{}
	for _, f := range n.Facts {
		switch {
		case f.ID == "":
			r.add("T01", Block, p, 0, "a fact has no id")
			continue
		case !facts.ValidID(f.ID):
			r.add("T01", Block, p, 0, "the fact id %q is not lower case words joined by dots and underscores, such as params.total", f.ID)
		case seen[f.ID]:
			r.add("T01", Block, p, 0, "the fact %q is in the file more than once", f.ID)
		}
		seen[f.ID] = true
		if f.Written != "" {
			r.add("T05", Block, p, 0, "the fact %q has the status %q; the build computes the status (04.9)", f.ID, f.Written)
		}
		r.factSources(p, f)
		if b := facts.Best(&f); b != nil && f.Value != nil && !facts.Same(f.Value, b.Value) {
			r.add("D01", Block, p, 0, "the fact %q has the value %v, but its best source gives %v", f.ID, f.Value, b.Value)
		}
		if facts.IsNumber(f.Value) {
			switch {
			case f.Unit == "":
				r.add("D01", Block, p, 0, "the fact %q is a number with no unit", f.ID)
			case !slices.Contains(facts.Units, f.Unit):
				r.add("D09", Block, p, 0, "the fact %q has the unit %q; the units are %s", f.ID, f.Unit, strings.Join(facts.Units, ", "))
			}
		}
		if f.Status == facts.Disputed {
			r.add("D05", Warn, p, 0, "the sources of the fact %q give different values; the page shows each value with its source", f.ID)
		}
		if facts.TimeBound(f.ID) && f.AsOf == "" {
			r.add("D07", Block, p, 0, "the fact %q is true only at a date, so it needs as_of", f.ID)
		}
		if f.AsOf != "" && !facts.IsDate(f.AsOf) {
			r.add("T01", Block, p, 0, "the as_of of the fact %q is %q; write a date such as 2026-11-13", f.ID, f.AsOf)
		}
	}
	if !facts.HasFacts(n.Kind) {
		return
	}
	if missing := facts.Missing(n.Kind, n.Facts); len(missing) > 0 {
		r.add("D03", Level, p, 0, "a %s needs the facts %s (10.11)", n.Kind, strings.Join(missing, ", "))
	}
	if len(n.Sources) == 0 && len(n.Facts) == 0 {
		r.add("D02", Level, n.Rel+"/node.md", 0, "the %s has no source", n.Kind)
	}
}

func (r *run) factSources(p string, f facts.Fact) {
	if len(f.Sources) == 0 {
		r.add("D01", Block, p, 0, "the fact %q has no source", f.ID)
		return
	}
	primary := false
	for i, s := range f.Sources {
		switch s.Kind {
		case facts.Primary:
			primary = true
			if s.Loc == "" && s.Row == "" {
				r.add("D04", Level, p, 0, "the primary source %d of the fact %q has no locator (loc or row)", i+1, f.ID)
			}
		case facts.Secondary:
		case facts.Estimate:
			if s.Estimator == "" || s.Method == "" {
				r.add("D06", Block, p, 0, "the estimate of the fact %q names no estimator or no method", f.ID)
			}
		case "":
			r.add("T01", Block, p, 0, "the source %d of the fact %q has no kind; write primary, secondary or estimate", i+1, f.ID)
		default:
			r.add("T01", Block, p, 0, "the source %d of the fact %q has the kind %q; write primary, secondary or estimate", i+1, f.ID, s.Kind)
		}
		if s.Src == "" {
			r.add("D01", Block, p, 0, "the source %d of the fact %q has no src", i+1, f.ID)
		}
		if s.Value == nil {
			r.add("D01", Block, p, 0, "the source %d of the fact %q gives no value", i+1, f.ID)
		}
		if s.AsOf != "" && !facts.IsDate(s.AsOf) {
			r.add("T01", Block, p, 0, "the as_of of the source %d of the fact %q is %q; write a date such as 2026-11-13", i+1, f.ID, s.AsOf)
		}
	}
	if !primary && f.NoPrimary == "" && f.Status != facts.Estimated {
		r.add("D04", Level, p, 0, "the fact %q has no primary source; cite one, or say why none exists in no_primary", f.ID)
	}
}
