// SPDX-License-Identifier: Apache-2.0

package cite

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/zeebo/blake3"
)

// Store is a local copy of the raw store ai-raw (spec 13.6): each fetched object once, named by its BLAKE3 hash,
// with its metadata in <hash>.meta.json and its extracted text in <hash>.doc.json. Nothing in it is published.
type Store struct{ Dir string }

// DefaultStore is the store in $AIK_RAW, or else in the user cache directory.
func DefaultStore() Store {
	if d := os.Getenv("AIK_RAW"); d != "" {
		return Store{d}
	}
	d, err := os.UserCacheDir()
	if err != nil {
		d = os.TempDir()
	}
	return Store{filepath.Join(d, "aik", "raw")}
}

// Meta says where an object came from.
type Meta struct {
	URL         string `json:"url"`
	Fetched     string `json:"fetched"`
	ContentType string `json:"content_type"`
	Size        int    `json:"size"`
	Licence     string `json:"licence_class,omitempty"`
	Fetcher     string `json:"fetcher"`
}

// Hash is the BLAKE3 hash of b in hex.
func Hash(b []byte) string {
	h := blake3.Sum256(b)
	return hex.EncodeToString(h[:])
}

func (s Store) path(hash, ext string) string { return filepath.Join(s.Dir, hash[:2], hash+ext) }

// Put stores b and its metadata and returns its hash.
func (s Store) Put(b []byte, m Meta) (string, error) {
	h := Hash(b)
	if err := os.MkdirAll(filepath.Dir(s.path(h, "")), 0o755); err != nil {
		return "", err
	}
	if _, err := os.Stat(s.path(h, "")); err != nil {
		if err := writeAtomic(s.path(h, ""), b); err != nil {
			return "", err
		}
	}
	m.Size = len(b)
	mb, _ := json.MarshalIndent(m, "", "  ")
	return h, writeAtomic(s.path(h, ".meta.json"), append(mb, '\n'))
}

// Has tells if the store holds the object.
func (s Store) Has(hash string) bool {
	_, err := os.Stat(s.path(hash, ""))
	return err == nil
}

// Get reads an object and checks its hash.
func (s Store) Get(hash string) ([]byte, error) {
	b, err := os.ReadFile(s.path(hash, ""))
	if err != nil {
		return nil, err
	}
	if Hash(b) != hash {
		return nil, fmt.Errorf("the object %s in %s does not have its hash", hash, s.Dir)
	}
	return b, nil
}

// Doc reads the extracted text of an object, or nil when there is none or it was made by another extractor version.
func (s Store) Doc(hash string) *Doc {
	b, err := os.ReadFile(s.path(hash, ".doc.json"))
	if err != nil {
		return nil
	}
	var d Doc
	if json.Unmarshal(b, &d) != nil || !isCurrent(d.Extractor) {
		return nil
	}
	return &d
}

// AnyDoc reads the extracted text of an object as it is stored, whatever extractor made it, or nil when there is none.
func (s Store) AnyDoc(hash string) *Doc {
	b, err := os.ReadFile(s.path(hash, ".doc.json"))
	if err != nil {
		return nil
	}
	var d Doc
	if json.Unmarshal(b, &d) != nil {
		return nil
	}
	return &d
}

// PutDoc stores the extracted text of an object.
func (s Store) PutDoc(hash string, d *Doc) error {
	b, err := json.Marshal(d)
	if err != nil {
		return err
	}
	return writeAtomic(s.path(hash, ".doc.json"), b)
}

func isCurrent(extractor string) bool {
	return len(extractor) > len(ExtractorVersion) && extractor[len(extractor)-len(ExtractorVersion):] == ExtractorVersion
}

func writeAtomic(path string, b []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// MaxObject is the largest object that Fetch keeps. A whole textbook is a source: the authors' PDF of Sutton and
// Barto's Reinforcement Learning is 73 MB.
const MaxObject = 128 << 20

// Fetch gets a URL with the User-Agent of aik and returns the body and its media type. It follows redirects and
// fails on a status other than 200.
func Fetch(ctx context.Context, url, userAgent string) ([]byte, string, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, MaxObject+1))
	if err != nil {
		return nil, "", err
	}
	if len(b) > MaxObject {
		return nil, "", fmt.Errorf("GET %s: the body is larger than %d MB", url, MaxObject>>20)
	}
	mt, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	return b, mt, nil
}

// ErrNotPDF means that a URL for a PDF gave something else, such as a page that asks for a login.
var ErrNotPDF = errors.New("the body is not a PDF")

// Extract reads the text of an object by its format: pdf or html.
func Extract(ctx context.Context, b []byte, format string) (*Doc, error) {
	switch format {
	case "pdf":
		if len(b) < 5 || string(b[:5]) != "%PDF-" {
			return nil, ErrNotPDF
		}
		return ExtractPDF(ctx, b)
	case "html":
		return ExtractHTML(b), nil
	}
	return nil, fmt.Errorf("the format %q has no extractor; use pdf or html", format)
}
