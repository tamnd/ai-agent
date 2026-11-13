# 09. Experiments and reproduction

In algorithm, the evidence that an algorithm works is a proof and a test.
In AI, most of what the field knows is empirical: dropout reduces overfitting, warmup stabilizes Transformer training, loss falls as a power law in compute, a 2-layer attention-only model forms induction heads.
Such claims cannot be proved, and a unit test cannot show them.
They can be **measured**: a small training run, with several seeds, pinned data and a statistical test fixed in advance, either shows the qualitative claim at its scale or does not.
An experiment is the AI analogue of a proof. It is weaker than a proof and stronger than a citation, and the page says exactly how strong it is.

## 9.1 What an experiment is

- An experiment belongs to one node and tests one or more claims of its `claims.yaml` (07.3). It lives in `exp/<slug>/` in the node directory (04.4).
- It has five parts: `experiment.yaml` (the claim, the design, the config, the data pins, the seeds, the budget class), `run.py` (the script), `expect.yaml` (the statistical test that decides the claim, fixed before the first run), `results.json` (written only by `aik exp run -update`), and `figures/` (SVG plots made by the run).
- An experiment that passes its test gives the claim the evidence label `measured-here` at the experiment's scale (07.4). It never changes what the page says about the claim at the paper's scale.
- An experiment that fails its test is kept and shown. It is evidence that the claim does not hold at that scale, and the page says so (9.7). Failed experiments are never deleted to make a page look better (rule R10).
- `aik exp run` runs `run.py` once for each arm and each seed, in the order of the seeds. Each run gets a job directory with `job.json` (the arm, the seed and the merged configuration) and the command `python3 run.py --job <dir>/job.json --out <dir>`. The script writes `<dir>/metrics.json`, `{"final": {"<metric>": <value>, ...}, "device": "cpu"}`, with the final value of each metric of `metrics`.
- An experiment is not a test of the code. The learning smoke test of 08.7.6 checks that code can learn; an experiment checks a claim about the method.
- The build computes the level R0 to R4 of the node from its experiments (9.11).

## 9.2 Which claims have a small-scale form

A claim has a small-scale form when a version of it can be stated and tested within the `gpu-medium` budget (9.4), and that version is what the literature itself uses to explain or check the claim. The curate step marks each claim of `claims.yaml` with `small_scale: yes | no | partial` and a note, and the critic checks the mark.

| Claim type | Small-scale form | Example | Test |
|---|---|---|---|
| comparative: method A improves B under a condition | yes, when the original paper or a replication shows it on a small dataset | dropout lowers the test loss of an over-parameterized MLP on MNIST; batch normalization speeds up early training of a small CNN on CIFAR-10; label smoothing improves calibration | difference of means over seeds, Welch t-test, effect size, confidence interval |
| qualitative behaviour | yes, when a known small setting shows it | double descent in test error as width grows on a noisy subset of CIFAR-10; grokking on modular arithmetic; the lottery ticket effect on a small MLP | a shape test: a peak or a delayed jump exists, with a bootstrap confidence interval on its location |
| trend or scaling relation | partial: the slope at small scale, never the extrapolation | loss against parameters on TinyStories over two decades of size follows a power law with a fitted exponent; the compute-optimal ratio at small scale | a fit with a confidence interval on the exponent; the claim at small scale is "a power law fits with R² above 0.98 over this range", not "the exponent is 0.076" |
| mechanism | yes, when the mechanism was found in small models | induction heads form in a 2-layer attention-only Transformer; superposition in a toy model of features | a detection metric on the trained model over seeds |
| invariance or equality | yes | weight decay and L2 regularization are equivalent for SGD and not for Adam; two parameterizations give the same function | an equivalence test (TOST) with a stated margin |
| failure or risk demonstration | yes, in a toy environment | reward hacking in a small gridworld; goal misgeneralization in a procedurally generated maze; a backdoor implanted by poisoning 1% of a small training set | rate of the failure over seeds, with a confidence interval |
| frontier capability | no | a model of this size solves this share of a hard benchmark; an ability emerges at 100 billion parameters | none; the claim keeps its published evidence label (07) |
| claim about a proprietary model or dataset | no | the training data of a closed model; the behaviour of a closed model | none |

Rules:

