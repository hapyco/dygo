# Explicit Patches

dygo uses Entity metadata as the normal source for database shape.

`dygo migrate` is intentionally additive. It can create missing metadata tables, add safe columns, and add missing metadata indexes or constraints. It does not guess destructive intent.

Explicit patches are the escape hatch for changes that metadata cannot infer safely. `dygo schema prune` is the separate command for intentionally dropping metadata-orphaned database objects after review.

This document describes the v1 patch runner.

## Core Rule

Metadata declares the desired current shape.

Patches explain unsafe transitions.

When the schema planner sees drift that could destroy data, rename data, reinterpret data, or fail against existing Records, it blocks `dygo migrate`. The app owner must write an explicit patch before the metadata-only path can continue.

This keeps the framework dogfooding its metadata model while still giving builders a controlled way to handle real production changes.

## Ownership

Patches belong to Apps.

They live under the app's manifest-defined `patches` path:

```txt
apps/crm/patches/
  0001_rename_customer_email_to_email.yml
  0002_backfill_deal_amounts.yml
```

Patch files are ordered deterministically by app dependency order, then app name, then filename. App dependencies always run before dependents. Apps at the same dependency level run by app name. Files inside one app run lexicographically by filename.

Patches are app lifecycle changes. They are not a second schema source, not SQL migration files, and not a replacement for Entity metadata.

## Patch File Shape

Patch files are YAML documents:

```yaml
kind: patch
version: 1
id: 0001_rename_customer_email_to_email
phase: pre-sync
description: Rename legacy customer email field before metadata sync.
operations:
  - type: rename-field
    entity: customer
    from: customer-email
    to: email
```

Required top-level fields:

| Field | Meaning |
|---|---|
| `kind` | Must be `patch`. |
| `version` | Patch schema version. v1 uses `1`. |
| `id` | Stable patch id. It must match the filename without `.yml` or `.yaml`. |
| `phase` | `pre-sync` or `post-sync`. |
| `description` | Human explanation shown in plans and logs. |
| `operations` | Ordered operations to run inside the patch transaction. |

Patch ids are scoped to the owning app. Do not edit an applied patch. If the database needs another change, add a new patch file.

## Phases

`pre-sync` patches run before metadata schema sync.

Use `pre-sync` when the patch needs the old database shape to still exist, for example:

- rename a field storage column before metadata switches to the new field
- rename an Entity table before metadata points at the new table
- backfill values before a field becomes required
- deduplicate Records before a unique constraint is declared

`post-sync` patches run after metadata schema sync.

Use `post-sync` when the patch needs the new metadata-backed shape, for example:

- populate a newly added optional field
- create app data that depends on new tables
- repair data after additive schema sync made the new columns available

v1 keeps patch execution explicit. `dygo migrate` does not run patches automatically.

Recommended workflow:

```sh
dygo migrate plan
dygo patches plan --phase pre-sync
dygo patches apply --phase pre-sync --confirm development/dygo
dygo migrate
dygo patches plan --phase post-sync
dygo patches apply --phase post-sync --confirm development/dygo
```

## Structured Operations

Structured operations are the preferred patch API. They preserve dygo's metadata language and let the runner validate app, Entity, and Field intent before it touches PostgreSQL.

v1 structured operation names:

| Operation | Purpose |
|---|---|
| `rename-field` | Rename one stored Field column for an Entity in the owning app. |
| `rename-entity` | Rename one Entity storage table in the owning app. |
| `copy-field` | Copy values from one stored Field to another. |
| `backfill-field` | Fill missing or matching values for one stored Field. |
| `drop-field` | Intentionally drop one stored Field column after any archive/copy step. |
| `change-field-type` | Cast or rewrite one stored Field column with an explicit expression. |
| `sql` | Escape hatch for app-specific SQL that structured operations cannot express. |

Structured operations target the owning app by default. Cross-app structured patches are not part of v1. If an app needs to repair data across app boundaries, use an explicit `sql` operation with a clear `reason`.

### Rename Field

```yaml
operations:
  - type: rename-field
    entity: customer
    from: customer-email
    to: email
```

