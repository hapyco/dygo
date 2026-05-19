# Directory Structure

This document describes the intended dygo framework repo shape and the generated project shape. dygo has three important app locations:

- framework repo `apps/` contains first-party apps shipped by dygo
- generated project `apps/` contains business apps owned by that project
- generated project `.dygo/apps/` contains framework-managed cached apps

## Framework Repo

```txt
dygo/
  README.md
  AGENT.md
  LICENSE
  go.mod
  go.sum

  cmd/
    dygo/
      main.go

  internal/
    cli/
    config/
    runtime/
    server/
    db/
    app/
    entity/
    record/
    permissions/
    auth/
    jobs/
    audit/
    storage/
    studio/
    telemetry/

  pkg/
    sdk/

  apps/
    core/
      app.yml
      entities/
      permissions/
      fixtures/
      patches/
      hooks/
      docs/

    studio/
      app.yml
      ui/
        package.json
        vite.config.ts
        src/
          app/
          shell/
          layouts/
          renderers/
          pages/
          stores/
          api/
          router/
          styles/
      entities/
      permissions/
      fixtures/
      hooks/
      docs/

  configs/
    dygo.yaml
    github.yml

  db/
    schema.sql

  docs/
    index.md
    doctrine.md
    platform-thesis.md
    nomenclature.md
    dir-structure.md
    app-model.md
    app-manifest.md
    patches.md
    studio.md
    secrets.md
    docs-strategy.md

  examples/
    apps/
    projects/

  scripts/
  deploy/
  testdata/
```

## Built-In Apps

`apps/core` is the required system app. dygo cannot boot properly without it.

It owns users, roles, permissions, sessions, installed apps, Entity/Field/Index/Constraint metadata contracts, patch history, core fixtures, core patches, and files or attachments when they are required by the runtime.

`apps/studio` is the first-party UI app.

It owns the Studio shell, navigation, command menu, Spaces UI, global renderers for lists and records, form and field renderers, child table rendering, saved views UI, jobs UI, audit log UI, settings UI, frontend stores, and the metadata API client.

## Business App Shape

A basic business app should define metadata and behavior only.

```txt
dygo-crm/
  app.yml
  entities/
    lead.yml
    deal.yml
    company.yml
  permissions/
    lead.permissions.yml
    deal.permissions.yml
    company.permissions.yml
  hooks/
    lead.go
    deal.go
    company.go
  fixtures/
    roles.yml
    lead-statuses.yml
  patches/
    0001_seed_default_pipeline.yml
  assets/
    icon.svg
  docs/
    index.md
```

Do not create default `views`, `spaces`, or `reports` folders for every app. Add them later only when a specific app needs them.

## Generated Project

When someone runs:

```sh
dygo new my-company
```

the generated project should look like this:

```txt
my-company/
  README.md
  .gitignore
  dygo.yml
  go.mod
  go.sum

  cmd/
    dygo/
      main.go

  apps/
    my-company/
      app.yml
      entities/
      permissions/
      hooks/
      fixtures/
      patches/
      assets/
      docs/

  configs/
    dygo.yaml
    secrets/
      development.yml.age
      staging.yml.age
      production.yml.age

  db/
    schema.sql

  docs/
    index.md

  var/
    storage/
      public/
      private/
    backups/
    logs/
    tmp/
    cache/
    imports/
    exports/

  .dygo/
    apps/
    cache/

  master.key
```

`dygo.yml` is the generated project root marker. CLI commands walk upward from the current directory to find it before reading apps, config, secrets, and future runtime state.

`master.key`, `.dygo/`, and `var/` are generated local state and ignored by the generated `.gitignore`. The encrypted files under `configs/secrets/` are safe to commit.

## Runtime Rules

`apps/` in the framework repo is for first-party apps shipped by dygo.

`apps/` in a generated project is for business apps owned by the project.

`.dygo/apps/` is for framework-managed cached apps and the generated-project Studio UI cache.

`db/schema.sql` is the generated Postgres schema snapshot after metadata schema sync runs.

`var/` is for runtime-generated data.

Studio is the global UI app that renders installed apps.

Business apps provide metadata and behavior first.
