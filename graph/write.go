// SPDX-License-Identifier: Apache-2.0

package graph

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/tamnd/ai-agent/node"
)

// Summary is build/graph/summary.json: the counts of the graph and the cycles of the edge types that may have none.
type Summary struct {
	Edges    int            `json:"edges"`
	Reverse  int            `json:"reverse"`
	Unknown  int            `json:"unknown"`
	Neighbor int            `json:"neighbour"`
	ByType   map[string]int `json:"by_type"`
	Cycles   [][]string     `json:"cycles,omitempty"`
}

// Acyclic are the edge types that may have no cycle (rules E03 and E04).
func Acyclic() []string {
	var out []string
	for _, e := range node.Edges {
		if e.Acyclic {
			out = append(out, e.Type)
		}
	}
	return out
}

// Write writes the graph to dir: edges.jsonl with the edges as written and derived, reverse.jsonl with the reverse
// edges, and summary.json.
func (g *Graph) Write(dir string) (Summary, error) {
	s := Summary{Edges: g.n, ByType: g.Counts()}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return s, err
	}
	es := g.Edges()
	for _, e := range es {
		switch {
		case e.Site != "":
			s.Neighbor++
		case !e.Known:
			s.Unknown++
		}
	}
	rev := g.Reversed()
	s.Reverse = len(rev)
	for _, t := range Acyclic() {
		s.Cycles = append(s.Cycles, g.Cycles(t)...)
	}
	if err := writeLines(filepath.Join(dir, "edges.jsonl"), es); err != nil {
		return s, err
	}
	if err := writeLines(filepath.Join(dir, "reverse.jsonl"), rev); err != nil {
		return s, err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return s, err
	}
	return s, os.WriteFile(filepath.Join(dir, "summary.json"), append(b, '\n'), 0o644)
}

func writeLines(p string, es []Edge) error {
	f, err := os.Create(p)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	enc := json.NewEncoder(w)
	for _, e := range es {
		if err := enc.Encode(e); err != nil {
			f.Close()
			return err
		}
	}
	if err := w.Flush(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
