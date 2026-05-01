# dygo

[![Go](https://img.shields.io/badge/go-1.26.2-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![License](https://img.shields.io/github/license/dygo-dev/dygo)](https://github.com/dygo-dev/dygo/blob/develop/LICENSE)
[![Status](https://img.shields.io/badge/status-early%20framework%20development-f2cc60)](https://github.com/dygo-dev/dygo/commits/develop)
[![Roadmap](https://img.shields.io/badge/roadmap-GitHub%20Projects-6f42c1)](https://github.com/orgs/dygo-dev/projects/1)

dygo is an open-source framework for building serious business software in Go.

It is designed for business processes, internal operating systems, enterprise applications, and workflow-heavy products where permissions, migrations, audit trails, observability, background jobs, secure configuration, and consistent operational UI matter from day one.

The goal is speed with structure: builders should focus on business logic while dygo handles the platform foundation.

## Status

dygo is in early framework development.

The current repository contains the first Go module, CLI entrypoint, config defaults, and project doctrine/docs. The framework APIs are not stable yet.

## Current CLI

```sh
go run ./cmd/dygo
go run ./cmd/dygo version
go run ./cmd/dygo serve
```

The default server address is:

```txt
127.0.0.1:6790
```

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
configs/           safe committed config files
docs/              project doctrine, thesis, and structure notes
```

## Docs

- [The dygo Doctrine](docs/doctrine.md)
- [Platform Thesis](docs/platform-thesis.md)
- [Directory Structure](docs/dir-structure.md)

## Roadmap

Roadmap work is tracked in GitHub Projects:

- [dygo Roadmap](https://github.com/orgs/dygo-dev/projects/1)

Repository/project metadata for maintainers and agents lives in [configs/github.yml](configs/github.yml).
