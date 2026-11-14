// SPDX-License-Identifier: Apache-2.0

package harvest

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// HubAPI is the API of the Hugging Face Hub. It needs no token for public repositories.
const HubAPI = "https://huggingface.co/api/"

// HubPageSize is the number of repositories of a page of the API.
const HubPageSize = 1000

// HubPart is a kind of repository of the Hub and the thresholds of its scope rule: a repository is a card when it has
// at least MinDownloads downloads in the last 30 days or MinLikes likes (S-M2 for models, S-D1 for datasets, 18.5).
type HubPart struct {
	Kind         string // "model" or "dataset"
	Path         string // "models" or "datasets"
	MinDownloads int
	MinLikes     int
	Expand       []string
}

// HubParts are the models and the datasets.
var HubParts = []HubPart{
	{Kind: "model", Path: "models", MinDownloads: 10000, MinLikes: 100,
		Expand: []string{"downloads", "likes", "tags", "createdAt", "sha", "gated", "pipeline_tag", "library_name"}},
	{Kind: "dataset", Path: "datasets", MinDownloads: 1000, MinLikes: 50,
		Expand: []string{"downloads", "likes", "tags", "createdAt", "sha", "gated", "paperswithcode_id"}},
}

// HubCursor is the cursor of the page after the repository with the id oid and the value v of the sort key. The API
// gives it in the Link header; the adapter makes it from the last repository of the page, so that a run from the
// snapshots needs only their bodies.
func HubCursor(key string, v int, oid string) string {
	q := fmt.Sprintf(`{"$or":[{%q:%d,"_id":{"$gt":%q}},{%q:{"$lt":%d}},{%q:null}]}`, key, v, oid, key, v, key)
	return base64.StdEncoding.EncodeToString([]byte(q))
}

// HubURL is a page of the repositories of the part, sorted by key ("downloads" or "likes"), the largest first. The
// first page has no cursor.
func HubURL(p HubPart, key, cursor string) string {
	v := url.Values{"sort": {key}, "direction": {"-1"}, "limit": {strconv.Itoa(HubPageSize)}, "expand[]": p.Expand}
	if cursor != "" {
		v.Set("cursor", cursor)
	}
	return HubAPI + p.Path + "?" + v.Encode()
}

// HubRepo is a repository of a page.
type HubRepo struct {
	OID       string   `json:"_id"`
	ID        string   `json:"id"`
	Downloads int      `json:"downloads"`
	Likes     int      `json:"likes"`
	Tags      []string `json:"tags"`
	Created   string   `json:"createdAt"`
	SHA       string   `json:"sha"`
	Gated     any      `json:"gated"` // false, "auto" or "manual"
	Pipeline  string   `json:"pipeline_tag"`
	Library   string   `json:"library_name"`
	PwC       string   `json:"paperswithcode_id"`
}

// HubPage reads a page of the API.
func HubPage(b []byte) ([]HubRepo, error) {
	var rs []HubRepo
	if err := json.Unmarshal(b, &rs); err != nil {
		return nil, &ErrSchema{"huggingface", "the page is not a list of repositories: " + err.Error()}
	}
	for _, r := range rs {
		if r.ID == "" || r.OID == "" {
			return nil, &ErrSchema{"huggingface", "a repository of the page has no id or no _id"}
		}
	}
	return rs, nil
}

func (r HubRepo) value(key string) int {
	if key == "likes" {
		return r.Likes
	}
	return r.Downloads
}

// owner is the user or organization of a repository id.
func owner(id string) string {
	o, _, _ := strings.Cut(id, "/")
	return o
}

// HubBase is a repository that another one derives from, as its tags say: "base_model:<relation>:<id>".
type HubBase struct {
	Relation string // finetune, quantized, merge, adapter, or "" when the tag names no relation
	ID       string
}

// Bases are the base models that the tags of r name.
func (r HubRepo) Bases() []HubBase {
	var out []HubBase
	for _, t := range r.Tags {
		rest, ok := strings.CutPrefix(t, "base_model:")
		if !ok {
			continue
		}
		rel, id, ok := strings.Cut(rest, ":")
		if !ok {
			rel, id = "", rest
		}
		if rel == "" {
			// A plain base_model tag goes with a tag that names the relation; keep it only when none does.
			if hasTagPrefix(r.Tags, "base_model:", ":"+id) {
				continue
			}
		}
		out = append(out, HubBase{Relation: rel, ID: id})
	}
	return out
}

func hasTagPrefix(tags []string, prefix, suffix string) bool {
	for _, t := range tags {
		if strings.HasPrefix(t, prefix) && strings.HasSuffix(t, suffix) && strings.Count(t, ":") >= 2 {
			return true
		}
	}
	return false
}

