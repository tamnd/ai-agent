// SPDX-License-Identifier: Apache-2.0

package computed

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestRead(t *testing.T) {
	root := t.TempDir()
	if ds, err := Read(root); err != nil || ds != nil {
		t.Fatalf("Read of a root with no file = %v, %v", ds, err)
	}
	write := func(s string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(root, "topics"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, File), []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	good := "- id: most-params\n  kind: model\n  where: {access: [open, Open weights]}\n  measure: params.total\n  order: max\n  what: the model with the most parameters\n  population: the model cards\n"
	write(good)
	ds, err := Read(root)
	if err != nil || len(ds) != 1 || ds[0].Measure != "params.total" {
		t.Fatalf("Read = %+v, %v", ds, err)
	}
	for bad, want := range map[string]string{
		strings.Replace(good, "order: max", "order: top", 1):                         `order "top"`,
		strings.Replace(good, "id: most-params", "id: Most_Params", 1):               "is not lower case",
		strings.Replace(good, "  measure: params.total\n", "", 1):                    "has no measure",
		strings.Replace(good, "  what: the model with the most parameters\n", "", 1): "needs what and population",
		good + good: "defined twice",
		strings.Replace(good, "order: max", "order: max\n  rank: 1", 1): "field rank not found",
		strings.Replace(good, "[open, Open weights]", "{a: b}", 1):      "is not a text",
	} {
		write(bad)
		if _, err := Read(root); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Read of\n%s\n= %v, want %q", bad, err, want)
		}
	}
}

func item(key, name string, fields map[string]any) Item {
	it := Item{Tag: strings.ToUpper(key), Key: key, Name: name, Fields: map[string]Value{}}
	for k, v := range fields {
		it.Fields[k] = Value{V: v, Unit: map[string]string{"params.total": "parameters"}[k], Src: "S:epoch-ai", Loc: k, AsOf: "2026-11-1" + key[len(key)-1:]}
	}
	return it
}

func TestCompute(t *testing.T) {
	items := []Item{
		item("a1", "Alpha", map[string]any{"params.total": json.Number("7e9"), "access": "Open weights", "released": "2023-02-24", "country": []any{"France"}}),
		item("b2", "Beta", map[string]any{"params.total": 4.05e11, "access": "open weights", "released": "2024-07-23", "country": []any{"United States", "France"}}),
		item("c3", "Gamma", map[string]any{"params.total": 1.8e12, "access": "API access", "released": "2023-03-14"}),
		item("d4", "Delta", map[string]any{"access": "Open weights", "released": "2022"}),
		item("e5", "Epsilon", map[string]any{"params.total": 4.05e11, "access": "Open weights", "country": []any{"China"}}),
	}
	d := Def{ID: "most-open", Kind: "model", Where: map[string]any{"access": "Open Weights"}, Measure: "params.total", Order: "max",
		What: "the model with the most total parameters", Population: "the model cards with open weights"}
	r, err := Compute(d, items)
	if err != nil {
		t.Fatal(err)
	}
	if r.Item != "b2" || r.Count != 3 || r.Missing != 1 || !reflect.DeepEqual(r.Ties, []string{"Epsilon"}) || r.AsOf != "2026-11-15" || r.Src != "S:epoch-ai" {
		t.Fatalf("Compute = %+v", r)
	}
	if want := "Among the model cards with open weights, the models with the most total parameters are Beta and Epsilon (405 billion parameters each), as of 2026-11-15."; r.Sentence != want {
		t.Fatalf("the sentence is\n%s\nwant\n%s", r.Sentence, want)
	}

	// The first by date, with a list field in where; "2022" orders before "2023-02-24".
	d = Def{ID: "first-french", Kind: "model", Where: map[string]any{"country": "france"}, Measure: "released", Order: "min", What: "the earliest model", Population: "the French model cards"}
	if r, err = Compute(d, items); err != nil || r.Name != "Alpha" || r.Count != 2 || len(r.Ties) != 0 {
		t.Fatalf("Compute first = %+v, %v", r, err)
	}
	if want := "Among the French model cards, the earliest model is Alpha (2023-02-24), as of 2026-11-12."; r.Sentence != want {
		t.Fatalf("the sentence is %q", r.Sentence)
	}
	d.Where = nil
	if r, _ = Compute(d, items); r.Name != "Delta" {
		t.Fatalf("the earliest of all is %s", r.Name)
	}

	for _, c := range []struct {
		d     Def
		items []Item
		want  string
	}{
		{Def{ID: "x", Measure: "params.total", Order: "max", Where: map[string]any{"access": "none"}}, items, "no  of the population gives params.total"},
		{Def{ID: "x", Measure: "mixed", Order: "max"}, []Item{item("a1", "A", map[string]any{"mixed": 1.0}), item("b2", "B", map[string]any{"mixed": "2024"})}, "is a number for a1 and a text for b2"},
		{Def{ID: "x", Measure: "m", Order: "max"}, []Item{{Key: "a", Fields: map[string]Value{"m": {V: 1.0}}}}, "no item of the population has the date"},
	} {
		if _, err := Compute(c.d, c.items); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("Compute(%s) = %v, want %q", c.d.Measure, err, c.want)
		}
	}
}
