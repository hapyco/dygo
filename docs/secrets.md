# Encrypted Secrets

dygo stores environment secrets as encrypted YAML files in the repository.

The encrypted files are safe to commit. The root `master.key` is local-only, ignored by git, and required to decrypt, edit, validate, or rotate secrets.

Database credentials use the same model. Local development should store `DATABASE_URL` in the `development` encrypted secrets file, not in plaintext config.

## Storage Model

Committed files:

```txt
configs/secrets/development.age.yaml
configs/secrets/staging.age.yaml
configs/secrets/production.age.yaml
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
go run ./cmd/dygo secrets rotate-key
```

`rotate-key` decrypts every environment with the existing `master.key`, generates a new `master.key`, and re-encrypts every existing environment file.

## Decrypted Shape

The encrypted file decrypts to YAML:

```yaml
version: 1
environment: development
secrets:
  DATABASE_URL:
    value: postgres://local
    updated_at: 2026-05-03T08:00:00Z
```

Secret names must use uppercase letters, numbers, and underscores, starting with a letter.

Good:

```txt
DATABASE_URL
STRIPE_SECRET_KEY
S3_BUCKET_NAME
```

Bad:

```txt
database_url
dev-token
1PASSWORD
```

## Adding Database Credentials

Open the development secrets file:

```sh
go run ./cmd/dygo secrets edit
```

Add `DATABASE_URL` under `secrets`:

```yaml
version: 1
environment: development
secrets:
  DATABASE_URL:
    value: postgres://user:password@127.0.0.1:5432/dygo
    updated_at: 2026-05-03T08:00:00Z
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

## Boundaries

Secrets can only be changed through `dygo secrets edit`. There are no public `set`, `get`, `show`, `list`, or `remove` commands.

`master.key` is intentionally project-local for now. Sharing it, backing it up, and injecting it into deployment environments are operational concerns outside this first implementation.
