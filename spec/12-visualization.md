# 12. Visualization

A visualization shows an AI idea working.
The reader can step a search tree forward and back, watch the shapes of tensors flow through a Transformer block as they change its width, train a tiny network in the page and see its decision boundary move, read an attention map of a real small model, run Q-learning on a grid world, and see benchmark scores and training compute plotted over time with every point linked to its source.
One runtime, `@aik/viz`, serves all of it.
Each scene is checked against the NumPy reference of the node (08), so the picture shows the real computation and not a cartoon of it.

This document copies algorithm's visualization design (2150/10: the trace protocol, the primitives, the player, code sync, frames, accessibility, the checks) and says what AI adds: tensor and training primitives, scenes generated from `shapes.yaml`, recorded traces from experiments, the in-browser notebooks, and charts of the registers.
Where this document is silent, 2150/10 applies.

## 12.1 Principles

1. **The picture follows the math.** A scene shows the objects that the derivation of the page talks about (06.5): the query, key and value matrices of attention, the mask of dropout, the advantage of policy gradient, the frontier of A\*. The narration uses the notation of the page (06.4) and links to the derivation step that it shows.
2. **The reader drives.** Step, step back, scrub, change the input, change a hyperparameter. Autoplay is the second mode, not the first.
3. **Trace, then draw.** A scene runs its computation to completion and records a trace of events. The player draws the trace. The two never mix, so the same trace drives the player, the static frames and the checks (12.11, 12.14). This is algorithm's rule.
4. **Real numbers, small sizes.** A training scene trains a real network with the reference code, on a toy problem small enough to run in the page. It never draws a loss curve that no computation made.
5. **Data charts are computed, dated and sourced.** A chart of the registers (12.9) is drawn from cards and results by the build, shows its population rule and its `as_of` date, and links each point to its source. No chart is drawn by hand, and no chart says "state of the art" (K04).
6. **Same primitives everywhere.** A tensor looks the same on every page, an attention map the same in every model. A reader learns the visual language once.
7. **Readable without colour, without motion and without script.** Each state has a mark as well as a colour. Reduced motion turns tweens off. With no script, the page shows a filmstrip of static frames and the data table of each chart.

What we learn from others, without copying their code or assets (19.3): Distill and the Transformer Circuits thread show what careful explorables and small multiples can do; the TensorFlow Playground (Apache-2.0) shows that a reader learns from training a tiny network in the page; the "Illustrated Transformer" and "Transformer Explainer" show that shape flow is the hard part for learners; Our World in Data and Epoch AI show that every point of a data chart should link to its source and that log axes need labels a reader can read.
These are linked from the pages that they help, and their figures are never copied.

## 12.2 Scene sources

A node has at most one `viz/` directory (04.4). It holds one of these:

| File | Kind of visualization | Used by | Runs |
|---|---|---|---|
| `viz/scene.ts` | a **step scene**: a computation with discrete steps, as in algorithm | search, planning, MCTS, beam search, backprop on a computation graph, BPE merges, value iteration, the forward pass of a block | in the browser, from the reader's input |
| `viz/scene.ts` with `kind: "train"` | a **training scene**: a tiny model trained in the page | MLP on 2-D data, logistic regression, k-means view, a 1-layer attention model on a copy task, a VAE on 2-D points, a policy on a grid | in the browser, in a Web Worker, seeded |
| `viz/shapes.scene.yaml` | an **architecture scene** generated from the node's `shapes.yaml` (08.6) | every `architecture` node with a shape contract | generated at build; the reader changes the sizes in the page |
| `viz/recorded.yaml` | a **recorded scene**: the trace comes from a run that is too large for the browser, recorded once | attention maps of a small open model, training curves of an experiment of 09, activations of a real checkpoint | the trace is a file in `ai-site`, made by `aik viz record` in the experiment's image |
| `viz/explorable.ts` | an **explorable**: a concept with controls and linked views, no steps | bias and variance, softmax temperature, ROC and threshold, KL divergence, double descent | in the browser |
| `viz/charts.yaml` | **register charts**: data charts over cards and results | model, benchmark, hardware, incident and policy pages, area pages, catalogues | generated at build from the registers (12.9) |
| `viz/presets.yaml` | the named inputs and settings of the scene | all of the above except charts | |

