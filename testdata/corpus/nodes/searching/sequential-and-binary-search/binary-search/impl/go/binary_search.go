// SPDX-License-Identifier: MIT
// Part of https://ai.tamnd.com/binary-search/. The authors are listed in prov/2B7QDX.jsonl.

// Package binarysearch finds a key in a sorted slice.
package binarysearch

import "cmp"

// Search returns an index i with a[i] == x, or -1 when x is not in a. The slice a must be sorted.
func Search[T cmp.Ordered](a []T, x T) int {
	lo, hi := 0, len(a) // @line L1
	for lo < hi {       // @line L2
		mid := lo + (hi-lo)/2 // @line L3
		switch {
		case a[mid] < x: // @line L4
			lo = mid + 1 // @line L5
		case a[mid] > x: // @line L6
			hi = mid // @line L7
		default:
			return mid // @line L8
		}
	}
	return -1 // @line L9
}
