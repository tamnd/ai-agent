// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/tamnd/ai-agent/contract"
	"github.com/tamnd/ai-agent/lang"
)

// cmdOracle runs the oracle of a problem (spec 09.5). Without -in, it runs the oracle on each shared case and checks that it agrees with the case (rule S02). With -in, it reads one object of arguments for each line and prints the output of the oracle for each line.
func cmdOracle(args []string) error {
	f := newFlags("oracle", "[flags] [problem ...]")
	root := f.String("root", ".", "the root of the content repository")
	in := f.String("in", "", "a JSONL file of arguments or of cases, one for each line, or - for stdin; print the oracle output of each line")
	unsafe := f.Bool("unsafe-local", false, "run the oracle on this machine, not in the sandbox; CI sets CI=true instead")
	sandbox := f.String("sandbox", "auto", "the container engine of the sandbox: docker, podman, or none")
	images := f.String("images", "manifests/images.yaml", "the manifest of the runner images")
	if err := parse(f, args); err != nil {
		return err
	}
	if *in != "" && f.NArg() != 1 {
		return usageError{"-in needs exactly one problem"}
	}
	c, err := load(*root)
	if err != nil {
		return err
	}
	slugs := f.Args()
	if len(slugs) == 0 {
		for _, n := range c.Nodes {
			if contract.Exists(n.Dir) {
				slugs = append(slugs, n.Slug)
			}
		}
		sort.Strings(slugs)
	}
	box, err := openSandbox(*sandbox, under(c.Root, *images), *unsafe)
	if err != nil {
		return err
	}
	if box == nil && !*unsafe && os.Getenv("CI") != "true" {
		return usageError{"the oracle is code from the repository; use the sandbox or CI, or give -unsafe-local"}
	}
	if box != nil {
		defer box.Reap()
		if err := box.Ready("python"); err != nil {
			return fmt.Errorf("the oracle runs in the Python image: %w", err)
		}
	}
	bad := 0
	for _, s := range slugs {
		n := c.BySlug[s]
		if n == nil || !contract.Exists(n.Dir) {
			return usageError{fmt.Sprintf("%s is not a problem with contract.yaml", s)}
		}
		k, cases, err := contract.Load(n.Dir)
		if err != nil {
			return fmt.Errorf("%s: %w", s, err)
		}
		o, err := k.StartOracle(lang.CheckerWrap(box, k))
		if err != nil {
			if *in == "" && errors.Is(err, contract.ErrNoOracle) {
				fmt.Printf("skip %s: %v\n", s, err)
				continue
			}
			return fmt.Errorf("%s: %w", s, err)
		}
		if *in != "" {
			err = oracleLines(o, k, *in)
		} else {
			var fails int
			fails, err = oracleCases(o, k, cases, box, s)
			bad += fails
			if err == nil {
				fails, err = oraclesAgree(k, box, s)
				bad += fails
			}
		}
		o.Close()
		if err != nil {
			return fmt.Errorf("%s: %w", s, err)
		}
	}
	if bad > 0 {
		return errFindings
	}
	return nil
}

// oracleCases checks the oracle on each shared case (S02) and prints one line for the problem and one for each disagreement.
func oracleCases(o *contract.Oracle, k *contract.Contract, cases []contract.Case, box *lang.Sandbox, slug string) (int, error) {
	var custom *contract.Custom
	if k.Checker.Kind == "custom" {
		var err error
		if custom, err = k.StartCustom(lang.CheckerWrap(box, k)); err != nil {
			return 0, err
		}
		defer custom.Close()
	}
	var msgs []string
	for _, cs := range cases {
		reason, out, err := k.Agree(o, cs, custom)
		if err != nil {
			msgs = append(msgs, fmt.Sprintf("S02 case %s: %v", cs.ID(), err))
			continue
		}
		if reason != "" {
			msgs = append(msgs, fmt.Sprintf("S02 case %s: the oracle gives %s: %s", cs.ID(), out, reason))
		}
	}
	state := "pass"
	if len(msgs) > 0 {
		state = "FAIL"
	}
	fmt.Printf("%-4s %s: the oracle %s agrees with %d of %d cases\n", state, slug, o.File, len(cases)-len(msgs), len(cases))
	for _, m := range msgs {
		fmt.Println(indent(m))
	}
	return len(msgs), nil
}

