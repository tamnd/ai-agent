## What this site is

This site explains algorithms and data structures. Each page is about one idea: an algorithm, a data structure, a problem, a technique, a concept or a theorem.

A full page about an algorithm has these parts:

- An explanation in plain words.
- The pseudocode, with a number for each line.
- A proof that the algorithm is correct, and a proof of its cost.
- A formal proof in Lean, when one exists.
- Code in many languages. Each implementation has native tests.
- A scene that shows the algorithm step by step on an input that you choose.

## How the pages are written

We write in a controlled form of English. The sentences are short. Each sentence has one idea. We use the active voice and simple words. This makes the text easy to read for people who learn English as a second language. It also makes the text easy to translate.

A page says what it does not know. If a proof is only a sketch, the page says so. If the tests do not run, the page says so.

## How the build checks a page

The build reads every page and checks it against the content standard. It checks the links, the front matter, the order of the sections, the pseudocode line ids and the markers in the code. It runs the native tests of each language in a pinned toolchain. It runs each scene on its presets and checks that the trace is the same each time.

The build then computes the levels of each page. Nobody writes a level by hand. The section "About this page" at the end of each page shows the levels and what the next level needs. Read [the badges page](/badges/) for the details.

## Who makes it

The site is an open project. The content is in [tamnd/ai](https://github.com/tamnd/ai). The toolchain, `aik`, is in [tamnd/ai-agent](https://github.com/tamnd/ai-agent). Each page lists its authors in its provenance file.

## Licences

The text and the pictures are CC BY-SA 4.0. The code of the implementations and the scenes is MIT. The toolchain is Apache 2.0. The fonts and the KaTeX files keep their own licences.

## Contact

To report a mistake, open an issue in the content repository. To report a security problem, use the private report form of GitHub. Do not open a public issue for a security problem.
