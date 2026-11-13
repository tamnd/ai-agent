// SPDX-License-Identifier: Apache-2.0

package lang

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/tamnd/ai-agent/contract"
)

// Inputs are the generated cases of a problem, with the oracle output as expect, and what is needed to check a new input: the generators for pre, and the oracle.
type Inputs struct {
	K      *contract.Contract
	G      *contract.Generated
	Gen    *contract.Generator
	Oracle *contract.Oracle
	Cases  []contract.Case
}

// MakeInputs runs the plan with the generators of the problem, and runs the oracle on each input (spec 09.4, 09.5). The caller closes gen and oracle.
func MakeInputs(k *contract.Contract, g *contract.Generated, gen *contract.Generator, oracle *contract.Oracle, plan []contract.GenRequest) (*Inputs, error) {
	if k.Checker.Kind == "any_of" {
		return nil, errors.New("the differential run needs one oracle output, so it does not support the checker any_of")
	}
	in := &Inputs{K: k, G: g, Gen: gen, Oracle: oracle}
	for _, q := range plan {
		cs, err := gen.Make(q)
		if err != nil {
			return nil, err
		}
		for _, c := range cs {
			if err := in.expect(&c); err != nil {
				return nil, fmt.Errorf("case %s: %w", c.Name, err)
			}
			in.Cases = append(in.Cases, c)
		}
	}
	return in, nil
}

// expect sets the expect of the case to the oracle output, and writes the large integers of the case as strings.
func (in *Inputs) expect(c *contract.Case) error {
	args, err := in.K.OracleArgs(c.Args, c.Type())
	if err != nil {
		return err
	}
	out, err := in.Oracle.Run(args, in.K.OutName())
	if err != nil {
		return err
	}
	c.Expect = out
	// The implementations read the integers outside the safe range of JSON as strings, as in a case file.
	safe, err := in.K.SafeCase(*c)
	if err != nil {
		return err
	}
	*c = safe
	return nil
}

// allowed tells if a new input meets the contract: the types, and pre of generated.yaml when there is one.
func (in *Inputs) allowed(c contract.Case) (bool, error) {
	if err := in.K.CheckArgs(c); err != nil {
		return false, nil
	}
	if in.G.Pre == "" {
		return true, nil
	}
	args, err := in.K.OracleArgs(c.Args, c.Type())
	if err != nil {
		return false, err
	}
	reason, err := in.Gen.Pre(in.G.Pre, args)
	return reason == "", err
}

// DiffFailure is one generated case that an implementation fails, with the shrunk case.
type DiffFailure struct {
	Case   contract.Case
	Reason string
	Got    json.RawMessage
	// Shrunk is the smallest failing case that the shrinker found, with the oracle output as expect, and ShrunkReason its reason. Calls is the number of runs that the shrinker made.
	Shrunk       contract.Case
	ShrunkReason string
	ShrunkGot    json.RawMessage
	Calls        int
}

// DiffResult is the result of the differential run of one implementation.
type DiffResult struct {
	Cases, Passed, Skipped int
	Failures               []DiffFailure
	Took                   time.Duration
}

// MaxDiffFailures is the number of failures after which the differential run of one implementation stops. One failure is enough to keep the implementation at I1 (rule I03).
const MaxDiffFailures = 3

// ShrinkBudget is the largest number of runs that the shrinker makes for one failure.
const ShrinkBudget = 400

// Diff runs each generated case on the implementation of the session, and shrinks each failure (spec 09.5, 09.11).
func Diff(ss *Session, in *Inputs) (DiffResult, error) {
	start := time.Now()
	r := DiffResult{}
	for _, c := range in.Cases {
		if !ss.Supports(c) {
			r.Skipped++
			continue
		}
		r.Cases++
		got, reason, err := ss.Run(c)
		if err != nil {
			return r, err
		}
		if reason == "" {
			r.Passed++
			continue
		}
		f := DiffFailure{Case: c, Reason: reason, Got: got}
		f.Shrunk, f.Calls, err = contract.Shrink(c, func(cand contract.Case) (bool, error) {
			ok, err := in.allowed(cand)
			if err != nil || !ok {
				return false, err
			}
			if err := in.expect(&cand); err != nil {
				return false, nil
			}
			_, reason, err := ss.Run(cand)
			return reason != "", err
		}, ShrinkBudget)
		if err != nil {
			return r, err
		}
		if err := in.expect(&f.Shrunk); err != nil {
			return r, err
		}
		f.ShrunkGot, f.ShrunkReason, err = ss.Run(f.Shrunk)
		if err != nil {
			return r, err
		}
		r.Failures = append(r.Failures, f)
		if len(r.Failures) >= MaxDiffFailures {
			break
		}
	}
	r.Took = time.Since(start)
	return r, nil
}

// Regression gives the regression case of a shrunk failure for cases/regress.jsonl (spec 09.11): the name regress-<date>-<short hash>, the node and language that it broke, and the oracle output as expect.
func Regression(k *contract.Contract, f DiffFailure, found string, day time.Time) contract.Case {
	c := f.Shrunk
	if safe, err := k.SafeCase(c); err == nil {
		c = safe
	}
	b, _ := json.Marshal(c.Args)
	h := sha256.Sum256(append([]byte(c.Type()+"/"), b...))
	c.Name = "regress-" + day.Format("2006-01-02") + "-" + hex.EncodeToString(h[:])[:7]
	c.Class, c.File, c.Line, c.Found = nil, "", 0, found
	return c
}
