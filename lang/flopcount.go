// SPDX-License-Identifier: Apache-2.0

package lang

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tamnd/ai-agent/node"
	"github.com/tamnd/ai-agent/shapes"
)

//go:embed aik_flopcount.py
var flopHarness string

// FlopLang is the framework whose forward pass aik flopcount counts: the NumPy reference (08.6, check 4).
const FlopLang = "numpy"

// FlopCase is the function of the NumPy test module that gives the forward call of a config: flop_case(c) takes one
// config of fixtures/shapes.json and returns (fn, args) or (fn, args, kwargs), and aik flopcount calls fn on the
// instrumented arrays.
const FlopCase = "flop_case"

// FlopConfig is the count of one config.
type FlopConfig struct {
	Config  string   `json:"config"`
	Counted *big.Int `json:"counted,omitempty"`
	Formula *big.Int `json:"formula,omitempty"`
	// Uncounted are the calls to functions with multiply-adds that the count does not follow, such as np.convolve and
	// np.linalg.solve, by name.
	Uncounted map[string]int `json:"uncounted,omitempty"`
	Error     string         `json:"error,omitempty"`
}

// OK tells if the count and the formula agree.
func (c FlopConfig) OK() bool {
	return c.Error == "" && c.Counted != nil && c.Formula != nil && c.Counted.Cmp(c.Formula) == 0
}

// FlopReport is the result of aik flopcount on one node.
type FlopReport struct {
	Slug    string       `json:"slug"`
	Total   string       `json:"total"` // the formula of shapes.yaml that the count is checked against
	Configs []FlopConfig `json:"configs,omitempty"`
	Error   string       `json:"error,omitempty"`
}

// OK tells if every config agrees.
func (r FlopReport) OK() bool {
	if r.Error != "" || len(r.Configs) == 0 {
		return false
	}
	for _, c := range r.Configs {
		if !c.OK() {
			return false
		}
	}
	return true
}

// FlopCount counts the multiply-adds of the NumPy forward pass of an architecture on the example and the sampled
// configs of its shape contract, and compares each count with the model formula total of shapes.yaml (08.6, check 4).
// The fixture is made again from shapes.yaml, so a stale fixtures/shapes.json does not hide a wrong formula.
func (r *Runner) FlopCount(n *node.Node, total string) FlopReport {
	rep := FlopReport{Slug: n.Slug, Total: total}
	if n.Shapes == nil {
		rep.Error = "the node has no " + shapes.FileName
		return rep
	}
	im := n.Lang(FlopLang)
	if im == nil {
		rep.Error = "the node has no NumPy reference"
		return rep
	}
	tests := files(*im, true, ".py")
	if len(tests) != 1 {
		rep.Error = fmt.Sprintf("the NumPy reference needs one test module with %s, and it has %d", FlopCase, len(tests))
		return rep
	}
	fx, err := n.Shapes.MakeFixture(shapes.Seed(n.Rel))
	if err != nil {
		rep.Error = err.Error()
		return rep
	}
	want := map[string]*big.Int{}
	for _, c := range fx.Configs {
		v, ok := c.Model[total]
		if !ok {
			rep.Error = fmt.Sprintf("the model of %s has no total %s", shapes.FileName, total)
			return rep
		}
		want[c.Name] = v
	}
	dir, err := os.MkdirTemp(r.TempDir, "aik-flopcount-")
	if err != nil {
		rep.Error = err.Error()
		return rep
	}
	if !r.Keep {
		defer os.RemoveAll(dir)
	}
	defer r.reap()
	if _, err := r.Gen(dir, n, *im); err != nil {
		rep.Error = err.Error()
		return rep
	}
	b, err := fx.Encode()
	if err != nil {
		rep.Error = err.Error()
		return rep
	}
	if err := write(dir, "aik_flopcount.json", string(b)); err != nil {
		rep.Error = err.Error()
		return rep
	}
	if err := write(dir, "aik_flopcount.py", flopHarness); err != nil {
		rep.Error = err.Error()
		return rep
	}
	ctx, cancel := context.WithTimeout(context.Background(), Timeout)
	defer cancel()
	mod := strings.TrimSuffix(filepath.ToSlash(tests[0]), ".py")
	var out, errOut bytes.Buffer
	cmd := r.command(ctx, im.Lang, dir, []string{"python3", "-B", "aik_flopcount.py", strings.ReplaceAll(mod, "/", "."), "aik_flopcount.json"})
	cmd.Stdout, cmd.Stderr = &out, &errOut
	start := time.Now()
	if err := cmd.Run(); err != nil {
		rep.Error = "the count did not run: " + tail(errOut.String()+err.Error(), 1000)
		return rep
	}
	r.logf("%s: counted in %s", n.Slug, time.Since(start).Round(time.Millisecond))
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	var res struct {
		Configs []struct {
			Config    string         `json:"config"`
			Counted   *big.Int       `json:"counted"`
			Uncounted map[string]int `json:"uncounted"`
			Error     string         `json:"error"`
		} `json:"configs"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &res); err != nil {
		rep.Error = "the count wrote no result: " + tail(out.String()+errOut.String(), 1000)
		return rep
	}
	if res.Error != "" {
		rep.Error = tail(res.Error, 1000)
		return rep
	}
	for _, c := range res.Configs {
		fc := FlopConfig{Config: c.Config, Counted: c.Counted, Formula: want[c.Config], Uncounted: c.Uncounted, Error: tail(c.Error, 1000)}
		if len(fc.Uncounted) == 0 {
			fc.Uncounted = nil
		}
		rep.Configs = append(rep.Configs, fc)
	}
	if len(rep.Configs) == 0 {
		rep.Error = "no config of the fixture runs forward: each is named"
	}
	return rep
}
