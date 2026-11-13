## Before you start

All work happens on GitHub. You need a GitHub account, Git and Go. To run the tests of an implementation, you need Docker or Podman.

Install the toolchain:

```sh
go install github.com/tamnd/ai-agent/cmd/aik@latest
```

Clone the content and check it:

```sh
git clone https://github.com/tamnd/ai
cd ai
aik check
aik site serve
```

`aik site serve` builds the site and serves it at http://localhost:8150.

## Write a new page

1. Find the page in the coverage plan, or open an issue that proposes it.
2. Run `aik new method my-method -area <top/sub>`. This makes the directory, the front matter and the sections of the kind.
3. Write the sections in the order that the content standard gives.
4. Run `aik check`. Fix each finding.
5. Open a pull request. Use one pull request for each page.

## Add an implementation

Each implementation is in `impl/<framework>/` with its tests next to it. Write the NumPy version first, from first principles. The PyTorch and JAX versions must give the same outputs as the NumPy version, within the tolerance of the node. Put a comment `eq: <id>` on each block of code that computes an equation of the page.

Run the tests:

```sh
aik test my-method
```

## Add a scene

A scene is `viz/scene.ts`. It imports only `@aik/viz`. It must give the same trace for the same input. Put at least 3 presets in `viz/presets.yaml`. Each step of the method must be reached by at least one preset.

```sh
aik viz check my-method
```

## Review a proof

A review makes a proof go from H2 to H3. Read the proof line by line. If it is correct, add a file `reviews/<tag>/proof-<id>.yaml` with the hash of the file. If the proof changes later, the hash does not match, and the level goes back to H2.

## Writing rules

- Write short sentences. Use one idea in each sentence.
- Use the active voice.
- Use simple words. Define each technical term before you use it.
- Do not put a line break in the middle of a sentence.
- Give each claim a source. Do not write "state of the art", "the best model" or "currently".
- On a contested question, give each position with its sources. Do not give the opinion of the site.
- Do not copy text from a paper or a book. Write it in your own words and cite the source.
