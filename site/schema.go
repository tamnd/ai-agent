// SPDX-License-Identifier: Apache-2.0

package site

import (
	"io/fs"
	"reflect"

	"github.com/tamnd/ai-agent/schema"
)

// RecordSchema is the schema of the JSON record of an item (16.10), schema/record.v1.json, made from Record.
func RecordSchema() map[string]any {
	g := schema.Gen{Tag: "json", AllOpen: true}
	s := g.Of(reflect.TypeFor[Record]())
	s["$schema"] = schema.Draft
	s["$id"] = schema.Base + "record.v1.json"
	s["title"] = "Record of an item"
	s["description"] = "The JSON record of a node or a card (spec 16.10), at /<slug>.json and in the items dataset: its fields, dated facts with sources, claims with their computed evidence labels, ladders, edges both ways and the result cards that name it. It never holds the prose. A field that a later version adds is allowed, so a reader must ignore what it does not know."
	return s
}

// schemas writes the schema files at /data/schema/.
func (b *builder) schemas() error {
	return fs.WalkDir(schema.Files, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := fs.ReadFile(schema.Files, p)
		if err != nil {
			return err
		}
		return b.write("/data/schema/"+p, data)
	})
}
