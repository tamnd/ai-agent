## Before you start

All work happens on GitHub. You need a GitHub account, Git and Go. To run the tests of a language, you need its toolchain, or Docker.

Install the toolchain:

```sh
go install github.com/tamnd/ai-agent/cmd/aik@latest
```

Clone the content and check it:

```sh
git clone https://github.com/tamnd/ai
cd algorithm
aik check
aik site serve
```

`aik site serve` builds the site and serves it at http://localhost:8150.

## Write a new page

1. Find the page in the coverage plan, or open an issue that proposes it.
2. Run `aik new algorithm my-algorithm`. This makes the directory and the front matter.
3. Write the sections in the order that the content standard gives.
4. Run `aik check`. Fix each finding.
5. Open a pull request. Use one pull request for each page.

## Add an implementation

Each implementation is one file in `impl/<language>/` with a native test file next to it. The code follows the pseudocode of the page. Put a comment `@line L4` on the line of code that does the step L4.

Run the tests:

```sh
aik test my-algorithm
```

## Add a scene

A scene is `viz/scene.ts`. It imports only `@aik/viz`. It must give the same trace for the same input. Put at least 3 presets in `viz/presets.yaml`. Each line id of the pseudocode must be reached by at least one preset.

```sh
aik viz check my-algorithm
```

## Review a proof

A review makes a proof go from H2 to H3. Read the proof line by line. If it is correct, add a file `reviews/<tag>/proof-<id>.yaml` with the hash of the file. If the proof changes later, the hash does not match, and the level goes back to H2.

## Writing rules

- Write short sentences. Use one idea in each sentence.
- Use the active voice.
- Use simple words. Define each technical term before you use it.
- Do not put a line break in the middle of a sentence.
- Do not copy text from a book. Write it in your own words and cite the source.
