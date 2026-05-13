# dygo Documentation

dygo is an opinionated Go framework for building serious business software.

It is built for business processes, internal operating systems, enterprise applications, and workflow-heavy products where permissions, audits, observability, metadata-driven schema sync, secure configuration, apps, schema-driven entities, jobs, and a consistent Studio UI matter from the beginning.

These docs live in the repository so they are versioned with code, reviewed in PRs, and easy for coding agents to read.

## Start Here

- [The dygo Doctrine](doctrine.md) explains the beliefs behind the framework.
- [Platform Thesis](platform-thesis.md) explains why dygo exists and what it should make possible.
- [Installation](installation.md) explains release binaries, installer scripts, and `dygo upgrade`.
- [Nomenclature](nomenclature.md) defines the core vocabulary used across the framework.
- [App Model](app-model.md) explains built-in apps, business apps, and app install locations.
- [App Manifest](app-manifest.md) defines the first `app.yml` schema.
- [Entity Metadata](entity-metadata.md) defines the Entity YAML schema, app-scoped identity, route slugs, Record naming, storage naming, field types, indexes, and constraints.
- [Metadata Authoring](metadata-authoring.md) explains JSON Schemas and editor support for dygo YAML files.
- [Config](config.md) explains required project config and current runtime settings.
- [Database](database.md) explains PostgreSQL config, secrets, metadata schema plans, schema sync, explicit schema prune, schema snapshots, and `dygo db check`.
- [Explicit Patches](patches.md) explains how unsafe metadata changes are handled without reintroducing SQL migrations, and where patches differ from schema prune.
- [Fixtures](fixtures.md) explains app-owned seed Records and the `dygo fixtures apply` command.
- [Record Hooks](record-hooks.md) explains compiled app Record hook registration and the `hooks/<entity>.go` convention.
- [Server](server.md) explains `dygo serve`, the health endpoint, and runtime APIs.
- [Auth](auth.md) explains Administrator bootstrap, session login, and authenticated API identity.
- [Records](records.md) explains the first metadata-powered Record CRUD API.
- [Studio](studio.md) explains the first-party global UI app.
- [Encrypted Secrets](secrets.md) explains repo-stored encrypted secrets and the `dygo secrets` CLI.
- [Directory Structure](dir-structure.md) describes the intended repository layout.
- [Documentation Strategy](docs-strategy.md) explains why docs live in `/docs` instead of GitHub Wiki.

## Working Notes

- [Project README](../README.md) gives a short overview and basic development commands.
- [Contributing](../CONTRIBUTING.md) explains the current paused contribution status.
- [Agent Instructions](../AGENT.md) stores repo-level guidance for coding agents.
