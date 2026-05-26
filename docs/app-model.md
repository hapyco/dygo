# App Model

dygo is a framework runtime with installable Apps.

The runtime loads installed apps, applies their patches and fixtures, registers their entities and permissions, and exposes their behavior through APIs, jobs, hooks, and the Studio.

## Built-In Apps

The framework repo may contain first-party apps that ship with dygo.

`apps/core` is the required system app. dygo cannot boot properly without it.

It owns:

- system Entity, Field, Index, and Constraint contracts
- users
- roles
- permissions
- sessions
- installed App state
- files and attachments when required by the runtime
- core fixtures
- core patches

The framework repo includes the initial Core app manifest at `apps/core/app.yml`.

The first Core Entity contracts live under `apps/core/entities/`. dygo creates Core SQL tables from this metadata through the same schema path used by every other App. Core is required, but it is still an App.

After schema sync succeeds, dygo persists discovered App, Entity, Field, Index, and Constraint metadata into Core records. YAML metadata stays the source of truth; the Core records are the runtime registry that APIs and Studio read.

The first generic Record API uses this runtime registry. Auth resolves an Administrator account and session identity from Core records. Record APIs ask the single permission engine before reading or mutating Records. The engine allows Administrator users first, then evaluates flat Core role permissions. App lifecycle patches, sharing rules, row-level permissions, field-level permissions, and Studio screens are separate follow-up layers.

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
- collection renderer
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
    lead/
      entity.yml
      fixtures.yml
      hooks.go
      permissions.yml
    deal/
      entity.yml
      hooks.go
    company/
      entity.yml
  patches/
    0001_seed_default_pipeline.yml
  assets/
    icon.svg
  docs/
    index.md
```

Business apps should not need default `views`, `spaces`, or `reports` folders at the start. Add those only when the app needs custom behavior beyond global Studio rendering.

Each app is described by an `app.yml` manifest. See [App Manifest](app-manifest.md) for the v1 schema.

Entity files live in the app's manifest-defined `entities` directory. Entity identity is app-scoped, so `crm/contact` and `support/contact` can both exist when their route slugs are unique.

Entity metadata uses singular keys only. Studio record URLs use `route.slug`, defaulting to the Entity key, at `/{slug}`. Non-Core storage tables are app-scoped by default, so `crm/lead` maps to `crm_lead`.

Every Record also has a system `name` generated from Entity `name` metadata. Apps can choose manual names, random names, format names, or series names. The numeric `id` remains dygo's internal primary key.

Hooks are app-owned Go code inside Entity bundles. A file such as `entities/lead/hooks.go` belongs to Entity `lead`. dygo validates this convention, but the code must still be compiled into a project runner through `pkg/sdk/runtime`; dygo does not dynamically load Go source files.

Patches are app-owned lifecycle changes for unsafe transitions that metadata cannot infer, such as renames, drops, destructive type changes, and data backfills. See [Explicit Patches](patches.md) for the v1 runner workflow.

Fixtures are app-owned seed Records for roles, permissions, and reference data. They live inside Entity bundles as `entities/<entity>/fixtures.yml` and can be applied explicitly with `dygo fixture apply`. See [Fixtures](fixtures.md) for the v1 file shape.

## Install Locations

Framework repo `apps/` contains first-party apps shipped by dygo.

Generated project `apps/` contains business apps owned by the project.

Generated project `.dygo/apps/` contains framework-managed cached apps.

Generated project `.dygo/` contains runtime-generated local state, cached apps, logs, temp files, and local secret keys.

## Hierarchy

dygo runtime loads installed Apps.

Core is the required system App.

Studio is the first-party UI App.

Business Apps define Entities, Permissions, Hooks, Fixtures, and Patches.

Studio initially groups metadata by App and renders those Apps globally through Spaces, Records, Forms, Lists, and Saved Views.