// HubSelect applies the rest of S-M2 to the repositories over the thresholds: a quantization, merge, adapter or
// fine-tune of a carded repository is not a card but a count on the card of its base. Two kinds of derived
// repositories stay cards: a fine-tune by the owner of its base, which is the developer's instruction model, and a
// repository with more downloads than its base. It returns the cards and the counts of derivatives by base and
// relation.
func HubSelect(repos map[string]HubRepo) (cards []HubRepo, derived map[string]map[string]int) {
	const (
		unknown = iota
		visiting
		card
		count
	)
	state := map[string]int{}
	baseOf := map[string]HubBase{}
	var decide func(id string) int
	decide = func(id string) int {
		switch state[id] {
		case card, count:
			return state[id]
		case visiting:
			return card // a cycle of tags; keep it
		}
		state[id] = visiting
		r := repos[id]
		out := card
		for _, b := range r.Bases() {
			base, ok := repos[b.ID]
			if !ok || b.ID == id {
				continue
			}
			if b.Relation == "finetune" && owner(b.ID) == owner(id) || r.Downloads > base.Downloads {
				continue
			}
			if decide(b.ID) == card {
				out, baseOf[id] = count, b
				break
			}
		}
		state[id] = out
		return out
	}
	ids := make([]string, 0, len(repos))
	for id := range repos {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	derived = map[string]map[string]int{}
	for _, id := range ids {
		if decide(id) == card {
			cards = append(cards, repos[id])
			continue
		}
		b := baseOf[id]
		rel := b.Relation
		if rel == "" {
			rel = "derivative"
		}
		if derived[b.ID] == nil {
			derived[b.ID] = map[string]int{}
		}
		derived[b.ID][rel]++
	}
	return cards, derived
}

// HubRow is the row of a repository of the part.
func HubRow(p HubPart, r HubRepo, snap string) Row {
	id := r.ID
	key := "hub" // the key of spec 04
	if p.Kind == "dataset" {
		id, key = "datasets/"+r.ID, "hub_dataset"
	}
	row := Row{ID: id, Name: r.ID, Kind: p.Kind, URL: "https://huggingface.co/" + id,
		SameAs: map[string]string{key: r.ID}, Snapshot: snap, Fields: map[string]any{}}
	if r.PwC != "" {
		row.SameAs["pwc"] = "dataset/" + r.PwC
	}
	links := map[string][]string{"author": {owner(r.ID)}}
	for _, t := range r.Tags {
		k, v, ok := strings.Cut(t, ":")
		if !ok || v == "" {
			continue
		}
		switch k {
		case "license":
			row.Fields["licence"] = v
		case "arxiv":
			links["paper"] = append(links["paper"], "arxiv:"+v)
		case "doi":
			links["paper"] = append(links["paper"], "doi:"+strings.ToLower(v))
		case "dataset":
			links["dataset"] = append(links["dataset"], v)
		}
	}
	for _, b := range r.Bases() {
		links["base_model"] = append(links["base_model"], b.ID)
	}
	for k := range links {
		sort.Strings(links[k])
	}
	row.Links = links
	if r.Pipeline != "" {
		row.Labels = []string{r.Pipeline}
	}
	if r.Library != "" {
		row.Fields["library"] = r.Library
	}
	if r.SHA != "" {
		row.Fields["commit"] = r.SHA
	}
	switch g := r.Gated.(type) {
	case string:
		row.Fields["gated"] = g
	case bool:
		if g {
			row.Fields["gated"] = "yes"
		}
	}
	setFact(&row, "created", date(r.Created), "createdAt")
	setSignal(&row, "downloads_30d", r.Downloads)
	setSignal(&row, "likes", r.Likes)
	return row
}

// harvestHub reads the models and the datasets of the Hub over the thresholds of S-M2 and S-D1: the pages sorted by
// downloads until a repository has fewer than the threshold, then the pages sorted by likes the same way. -limit keeps
// at most that many repositories of each part and sort.
func harvestHub(ctx context.Context, r *Run, o Options) ([]Row, error) {
	var out []Row
	for _, p := range HubParts {
		repos := map[string]HubRepo{}
		snaps := map[string]string{}
		for _, key := range []string{"downloads", "likes"} {
			least := p.MinDownloads
			if key == "likes" {
				least = p.MinLikes
			}
			cursor, kept := "", 0
		pages:
			for {
				b, snap, err := r.Get(ctx, HubURL(p, key, cursor), 64<<20)
				if err != nil {
					return nil, fmt.Errorf("huggingface %s by %s: %w", p.Path, key, err)
				}
				page, err := HubPage(b)
				if err != nil {
					return nil, err
				}
				for _, repo := range page {
					if repo.value(key) < least || o.Limit > 0 && kept >= o.Limit {
						break pages
					}
					kept++
					if _, ok := repos[repo.ID]; !ok {
						repos[repo.ID], snaps[repo.ID] = repo, snap
					}
				}
				if len(page) < HubPageSize {
					break
				}
				last := page[len(page)-1]
				cursor = HubCursor(key, last.value(key), last.OID)
			}
		}
		cards, derived := HubSelect(repos)
		n := 0
		for _, c := range cards {
			row := HubRow(p, c, snaps[c.ID])
			if d := derived[c.ID]; len(d) > 0 {
				setSignal(&row, "derivatives", d)
			}
			out = append(out, row)
		}
		for _, d := range derived {
			for _, k := range d {
				n += k
			}
		}
		if n > 0 {
			r.Skip(n, fmt.Sprintf("%ss that derive from a carded %s, counted on its card (S-M2)", p.Kind, p.Kind))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}
