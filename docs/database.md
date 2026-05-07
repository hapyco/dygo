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
configs/secrets/development.age.yaml
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

Drop or reset the configured database only with explicit confirmation:

```sh
go run ./cmd/dygo db drop --force
go run ./cmd/dygo db reset --force
```

`db reset` drops the database, creates it again, syncs metadata schema, and updates `db/schema.sql`.

All database commands default to `development` and support `--env staging` or `--env production`.

## Metadata Schema Sync

dygo has one normal schema input:

```txt
apps/*/entities/*.yml
  desired table schema
```

During `dygo migrate` and `dygo db prepare`, dygo loads every discovered App from `apps/` and `.dygo/apps/`, then creates or updates tables from each App's Entity metadata. Core is handled this way too: `apps/core/entities/*.yml` is the source for Core tables such as `app`, `entity`, `field`, `index`, `constraint`, `user`, `role`, `permission`, and `session`.

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

Safe operations include creating missing metadata tables, adding safe metadata columns, and adding missing metadata indexes or constraints. Composite indexes, composite unique constraints, and structured check constraints come from top-level Entity metadata. Unsafe or unsupported drift blocks `dygo migrate` before any operation is applied.

After the schema plan succeeds, `dygo migrate` upserts discovered Apps, Entities, Fields, Indexes, and Constraints into the Core metadata tables. This gives later runtime APIs and Studio a database-backed metadata registry while the YAML files remain the source of truth.

The current sync path is intentionally additive. Removing fields, renaming fields, renaming tables, destructive type changes, and unsafe required/unique/check/foreign-key changes are not inferred automatically. Those cases need an explicit app patch or, for plain metadata-orphaned objects, an explicit schema prune.

See [Explicit Patches](patches.md) for the design model that maps unsafe planner diagnostics to app-owned patch work.

There is no SQL migration file path or `migrations` table in this model. dygo compares metadata intent with the database shape and moves the database forward through metadata.

## Schema Prune

`dygo schema prune` is the explicit destructive cleanup command. It removes database objects that are present in PostgreSQL but no longer represented by loaded Entity metadata.

Preview the prune plan:

```sh
go run ./cmd/dygo schema prune
```

Apply the prune plan:

```sh
go run ./cmd/dygo schema prune --force
```

`dygo schema prune` defaults to `development` and supports `--env staging` or `--env production`.

Preview mode is the default and exits after printing the destructive plan. `--force` applies the plan in one transaction and updates `db/schema.sql` only after a successful prune.

Prune can drop extra constraints, extra non-constraint indexes, extra columns, and extra tables. It skips primary keys, not-null constraints, system columns, and indexes that back constraints. Generated SQL uses quoted identifiers and does not use `CASCADE`; hidden dependencies should fail instead of widening the blast radius.

Prune still refuses non-prunable blockers such as type drift, required drift, unsupported storage, missing system columns, and changed index or constraint definitions. Use app-owned patches for renames, backfills, type changes, and other unsafe transitions that need intent.

## Schema Snapshot

After a successful `dygo migrate`, `dygo schema prune --force`, `db prepare`, or `db reset`, dygo writes a Postgres-native schema snapshot:

```txt
db/schema.sql
```

The snapshot is generated with `pg_dump --schema-only --no-owner --no-privileges`. dygo looks for `pg_dump` in `PATH`, then checks Postgres.app's latest macOS path.

You can also manage the snapshot manually:

```sh
go run ./cmd/dygo db schema dump
```

`db/schema.sql` is generated output. It is useful for review and debugging, but app Entity metadata remains the source of truth.

## Boundaries

The schema sync foundation creates tables and persists metadata. The generic Record API and session auth can read and write DB-backed fields through that metadata. Permission enforcement, app lifecycle patches, child table storage, and destructive metadata transitions are still separate layers.