- `experiment.yaml` has a `scale_note` that states the scale of the experiment and the scale of the source's claim in one sentence each (rule R03). The page prints it in the experiment box (9.7).
- A small-scale experiment is **never** presented as a reproduction of the paper's full result (rule R04). The words "reproduces the paper" are forbidden; the template says "The claim holds at this scale" or "The claim did not hold at this scale".
- An experiment that reruns a paper's own small-scale setting (the same dataset, model and protocol as a table of the paper) has `replicates: {work, locator}`, and then the page may say "This reruns the setting of Table 2 of the paper", with the numbers side by side.

## 9.3 Seeds and statistical tests

### 9.3.1 The design

`experiment.yaml`:

```yaml
# nodes/deep-learning-training/regularization/dropout/exp/mlp-mnist/experiment.yaml
id: mlp-mnist
claims: [c-dropout-reduces-overfitting]       # ids from claims.yaml (07.3)
question: "Does dropout lower the test loss of an over-parameterized MLP trained on a small subset of MNIST?"
scale_note:
  here: "A 2-hidden-layer MLP with 1,024 units per layer, 6,000 training images, 30 epochs."
  source: "Srivastava et al. (2014) report it on full MNIST with larger nets and max-norm constraints."
replicates: null
arms:                                          # the conditions compared
  - {id: no-dropout, config: {p: 0.0}}
  - {id: dropout, config: {p: 0.5}}
base_config:
  model: {hidden: [1024, 1024], activation: relu}
  optimizer: {name: sgd, lr: 0.05, momentum: 0.9}
  epochs: 30
  batch_size: 128
data:
  - {dataset: mnist, version: "lecun-1998", subset: "train[:6000], test", pin: "S:mnist-ai-data", hash: "blake3:2f9a…"}
seeds: {n: 10, base: 20261113}                # seeds 20261113 .. 20261122, the same for every arm
budget: cpu-small
image: aik-exp-pytorch-cpu
framework: pytorch
uses_impl: ../../impl/pytorch/dropout.py       # the node's own reference implementation
metrics: [test_loss, test_accuracy, train_loss]
```

Rules:

- **Seeds.** Every arm runs with the same seeds. The default count is 10 for `cpu-small`, 5 for `gpu-small` and 3 for `gpu-medium` (Q-33). A test whose power, computed by `aik exp power` from a pilot run, is under 0.8 for the effect size of the claim needs more seeds or a smaller claim (rule R07).
- **The node's own code.** An experiment uses the node's reference implementation of the method (`uses_impl`), so that the experiment also tests the code. It may use a library version for the rest of the model.
- **Arms are complete.** A comparative claim has a control arm. A trend claim has at least 5 points of the variable on a log grid.
- **Data and hyperparameters are fixed in the file.** Tuning is part of the design: an experiment that tunes hyperparameters per arm names the search space and the budget per arm in `tuning:`, and the same budget applies to every arm (rule R08).

### 9.3.2 The test

`expect.yaml` states the test that decides the claim. It is committed, and its hash recorded in `prov`, **before** the first full run (rule R06). This is a pre-registration: a change to `expect.yaml` after results exist resets the node's R level for this experiment to R1 until a person reviews the change.

```yaml
# exp/mlp-mnist/expect.yaml
claim: c-dropout-reduces-overfitting
primary:
  metric: test_loss
  at: final                                   # final | best | step:<n> | curve
  compare: {treatment: dropout, control: no-dropout}
  direction: lower
  test: welch-t                               # welch-t | mann-whitney | bootstrap-diff | tost | fit | shape
  alpha: 0.01
  min_effect: {kind: relative, value: 0.05}   # dropout must lower test loss by at least 5% of the control mean
secondary:                                    # reported, not decisive
  - {metric: test_accuracy, direction: higher, test: bootstrap-diff}
  - {metric: train_loss, direction: higher, test: bootstrap-diff, note: "dropout should fit the training set less"}
correction: holm                              # across the primary tests of this file, when there are several
report: {ci: 0.95, bootstrap: {resamples: 10000, method: bca}}
```

| Test | Used for | Passes when |
|---|---|---|
| `welch-t` | difference of two means over seeds | p < alpha in the claimed direction, and the effect is at least `min_effect` |
| `mann-whitney` | the same, when the metric is skewed (rates, counts) | as above, on the Hodges-Lehmann shift |
| `bootstrap-diff` | any difference, reported with a confidence interval | the BCa interval excludes zero in the claimed direction |
| `tost` | equivalence ("A and B are the same within a margin") | both one-sided tests pass at alpha |
| `fit` | trends: a power law, a linear relation, an exponent | the fit meets `r2_min`, and the confidence interval of the parameter lies inside the claimed range |
| `shape` | qualitative behaviour: a peak, a delayed jump, a crossing | the feature is found by its detector in at least `min_share` of seeds, with a bootstrap interval on its location |
| `rate` | failure and risk demonstrations | the lower bound of the Wilson interval of the rate is above the claimed rate |