The runner maps Entity and Field names to the actual storage table and column names. The patch author writes dygo metadata names, not raw PostgreSQL identifiers.

### Rename Entity

```yaml
operations:
  - type: rename-entity
    from: customer
    to: account
```

The runner maps app-scoped Entity keys to storage table names.

### Copy Field

```yaml
operations:
  - type: copy-field
    entity: deal
    from: legacy-amount
    to: amount
    when:
      to-is-null: true
```

`copy-field` is for preserving data before metadata or prune removes the old Field.

### Backfill Field

```yaml
operations:
  - type: backfill-field
    entity: deal
    field: status
    value: open
    when:
      field-is-null: true
```

`backfill-field` is for safe scalar fills before enabling required, unique, or check metadata.

### Drop Field

```yaml
operations:
  - type: drop-field
    entity: customer
    field: legacy-status
```

Use `drop-field` only after the patch has archived, copied, or intentionally abandoned the old data. Plain metadata-orphaned cleanup should usually use `dygo schema prune` instead.

### Change Field Type

```yaml
operations:
  - type: change-field-type
    entity: deal
    field: amount
    to: decimal
    using: nullif(trim(amount), '')::numeric
```

`change-field-type` requires an explicit `using` expression. dygo should not guess how to reinterpret existing values.

## SQL Escape Hatch

Use `sql` only when structured operations cannot express the transition:

```yaml
operations:
  - type: sql
    name: normalize-emails
    reason: Existing emails need cleanup before a unique constraint can be added.
    statement: |
      UPDATE "crm_customer"
      SET "email" = lower(trim("email"))
      WHERE "email" IS NOT NULL;
```

Rules for SQL operations:

- `name`, `reason`, and `statement` are required.
- The statement runs inside the patch transaction.
- Transaction control such as `BEGIN`, `COMMIT`, and `ROLLBACK` is rejected.
- Database-level operations such as `CREATE DATABASE`, `DROP DATABASE`, and `ALTER SYSTEM` are rejected.
- The statement is printed in `dygo patches plan`.
- The patch checksum changes if the SQL text changes.

SQL exists for real production needs, but it is a reviewed escape hatch, not the normal patch API.

## Runner Semantics

v1 applies one patch per transaction.

For each pending patch, the runner:

1. Load and validate the patch file.
2. Check the patch ledger for an existing applied record.
3. Refuse to continue if the same app/id has an applied record with a different checksum.
4. Begin a transaction.
5. Run operations in file order.
6. Insert the patch ledger row.
7. Commit the transaction.

If any operation fails, the transaction rolls back and no successful ledger row is written. The same patch can be retried after the author fixes the cause.

Patches should be written to tolerate retries where practical, but v1 does not require every operation to be globally idempotent. Structured operations should validate the expected before/after shape and fail clearly when the database is not in the expected state.

After a successful apply that ran at least one patch, dygo refreshes `db/schema.sql`. If the snapshot refresh fails after patches are committed, dygo reports the snapshot error; it does not roll back already committed patches.

dygo does not automate backups before patches yet. Take and verify backups before applying patches to production.

## Patch Ledger

Applied patches are tracked in Core metadata, not in a generic SQL migration table.

The Core `patch-run` record stores:

| Field | Meaning |
|---|---|
| `app` | Owning app name. |
| `patch-id` | Patch id from the file. |
| `path` | Repository-relative patch file path. |
| `phase` | `pre-sync` or `post-sync`. |
| `checksum` | SHA-256 of the exact patch file bytes. |
| `applied-at` | Timestamp after operations complete. |
| `dygo-version` | dygo version that applied the patch, when available. |

The ledger records successful patches. Failed patches are reported by the command and are not recorded as applied.

## CLI Shape

v1 patch commands:

```sh
dygo patches plan --phase pre-sync
dygo patches apply --phase pre-sync --confirm development/dygo
dygo patches plan --phase post-sync
dygo patches apply --phase post-sync --confirm development/dygo
```

All commands default to `development` and support `--env staging` or `--env production`.

`dygo patches plan`:

- discover patch files
- validate schema and ordering
- compare patches with the ledger
- print pending and applied patches
- show SQL escape hatch statements
- fail on duplicate ids or checksum mismatches
- perform no database writes

