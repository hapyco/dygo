# Encrypted Secrets

dygo stores environment secrets as encrypted YAML files in the repository.

The encrypted files are safe to commit. The root `master.key` is local-only, ignored by git, and required to decrypt, edit, validate, or rotate secrets.

Database credentials use the same model. Local development should store `DATABASE_URL` in the `development` encrypted secrets file, not in plaintext config.

## Storage Model

Committed files:

```txt
configs/secrets/development.yml.age
configs/secrets/staging.yml.age
configs/secrets/production.yml.age
```

Ignored local files:

```txt
master.key
.dygo/secrets/tmp/
```

dygo uses `filippo.io/age` with one hybrid age identity in `master.key`. The public encryption recipient is derived from that key when dygo writes encrypted files, so separate recipient files are not needed.

Do not commit `master.key`. Anyone with that file can decrypt every environment secret file in the project.

## Environments

Use full environment names only:

- `development`
- `staging`
- `production`

Do not use short forms like `dev` or `prod`.

## Commands

Secret commands discover the dygo project root before reading or writing `master.key`, `configs/secrets/`, and `.dygo/secrets/tmp/`, so they can be run from nested directories inside a project.

`dygo new <name>` runs the same initialization for new projects and seeds `DATABASE_URL` in development secrets. Run `dygo secrets init` directly only for an existing project that does not have encrypted secrets yet.

Initialize secrets:

```sh
go run ./cmd/dygo secrets init
```

This creates `master.key` and missing encrypted files for `development`, `staging`, and `production`.

Edit development secrets:

```sh
go run ./cmd/dygo secrets edit
```

Without `--editor`, dygo opens `nano`.

Edit another environment:

```sh
go run ./cmd/dygo secrets edit --env staging
```

Choose an editor explicitly:

```sh
go run ./cmd/dygo secrets edit --editor nano
go run ./cmd/dygo secrets edit --env staging --editor "code --wait"
```

Validate secrets and config references:

```sh
go run ./cmd/dygo secrets validate
go run ./cmd/dygo secrets validate --env staging
```

Rotate the project master key:

```sh
go run ./cmd/dygo secrets rotate-key --confirm my-company/master.key
```

`rotate-key` requires the exact confirmation token `<project-name>/master.key`, where `<project-name>` comes from `dygo.yml`. It decrypts every environment with the existing `master.key`, stages and verifies the rotated key and encrypted files, replaces files in a recoverable order, and then re-encrypts every environment file for the new key.

## Decrypted Shape

The encrypted file decrypts to a plain YAML mapping, similar to Rails credentials:

```yaml
DATABASE_URL: postgres://local
STRIPE_SECRET_KEY: sk_test_example
```

Nested YAML is allowed:

```yaml
database:
  url: postgres://local
stripe:
  secret_key: sk_test_example
```

Secret references use either root keys or dot-separated paths:

```txt
DATABASE_URL
database.url
stripe.secret_key
```

Secret references must be non-empty and cannot contain empty path segments.

```txt
database..url
.DATABASE_URL
DATABASE_URL.
```

## Adding Database Credentials

Open the development secrets file:

```sh
go run ./cmd/dygo secrets edit
```

Add `DATABASE_URL`:

```yaml
DATABASE_URL: postgres://user:password@127.0.0.1:5432/dygo
```

Then validate:

```sh
go run ./cmd/dygo secrets validate
```

## Manifest References

Manifests should reference secret names, not raw values.

```yaml
env:
  DATABASE_URL:
    secret: DATABASE_URL
```

`dygo secrets validate --env <environment>` checks existing YAML under `configs/` for this shape and fails when a referenced secret is missing.

The project database config also references secrets:

```yaml
database:
  url:
    secret: DATABASE_URL
```

Nested references work the same way:

```yaml
database:
  url:
    secret: database.url
```

## Boundaries

Secrets can only be changed through `dygo secrets edit`. There are no public `set`, `get`, `show`, `list`, or `remove` commands.

`master.key` is intentionally project-local for now. Sharing it, backing it up, and injecting it into deployment environments are operational concerns outside this first implementation.

dygo still uses one local root key for development, staging, and production. Per-environment recipients, KMS, Vault, and other external production secret providers are future work.
