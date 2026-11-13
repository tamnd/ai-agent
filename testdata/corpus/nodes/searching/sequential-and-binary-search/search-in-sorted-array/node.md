---
tag: 6SRCHA
kind: problem
slug: search-in-sorted-array
name: Search in a sorted array
aliases: [sorted search, ordered search]
area: searching/sequential-and-binary-search
level: intro
summary: Given a sorted array and a key, find an index of the key, or report that the key is not in the array.
model: comparison-model
complexity:
  vars: {n: the number of items}
  time: {worst: Ω(log n)}
sources:
  - book: isbn:9780201896855
    title: Donald E. Knuth, The Art of Computer Programming, Volume 3, second edition
    locator: Section 6.2.1
---

## In plain words

You have a list of items in order, such as the words in a dictionary. You want to know if a word is in the list, and where. Because the list is in order, you do not need to look at each item.

## Definition

**Input.** An array $A[0 \ldots n-1]$ with $A[0] \le A[1] \le \cdots \le A[n-1]$ under a total order, and a key $x$.

**Output.** An index $i$ with $A[i] = x$, or NIL when no such index exists.

## Variants

- **Lower bound.** Find the smallest index $i$ with $A[i] \ge x$. This is also the number of items less than $x$.
- **Upper bound.** Find the smallest index $i$ with $A[i] > x$.
- **Predecessor and successor.** Find the largest item less than $x$, or the smallest item larger than $x$.
- **Unknown length.** The array is very long, or its length is not known.

## Examples

| A | x | output |
|---|---|---|
| 2 5 8 12 16 23 38 | 23 | 5 |
| 2 5 8 12 16 23 38 | 7 | NIL |
| 1 3 3 3 5 | 3 | 1, 2 or 3 |

## Known algorithms

## Bounds

In the [comparison model](/comparison-model/) with three-way comparisons, each algorithm needs $\lceil \lg(n+1) \rceil$ comparisons in the worst case. A decision tree for the problem must have a different outcome for each of the $n$ positions and for "not found". [Binary search](/binary-search/) reaches this bound. If the algorithm can do arithmetic on the keys, interpolation search uses $O(\log \log n)$ steps on average for keys from a uniform distribution.

## Applications

- Look up a key in a sorted table, such as a symbol table or an index on disk.
- Count the items in a range $[x, y]$ with two lower-bound searches.
- Search on the answer: find the smallest value that satisfies a monotone test.

## References
