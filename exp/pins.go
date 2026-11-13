// SPDX-License-Identifier: Apache-2.0

package exp

import (
	"errors"
	"fmt"
	"os"

	"go.yaml.in/yaml/v3"
)

// PinsFile is the register of the data pins, relative to the content root (spec 9.5).
const PinsFile = "registers/data-pins.yaml"

// PinFile is one file of a pinned dataset.
type PinFile struct {
	Name   string `yaml:"name"`
	SHA256 string `yaml:"sha256,omitempty"`
	BLAKE3 string `yaml:"blake3,omitempty"`
	Bytes  int64  `yaml:"bytes,omitempty"`
	// URL is where aik exp run fetches the file when the data cache does not have it. It is https.
	URL string `yaml:"url,omitempty"`
}

// Licence is the licence of a pinned dataset.
type Licence struct {
	SPDX   string `yaml:"spdx"`
	Source string `yaml:"source,omitempty"`
}

// Pin is one dataset of the register.
type Pin struct {
	ID       string    `yaml:"id"`
	Dataset  string    `yaml:"dataset"`
	Version  string    `yaml:"version"`
	Files    []PinFile `yaml:"files,omitempty"`
	Origin   string    `yaml:"origin,omitempty"`
	Licence  Licence   `yaml:"licence"`
	Mirror   string    `yaml:"mirror"` // allowed | not-allowed | unknown
	Location string    `yaml:"location,omitempty"`
}

// LoadPins reads the register of the data pins. It gives nil and no error when the register does not exist.
func LoadPins(path string) (map[string]*Pin, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var list []*Pin
	if err := yaml.Unmarshal(b, &list); err != nil {
		return nil, fmt.Errorf("%s: %w", PinsFile, err)
	}
	out := map[string]*Pin{}
	for _, p := range list {
		if _, dup := out[p.ID]; dup {
			return nil, fmt.Errorf("%s: the pin %q is there twice", PinsFile, p.ID)
		}
		out[p.ID] = p
	}
	return out, nil
}

// PinID gives the id of the pin of a data entry: the pin without the prefix S:.
func PinID(d Data) string {
	if len(d.Pin) > 2 && d.Pin[:2] == "S:" {
		return d.Pin[2:]
	}
	return d.Pin
}
