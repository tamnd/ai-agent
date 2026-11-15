// SPDX-License-Identifier: Apache-2.0

package exp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"regexp"
	"sort"
	"time"
)

// The files of the compute budget (09.4.2, 09.10): the register of the GPU pool with the monthly cap, and the ledger of
// the batches that ran.
const (
	ComputeFile = "registers/compute.yaml"
	LedgerFile  = "reports/compute/ledger.jsonl"
)

// Compute is registers/compute.yaml: the provider of the GPU pool, the instance type and hourly price of each GPU
// class, and the monthly caps. Raising a cap is a pull request that a person approves.
type Compute struct {
	Provider string              `yaml:"provider"`
	Caps     []Cap               `yaml:"caps"`
	Classes  map[string]Instance `yaml:"classes"`
	Note     string              `yaml:"note,omitempty"`
}

// Cap is the monthly cap in US dollars from a month on, until the next cap.
type Cap struct {
	From string  `yaml:"from"` // YYYY-MM
	USD  float64 `yaml:"usd"`
	Note string  `yaml:"note,omitempty"`
}

// Instance is the machine of one GPU class in the pool.
type Instance struct {
	Instance   string  `yaml:"instance"`
	GPU        string  `yaml:"gpu"`
	USDPerHour float64 `yaml:"usd_per_hour"`
	// Spot tells if the pool may use spot machines for the class: run.py checkpoints at least every 15 minutes.
	Spot bool `yaml:"spot,omitempty"`
}

var monthRE = regexp.MustCompile(`^\d{4}-(0[1-9]|1[0-2])$`)

// ReadCompute reads and checks registers/compute.yaml.
func ReadCompute(file string) (*Compute, error) {
	var c Compute
	ok, err := readYAML(file, &c)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	if !ok {
		return nil, fmt.Errorf("%s is not there: the GPU classes have no price and the month no cap", file)
	}
	var errs []error
	for i, cp := range c.Caps {
		switch {
		case !monthRE.MatchString(cp.From):
			errs = append(errs, fmt.Errorf("cap %d: from is a month YYYY-MM, not %q", i+1, cp.From))
		case i > 0 && cp.From <= c.Caps[i-1].From:
			errs = append(errs, fmt.Errorf("cap %d: the caps are in the order of their months", i+1))
		}
		if cp.USD < 0 || math.IsNaN(cp.USD) {
			errs = append(errs, fmt.Errorf("cap %d: usd is 0 or more", i+1))
		}
	}
	if len(c.Caps) == 0 {
		errs = append(errs, errors.New("the register has no caps"))
	}
	for _, name := range sortedKeys(c.Classes) {
		in := c.Classes[name]
		b, known := Budgets[name]
		switch {
		case !known || b.Name == "cpu-small":
			errs = append(errs, fmt.Errorf("classes: %q is not a GPU class of spec 9.4.1", name))
		case in.USDPerHour <= 0 || math.IsNaN(in.USDPerHour) || math.IsInf(in.USDPerHour, 0):
			errs = append(errs, fmt.Errorf("classes: %s has no usd_per_hour", name))
		case in.Instance == "":
			errs = append(errs, fmt.Errorf("classes: %s names no instance", name))
		}
	}
	if err := errors.Join(errs...); err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	return &c, nil
}

// CapOf gives the cap of a month YYYY-MM: the last cap from that month or before. It gives false before the first.
func (c *Compute) CapOf(month string) (float64, bool) {
	usd, ok := 0.0, false
	for _, cp := range c.Caps {
		if cp.From <= month {
			usd, ok = cp.USD, true
		}
	}
	return usd, ok
}

// Batch is a batch of experiments of one GPU class: the unit of approval and cost. reports/compute/ledger.jsonl has one
// line for each batch that ran.
type Batch struct {
	ID          string     `json:"batch"`
	Date        string     `json:"date"`
	Class       string     `json:"class"`
	Experiments []string   `json:"experiments"`
	GPUHours    float64    `json:"gpu_hours"`
	USD         float64    `json:"usd"`
	Runs        []string   `json:"runs,omitempty"`
	Items       []PlanItem `json:"items,omitempty"` // in a plan, not in the ledger
}

// ReadLedger reads reports/compute/ledger.jsonl. A ledger that is not there has no batches.
func ReadLedger(file string) ([]Batch, error) {
	b, err := os.ReadFile(file)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Batch
	sc := bufio.NewScanner(bytes.NewReader(b))
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for i := 1; sc.Scan(); i++ {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var bt Batch
		dec := json.NewDecoder(bytes.NewReader(line))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&bt); err != nil {
			return nil, fmt.Errorf("%s:%d: %w", file, i, err)
		}
		if _, err := time.Parse(time.DateOnly, bt.Date); err != nil || bt.USD < 0 || math.IsNaN(bt.USD) {
			return nil, fmt.Errorf("%s:%d: a batch has a date YYYY-MM-DD and a price of 0 or more", file, i)
		}
		out = append(out, bt)
	}
	return out, sc.Err()
}

