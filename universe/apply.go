// SPDX-License-Identifier: Apache-2.0

package universe

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/tamnd/ai-agent/card"
	"github.com/tamnd/ai-agent/harvest"
)

// own are the fields of a card that a harvested row never writes as a plain field.
var own = map[string]bool{"tag": true, "kind": true, "slug": true, "owner": true, "name": true, "role": true,
	"aliases": true, "same_as": true, "summary": true, "harvest": true, "rules": true}

func (s *shards) sync(j *job, rows map[string]*harvest.Row, rep *SyncReport) (*card.Line, string, error) {
	kind := j.kind()
	l := s.find(j)
	if o, ok := s.claimed[l]; l != nil && ok && o != j {
		// Two items of the register lead to one card: the merge keeps them apart, so each has its own card until a
		// person merges them (18.3).
		rep.Notes = append(rep.Notes, fmt.Sprintf("%s and %s lead to the card %s; %s gets a card of its own until a person merges them (18.3)", o.id(), j.id(), l.Card.Tag, j.id()))
		l = nil
	}
	state := "unchanged"
	if l == nil {
		name, slug := "", ""
		if j.cur != nil {
			name, slug = j.cur.Name, j.cur.Slug
		}
		if j.item != nil {
			if name == "" {
				name = j.item.Name
			}
			if slug == "" {
				slug = SlugFor(j.item)
			}
		}
		if slug == "" {
			slug = card.Slugify(name)
		}
		if slug == "" {
			return nil, "", fmt.Errorf("the item %q has no name to make a slug from", name)
		}
		l = s.create(kind, s.slug(slug, kind), name)
		state = "created"
	}
	s.claimed[l] = j
	e := &editor{fields: l.Card.Fields, rep: rep, slug: l.Card.Slug}
	e.aliases(l.Card.Name, j.aliases())
	e.sameAs(j.sameAs())
	var rs []*harvest.Row
	for _, ref := range j.sources() {
		if r := rows[ref]; r != nil && r.Kind == kind {
			rs = append(rs, r)
		}
	}
	sort.SliceStable(rs, func(a, b int) bool { return priority(rs[a].Source) < priority(rs[b].Source) })
	for _, r := range rs {
		paths := make([]string, 0, len(r.Facts))
		for p := range r.Facts {
			paths = append(paths, p)
		}
		sort.Strings(paths)
		for _, p := range paths {
			f := r.Facts[p]
			e.fact(p, f.V, "S:"+r.Source, f.Loc)
		}
		keys := make([]string, 0, len(r.Fields))
		for k := range r.Fields {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if _, ok := e.fields[k]; !ok && !own[k] {
				e.set(k, r.Fields[k])
			}
		}
		e.harvest(card.Harvest{Source: r.Source, Snapshot: r.Snapshot, Row: r.ID, Date: r.Date})
	}
	if e.err != nil {
		return nil, "", e.err
	}
	if e.changed {
		b, err := card.Encode(l.Card.Fields)
		if err != nil {
			return nil, "", err
		}
		c, err := card.Parse(b)
		if err != nil {
			return nil, "", err
		}
		l.Card, l.Changed = c, true
		if state == "unchanged" {
			state = "updated"
		}
	}
	return l, state, nil
}

type editor struct {
	fields  map[string]json.RawMessage
	rep     *SyncReport
	slug    string
	changed bool
	err     error
}

func (e *editor) set(k string, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		e.err = err
		return
	}
	if old, ok := e.fields[k]; ok && bytes.Equal(canon(old), canon(b)) {
		return
	}
	e.fields[k] = b
	e.changed = true
}

// canon is the JSON of a value after a round trip, so 4.05e11 and 405000000000 compare equal.
func canon(b []byte) []byte {
	var v any
	if json.Unmarshal(b, &v) != nil {
		return b
	}
	out, _ := json.Marshal(v)
	return out
}

func (e *editor) aliases(name string, add []string) {
	var have []string
	_ = json.Unmarshal(e.fields["aliases"], &have)
	n := len(have)
	for _, a := range add {
		if a != "" && !strings.EqualFold(a, name) && !containsFold(have, a) && len(have) < 20 {
			have = append(have, a)
		}
	}
	if len(have) > n {
		sort.Strings(have)
		e.set("aliases", have)
	}
}

func (e *editor) sameAs(add map[string]string) {
	have := map[string]any{}
	_ = json.Unmarshal(e.fields["same_as"], &have)
	n := len(have)
	for k, v := range add {
		if _, ok := have[k]; !ok && v != "" {
			have[k] = v
		}
	}
	if len(have) > n {
		e.set("same_as", have)
	}
}

// fact writes the fact at a path such as training.compute_flop. A missing fact is added; a fact of the same source
// takes the new value; a fact of another source is kept, and a different value counts as a disagreement.
func (e *editor) fact(path string, v any, src, loc string) {
	parts := strings.Split(path, ".")
	var tree any
	if raw, ok := e.fields[parts[0]]; ok {
		d := json.NewDecoder(bytes.NewReader(raw))
		d.UseNumber()
		if err := d.Decode(&tree); err != nil {
			e.err = err
			return
		}
	}
	nf := map[string]any{"v": v, "src": src}
	if loc != "" {
		nf["loc"] = loc
	}
	out, what := put(tree, parts[1:], nf)
	switch what {
	case "added":
		e.rep.Added++
	case "changed":
		e.rep.Changed++
	case "disagree":
		e.rep.Disagree++
		return
	case "clash":
		e.rep.Notes = append(e.rep.Notes, fmt.Sprintf("the card %s has a field %s that is not a fact, so the fact %s is not written", e.slug, parts[0], path))
		return
	default:
		return
	}
	e.set(parts[0], out)
}

func put(tree any, rest []string, nf map[string]any) (any, string) {
	if len(rest) == 0 {
		old, ok := tree.(map[string]any)
		if tree == nil {
			return nf, "added"
		}
		if !ok || old["v"] == nil {
			return tree, "clash"
		}
		ob, _ := json.Marshal(old["v"])
		nb, _ := json.Marshal(nf["v"])
		same := bytes.Equal(canon(ob), canon(nb))
		if old["src"] != nf["src"] {
			if same {
				return tree, ""
			}
			return tree, "disagree"
		}
		if same && old["loc"] == nf["loc"] {
			return tree, ""
		}
		for k, v := range old {
			if k != "v" && k != "loc" {
				nf[k] = v
			}
		}
		return nf, "changed"
	}
	m, ok := tree.(map[string]any)
	if tree != nil && (!ok || m["v"] != nil) {
		return tree, "clash"
	}
	if m == nil {
		m = map[string]any{}
	}
	sub, what := put(m[rest[0]], rest[1:], nf)
	m[rest[0]] = sub
	return m, what
}

func (e *editor) harvest(h card.Harvest) {
	var list []card.Harvest
	if raw, ok := e.fields["harvest"]; ok {
		if json.Unmarshal(raw, &list) != nil {
			var one card.Harvest
			_ = json.Unmarshal(raw, &one)
			list = []card.Harvest{one}
		}
	}
	found := false
	for i := range list {
		if list[i].Source == h.Source {
			found = true
			if list[i] == h {
				return
			}
			list[i] = h
		}
	}
	if !found {
		list = append(list, h)
	}
	if len(list) == 1 {
		e.set("harvest", list[0])
		return
	}
	e.set("harvest", list)
}
