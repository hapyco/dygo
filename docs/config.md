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
```

## Defaults

Omitted fields fall back to dygo defaults:

```txt
server.host = 127.0.0.1
server.port = 6790
```

An empty config file is valid and resolves to the defaults.

## Validation

`server.host` must be non-empty after defaults are applied.

`server.port` must be between `1` and `65535`.

Unknown YAML fields and duplicate YAML keys are rejected.

## Boundaries

dygo does not use Viper yet.

There are no `DYGO_` environment overrides yet.

There are no global config flags yet.

Secrets stay separate from runtime config. Future config schemas may reference secret names, but raw secret values should not live in `configs/dygo.yaml`.

`dygo serve` uses this config to choose the HTTP bind address.
