# dygo

[![Go](https://img.shields.io/badge/go-1.26.2-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![License](https://img.shields.io/github/license/dygo-dev/dygo)](https://github.com/dygo-dev/dygo/blob/develop/LICENSE)
[![Status](https://img.shields.io/badge/status-early%20framework%20development-f2cc60)](https://github.com/dygo-dev/dygo/commits/develop)
[![Contributions](https://img.shields.io/badge/contributions-paused-lightgrey)](CONTRIBUTING.md)
[![Roadmap](https://img.shields.io/badge/roadmap-GitHub%20Projects-6f42c1)](https://github.com/orgs/dygo-dev/projects/2/views/1)

dygo is an open-source framework for building serious business software in Go.

It is designed for business processes, internal operating systems, enterprise applications, and workflow-heavy products where permissions, metadata-driven schema, audit trails, observability, background jobs, secure configuration, and a consistent Studio UI matter from day one.

The goal is speed with structure: builders should focus on business logic while dygo handles the platform foundation.

## Status

dygo is in early framework development.

The current repository contains the first Go module, CLI entrypoint, config defaults, HTTP server skeleton, encrypted credentials, app/entity metadata validation, and PostgreSQL schema sync foundation. The framework APIs are not stable yet.

## Current CLI

```sh
go run ./cmd/dygo
go run ./cmd/dygo version
go run ./cmd/dygo doctor
go run ./cmd/dygo serve
go run ./cmd/dygo db check
go run ./cmd/dygo db create
go run ./cmd/dygo db prepare
go run ./cmd/dygo db schema dump
go run ./cmd/dygo migrate
go run ./cmd/dygo apps list
go run ./cmd/dygo apps validate
go run ./cmd/dygo entities list
go run ./cmd/dygo entities validate
go run ./cmd/dygo secrets init
go run ./cmd/dygo secrets edit
go run ./cmd/dygo secrets validate
go run ./cmd/dygo secrets rotate-key
```

`go run ./cmd/dygo serve` starts the local HTTP server.

The default server address is:

```txt
127.0.0.1:6790
```

The first health endpoint is:

```txt
GET /health
```

Project-aware commands discover the dygo root by walking upward from the current directory. Generated projects use `dygo.yml` as the root marker; the framework repository is also recognized during development.

## Development

Requirements:

- Go 1.26.2+

Verify the repo:

```sh
go test ./...
go test -race ./...
go vet ./...
```

## Project Shape

```txt
cmd/dygo/          executable entrypoint
internal/cli/      private CLI implementation
internal/config/   private config defaults and loading code
internal/db/       private PostgreSQL code
apps/              first-party dygo apps such as core and Studio
configs/           safe committed config files
db/                generated schema snapshot
docs/              project doctrine, thesis, and structure notes
```

## Docs

- [Documentation Index](docs/index.md)
- [The dygo Doctrine](docs/doctrine.md)
- [Platform Thesis](docs/platform-thesis.md)
- [Nomenclature](docs/nomenclature.md)
- [App Model](docs/app-model.md)
- [App Manifest](docs/app-manifest.md)
- [Entity Metadata](docs/entity-metadata.md)
- [Config](docs/config.md)
- [Database](docs/database.md)
- [Server](docs/server.md)
- [Studio](docs/studio.md)
- [Encrypted Secrets](docs/secrets.md)
- [Contributing](CONTRIBUTING.md)
- [Documentation Strategy](docs/docs-strategy.md)
- [Directory Structure](docs/dir-structure.md)

## Roadmap

Roadmap work is tracked in GitHub Projects:

- [dygo Roadmap](https://github.com/orgs/dygo-dev/projects/2/views/1)

Repository/project metadata for maintainers and agents lives in [configs/github.yml](configs/github.yml).
