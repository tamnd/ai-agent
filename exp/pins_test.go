// SPDX-License-Identifier: Apache-2.0

package exp

import (
	"slices"
	"testing"
)

func TestSplits(t *testing.T) {
	for subset, want := range map[string][]string{
		"train[:6000], test":                  {"train", "test"},
		"train[:50%],train[50%:], validation": {"train", "validation"},
		"":                                    nil,
		" , ":                                 nil,
		"test":                                {"test"},
	} {
		if got := (Data{Subset: subset}).Splits(); !slices.Equal(got, want) {
			t.Errorf("%q reads %q, not %q", subset, got, want)
		}
	}
	p := &Pin{Files: []PinFile{{Name: "a", Split: "train"}, {Name: "b", Split: "test"}, {Name: "c", Split: "test", Test: true}, {Name: "d"}}}
	if got := p.Splits(); len(got) != 2 || got["train"] || !got["test"] {
		t.Errorf("%v", got)
	}
}
