// SPDX-License-Identifier: Apache-2.0

package lang

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/tamnd/ai-agent/metric"
	"github.com/tamnd/ai-agent/node"
)

// VectorsMake is the script of a metric node that writes its test vectors with the official implementation, and
// VectorsImage the key of the image manifest for the image that has the official packages, aik-vectors (08.7.5,
// 08.9.1). The script runs on a copy of vectors/ and contract.yaml only: the official package never meets impl/.
const (
	VectorsMake  = "make.py"
	VectorsImage = "vectors"
)

// The most fields of one row that a run lists.
const maxFields = 8

// VectorField is one value of a row that a run writes differently, by its path in the row: want.score,
// want.precisions[2], by.
type VectorField struct {
	Path string `json:"path"`
	Old  any    `json:"old,omitempty"`
	New  any    `json:"new,omitempty"`
	// Within tells, for a number of want, if the new value is within the tolerance of the contract of the old one.
	Within *bool `json:"within,omitempty"`
}

// VectorRow is one row of a vector file that a run writes differently: added, removed, or changed.
type VectorRow struct {
	ID     string        `json:"id"`
	Change string        `json:"change"`
	Fields []VectorField `json:"fields,omitempty"`
	More   int           `json:"more,omitempty"` // the changed fields that are not listed
}

// VectorFile is one file of vectors/ that a run writes differently. A file whose rows are the same and whose bytes
// are not has no rows.
type VectorFile struct {
	File string      `json:"file"`
	New  bool        `json:"new,omitempty"`
	Rows []VectorRow `json:"rows,omitempty"`
}

// VectorRun is the result of running vectors/make.py of a metric node twice.
type VectorRun struct {
	Slug string `json:"slug"`
	// Changed are the files of the first run that are not the committed ones, and Unstable the files that the second
	// run writes with other bytes, so make.py is not deterministic.
	Changed  []VectorFile `json:"changed,omitempty"`
	Unstable []string     `json:"unstable,omitempty"`
	// Kept are the committed vector files that make.py does not write, such as vectors computed by hand.
	Kept []string `json:"kept,omitempty"`
	// Invalid is what is wrong with the vectors that make.py writes, against the contract.
	Invalid string `json:"invalid,omitempty"`
	Error   string `json:"error,omitempty"`

	made string // the vectors/ of the first run
}

// OK tells if make.py wrote valid vectors, the committed ones, twice.
func (vr *VectorRun) OK() bool {
	return vr.Error == "" && vr.Invalid == "" && len(vr.Changed) == 0 && len(vr.Unstable) == 0
}

// VectorsReady tells if the official implementations can run: the aik-vectors image in the sandbox, or Python here.
func (r *Runner) VectorsReady() error {
	if r.Box != nil {
		return r.Box.Ready(VectorsImage)
	}
	return Recipes["python"].Available()
}

