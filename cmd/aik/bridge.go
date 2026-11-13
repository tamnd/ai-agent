// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/tamnd/ai-agent/bridge"
)

// maxIndex is the largest index of a neighbour that aik bridge sync reads.
const maxIndex = 256 << 20

// cmdBridge keeps the bridge cards of spec 5.9.
func cmdBridge(args []string) error {
	if len(args) == 0 || args[0] != "sync" {
		return usageError{"use aik bridge sync al|m2|pg"}
	}
	f := newFlags("bridge sync", "[flags] al|m2|pg ...")
	root := f.String("root", ".", "the root of the content repository")
	from := f.String("from", "", "read the index from this file, not from the neighbour; name one site")
	dry := f.Bool("n", false, "print what would change, and write nothing")
	if err := parse(f, args[1:]); err != nil {
		return err
	}
	ids := f.Args()
	if len(ids) == 0 {
		return usageError{"name a site: al, m2 or pg"}
	}
	if *from != "" && len(ids) != 1 {
		return usageError{"-from reads the index of one site; name one site"}
	}
	for _, id := range ids {
		s := bridge.SiteOf(id)
		if s == nil {
			return usageError{fmt.Sprintf("%q is not a neighbour; use al, m2 or pg", id)}
		}
		var b []byte
		var err error
		if *from != "" {
			b, err = os.ReadFile(*from)
		} else {
			b, err = fetchIndex(s.URL())
		}
		if err != nil {
			return fmt.Errorf("%s: %w", s.Host, err)
		}
		idx, err := bridge.ParseIndex(b)
		if err != nil {
			return fmt.Errorf("%s: %w", s.Host, err)
		}
		scope, err := bridge.LoadScope(*root, s)
		if err != nil {
			return err
		}
		dir := *root
		if *dry {
			if dir, err = dryCopy(*root); err != nil {
				return err
			}
			defer os.RemoveAll(dir)
		}
		rep, err := bridge.Sync(dir, s, idx, scope, time.Now().UTC().Format(time.DateOnly))
		if err != nil {
			return err
		}
		printSync(s, rep, len(idx.Nodes))
	}
	return nil
}

func printSync(s *bridge.Site, rep *bridge.Report, items int) {
	commit := rep.Commit
	if len(commit) > 12 {
		commit = commit[:12]
	}
	fmt.Printf("%s: %d bridge cards from %d items at %s\n", s.Host, rep.Cards, items, commit)
	for _, l := range []struct {
		what string
		xs   []string
	}{{"added", rep.Added}, {"renamed", rep.Renamed}, {"changed", rep.Changed}, {"retired (broken-bridge)", rep.Retired}, {"back", rep.Back}} {
		if len(l.xs) == 0 {
			continue
		}
		fmt.Printf("  %s: %d\n", l.what, len(l.xs))
		for i, x := range l.xs {
			if i == 20 {
				fmt.Printf("    and %d more\n", len(l.xs)-20)
				break
			}
			fmt.Printf("    %s\n", x)
		}
	}
}

// dryCopy copies the bridge cards to a temporary root, so that a dry run can sync without a change to the content repository.
func dryCopy(root string) (string, error) {
	dir, err := os.MkdirTemp("", "aik-bridge-")
	if err != nil {
		return "", err
	}
	src := os.DirFS(root + "/" + bridge.Dir)
	if err := os.CopyFS(dir+"/"+bridge.Dir, src); err != nil && !errors.Is(err, os.ErrNotExist) {
		os.RemoveAll(dir)
		return "", err
	}
	return dir, nil
}

// fetchIndex reads the index of a neighbour over HTTPS.
func fetchIndex(url string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "aik/"+strings.TrimPrefix(ver(), "v")+" (+https://github.com/tamnd/ai-agent)")
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxIndex+1))
	if err != nil {
		return nil, err
	}
	if len(b) > maxIndex {
		return nil, fmt.Errorf("GET %s: the index is larger than %d MB", url, maxIndex>>20)
	}
	return b, nil
}
