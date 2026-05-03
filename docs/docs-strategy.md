# Documentation Strategy

dygo documentation lives in `/docs`.

Do not use GitHub Wiki for framework documentation.

## Why Repo Docs

Repository-based docs are the right default for dygo because they are:

- versioned with the code they describe
- reviewed in pull requests
- easier for coding agents to read
- able to evolve with implementation
- a better long-term path to a docs website

Docs should explain the framework clearly enough that a human or coding agent can make the next change without guessing the architecture.

## Future Direction

Keep the source docs in this repository.

Later, publish the same docs through VitePress or a similar static docs tool. The public site can live at `docs.dygo.dev` or `dygo.dev/docs`.

Do not add docs website tooling until the repo docs are useful on their own.
