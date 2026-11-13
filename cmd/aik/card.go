// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"

	"github.com/tamnd/ai-agent/universe"
)

// cmdCard writes the cards of spec 04.6: aik card sync [<catalog.jsonl>...]. It makes the card of each item with
// the outcome card, updates the facts of the cards that exist, and splits a shard that passes 5 MB (20.3).
func cmdCard(args []string) error {
	if len(args) == 0 || args[0] != "sync" {
		return usageError{"aik card sync [flags] [<catalog.jsonl>...]"}
	}
	f := newFlags("card sync", "[flags] [<catalog.jsonl>...]")
	root := f.String("root", ".", "the root of the content repository")
	date := f.String("date", "", "the date of the decisions that the catalog records (YYYY-MM-DD); the default is today in UTC")
	kind := f.String("kind", "", "sync only the items of this kind")
	source := f.String("source", "", "sync only the items with a row of this source")
	asJSON := f.Bool("json", false, "print the report as JSON")
	if err := parse(f, args[1:]); err != nil {
		return err
	}
	day, err := runDate(*date)
	if err != nil {
		return err
	}
	c, err := load(*root)
	if err != nil {
		return err
	}
	var cur []universe.Curated
	for _, p := range f.Args() {
		items, err := universe.ReadCurated(p)
		if err != nil {
			return err
		}
		cur = append(cur, items...)
	}
	rep, err := universe.Sync(*root, c, cur, day, universe.SyncOptions{Kind: *kind, Source: *source})
	if err != nil {
		return err
	}
	if *asJSON {
		b, _ := json.Marshal(rep)
		fmt.Println(string(b))
		return nil
	}
	fmt.Printf("%d card items: %d cards made, %d updated, %d unchanged; facts %d added, %d changed, %d kept where another source disagrees; %d rows merged\n",
		rep.Items, rep.Created, rep.Updated, rep.Unchanged, rep.Added, rep.Changed, rep.Disagree, rep.Merged)
	for _, n := range rep.Notes {
		fmt.Println("note:", n)
	}
	if rep.Created > 0 {
		fmt.Println("the new cards have provisional tags; run aik mint")
	}
	return nil
}
