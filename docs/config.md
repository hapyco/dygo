# dygo Config

dygo project config lives at:

```txt
dygo.yml
```

The file is required for project-aware runtime commands.

Queue config lives at:

```txt
config/queues.yml
```

The queue file is required for Jobs, Schedules, `dygo doctor`, `dygo db migrate`, and `dygo worker`.

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

Queue config shape:

```yaml
queues:
  - name: default
    concurrency: 4
```

## Defaults

Omitted fields fall back to dygo defaults:

```txt
server.host = 127.0.0.1
server.port = 6790
database.driver = postgres
```

The database URL does not have a raw default. `database.url.secret` must name an encrypted secret.

Generated projects include `config/queues.yml` with the `default` queue and `concurrency: 4`.

## Validation

`server.host` must be non-empty after defaults are applied.

`server.port` must be between `1` and `65535`.

`database.driver` must be `postgres`.

`database.url.secret` must be a valid dygo secret name, such as `DATABASE_URL`.

Unknown YAML fields and duplicate YAML keys are rejected.

In `config/queues.yml`, queue names must be kebab-case and `concurrency` must be greater than `0`.

## Boundaries

dygo does not use Viper yet.

There are no `DYGO_` environment overrides yet.

There are no global config flags yet.

Secrets stay separate from runtime config. `dygo.yml` references secret names only; raw secret values must not live there.

`dygo dev` and `dygo serve` use this config to choose the HTTP bind address.

`dygo db check`, `dygo db create`, `dygo db drop`, `dygo db migrate`, `dygo db prune`, and `dygo db reset` use this config to find the encrypted database URL secret.

`dygo fixture apply`, `dygo fixture export`, `dygo setup`, and `dygo permission` also use this config when they need the selected environment's database URL.

`dygo db migrate`, `dygo doctor`, Job commands, and `dygo worker` use `config/queues.yml` to validate Job queue references and choose worker concurrency.