Every `.ts` file imports only `@aik/viz`, the runtime of `tamnd/ai-agent` (MIT, 03.1).
`@aik/viz` starts as a copy of algorithm's `@algo/viz` (2150/10.2), so the step protocol, the player and the primitives `array`, `graph`, `tree`, `table`, `stack`, `queue`, `grid` and `chart` are the same, and algorithm's scenes for k-means, A\* or alpha-beta could be read by this player.
It does not import `@algo/viz`; the two copies change apart, as for the Go packages (03.4).

## 12.3 The trace protocol

The trace format is algorithm's (2150/10.2.2 and 10.2.3) at version 2: a list of steps, each with a `line` id, a `note` of at most 140 characters, the events, the watched `vars`, and a `phase`.
Version 2 adds:

- **Tensor values.** A `vars` entry may be a tensor reference `{"t": "q", "shape": [4, 8], "dtype": "f32"}` whose data is in a binary side file (`trace.bin`, little-endian, one buffer per tensor id and step). This keeps traces of numeric scenes small.
- **Derivation links.** A step may have `eq`, the id of an equation of the page's derivation (06.5), so that the player can highlight the equation that the step computes.
- **Metrics.** A step may carry `metrics` (`{"loss": 0.412, "acc": 0.87}`), which the training primitives plot.
- **Seeds.** The trace header records the seed, the preset id and the version of `@aik/viz`, so that a trace can be rebuilt exactly.

New event groups:

| Group | Events | Example |
|---|---|---|
| tensor | `tensor.create(id, shape, data?)`, `tensor.set(id, data)`, `tensor.slice(id, ranges, state)`, `tensor.op(name, inputs, output)`, `tensor.reshape(id, shape)`, `tensor.broadcast(id, shape)` | `v.op("matmul", ["Q", "K^T"], "S")` |
| compute graph | `cg.node(id, op, inputs)`, `cg.forward(id, value)`, `cg.backward(id, grad)`, `cg.accumulate(id, grad)` | the forward and backward passes of backpropagation |
| training | `train.step(t)`, `train.batch(indices)`, `train.update(param, delta)`, `train.eval(split, metrics)`, `train.boundary(grid)` | one SGD step on a minibatch |
| tokens | `tok.text(s)`, `tok.split(spans)`, `tok.merge(a, b, new)`, `tok.ids(ids)`, `tok.probs(position, top)` | a BPE merge; the next-token distribution at a position |
| distribution | `dist.set(id, probs, labels)`, `dist.temperature(id, t)`, `dist.truncate(id, rule)`, `dist.sample(id, index)` | top-p truncation of a softmax |
| attention | `attn.map(layer, head, matrix)`, `attn.mask(kind)`, `attn.focus(query_index)` | a causal mask applied to a score matrix |
| environment | `env.reset(state)`, `env.act(action)`, `env.reward(r)`, `env.value(state, v)`, `env.policy(state, probs)`, `env.done()` | one step of Q-learning on a grid world |
| search tree | `st.expand(node, children)`, `st.select(path)`, `st.rollout(node, result)`, `st.backup(path, value)`, `st.prune(node)` | the four phases of MCTS |

## 12.4 The primitives

`@aik/viz` has the primitives of `@algo/viz` (2150/10.2.4) and these new ones.
A scene uses only primitives; a new primitive is a change to `@aik/viz` with its own review, not drawing code in one scene (rule V13).

