# Database

dygo uses PostgreSQL as its platform database in v1.

Database credentials live in encrypted environment secrets. This includes local development.

## Config

Committed config references the database URL secret by name:

```yaml
database:
  driver: postgres
  url:
    secret: DATABASE_URL
```

Do not put a raw database URL in `dygo.yml`.

## Development Secret

Set the local development database URL by editing development secrets:

```sh
dygo secret edit
```

The value is encrypted into:

```txt
config/secrets/development.yml.age
```

The local `.dygo/secrets/master.key` decrypts and re-encrypts the file. Do not commit `.dygo/secrets/master.key`.

## Connectivity Check

Check the development database:

```sh
dygo db check
```

Check another environment:

```sh
dygo db check --env staging
dygo db check --env production
```

`dygo db check` decrypts the selected environment's `DATABASE_URL`, connects with the PostgreSQL runtime pool, pings the database, and prints a success message.

Connection errors must not print the raw database URL.

## Database Lifecycle

The database name comes from the selected environment's `DATABASE_URL`.

Create the configured database if it is missing:

```sh
dygo db create
```

Migrate an existing database. `db migrate` requires the configured database to exist, prints the full plan, prompts, then applies pre-sync patches, metadata schema sync, post-sync patches, and schema snapshot refresh.

```sh
dygo db migrate
```

```sh
dygo db migrate --dry-run
dygo db migrate --yes
```

`db migrate` does not create the database. If the database is missing, it reports that migration cannot continue.

Prepare a usable environment. `db prepare` is the non-destructive bootstrap command. It creates the configured database if missing, then runs migration, access apply, and fixture apply:

```sh
dygo db prepare
dygo db prepare --dry-run
dygo db prepare --yes
```

After preparation, create the first Administrator account before opening Studio:

```sh
dygo setup
```

Drop or reset the configured database only after reviewing the printed plan:

```sh
dygo db drop
dygo db reset
dygo db reset --dry-run
```

Use `--yes` for agents, scripts, or CI after the plan is acceptable:

```sh
dygo db drop --yes
dygo db reset --yes
```

`db reset` is the destructive rebuild command. It drops the database, creates it again, and runs the same preparation workflow as `db prepare`.

For staging or production destructive commands, pass `--force` in addition to the prompt or `--yes`:

```sh
dygo db drop --env staging --force
dygo db reset --env production --force --yes
```

dygo does not automate backups before destructive database operations yet. Take and verify any required backup before running destructive commands, especially for production.

All database commands default to `development` and support `--env staging` or `--env production`.

## Metadata Schema Sync

dygo has one normal schema input:

```txt
apps/*/entities/*/entity.yml
apps/*/entities/_collections/*.yml
apps/*/entities/_collections/*/entity.yml
  desired table schema
```

Runtime metadata also includes app-owned Jobs and Schedules:

```txt
apps/*/jobs/*/job.yml
apps/*/jobs/_schedules.yml
config/queues.yml
```

Job and Schedule files do not create per-Job tables. They sync into Core metadata records after the Entity schema plan succeeds.

During `dygo db migrate`, dygo loads every discovered App from `apps/` and `.dygo/apps/`, then creates or updates tables from each App's Entity metadata. Core is handled this way too: `apps/core/entities/` is the source for Core tables such as `app`, `activity`, `log`, `entity`, `field`, `index`, `constraint`, `job`, `job-execution`, `schedule`, `naming-series`, `patch-run`, `user`, `role`, `permission`, and `session`.

Preview metadata sync:

```sh
dygo db migrate --dry-run
```

Apply metadata sync:

```sh
dygo db migrate
```

`dygo db migrate` defaults to `development` and supports `--env staging` or `--env production`.

Before applying changes, dygo builds a schema plan from metadata and compares it with the live PostgreSQL `public` schema. The plan classifies safe operations separately from unsafe or unsupported drift.

Safe operations include creating missing metadata tables, adding safe metadata columns, and adding missing metadata indexes or constraints. Each metadata-backed table has system `id`, `name`, `created_at`, and `updated_at` columns. Collection child tables also include `parent_entity_id`, `parent_record_id`, `parent_field_id`, and 1-based `ordinal`, plus a unique constraint on `(parent_entity_id, parent_record_id, parent_field_id, ordinal)`. The Record `name` column is generated from Entity `name` metadata except for Single Entities, where it is fixed to the Entity key, and collection row Entities, where it is framework-owned random length `16`. Composite indexes and composite unique constraints come from top-level Entity metadata; single-field structured checks come from Field metadata. Unsafe or unsupported drift blocks `dygo db migrate` before any operation is applied.

