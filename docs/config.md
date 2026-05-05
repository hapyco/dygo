# dygo Config

dygo project config lives at:

```txt
configs/dygo.yaml
```

The file is required for project-aware runtime commands.

## Shape

```yaml
server:
  host: 127.0.0.1
  port: 6790
database:
  driver: postgres
  url:
    secret: DATABASE_URL
```

## Defaults

Omitted fields fall back to dygo defaults:

```txt
server.host = 127.0.0.1
server.port = 6790
database.driver = postgres
```

The database URL does not have a raw default. `database.url.secret` must name an encrypted secret.

## Validation

`server.host` must be non-empty after defaults are applied.

`server.port` must be between `1` and `65535`.

`database.driver` must be `postgres`.

`database.url.secret` must be a valid dygo secret name, such as `DATABASE_URL`.

Unknown YAML fields and duplicate YAML keys are rejected.

## Boundaries

dygo does not use Viper yet.

There are no `DYGO_` environment overrides yet.

There are no global config flags yet.

Secrets stay separate from runtime config. `configs/dygo.yaml` references secret names only; raw secret values must not live there.

`dygo serve` uses this config to choose the HTTP bind address.

`dygo db check` uses this config to find the encrypted database URL secret.

`dygo migrate status`, `dygo migrate up`, and `dygo migrate down` use this config to find the same encrypted database URL secret before running PostgreSQL migrations.
