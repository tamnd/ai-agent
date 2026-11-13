// SPDX-License-Identifier: Apache-2.0

package node

import "testing"

func TestRenameRefs(t *testing.T) {
	in := `---
tag: X
slug: merge-sort
area: sorting/binary-search-trees
summary: Not binary-search, the words stay.
solves:
  - problem: binary-search
    note: keep binary-search here too
prereqs: [binary-search, binary-search-tree, loop-invariant]
see_also: [binary-search]
sources:
  - url: https://example.com/binary-search/
---

See [it](/binary-search/), [the proof](/binary-search/proof/), [a part](/binary-search#plain) and [other](/binary-search-tree/).
`
	want := `---
tag: X
slug: merge-sort
area: sorting/binary-search-trees
summary: Not binary-search, the words stay.
solves:
  - problem: bisection
    note: keep bisection here too
prereqs: [bisection, binary-search-tree, loop-invariant]
see_also: [bisection]
sources:
  - url: https://example.com/binary-search/
---

See [it](/bisection/), [the proof](/bisection/proof/), [a part](/bisection#plain) and [other](/binary-search-tree/).
`
	if got := string(RenameRefs([]byte(in), "binary-search", "bisection")); got != want {
		t.Errorf("RenameRefs gives\n%s\nwant\n%s", got, want)
	}
}

func TestSetSlugArea(t *testing.T) {
	in := "---\ntag: X\nslug: a\narea: x/y\n---\n\nslug: a\n"
	want := "---\ntag: X\nslug: b\narea: p/q\n---\n\nslug: a\n"
	if got := string(SetSlugArea([]byte(in), "b", "p/q")); got != want {
		t.Errorf("SetSlugArea gives %q, want %q", got, want)
	}
}