The statistics are computed by the package `exp` of `aik`, in Go, from the per-seed metrics, so that the check and the build need no image. The tests of the package compare its numbers with SciPy. The same code computes every page's numbers. In M0 a metric is read at `final` only, and the test `shape` is not built yet: an experiment that uses it stays inconclusive.

## 9.4 Budget classes and the GPU pool

### 9.4.1 The classes

| Class | Hardware | Cap per experiment (all arms, all seeds) | Where it runs | When |
|---|---|---|---|---|
| `cpu-small` | 2 CPU cores, 4 GiB memory | 10 minutes of wall time | GitHub Actions and the build server, in the sandbox of 08.9 | in the pull request (F2), and nightly when inputs change |
| `gpu-small` | one GPU with 24 GB of memory (L4, A10G or a similar class), 8 CPU cores, 32 GiB memory | 2 GPU-hours | the GPU pool | in the weekly batch of F6 (03.5) |
| `gpu-medium` | one GPU with 80 GB of memory (A100 80GB, H100 or a similar class), 16 CPU cores, 128 GiB memory | 8 GPU-hours | the GPU pool | in the weekly batch, with a person's approval for each new experiment |

Rules:

- The class is part of the design. An experiment that exceeds its cap is stopped and fails (rule R09); it cannot be moved to a larger class without a new review.
- Multi-GPU and multi-node experiments are out of scope in v1. A claim that needs them keeps its published evidence label (Q-32).
- The class caps are hard limits for one run of the experiment. The monthly cap (9.10) limits the sum.
- The hardware class, the actual device name, the driver, CUDA, cuDNN or ROCm versions, and the framework flags are recorded in `results.json` (9.6).

### 9.4.2 The GPU pool

- `aik exp pool` starts on-demand GPU machines with the pinned image, runs a batch of experiments, uploads outputs to `ai-runs`, and stops the machines. The provider adapter is in `registers/compute.yaml`, with the instance types for each class and their hourly price (Default: one on-demand cloud provider, revisited at M3, Q-32).
- Experiments are packed into batches by class. A batch is the unit of approval and cost: `aik exp plan` prints the batch with its estimated GPU-hours and price, and the batch runs only when the month's remaining budget covers it.
- Spot or preemptible machines may be used when `run.py` checkpoints at least every 15 minutes; `aik exp run` resumes from the last checkpoint.
- The pool machines have network access only in the **prepare** step, which fetches the pinned image and the pinned data by hash (9.5). The **run** step has no network, as in the sandbox.

## 9.5 Data pins

Every dataset that an experiment reads is pinned:

```yaml
# registers/data-pins.yaml (part)
- id: mnist-ai-data
  dataset: mnist                       # the dataset card or node
  version: "lecun-1998"
  files:
    - {name: train-images-idx3-ubyte.gz, blake3: "…", bytes: 9912422}
    - {name: train-labels-idx1-ubyte.gz, blake3: "…", bytes: 28881}
  origin: "the original distribution URL, recorded in the raw store"
  licence: {spdx: CC-BY-SA-3.0, source: "S:mnist-licence-note"}
  mirror: allowed                      # allowed | not-allowed | unknown
  location: "r2://ai-data/pins/mnist-ai-data/"
```

Rules:

- **Mirroring.** A dataset is copied to `ai-data` only when its licence allows redistribution (`mirror: allowed`), checked by the licence rules of 19.8 (rule C10). Otherwise the prepare step fetches it from its origin and checks the hash. A fetched copy lives in the pool's private cache and is never published. A dataset whose licence is `unknown` is not used until the licence is resolved.
- **Hash first.** A run whose input files do not match the pinned hashes fails before training (rule R02).
- **Preferred data.** Experiments prefer small, openly licensed, well-documented data: synthetic generators in the node (a two-moons generator, modular arithmetic, a copy task, a toy model of features), MNIST, Fashion-MNIST, CIFAR-10 and CIFAR-100, Tiny Shakespeare, TinyStories, the OpenML-CC18 suite, WikiText-103, small Gymnasium and MinAtar environments, and small subsets of larger corpora under their licences. `registers/data-pins.yaml` lists them, and the curate step chooses from it first.
- **Subsets.** A subset is defined by a deterministic rule in the pin (`train[:6000]`, or a seeded sample with the seed), never by a file that the experiment writes.
- **Environments.** RL environments are pinned by package version and environment id (`gymnasium==<v>`, `CartPole-v1`), and their versions are part of the image.