Existing early-development databases created before system Record names may report a missing `name` system column. That is treated as unsupported drift because dygo cannot safely invent stable names for existing rows without an explicit patch or reset.

After the schema plan succeeds, `dygo db migrate` upserts discovered Apps, Entities, Fields, Indexes, Constraints, Jobs, and Schedules into the Core metadata tables. This gives runtime APIs, workers, and Studio a database-backed metadata registry while the YAML files remain the source of truth.

File-backed Jobs whose `job.yml` was removed are marked retired, not deleted, so old Job Executions remain inspectable. File-backed Schedules removed from `_schedules.yml` are also marked retired instead of being deleted.

App-owned access metadata and fixtures are not applied by `dygo db migrate`. Use `dygo access apply` and `dygo fixture apply` explicitly, or run `dygo db prepare` when preparing a full usable environment. See [Access](access.md) and [Fixtures](fixtures.md) for their file shapes.

The current sync path is intentionally additive. Removing fields, renaming fields, renaming tables, destructive type changes, and unsafe required/unique/check/foreign-key changes are not inferred automatically. Those cases need an explicit app patch or, for plain metadata-orphaned objects, an explicit schema prune.

See [Explicit Patches](patches.md) for the model that maps unsafe planner diagnostics to app-owned patch work.

Explicit patches are planned and applied around metadata sync as part of `dygo db migrate`:

```sh
dygo db migrate --dry-run
dygo db migrate
dygo db migrate --yes
```

Patch apply records a Core `patch-run` ledger row only after a patch succeeds. `dygo db migrate` refreshes `db/schema.sql` after schema changes. dygo does not automate backups before patches yet; take and verify backups before applying patches to production.

There is no SQL migration file path or `migrations` table in this model. dygo compares metadata intent with the database shape and moves the database forward through metadata.

## Schema Prune

`dygo db prune` is the explicit destructive cleanup command for metadata-orphaned objects in dygo's managed schema. Metadata is the source of truth for that schema.

Preview the prune plan:

```sh
dygo db prune --dry-run
```

Apply the prune plan:

```sh
dygo db prune
dygo db prune --yes
```

`dygo db prune` defaults to `development` and supports `--env staging` or `--env production`.

`--dry-run` exits after printing the destructive plan. Without `--dry-run`, `dygo db prune` prints the plan and prompts before writing. `--yes` skips the prompt. Protected environments require `--force`.

Prune can drop extra tables, constraints, non-constraint indexes, and columns that are present in the managed schema but absent from loaded Entity metadata. It skips primary keys, not-null constraints, system columns, and indexes that back constraints. Generated SQL uses quoted identifiers and does not use `CASCADE`; hidden dependencies should fail instead of widening the blast radius.

Prune still refuses non-prunable blockers such as type drift, required drift, unsupported storage, missing system columns, and changed index or constraint definitions. Use app-owned patches for renames, backfills, type changes, and other unsafe transitions that need intent. Do not keep long-lived unmanaged tables or columns in dygo's managed schema unless you expect prune to remove them; use another PostgreSQL schema or model them as metadata instead.

## Schema Snapshot

After a successful `dygo db migrate`, `dygo db prune`, or `dygo db reset`, dygo writes a Postgres-native schema snapshot:

```txt
db/schema.sql
```

The snapshot is generated with `pg_dump --schema-only --no-owner --no-privileges`. dygo looks for `pg_dump` in `PATH`, then checks Postgres.app's latest macOS path.

Manual schema dump commands are intentionally not public. `dygo doctor` reports whether the schema snapshot is missing or out of date.

`db/schema.sql` is generated output. It is useful for review and debugging, but app Entity metadata remains the source of truth.

## Boundaries

The schema sync foundation creates tables and persists metadata. The generic Record API, fixture runner, session auth, and Activity writer can read and write DB-backed fields through that metadata. Activity is append-only Record history for product timelines; compliance-grade audit logging, app lifecycle patches, fixture support for collection rows, and destructive metadata transitions are still separate layers.
