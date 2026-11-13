The build gives each page a level on each of its ladders. A level comes from evidence in the repository. Nobody writes a level by hand. If the evidence goes away, the level goes down.

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

## D: the data facts

A model, a dataset, a benchmark, a hardware item, a policy and a system have facts, such as the number of parameters or the release date. Each fact is in the file `facts.yaml` of the page, with its sources.

| Level | What it needs |
| :-- | :-- |
| D0 | A name and a source. |
| D1 | Each required fact of the kind has a value from a source, and each number has a unit. |
| D2 | D1, and each fact has a primary source with a locator, or says why no primary source exists. Each fact that is true only at a date has `as_of`. |
| D3 | D2, and a person reviewed this exact fact file. |

The build also computes the status of each fact from its sources.

| Status | What it means |
| :-- | :-- |
| agreed | A primary source and at least one other source give the same value. |
| primary-only | Only primary sources give the value. |
| secondary-only | Only secondary sources, such as a database, give the value. |
| disputed | The sources give different values. The page shows each value with its source. |
| estimated | No primary source gives the value. An estimator computes it with a named method. |

Two numbers are the same value when they differ by 1% or less. Two dates are the same when they agree to the precision of the less precise date, so 2024-07 and 2024-07-23 agree.

## Evidence labels

Each claim of a page is in the file `claims.yaml` of the page, with a type and its sources. The claim sentence in the text ends with a chip. The build computes the label of the chip from the sources of the claim. The writer does not write it.

| Label | What it needs |
| :-- | :-- |
| proved | A Lean proof or a full derivation proves the claim. Only a mathematical claim can have this label. |
| measured here | An experiment of this site tests the claim and passes. The chip always shows the scope of the experiment. |
| replicated | At least two independent sources support the claim within its scope. |
| reported | At least one source supports the claim, and no other label applies. |
| contested | An independent source, a failed replication or a failed experiment of this site is against the claim. |

A claim can have more than one label. The chip shows one of them, in this order: proved, contested, measured here, replicated, reported. The Evidence table of the page shows all of them.

A source counts only when the citation check finds the claimed text or number in a stored copy of the source. The citation check is not in the toolchain yet. Until then, each chip says "not checked yet". A definition has no label. Its chip says "source".

Two sources are independent when they share no author and no institution, when one does not reuse the weights of the other, and when one is not a review of the other. A claim whose only sources are by the developer of the system it is about can be at most reported.

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
