package status

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/tamnd/ai-agent/facts"
	"github.com/tamnd/ai-agent/node"
)

var dNames = []string{"a name and a source", "required facts", "primary sources", "reviewed facts"}

// Data computes the D ladder of a node whose kind carries facts (spec 10.11). It returns nil for the other kinds.
func Data(n *node.Node, env Env) *Rung {
	if !facts.HasFacts(n.Kind) {
		return nil
	}
	r := &Rung{Max: 3}
	defer func() { r.Label, r.Name = fmt.Sprintf("D%d", r.Level), dNames[r.Level] }()
	if len(n.Sources) == 0 && len(n.Facts) == 0 {
		r.Next = []string{"add a source to the front matter or a fact to " + facts.File + " (D02)"}
		return r
	}
	// D1: each required fact is present, and each fact has a source that gives its value.
	if missing := facts.Missing(n.Kind, n.Facts); len(missing) > 0 {
		r.Next = append(r.Next, "add the facts "+strings.Join(missing, ", ")+" to "+facts.File+" (D03)")
	}
	for _, f := range n.Facts {
		if facts.Best(&f) == nil {
			r.Next = append(r.Next, "give the fact "+f.ID+" a source with a value (D01)")
		}
		if facts.IsNumber(f.Value) && f.Unit == "" {
			r.Next = append(r.Next, "give the fact "+f.ID+" a unit (D01)")
		}
	}
	if len(r.Next) > 0 {
		return r
	}
	r.Level = 1
	// D2: each fact cites a primary source with a locator, or says why none exists, or is an estimate that names its estimator and method; each time-bound fact has as_of.
	for _, f := range n.Facts {
		if next := primaryNext(f); next != "" {
			r.Next = append(r.Next, next)
		}
		if facts.TimeBound(f.ID) && f.AsOf == "" {
			r.Next = append(r.Next, "give the fact "+f.ID+" an as_of date (D07)")
		}
	}
	if len(r.Next) > 0 {
		return r
	}
	r.Level = 2
	ok, why := Reviewed(env.root(), n.Tag, "facts", filepath.Join(n.Dir, facts.File))
	if !ok {
		r.Next = []string{"a reviewer checks the facts against the primary sources: " + why}
		return r
	}
	r.Level = 3
	return r
}

func primaryNext(f facts.Fact) string {
	if f.Status == facts.Estimated {
		for _, s := range f.Sources {
			if s.Kind == facts.Estimate && (s.Estimator == "" || s.Method == "") {
				return "name the estimator and the method of the fact " + f.ID + " (D06)"
			}
		}
		return ""
	}
	for _, s := range f.Sources {
		if s.Kind == facts.Primary {
			if s.Loc == "" && s.Row == "" {
				return "give the primary source of the fact " + f.ID + " a locator (D04)"
			}
			return ""
		}
	}
	if f.NoPrimary != "" {
		return ""
	}
	return "cite a primary source for the fact " + f.ID + ", or say why none exists in no_primary (D04)"
}

func (e Env) root() string {
	if e.Corpus == nil {
		return ""
	}
	return e.Corpus.Root
}
