// SPDX-License-Identifier: Apache-2.0

package harvest

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/tamnd/ai-agent/cite"
)

// Row is one inventory row (spec 13.3): an item that a source names, with the source's own id, the name, the kind
// that the adapter guesses, the ids of other sources in same_as, and the snapshot that it was read from.
//
// Facts are the values that a card takes from the row, by path ("params.total"), each with the source of the row as
// its src (04.6). Fields are plain card fields, such as the authors of a work. Labels are the source's own
// categories, for placing the item; Links name related items by the ids of the source; Signals feed the scope rules
// of 18.5. None of them holds prose of the source.
type Row struct {
	Source   string              `json:"source"`
	ID       string              `json:"id"`
	Name     string              `json:"name"`
	Kind     string              `json:"kind"`
	URL      string              `json:"url,omitempty"`
	Aliases  []string            `json:"aliases,omitempty"`
	SameAs   map[string]string   `json:"same_as,omitempty"`
	Fields   map[string]any      `json:"fields,omitempty"`
	Facts    map[string]Fact     `json:"facts,omitempty"`
	Labels   []string            `json:"labels,omitempty"`
	Links    map[string][]string `json:"links,omitempty"`
	Signals  map[string]any      `json:"signals,omitempty"`
	Snapshot string              `json:"snapshot"`
	Date     string              `json:"date"`
}

// Fact is a value of a row. Loc is where the source has it, when the source has places (a column, a table).
type Fact struct {
	V   any    `json:"v"`
	Loc string `json:"loc,omitempty"`
}

// Snapshot is a line of manifests/<source>.jsonl: one fetched object, by its BLAKE3 hash in the raw store.
type Snapshot struct {
	URL     string `json:"url"`
	Fetched string `json:"fetched"`
	Hash    string `json:"hash"`
	Size    int    `json:"size"`
	Class   string `json:"licence_class"`
	Adapter string `json:"adapter"`
}

// Report is the run record of 13.4, reports/harvest/<source>/<date>.json.
type Report struct {
	Source    string   `json:"source"`
	Date      string   `json:"date"`
	Adapter   string   `json:"adapter"`
	Licence   string   `json:"licence"`
	Class     string   `json:"licence_class"`
	Partial   bool     `json:"partial"`
	Snapshots []string `json:"snapshots"`
	Requests  int      `json:"requests"`
	Rows      int      `json:"rows"`
	New       int      `json:"new"`
	Changed   int      `json:"changed"`
	Removed   int      `json:"removed"`
	Unchanged int      `json:"unchanged"`
	Skipped   int      `json:"skipped,omitempty"`
	Notes     []string `json:"notes,omitempty"`
}

// The paths of the harvest in the content repository (spec 20.2).
const (
	InventoryDir = "registers/inventory"
	ManifestDir  = "manifests"
	ReportDir    = "reports/harvest"
)

// Options are the flags of a run.
type Options struct {
	// Limit keeps at most this many rows of each part of the source; 0 keeps all. A limited run is partial: it
	// removes no row of the inventory.
	Limit int
	// Since keeps only items dated on or after this date, for the sources that date their items. A run with Since is
	// partial.
	Since string
}

// Run is one run of an adapter on one date.
type Run struct {
	Root    string
	Source  Source
	Client  *Client
	Store   cite.Store
	Date    string
	Version string
	// Offline reads each URL from the newest snapshot of the manifest instead of the network.
	Offline bool
	// Skipped counts the items of the source that the adapter did not keep, and Notes says why.
	Skipped int
	Notes   []string
	// Partial is set by an adapter that could not read a part of the source, so that the run removes no row.
	Partial bool

	prior []Snapshot
	snaps []Snapshot
}

// adapterName is the adapter field of snapshots and reports.
func (r *Run) adapterName() string {
	return "aik/" + strings.TrimPrefix(r.Version, "v") + " harvest/" + r.Source.ID
}

