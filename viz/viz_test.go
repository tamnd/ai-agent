package viz

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildAndCheck(t *testing.T) {
	b, err := Build("testdata/ok/scene.ts", true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "AlgoScenes") {
		t.Fatalf("the bundle does not register the scene")
	}
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is not installed")
	}
	presets := []Preset{{"empty", []int{}}, {"one", []int{5}}, {"three", []int{3, 1, 2}}}
	r := Check(Scene{Slug: "demo", Bundle: b, Presets: presets, IDs: []string{"L1", "L2"}})
	if len(r.Fails) > 0 {
		t.Fatalf("fails: %v", r.Fails)
	}
	r = Check(Scene{Slug: "demo", Bundle: b, Presets: presets[:1], IDs: []string{"L1", "L2", "L3"}})
	if len(r.Fails) != 2 {
		t.Fatalf("want 2 fails (V10, and V05 for L2 and L3), got %v", r.Fails)
	}
}

func TestOnlyViz(t *testing.T) {
	_, err := Build("testdata/bad/scene.ts", false)
	if err == nil || !strings.Contains(err.Error(), "V01") {
		t.Fatalf("want a V01 error, got %v", err)
	}
}

func TestViews(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is not installed")
	}
	for _, c := range []struct {
		dir, slug string
		presets   []Preset
		ids       []string
	}{
		{"graph", "graph-demo", []Preset{
			{"line", map[string]any{"n": 3, "edges": [][]int{{0, 1, 4}, {1, 2, 1}}}},
			{"shortcut", map[string]any{"n": 4, "edges": [][]int{{0, 1, 1}, {0, 2, 5}, {1, 2, 2}, {2, 3, 1}, {1, 3, 7}}}},
			{"apart", map[string]any{"n": 3, "edges": [][]int{{0, 1, 2}}}},
		}, []string{"L1", "L2", "L3"}},
		{"tree", "tree-demo", []Preset{
			{"pairs", map[string]any{"n": 4, "unions": [][]int{{0, 1}, {2, 3}}}},
			{"chain", map[string]any{"n": 5, "unions": [][]int{{0, 1}, {1, 2}, {3, 4}, {4, 0}, {2, 3}}}},
			{"none", map[string]any{"n": 3, "unions": [][]int{}}},
		}, []string{"L1", "L2"}},
		{"string", "string-demo", []Preset{
			{"two", map[string]any{"text": "abababca abababca", "pattern": "ababca"}},
			{"none", map[string]any{"text": "aaaa", "pattern": "ab"}},
			{"self", map[string]any{"text": "aaa", "pattern": "aaa"}},
		}, []string{"L1", "L2", "L3"}},
		{"table", "table-demo", []Preset{
			{"words", map[string]any{"a": "ABCBDAB", "b": "BDCABA"}},
			{"same", map[string]any{"a": "abc", "b": "abc"}},
			{"apart", map[string]any{"a": "ab", "b": "cd"}},
		}, []string{"L1", "L2", "L3"}},
		{"heap", "heap-demo", []Preset{
			{"mixed", map[string]any{"values": []int{5, 3, 8, 1, 9, 2, 7}, "pops": 3}},
			{"sorted", map[string]any{"values": []int{1, 2, 3, 4}, "pops": 4}},
			{"one", map[string]any{"values": []int{4}, "pops": 1}},
		}, []string{"L1", "L2"}},
	} {
		b, err := Build("testdata/"+c.dir+"/scene.ts", true)
		if err != nil {
			t.Fatal(err)
		}
		if r := Check(Scene{Slug: c.slug, Bundle: b, Presets: c.presets, IDs: c.ids}); len(r.Fails) > 0 {
			t.Errorf("%s: %v", c.dir, r.Fails)
		}
	}
}

func TestViewErrors(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is not installed")
	}
	src := `import { scene } from "@aik/viz";
export default scene({
  slug: "bad-views",
  input: { type: "x" },
  run(v, input: { k: number }) {
    const g = v.graph("G", { nodes: [1, 2], edges: [[1, 2]] });
    const t = v.tree("T", { binary: true, nodes: [{ id: "r" }, { id: "a", parent: "r", side: "left" }] });
    v.line("L1");
    if (input.k === 1) g.edge(2, 3);
    if (input.k === 2) t.insert("r", "b", "left");
    if (input.k === 3) t.insert("a", "r", "left");
    if (input.k === 4) g.mark(1, "glow" as any);
    const tb = v.table("D", 2, 3);
    const h = v.heap("H", [3, 1]);
    const s = v.string("S", "ab");
    if (input.k === 5) tb.fill(1, 1, 4, { from: [[2, 0]] });
    if (input.k === 6) h.swap(0, 2);
    if (input.k === 7) s.window(2, 3);
    if (input.k === 8) tb.mark([0, 0], "warm" as any);
    return 0;
  },
});
`
	dir := t.TempDir()
	path := filepath.Join(dir, "scene.ts")
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	b, err := Build(path, false)
	if err != nil {
		t.Fatal(err)
	}
	presets := []Preset{{"edge", map[string]int{"k": 1}}, {"side", map[string]int{"k": 2}}, {"cycle", map[string]int{"k": 3}}, {"state", map[string]int{"k": 4}}, {"cell", map[string]int{"k": 5}}, {"heap", map[string]int{"k": 6}}, {"window", map[string]int{"k": 7}}, {"tstate", map[string]int{"k": 8}}}
	r := Check(Scene{Slug: "bad-views", Bundle: b, Presets: presets, IDs: []string{"L1"}})
	for _, want := range []string{"has no edge 2-3", "has a left child already", "cannot go under its own subtree", `the state "glow" is not in the vocabulary (V09)`, "has no cell [2, 0]", "the heap H has no cell 2", "has no window 2..3", `the state "warm" is not in the vocabulary (V09)`} {
		found := false
		for _, f := range r.Fails {
			found = found || strings.Contains(f, want)
		}
		if !found {
			t.Errorf("no fail says %q: %v", want, r.Fails)
		}
	}
}
