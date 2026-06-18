# Fixtures

Fixtures are app-owned seed Records.

They are for reference data, demo/setup data, and other ordinary runtime defaults that should be versioned with an App. App access roles and grants move to [Access](access.md); they should not remain generic fixtures once the access metadata loader exists.

Apply fixtures explicitly:

```sh
dygo fixture apply
```

Use `dygo db prepare` when preparing a full usable environment that should run migration, access apply, and fixture apply together.

Use fixture commands when authoring, debugging, or exporting fixture files. For example, validate fixture files without database writes:

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
apps/core/entities/country/fixtures.yml
apps/core/entities/currency/fixtures.yml
```

Do not use numeric prefixes inside Entity bundle names. dygo orders fixture application from link dependencies, so linked fixture Records can reference each other without filename tricks.

Each `fixtures.yml` file declares one Entity:

```yaml
entity: country
match: [code]
records:
  - name: United States
    code: US
```

`match` makes fixtures idempotent. It must reference a unique field or a top-level unique constraint on the Entity. Every record must include all match fields.

## Link References

Use natural-key references for link fields instead of raw database IDs:

```yaml
entity: city
match: [code]
records:
  - name: Karachi
    code: KHI
    country:
      match:
        code: PK
```

dygo infers the target Entity from Field metadata and resolves the linked Record through that target's own unique match fields.

## Core Bootstrap Fixtures

Until the access metadata loader exists, Core still ships default roles and permissions as fixture files. That is a bootstrap bridge, not the long-term authoring model.

Core ships the first default roles:

- `studio-member`: baseline role for users who can sign in to Studio and read structural metadata needed by the shell.
- `system-manager`: operational role for managing users, roles, role assignments, and permissions.

Administrator remains a `user` flag, not a role. It is the only v1 bypass. `system-manager` still goes through the permission engine.

Core fixtures intentionally do not grant generic `session` Record access yet. Session management needs a dedicated surface that does not expose token digest fields through normal Record reads.

Core fixtures also do not grant `studio-member` generic `activity` Record access. Activity rows can include snapshots and field diffs, so normal users should read them through scoped per-Record Activity APIs. `system-manager` receives read-only activity access for operational inspection.

The target model moves these Core role and permission definitions to:

```txt
apps/core/access/_roles.yml
apps/core/access/*.access.yml
```

## Fixture Eligibility

Fixture validate, apply, and export should share one central eligibility policy.

Target policy:

| Entity or group | Fixture policy | Reason |
| --- | --- | --- |
| Ordinary business and reference Entities | allowed | App-owned seed data is the main fixture use case. |
| `core/role` | denied | Role authoring belongs in `access/_roles.yml`. |
| `core/permission` | denied | Entity grants belong in `access/<entity>.access.yml`. |
| `core/user`, `core/user-role`, `core/configuration` | allowed for now | Accounts, assignments, and global defaults can be explicit environment/demo setup data until Studio/admin tooling owns them. |
| Collection Entities | denied as standalone fixtures | Parent Entity fixtures own collection row data. |

Denied fixture files should fail with an error that names the correct authoring source.

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

`dygo db migrate` does not apply fixtures. Use `dygo fixture apply` for explicit fixture loading, or `dygo db prepare` for first-time environment preparation.

Fixtures do not delete Records, prune schema, run patches, track history, or expose HTTP endpoints.
