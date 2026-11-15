// SPDX-License-Identifier: Apache-2.0

package check

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/ai-agent/node"
)

func TestFixtures(t *testing.T) {
	n := &node.Node{Dir: t.TempDir(), Rel: "nodes/x/toy"}
	n.Slug = "toy"
	put := func(name string, b []byte) {
		p := filepath.Join(n.Dir, "fixtures", name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	msgs := func() string {
		r := &run{}
		r.fixtures(n)
		var out []string
		for _, f := range r.out {
			out = append(out, f.Rule+" "+f.Msg)
		}
		return strings.Join(out, "\n")
	}
	if got := msgs(); got != "" {
		t.Errorf("a node with no fixtures: %s", got)
	}
	put("make.py", []byte("pass\n"))
	if got := msgs(); !strings.Contains(got, "no manifest.yaml") {
		t.Errorf("no manifest: %s", got)
	}
	data := []byte("arrays")
	h := sha256.Sum256(data)
	put("a.npz", data)
	put("b.npz", data)
	put("manifest.yaml", []byte("files:\n  a.npz: {sha256: "+hex.EncodeToString(h[:])+"}\n  gone.npz: {sha256: 00}\n"))
	got := msgs()
	if !strings.Contains(got, "gone.npz, which is not in fixtures/") || !strings.Contains(got, "the manifest has no hash of the file") || strings.Contains(got, "a.npz") {
		t.Errorf("the manifest: %s", got)
	}
	put("a.npz", []byte("other"))
	put("big.bin", make([]byte, MaxFixtures))
	if got := msgs(); !strings.Contains(got, "its sha256 is not") || !strings.Contains(got, "at most 1024 KB") {
		t.Errorf("a changed file and too many bytes: %s", got)
	}
}
