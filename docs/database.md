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

`db prepare` creates the database if needed, applies pending migrations, and updates `db/schema.sql`.

Drop or reset the configured database only with explicit confirmation:

```sh
go run ./cmd/dygo db drop --force
go run ./cmd/dygo db reset --force
```

`db reset` drops the database, creates it again, applies migrations, and updates `db/schema.sql`.

Print the latest applied migration version:

```sh
go run ./cmd/dygo db version
```

All database commands default to `development` and support `--env staging` or `--env production`.

## Migrations

dygo uses paired raw SQL migrations:

```txt
20260505180000_create_core_tables.up.sql
20260505180000_create_core_tables.down.sql
```

Framework-owned migrations live under:

```txt
internal/db/migrations/
```

Project-owned migrations live under:

```txt
db/migrations/
```

Framework migrations run before project migrations. Each migration runs in a database transaction. Applied migrations are tracked in the database table `migrations` with scope, version, name, checksums, and applied timestamp.

Check migration status:

```sh
go run ./cmd/dygo migrate status
```

Apply pending migrations:

```sh
go run ./cmd/dygo migrate up
```

Roll back applied migrations:

```sh
go run ./cmd/dygo migrate down --steps 1
```

Roll back and reapply migrations while developing migration files:

```sh
go run ./cmd/dygo migrate redo --steps 1
```

All migration commands default to `development` and support `--env staging` or `--env production`.

## Schema Snapshot

After a successful `migrate up` or `migrate down`, dygo writes a Postgres-native schema snapshot:

```txt
db/schema.sql
```

The snapshot is generated with `pg_dump --schema-only --no-owner --no-privileges`. dygo looks for `pg_dump` in `PATH`, then checks Postgres.app's latest macOS path.

You can also manage the snapshot manually:

```sh
go run ./cmd/dygo db schema dump
go run ./cmd/dygo db schema load --force
```

`db schema load --force` replaces the database's `public` schema from `db/schema.sql`.

## Boundaries

The migration foundation creates platform schema only. It does not store Records, resolve permissions, run app lifecycle patches, or perform authentication yet.
