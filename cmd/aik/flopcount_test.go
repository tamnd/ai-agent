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

func TestFixturesCommand(t *testing.T) {
	// The runs of make.py are tested in lang; the test corpus has no fixtures/make.py.
	if got := run([]string{"fixtures", "make", "-root", corpus, "-unsafe-local"}); got != 0 {
		t.Errorf("fixtures make with no make.py exits %d", got)
	}
	if got := run([]string{"fixtures", "sample"}); got != 3 {
		t.Errorf("fixtures sample, which is planned, exits %d", got)
	}
	if got := run([]string{"fixtures"}); got != 2 {
		t.Errorf("fixtures with no subcommand exits %d", got)
	}
}
