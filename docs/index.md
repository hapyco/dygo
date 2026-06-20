# dygo Documentation

dygo is an opinionated Go framework for building metadata-driven business software.

Use this index to find the right document for the task. The docs are kept in the repository so behavior, CLI commands, and file formats can be reviewed with the code that implements them.

## Start Here

- [Installation](installation.md) explains release binaries, installer scripts, and project upgrades.
- [CLI](cli.md) is the command reference for project, database, metadata, server, Job, worker, and secret commands.
- [Doctrine](doctrine.md) explains the framework principles behind dygo's defaults.
- [Nomenclature](nomenclature.md) defines terms used across code, docs, CLI output, metadata, and Studio.

## Build Apps

- [App Model](app-model.md) explains built-in apps, business apps, and app install locations.
- [App Manifest](app-manifest.md) defines `app.yml`.
- [Entity Metadata](entity-metadata.md) defines Entity YAML, route slugs, Record naming, storage naming, field types, indexes, and constraints.
- [Access](access.md) records the proposed app role and Entity access authoring model.
- [Workflow](workflow.md) records the proposed model for lifecycle actions, status transitions, approvals, and document-style operations.
- [Metadata Authoring](metadata-authoring.md) explains JSON Schemas and editor support for dygo YAML files.
- [Fixtures](fixtures.md) explains app-owned seed Records and `dygo fixture apply`.
- [Record Hooks](record-hooks.md) explains compiled app Record hooks and the `entities/<entity>/hooks.go` convention.
- [App SDK](sdk.md) explains the Go package app code uses for hooks, Jobs, Record access, and Logs.

## Run The Platform

- [Config](config.md) explains `dygo.yml`, queue config, and current runtime settings.
- [Database](database.md) explains PostgreSQL config, metadata schema sync, explicit patches, schema prune, schema snapshots, and `dygo db check`.
- [Explicit Patches](patches.md) explains how unsafe metadata changes are handled without SQL migration files.
- [Server](server.md) explains `dygo dev`, `dygo serve`, the health endpoint, auth, metadata APIs, and Record APIs.
- [Auth](auth.md) explains Administrator bootstrap, session login, and authenticated API identity.
- [Records](records.md) explains the metadata-powered Record CRUD API.
- [Logs](logs.md) explains persisted framework and app diagnostic events.
- [Encrypted Secrets](secrets.md) explains repo-stored encrypted secrets and `dygo secret`.

## Jobs And Automation

- [Jobs](jobs.md) explains Job metadata, Job Executions, retries, SDK usage, and Job CLI operations.
- [Queues And Workers](queues.md) explains queue config, worker claiming, notifications, timers, and production worker processes.
- [Schedules](schedule.md) explains app-owned recurring Schedules and how workers turn them into Job Executions.

## Studio

- [Studio](studio.md) explains the first-party global UI app, route model, and design responsibilities.
- [Dialogs](dialogs.md) records the proposed shared Studio dialog API.

## Maintainers

- [Directory Shape](dir.md) documents the generated project layout, deployed runtime layout, and framework repository layout.
- [Maintainer Notes](notes.md) records repo-maintenance notes that are useful to keep versioned but are not framework reference material.
- [Roadmap](todo.md) tracks local issue status for the dygo repository.
- [Project README](../README.md) gives a short overview and quick start.
- [Contributing](../CONTRIBUTING.md) explains the current paused contribution status.
- [Security Policy](../SECURITY.md) explains private vulnerability reporting and safe research guidelines.
- [Agent Instructions](../AGENT.md) stores repo-level guidance for coding agents.