| Primitive | Draws | Views | Used by (examples) |
|---|---|---|---|
| `tensor` | a tensor of rank 0 to 4 as a grid of cells, with its shape label and named axes | `cells` (values), `heat` (diverging colour with values in the text view), `block` (shape only, sized boxes) | attention, convolution, normalization, embeddings, quantization |
| `layers` | an architecture as a stack or graph of blocks, each with its input and output shapes and parameter count | `stack`, `graph`, `unrolled` (for recurrent nets) | every architecture scene (12.6) |
| `compgraph` | a computation graph with forward values and backward gradients on its edges | `layered` | backpropagation, autodiff, the chain rule, residual gradients |
| `heatmap` | a matrix with labelled rows and columns | `square`, `triangle` (causal), `bipartite` (query to key lines) | attention maps, confusion matrices, correlation, transition matrices |
| `plane2d` | points, a decision boundary or a field on a 2-D plane | `scatter`, `boundary`, `contour`, `vector-field` | classifiers on toy data, loss surfaces, gradient descent paths, diffusion on 2-D points, score fields |
| `curve` | a metric against steps or a hyperparameter, with confidence bands | `line`, `band`, `log` | training loss, validation loss, learning-rate schedules, double descent |
| `tokens` | a token strip with ids, boundaries and per-token values | `strip`, `probs` | tokenization, next-token probabilities, log-likelihood per token, KV cache growth |
| `dist` | a discrete distribution as bars | `bars`, `sorted`, `cumulative` | softmax, temperature, top-k and top-p, policies, class probabilities |
| `embedding` | vectors projected to 2-D, with the projection named | `pca`, `fixed` (precomputed coordinates) | word embeddings, CLIP space, representation drift; t-SNE and UMAP only as recorded coordinates with the method named |
| `image` | small images and kernels, with per-pixel values in the text view | `pixels`, `kernel`, `patches` | convolution, filters, ViT patching, saliency, adversarial perturbations, diffusion samples |
| `gridworld` | an environment grid with walls, rewards, an agent, values and arrows for the policy | `values`, `policy`, `q` | MDPs, value iteration, Q-learning, reward hacking demos |
| `searchtree` | a search or game tree with visit counts and values | `tidy`, `radial` | MCTS, minimax, beam search, tree of thoughts |
| `timeline` | dated events and intervals | `axis`, `swimlanes` | history pages, policy pages (dates of application), model families |

### 12.4.1 States and colour scales

The state vocabulary of algorithm (2150/10.2.5: `default`, `active`, `compare`, `swap`, `pivot`, `frontier`, `visited`, `path`, `rejected`, `invariant`) is kept, with marks as well as colours.
AI adds these states:

| State | Meaning | Mark (besides colour) |
|---|---|---|
| `masked` | dropped, masked out, padded | grey with diagonal hatch |
| `attended` | the keys that the focused query attends to most (top share of weight) | thick outline with a weight label |
| `gradient` | carrying a gradient in the backward pass | arrow pointing back |
| `updated` | a parameter changed in this step | small triangle marker |
| `sampled` | the value drawn from a distribution | star marker |
| `out-of-range` | a value clipped by quantization or overflow | cross marker |

Numeric values use two scales only, both colour-blind safe and readable in both themes: a **sequential** scale for non-negative values (probabilities, attention weights, counts) and a **diverging** scale centred on zero for signed values (weights, gradients, advantages).
The scale and its numbers are shown in a legend on each view.
A colour never carries a value alone: the text view (12.12) shows each value as a number.

## 12.5 The player

The player is algorithm's (2150/10.3): a stage, a timeline with phase chapters, step forward and back, scrub, speed, input and preset controls, the narration line, the watched variables, and a code panel.
AI adds:

- **The equation panel.** For a step with `eq`, the derivation equation of the page is shown and highlighted, so the reader sees which line of math the step computes.
- **The hyperparameter panel.** Training scenes and explorables have sliders and selects defined in `presets.yaml` (learning rate, batch size, width, depth, dropout rate, temperature, number of heads). A change reruns the scene in the Web Worker and keeps the old run as a faint overlay, so the reader can compare two settings.
- **Code sync in three frameworks.** The code panel has tabs for the NumPy reference and the PyTorch and JAX versions (08). Each step's `line` id maps to the `# @line` markers of each file, as in algorithm's code sync (2150/10.5). The NumPy tab is the default, because it is what the scene runs.
- **Seed control.** A training scene shows its seed and a "new seed" button, so the reader can see how much a result varies. The default preset uses the seed of the checked trace.

