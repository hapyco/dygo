# App Manifest

Every dygo app is described by an `app.yml` file at the app root.

The manifest is intentionally small in v1. It identifies the app, names its dependencies, and records the standard app-relative paths dygo should read later.

## Example

```yaml
name: dygo-crm
label: CRM
version: 0.1.0
description: Customer relationship management
dependencies:
  - core
paths:
  entities: entities
  permissions: permissions
  hooks: hooks
  fixtures: fixtures
  patches: patches
  docs: docs
  assets: assets
```

## Fields

`name` is required and must use kebab-case. This is the canonical app name used by dependency references and future CLI commands.

`label` is required and is the human-facing app name.

`version` is required and should look like `major.minor.patch`.

`description` is optional.

`dependencies` is optional and contains app names only. Version constraints and remote source references are intentionally out of scope for v1.

`paths` is optional. Omitted paths use dygo's standard app folder names:

```txt
entities
permissions
hooks
fixtures
patches
docs
assets
```

Path values must be relative, clean, use forward slashes, and stay inside the app directory.

The app root directory should usually match the manifest `name`.

List discovered apps from the current project:

```sh
go run ./cmd/dygo apps list
go run ./cmd/dygo apps validate
go run ./cmd/dygo entities validate
```

The app commands read app manifests from `apps/` and `.dygo/apps/`. Entity validation uses the discovered apps to load each app's `entities/` directory.

## V1 Boundaries

The app manifest loader is internal for now.

The manifest does not include an app `type` field yet. Core and Studio bootstrap rules will be defined by later tasks.

The manifest does not fetch, install, migrate, or write to the database. It only gives dygo a validated description of app metadata on disk.