// MakeVectors runs vectors/make.py of a metric node twice, on copies of vectors/ and contract.yaml with the .jsonl
// files taken out, so that each file it writes is its own. It checks the rows against the contract and compares them
// with the committed ones by id. Call Write to copy the changed files to the node, and Close to remove the work tree.
func (r *Runner) MakeVectors(n *node.Node) *VectorRun {
	vr := &VectorRun{Slug: n.Slug}
	src := filepath.Join(n.Dir, metric.VectorsDir)
	if _, err := os.Stat(filepath.Join(src, VectorsMake)); err != nil {
		vr.Error = "the node has no " + metric.VectorsDir + "/" + VectorsMake
		return vr
	}
	k, err := metric.Load(n.Dir)
	if err != nil {
		vr.Error = metric.ContractFile + ": " + err.Error()
		return vr
	}
	dir, err := os.MkdirTemp(r.TempDir, "aik-vectors-")
	if err != nil {
		vr.Error = err.Error()
		return vr
	}
	vr.made = filepath.Join(dir, "a", metric.VectorsDir)
	defer r.reap()
	for _, sub := range []string{"a", "b"} {
		if err := copyVectors(n.Dir, filepath.Join(dir, sub)); err != nil {
			vr.Error = err.Error()
			return vr
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), Timeout)
	defer cancel()
	for _, sub := range []string{"a", "b"} {
		var out, errOut bytes.Buffer
		cmd := r.command(ctx, VectorsImage, dir, []string{"python3", "-B", sub + "/" + metric.VectorsDir + "/" + VectorsMake})
		cmd.Stdout, cmd.Stderr = &out, &errOut
		if err := cmd.Run(); err != nil {
			vr.Error = "make.py fails: " + tail(errOut.String()+"\n"+err.Error(), 1500)
			return vr
		}
	}
	committed, err := metric.Files(n.Dir)
	if err != nil {
		vr.Error = err.Error()
		return vr
	}
	made, err := metric.Files(filepath.Join(dir, "a"))
	if err != nil {
		vr.Error = err.Error()
		return vr
	}
	if len(made) == 0 {
		vr.Error = "make.py writes no .jsonl file in " + metric.VectorsDir + "/"
		return vr
	}
	wrote := map[string]bool{}
	for _, f := range made {
		wrote[f] = true
		if !sameBytes(filepath.Join(dir, "a", f), filepath.Join(dir, "b", f)) {
			vr.Unstable = append(vr.Unstable, path.Base(f))
		}
		old := filepath.Join(n.Dir, filepath.FromSlash(f))
		fc, err := diffVectors(old, filepath.Join(dir, "a", filepath.FromSlash(f)), k.Tolerance)
		if err != nil {
			vr.Error = err.Error()
			return vr
		}
		if fc != nil {
			fc.File = path.Base(f)
			vr.Changed = append(vr.Changed, *fc)
		}
	}
	// The files that make.py does not write go back, so that the check of the ids sees every file.
	for _, f := range committed {
		if wrote[f] {
			continue
		}
		vr.Kept = append(vr.Kept, path.Base(f))
		b, err := os.ReadFile(filepath.Join(n.Dir, filepath.FromSlash(f)))
		if err == nil {
			err = os.WriteFile(filepath.Join(dir, "a", filepath.FromSlash(f)), b, 0o644)
		}
		if err != nil {
			vr.Error = err.Error()
			return vr
		}
	}
	vs, err := metric.ReadVectors(filepath.Join(dir, "a"), k)
	var bad []string
	if err != nil {
		bad = append(bad, strings.TrimPrefix(err.Error(), filepath.Join(dir, "a")+string(filepath.Separator)))
	}
	for _, v := range vs {
		if v.File == metric.VectorsDir+"/official.jsonl" && v.By == "hand" {
			bad = append(bad, fmt.Sprintf("official.jsonl:%d: the vector %s is computed by hand; official.jsonl holds the vectors of the official implementation, and edge.jsonl the others", v.Line, v.ID))
		}
	}
	vr.Invalid = strings.Join(bad, "; ")
	return vr
}

// copyVectors copies contract.yaml and vectors/ of the node in src to dst, without the .jsonl files of vectors/ and
// without hidden files and __pycache__.
func copyVectors(src, dst string) error {
	if err := copyNode(filepath.Join(src, metric.VectorsDir), filepath.Join(dst, metric.VectorsDir)); err != nil {
		return err
	}
	m, err := filepath.Glob(filepath.Join(dst, metric.VectorsDir, "*.jsonl"))
	if err != nil {
		return err
	}
	for _, p := range m {
		if err := os.Remove(p); err != nil {
			return err
		}
	}
	b, err := os.ReadFile(filepath.Join(src, metric.ContractFile))
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dst, metric.ContractFile), b, 0o644)
}

// row is one line of a vector file, decoded with its numbers as json.Number.
type row struct {
	id string
	v  map[string]any
}

func readRows(p string) ([]row, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var out []row
	sc := bufio.NewScanner(bytes.NewReader(b))
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	line := 0
	for sc.Scan() {
		line++
		t := bytes.TrimSpace(sc.Bytes())
		if len(t) == 0 {
			continue
		}
		dec := json.NewDecoder(bytes.NewReader(t))
		dec.UseNumber()
		var m map[string]any
		if err := dec.Decode(&m); err != nil {
			return nil, fmt.Errorf("%s:%d: %w", filepath.Base(p), line, err)
		}
		id, _ := m["id"].(string)
		out = append(out, row{id, m})
	}
	return out, sc.Err()
}