## 9.6 Results in git

`results.json` is the summary that the site uses. It is written only by `aik exp run -update` from the outputs in `ai-runs`, never by hand or by an agent's text edit (rule R05). It is signed by the run id, and `prov/<tag>.jsonl` records the run.

```json
{"experiment":"mlp-mnist","node":"3KD9QX","run":"r-2026-12-04-7f3c",
 "date":"2026-12-04","image":"ghcr.io/tamnd/aik-exp-pytorch-cpu@sha256:…",
 "hardware":{"class":"cpu-small","cpu":"AMD EPYC 7763","threads":2},
 "inputs_hash":"sha256:…","expect_hash":"sha256:…",
 "seeds":[20261113,20261114,20261115,20261116,20261117,20261118,20261119,20261120,20261121,20261122],
 "arms":{"no-dropout":{"test_loss":{"mean":0.1532,"sd":0.0081,"per_seed":[…]}},
         "dropout":{"test_loss":{"mean":0.0958,"sd":0.0046,"per_seed":[…]}}},
 "primary":{"metric":"test_loss","test":"welch-t","p":1.2e-9,"effect_relative":-0.375,
            "ci95":[-0.0639,-0.0509],"passed":true},
 "secondary":[…],
 "figures":["figures/test-loss-by-epoch.svg"],
 "wall_time_s":512,"cost_usd":0.0}
```

The numbers above illustrate the format; the run writes the real ones.

- The page renders every number of the experiment box from `results.json` (rule R11).
- `aik exp verify` checks that `results.json` matches the outputs in `ai-runs` for the recorded run, while they are retained (9.9), and that its `inputs_hash` and `expect_hash` match the current files. A stale `results.json` (inputs changed since the run) drops the experiment to R1 until it is rerun.

## 9.7 How a page shows an experiment

The experiment box (16.4) on a node page:

- the claim, as the page states it, with its claim id;
- the scale note: "Here: … Source: …" (9.2);
- the result: the primary metric for each arm with its mean and 95% confidence interval, the test, the p-value or the interval, and the effect size, in one table;
- the verdict, from the template only: "The claim holds at this scale.", "The claim did not hold at this scale.", or "Inconclusive at this scale (the test had too little power).";
- the figure;
- links: the code, `experiment.yaml`, `expect.yaml`, the run record in `ai-runs` while it exists, and a "Run it yourself" command (`aik exp run nodes/…/exp/mlp-mnist`);
- the line "This is a small-scale check, not a reproduction of the paper's result." unless `replicates` is set (rule R04).

A failed or inconclusive experiment is shown the same way, and the claim's evidence label does not get `measured-here` from it. When the claim is stated at the experiment's scale (a claim that a paper made about small models), the failure adds `evidence_against` to the claim (07.4).

## 9.8 Determinism and hardware

- GPU training is not bit-for-bit reproducible across hardware, drivers and library versions, even with fixed seeds. The checks of this document are statistical (9.3), so a rerun on other hardware should give the same verdict, not the same numbers.
- `run.py` sets the seeds of Python, NumPy and the framework, and sets the deterministic flags of the framework where they exist (`torch.use_deterministic_algorithms(True)`, the XLA determinism flags), unless `experiment.yaml` says `deterministic: false` with a reason (some kernels have no deterministic version).
- The nightly job reruns a sample of passed `cpu-small` experiments, and the monthly batch reruns a sample of 5% of passed GPU experiments. A verdict that changes on rerun is a finding (rule R12): the experiment is marked flaky, counts as failed, and gets an issue, as flaky tests do in algorithm.

## 9.9 Retention of `ai-runs`

| Output | Kept |
|---|---|
| metrics per step and per seed (JSONL), logs, configs, environment records | forever (small) |
| figures | forever |
| checkpoints of the final models | 90 days, then deleted unless the experiment is pinned (`keep_checkpoints: true`, used by scenes and notebooks that load a trained toy model, at most 200 MB per experiment) |
| intermediate checkpoints | until the run ends successfully |
| outputs of superseded runs | the last two runs per experiment are kept; older runs keep only metrics and logs |

