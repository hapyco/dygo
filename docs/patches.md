# Explicit Patches

dygo uses Entity metadata as the normal source for database shape.

`dygo migrate` is intentionally additive. It can create missing metadata tables, add safe columns, and add missing metadata indexes or constraints. It does not guess destructive intent.

Explicit patches are the escape hatch for changes that metadata cannot infer safely. `dygo schema prune` is the separate command for intentionally dropping metadata-orphaned database objects after review.

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
  0001_rename-customer-email-to-email.yml
  0002_backfill-deal-amounts.yml
```

Patch files are ordered deterministically by filename. The exact executable patch schema and runner are future work; examples in this document are design notes only and are not executable yet.

Patches are app lifecycle changes. They are not a second schema source, not SQL migration files, and not a replacement for Entity metadata.

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
| New foreign key | Existing Records may point at missing targets. | Repair references, create missing targets, or clear invalid values. |

## Planner Diagnostics

`dygo migrate plan` should remain the first command to run before applying schema changes. When it reports a blocker, use the diagnostic kind to decide the next action.

| Diagnostic kind | Meaning | Expected action |
|---|---|---|
| `extra-column` | The database has a column that Entity metadata no longer declares. | Use `dygo schema prune` for an intentional drop, write a patch to archive/rename/backfill first, or restore the field in metadata. |
| `extra-table` | The database has a table that no loaded Entity declares. | Use `dygo schema prune` for an intentional drop, write a patch to archive/rename/move data first, or restore the Entity metadata. |
| `column-type-drift` | The database column type differs from metadata. | Write a patch to cast/backfill safely, then update metadata. |
| `column-required-drift` | Database nullability differs from metadata. | Backfill or relax data intentionally, then rerun metadata sync. |
| `missing-required-column` | Metadata requires a new column without a safe default. | Add a safe default or write a patch that creates/backfills the column first. |
| `index-definition-drift` | An existing index name differs from metadata intent. | Write a patch to drop/recreate or rename the index intentionally. |
| `constraint-type-drift` | An existing constraint name has a different constraint type. | Write a patch to replace the constraint intentionally. |
| `constraint-definition-drift` | An existing constraint name has different columns or rules. | Write a patch to replace or rename the constraint intentionally. |
| `extra-index` | The database has a non-constraint index that metadata no longer declares. | Use `dygo schema prune` for an intentional drop, or restore the index in metadata. |
| `extra-constraint` | The database has a constraint that metadata no longer declares. | Use `dygo schema prune` for an intentional drop, or restore the constraint in metadata. |
| `unsupported-field-storage` | Metadata uses a field type whose storage is not implemented yet. | Wait for storage support or change metadata to supported field types. |

## Change Workflows

### Rename A Field

1. Add a patch that renames or copies the old column to the new column.
2. Update Entity metadata to use the new field name.
3. Run `dygo migrate plan`.
4. Run `dygo migrate` only when the plan has no blockers.

Illustrative design note:

```yaml
kind: patch
name: rename-customer-email-to-email
operation: rename-column
entity: customer
from: customer-email
to: email
```

### Remove A Field

1. Add a patch that archives, moves, or intentionally drops the old column.
2. Remove the field from Entity metadata.
3. Run `dygo migrate plan`.
4. Run `dygo migrate` after the plan is clean.

Illustrative design note:

```yaml
kind: patch
name: drop-legacy-status
operation: drop-column
entity: deal
field: legacy-status
```

### Change A Type

1. Add a patch that validates existing values.
2. Backfill or normalize values that cannot cast safely.
3. Change the field type in Entity metadata.
4. Run `dygo migrate plan`.
5. Run `dygo migrate` after the plan is clean.

Illustrative design note:

```yaml
kind: patch
name: convert-amount-to-decimal
operation: change-column-type
entity: deal
field: amount
to: decimal
```

## Boundaries

Patches do not bring back SQL migration folders.

Patches do not create a generic `migrations` table.

`dygo migrate` does not guess renames, drops, type changes, or destructive cleanup.

`dygo schema prune` removes metadata-orphaned columns, tables, indexes, and constraints only after an explicit preview. It does not guess renames, backfill data, convert types, run patches, or use `CASCADE`.

Patch execution tracking can later become Core app metadata or records. That tracking should describe app lifecycle work, not a separate framework migration system.

## Future Work

The patch runner is not implemented yet. Future tasks should define:

- the executable patch file schema
- patch validation and diagnostics
- patch ordering across app dependencies
- patch run tracking in Core metadata
- rollback policy
- `dygo patches` or app lifecycle CLI shape
