# SPDX-License-Identifier: MIT
# Part of https://ai.tamnd.com/binary-search/. The authors are listed in prov/2B7QDX.jsonl.

from collections.abc import Sequence
from typing import Protocol, TypeVar


class Ordered(Protocol):
    def __lt__(self, other: "Ordered", /) -> bool: ...


T = TypeVar("T", bound=Ordered)


def binary_search(a: Sequence[T], x: T) -> int:
    """Return an index i with a[i] == x, or -1 when x is not in a. The list a must be sorted."""
    lo, hi = 0, len(a)  # @line L1
    while lo < hi:  # @line L2
        mid = lo + (hi - lo) // 2  # @line L3
        if a[mid] < x:  # @line L4
            lo = mid + 1  # @line L5
        elif x < a[mid]:  # @line L6
            hi = mid  # @line L7
        else:
            return mid  # @line L8
    return -1  # @line L9
