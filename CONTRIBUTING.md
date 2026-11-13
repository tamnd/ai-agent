# Contributing

Thank you for your help. This file tells you how to change the toolchain. To change the content, read CONTRIBUTING.md in [tamnd/ai](https://github.com/tamnd/ai).

## Before you start

1. Find an issue, or open one. For a large change, write your plan in the issue first.
2. Read the part of the [specification](https://ai.tamnd.com/spec/) that your change touches. The code follows the specification. If the code and the specification do not agree, change both in the same pull request.

## Make a change

1. Fork the repository and make a branch from `main`.
2. Make the change. Add a test for each bug fix and each new rule.
3. Run `make test` and `make lint`.
4. If your change touches the site, run `make site` and look at the pages.
5. Open a pull request. Fill in the template.

## Style

- Format the code with `gofmt`.
- Write comments, messages, documents and commit messages in plain English. Use short sentences and the active voice. We follow ASD-STE100 where we can.
- A commit message has a short first line in the imperative mood, for example "Add the X01 link rule".
- Error messages say what is wrong and what to do.

## Sign-off

Each commit must have a `Signed-off-by` line (the [Developer Certificate of Origin](https://developercertificate.org)). Use `git commit -s`. With this line, you state that you have the right to give the change under the licence of the repository.

## Licence

You give your change under the Apache License 2.0. A change to `viz/packages/core` is also under the MIT licence.
