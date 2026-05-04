# App Model

dygo is a framework runtime with installable Apps.

The runtime loads installed apps, applies their patches and fixtures, registers their entities and permissions, and exposes their behavior through APIs, jobs, hooks, and the Studio.

## Built-In Apps

The framework repo may contain first-party apps that ship with dygo.

`apps/core` is the required system app. dygo cannot boot properly without it.

It owns:

- users
- roles
- permissions
- sessions
- installed apps
- patch ledger
- migration ledger
- files and attachments when required by the runtime
- core fixtures
- core patches

The framework repo includes the initial Core app manifest at `apps/core/app.yml`.

`apps/studio` is the first-party UI app.

It owns:

- Studio shell
- navigation
- command menu
- Spaces UI
- global list renderer
- global record renderer
- global form renderer
- field renderers
- child table renderer
- saved views UI
- jobs UI
- audit log UI
- settings UI
- metadata API client
- frontend stores

The framework repo includes the initial Studio app manifest at `apps/studio/app.yml`.

## Business Apps

Business apps define metadata and behavior for a project.

A basic business app should stay small:

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

Business apps should not need default `views`, `spaces`, `reports`, or `migrations` folders at the start. Add those only when the app needs custom behavior beyond global Studio rendering.

Each app is described by an `app.yml` manifest. See [App Manifest](app-manifest.md) for the v1 schema.

Entity files live in the app's manifest-defined `entities` directory. Entity names are unique within the owning app for v1, and each Entity's `module` must be declared in the owning app manifest.

## Install Locations

Framework repo `apps/` contains first-party apps shipped by dygo.

Generated project `apps/` contains business apps owned by the project.

Generated project `.dygo/apps/` contains framework-managed cached apps.

Generated project `var/` contains runtime-generated data.

## Hierarchy

dygo runtime loads installed Apps.

Core is the required system App.

Studio is the first-party UI App.

Business Apps define Entities, Permissions, Hooks, Fixtures, and Patches.

Studio renders those Apps globally through Spaces, Records, Forms, Lists, and Saved Views.
