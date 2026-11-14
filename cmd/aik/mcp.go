// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/tamnd/ai-agent/mcp"
)

// cmdMCP serves the MCP server of 16.10 over a content tree: on stdio for a local client such as Claude Code, or over
// streamable HTTP with -http. It only reads.
func cmdMCP(args []string) error {
	f := newFlags("mcp", "[flags]")
	root := f.String("root", ".", "the root of the content repository")
	base := f.String("base", "https://ai.tamnd.com", "the base URL of the site, for the URLs in the results")
	asOf := f.String("as-of", "", "the date the data is as of, YYYY-MM-DD; today by default")
	addr := f.String("http", "", "serve over streamable HTTP on this address, such as localhost:8151, not on stdio")
	if err := parse(f, args); err != nil {
		return err
	}
	if f.NArg() > 0 {
		return usageError{"aik mcp takes no arguments"}
	}
	if *asOf == "" {
		*asOf = time.Now().UTC().Format("2006-01-02")
	} else if _, err := time.Parse("2006-01-02", *asOf); err != nil {
		return usageError{"-as-of is a date, YYYY-MM-DD"}
	}
	c, err := load(*root)
	if err != nil {
		return err
	}
	d, err := mcp.Load(c, mcp.Options{Version: ver(), BaseURL: strings.TrimRight(*base, "/"), AsOf: *asOf})
	if err != nil {
		return err
	}
	srv := mcp.New(d)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if *addr == "" {
		return mcp.Stdio(ctx, srv)
	}
	hs := &http.Server{Addr: *addr, Handler: mcp.Handler(srv), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		_ = hs.Close()
	}()
	fmt.Fprintf(os.Stderr, "aik mcp: serving %d nodes and %d cards on http://%s\n", len(c.Nodes), len(c.Cards), *addr)
	if err := hs.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
