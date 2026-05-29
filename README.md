# dygo

[![Go](https://img.shields.io/badge/go-1.26.2-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![License](https://img.shields.io/badge/license-O%27Saasy-blue)](LICENSE)
[![Status](https://img.shields.io/badge/status-early%20framework%20development-f2cc60)](https://github.com/hapyco/dygo/commits/develop)
[![Contributions](https://img.shields.io/badge/contributions-paused-lightgrey)](CONTRIBUTING.md)
[![Issues](https://img.shields.io/badge/issues-GitHub-2ea44f)](https://github.com/hapyco/dygo/issues)

dygo is a source-available framework for building serious business software in Go.

It is designed for business processes, internal operating systems, enterprise applications, and workflow-heavy products where permissions, metadata-driven schema, audit trails, observability, background jobs, secure configuration, and a consistent Studio UI matter from day one.

The goal is speed with structure: builders should focus on business logic while dygo handles the platform foundation.

## License

dygo is released under the [O'Saasy License](LICENSE). It is free to use, modify, self-host, and build with, but competing hosted, managed, SaaS, or cloud services where dygo itself is the primary value are reserved for the original licensor.

## Status

dygo is in early framework development.

The current repository contains the first Go module, CLI entrypoint, config defaults, HTTP server, encrypted credentials, app/entity metadata validation, PostgreSQL schema sync, Core metadata registry, metadata API, session auth, generic Record API foundation with system Record naming, Record permission enforcement, and a compiled Record hook SDK. The framework APIs are not stable yet.

## Install

The intended installer is:

```sh
curl -fsSL https://dygo.dev/install | sh
```

Until `dygo.dev/install` is wired, use:

```sh
curl -fsSL https://raw.githubusercontent.com/hapyco/dygo/main/scripts/install.sh | sh
```

## CLI

See [dygo CLI](docs/cli.md) for the command surface.

`dygo dev` starts dygo on `127.0.0.1:6790` for local development. In this source checkout it also starts the Studio development asset server internally and proxies Studio through the same dygo origin.

The default server address is:

```txt
127.0.0.1:6790
```

The first HTTP endpoints are:

```txt
GET /health
POST /api/v1/auth/login
POST /api/v1/auth/logout
GET /api/v1/auth/me
GET /api/v1/apps
GET /api/v1/apps/{app}
GET /api/v1/entities
GET /api/v1/entities/{entity}/meta
GET /api/v1/records/{entity}
GET /api/v1/records/{entity}/{id}
GET /api/v1/records/{entity}/name/{name}
GET /api/v1/records/{entity}/{id}/activity
POST /api/v1/records/{entity}
PATCH /api/v1/records/{entity}/{id}
DELETE /api/v1/records/{entity}/{id}
```

The API endpoints are generic and metadata-powered; dygo does not create separate handlers for each Entity. `{entity}` is the Entity route slug, defaulting to the file-derived Entity key.

Metadata and Record APIs require an authenticated Studio session. Metadata visibility is permission-aware, and Record routes check Entity actions through the permission engine.

Scoped Record Activity is read through the target Record route and checked against the target Entity's `read` permission.

Project-aware commands discover the dygo root by walking upward from the current directory. Generated projects use `dygo.yml` as the root marker and runtime config file; the framework repository is also recognized during development.

`dygo new <name>` creates a project with one app under `apps/`, a project-local `cmd/dygo` runner, encrypted secrets, a development `DATABASE_URL` secret, and cached Studio UI assets when the running dygo build has them. It does not create a database, run schema sync, apply fixtures, or create the first Administrator; run the printed first-run commands before opening Studio.

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
config/            committed project support files
.dygo/             generated local state, cached apps, logs, temp files, and local secret keys
db/                generated schema snapshot
docs/              framework documentation
```

## Docs

- [Documentation Index](docs/index.md)
- [Installation](docs/installation.md)
- [CLI](docs/cli.md)
- [Doctrine](docs/doctrine.md)
- [Nomenclature](docs/nomenclature.md)
- [App Model](docs/app-model.md)
- [App Manifest](docs/app-manifest.md)
- [Entity Metadata](docs/entity-metadata.md)
- [Metadata Authoring](docs/metadata-authoring.md)
- [Config](docs/config.md)
- [Database](docs/database.md)
- [Explicit Patches](docs/patches.md)
- [Fixtures](docs/fixtures.md)
- [Server](docs/server.md)
- [Auth](docs/auth.md)
- [Records](docs/records.md)
- [App SDK](docs/sdk.md)
- [Studio](docs/studio.md)
- [Encrypted Secrets](docs/secrets.md)
- [Contributing](CONTRIBUTING.md)
- [Security Policy](SECURITY.md)

## Issues

Open issue tracking lives in the GitHub issue list:

- [dygo issues](https://github.com/hapyco/dygo/issues)

Repository and issue-tracking metadata for maintainers and agents lives in [config/github.yml](config/github.yml).
