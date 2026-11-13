// SPDX-License-Identifier: Apache-2.0

package exp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// DataDir is the directory of the pinned data, relative to the content root. aik exp run fetches each file there once, and checks its hash before each run.
const DataDir = "build/data"

// Fetch gives the body of a URL. aik gives an HTTPS client with a generic user agent. The tests give a fake.
type Fetch func(ctx context.Context, url string) (io.ReadCloser, error)

var (
	pinID    = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	fileName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
)

// Prepare makes each pinned dataset of the design ready in cache/<pin>/ and checks the sha256 of each file (rule R02). It fetches a file only when the cache does not have it with the right hash. It gives the absolute directory of each pin, by pin id. A generator needs no preparation.
//
// Prepare runs before the sandbox, because the sandbox has no network (spec 9.4.2). A file whose hash does not match stops the run before training.
func Prepare(ctx context.Context, pins map[string]*Pin, d *Design, cache string, fetch Fetch) (map[string]string, error) {
	out := map[string]string{}
	for i, data := range d.Data {
		if data.Pin == "" {
			continue
		}
		id := PinID(data)
		if _, done := out[id]; done {
			continue
		}
		p := pins[id]
		switch {
		case p == nil:
			return nil, fmt.Errorf("data %d: the pin %q is not in %s", i+1, data.Pin, PinsFile)
		case !pinID.MatchString(id):
			return nil, fmt.Errorf("data %d: the pin id %q has characters other than a to z, 0 to 9 and -", i+1, id)
		case p.Licence.SPDX == "" || strings.EqualFold(p.Licence.SPDX, "unknown") || p.Mirror == "" || p.Mirror == "unknown":
			return nil, fmt.Errorf("data %d: the licence of the pin %s is not resolved (9.5)", i+1, id)
		case len(p.Files) == 0:
			return nil, fmt.Errorf("data %d: the pin %s lists no files", i+1, id)
		}
		dir, err := filepath.Abs(filepath.Join(cache, id))
		if err != nil {
			return nil, err
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
		for _, f := range p.Files {
			if err := prepareFile(ctx, id, dir, f, fetch); err != nil {
				return nil, err
			}
		}
		out[id] = dir
	}
	return out, nil
}

func prepareFile(ctx context.Context, id, dir string, f PinFile, fetch Fetch) error {
	if !fileName.MatchString(f.Name) {
		return fmt.Errorf("the pin %s: the file name %q is not a plain file name", id, f.Name)
	}
	want := strings.ToLower(strings.TrimPrefix(f.SHA256, "sha256:"))
	if len(want) != 64 {
		return fmt.Errorf("the pin %s: the file %s has no sha256; aik exp run checks sha256", id, f.Name)
	}
	path := filepath.Join(dir, f.Name)
	if got, err := hashFile(path); err == nil && got == want {
		return nil
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if f.URL == "" {
		return fmt.Errorf("the pin %s: the file %s is not in %s and has no url", id, f.Name, dir)
	}
	if !strings.HasPrefix(f.URL, "https://") {
		return fmt.Errorf("the pin %s: the url of %s is not https", id, f.Name)
	}
	body, err := fetch(ctx, f.URL)
	if err != nil {
		return fmt.Errorf("the pin %s: %s: %w", id, f.Name, err)
	}
	defer body.Close()
	tmp, err := os.CreateTemp(dir, "."+f.Name+".part-")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	h := sha256.New()
	var r io.Reader = body
	if f.Bytes > 0 {
		r = io.LimitReader(body, f.Bytes+1)
	}
	n, err := io.Copy(io.MultiWriter(tmp, h), r)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return fmt.Errorf("the pin %s: %s: %w", id, f.Name, err)
	}
	if f.Bytes > 0 && n != f.Bytes {
		return fmt.Errorf("the pin %s: %s has %d bytes, and the pin gives %d (rule R02)", id, f.Name, n, f.Bytes)
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != want {
		return fmt.Errorf("the pin %s: %s has the sha256 %s, and the pin gives %s (rule R02)", id, f.Name, got, want)
	}
	return os.Rename(tmp.Name(), path)
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
