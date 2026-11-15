// SPDX-License-Identifier: Apache-2.0

package main

import "testing"

func TestVectorsCommand(t *testing.T) {
	// The runs of make.py are tested in lang; the test corpus has no vectors/make.py.
	if got := run([]string{"vectors", "make", "-root", corpus, "-unsafe-local"}); got != 0 {
		t.Errorf("vectors make with no make.py exits %d", got)
	}
	if got := run([]string{"vectors"}); got != 2 {
		t.Errorf("vectors with no subcommand exits %d", got)
	}
	if got := run([]string{"vectors", "make", "-root", corpus, "-unsafe-local", "no-such-node"}); got != 2 {
		t.Errorf("vectors make of an unknown slug exits %d", got)
	}
}