// diffVectors compares the rows of a vector file by id. It gives nil when the bytes are the same.
func diffVectors(old, new string, tol metric.Tolerance) (*VectorFile, error) {
	if sameBytes(old, new) {
		return nil, nil
	}
	fc := &VectorFile{}
	nr, err := readRows(new)
	if err != nil {
		return nil, err
	}
	or, err := readRows(old)
	if errors.Is(err, os.ErrNotExist) {
		fc.New = true
		for _, r := range nr {
			fc.Rows = append(fc.Rows, VectorRow{ID: r.id, Change: "added"})
		}
		return fc, nil
	}
	if err != nil {
		return nil, err
	}
	before := map[string]map[string]any{}
	for _, r := range or {
		before[r.id] = r.v
	}
	seen := map[string]bool{}
	for _, r := range nr {
		seen[r.id] = true
		o, ok := before[r.id]
		if !ok {
			fc.Rows = append(fc.Rows, VectorRow{ID: r.id, Change: "added"})
			continue
		}
		if reflect.DeepEqual(o, r.v) {
			continue
		}
		vr := VectorRow{ID: r.id, Change: "changed"}
		a, b := map[string]any{}, map[string]any{}
		flatten("", o, a)
		flatten("", r.v, b)
		var paths []string
		for p := range a {
			paths = append(paths, p)
		}
		for p := range b {
			if _, ok := a[p]; !ok {
				paths = append(paths, p)
			}
		}
		sort.Strings(paths)
		for _, p := range paths {
			x, y := a[p], b[p]
			if reflect.DeepEqual(x, y) {
				continue
			}
			if len(vr.Fields) == maxFields {
				vr.More++
				continue
			}
			f := VectorField{Path: p, Old: x, New: y}
			if strings.HasPrefix(p, "want.") {
				if w, ok := number(x); ok {
					if g, ok := number(y); ok {
						in := math.Abs(g-w) <= tol.Atol+tol.Rtol*math.Abs(w)
						f.Within = &in
					}
				}
			}
			vr.Fields = append(vr.Fields, f)
		}
		fc.Rows = append(fc.Rows, vr)
	}
	for _, r := range or {
		if !seen[r.id] {
			fc.Rows = append(fc.Rows, VectorRow{ID: r.id, Change: "removed"})
		}
	}
	return fc, nil
}

// flatten gives the leaves of a decoded JSON value by path: a.b, a.c[2].
func flatten(prefix string, v any, out map[string]any) {
	switch t := v.(type) {
	case map[string]any:
		if len(t) == 0 && prefix != "" {
			out[prefix] = t
		}
		for k, x := range t {
			p := k
			if prefix != "" {
				p = prefix + "." + k
			}
			flatten(p, x, out)
		}
	case []any:
		if len(t) == 0 {
			out[prefix] = t
		}
		for i, x := range t {
			flatten(prefix+"["+strconv.Itoa(i)+"]", x, out)
		}
	default:
		out[prefix] = v
	}
}

func number(v any) (float64, bool) {
	n, ok := v.(json.Number)
	if !ok {
		return 0, false
	}
	f, err := n.Float64()
	return f, err == nil
}

// Write copies the changed files of the first run to vectors/ of the node and gives their names.
func (vr *VectorRun) Write(n *node.Node) ([]string, error) {
	var out []string
	for _, c := range vr.Changed {
		b, err := os.ReadFile(filepath.Join(vr.made, c.File))
		if err != nil {
			return out, err
		}
		if err := os.WriteFile(filepath.Join(n.Dir, metric.VectorsDir, c.File), b, 0o644); err != nil {
			return out, err
		}
		out = append(out, c.File)
	}
	return out, nil
}

// Close removes the work tree of the run.
func (vr *VectorRun) Close(keep bool) {
	if vr.made != "" && !keep {
		os.RemoveAll(filepath.Dir(filepath.Dir(vr.made)))
	}
}
