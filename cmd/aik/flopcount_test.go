// SPDX-License-Identifier: Apache-2.0

package main

import "testing"

func TestFlopcountAndMutate(t *testing.T) {
	// The test corpus has no architecture with shapes.yaml, so there is nothing to count; the counts are tested in lang.
	if got := run([]string{"flopcount", "-root", corpus, "-unsafe-local"}); got != 0 {
		t.Errorf("flopcount with no architecture exits %d", got)
	}
	if got := run([]string{"flopcount", "-root", corpus, "-unsafe-local", "no-such-node"}); got != 2 {
		t.Errorf("flopcount of an unknown slug exits %d", got)
	}
	if got := run([]string{"mutate", "-kind", "diff"}); got != 2 {
		t.Errorf("mutate with -kind exits %d", got)
	}
}
