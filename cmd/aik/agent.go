// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/tamnd/ai-agent/agent"
)

// cmdAgent shows and checks what the agents of 14 run on: the register of models with the roles, the prompts and the
// run records. The agents are the scripts of work/.
func cmdAgent(args []string) error {
	if len(args) == 0 {
		return usageError{"use aik agent models, aik agent prompts or aik agent runs"}
	}
	switch args[0] {
	case "models":
		return agentModels(args[1:])
	case "prompts":
		return agentPrompts(args[1:])
	case "runs":
		return agentRuns(args[1:])
	}
	return usageError{fmt.Sprintf("aik agent has no subcommand %q; use models, prompts or runs", args[0])}
}

// agentModels prints the model of each role and the auditor of each rung of the ladders, and checks the register by
// the rules of 14.2 to 14.4 and 15.4; a problem exits 1.
func agentModels(args []string) error {
	f := newFlags("agent models", "[flags]")
	root := f.String("root", ".", "the root of the content repository")
	file := f.String("models", "", "the register of models; the default is registers/models.yaml under -root, or AI_MODELS")
	if err := parse(f, args); err != nil {
		return err
	}
	if f.NArg() > 0 {
		return usageError{"aik agent models takes no arguments"}
	}
	p := *file
	if p == "" {
		p = os.Getenv("AI_MODELS")
	}
	if p == "" {
		p = under(*root, agent.ModelsFile)
	}
	m, err := agent.LoadModels(p)
	if err != nil {
		return err
	}
	fmt.Printf("%s, version %s\n\n", p, m.Version)
	for _, r := range agent.Roles {
		val := "-"
		switch {
		case r.Key == "auditor_for":
			val = "by the writer's family, below"
		case r.Key != "":
			refs, err := m.Role(r.Key)
			if err != nil {
				val = "missing"
			} else {
				ss := make([]string, len(refs))
				for i, x := range refs {
					ss[i] = x.String()
				}
				val = strings.Join(ss, ", ")
			}
		}
		note := ""
		if r.Note != "" {
			note = "  " + r.Note
		}
		fmt.Printf("  %-19s %s%s\n", r.Name, val, note)
	}
	for _, hard := range []bool{false, true} {
		name := "writer_ladder"
		if hard {
			name = "writer_ladder_hard"
		}
		l, err := m.Ladder(hard)
		if err != nil {
			continue
		}
		fmt.Printf("\n%s\n", name)
		for i, w := range l {
			a, fb, err := m.AuditorFor([]agent.Ref{w})
			switch {
			case err != nil:
				fmt.Printf("  %d %-24s no auditor: %v\n", i, w, err)
			case fb != nil:
				fmt.Printf("  %d %-24s audited by %s, or %s when it is not available\n", i, w, a, fb)
			default:
				fmt.Printf("  %d %-24s audited by %s; the item waits when it is not available\n", i, w, a)
			}
		}
	}
	if probs := m.Check(); len(probs) > 0 {
		fmt.Println()
		for _, s := range probs {
			fmt.Fprintln(os.Stderr, "  "+s)
		}
		return errFindings
	}
	return nil
}

func agentPrompts(args []string) error {
	f := newFlags("agent prompts", "[flags]")
	dir := f.String("dir", "work/prompts", "the directory of the prompts, <role>/v<n>.md")
	asJSON := f.Bool("json", false, "print the prompts as JSON")
	if err := parse(f, args); err != nil {
		return err
	}
	if f.NArg() > 0 {
		return usageError{"aik agent prompts takes no arguments"}
	}
	ps, err := agent.LoadPrompts(*dir)
	if err != nil {
		return err
	}
	if *asJSON {
		return printJSON(ps)
	}
	if len(ps) == 0 {
		fmt.Printf("no prompts under %s\n", *dir)
		return nil
	}
	for _, p := range ps {
		fmt.Printf("  %-10s %-4s %7d bytes  sha256:%s  %s\n", p.Role, p.Version, p.Bytes, p.SHA256[:12], p.File)
	}
	return nil
}

func agentRuns(args []string) error {
	f := newFlags("agent runs", "[flags]")
	root := f.String("root", ".", "the root of the content repository")
	role := f.String("role", "", "only the runs of this role, writer or verifier")
	n := f.Int("n", 20, "print this many runs, newest first; -1 prints all")
	asJSON := f.Bool("json", false, "print the runs as JSON")
	if err := parse(f, args); err != nil {
		return err
	}
	if f.NArg() > 0 {
		return usageError{"aik agent runs takes no arguments"}
	}
	runs, err := agent.LoadRuns(*root)
	if err != nil {
		return err
	}
	var out []agent.Run
	for _, r := range runs {
		if *role == "" || r.Role == *role {
			out = append(out, r)
		}
	}
	if *n >= 0 && len(out) > *n {
		out = out[:*n]
	}
	if *asJSON {
		if out == nil {
			out = []agent.Run{}
		}
		return printJSON(out)
	}
	if len(out) == 0 {
		fmt.Printf("no run records in %s\n", agent.RunsDir)
		return nil
	}
	for _, r := range out {
		counts := r.Status
		if counts == nil {
			counts = r.Verdict
		}
		spent := ""
		if r.Spent > 0 {
			spent = fmt.Sprintf(" $%.2f", r.Spent)
		}
		fmt.Printf("  %-28s %s %-9s %-10s %3d items, %3d nodes%s  %s\n", r.Run, r.Date, r.Role, r.Prompt, r.Items, r.Slugs, spent, countsLine(counts))
	}
	return nil
}

func countsLine(m map[string]int) string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	parts := make([]string, len(ks))
	for i, k := range ks {
		parts[i] = fmt.Sprintf("%s %d", k, m[k])
	}
	return strings.Join(parts, ", ")
}
