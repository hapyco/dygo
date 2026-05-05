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

## Boundaries

The database foundation does not create schemas, run migrations, store Records, resolve permissions, or perform authentication yet.
