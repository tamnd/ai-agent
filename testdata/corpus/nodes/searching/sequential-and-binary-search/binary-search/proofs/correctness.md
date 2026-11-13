---
title: Binary search returns an index of x, or NIL when x is not in A
proves: [correctness, termination]
status: full
---

Let $A[0 \ldots n-1]$ be sorted in non-decreasing order. We prove three facts about BINARY-SEARCH$(A, x)$.

**Invariant.** Before each test of line L2, these statements are true:

1. $0 \le lo \le hi \le n$.
2. For each index $i < lo$, $A[i] < x$.
3. For each index $i \ge hi$, $A[i] > x$.

*Start.* Line L1 sets $lo = 0$ and $hi = n$. Statement 1 is true. Statements 2 and 3 are true because no index is less than 0 and no index is at least $n$.

*Step.* Assume that the statements are true and that $lo < hi$. Line L3 sets $mid = lo + \lfloor (hi - lo)/2 \rfloor$. Because $hi - lo \ge 1$, we have $lo \le mid < hi$.

- If $A[mid] < x$, line L5 sets $lo' = mid + 1$. For each $i$ with $lo \le i \le mid$, $A[i] \le A[mid] < x$ because $A$ is sorted. With statement 2 for the old $lo$, each $i < lo'$ has $A[i] < x$. Also $lo' \le hi$ because $mid < hi$.
- If $A[mid] > x$, line L7 sets $hi' = mid$. For each $i$ with $mid \le i < hi$, $A[i] \ge A[mid] > x$. With statement 3 for the old $hi$, each $i \ge hi'$ has $A[i] > x$. Also $lo \le hi'$ because $lo \le mid$.
- Otherwise $A[mid] = x$, and line L8 returns.

So the statements are true before the next test of line L2.

**Termination.** The value $hi - lo$ is a non-negative integer by statement 1. In each step that does not return, it becomes smaller: in the first case $hi - (mid + 1) < hi - lo$ because $mid \ge lo$, and in the second case $mid - lo < hi - lo$ because $mid < hi$. A non-negative integer cannot become smaller for ever. So the loop stops.

**Result.** If line L8 returns $mid$, then $A[mid] = x$ and $0 \le mid < n$, so the result is correct. If line L9 returns NIL, then $lo \ge hi$, and by statement 1, $lo = hi$. By statements 2 and 3, each index $i$ has $A[i] < x$ or $A[i] > x$. So no item of $A$ is equal to $x$, and NIL is correct. $\blacksquare$

**The number of steps.** Let $k = hi - lo$ before a step. The new size is $\lfloor (k-1)/2 \rfloor$ or $\lceil (k-1)/2 \rceil$, and both are at most $\lfloor k/2 \rfloor$. Then after $t$ steps the size is at most $\lfloor n/2^t \rfloor$, which is 0 when $t = \lfloor \lg n \rfloor + 1$. So the loop runs at most $\lfloor \lg n \rfloor + 1$ times.