// Get gives the body of a URL and the snapshot id ("b3:<hash>") that it is stored under. It fetches the URL and
// stores it, or offline it reads the newest snapshot of the URL in the manifest.
func (r *Run) Get(ctx context.Context, url string, limit int64) ([]byte, string, error) {
	if r.Offline {
		if r.prior == nil {
			p, err := ReadManifest(r.Root, r.Source.ID)
			if err != nil {
				return nil, "", err
			}
			r.prior = p
		}
		for i := len(r.prior) - 1; i >= 0; i-- {
			s := r.prior[i]
			if s.URL != url {
				continue
			}
			b, err := r.Store.Get(strings.TrimPrefix(s.Hash, "b3:"))
			if err != nil {
				return nil, "", fmt.Errorf("the snapshot of %s: %w", url, err)
			}
			r.use(s)
			return b, s.Hash, nil
		}
		return nil, "", fmt.Errorf("offline: the manifest of %s has no snapshot of %s", r.Source.ID, url)
	}
	b, err := r.Client.Get(ctx, url, limit)
	if err != nil {
		return nil, "", err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	h, err := r.Store.Put(b, cite.Meta{URL: url, Fetched: now, Licence: r.Source.Class, Fetcher: r.adapterName()})
	if err != nil {
		return nil, "", err
	}
	s := Snapshot{URL: url, Fetched: now, Hash: "b3:" + h, Size: len(b), Class: r.Source.Class, Adapter: r.adapterName()}
	r.use(s)
	return b, s.Hash, nil
}

func (r *Run) use(s Snapshot) {
	for _, o := range r.snaps {
		if o.Hash == s.Hash && o.URL == s.URL {
			return
		}
	}
	r.snaps = append(r.snaps, s)
}

// Skip counts items that the adapter does not keep, with the reason.
func (r *Run) Skip(n int, why string) {
	if n <= 0 {
		return
	}
	r.Skipped += n
	r.Notes = append(r.Notes, fmt.Sprintf("%d skipped: %s", n, why))
}

// Finish writes the inventory, the manifest, the run record and the row of registers/sources.yaml. An unchanged row
// keeps the snapshot and the date of the run that last changed it, so a weekly run is a small diff.
func (r *Run) Finish(rows []Row, o Options) (*Report, error) {
	rep := &Report{Source: r.Source.ID, Date: r.Date, Adapter: r.adapterName(), Licence: r.Source.Licence, Class: r.Source.Class,
		Partial: o.Limit > 0 || o.Since != "" || r.Partial, Skipped: r.Skipped, Notes: r.Notes}
	if r.Client != nil {
		rep.Requests = r.Client.Requests
	}
	for _, s := range r.snaps {
		rep.Snapshots = append(rep.Snapshots, s.Hash)
	}
	byID := map[string]Row{}
	for _, row := range rows {
		if row.ID == "" || row.Name == "" {
			continue
		}
		if _, dup := byID[row.ID]; dup {
			rep.Notes = append(rep.Notes, "the source names "+row.ID+" twice; the first row is kept")
			continue
		}
		row.Source, row.Date = r.Source.ID, r.Date
		byID[row.ID] = row
	}
	old, err := ReadInventory(r.Root, r.Source.ID)
	if err != nil {
		return nil, err
	}
	oldByID := map[string]Row{}
	for _, row := range old {
		oldByID[row.ID] = row
	}
	for id, row := range byID {
		prev, ok := oldByID[id]
		switch {
		case !ok:
			rep.New++
		case Same(prev, row):
			rep.Unchanged++
			byID[id] = prev
		default:
			rep.Changed++
		}
	}
	for id, row := range oldByID {
		if _, ok := byID[id]; ok {
			continue
		}
		if rep.Partial {
			byID[id] = row // a partial run did not look at the row
			continue
		}
		rep.Removed++
	}
	out := slices.Collect(maps.Values(byID))
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	rep.Rows = len(out)
	if err := WriteInventory(r.Root, r.Source.ID, out); err != nil {
		return nil, err
	}
	if !r.Offline {
		if err := r.appendManifest(); err != nil {
			return nil, err
		}
	}
	if _, err := Register(r.Root, r.Source); err != nil {
		return nil, err
	}
	b, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return nil, err
	}
	path := filepath.Join(r.Root, ReportDir, r.Source.ID, r.Date+".json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	return rep, os.WriteFile(path, append(b, '\n'), 0o644)
}

// Same tells if two rows say the same, apart from the snapshot and the date.
func Same(a, b Row) bool {
	a.Snapshot, b.Snapshot, a.Date, b.Date = "", "", "", ""
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	return bytes.Equal(ja, jb)
}

func (r *Run) appendManifest() error {
	if len(r.snaps) == 0 {
		return nil
	}
	path := filepath.Join(r.Root, ManifestDir, r.Source.ID+".jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644) //nolint:gosec // a public manifest of the content repo
	if err != nil {
		return err
	}
	for _, s := range r.snaps {
		b, _ := json.Marshal(s)
		if _, err := f.Write(append(b, '\n')); err != nil {
			_ = f.Close()
			return err
		}
	}
	return f.Close()
}

// ReadManifest reads manifests/<source>.jsonl, oldest first.
func ReadManifest(root, source string) ([]Snapshot, error) {
	var out []Snapshot
	err := readLines(filepath.Join(root, ManifestDir, source+".jsonl"), func(b []byte) error {
		var s Snapshot
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		out = append(out, s)
		return nil
	})
	return out, err
}

// ReadInventory reads registers/inventory/<source>.jsonl. A missing file is an empty inventory.
func ReadInventory(root, source string) ([]Row, error) {
	var out []Row
	err := readLines(filepath.Join(root, InventoryDir, source+".jsonl"), func(b []byte) error {
		var row Row
		if err := json.Unmarshal(b, &row); err != nil {
			return err
		}
		out = append(out, row)
		return nil
	})
	return out, err
}

// ReadInventories reads every inventory of the content repository, by source id.
func ReadInventories(root string) (map[string][]Row, error) {
	files, err := filepath.Glob(filepath.Join(root, InventoryDir, "*.jsonl"))
	if err != nil {
		return nil, err
	}
	out := map[string][]Row{}
	for _, f := range files {
		src := strings.TrimSuffix(filepath.Base(f), ".jsonl")
		rows, err := ReadInventory(root, src)
		if err != nil {
			return nil, err
		}
		out[src] = rows
	}
	return out, nil
}

// WriteInventory writes the rows, one per line, in the order given.
func WriteInventory(root, source string, rows []Row) error {
	var buf bytes.Buffer
	for _, row := range rows {
		b, err := marshal(row)
		if err != nil {
			return err
		}
		buf.Write(b)
		buf.WriteByte('\n')
	}
	path := filepath.Join(root, InventoryDir, source+".jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

// marshal writes JSON without escaping <, > and &, which names and URLs hold.
func marshal(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

func readLines(path string, each func([]byte) error) error {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64<<10), 16<<20)
	for n := 1; sc.Scan(); n++ {
		b := bytes.TrimSpace(sc.Bytes())
		if len(b) == 0 {
			continue
		}
		if err := each(b); err != nil {
			return fmt.Errorf("%s:%d: %w", path, n, err)
		}
	}
	return sc.Err()
}
