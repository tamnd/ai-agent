// SPDX-License-Identifier: Apache-2.0

package status

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/ai-agent/facts"
	"github.com/tamnd/ai-agent/node"
)

func TestDataLadder(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "nodes", "m")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, facts.File), []byte("[]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	env := Env{Corpus: &node.Corpus{Root: root}}
	primary := func(id string, v any) facts.Fact {
		f := facts.Fact{ID: id, Sources: []facts.Source{{Src: "card", Kind: facts.Primary, Loc: "p. 1", Value: v}}}
		facts.Compute(&f)
		return f
	}
	n := &node.Node{Dir: dir, Front: node.Front{Kind: "model", Tag: "7TQYMD"}}
	if got := Data(&node.Node{Front: node.Front{Kind: "concept"}}, env); got != nil {
		t.Errorf("a concept has the D ladder: %+v", got)
	}
	level := func(want int, next string) {
		t.Helper()
		r := Data(n, env)
		if r.Level != want || r.Label != "D"+string(rune('0'+want)) {
			t.Fatalf("level %s, want D%d; next %q", r.Label, want, r.Next)
		}
		if next != "" && !strings.Contains(strings.Join(r.Next, "; "), next) {
			t.Errorf("D%d next is %q, want %q", want, r.Next, next)
		}
	}
	level(0, "(D02)")
	n.Facts = []facts.Fact{primary("developer", "Example Lab")}
	level(0, "add the facts released, architecture")
	for _, id := range []string{"released", "architecture", "modalities"} {
		n.Facts = append(n.Facts, primary(id, "x"))
	}
	p := primary("params.total", 7e9)
	p.Unit = "parameters"
	n.Facts = append(n.Facts, p, primary("access", "open weights"), primary("licence", "MIT"))
	n.Facts[len(n.Facts)-1].Sources[0].Loc = ""
	level(1, "a locator (D04)")
	n.Facts[len(n.Facts)-1].Sources[0].Loc = "p. 2"
	level(1, "an as_of date (D07)")
	n.Facts[len(n.Facts)-2].AsOf, n.Facts[len(n.Facts)-1].AsOf = "2026-11-13", "2026-11-13"
	level(2, "no review record reviews/7TQYMD/facts.yaml")
	h, err := Hash(filepath.Join(dir, facts.File))
	if err != nil {
		t.Fatal(err)
	}
	rev := filepath.Join(root, "reviews", "7TQYMD", "facts.yaml")
	if err := os.MkdirAll(filepath.Dir(rev), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rev, []byte("part: facts\nhash: "+h+"\nreviewer: someone\ndate: 2026-11-13\nverdict: approve\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	level(3, "")
}
