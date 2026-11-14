// SPDX-License-Identifier: Apache-2.0

// Package schema holds the schemas of the files and records of spec 20.2 (node.v1.json, card.v1.json, claims.v1.json,
// experiment.v1.json, expect.v1.json, facts.v1.yaml, record.v1.json) and the generator that makes them from the Go
// types and tables that read those files, so a schema cannot drift from the code. The tests compare the files with the generator; go test ./schema -update writes them again.
//
// The code is the authority: aik check applies rules that a schema cannot say (a slug that exists, a role that the
// type of a claim allows). The schemas are for editors and for programs that read the records.
package schema

import (
	"bytes"
	"embed"
	"encoding/json"
	"maps"
	"reflect"
	"slices"
	"strings"
)

// Files are the schema files, as the site publishes them at /data/schema/.
//
//go:embed *.json *.yaml
var Files embed.FS

// Draft is the JSON Schema dialect of the files.
const Draft = "https://json-schema.org/draft/2020-12/schema"

// Gen makes the schema of a Go type from its struct tags.
type Gen struct {
	// Tag is the struct tag that names the fields: yaml for a file, json for a record.
	Tag string
	// Enums gives the allowed values of a field, by "Type.field" as the Go type name and the name in the tag.
	Enums map[string][]string
	// Types replaces the schema of a type, by its Go name, such as a type that reads a string or a mapping.
	Types map[string]map[string]any
	// Fields replaces the schema of a field, by "Type.field".
	Fields map[string]map[string]any
	// Open lists the Go types whose objects may have other keys. AllOpen opens every object, for a record that a later
	// version may extend.
	Open    []string
	AllOpen bool
	// Optional lists the fields, by "Type.field", that a file may leave out though the Go field has no omitempty.
	Optional []string

	seen map[reflect.Type]bool
}

// Of gives the schema of a type.
func (g *Gen) Of(t reflect.Type) map[string]any {
	if g.seen == nil {
		g.seen = map[reflect.Type]bool{}
	}
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if s, ok := g.Types[t.Name()]; ok {
		return s
	}
	switch t.Kind() {
	case reflect.String:
		return map[string]any{"type": "string"}
	case reflect.Bool:
		return map[string]any{"type": "boolean"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return map[string]any{"type": "integer"}
	case reflect.Float32, reflect.Float64:
		return map[string]any{"type": "number"}
	case reflect.Slice, reflect.Array:
		return map[string]any{"type": "array", "items": g.Of(t.Elem())}
	case reflect.Map:
		return map[string]any{"type": "object", "additionalProperties": g.Of(t.Elem())}
	case reflect.Struct:
		if g.seen[t] {
			return map[string]any{} // a recursive type; none of the files has one
		}
		g.seen[t] = true
		defer delete(g.seen, t)
		s := map[string]any{"type": "object"}
		props := map[string]any{}
		var req []string
		var extra any = false
		if g.AllOpen || slices.Contains(g.Open, t.Name()) {
			extra = true
		}
		g.fields(t, props, &req, &extra)
		s["properties"] = props
		if len(req) > 0 {
			s["required"] = req
		}
		if extra != true {
			s["additionalProperties"] = extra
		}
		return s
	}
	return map[string]any{} // any
}

// fields adds the fields of a struct, with the fields of an inline or embedded struct.
func (g *Gen) fields(t reflect.Type, props map[string]any, req *[]string, extra *any) {
	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		name, opts, _ := strings.Cut(f.Tag.Get(g.Tag), ",")
		if name == "-" {
			continue
		}
		ft := f.Type
		for ft.Kind() == reflect.Pointer {
			ft = ft.Elem()
		}
		inline := strings.Contains(","+opts+",", ",inline,") || (f.Anonymous && name == "")
		if inline {
			switch ft.Kind() {
			case reflect.Struct:
				g.fields(ft, props, req, extra)
			case reflect.Map:
				*extra = g.Of(ft.Elem())
			}
			continue
		}
		if name == "" {
			name = f.Name
		}
		s := g.Of(f.Type)
		if o, ok := g.Fields[t.Name()+"."+name]; ok {
			s = o
		}
		if vs, ok := g.Enums[t.Name()+"."+name]; ok {
			s = enum(s, vs)
		}
		omit := strings.Contains(","+opts+",", ",omitempty,")
		if g.Tag == "json" && !omit && (ft.Kind() == reflect.Slice || ft.Kind() == reflect.Map) {
			// encoding/json writes a nil slice or map as null
			s = maps.Clone(s)
			s["type"] = []string{s["type"].(string), "null"}
		}
		if f.Type.Kind() == reflect.Pointer {
			// a pointer reads null as nil, and encoding/json writes nil as null
			if t, ok := s["type"].(string); ok {
				s = maps.Clone(s)
				s["type"] = []string{t, "null"}
			}
		}
		props[name] = s
		if !omit && f.Type.Kind() != reflect.Pointer && !slices.Contains(g.Optional, t.Name()+"."+name) {
			*req = append(*req, name)
		}
	}
}

// enum puts the values on a string, or on the items of an array of strings.
func enum(s map[string]any, vs []string) map[string]any {
	out := map[string]any{}
	for k, v := range s {
		out[k] = v
	}
	if out["type"] == "array" {
		out["items"] = map[string]any{"type": "string", "enum": vs}
		return out
	}
	out["enum"] = vs
	return out
}

// Encode gives the file of a schema: indented JSON with sorted keys and a final newline.
func Encode(s map[string]any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	err := enc.Encode(s)
	return buf.Bytes(), err
}