`dygo patches apply`:

- require `--phase`
- require typed confirmation using `<environment>/<database-name>`
- refuse checksum mismatches
- run only pending patches for that phase
- stop at the first failed patch
- print the applied patch ids

## When To Use A Patch

Use a patch when the change cannot be proven safe from metadata alone.

| Change | Why metadata cannot infer it | Patch responsibility |
|---|---|---|
| Field rename | Removing one field and adding another could be a rename or two unrelated changes. | Rename the storage column or copy data, then update metadata. |
| Field removal | Dropping a column destroys data. | Archive, copy, or intentionally drop the old storage. |
| Entity/table rename | A new Entity `name` could be a new table or a renamed table. | Rename the table or move data, then update metadata. |
| Type change | Existing values may not cast cleanly. | Validate, clean, cast, or backfill data before metadata expects the new type. |
| Required field without safe default | Existing Records may not have a value. | Backfill values, then mark the field required. |
| New unique constraint | Existing Records may contain duplicates. | Deduplicate or merge Records, then add uniqueness metadata. |
| New check constraint | Existing Records may violate the rule. | Clean or normalize values, then add the check metadata. |
| New strict relationship rule | Existing Records may point at missing targets. | Repair references, create missing targets, or clear invalid values before enabling the rule. |

## Planner Diagnostics

`dygo migrate plan` should remain the first command to run before applying schema changes. When it reports a blocker, use the diagnostic kind to decide the next action.

| Diagnostic kind | Meaning | Expected action |
|---|---|---|
| `extra-column` | The database has a column that Entity metadata no longer declares. | Use `dygo schema prune` only when the column is known to be dygo-owned, write a patch to archive/rename/backfill first, or restore the field in metadata. |
| `extra-table` | The database has a table that no loaded Entity declares. | Use `dygo schema prune` only when the table is known to be dygo-owned, write a patch or perform a manual reviewed cleanup, or restore the Entity metadata. Unknown tables are not automatic prune candidates. |
| `column-type-drift` | The database column type differs from metadata. | Write a patch to cast/backfill safely, then update metadata. |
| `column-required-drift` | Database nullability differs from metadata. | Backfill or relax data intentionally, then rerun metadata sync. |
| `missing-required-column` | Metadata requires a new column without a safe default. | Add a safe default or write a patch that creates/backfills the column first. |
| `index-definition-drift` | An existing index name differs from metadata intent. | Write a patch to drop/recreate or rename the index intentionally. |
| `constraint-type-drift` | An existing constraint name has a different constraint type. | Write a patch to replace the constraint intentionally. |
| `constraint-definition-drift` | An existing constraint name has different columns or rules. | Write a patch to replace or rename the constraint intentionally. |
| `extra-index` | The database has a non-constraint index that metadata no longer declares. | Use `dygo schema prune` only when the index is known to be dygo-owned, or restore the index in metadata. |
| `extra-constraint` | The database has a constraint that metadata no longer declares. | Use `dygo schema prune` only when the constraint is known to be dygo-owned, or restore the constraint in metadata. |
| `unsupported-field-storage` | Metadata uses a field type whose storage is not implemented yet. | Wait for storage support or change metadata to supported field types. |

## Boundaries

Patches do not bring back SQL migration folders.

Patches do not create a generic `migrations` table.

`dygo migrate` does not guess renames, drops, type changes, or destructive cleanup.

`dygo schema prune` removes metadata-orphaned tables, columns, indexes, and constraints only when the live object is known to be dygo-owned and only after an explicit preview. It blocks unknown public-schema objects instead of assuming dygo owns them. It does not guess renames, backfill data, convert types, run patches, or use `CASCADE`.

Patch execution tracking is Core app metadata or records. That tracking describes app lifecycle work, not a separate framework migration system.

## Implementation Status

Implemented v1 slices:

- add Core patch ledger records
- add patch discovery and YAML validation
- add structured operation planners
- add SQL escape hatch validation
- add `dygo patches plan`
- add `dygo patches apply`
- add docs and tests for pre-sync and post-sync workflows
