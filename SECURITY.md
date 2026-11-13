# Security

## Report a problem

Do not open a public issue for a security problem. Use [private vulnerability reporting](https://github.com/tamnd/ai-agent/security/advisories/new).

Tell us the version or commit, the steps to cause the problem and what an attacker can do with it. We reply in 7 days. We fix a confirmed problem in 30 days when we can, and we credit you in the advisory if you agree.

## Supported versions

We fix security problems in the latest release only.

## Scope

These are in scope:

- `aik` reads a file outside the content tree, or writes a file outside its output directory.
- `aik site build` makes a page that runs script from the content.
- A scene check escapes the JavaScript runtime.
- A problem in the release workflow or the build provenance.

`aik test` runs code from the content tree. This is its purpose. It is not a security problem that a test can do what a program can do. On a computer, `aik test` runs only with the flag `-unsafe-local`.
