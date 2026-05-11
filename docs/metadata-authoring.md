# Metadata Authoring

dygo includes JSON Schemas for the YAML files that define apps, Entities, fixtures, and project config.

These schemas help editors and agents suggest valid keys and catch obvious shape mistakes while writing metadata. They are not the runtime source of truth. The Go validators behind `dygo apps validate`, `dygo entities validate`, fixture apply, and config loading remain authoritative.

## Files

```txt
schemas/app.schema.json       app.yml manifests
schemas/entity.schema.json    Entity metadata
schemas/fixture.schema.json   app-owned fixtures
schemas/config.schema.json    configs/dygo.yaml
```

The repository also includes `.vscode/settings.json` with YAML schema mappings for the standard dygo paths:

```txt
apps/*/app.yml
apps/*/entities/*.yml
apps/*/fixtures/*.yml
configs/dygo.yaml
```

The same mappings include `.dygo/apps/*/...` for cached app metadata.

## Editor Setup

Install a YAML language server integration such as the VS Code YAML extension. After that, VS Code reads the committed workspace settings and applies the dygo schemas automatically.

The schemas cover the fixed metadata envelope and common enums such as built-in field types, naming strategies, check operators, and app paths. Dynamic fixture record fields are intentionally permissive because their valid names and values depend on Entity metadata.

## Validation

Use editor feedback for fast authoring, then run dygo validation before trusting metadata:

```sh
go run ./cmd/dygo apps validate
go run ./cmd/dygo entities validate
go run ./cmd/dygo fixtures apply
```
