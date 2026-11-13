// SPDX-License-Identifier: Apache-2.0

package harvest

import (
	"archive/tar"
	"bytes"
	"compress/bzip2"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// The AI Incident Database publishes a full export each week on its snapshots page, and a JSON twin of each
// incident page. A full run reads the newest export (about 120 MB); a run with a limit reads the incident pages, one
// request per incident, so a sample does not fetch the whole database.
const (
	AIIDSnapshots = "https://incidentdatabase.ai/research/snapshots/"
	AIIDBase      = "https://incidentdatabase.ai"
)

// AIIDPage is the URL of the JSON twin of the page of an incident.
func AIIDPage(id int) string {
	return fmt.Sprintf("%s/page-data/cite/%d/page-data.json", AIIDBase, id)
}

var backupRE = regexp.MustCompile(`https://[a-z0-9.-]+/backup-(\d{14})\.tar\.bz2`)

func harvestAIID(ctx context.Context, r *Run, o Options) ([]Row, error) {
	if o.Limit > 0 {
		return aiidPages(ctx, r, o)
	}
	page, _, err := r.Get(ctx, AIIDSnapshots, 8<<20)
	if err != nil {
		return nil, err
	}
	latest := ""
	for _, m := range backupRE.FindAllStringSubmatch(string(page), -1) {
		if latest == "" || m[1] > backupRE.FindStringSubmatch(latest)[1] {
			latest = m[0]
		}
	}
	if latest == "" {
		return nil, &ErrSchema{"aiid", "the snapshots page links no backup-<date>.tar.bz2"}
	}
	b, snap, err := r.Get(ctx, latest, 512<<20)
	if err != nil {
		return nil, err
	}
	csv, err := tarFile(b, "incidents.csv")
	if err != nil {
		return nil, err
	}
	return AIIDIncidents(csv, snap, o)
}

// tarFile reads the file of a .tar.bz2 whose name ends with name.
func tarFile(b []byte, name string) ([]byte, error) {
	tr := tar.NewReader(bzip2.NewReader(bytes.NewReader(b)))
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil, &ErrSchema{"aiid", "the export has no " + name}
		}
		if err != nil {
			return nil, err
		}
		if h.Name == name || strings.HasSuffix(h.Name, "/"+name) {
			return io.ReadAll(io.LimitReader(tr, 256<<20))
		}
	}
}

// AIIDIncidents reads incidents.csv of the export. The database is CC BY-SA 4.0 (class B), so the row keeps the id,
// the title, the date and the entity ids, and never the description (19.2).
func AIIDIncidents(b []byte, snap string, o Options) ([]Row, error) {
	t, err := readTable("aiid", b, "incident_id", "date", "title", "reports", "Alleged deployer of AI system", "Alleged developer of AI system", "Alleged harmed or nearly harmed parties")
	if err != nil {
		return nil, err
	}
	var out []Row
	for _, rec := range t.rows {
		id, err := strconv.Atoi(t.get(rec, "incident_id"))
		if err != nil {
			continue
		}
		var reports []int
		_ = json.Unmarshal([]byte(t.get(rec, "reports")), &reports)
		var dep, dev, harmed []string
		_ = json.Unmarshal([]byte(t.get(rec, "Alleged deployer of AI system")), &dep)
		_ = json.Unmarshal([]byte(t.get(rec, "Alleged developer of AI system")), &dev)
		_ = json.Unmarshal([]byte(t.get(rec, "Alleged harmed or nearly harmed parties")), &harmed)
		row := aiidRow(id, t.get(rec, "title"), t.get(rec, "date"), len(reports), dep, dev, harmed, snap, "incidents.csv")
		if row.Name == "" || o.Since != "" && date(t.get(rec, "date")) < o.Since {
			continue
		}
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Signals["aiid_id"].(int) < out[j].Signals["aiid_id"].(int) })
	return keep(out, o.Limit), nil
}

func aiidRow(id int, title, day string, reports int, dep, dev, harmed []string, snap, loc string) Row {
	sid := strconv.Itoa(id)
	row := Row{ID: sid, Name: clean(title), Kind: "incident", URL: fmt.Sprintf("%s/cite/%d/", AIIDBase, id),
		SameAs: map[string]string{"aiid": sid}, Snapshot: snap}
	setFact(&row, "date", date(day), loc)
	setFact(&row, "entities.deployers", dep, loc)
	setFact(&row, "entities.developers", dev, loc)
	setFact(&row, "entities.harmed", harmed, loc)
	setSignal(&row, "aiid_id", id)
	setSignal(&row, "reports", reports)
	return row
}

type aiidPageData struct {
	Result struct {
		Data struct {
			Incident *struct {
				ID        json.Number `json:"incident_id"`
				Title     string      `json:"title"`
				Date      string      `json:"date"`
				Reports   []struct{}  `json:"reports"`
				Developer []string    `json:"Alleged_developer_of_AI_system"`
				Deployer  []string    `json:"Alleged_deployer_of_AI_system"`
				Harmed    []string    `json:"Alleged_harmed_or_nearly_harmed_parties"`
			} `json:"incident"`
		} `json:"data"`
	} `json:"result"`
}

// AIIDPageRow reads the JSON twin of an incident page. It returns false for a page with no incident.
func AIIDPageRow(b []byte, snap string) (Row, bool, error) {
	var p aiidPageData
	if err := json.Unmarshal(b, &p); err != nil {
		return Row{}, false, &ErrSchema{"aiid", "the incident page is not JSON: " + err.Error()}
	}
	in := p.Result.Data.Incident
	if in == nil {
		return Row{}, false, nil
	}
	id, err := strconv.Atoi(in.ID.String())
	if err != nil {
		return Row{}, false, &ErrSchema{"aiid", "the incident id " + in.ID.String() + " is not a number"}
	}
	row := aiidRow(id, in.Title, in.Date, len(in.Reports), in.Deployer, in.Developer, in.Harmed, snap, "page-data")
	return row, row.Name != "", nil
}

// aiidPages reads the incident pages from the first incident until it has the limit, and stops after 20 missing
// incidents in a row.
func aiidPages(ctx context.Context, r *Run, o Options) ([]Row, error) {
	var out []Row
	miss := 0
	for id := 1; len(out) < o.Limit && miss < 20; id++ {
		b, snap, err := r.Get(ctx, AIIDPage(id), 16<<20)
		if err != nil {
			if strings.Contains(err.Error(), "404") {
				miss++
				continue
			}
			return nil, err
		}
		row, ok, err := AIIDPageRow(b, snap)
		if err != nil {
			return nil, err
		}
		if !ok {
			miss++
			continue
		}
		miss = 0
		if o.Since != "" && row.Facts["date"].V.(string) < o.Since {
			continue
		}
		out = append(out, row)
	}
	return out, nil
}
