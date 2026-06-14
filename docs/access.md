# Access

Status: proposed.

This document records the intended authoring model for dygo roles and Entity access. The current runtime still reads live Core `role`, `user-role`, and `permission` Records from the database.

## Direction

Roles and access grants are app metadata, not ordinary seed data.

App authors should be able to review an app and answer:

- which roles the app defines
- which Entities each role can access
- which actions each role can use
- which runtime assignments are outside the app contract

`dygo db migrate` should eventually validate this metadata and sync it into Core permission Records.

The target app shape is:

```txt
apps/sales/
  app.yml
  entities/
    customer/
      customer.entity.yml
    invoice/
      invoice.entity.yml
  access/
    _roles.yml
    customer.access.yml
    invoice.access.yml
```

## Roles

Each app gets one role vocabulary file:

```txt
apps/<app>/access/_roles.yml
```

It defines app-owned roles only, not Entity grants:

```yaml
roles:
  - name: manager
    label: Sales Manager
    description: Can manage sales operations.

  - name: user
    label: Sales User
    description: Can create and update sales records.
```

`role` remains a Core Entity in the database. `_roles.yml` is the app authoring source that syncs Core `role` Records; it is not a fixture file.

Role names are app-scoped. A role named `manager` in `apps/sales/access/_roles.yml` has the canonical identity `sales/manager`.

Cross-app role references must be explicit:

```txt
manager              -> sales/manager inside the sales app
core/system-manager  -> system-manager role from the core app
```

Do not use ambiguous global role lookup.

## Entity Access

Each Entity gets one access file:

```txt
apps/<app>/access/<entity>.access.yml
```

The file owns the access contract for that Entity. The first supported section is simple role-to-action permissions:

```yaml
entity: invoice

permissions:
  - role: manager
    actions: [read, create, update, delete, export, print]

  - role: user
    actions: [read, create, update, print]

  - role: core/system-manager
    actions: [read, create, update, delete, export, print]
```

Bare role names resolve inside the same app as the access file. App-qualified role references resolve across apps.

The first supported action names should match the existing permission engine:

```txt
read
create
update
delete
export
print
```

Validation should fail when:

- a role name is duplicated inside one app
- an access file references an unknown Entity
- an access file references an unknown role
- an access file references an unsupported action
- a cross-app role reference omits the app name
- two access files define grants for the same Entity

## Why Access Is Separate

Roles are subjects. Entities are objects. Access files define what those subjects can do to those objects.

Putting grants inside `_roles.yml` makes it easy to answer "what can this role do?" but hard to answer "who can touch invoices?" A framework should optimize for access review. Per-Entity access files make the object rules obvious.

Putting access rules inside each Entity bundle is better than fixtures, but still mixes authorization with schema, hooks, and seed data. Keeping access under `apps/<app>/access/` gives reviewers one place to inspect an app's access contract:

```txt
apps/sales/access/
```

It also keeps Entity bundles focused on data shape and behavior:

```txt
entities/invoice/invoice.entity.yml  - schema and metadata
entities/invoice/hooks.go            - app behavior
entities/invoice/fixtures.yml        - seed Records
access/invoice.access.yml            - Entity access contract
```

Use `entities/<entity>/<entity>.access.yml` only if dygo later chooses stronger Entity co-location over centralized access review.

## Runtime Data

User-role assignments are not app metadata.

These belong in Studio, setup flows, admin CLI commands, or environment/demo fixtures:

```txt
Tahseen -> sales-manager
Ali     -> sales-user
```

## Core Bootstrap

Core should use the same target model:

```txt
apps/core/access/_roles.yml
apps/core/access/*.access.yml
```

Core can still seed framework-owned bootstrap roles and grants through fixtures until the metadata loader exists. That is an implementation bridge, not the long-term authoring model.

Administrator remains a `user.administrator` flag, not a role.

## Access Sprint Tasks

- add shape helpers for `apps/<app>/access/`, `access/_roles.yml`, and `access/<entity>.access.yml`
- update app and Entity generators to create the new access files
- replace generated root `roles.yml` with `access/_roles.yml`
- add loaders and validators for `_roles.yml` and `<entity>.access.yml`
- validate app-scoped role names and explicit cross-app role references
- validate that each access file points at one known Entity
- sync validated roles and Entity grants during `dygo db migrate`
- remove stale `entities/<entity>/permissions.yml`, root `roles.yml`, and `permissions/` documentation once the loader exists
- side task: rename Entity metadata from `entities/<entity>/entity.yml` to `entities/<entity>/<entity>.entity.yml`
- side task: update Entity shape helpers, generators, validators, JSON Schemas, metadata loading, and docs for the `<entity>.entity.yml` filename

## Open Decisions

- how app-scoped role identities map onto current Core `role.name`
- whether removed grants disable or delete live permission Records
- whether generated apps should include empty `access/_roles.yml`
- how Studio-authored access changes export back to app metadata