The total size of `ai-runs` is reported monthly in `reports/compute/`. When it passes 1 TB, the oldest unpinned checkpoints are deleted first. The site never depends on `ai-runs` (03.3): everything that a page shows is in `results.json` and `figures/` in git.

## 9.10 The compute budget

| Milestone | Monthly cap (Default) | What it buys (Estimate) |
|---|---|---|
| M0 to M1 | $100 | `cpu-small` only in CI (free), plus a pilot batch of 50 `gpu-small` experiments |
| M2 | $250 | about 150 `gpu-small` experiments per month |
| M3 onwards | **$400** | about 200 `gpu-small` and 15 `gpu-medium` experiments per month, plus reruns |

- The cap lives in `registers/compute.yaml`. Raising it is a pull request that a person approves.
- `reports/compute/ledger.jsonl` records every batch: experiments, GPU-hours, price, the class, and the run ids. `aik exp plan` refuses a batch that would pass the month's cap.
- Priority order inside the cap: core nodes first (5.10), then claims marked `small_scale: yes` with no experiment yet, then experiments whose inputs changed, then reruns for R3, then the rest.
- At about $1 per `gpu-small` experiment and $10 per `gpu-medium` experiment (Estimate), 2,500 experiments by M5 (02.4) cost about $5,000 to $8,000 in total, most of them `cpu-small` and `gpu-small`.

## 9.11 The reproduction ladder R0 to R4

The build computes one level per node from its experiments (04.10).

| Level | It needs | Rules |
|---|---|---|
| R0 | no experiment; the page shows the evidence labels of the published results only | |
| R1 | an experiment exists, its `expect.yaml` was committed before the first full run, its data is pinned, it ran within its budget class, and `results.json` records seeds and confidence intervals | R01, R02, R05, R06, R09 |
| R2 | R1, and the primary test of `expect.yaml` passes: the claim holds at the experiment's scale | R03, R04, R07, R08 |
| R3 | R2 at a second scale or on a second dataset or environment named in the claim, so that the result is not a single point; both pass | R13 |
| R4 | R3, and a person reviewed the design (`experiment.yaml`, `expect.yaml`, `run.py`) and this exact results hash (15.5) | |

A node with several claims takes, for each claim, the best experiment; the node's R level is the minimum over the claims marked `small_scale: yes` (rule R14). The target for methods, architectures and findings whose main claim has a small-scale form is R2 (04.10).

## 9.12 Who writes experiments

The experimenter role (14.2) writes `experiment.yaml`, `expect.yaml` and `run.py` from the page's claims and the source's setting. It runs a pilot of one seed per arm in `cpu-small` or a short `gpu-small` run, computes the power, and fixes `expect.yaml` before the full run. A model of another family audits the design (15.4): is the control fair, is the tuning budget equal, does the scale note say the truth, could the test pass for a reason other than the claim. A person approves each new `gpu-medium` experiment and reviews core experiments for R4.

## 9.13 The rules of this document

The rules are listed in 15.3 with their severities. In short:

| Rule | Checks |
|---|---|
| R01 | `experiment.yaml` is valid, names claims that exist in `claims.yaml`, and has a control arm for comparative claims |
| R02 | every input matches its pinned hash; every dataset is in `registers/data-pins.yaml` with a resolved licence |
| R03 | `scale_note` states the experiment's scale and the source's scale |
| R04 | no page text presents a small-scale experiment as a reproduction of the paper's result; the small-scale line is present unless `replicates` is set |
| R05 | `results.json` is written only by `aik exp run -update` and matches the run record |
| R06 | `expect.yaml` was committed before the first full run; a later change resets the experiment to R1 until reviewed |
| R07 | the test's power is at least 0.8 for the claimed effect size, from the pilot |
| R08 | every arm has the same tuning budget and the same seeds |
| R09 | the run stayed within its budget class cap |
| R10 | a failed or inconclusive experiment is not deleted or hidden; it is shown with its verdict |
| R11 | every number of the experiment box is rendered from `results.json` |
| R12 | a verdict that changes on rerun marks the experiment flaky and failed |
| R13 | at R3, the second scale or dataset is named in the claim and passes |
| R14 | the node's R level is the minimum over its small-scale claims |
