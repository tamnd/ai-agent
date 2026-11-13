---
tag: 7NVRNT
kind: concept
slug: loop-invariant
name: Loop invariant
aliases: [invariant]
area: foundations/correctness-and-invariants
level: intro
summary: A statement about the variables of a loop that is true before each step. It is the main tool to prove that a loop is correct.
sources:
  - book: isbn:9780262046305
    title: Cormen, Leiserson, Rivest and Stein, Introduction to Algorithms, fourth edition
    locator: Section 2.1
  - paper: doi:10.1145/363235.363259
    title: C. A. R. Hoare, An axiomatic basis for computer programming (1969)
---

## In plain words

A loop changes its variables at each step. A loop invariant is a fact about these variables that does not change. It is true before the first step. Each step keeps it true. When the loop stops, the invariant and the reason for the stop together tell you that the result is correct.

## The idea

To prove a loop with an invariant $P$, you show three things:

1. **Start.** $P$ is true before the first step.
2. **Step.** If $P$ is true and the loop condition is true, then $P$ is true after one step.
3. **End.** When the loop condition is false, $P$ gives the result that you want.

This is a proof by induction on the number of steps. You also need a proof that the loop stops. For this, find a *variant*: a non-negative integer that becomes smaller at each step.

A good invariant describes the part of the work that is done. It is often a weaker form of the result. For example, "the result" is "the full array is sorted", and the invariant is "the first $i$ items are sorted".

## Examples

- In insertion sort, the invariant is: the part $A[0 \ldots i-1]$ holds the first $i$ items of the input, in sorted order. At the end, $i = n$.
- In [binary search](/binary-search/), the invariant is: if the key is in the array, it is in $A[lo \ldots hi-1]$. The variant is $hi - lo$.
- In merge sort, the merge loop has the invariant: the output part holds the smallest items of the two inputs, in sorted order.
- In the Euclidean algorithm, the invariant is $\gcd(a, b) = \gcd(a_0, b_0)$, and the variant is $b$.

## Where it is used

Loop invariants are the base of Hoare logic, of the proofs on this site, and of tools that check programs, such as Dafny, Why3 and Lean. Programmers also write them as assertions, so that a test fails when the invariant is false.

## References
