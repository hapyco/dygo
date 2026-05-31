# dygo

[![Go](https://img.shields.io/badge/go-1.26.2-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![License](https://img.shields.io/badge/license-O%27Saasy-blue)](LICENSE)
[![Status](https://img.shields.io/badge/status-early%20framework%20development-f2cc60)](https://github.com/hapyco/dygo/commits/develop)
[![Contributions](https://img.shields.io/badge/contributions-paused-lightgrey)](CONTRIBUTING.md)
[![Issues](https://img.shields.io/badge/issues-GitHub-2ea44f)](https://github.com/hapyco/dygo/issues)

dygo is a source-available Go framework for building serious business software.

It gives business apps a conventional platform foundation: metadata-driven PostgreSQL schema, encrypted configuration, session auth, permissions, generic Record APIs, Record hooks, durable Jobs, recurring Schedules, and a first-party Studio UI.

## Status

dygo is in early framework development. The repository contains the framework module, CLI, project generator, metadata validators, PostgreSQL schema sync, Core metadata registry, session auth, generic Record APIs, Record permission enforcement, compiled Record hooks, durable Jobs, recurring Schedules, and worker runtime.

Framework APIs and file formats may still change before the first stable release.

## Quick Start

Install dygo:

```sh
curl -fsSL https://dygo.dev/install | sh
```

Until `dygo.dev/install` is wired, use the repository-hosted installer:

```sh
curl -fsSL https://raw.githubusercontent.com/hapyco/dygo/main/scripts/install.sh | sh
```

Create and run a project:

```sh
dygo new my-system
cd my-system
dygo db migrate
dygo setup
dygo dev
```

`dygo dev` starts the local server on `http://localhost:6790/`. In this source checkout it also starts the Studio Vite server and proxies Studio through the same dygo origin.

## What You Build With

- Apps live under `apps/<app>` and describe business modules.
- Entities define metadata-backed Records, fields, indexes, constraints, permissions, hooks, fixtures, and route slugs.
- `dygo db migrate` compares metadata with PostgreSQL, applies safe schema changes, syncs Core metadata records, applies fixtures, and refreshes `db/schema.sql`.
- Studio renders global app surfaces from metadata.
- Jobs run durable background work through PostgreSQL-backed Job Executions.
- Schedules create Job Executions from app-owned cron metadata.

## Common Commands

```sh
dygo generate app crm
dygo generate entity crm/contact
dygo generate job crm/send-welcome-email

dygo app validate
dygo entity validate
dygo doctor

dygo db migrate
dygo dev
dygo serve
dygo worker
```

See [CLI](docs/cli.md) for the full command surface.

## Documentation

- [Documentation Index](docs/index.md)
- [Installation](docs/installation.md)
- [CLI](docs/cli.md)
- [App Model](docs/app-model.md)
- [Entity Metadata](docs/entity-metadata.md)
- [Database](docs/database.md)
- [Records](docs/records.md)
- [Record Hooks](docs/record-hooks.md)
- [Jobs](docs/jobs.md)
- [Queues And Workers](docs/queues.md)
- [Schedules](docs/schedule.md)
- [Server](docs/server.md)
- [Studio](docs/studio.md)
- [App SDK](docs/sdk.md)
- [Encrypted Secrets](docs/secrets.md)

## Repository Development

Requirements:

- Go 1.26.2+
- PostgreSQL for database-backed commands and runtime tests
- Node.js only when working on Studio UI assets

Useful focused checks:

```sh
go test ./cmd/dygo ./pkg/... ./internal/... ./schemas
go vet ./...
```

## License

dygo is released under the [O'Saasy License](LICENSE). It is free to use, modify, self-host, and build with, but competing hosted, managed, SaaS, or cloud services where dygo itself is the primary value are reserved for the original licensor.

## Security

Report possible security vulnerabilities privately through the [Security Policy](SECURITY.md).

## Issues

Open issue tracking lives in [GitHub Issues](https://github.com/hapyco/dygo/issues). Repository and issue-tracking metadata for maintainers and agents lives in [config/github.yml](config/github.yml).
