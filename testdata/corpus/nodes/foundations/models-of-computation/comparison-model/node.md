---
tag: 9CMPMD
kind: concept
slug: comparison-model
name: Comparison model
aliases: [decision tree model, comparison-based model]
area: foundations/models-of-computation
level: intro
summary: A model where an algorithm learns about its input only by comparing two items. The cost is the number of comparisons.
sources:
  - book: isbn:9780262046305
    title: Cormen, Leiserson, Rivest and Stein, Introduction to Algorithms, fourth edition
    locator: Section 8.1
  - book: isbn:9780201896855
    title: Donald E. Knuth, The Art of Computer Programming, Volume 3, second edition
    locator: Section 5.3.1
---

## The machine

The input is a sequence of items from a set with a total order. The algorithm cannot read the items. It can only ask "is $a_i < a_j$?" or, with a three-way comparison, "is $a_i$ less than, equal to or larger than $a_j$?". It can move and copy items, and it can do any other computation that does not look at the items.

An algorithm in this model is a **decision tree**. Each inner node is a comparison. Each branch is an outcome. Each leaf is an output. The run on an input follows one path from the root to a leaf.

## What it counts

The cost is the number of comparisons. The worst-case cost is the depth of the tree. The average cost is the average depth of the leaves, over a distribution of inputs. The other work of the algorithm is free in the model. For most algorithms it is a constant factor of the number of comparisons, so the model also gives the time.

## Relation to other models

The word RAM can do arithmetic on the items and use them as addresses. So it can beat the bounds of the comparison model. Counting sort, radix sort and hash tables do this. The comparison model is still the right model for code that must work for any type with an order, such as the library sort of most languages, which takes a comparison function.

## Known results in the model

- **Sorting** needs $\lceil \lg(n!) \rceil = n \lg n - \Theta(n)$ comparisons in the worst case and on average. A binary tree with $n!$ leaves has a depth of at least $\lg(n!)$. See the sorting problem.
- **Search in a sorted array** needs $\lceil \lg(n+1) \rceil$ three-way comparisons. See [search in a sorted array](/search-in-sorted-array/).
- **The maximum** of $n$ items needs $n - 1$ comparisons. Each item except the maximum must lose one comparison.
- **The maximum and the minimum** together need $\lceil 3n/2 \rceil - 2$ comparisons.
- **Merging** two sorted lists of $n$ items each needs $2n - 1$ comparisons in the worst case.

## References
