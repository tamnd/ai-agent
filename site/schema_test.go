// SPDX-License-Identifier: Apache-2.0

package site

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/tamnd/ai-agent/schema"
)

var updateSchema = flag.Bool("update", false, "write schema/record.v1.json from Record")

func TestRecordSchema(t *testing.T) {
	b, err := schema.Encode(RecordSchema())
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join("..", "schema", "record.v1.json")
	if *updateSchema {
		if err := os.WriteFile(file, b, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	old, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(old, b) {
		t.Errorf("%s is not what RecordSchema makes; run go test ./site -run TestRecordSchema -update", file)
	}
	var s map[string]any
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatal(err)
	}
	props := s["properties"].(map[string]any)
	for _, k := range []string{"url", "as_of", "kind", "key", "fields", "facts", "claims", "status", "edges_out", "edges_in"} {
		if _, ok := props[k]; !ok {
			t.Errorf("record.v1 has no %s", k)
		}
	}
	if _, ok := props["Head"]; ok {
		t.Error("record.v1 has the embedded Head as a property")
	}
}