// Spent gives the price of the batches of the ledger in a month YYYY-MM.
func Spent(ledger []Batch, month string) float64 {
	s := 0.0
	for _, b := range ledger {
		if b.Date[:7] == month {
			s += b.USD
		}
	}
	return s
}

// PlanItem is one experiment that needs a run.
type PlanItem struct {
	Experiment string  `json:"experiment"` // slug/experiment
	Class      string  `json:"class"`
	Why        string  `json:"why"`       // not run, or why its results are stale
	Hours      float64 `json:"gpu_hours"` // the estimate; 0 for cpu-small
	Estimate   string  `json:"estimate"`  // how the hours are estimated
	USD        float64 `json:"usd"`
	// Approval tells that a person approves the experiment before it runs: a new gpu-medium experiment (9.4.1).
	Approval bool `json:"approval,omitempty"`
	new      bool
}

// Needs gives the plan item of an experiment that has never run or whose results are stale, and false for one whose
// results are current. The hours are those of the last run with a quarter more, and at most the cap of the class;
// without a last run, the cap of the class, the most that a run can take (rule R09).
func Needs(ref string, e *Experiment) (PlanItem, bool) {
	it := PlanItem{Experiment: ref, Why: "not run", new: true}
	if e.Results != nil {
		it.Why, it.new = e.Stale(), false
		if it.Why == "" {
			return it, false
		}
	}
	if e.Design != nil {
		it.Class = e.Design.Budget
	}
	b, ok := Budgets[it.Class]
	if !ok || b.Name == "cpu-small" {
		it.Estimate = "free, in CI"
		return it, true
	}
	capH := float64(b.Seconds) / 3600
	it.Hours, it.Estimate = capH, "the cap of the class"
	if e.Results != nil && e.Results.WallTimeS > 0 {
		it.Hours, it.Estimate = math.Min(capH, math.Ceil(e.Results.WallTimeS*1.25/360)/10), "the last run and a quarter more"
	}
	it.Approval = it.new && b.Name == "gpu-medium"
	return it, true
}

// Plan is what aik exp plan prints: the experiments of cpu-small that CI runs, the batches of the GPU classes that fit
// the month's remaining budget, and the experiments that wait for the next month.
type Plan struct {
	Month     string     `json:"month"`
	Cap       float64    `json:"cap_usd"`
	Spent     float64    `json:"spent_usd"`
	Remaining float64    `json:"remaining_usd"`
	CPU       []PlanItem `json:"cpu,omitempty"`
	Batches   []Batch    `json:"batches,omitempty"`
	Waiting   []PlanItem `json:"waiting,omitempty"`
}

// MakePlan packs the items of the GPU classes into one batch for each class, the experiments that never ran first and
// then those whose inputs changed, each in the order given, while the price stays within the month's remaining
// budget. An item that does not fit waits, and a smaller one after it may still fit. The compute register may be nil
// when no item is of a GPU class.
func MakePlan(items []PlanItem, comp *Compute, ledger []Batch, now time.Time) (*Plan, error) {
	p := &Plan{Month: now.Format("2006-01")}
	var gpu []PlanItem
	for _, it := range items {
		if it.Hours == 0 {
			p.CPU = append(p.CPU, it)
		} else {
			gpu = append(gpu, it)
		}
	}
	if len(gpu) == 0 {
		return p, nil
	}
	if comp == nil {
		return nil, fmt.Errorf("%s is not there: the GPU classes have no price and the month no cap", ComputeFile)
	}
	usd, ok := comp.CapOf(p.Month)
	if !ok {
		return nil, fmt.Errorf("%s has no cap for %s", ComputeFile, p.Month)
	}
	p.Cap, p.Spent = usd, round2(Spent(ledger, p.Month))
	p.Remaining = round2(math.Max(0, p.Cap-p.Spent))
	sort.SliceStable(gpu, func(i, j int) bool { return gpu[i].new && !gpu[j].new })
	left := p.Remaining
	byClass := map[string]*Batch{}
	for _, it := range gpu {
		in, ok := comp.Classes[it.Class]
		if !ok {
			return nil, fmt.Errorf("%s has no machine for the class %s", ComputeFile, it.Class)
		}
		it.USD = round2(it.Hours * in.USDPerHour)
		if it.USD > left+1e-9 {
			p.Waiting = append(p.Waiting, it)
			continue
		}
		left -= it.USD
		b := byClass[it.Class]
		if b == nil {
			b = &Batch{ID: "b-" + now.Format("2006-01-02") + "-" + it.Class, Date: now.Format(time.DateOnly), Class: it.Class}
			byClass[it.Class] = b
		}
		b.Items = append(b.Items, it)
		b.Experiments = append(b.Experiments, it.Experiment)
		b.GPUHours = round2(b.GPUHours + it.Hours)
		b.USD = round2(b.USD + it.USD)
	}
	for _, name := range sortedKeys(byClass) {
		p.Batches = append(p.Batches, *byClass[name])
	}
	return p, nil
}

func round2(x float64) float64 { return math.Round(x*100) / 100 }

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
