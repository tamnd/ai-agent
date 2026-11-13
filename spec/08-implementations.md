# 08. Implementations

An implementation is code that a reader can read, run and copy.
It is written to be read first, to be correct second, and to be fast third.
In algorithm, one problem contract checks every language.
Here, one NumPy reference written from first principles checks every framework: the PyTorch and JAX versions must agree with it on shared fixtures, and gradients must agree with finite differences.

## 8.1 What an implementation is

- An implementation belongs to one node and one framework. It lives in `impl/<framework>/` in the node directory (04.4).
- The kinds that have implementations (04.2): `method`, `architecture` and `metric` always when the kind's targets ask for it (04.10); `concept`, `finding` and `risk` optionally, as a demonstration; `task` through its contract adapter (10.2); `software` through usage examples, which are not reference implementations and have no X level.
- Each implementation has one **reference** version. It can also have labelled versions (8.5).
- It has its own tests next to it, in the test framework of its language (8.3). The shared fixtures of the node (8.7) run through every framework, so one set of numbers checks them all.
- It follows the equations of the page, and marks each block of code with the id of the equation that it computes (`# eq: attn-scores`), so that the page, the scene and the code can point at the same thing (8.3, 12).
- The NumPy reference writes the backward pass by hand for every method and architecture whose page derives the gradient (06.5). The PyTorch and JAX versions use automatic differentiation, and their gradients are tested against the NumPy backward and against finite differences (8.7.4). This is how the derivation on the page is tested.
- The build computes its level X0 to X4 (8.10). Nobody writes the level.

## 8.2 The framework register and the tiers

### 8.2.1 The register

`registers/frameworks.yaml` lists each framework. A framework is in the register before any implementation in it can merge (rule X12). Each entry has the fields of this example:

```yaml
- id: pytorch                       # the directory name under impl/
  name: PyTorch
  tier: 1
  language: python
  ext: [py]
  file_case: snake                  # scaled_dot_product_attention.py
  packages: pytorch-ref             # the pinned package set of 8.4
  test: {framework: pytest, file: "test_<name>.py"}
  format: {tool: ruff-format}
  lint: {tool: ruff, args: ["--select", "E,F,W,B,UP,NPY"]}
  types: {tool: pyright, mode: basic}
  mutation: {tool: mutmut}
  image: aik-runner-pytorch         # 8.9
  default_dtype: float64            # in tests; float32 in experiments (09)
  determinism: ["torch.use_deterministic_algorithms(True)", "CUBLAS_WORKSPACE_CONFIG=:4096:8"]
  highlight: python
  since: 2026-12
```

### 8.2.2 The tiers

| Tier | Frameworks | Required for | Count |
|---|---|---|---|
| **1** | `numpy` (NumPy from scratch, the reference), `pytorch`, `jax` | each method, architecture and metric node of the core; each node of the long tail that has an implementation at all has at least `numpy` | 3 |
| **2** | `c` (C99, no dependencies), `cpp` (C++20 with Eigen only), `rust` (stable, `ndarray` only), `julia` (Base and the standard library only), `typescript` (no framework, typed arrays; it also powers the in-browser notebooks of 12.8) | the core architectures and metrics only, from M3 (Q-36) | 5 |
| **3** | open: for example TensorFlow and Keras, MLX, tinygrad, Flax NNX layers, Swift, Go, R, Mojo | nothing; a contributor adds them | any |

Rules for the tiers:

- **NumPy is always first.** No framework version merges before the NumPy reference of the same node is at X1 (rule X01). The NumPy reference is the oracle of the differential tests (8.7.3), as the Lean model is the oracle in algorithm.
- Tier 3 has the same harness, the same fixtures and the same ladder. It is not in the "complete" test of 04.10 and not in the coverage levels of 18.2. The site shows tier-3 code in the framework menu under "More frameworks".
- `software` nodes may show usage examples in any framework, in `examples/`, with no X level.

### 8.2.3 The evidence for the choice

