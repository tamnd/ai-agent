// SPDX-License-Identifier: Apache-2.0

package course

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	root := t.TempDir()
	if cs, err := Load(root); err != nil || cs != nil {
		t.Fatalf("no course: %v %v", cs, err)
	}
	dir := filepath.Join(root, filepath.FromSlash(Dir))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"b.yaml": "id: b-course\ntitle: B\nsections:\n  - heading: One\n    items:\n      - {name: Thing}\n      - {name: Other, kind: method, node: other}\n",
		"a.yaml": "title: A\nsections: []\n",
	}
	for f, s := range files {
		if err := os.WriteFile(filepath.Join(dir, f), []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cs, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 2 || cs[0].ID != "a" || cs[1].ID != "b-course" {
		t.Fatalf("courses %+v", cs)
	}
	if it := cs[1].Sections[0].Items[0]; it.Kind != "concept" || cs[1].Len() != 2 {
		t.Errorf("item %+v, len %d", it, cs[1].Len())
	}
	if err := os.WriteFile(filepath.Join(dir, "bad.yaml"), []byte("sections: {"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root); err == nil {
		t.Error("a broken course file gives no error")
	}
}
