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

Do not put a raw database URL in `configs/dygo.yaml`.

## Development Secret

Set the local development database URL by editing development secrets:

```sh
go run ./cmd/dygo secrets edit
```

The value is encrypted into:

```txt
configs/secrets/development.yml.age
```

The local `master.key` at the project root decrypts and re-encrypts the file. Do not commit `master.key`.

## Connectivity Check

Check the development database:

```sh
go run ./cmd/dygo db check
```

Check another environment:

```sh
go run ./cmd/dygo db check --env staging
go run ./cmd/dygo db check --env production
```

`dygo db check` decrypts the selected environment's `DATABASE_URL`, connects with the PostgreSQL runtime pool, pings the database, and prints a success message.

Connection errors must not print the raw database URL.

## Database Lifecycle

The database name comes from the selected environment's `DATABASE_URL`.

Create the configured database if it is missing:

```sh
go run ./cmd/dygo db create
```

Prepare the database for running dygo:

```sh
go run ./cmd/dygo db prepare
```

`db prepare` creates the database if needed, syncs metadata schema, and updates `db/schema.sql`.

`db prepare` does not apply fixtures in v1. Apply seed Records explicitly after metadata sync:

```sh
go run ./cmd/dygo fixtures apply
```

After fixtures are applied, create the first Administrator account before opening Studio:

```sh
go run ./cmd/dygo setup admin
```

Drop or reset the configured database only with explicit confirmation:

```sh
go run ./cmd/dygo db drop --confirm development/dygo
go run ./cmd/dygo db reset --confirm development/dygo
```

`db reset` drops the database, creates it again, syncs metadata schema, and updates `db/schema.sql`.

The confirmation value is always `<environment>/<database-name>`, where the database name comes from the selected environment's encrypted `DATABASE_URL`. For staging or production, include the matching environment:

```sh
go run ./cmd/dygo db drop --env staging --confirm staging/dygo_staging
go run ./cmd/dygo db reset --env production --confirm production/dygo_prod
```

dygo does not automate backups before destructive database operations yet. Take and verify any required backup before running destructive commands, especially for production.

All database commands default to `development` and support `--env staging` or `--env production`.

## Metadata Schema Sync

dygo has one normal schema input:

```txt
apps/*/entities/*.yml
apps/*/entities/*/*.yml
  desired table schema
```

During `dygo migrate` and `dygo db prepare`, dygo loads every discovered App from `apps/` and `.dygo/apps/`, then creates or updates tables from each App's Entity metadata. Core is handled this way too: `apps/core/entities/` is the source for Core tables such as `app`, `activity`, `entity`, `field`, `index`, `constraint`, `naming-series`, `patch-run`, `user`, `role`, `permission`, and `session`.

Preview metadata sync:

```sh
go run ./cmd/dygo migrate plan
```

Apply metadata sync:

```sh
go run ./cmd/dygo migrate
```

`dygo migrate plan` and `dygo migrate` default to `development` and support `--env staging` or `--env production`.

Before applying changes, dygo builds a schema plan from metadata and compares it with the live PostgreSQL `public` schema. The plan classifies safe operations separately from unsafe or unsupported drift.

Safe operations include creating missing metadata tables, adding safe metadata columns, and adding missing metadata indexes or constraints. Each metadata-backed table has system `id`, `name`, `created_at`, and `updated_at` columns; the Record `name` column is generated from Entity `naming` metadata except for Single Entities, where it is fixed to the Entity name. Composite indexes and composite unique constraints come from top-level Entity metadata; single-field structured checks come from Field metadata. Unsafe or unsupported drift blocks `dygo migrate` before any operation is applied.

Existing early-development databases created before system Record names may report a missing `name` system column. That is treated as unsupported drift because dygo cannot safely invent stable names for existing rows without an explicit patch or reset.

After the schema plan succeeds, `dygo migrate` upserts discovered Apps, Entities, Fields, Indexes, and Constraints into the Core metadata tables. This gives later runtime APIs and Studio a database-backed metadata registry while the YAML files remain the source of truth.

App-owned fixtures can be applied after metadata sync. See [Fixtures](fixtures.md) for the `dygo fixtures apply` command and fixture file shape.

The current sync path is intentionally additive. Removing fields, renaming fields, renaming tables, destructive type changes, and unsafe required/unique/check changes are not inferred automatically. Those cases need an explicit app patch or, for plain metadata-orphaned objects, an explicit schema prune.

See [Explicit Patches](patches.md) for the model that maps unsafe planner diagnostics to app-owned patch work.

Explicit patches are planned and applied around metadata sync. They do not run automatically as part of `dygo migrate`:

```sh
go run ./cmd/dygo migrate plan
go run ./cmd/dygo patches plan --phase pre-sync
go run ./cmd/dygo patches apply --phase pre-sync --confirm development/dygo
go run ./cmd/dygo migrate
go run ./cmd/dygo patches plan --phase post-sync
go run ./cmd/dygo patches apply --phase post-sync --confirm development/dygo
```

Patch apply uses the same typed confirmation shape as destructive DB and schema commands. It applies one pending patch per transaction, records a Core `patch-run` ledger row only after that patch succeeds, and refreshes `db/schema.sql` after a successful apply. dygo does not automate backups before patches yet; take and verify backups before applying patches to production.

There is no SQL migration file path or `migrations` table in this model. dygo compares metadata intent with the database shape and moves the database forward through metadata.

## Schema Prune

`dygo schema prune` is the explicit destructive cleanup command. It removes database objects that are present in PostgreSQL but no longer represented by loaded Entity metadata.

Preview the prune plan:

```sh
go run ./cmd/dygo schema prune
```

Apply the prune plan:

```sh
go run ./cmd/dygo schema prune --confirm development/dygo
```

`dygo schema prune` defaults to `development` and supports `--env staging` or `--env production`.

Preview mode is the default and exits after printing the destructive plan. `--confirm <environment>/<database-name>` applies the plan in one transaction and updates `db/schema.sql` only after a successful prune.

Prune can drop extra constraints, extra non-constraint indexes, extra columns, and extra tables. It skips primary keys, not-null constraints, system columns, and indexes that back constraints. Generated SQL uses quoted identifiers and does not use `CASCADE`; hidden dependencies should fail instead of widening the blast radius.

Prune still refuses non-prunable blockers such as type drift, required drift, unsupported storage, missing system columns, and changed index or constraint definitions. Use app-owned patches for renames, backfills, type changes, and other unsafe transitions that need intent.

## Schema Snapshot

After a successful `dygo migrate`, confirmed `dygo schema prune`, `db prepare`, or confirmed `db reset`, dygo writes a Postgres-native schema snapshot:

```txt
db/schema.sql
```

The snapshot is generated with `pg_dump --schema-only --no-owner --no-privileges`. dygo looks for `pg_dump` in `PATH`, then checks Postgres.app's latest macOS path.

You can also manage the snapshot manually:

```sh
go run ./cmd/dygo db schema dump
go run ./cmd/dygo db schema check
```

`db schema check` compares the committed `db/schema.sql` with a fresh live schema dump for the selected environment. It does not rewrite the snapshot; if it reports drift, run `dygo db schema dump` after confirming the live schema is the intended one.

`db/schema.sql` is generated output. It is useful for review and debugging, but app Entity metadata remains the source of truth.

## Boundaries

The schema sync foundation creates tables and persists metadata. The generic Record API, fixture runner, session auth, and Activity writer can read and write DB-backed fields through that metadata. Activity is append-only Record history for product timelines; compliance-grade audit logging, app lifecycle patches, collection row storage, and destructive metadata transitions are still separate layers.
