// SPDX-License-Identifier: Apache-2.0

package exp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func pinOf(body string) map[string]*Pin {
	sum := sha256.Sum256([]byte(body))
	return map[string]*Pin{"toy": {
		ID: "toy", Dataset: "toy", Licence: Licence{SPDX: "CC0-1.0"}, Mirror: "allowed",
		Files: []PinFile{{Name: "a.bin", SHA256: hex.EncodeToString(sum[:]), Bytes: int64(len(body)), URL: "https://example.org/a.bin"}},
	}}
}

func TestPrepare(t *testing.T) {
	cache := t.TempDir()
	d := &Design{Data: []Data{{Pin: "S:toy"}, {Generator: "gen.py"}}}
	calls := 0
	fetch := func(_ context.Context, url string) (io.ReadCloser, error) {
		calls++
		return io.NopCloser(strings.NewReader("hello")), nil
	}
	dirs, err := Prepare(context.Background(), pinOf("hello"), d, cache, fetch)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dirs["toy"], "a.bin"))
	if err != nil || string(b) != "hello" {
		t.Fatalf("the file is %q, %v", b, err)
	}
	if _, err := Prepare(context.Background(), pinOf("hello"), d, cache, fetch); err != nil || calls != 1 {
		t.Errorf("the second prepare fetched again (%d calls, %v)", calls, err)
	}
	// A changed file in the cache is fetched again.
	if err := os.WriteFile(filepath.Join(dirs["toy"], "a.bin"), []byte("HELLO"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Prepare(context.Background(), pinOf("hello"), d, cache, fetch); err != nil || calls != 2 {
		t.Errorf("a changed file was not fetched again (%d calls, %v)", calls, err)
	}
}

func TestPrepareRefuses(t *testing.T) {
	d := &Design{Data: []Data{{Pin: "S:toy"}}}
	bad := func(_ context.Context, _ string) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("hellp")), nil
	}
	fail := func(_ context.Context, _ string) (io.ReadCloser, error) { return nil, errors.New("no network") }
	for name, c := range map[string]struct {
		pins  map[string]*Pin
		fetch Fetch
		want  string
	}{
		"hash":    {pinOf("hello"), bad, "sha256"},
		"network": {pinOf("hello"), fail, "no network"},
		"missing": {map[string]*Pin{}, bad, "not in"},
		"licence": {func() map[string]*Pin { p := pinOf("hello"); p["toy"].Mirror = "unknown"; return p }(), bad, "licence"},
		"name":    {func() map[string]*Pin { p := pinOf("hello"); p["toy"].Files[0].Name = "../a.bin"; return p }(), bad, "plain file name"},
		"http":    {func() map[string]*Pin { p := pinOf("hello"); p["toy"].Files[0].URL = "http://example.org/a"; return p }(), bad, "https"},
		"nohash":  {func() map[string]*Pin { p := pinOf("hello"); p["toy"].Files[0].SHA256 = ""; return p }(), bad, "no sha256"},
	} {
		cache := t.TempDir()
		_, err := Prepare(context.Background(), c.pins, d, cache, c.fetch)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: the error is %v, want %q", name, err, c.want)
		}
		if name == "hash" {
			if _, err := os.Stat(filepath.Join(cache, "toy", "a.bin")); err == nil {
				t.Error("a file with the wrong hash stayed in the cache")
			}
		}
	}
}
