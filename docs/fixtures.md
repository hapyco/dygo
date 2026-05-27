# Fixtures

Fixtures are app-owned seed Records.

They are for roles, permissions, reference data, and other runtime defaults that should be versioned with an App.

Use the normal app-state workflow to sync metadata and apply fixtures:

```sh
dygo db migrate
```

Use explicit fixture commands when authoring, debugging, or exporting fixture files. For example, validate fixture files without database writes:

```sh
dygo fixture validate
```

Apply fixtures directly to another encrypted environment with `--env`:

```sh
dygo fixture apply --env staging
```

## File Shape

Fixture files live inside normal Entity bundles:

```txt
apps/core/entities/role/fixtures.yml
apps/core/entities/permission/fixtures.yml
```

Do not use numeric prefixes inside Entity bundle names. dygo orders fixture application from link dependencies, so permission fixtures can reference role fixtures without filename tricks.

Each `fixtures.yml` file declares one Entity:

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

Core fixtures also do not grant `studio-member` generic `activity` Record access. Activity rows can include snapshots and field diffs, so normal users should read them through scoped per-Record Activity APIs. `system-manager` receives read-only activity access for operational inspection.

## Apply Behavior

`dygo fixture validate` discovers fixtures from all loaded Apps and validates fixture files, match fields, link references, dependency cycles, and collection limitations without writing records.

`dygo fixture apply` performs the same validation, prints a plan, prompts, then applies records in deterministic order inside one transaction. Apply order is derived from link dependencies between fixture Entities, not from numeric filename prefixes.

For each fixture record, dygo finds an existing Record through `match`. If one exists, it updates it. If none exists, it creates it through the generic Record runtime.

Use `--dry-run` to print the plan without writing, and `--yes` to skip the interactive prompt after reviewing the plan:

```sh
dygo fixture apply --dry-run
dygo fixture apply --yes
```

The command prints:

```txt
fixtures applied: 3 created, 2 updated (development)
```

## Boundaries

`dygo db migrate` applies fixtures as part of the normal app-state workflow. Keep `dygo fixture apply`, `dygo fixture validate`, and `dygo fixture export <app>/<entity>` for app-author tooling, debugging, and exporting Studio-authored Records back into app-owned fixture files.

Fixtures do not delete Records, prune schema, run patches, track history, or expose HTTP endpoints.
