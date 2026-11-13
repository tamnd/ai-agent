// Command aik is the toolchain of tamnd/ai (spec 18.3).
//
// Exit codes: 0 means success, 1 means that a check found a problem, 2 means a usage error and 3 means that the command is planned but not built yet.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"sort"
	"strings"
)

// version is set by the release build with -ldflags "-X main.version=v0.1.0".
var version = ""

func ver() string {
	if version != "" {
		return version
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return "dev"
}

type command struct {
	name string
	help string
	run  func(args []string) error
}

// planned are the commands of spec 18.3 that are not built yet, with the milestone that builds them.
var planned = map[string]string{
	"audit":      "M0",
	"review":     "M0",
	"bench":      "M2",
	"build":      "M1",
	"ingest":     "M1",
	"universe":   "M1",
	"coverage":   "M1",
	"queue":      "M1",
	"agent":      "M1",
	"prov":       "M1",
	"quarantine": "M1",
	"search":     "M1",
	"release":    "M1",
	"bump":       "M1",
	"mcp":        "M3",
}

var commands []command

func init() {
	commands = []command{
		{"new", "make a node directory with a provisional id and the sections of its kind", cmdNew},
		{"fmt", "put node files, registers and topics in canonical form", cmdFmt},
		{"mint", "give each provisional id a permanent tag", cmdMint},
		{"mv", "rename a node or move it to another area; the old URL redirects", cmdMv},
		{"check", "run the audit rules, and the scene checks with -viz", cmdCheck},
		{"status", "show the ladders of a node and what the next level needs", cmdStatus},
		{"gen", "write the build tree of each language to build/lang", cmdGen},
		{"test", "run the native tests and the shared cases", cmdTest},
		{"lean", "build the Lean package and check its axioms: lean build", cmdLean},
		{"exp", "list and run the experiments: exp list | run", cmdExp},
		{"shapes", "write the shape fixtures of an architecture and show its counts: shapes sample | show", cmdShapes},
		{"cite", "check each claim against the stored text of its source: cite <slug>... | -all | show <work> [<locator>]", cmdCite},
		{"results", "print the computed results table of a benchmark version: results table benchmark@version", cmdResults},
		{"bridge", "refresh the bridge cards from a neighbour's index: bridge sync al|m2|pg", cmdBridge},
		{"viz", "bundle and check the scenes: viz build | check", cmdViz},
		{"oracle", "run the oracle of a problem on its cases or on given arguments", cmdOracle},
		{"site", "render and preview the site: site build | serve | check", cmdSite},
		{"version", "print the version", func([]string) error { fmt.Println("aik", ver()); return nil }},
		{"help", "show this help", func([]string) error { usage(os.Stdout); return nil }},
	}
}

func usage(w io.Writer) {
	fmt.Fprintf(w, "aik is the toolchain of https://ai.tamnd.com.\n\nUsage:\n\n  aik <command> [flags]\n\nCommands:\n\n")
	for _, c := range commands {
		fmt.Fprintf(w, "  %-9s %s\n", c.name, c.help)
	}
	var later []string
	for k := range planned {
		later = append(later, k)
	}
	sort.Strings(later)
	fmt.Fprintf(w, "\nPlanned, not built yet: %s.\n\nRun \"aik <command> -h\" for the flags of a command.\n", strings.Join(later, ", "))
}

// errFindings means that a check ran and found problems. main exits with 1 and prints nothing more.
var errFindings = errors.New("findings")

type usageError struct{ msg string }

func (e usageError) Error() string { return e.msg }

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		usage(os.Stderr)
		return 2
	}
	name := args[0]
	if name == "-h" || name == "--help" {
		usage(os.Stdout)
		return 0
	}
	if name == "-v" || name == "--version" {
		fmt.Println("aik", ver())
		return 0
	}
	for _, c := range commands {
		if c.name != name {
			continue
		}
		err := c.run(args[1:])
		switch {
		case err == nil:
			return 0
		case errors.Is(err, errFindings):
			return 1
		case errors.Is(err, flag.ErrHelp):
			return 0
		}
		var ue usageError
		if errors.As(err, &ue) {
			if ue.msg != "" {
				fmt.Fprintln(os.Stderr, "aik "+name+": "+ue.msg)
			}
			return 2
		}
		fmt.Fprintln(os.Stderr, "aik "+name+": "+err.Error())
		return 1
	}
	if m, ok := planned[name]; ok {
		fmt.Fprintf(os.Stderr, "aik %s is planned for milestone %s and is not built yet. See https://github.com/tamnd/ai-agent/milestones.\n", name, m)
		return 3
	}
	fmt.Fprintf(os.Stderr, "aik: unknown command %q\n\n", name)
	usage(os.Stderr)
	return 2
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func readJSON(data []byte, v any) error { return json.Unmarshal(data, v) }