| Signal | What it says (2026) |
|---|---|
| research code | PyTorch is the framework of most papers with code on arXiv and at NeurIPS, ICML, ICLR, ACL and CVPR (01.5); JAX is the second, and the main one at Google DeepMind and in much of the large-scale training literature |
| teaching | the canon (5.10) teaches from scratch in NumPy (d2l, Prince's notebooks, CS231n-style assignments) and then in PyTorch; D2L ships PyTorch, JAX, TensorFlow and MXNet versions of each section |
| inference at the edge | llama.cpp and ggml (C and C++), candle and burn (Rust), transformers.js and ONNX Runtime Web (TypeScript) carry much of local and in-browser inference; these are the reasons for the tier-2 list |
| reader need | a reader who wants to understand writes NumPy; a reader who wants to use writes PyTorch or JAX; a reader who wants to run in the browser needs TypeScript |

### 8.2.4 The pinned toolchains

`manifests/frameworks.yaml` pins each framework's toolchain. The values below are the Default for M0, as of 2026-11; M0 sets the exact versions to the latest stable releases and records them.

| Framework | Language and toolchain | Packages of the reference set |
|---|---|---|
| `numpy` | CPython 3.13 | NumPy 2.x |
| `pytorch` | CPython 3.13 | PyTorch 2.x (CPU wheel in tests), NumPy 2.x |
| `jax` | CPython 3.13 | JAX and jaxlib 0.x (CPU), NumPy 2.x |
| `c` | GCC 15 and Clang 21, `-std=c99 -O2 -Wall -Wextra -Werror` | libc and libm |
| `cpp` | GCC 15 and Clang 21, `-std=c++20` | Eigen 3.4 or 5.x |
| `rust` | stable Rust of M0 | `ndarray`, `approx` in tests |
| `julia` | Julia 1.12 | Base, LinearAlgebra, Random, Test |
| `typescript` | Node 24 LTS, TypeScript 5.x | none; `vitest` in tests |

The test packages (pytest, hypothesis, scipy for reference special functions in tests only) are in the images and are listed in the same manifest. A new pin is a pull request that moves the version and carries the full test matrix report of the framework with it (8.9).

## 8.3 Conventions

### 8.3.1 Rules for every framework

- **The header.** Every file starts with the licence header and the link to its page (8.12).
- **The docstring names the node and the equations.** The module docstring says what the code computes in one sentence, names the page, and lists the equation ids that it implements.
- **Equation markers.** Each block that computes an equation of the page has a comment `eq: <id>` with an id that exists on the page (rule X11). The page highlights the block when the reader hovers the equation, and the reverse.
- **Shape comments.** Every array that the code creates or reshapes has a shape comment in the axis names of `shapes.yaml` (8.6): `# (B, T, H, D)`. The NumPy and PyTorch versions use the same names. Rule X13 checks that each name in a comment is an axis of the node's `shapes.yaml`.
- **Notation.** Variable names follow the notation register (06.4): `W_q`, `W_k`, `W_v`, `d_model`, `n_heads`, `logits`, `log_probs`. A name that differs from the page's symbol has a comment that maps it.
- **No hidden state.** No global random state, no module-level caches, no mutation of inputs. Randomness enters through an explicit generator argument: `rng: np.random.Generator` in NumPy, `generator: torch.Generator` in PyTorch, `key: jax.Array` in JAX.
- **Pure forward and backward in the reference.** The NumPy reference has `init(rng, config) -> params`, `forward(params, x, ...) -> (y, cache)` and, where the page derives the gradient, `backward(params, cache, dy) -> (dx, dparams)`. The PyTorch reference is a `torch.nn.Module` with the same `config` fields and a `from_params(params)` constructor, so the fixtures can load the same weights. The JAX reference is a pair of pure functions `init` and `apply` with the same parameter tree.
- **Readable first.** The reference is the plainest code that matches the page. A version that is faster but harder to read is a labelled version (8.5). A reference over 200 lines raises a warning (rule X14).
- **Numerically careful.** Softmax and log-softmax subtract the maximum; log-sum-exp is used for log-likelihoods; division by a standard deviation adds an `eps` that the page names; losses on logits use the logits form (8.11).

### 8.3.2 The table of tools

| Framework | Test | Format | Lint | Types | Mutation |
|---|---|---|---|---|---|
| `numpy`, `pytorch`, `jax` | pytest, with hypothesis for property tests | ruff format | ruff | pyright (basic) | mutmut |
| `c` | a small `aik` test runner over `assert.h` | clang-format | clang-tidy | the compiler | Mull |
| `cpp` | doctest | clang-format | clang-tidy | the compiler | Mull |
| `rust` | built-in `#[test]` | rustfmt | clippy `-D warnings` | the compiler | cargo-mutants |
| `julia` | Test | JuliaFormatter | Aqua.jl and JET.jl | JET.jl | `aik mutate` (8.8) |
| `typescript` | vitest | prettier | eslint | `tsc --strict` | StrykerJS |

### 8.3.3 The interface of each kind

Each kind with code has one interface, in `schema/impl.v1.yaml`, so that the fixtures and the harness can call every framework the same way.

| Kind | Entry points | Example |
|---|---|---|
| `method` (a layer, a regularizer, an attention variant) | `init`, `forward`, `backward` (reference), and `config` | dropout, layer normalization, LoRA, rotary position embedding |
| `method` (a training method: an optimizer, a loss, a schedule) | `init_state(params)`, `step(params, grads, state, hparams) -> (params, state)`, or `loss(logits, targets, ...)` | Adam, label smoothing, cosine schedule with warmup, DPO loss |
| `method` (an inference method) | `decode(model_fn, prompt, rng, hparams) -> tokens` against a fixed toy model | top-p sampling, beam search, speculative decoding with a fixed draft model |
| `architecture` | `init(rng, config)`, `forward(params, x, ...)`, `param_count(config)`, `flops(config, inputs)` | Transformer, ResNet-18, LSTM, Mamba block, U-Net |
| `metric` | `score(predictions, references, **options) -> float or dict` | BLEU, ROUGE-L, F1, mAP, pass@k, expected calibration error |
| `concept`, `finding`, `risk` (demo) | `demo(rng) -> dict` that produces the numbers of the page's worked example | the bias-variance decomposition on a toy problem; a prompt-injection string against a toy agent |

For metrics the interface is the contract of the node (`contract.yaml`, 04.4): the signature, the input and output types, the options that change the score (tokenization, smoothing, casing), and the tolerance.

### 8.3.4 Backward passes

- The NumPy reference writes the backward pass by hand for each method and architecture whose page has a gradient derivation (06.5). Each step of the backward pass carries the `eq:` id of the derivation step that it computes.
- When the page has no derivation (a large architecture made of derived parts), the NumPy reference composes the backward passes of its parts, and the test checks the composition.
- The PyTorch version does not write `backward` by hand unless the page is about a custom gradient (straight-through estimators, gradient reversal, the reparameterization trick, a custom attention kernel). Then it uses `torch.autograd.Function`, and the JAX version uses `jax.custom_vjp`.

## 8.4 The pinned package sets

An implementation imports only the packages of its framework's pinned set (rule X07). The sets live in `manifests/packages.yaml` with exact versions and hashes, and the runner image contains exactly them.

| Set | Allowed in `impl/<framework>/*.py` (not tests) | Allowed in tests |
|---|---|---|
| `numpy-ref` | `numpy`, the Python standard library | the set, plus `pytest`, `hypothesis`, `scipy` (for special functions and reference statistics) |
| `pytorch-ref` | `torch`, `numpy`, the standard library | the set, plus the test packages, plus the NumPy reference of the same node (imported through the fixture loader) |
| `jax-ref` | `jax`, `jax.numpy`, `numpy`, the standard library | as for `pytorch-ref` |
| tier 2 | the packages of 8.2.4 | the test framework of 8.3.2 |

The **no self-import rule** (X08): an implementation may not call the framework's own version of the thing that the node is about. `impl/pytorch/dropout.py` may not call `torch.nn.Dropout` or `torch.nn.functional.dropout`; `impl/pytorch/attention.py` may not call `torch.nn.functional.scaled_dot_product_attention` or `torch.nn.MultiheadAttention`; `impl/jax/adam.py` may not import `optax`. The tests may and should call them, as a second oracle (8.7.3). `registers/self-import.yaml` lists, for each node, the framework symbols that its code may not use; the curate step fills it, and the critic role reviews it.

Higher-level libraries (Flax, Optax, Equinox, Hugging Face Transformers, timm, torchvision models) are not in the reference sets. They appear in labelled `library` versions (8.5) and in experiments (09.4), where their own pinned sets apply.

## 8.5 Versions with labels

| Label | What it is | Rules |
|---|---|---|
| `reference` | the plain version that matches the page, and the one whose level is the X level | required; one per framework |
| `efficient` | a faster version that a page discusses: a fused kernel in plain PyTorch, a tiled attention that shows the FlashAttention idea, a vectorized beam search | it must pass every test of the reference; rule X16 needs it to be at least 1.3 times faster on the largest fixture, or to use at most 0.7 times the peak memory |
| `library` | how to do the same thing with the framework's library: `torch.nn.functional.scaled_dot_product_attention`, `optax.adam`, `torchmetrics` | it must agree with the reference on the fixtures; it shows the reader the API and its defaults, and the page lists the differences of default (an `eps`, a bias-correction choice, a smoothing method) |
| `historical` | the form of the original paper when it differs from the modern one (post-norm Transformer, the original LSTM without forget gate, Adam without bias correction) | it must pass its own fixtures, and the page says how it differs |

Each version is a file with the label in its name (`attention_efficient.py`) and a `versions.yaml` entry in `impl/<framework>/` with the label, a one-line note and the page section that discusses it.

## 8.6 The shape contract of an architecture

An architecture node has `shapes.yaml`. It states the tensor shapes of each block, the parameter count and the FLOP count as formulas in the config variables, and the build checks every formula against every framework's code.

```yaml
# nodes/deep-learning-architectures/transformers/transformer/shapes.yaml (part)
config:
  d_model: {meaning: "width of the residual stream", example: 512}
  n_heads: {meaning: "number of attention heads", example: 8}
  d_head:  {meaning: "width of each head", formula: "d_model / n_heads"}
  d_ff:    {meaning: "width of the MLP hidden layer", example: 2048}
  n_layers: {meaning: "number of blocks", symbol: "L", example: 6}
  vocab:   {meaning: "size of the vocabulary", example: 32000, sample: [5, 20]}
axes:
  B: {meaning: batch, example: 2}
  T: {meaning: sequence length, example: 5}
  H: n_heads
  D: d_head
  M: d_model
  F: d_ff
  V: vocab
blocks:
  - id: attn
    eq: [attn-scores, attn-softmax, attn-output]
    inputs:  {x: "(B, T, M)"}
    outputs: {y: "(B, T, M)"}
    internals: {q: "(B, H, T, D)", k: "(B, H, T, D)", v: "(B, H, T, D)", scores: "(B, H, T, T)"}
    params: "4 * M * M + 4 * M"            # W_q, W_k, W_v, W_o with biases
    flops_forward: "8 * B * T * M * M + 4 * B * T * T * M"
  - id: mlp
    eq: [mlp]
    inputs: {x: "(B, T, M)"}
    outputs: {y: "(B, T, M)"}
    params: "2 * M * F + F + M"
    flops_forward: "4 * B * T * M * F"
  - id: ln1
    params: "2 * M"
  - id: ln2
    params: "2 * M"
  - id: block
    of: [attn, mlp, ln1, ln2]
    params: "4 * M * M + 2 * M * F + 9 * M + F"
model:
  params_non_embedding: {formula: "n_layers * block.params", symbol: "N", meaning: "parameters without the embedding"}
  params: "params_non_embedding + V * M"   # tied embeddings
  flops_per_token_forward: "2 * params_non_embedding + 2 * n_layers * T * M"
  sources:
    - {work: vaswani-2017-attention, locator: "Section 3.3 and Table 1"}
    - {work: kaplan-2020-scaling-laws, locator: "Section 2.1, Table 1"}
configs:
  base: {d_model: 512, n_heads: 8, d_ff: 2048, n_layers: 6, vocab: 37000, source: {work: vaswani-2017-attention, locator: "Table 3"}}
```

The parts of the file:

- **`config`** has the size variables. A variable has an `example` (a whole number of at least 1) or a `formula` in the other names, never both. `meaning` is the text of the table, `symbol` is its TeX in the formulas of the page, and `sample: [low, high]` is the range of `aik shapes sample` (by default `[min(2, example), min(example, 8)]`).
- **`axes`** has the letters of the tensor shapes. An axis whose value names a config variable (`H: n_heads`) is another name for it. Any other axis is free, an input of the forward pass such as the batch or the sequence length, and it needs `{meaning, example}`.
- **`blocks`** has the parts. `params` and `flops_forward` are formulas. A block with `of` is made of other blocks: when it has no `params` of its own, its count is the sum of its parts; when it has one, the parts must add up to it. A block with no `flops_forward` has no multiply-adds; its minor FLOPs are not counted. `eq` names the equation anchors `{#eq-...}` of `node.md` that the block computes.
- **`model`** has the totals. Each key other than `sources` and `note` is a formula, as a string or as `{formula, meaning, symbol}`. `params` is required.
- **`configs`** has named configs, such as the base and big models of a paper, each with an optional `source`. A config sets the size variables and free axes that it names; the others keep their example.

A formula has whole numbers, the names of the file, `block.params` and `block.flops_forward`, the operators `+ - * / ^` and parentheses. The build evaluates it exactly, with rationals, and a size, a count or a dimension that is not a whole number is an error.

`aik shapes sample -w <slug>` writes `fixtures/shapes.json`: the example config, two small random configs from a seed that is the hash of the node's path (so that every machine draws the same ones), and each named config. For each config it gives every size, the shape of every input, output and internal of each block, each block's `params` and `flops_forward`, and each total. The tests of every framework read this file. `aik shapes show <slug>` prints the counts of each config.

The checks (rule X09):

1. **The file.** Every name of every formula is defined, nothing depends on itself, the sizes are whole numbers at the example and at the named configs, the parts of each block add up to it, and each `eq` names an anchor of `node.md`. `aik check` runs this on every pull request, and an error blocks.
2. **Shapes.** The test harness runs `forward` of each framework on random inputs for the example config and the two sampled configs of `fixtures/shapes.json`, and checks the shape of each output and of each internal that the code exposes through `cache`.
3. **Parameter count.** `param_count(config)` of the code, the count of the actual parameter arrays, and the formula of `shapes.yaml` agree exactly for every config of the fixture.
4. **FLOPs.** `flops(config, inputs)` of the code and the formula agree exactly. The build also counts the multiply-adds of the NumPy forward with an instrumented array type (`aik flopcount`) on the small configs and checks that it agrees with the formula within the conventions of `schema/flops.md` (a multiply-add is 2 FLOPs; softmax, normalization and activations are counted separately and listed under `flops_minor`).
5. **The fixture is current.** `fixtures/shapes.json` is what `aik shapes sample` gives from `shapes.yaml`; a node with code and no fixture, or with a stale one, stays below X2.
6. **The page and the shapes agree.** Every parameter count and FLOP formula that the page prints is rendered from `shapes.yaml`, never typed in the text (rule X20). The generated section "Shapes and parameters" has the sizes, the blocks with their shapes and formulas, and the totals at each config. In the prose, `{shape:params}` prints the formula of `params` as math, and `{shape:params@base}` prints its value at the config `base` (or `example`); a reference that names no formula or no config blocks. The calculators of 16.14 use the same formulas.

## 8.7 Tests

### 8.7.1 Own tests

Each implementation has tests in its framework (8.3.2) for:

- **shapes** of outputs for the configs of 8.6, and for the edge configs: batch of one, sequence of one, one head, odd sizes;
- **the worked example** of the page: the same small input gives the numbers that the page prints (06.6); the page's numbers are rendered from this test's fixture, so they cannot drift;
- **edge cases** of the kind: an all-masked attention row, a zero-variance input to normalization, `p = 0` and `p` close to 1 for dropout, empty hypothesis and no n-gram overlap for BLEU, an empty reference set;
- **invariants** stated on the page, as property tests with hypothesis: softmax is invariant to adding a constant to every logit; attention without positions is equivariant to permuting the sequence; layer normalization output has zero mean and unit variance per position up to `eps`; a metric is symmetric when the page says it is.

### 8.7.2 Shared fixtures

`fixtures/` in the node directory holds the shared inputs, made by `fixtures/make.py` with a fixed seed, and stored as small `.npz` files (at most 1 MB per node, rule X15):

```
fixtures/
  make.py                 deterministic; writes the files below
  params-small.npz        a parameter tree in the reference's layout
  input-small.npz         inputs, masks, targets
  expected-small.npz      the NumPy reference's outputs and gradients, in float64
  manifest.yaml           seeds, configs, the hash of each file, the NumPy version that made it
```

Every framework loads the same parameters, runs the same inputs, and compares with `expected-small.npz`. When the NumPy reference changes, `aik fixtures make` rewrites the expected files, and the pull request shows the diff of every number that changed.

### 8.7.3 Differential tests

| Comparison | Inputs | Tolerance | Needed for |
|---|---|---|---|
| framework reference vs NumPy reference, forward | shared fixtures, and 50 random inputs per PR run (500 nightly) from `aik fixtures sample` | float64: `rtol=1e-10, atol=1e-12`; float32: `rtol=1e-5, atol=1e-6`; the node's `contract.yaml` or `shapes.yaml` may loosen it with a reason | X2 |
| framework reference vs NumPy reference, gradients | the same | float64: `rtol=1e-8, atol=1e-10` | X2 |
| framework reference vs the framework's own library version (`torch.nn.functional.layer_norm`, `jax.nn.softmax`, `optax.adam`) | the same | float32 tolerances; known differences of default are set explicitly in the test and listed in `versions.yaml` | X2 when such a library version exists |
| labelled versions vs reference | the same | float32 tolerances | for each labelled version |
| tier 2 vs NumPy reference | shared fixtures, read from `.npz` through a small loader in each language | float64 tolerances | X2 for tier 2 |

Tests run on CPU, in float64 unless the node is about precision (mixed precision, quantization), where the contract names the dtypes and tolerances. GPU numerics are the business of experiments (09.8), not of X levels.

### 8.7.4 Gradient checks

For each `backward` of the NumPy reference and for the autodiff gradients of each framework:

- central finite differences in float64, `h = 1e-6` scaled by the magnitude of each input, on the small fixtures, for every parameter and input entry when there are at most 2,000 of them, otherwise for 2,000 entries sampled with a fixed seed;
- the check passes when the relative error `|g - g_fd| / max(|g|, |g_fd|, 1e-8)` is under `1e-6` for every checked entry;
- non-differentiable points (ReLU at 0, max pooling ties, `abs` at 0) are avoided by the sampler, which moves inputs away from the kinks the contract lists in `kinks:`;
- stochastic layers are checked with a fixed mask or a fixed key, so that the function is deterministic for the check.

A gradient check failure in the NumPy reference blocks X1 (the derivation and the code disagree; rule X04). In a framework version, it blocks X2.

### 8.7.5 Metric test vectors

A metric node has `vectors/` (04.4):

- `official.jsonl`: inputs and outputs computed by the official or canonical implementation, with its name, version and options (sacreBLEU with its signature string for BLEU and chrF; `rouge-score` for ROUGE; `pycocotools` for COCO mAP; the `human-eval` package for pass@k; `jiwer` for WER; the scikit-learn functions for classification metrics; `torchmetrics` and `clean-fid` where the field uses them). `aik vectors make <node>` runs the official implementation in a separate image, `aik-vectors`, that has those packages, and writes the file. The official package never enters the reference sets.
- `edge.jsonl`: the edge cases of the contract, with the expected outputs computed by the official implementation where it accepts them, and by hand with a note where it does not.
- `variants.yaml`: the options that change the score (tokenization, casing, smoothing, the interpolation of the precision-recall curve) and the vector files for each one. The page shows the options as a table and says which ones the main papers use (10.5).

Every framework's `score` must reproduce every vector within the contract's tolerance (rule X05). This is how a reader knows that our BLEU is the BLEU of the papers, or exactly how it differs.

### 8.7.6 The learning smoke test

Each architecture and each training method has one test that the code can learn: the model fits a tiny fixed batch (for example 8 examples of a copy task or 32 points of a two-moons set) to a loss under a threshold within a fixed number of steps on CPU. It catches wrong signs, detached gradients and broken masks that shape and gradient tests can miss. It runs in at most 30 seconds per framework, and it is part of X2. It is not an experiment: it makes no claim about the method (09.1).

## 8.8 Mutation testing

As in algorithm 09.6, with the operators adapted to numerical code:

- **Tools.** mutmut for the Python frameworks, Mull for C and C++, cargo-mutants for Rust, StrykerJS for TypeScript, and `aik mutate` (the generic mutator on the tree-sitter grammar, from algorithm) for Julia and as the common operator set for all.
- **Operators.** The algorithm set (relational and arithmetic operator changes, constants by plus or minus one, boolean negation, statement deletion, argument swaps) plus numerical operators: swap two axes in a `transpose`, `sum` or `mean`; change the axis of a reduction; drop a `keepdims`; change `sqrt(d)` to `d`; remove the max subtraction of a softmax; change `eps` by a factor of 1,000; swap `mean` and `sum`; drop a mask; change a `+ 1` in a bias correction; flip the sign of a gradient term.
- **What kills a mutant.** The own tests, the shared fixtures, the differential tests and the gradient checks. A mutant that crashes or times out counts as killed.
- **Equivalent mutants.** Listed in `impl/<framework>/mutants.ignore` with a reason, at most 5% (rule X10). Example: removing the max subtraction in a softmax is equivalent on the fixtures but not on large logits, so the right answer is a test with large logits, not an ignore entry.
- **The threshold.** At least **80%** of the non-equivalent mutants killed for X3 (rule X06). The threshold is lower than algorithm's 85% because numerical mutants near tolerances are harder to kill without making tests brittle. M2 measures the real distribution and may raise it (Default).
- **The budget.** At most 150 mutants per file in PR runs, sampled with a fixed seed. The nightly run tests all mutants of files that changed since the last release.

## 8.9 Runner images and the sandbox

### 8.9.1 Images

- Each framework has one CPU image, `ghcr.io/tamnd/aik-runner-<framework>`, built from `ai-agent/images/<framework>/Dockerfile`. It holds the toolchain, the pinned package set, the test, format, lint, type and mutation tools.
- Experiments use separate images, `ghcr.io/tamnd/aik-exp-<stack>` (for example `aik-exp-pytorch-cu`), with GPU builds of the frameworks and the pinned experiment packages (09.4).
- `aik-vectors` holds the official metric implementations (8.7.5). It runs only `aik vectors make`, never the code of a pull request.
- `manifests/images.yaml` in the content repository pins each image by digest, as in algorithm. A new image is a pull request that moves the digest and carries the full test matrix report of the framework.
- Images are built for `linux/amd64` and `linux/arm64`. The nightly run uses both; a test that passes on one and fails on the other is a finding (rule X17).

### 8.9.2 The sandbox

| Limit | Value |
|---|---|
| network | none (`--network none`) |
| file system | the build tree read-only; a tmpfs of 1 GiB |
| user | unprivileged, no new privileges, the default seccomp profile |
| CPU | 2 cores; `OMP_NUM_THREADS`, `MKL_NUM_THREADS` and `XLA_FLAGS` set to match, so that results do not depend on the machine |
| memory | 4 GiB per test run (PyTorch and JAX need more than algorithm's 2 GiB) |
| time | 120 s for the own tests of one framework of one node, 60 s for the differential and gradient tests, 30 s for the learning smoke test, 900 s for a mutation run of one node |
| processes | at most 256 |

Code from a pull request runs only inside the sandbox, on CI runners or the build server (rule X18: `aik test` refuses to run outside a container unless `-unsafe-local` is given).

### 8.9.3 The cache

A test result is cached under the hash of the implementation files, the fixtures, the vectors, `shapes.yaml`, `contract.yaml`, the test files and the image digest. A change to the NumPy reference invalidates every framework of that node, because it is their oracle.

### 8.9.4 Scale (Estimate)

About 3,000 nodes with code at M5, times 3 tier-1 frameworks, plus about 400 core nodes in 5 tier-2 frameworks, gives about 9,000 implementations, the number of 02.4 once versions are not counted. A cold full run on the build server (32 cores) is estimated at 4 to 6 hours, most of it mutation testing; a warm nightly run at under 40 minutes. M2 measures both (21).

## 8.10 The execution ladder X0 to X4

The build computes one level for each node and each framework, from the reference version (04.10).

| Level | It needs | Rules |
|---|---|---|
| X0 | no reference version, or one that fails a check below X1 | |
| X1 | it runs in its pinned image with no network; it imports only its package set; it passes its own tests (shapes, the worked example, edge cases, invariants); for the NumPy reference, the hand-written backward passes the gradient check | X01, X03, X04, X07, X08, X11, X12 |
| X2 | X1, and it agrees with the NumPy reference on the shared fixtures and the random differential inputs within the tolerance; autodiff gradients pass the finite-difference check; metrics reproduce the official test vectors; architectures pass the shape contract; the learning smoke test passes | X02, X04, X05, X09 |
| X3 | X2, and format, lint and type checks are clean; the conventions of 8.3 hold (shape comments, equation markers, no hidden state); its tests kill at least 80% of the mutants | X06, X10, X13 |
| X4 | X3, and a person who is not the author reviewed this exact file hash (15.5) | |

For the NumPy reference, X2 means: it passes the shared fixtures that it made itself, the official vectors (metrics) and the shape contract (architectures), and it agrees with at least one framework's library version where one exists (the second oracle).

A node is "runs everywhere" (15.1) when `numpy` and `pytorch` are at X3 and `jax` at X2, the target of 04.10.

## 8.11 Numerics

| Topic | Rule |
|---|---|
| dtype in tests | float64 for every comparison, unless the node is about precision |
| dtype in the reference | the code is generic in dtype; it never casts to float32 silently |
| softmax, log-softmax, log-sum-exp | subtract the maximum; tests include logits of magnitude 1e4 |
| cross-entropy | computed from logits with log-softmax; the probability form is a labelled `historical` or teaching version |
| normalization | the `eps` of the page, inside the square root as the page writes it; the page says where frameworks differ (PyTorch's `LayerNorm` and JAX's defaults) |
| masks | additive masks use the dtype's minimum or `-inf` with a guard for all-masked rows; the edge test covers the all-masked row |
| random numbers | explicit generators; the fixtures store the masks and noise, so frameworks with different random number generators compare on the same draws |
| NaN and Inf | the tests fail on any NaN or Inf in outputs or gradients unless the contract says the output may be infinite |

## 8.12 The licence header

```python
# SPDX-License-Identifier: MIT
# Part of https://ai.tamnd.com/dropout/ (ai://3KD9QX). The authors are listed in prov/3KD9QX.jsonl.
```

Rule X19 checks the header. The code is MIT (19.2). Code adapted from a source with a compatible licence keeps the source's notice in the file and in `prov`, and the copy check (13.7) compares every implementation with the code corpora of the raw store.

## 8.13 A worked example: scaled dot-product attention

The NumPy reference, with its backward pass:

```python
# SPDX-License-Identifier: MIT
# Part of https://ai.tamnd.com/scaled-dot-product-attention/ (ai://7HQ2MV). The authors are listed in prov/7HQ2MV.jsonl.
"""Scaled dot-product attention, forward and backward.

Implements eq: attn-scores, attn-softmax, attn-output and the gradients of derivations/gradient.md.
"""
import numpy as np


def forward(q, k, v, mask=None):
    """q: (B, H, T, D), k: (B, H, S, D), v: (B, H, S, D), mask: (B, 1, T, S) bool, True = keep."""
    d = q.shape[-1]
    scores = q @ np.swapaxes(k, -1, -2) / np.sqrt(d)        # eq: attn-scores   (B, H, T, S)
    if mask is not None:
        scores = np.where(mask, scores, -np.inf)
    m = np.max(scores, axis=-1, keepdims=True)              # eq: attn-softmax  (B, H, T, 1)
    m = np.where(np.isfinite(m), m, 0.0)                    # all-masked rows
    e = np.exp(scores - m)                                  # (B, H, T, S)
    z = np.sum(e, axis=-1, keepdims=True)                   # (B, H, T, 1)
    a = np.divide(e, z, out=np.zeros_like(e), where=z > 0)  # (B, H, T, S)
    y = a @ v                                               # eq: attn-output   (B, H, T, D)
    return y, (q, k, v, a)


def backward(cache, dy):
    """dy: (B, H, T, D). Returns dq, dk, dv."""
    q, k, v, a = cache
    d = q.shape[-1]
    dv = np.swapaxes(a, -1, -2) @ dy                        # eq: grad-v        (B, H, S, D)
    da = dy @ np.swapaxes(v, -1, -2)                        # eq: grad-a        (B, H, T, S)
    ds = a * (da - np.sum(da * a, axis=-1, keepdims=True))  # eq: grad-softmax  (B, H, T, S)
    dq = ds @ k / np.sqrt(d)                                # eq: grad-q        (B, H, T, D)
    dk = np.swapaxes(ds, -1, -2) @ q / np.sqrt(d)           # eq: grad-k        (B, H, S, D)
    return dq, dk, dv
```

The PyTorch reference computes the same three equations with `torch` operations and leaves the gradient to autograd. Its test loads `fixtures/`, compares the forward and the gradients with `expected-small.npz`, and compares with `torch.nn.functional.scaled_dot_product_attention` as the second oracle. The JAX reference is the same in `jax.numpy`, tested with `jax.grad` against the same file. The own tests check the shapes for `S != T`, the all-masked row, the invariance of the output to adding a constant to all scores of a row, and the permutation equivariance of the output when `q`, `k` and `v` are permuted together with no mask.

## 8.14 Who writes implementations

The implementer role (14.2) writes the NumPy reference from the page and its derivations, then each framework version, then the tests, in that order, under the gate of 14.6. The tester role adds tests that kill surviving mutants. A different model family audits the code against the page (15.4). A person reviews core implementations for X4.

## 8.15 The rules of this document

The rules are listed in 15.3 with their severities. In short:

| Rule | Checks |
|---|---|
| X01 | the NumPy reference exists and is at X1 before any other framework version of the node merges |
| X02 | the framework reference agrees with the NumPy reference on the fixtures and the random inputs, forward and gradients, within the tolerance |
| X03 | it runs in its pinned image with no network and passes its own tests |
| X04 | the hand-written backward (NumPy) and the autodiff gradients (frameworks) pass the finite-difference check |
| X05 | a metric reproduces every official and edge test vector within the contract's tolerance |
| X06 | its tests kill at least 80% of the non-equivalent mutants |
| X07 | it imports only the packages of its framework's pinned set |
| X08 | it does not call the framework's own version of the node's subject (the self-import register) |
| X09 | an architecture passes its shape contract: shapes, parameter count and FLOP formulas agree with the code |
| X10 | at most 5% of mutants are marked equivalent, each with a reason |
| X11 | every `eq:` marker names an equation id of the page, and every equation in the page's implementation section has a marker |
| X12 | the framework is in the register, and file names follow its conventions |
| X13 | format, lint and type checks are clean; shape comments use the axes of `shapes.yaml` |
| X14 | warning: a reference version is longer than 200 lines |
| X15 | fixtures are deterministic, at most 1 MB per node, and the manifest hashes match |
| X16 | an `efficient` version is at least 1.3 times faster or uses at most 0.7 times the peak memory on the largest fixture |
| X17 | results are the same on amd64 and arm64 within the tolerance |
| X18 | tests of a pull request run only in the sandbox |
| X19 | the licence header is present and names the node's page and tag |
| X20 | the numbers that the page prints from a worked example come from the test fixture, and its parameter and FLOP formulas from `shapes.yaml`, never from the text |