// oracleLines runs the oracle on each line of the file and prints {"args": ..., "output": ...} for each line. A line is the object of the arguments, or a case with args and T.
func oracleLines(o *contract.Oracle, k *contract.Contract, path string) error {
	var r io.Reader = os.Stdin
	if path != "-" {
		fh, err := os.Open(path)
		if err != nil {
			return err
		}
		defer fh.Close()
		r = fh
	}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	enc := json.NewEncoder(os.Stdout)
	for line := 1; sc.Scan(); line++ {
		if len(sc.Bytes()) == 0 {
			continue
		}
		var a map[string]json.RawMessage
		if err := json.Unmarshal(sc.Bytes(), &a); err != nil {
			return fmt.Errorf("line %d: %w", line, err)
		}
		t := "int"
		if _, ok := a["args"]; ok {
			var c contract.Case
			if err := json.Unmarshal(sc.Bytes(), &c); err != nil {
				return fmt.Errorf("line %d: %w", line, err)
			}
			a, t = c.Args, c.Type()
		}
		a, err := k.OracleArgs(a, t)
		if err != nil {
			return fmt.Errorf("line %d: %w", line, err)
		}
		out, err := o.Run(a, k.OutName())
		if err != nil {
			return fmt.Errorf("line %d: %w", line, err)
		}
		if err := enc.Encode(map[string]json.RawMessage{"args": json.RawMessage(sc.Bytes()), "output": out}); err != nil {
			return err
		}
	}
	return sc.Err()
}

// oraclesAgree checks that the Python oracles of the problem agree with each other on each tiny and small input of each generator (rule S07). It needs two oracles and cases/generated.yaml; else it does nothing. The output of each other oracle must pass the checker with the output of the first oracle as expect.
func oraclesAgree(k *contract.Contract, box *lang.Sandbox, slug string) (int, error) {
	files := k.OracleFiles()
	if len(files) < 2 {
		return 0, nil
	}
	g, err := k.LoadGenerated()
	if err != nil || g == nil {
		return 0, err
	}
	wrap := lang.CheckerWrap(box, k)
	gen, err := k.StartGenerator(wrap)
	if err != nil {
		return 0, err
	}
	defer gen.Close()
	var oracles []*contract.Oracle
	defer func() {
		for _, o := range oracles {
			o.Close()
		}
	}()
	for _, f := range files {
		o, err := k.StartOracleFile(f, wrap)
		if err != nil {
			return 0, err
		}
		oracles = append(oracles, o)
	}
	var custom *contract.Custom
	if k.Checker.Kind == "custom" {
		if custom, err = k.StartCustom(wrap); err != nil {
			return 0, err
		}
		defer custom.Close()
	}
	var msgs []string
	total := 0
	for _, q := range g.Plan(k, []string{"tiny", "small"}, g.Seeds.PR) {
		cs, err := gen.Make(q)
		if err != nil {
			return 0, err
		}
		for _, c := range cs {
			total++
			args, err := k.OracleArgs(c.Args, c.Type())
			if err != nil {
				return 0, err
			}
			want, err := oracles[0].Run(args, k.OutName())
			if err != nil {
				msgs = append(msgs, fmt.Sprintf("S07 case %s: %v", c.Name, err))
				continue
			}
			c.Expect = want
			for _, o := range oracles[1:] {
				got, err := o.Run(args, k.OutName())
				if err == nil {
					var reason string
					if reason, err = k.Verdict(c, got, custom); err == nil && reason != "" {
						err = fmt.Errorf("%s gives %s and %s gives %s: %s", o.File, short(got), oracles[0].File, short(want), reason)
					}
				}
				if err != nil {
					msgs = append(msgs, fmt.Sprintf("S07 case %s %s: %v", c.Name, short(argsJSON(c.Args)), err))
				}
			}
			if len(msgs) >= 5 {
				break
			}
		}
		if len(msgs) >= 5 {
			break
		}
	}
	state := "pass"
	if len(msgs) > 0 {
		state = "FAIL"
	}
	if len(msgs) >= 5 {
		fmt.Printf("%-4s %s: the oracles %s disagree on %d of the first %d tiny and small inputs, and the check stops there\n", state, slug, strings.Join(files, ", "), len(msgs), total)
	} else {
		fmt.Printf("%-4s %s: the oracles %s agree on %d of %d tiny and small inputs\n", state, slug, strings.Join(files, ", "), total-len(msgs), total)
	}
	for _, m := range msgs {
		fmt.Println(indent(m))
	}
	return len(msgs), nil
}
