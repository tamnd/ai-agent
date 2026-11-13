The build gives each page a level on each of five ladders. A level comes from evidence in the repository. Nobody writes a level by hand. If the evidence goes away, the level goes down.

A review counts only for the exact file that the reviewer read. The review record keeps the SHA-256 hash of the file. When the file changes, the hash does not match, and the level goes down until a new review.

## H: the human proof

| Level | What it needs |
| :-- | :-- |
| H0 | Nothing. The page has no proof. |
| H1 | A sketch: the main idea of the proof, marked as a sketch. |
| H2 | A full proof of correctness and of each cost claim of the front matter. |
| H3 | H2, and a person who is not the author reviewed this exact proof. |

## L: the Lean proof

| Level | What it needs |
| :-- | :-- |
| L0 | Nothing. The page has no Lean part. |
| L1 | The Lean specification and the model exist, and a person checked that the specification says the same as the problem. |
| L2 | The correctness theorem is proved, with no `sorry` and only the standard axioms. |
| L3 | L2, and each time and space claim is proved in the cost model. |
| L4 | L3, and an implementation is proved to refine the model. |

## I: the code in one language

| Level | What it needs |
| :-- | :-- |
| I0 | The code is not checked yet. |
| I1 | The code builds with no warnings and passes the shared cases of the contract. |
| I2 | I1, and the code passes the random and the adversarial tests against a reference, and its tests kill at least 85% of the mutants. |
| I3 | I2, and lint and format are clean, the cost check passes and the code follows the conventions. |
| I4 | I3, and a person reviewed this exact file. |

The shared cases and the adapter that runs them do not exist yet. Until they exist, each implementation stays at I0. The page shows the result of the native tests next to the code, so that you can see that the tests pass.

## V: the visual

| Level | What it needs |
| :-- | :-- |
| V0 | Nothing. The page has no picture. |
| V1 | Static figures, or a scene that passes the checks that exist now. |
| V2 | An interactive scene with step back, custom input and code sync, that passes all the trace checks. |
| V3 | V2, and a person reviewed it for clarity and accessibility. |

The scene checks that exist now are these: the scene builds, it imports only `@aik/viz`, it gives the same trace twice, it has at least 3 presets and its presets reach each line of the pseudocode. The checks against a reference and the budgets come later. Until then, a scene stays at V1.

## W: the writing

The W level comes from the sections of the page. Each kind of page has its own list.

| Level | Name | For an algorithm, it needs |
| :-- | :-- | :-- |
| W0 | stub | A name and a summary. |
| W1 | outline | The lead paragraph, The problem, pseudocode, the cost claims and one source with a locator. |
| W2 | basic | In plain words, Example, Correctness, Complexity and Properties. |
| W3 | standard | Pitfalls, History, three sources, prerequisites that exist and a scene preset. |
| W4 | complete | Variants and improvements, When to use it and an exercise. |
| W5 | reviewed | W4, and a person reviewed the text. |

## When a page is complete

A page is complete when it has H3, L3, I3 in each tier 1 language, I2 in each tier 2 language, V2 and W4.
