// SPDX-License-Identifier: MIT
// Part of https://ai.tamnd.com/binary-search/. The authors are listed in prov/2B7QDX.jsonl.

package binarysearch

import (
	"math/rand/v2"
	"slices"
	"testing"
)

func TestExamples(t *testing.T) {
	a := []int{2, 5, 8, 12, 16, 23, 38, 56, 72, 91}
	for _, c := range []struct{ x, want int }{{23, 5}, {2, 0}, {91, 9}, {30, -1}, {1, -1}, {100, -1}} {
		if got := Search(a, c.x); got != c.want {
			t.Errorf("Search(a, %d) = %d, want %d", c.x, got, c.want)
		}
	}
}

func TestEdges(t *testing.T) {
	if got := Search([]int{}, 1); got != -1 {
		t.Errorf("empty: got %d", got)
	}
	if got := Search([]int{7}, 7); got != 0 {
		t.Errorf("single: got %d", got)
	}
	if got := Search([]string{"ant", "bee", "cat"}, "bee"); got != 1 {
		t.Errorf("strings: got %d", got)
	}
	a := []int{1, 3, 3, 3, 5}
	if i := Search(a, 3); i < 0 || a[i] != 3 {
		t.Errorf("duplicates: got %d", i)
	}
}

func TestRandom(t *testing.T) {
	r := rand.New(rand.NewPCG(2150, 0))
	for range 2000 {
		a := make([]int, r.IntN(31))
		for i := range a {
			a[i] = r.IntN(41) - 20
		}
		slices.Sort(a)
		x := r.IntN(51) - 25
		i := Search(a, x)
		if slices.Contains(a, x) {
			if i < 0 || i >= len(a) || a[i] != x {
				t.Fatalf("Search(%v, %d) = %d", a, x, i)
			}
		} else if i != -1 {
			t.Fatalf("Search(%v, %d) = %d, want -1", a, x, i)
		}
	}
}
