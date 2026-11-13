// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/tamnd/ai-agent/universe"
)

// cmdUniverse builds the universe register of spec 18.3: aik universe merge.
func cmdUniverse(args []string) error {
	if len(args) == 0 || args[0] != "merge" {
		return usageError{"aik universe merge [flags]"}
	}
	f := newFlags("universe merge", "[flags]")
	root := f.String("root", ".", "the root of the content repository")
	date := f.String("date", "", "the date of the decisions of the scope rules (YYYY-MM-DD); the default is today in UTC")
	asJSON := f.Bool("json", false, "print the report as JSON")
	if err := parse(f, args[1:]); err != nil {
		return err
	}
	day, err := runDate(*date)
	if err != nil {
		return err
	}
	if _, err := os.Stat(*root); err != nil {
		return err
	}
	rep, err := universe.Merge(*root, day)
	if err != nil {
		return err
	}
	if *asJSON {
		b, _ := json.Marshal(rep)
		fmt.Println(string(b))
		return nil
	}
	fmt.Printf("%d rows into %d items: %d new, %d updated, %d unchanged; %d conflicts for the curator (%s)\n",
		rep.Rows, rep.Items, rep.New, rep.Updated, rep.Unchanged, rep.Conflicts, universe.ReportFile)
	fmt.Println("outcomes:", counts(rep.ByOutcome))
	fmt.Println("kinds:   ", counts(rep.ByKind))
	if len(rep.Decided) > 0 {
		fmt.Println("decided by rule:", counts(rep.Decided))
	}
	return nil
}

func counts(m map[string]int) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return m[keys[i]] > m[keys[j]] || m[keys[i]] == m[keys[j]] && keys[i] < keys[j] })
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "%s %d", k, m[k])
	}
	return b.String()
}
