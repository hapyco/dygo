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

Fixture files live under an App's manifest-defined `fixtures` directory.

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

## Apply Behavior

`dygo fixtures apply` discovers fixtures from all loaded Apps, validates metadata first, then applies records in deterministic order inside one transaction.

For each fixture record, dygo finds an existing Record through `match`. If one exists, it updates it. If none exists, it creates it through the generic Record runtime.

The command prints:

```txt
fixtures applied: 3 created, 2 updated (development)
```

## Boundaries

`dygo db prepare` does not apply fixtures in v1. It still creates the database and syncs metadata schema only.

No active Core role or permission fixture files are shipped yet. The runner exists first so the initial Core access model can be chosen deliberately.

Fixtures do not delete Records, prune schema, run patches, track history, or expose HTTP endpoints.
