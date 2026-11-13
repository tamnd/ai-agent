---
tag: 8ASYMP
kind: definition
slug: asymptotic-notation
name: Asymptotic notation
aliases: [big O notation, Landau notation, big Theta, big Omega]
area: foundations/asymptotic-analysis
level: intro
summary: The notations O, Ω and Θ compare the growth of two functions for large inputs and ignore constant factors.
sources:
  - book: isbn:9780262046305
    title: Cormen, Leiserson, Rivest and Stein, Introduction to Algorithms, fourth edition
    locator: Chapter 3
  - paper: doi:10.1145/1008328.1008329
    title: Donald E. Knuth, Big Omicron and big Omega and big Theta (1976)
---

## Definition

Let $f$ and $g$ be functions from the natural numbers to the non-negative real numbers.

- $f(n) = O(g(n))$ when there are constants $c > 0$ and $n_0$ with $f(n) \le c \cdot g(n)$ for each $n \ge n_0$. So $f$ grows at most as fast as $g$.
- $f(n) = \Omega(g(n))$ when there are constants $c > 0$ and $n_0$ with $f(n) \ge c \cdot g(n)$ for each $n \ge n_0$. So $f$ grows at least as fast as $g$.
- $f(n) = \Theta(g(n))$ when $f(n) = O(g(n))$ and $f(n) = \Omega(g(n))$. So $f$ and $g$ grow at the same rate.
- $f(n) = o(g(n))$ when $f(n)/g(n) \to 0$, and $f(n) = \omega(g(n))$ when $f(n)/g(n) \to \infty$.

The sign $=$ here means "is in the set". $O(g)$ is a set of functions, and $f = O(g)$ means $f \in O(g)$.

## Examples and non-examples

- $3n^2 + 10n + 7 = \Theta(n^2)$. Take $c = 4$ and $n_0 = 12$ for the upper bound, and $c = 3$ for the lower bound.
- $n \lg n = O(n^2)$, but $n \lg n \ne \Omega(n^2)$.
- $\lg(n!) = \Theta(n \log n)$. The base of a logarithm changes only a constant factor, so $O(\log n)$ needs no base.
- $2^{n+1} = O(2^n)$, but $2^{2n} \ne O(2^n)$.
- Not an example: $n^{\sin n}$ is not $O(1)$, and it is not $\Omega(n^{1/2})$ either. Some functions do not compare.

## Related definitions

- **Cost measures.** The time of an algorithm is a function of the size of the input. A claim such as "the worst-case time is $\Theta(n \log n)$" uses the maximum over the inputs of size $n$. The [comparison model](/comparison-model/) is one way to fix what one step is.
- **Amortized and expected costs** use the same notation on an average over a sequence of operations, or over random choices.

## References