## 12.6 Architecture scenes from `shapes.yaml`

Every `architecture` node has a shape contract, `shapes.yaml` (08.6): its size variables (`d_model`, `n_heads`, `d_ff`, `n_layers`, `seq_len`, `vocab`, `batch`), the blocks, the shape of each tensor between blocks as an expression in the variables, and the formulas for parameters and FLOPs.
`aik viz shapes <slug>` generates the architecture scene from it. Nobody draws architecture diagrams by hand.

The scene:

```
┌ Transformer block (pre-norm) ───────────────────────────────── d_model [512 ▾] heads [8 ▾] seq [128 ▾] ┐
│                                                                                                         │
│  x  [B, T, d]          ── LayerNorm ──▶  [B, T, d]                                                       │
│     │                                      │                                                             │
│     │                         ┌── W_Q ──▶ Q [B, h, T, d/h] ──┐                                           │
│     │                         ├── W_K ──▶ K [B, h, T, d/h] ──┼─▶ softmax(QKᵀ/√(d/h) + M) V ─▶ [B, T, d]  │
│     │                         └── W_V ──▶ V [B, h, T, d/h] ──┘                │                          │
│     └────────────────────────────── + ◀──────────── W_O ◀─────────────────────┘                          │
│                                     │                                                                    │
│                                     ▼  … LayerNorm ─▶ MLP [B, T, d_ff] ─▶ [B, T, d] ─▶ +                 │
│                                                                                                         │
│  Parameters in this block: 12·d² + 13·d  = 3,152,384     FLOPs per token (forward): 24·d² + 4·T·d       │
└─────────────────────────────────────────────────────────────────────────────────────────────────────────┘
```

- The reader changes a size variable, and every shape, the parameter count and the FLOP count update. The formulas are the ones of `shapes.yaml`, the same that the calculators use (16.14) and that the shape tests check against the PyTorch and JAX implementations (08.6). So the diagram, the calculator and the code cannot disagree (rule V14).
- Clicking a block opens its own node (`[[multi-head-attention]]`, `[[layer-normalization]]`) or, when the block has a step scene, plays it in place.
- A model node (04.2) whose `architecture` has a shape contract shows the same scene with the model's published sizes filled in and locked, with a source for each size (04.9).
- Variants show their difference: the scene of `rmsnorm` or of `grouped-query-attention` can be shown as a diff against its parent's scene, with the changed blocks marked `updated`.

## 12.7 Training scenes and recorded traces

**Training in the page.** A training scene runs a real, tiny training loop in a Web Worker. It uses the `@aik/viz` port of the NumPy reference of its node: a small TypeScript array library with `Float64Array`, the same initialization, the same seeded PRNG (PCG32, as in the NumPy fixtures of 08.7) and the same order of operations.
Limits: at most 10,000 parameters, at most 2,000 steps, at most 2 seconds of compute on the reference phone (12.13).
Examples: an MLP on the two-spirals data with the boundary redrawn every 20 steps; dropout on and off side by side; momentum and Adam on the Rosenbrock and other 2-D surfaces; a one-layer attention model learning to copy a sequence of 8 tokens; Q-learning on a 6 by 6 grid with a reward-hacking variant; a diffusion model on 2-D points with the forward and reverse processes.

**The agreement check.** For the default preset, `aik viz check` runs the same training with the NumPy reference in its image and compares the metrics of the trace with the NumPy run at every recorded step, within a tolerance set in `presets.yaml` (Default: relative 1e-6 on the loss for float64). This is rule V03, the AI form of algorithm's oracle check, and it makes the picture a picture of the reference.

**Recorded traces.** What cannot run in the page is recorded once:

