// SPDX-License-Identifier: Apache-2.0

package node

import "testing"

func TestIsBuildFile(t *testing.T) {
	for name, want := range map[string]bool{"go.mod": true, "x/Cargo.toml": true, "Impl.csproj": true, "merge_sort.go": false, "package.json": true, "README.md": false} {
		if got := IsBuildFile(name); got != want {
			t.Errorf("IsBuildFile(%q) = %v", name, got)
		}
	}
}
