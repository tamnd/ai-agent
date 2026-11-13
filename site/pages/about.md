## What this site is

This site is an open encyclopedia of artificial intelligence. It covers each idea, method, architecture, model, task, benchmark, dataset, result, system, risk, law, argument, person and event of AI. An item with a lot to say gets a page. Each other item gets a card with checked facts and sources.

A full page about a method has these parts:

- An explanation in plain words.
- The setting and the method, with the equations.
- A derivation of each equation, or a citation of the exact equation in its source.
- A worked example with numbers.
- The hyperparameters and the cost.
- A reference implementation in NumPy, with PyTorch and JAX versions that must agree with it.
- An experiment that reproduces the claim at small scale, when that is cheap.
- The evidence for each claim, with its source and its date.

## How the site labels a claim

A sentence that states a fact about AI is a claim. The build gives each claim one of five labels:

- **proved**: a theorem, with a proof on the page or in Lean.
- **measured here**: an experiment in this repository reproduces it.
- **replicated**: independent published replications exist.
- **reported**: one source says it.
- **contested**: the sources disagree, and the page gives each position with its sources.

Nobody writes a label by hand. A number that changes, such as a parameter count or a benchmark score, has a source and an "as of" date. The text never says "state of the art" or "currently". Those words come only from computed tables with their dates.

## How the pages are written

We write in a controlled form of English. The sentences are short. Each sentence has one idea. We use the active voice and simple words. This makes the text easy to read for people who learn English as a second language. It also makes the text easy to translate.

A page says what it does not know. If a proof is only a sketch, the page says so. If the tests do not run, the page says so.

## How the build checks a page

The build reads every page and checks it against the content standard. It checks the links, the front matter, the order of the sections, the claims and their sources. It runs the tests of each reference implementation in a pinned image with no network. It runs each scene on its presets and checks that the trace is the same each time.

The build then computes the levels of each page. Nobody writes a level by hand. The section "About this page" at the end of each page shows the levels and what the next level needs. Read [the badges page](/badges/) for the details.

## Who makes it

The site is an open project. The content is in [tamnd/ai](https://github.com/tamnd/ai). The toolchain, `aik`, is in [tamnd/ai-agent](https://github.com/tamnd/ai-agent). Each page lists its authors in its provenance file.

## Licences

The text and the pictures are CC BY-SA 4.0. The code of the implementations and the scenes is MIT. The Lean proofs and the toolchain are Apache 2.0. The facts and the registers are CC0. The fonts and the KaTeX files keep their own licences.

## Contact

To report a mistake, open an issue in the content repository. To report a security problem, use the private report form of GitHub. Do not open a public issue for a security problem.