- the attention maps and activations of a small open model on fixed prompts (Default: a model of at most 200M parameters under an open licence, pinned by hash in `presets.yaml`);
- the training curves of an experiment (09), read from its `results.json`, with the confidence band over seeds;
- the steps of a long search (MCTS on a 9 by 9 Go position).

`aik viz record <slug>` runs the recording in the pinned image, writes the trace and `trace.bin` to `ai-site` under the trace's hash, and writes the hash into `recorded.yaml`. The page shows "Recorded on <date> with <model> <hash>" next to the scene. A recorded trace of an experiment is rebuilt when the experiment's `results.json` changes (rule V10).

## 12.8 In-browser notebooks

Many pages end with "Run it yourself". The reader can run the NumPy reference of the node, change it, and run small models, all in the browser.
There is no server execution in v1 (02.6, Q-09): code never runs on our machines on behalf of a reader.

**Python.** Pyodide, pinned in `registers/browser-runtimes.yaml`, with NumPy and the small pure-Python packages that the reference set of 08.4 allows.

- The notebook opens with the node's `impl/numpy/<name>.py` and the worked example of the page as cells.
- The build runs every cell in the same Pyodide version headless and records the outputs. The page shows the recorded output until the reader presses "Run", so a reader with no script sees the right output.
- A cell whose recorded output differs from the output of the NumPy reference in its image fails rule V11. The notebook and the tested code cannot drift apart.
- PyTorch and JAX do not run in Pyodide. Their tabs show the code and the output recorded by the X runner (08.9), with the label "recorded output".

**Small models.** ONNX Runtime Web with the WebGPU back end, and its WebAssembly back end when WebGPU is missing.

- Uses: a small tokenizer to try on any text; a sentiment classifier; a small image classifier on the reader's own image, which never leaves the browser; next-token probabilities of a small language model; a tiny diffusion sampler.
- Each model is an ONNX file exported by `aik viz export` from a pinned checkpoint with an open licence, stored in `ai-site` under its hash, with the licence and the source in the page.
- Limits (Default, Q-37): at most 50 MB per model by default, and up to 300 MB behind an explicit "Download 280 MB model" button. Models load only on that click and are cached by the service worker.
- Every model demo shows the model card: its name, size, source, licence and known limits, from its card (04.6).

**Isolation.** Notebook pages send COOP and COEP headers so that Pyodide and ONNX Runtime can use threads (16.18). Runtimes load from our own origin, never from a third-party CDN. A notebook has no network access to anything but the site's own assets: the CSP forbids `connect-src` beyond the site.

**State.** The reader's edits are kept in `localStorage` per node, with a "Reset" button. There is no account and no upload.

## 12.9 Charts of the registers

The registers hold dated facts about models, benchmarks, results, hardware, datasets, incidents and policies (04.9, 10.6).
Charts of them are generated by the build from `viz/charts.yaml` (on a node) or `topics/charts.yaml` (site-wide).
They are the only place where the site shows trends.

```yaml
# nodes/evaluation/benchmarks-and-their-design/swe-bench/viz/charts.yaml
- id: progress-verified
  type: progress                 # score of result cards against date
  benchmark: swe-bench@verified
  setting: {scaffold: any, attempts: 1}
  y: score
  population: "all result cards for this version and setting"
  frontier: true                 # draw the computed best-so-far step line (Q-38)
  color_by: access               # open weights, API, closed
- id: compute-vs-score
  type: scatter
  x: model.training.compute_flop
  y: score
  x_scale: log
```

The chart types:

