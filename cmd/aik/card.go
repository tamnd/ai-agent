// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"

	"github.com/tamnd/ai-agent/card"
	"github.com/tamnd/ai-agent/universe"
)

// cmdCard writes the cards of spec 04.6: aik card sync [<catalog.jsonl>...]. It makes the card of each item with
// the outcome card, updates the facts of the cards that exist, and splits a shard that passes 5 MB (20.3). aik card
// drift compares the cards with those of a base tree.
func cmdCard(args []string) error {
	if len(args) > 0 && args[0] == "drift" {
		return cardDrift(args[1:])
	}
	if len(args) == 0 || args[0] != "sync" {
		return usageError{"aik card sync [flags] [<catalog.jsonl>...] | aik card drift -base <dir> [flags]"}
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

// cardDrift is aik card drift -base <dir>: the facts of the cards of base that the cards of root moved beyond their
// tolerance or dropped (14.9). It exits with 1 when there is one, so that the daily job merges its pull request by
// itself only when there is none.
func cardDrift(args []string) error {
	f := newFlags("card drift", "-base <dir> [flags]")
	root := f.String("root", ".", "the root of the content repository with the change")
	base := f.String("base", "", "the root of the same repository before the change, such as a git worktree of main")
	asJSON := f.Bool("json", false, "print the moves as JSON")
	if err := parse(f, args); err != nil {
		return err
	}
	if *base == "" || f.NArg() > 0 {
		return usageError{"aik card drift -base <dir> [-root <dir>] [-json]"}
	}
	old, bad := card.Read(*base)
	if len(bad) > 0 {
		return fmt.Errorf("the base cards: %w", bad[0])
	}
	cur, bad := card.Read(*root)
	if len(bad) > 0 {
		return fmt.Errorf("the cards: %w", bad[0])
	}
	moves := card.Drift(old, cur)
	if *asJSON {
		if moves == nil {
			moves = []card.Move{}
		}
		b, _ := json.Marshal(moves)
		fmt.Println(string(b))
	} else {
		for _, m := range moves {
			fmt.Println(m)
		}
		fmt.Printf("%d cards before, %d after; %d facts of existing cards moved beyond the tolerance or were dropped\n", len(old), len(cur), len(moves))
	}
	if len(moves) > 0 {
		return errFindings
	}
	return nil
}
