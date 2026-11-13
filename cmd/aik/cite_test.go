// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tamnd/ai-agent/cite"
)

func TestCiteVerdict(t *testing.T) {
	root := t.TempDir()
	rs := []cite.Record{
		{Claim: "c-a", Work: "w", Locator: "Section 1", Key: "k1", Pass: true},
		{Claim: "c-b", Work: "w", Locator: "Section 2", Key: "k2", Pass: true},
	}
	if err := cite.WriteRecords(root, "T", rs); err != nil {
		t.Fatal(err)
	}
	write := func(s string) string {
		p := filepath.Join(t.TempDir(), "v.json")
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	good := write(`[{"tag":"T","key":"k1","verdict":"supported"},{"tag":"T","key":"gone","verdict":"unsupported"}]`)
	bad := write(`[{"tag":"T","key":"k2","verdict":"maybe"}]`)
	for _, c := range []struct {
		args []string
		want int
	}{
		{[]string{"cite", "verdict", "-root", root, "-file", good}, 2},
		{[]string{"cite", "verdict", "-root", root, "-file", bad, "-verifier", "codex:m", "-run", "r1"}, 2},
		{[]string{"cite", "verdict", "-root", root, "-file", good, "-verifier", "codex:m", "-run", "r1"}, 0},
		{[]string{"cite", "passages", "-root", root}, 2},
	} {
		if got := run(c.args); got != c.want {
			t.Errorf("aik %v exits with %d, want %d", c.args, got, c.want)
		}
	}
	ix, err := cite.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	a := ix.Records["T"][cite.RecordID("c-a", "w", "Section 1")]
	if a.Verdict != cite.Supported || a.Verifier != "codex:m" || a.Run != "r1" {
		t.Errorf("c-a = %+v, want supported by codex:m in r1", a)
	}
	if b := ix.Records["T"][cite.RecordID("c-b", "w", "Section 2")]; b.Verdict != "" {
		t.Errorf("c-b has the verdict %q of an invalid file", b.Verdict)
	}
}
