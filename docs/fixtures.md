# Fixtures

Fixtures are app-owned seed Records.

They are for roles, permissions, reference data, and other runtime defaults that should be versioned with an App.

Run metadata sync before applying fixtures:

```sh
go run ./cmd/dygo migrate
go run ./cmd/dygo fixtures apply
```

Use another encrypted environment with `--env`:

```sh
go run ./cmd/dygo fixtures apply --env staging
```

## File Shape

Fixture files live under an App's manifest-defined `fixtures` directory. The file name must match the Entity name:

```txt
apps/core/fixtures/role.yml
apps/core/fixtures/permission.yml
```

Do not use numeric prefixes such as `001_role.yml`. dygo orders fixture application from link dependencies, so `permission.yml` can reference roles seeded by `role.yml` without filename tricks.

Each `*.yml` file declares one Entity:

```yaml
entity: role
match: [name]
records:
  - name: system-manager
    label: System Manager
    enabled: true
```

`match` makes fixtures idempotent. It must reference a unique field or a top-level unique constraint on the Entity. Every record must include all match fields.

## Link References

Use natural-key references for link fields instead of raw database IDs:

```yaml
entity: permission
match: [entity, role]
records:
  - entity:
      match:
        name: user
    role:
      match:
        name: system-manager
    read: true
    create: true
    update: true
    delete: true
```

dygo infers the target Entity from Field metadata and resolves the linked Record through that target's own unique match fields.

## Core Fixtures

Core ships the first default roles:

- `studio-member`: baseline role for users who can sign in to Studio and read structural metadata needed by the shell.
- `system-manager`: operational role for managing users, roles, role assignments, and permissions.

Administrator remains a `user` flag, not a role. It is the only v1 bypass. `system-manager` still goes through the permission engine.

Core fixtures intentionally do not grant generic `session` Record access yet. Session management needs a dedicated surface that does not expose token digest fields through normal Record reads.

Core fixtures also do not grant `studio-member` generic `activity` Record access. Activity rows can include snapshots and field diffs, so normal users should read them later through scoped per-Record activity APIs. `system-manager` receives read-only activity access for operational inspection.

## Apply Behavior

`dygo fixtures apply` discovers fixtures from all loaded Apps, validates metadata first, then applies records in deterministic order inside one transaction. Apply order is derived from link dependencies between fixture Entities, not from numeric filename prefixes.

For each fixture record, dygo finds an existing Record through `match`. If one exists, it updates it. If none exists, it creates it through the generic Record runtime.

The command prints:

```txt
fixtures applied: 3 created, 2 updated (development)
```

## Boundaries

`dygo db prepare` does not apply fixtures in v1. It still creates the database and syncs metadata schema only.

Fixtures do not delete Records, prune schema, run patches, track history, or expose HTTP endpoints.
