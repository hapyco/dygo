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

During `dygo migrate` and `dygo db prepare`, dygo loads discovered Apps and Entities, then creates or updates tables from Entity metadata. Core is handled this way too: `apps/core/entities/*.yml` is the source for Core tables such as `apps`, `entities`, `fields`, `users`, `roles`, `permissions`, and `sessions`.

Run metadata sync:

```sh
go run ./cmd/dygo migrate
```

`dygo migrate` defaults to `development` and supports `--env staging` or `--env production`.

The current sync path is intentionally additive. Adding an Entity or field creates the corresponding table or column. Removing, renaming, destructive type changes, and unsafe required/unique changes are not inferred automatically. Those cases need an explicit app patch once the patch runner exists.

There is no SQL migration file path or `migrations` table in this model. dygo compares metadata intent with the database shape and moves the database forward through metadata.

## Schema Snapshot

After a successful `dygo migrate`, `db prepare`, or `db reset`, dygo writes a Postgres-native schema snapshot:

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

The schema sync foundation creates platform schema only. It does not store Records, resolve permissions, run app lifecycle patches, or perform authentication yet.