| Type | Shows | Example |
|---|---|---|
| `progress` | each result card of one benchmark version and one setting, by date, with an optional best-so-far step line | MMLU 5-shot, ImageNet top-1, SWE-bench Verified |
| `trend` | a fact of model or hardware cards against date, on a log axis | training compute of notable models (Epoch AI's notable set and our scope rule, 18.5), parameters, hardware FLOP/s per dollar |
| `scatter` | two facts against each other | training compute against benchmark score; parameters against tokens |
| `fit` | a published or measured fit with its data points and its uncertainty | a scaling-law fit from a source, or from an experiment of 09 with its confidence band |
| `count` | counts per period | incidents per year by risk (AIID), works per year in an area (OpenAlex), notable models per country (the AI Index) |
| `timeline` | dated events and intervals | the application dates of the EU AI Act and the Digital Omnibus on AI; the releases of a model family |
| `table-chart` | a small multiple of bars or dots, one row per item | the facts of the models of one family |

Rules (rule numbers in 12.14):

- Every chart has a caption with its population rule, its source registers, and "Data as of <date>". The date is the newest `as_of` of the facts it uses (V15).
- Every point links to its card, and the card links to its source with the locator. A hover or focus shows the value, the source and the date.
- Every chart has its data table under it (shown with no script, and in the text view), and a download of the data as CSV and JSON from `ai-data` (16.10).
- Log axes say "log scale" and label powers of ten in words a reader knows (1e21 FLOP is written "10²¹ FLOP").
- Results of different settings, protocol versions or benchmark versions are never joined by one line (V16). The comparability rules of 10.6 decide what can share a line.
- A best-so-far line is labelled "best reported score in this setting as of <date>", never "state of the art" (K04).
- No extrapolation is drawn unless a cited source made it, and then it is dashed and labelled with the source.
- Facts with status `estimated` or `disputed` (04.9) are drawn with a hollow mark, and the legend says so.

## 12.10 Explorables

An explorable is a concept with controls and linked views and no steps (2150/10.6).
The first list, by area (each is a work item at M2 and M3):

| Area | Explorables |
|---|---|
| foundations and learning theory | bias and variance with polynomial degree; overfitting and the train and test curves; VC dimension of half-planes (shatter points); the PAC bound as a function of the sample size |
| optimization | gradient descent on 2-D surfaces with learning rate; momentum and Nesterov; Adam's bias correction; learning-rate warmup and cosine schedules |
| deep learning | activation functions and their gradients; initialization and the variance of activations through depth; batch and layer normalization; dropout; residual connections and gradient flow; double descent |
| architectures | convolution with stride, padding and dilation; receptive fields; attention as a soft lookup; positional encodings (sinusoidal, rotary); the causal mask; mixture-of-experts routing |
| language models | BPE training on the reader's text; softmax temperature, top-k and top-p; beam search against sampling; perplexity; the KV cache growing with context |
| generative models | the VAE latent space on 2-D data; GAN training on 2-D points; the diffusion forward and reverse processes; classifier-free guidance strength |
| evaluation | the confusion matrix and the threshold; ROC and precision-recall curves; calibration and reliability diagrams; pass@k from n samples; BLEU and its brevity penalty |
| RL | the Bellman backup; value iteration on a grid; exploration with epsilon-greedy and UCB on a bandit; discount factor and horizon |
| efficient AI | quantization error by bit width; pruning by magnitude; LoRA rank against fit |
| safety, fairness and interpretability | the incompatibility of calibration and equal error rates at different base rates; adversarial perturbation size against accuracy; reward hacking on a grid; superposition in a toy model |

## 12.11 Static frames, filmstrips and social images

As in algorithm (2150/10.7): `aik viz frames` renders the key frames of each scene (the first step of each phase and the last step) to SVG, headless in goja.
The filmstrip is what a reader with no script sees.
The social image of a page is its scene's chosen frame, or for a model or benchmark page, its main chart.
Architecture scenes render one frame per preset size.
Register charts render to SVG at build, so they are complete without script; the script adds hover and filtering only.

## 12.12 Accessibility

Algorithm's rules apply as they are (2150/10.8.1): a labelled region, an `aria-live="polite"` narration that speaks steps at slow speeds and only phase titles at high speeds, a text view on `t`, marks besides colours, reduced motion, no flashing above three times a second, visible focus, no focus trap.
AI adds:

- **Tensors and heatmaps in the text view** are HTML tables with row and column headers, and for large tensors a summary (shape, minimum, maximum, mean) with the slice that the step touched.
- **Charts** have a text summary written by the build from the data ("42 results from 2023-03 to 2026-10; the best reported score in this setting rose from 33.2 to 78.4"), the data table, and keyboard navigation from point to point.
- **Sliders** have a label, a range, a step, the current value as text, and keyboard control.
- **Images** that the reader uploads to a model demo stay local, and the demo says so.

## 12.13 Performance budgets

| Budget | Value |
|---|---|
| `@aik/viz` runtime, gzip | at most 90 KB (algorithm's is 60 KB; the tensor and chart primitives add the rest), loaded once and cached |
| one scene bundle, gzip | at most 20 KB |
| one trace with `trace.bin`, gzip | at most 300 KB for the default preset; recorded traces at most 2 MB |
| steps on any preset | at most 20,000 |
| training scene | at most 2 s to finish the default preset on the reference phone; progress is drawn as it trains |
| chart | at most 5,000 points drawn; more points are binned, and the data table and download keep all |
| first draw | at most 100 ms after the scene loads; scripts load only when the scene scrolls into view |
| Pyodide and ONNX Runtime | loaded only on "Run"; never counted in the page budget of 16.17 |
| layout shift | none: the page reserves the height of the scene from its filmstrip |

The reference phone is the one of algorithm (2150/14.13).

## 12.14 Checks

`aik viz check <slug>` runs these checks in CI for each changed scene, and the nightly build runs them for every scene.
They are the V rules of the audit (15.3).

| Rule | Check |
|---|---|
| V01 | the scene builds, imports only `@aik/viz`, and uses no DOM, network, timer or unseeded random |
| V02 | the trace is deterministic: the same input and seed give the same trace hash twice |
| V03 | the result agrees with the NumPy reference of the node (08) on each preset and on 20 random inputs within the limits: equal outputs for step scenes under the contract's tolerance; for training scenes, metrics within the tolerance of `presets.yaml` at every recorded step |
| V04 | each `line` id of the trace is a line id of the pseudocode or of the NumPy reference's `# @line` markers; each `eq` id is an equation id of the page |
| V05 | the presets together reach every line id |
| V06 | line-trace agreement with the NumPy reference, as algorithm's V06: the sequence of line ids of the scene equals the sequence that the reference runs on the same input, with repeats merged |
| V07 | shapes in the trace equal the shapes that `shapes.yaml` gives for the preset's sizes |
| V08 | the budgets of 12.13 hold |
| V09 | each phase has a note; notes are at most 140 characters; every state is in the vocabulary; the text view renders each step; every colour scale has a legend |
| V10 | presets are valid, at least 3 for step scenes; a recorded trace's hash matches its file, its source model or experiment is pinned, and it is newer than the experiment's `results.json` |
| V11 | the filmstrip, the social image and the notebook outputs render, and each notebook cell's recorded output equals the reference output |
| V12 | each control of an explorable has a label and a range, and each value of the range draws without error |
| V13 | the scene uses only the primitives of `@aik/viz` |
| V14 | the parameter and FLOP numbers that an architecture scene shows equal those of `shapes.yaml` and of the calculator (16.14) for the same sizes |
| V15 | every register chart has a population rule, sources and a "Data as of" date, and every point links to a card |
| V16 | no chart line joins results of different benchmark versions, settings or protocols that 10.6 marks as not comparable; estimated and disputed facts have hollow marks |

V03 and V06 are the checks that matter most for scenes, as in algorithm. V15 and V16 are the ones that matter most for charts.

## 12.15 The visualization ladder V0 to V3

The build computes one level for each node (04.10).

| Level | It needs |
|---|---|
| V0 | no scene, explorable, chart or figure |
| V1 | static figures in `figures/`, or a scene whose frames render (V11) but that fails a check needed for V2 |
| V2 | an interactive scene, explorable or chart set that passes V01 to V16 as they apply, with step back, custom input or hyperparameter controls, at least 3 presets, code sync in the NumPy tab, the text view and narration for each phase |
| V3 | V2, and a person who is not the author reviewed it for clarity and accessibility, with the checklist of 15.4, for this exact hash of the scene files and presets |

Which nodes need which level (the `viz: expected` default of `schema/targets.v1.yaml`):

| Kind | Needs, for a core node |
|---|---|
| `architecture` | V2 (the generated scene of 12.6 is enough) |
| `method` | V2 when it has a step or training form (dropout, attention, beam search, MCTS, PPO on a grid); V1 otherwise |
| `metric` | V2 for the metrics of the explorables list; V1 otherwise |
| `concept`, `theorem`, `finding` | V2 for the items of the explorables list (12.10); optional otherwise |
| `model`, `benchmark`, `hardware`, `policy`, `area` | V2 by register charts when the registers have at least 5 data points; otherwise none |
| `task` | V1: a figure of an input and its output |
| `argument`, `open-problem`, `person`, `organization`, `work`, `risk`, `definition` | none; a timeline is optional |

## 12.16 Authoring

The steps for a person or the visualizer role (14):

1. `aik new viz <slug>` writes `viz/scene.ts` and `viz/presets.yaml`. It picks the main primitive from the kind and the contract: an architecture gets `viz/shapes.scene.yaml` from `shapes.yaml`; a method with `impl/numpy/` gets a step scene with one `v.line` call per `# @line` marker of the reference.
2. Copy the steps of the NumPy reference into `run`, and add the events. Keep the structure of the reference, so that V06 passes.
3. `aik viz serve <slug>` opens the player on a local server with reload on save, a trace inspector, and the NumPy run next to it for comparison.
4. `aik viz check <slug>` runs the checks of 12.14. `aik viz frames <slug>` shows the filmstrip.
5. Open a pull request. The check prints the V level before and after, as for every ladder.

## 12.17 Example: the scene of scaled dot-product attention

The node `scaled-dot-product-attention` has a NumPy reference with markers:

```python
def attention(q, k, v, mask=None):          # @line L1
    d_k = q.shape[-1]
    s = q @ k.swapaxes(-1, -2) / np.sqrt(d_k)  # @line L2
    if mask is not None:
        s = np.where(mask, s, -np.inf)       # @line L3
    w = softmax(s, axis=-1)                  # @line L4
    return w @ v, w                          # @line L5
```

The scene, in outline:

```ts
import { scene } from "@aik/viz";

export default scene({
  slug: "scaled-dot-product-attention",
  input: { tokens: { length: [2, 12] }, d_k: [2, 16], mask: ["none", "causal"] },
  run(v, input) {
    const [Q, K, V] = v.fixtures.qkv(input);         // the seeded fixtures of 08.7
    v.line("L1", "Each query will look up all keys."); v.phase("scores");
    const S = v.op("matmul_scaled", [Q, K.T], "S", { eq: "eq-scores" });
    v.line("L2", "Scores are dot products, divided by the square root of d_k.");
    if (input.mask === "causal") { v.line("L3", "A query may not see later keys."); S.mask("causal"); }
    v.phase("weights");
    const W = v.op("softmax_rows", [S], "W", { eq: "eq-softmax" });
    v.line("L4", "Each row of weights sums to 1.");
    v.phase("mix");
    const O = v.op("matmul", [W, V], "O");
    v.line("L5", "Each output is a weighted mix of the values.");
    return O;
  },
});
```

The presets are `tiny` (4 tokens, d_k 4), `causal` (6 tokens, causal mask), and `sharp` (scores scaled up, so the reader sees softmax saturate and learns why the 1/√d_k factor is there; it links to the derivation `scaled-dot-product.md` of the Transformer node).
The heatmap of `W` is the `triangle` view under the causal mask.
V03 compares `O` and `W` with the NumPy reference for each preset; V06 checks the line order; V07 checks that `S` has shape `[T, T]`.
