# dygo Documentation

dygo is an opinionated Go framework for building serious business software.

These docs describe the framework concepts, file formats, CLI commands, and runtime behavior.

## Start Here

- [Doctrine](doctrine.md) explains the framework principles.
- [Installation](installation.md) explains release binaries, installer scripts, and `dygo upgrade`.
- [CLI](cli.md) documents the dygo command surface.
- [Nomenclature](nomenclature.md) defines the core vocabulary used across the framework.
- [App Model](app-model.md) explains built-in apps, business apps, and app install locations.
- [App Manifest](app-manifest.md) defines the first `app.yml` schema.
- [Entity Metadata](entity-metadata.md) defines the Entity YAML schema, app-scoped identity, route slugs, Record naming, storage naming, field types, indexes, and constraints.
- [Metadata Authoring](metadata-authoring.md) explains JSON Schemas and editor support for dygo YAML files.
- [Config](config.md) explains required project config and current runtime settings.
- [Database](database.md) explains PostgreSQL config, secrets, metadata schema plans, schema sync, explicit schema prune, schema snapshots, and `dygo db check`.
- [Explicit Patches](patches.md) explains how unsafe metadata changes are handled without reintroducing SQL migrations, and where patches differ from schema prune.
- [Fixtures](fixtures.md) explains app-owned seed Records and the `dygo fixture apply` command.
- [Record Hooks](record-hooks.md) explains compiled app Record hook registration and the `entities/<entity>/hooks.go` convention.
- [Server](server.md) explains `dygo serve`, the health endpoint, and runtime APIs.
- [Auth](auth.md) explains Administrator bootstrap, session login, and authenticated API identity.
- [Records](records.md) explains the first metadata-powered Record CRUD API.
- [App SDK](sdk.md) explains the public Go package app code can compile against.
- [Studio](studio.md) explains the first-party global UI app.
- [Encrypted Secrets](secrets.md) explains repo-stored encrypted secrets and the `dygo secret` CLI.
- [Security Policy](../SECURITY.md) explains private vulnerability reporting and safe research guidelines.

## Working Notes

- [Notes](notes.md) stores internal planning and reduction notes that are not framework reference docs.
- [Project README](../README.md) gives a short overview and basic development commands.
- [Contributing](../CONTRIBUTING.md) explains the current paused contribution status.
- [Security Policy](../SECURITY.md) explains how to report vulnerabilities privately.
- [Agent Instructions](../AGENT.md) stores repo-level guidance for coding agents.
