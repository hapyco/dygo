# dygo Documentation

dygo is an opinionated Go framework for building serious business software.

It is built for business processes, internal operating systems, enterprise applications, and workflow-heavy products where permissions, audits, observability, migrations, secure configuration, apps, schema-driven entities, jobs, and a consistent Studio UI matter from the beginning.

These docs live in the repository so they are versioned with code, reviewed in PRs, and easy for coding agents to read.

## Start Here

- [The dygo Doctrine](doctrine.md) explains the beliefs behind the framework.
- [Platform Thesis](platform-thesis.md) explains why dygo exists and what it should make possible.
- [Nomenclature](nomenclature.md) defines the core vocabulary used across the framework.
- [App Model](app-model.md) explains built-in apps, business apps, and app install locations.
- [App Manifest](app-manifest.md) defines the first `app.yml` schema.
- [Entity Metadata](entity-metadata.md) defines the first Entity YAML schema and built-in field types.
- [Config](config.md) explains required project config and current runtime settings.
- [Database](database.md) explains PostgreSQL config, secrets, and `dygo db check`.
- [Server](server.md) explains `dygo serve` and the health endpoint.
- [Studio](studio.md) explains the first-party global UI app.
- [Encrypted Secrets](secrets.md) explains repo-stored encrypted secrets and the `dygo secrets` CLI.
- [Directory Structure](dir-structure.md) describes the intended repository layout.
- [Documentation Strategy](docs-strategy.md) explains why docs live in `/docs` instead of GitHub Wiki.

## Working Notes

- [Project README](../README.md) gives a short overview and basic development commands.
- [Contributing](../CONTRIBUTING.md) explains the current paused contribution status.
- [Agent Instructions](../AGENT.md) stores repo-level guidance for coding agents.
