# Encrypted Secrets

dygo stores secrets as encrypted files in the repository.

The encrypted files are safe to commit. The private keys are local-only and ignored by git.

This gives dygo a simple default: secret names and encrypted state can be reviewed with the app, while raw values stay out of the repository.

## Storage Model

Committed files:

```txt
configs/secrets/development.age.yaml
configs/secrets/staging.age.yaml
configs/secrets/production.age.yaml
configs/secrets/recipients/development.txt
configs/secrets/recipients/staging.txt
configs/secrets/recipients/production.txt
```

Ignored local files:

```txt
.dygo/secrets/keys/development.agekey
.dygo/secrets/keys/staging.agekey
.dygo/secrets/keys/production.agekey
.dygo/secrets/tmp/
```

dygo uses `filippo.io/age` with hybrid age identities and ASCII-armored encrypted files.

Do not commit `.dygo/secrets/keys/`. Those files decrypt the committed secret files.

## Environments

Use full environment names only:

- `development`
- `staging`
- `production`

Do not use short forms like `dev` or `prod`.

## Commands

Secret commands discover the dygo project root before reading or writing `configs/secrets/` and `.dygo/secrets/`, so they can be run from nested directories inside a project.

Initialize an environment:

```sh
go run ./cmd/dygo secrets init --env development
```

Set a secret:

```sh
go run ./cmd/dygo secrets set --env development DATABASE_URL
go run ./cmd/dygo secrets set --env development DATABASE_URL --value postgres://local
go run ./cmd/dygo secrets set --env development DATABASE_URL --from-file ./database-url.txt
```

Read a secret for scripts:

```sh
go run ./cmd/dygo secrets get --env development DATABASE_URL
```

`get` prints the raw value. Use it intentionally.

Show a secret for humans:

```sh
go run ./cmd/dygo secrets show --env development DATABASE_URL
go run ./cmd/dygo secrets show --env development DATABASE_URL --reveal
```

`show` redacts by default. `--reveal` prints the raw value.

List secrets:

```sh
go run ./cmd/dygo secrets list --env development
```

`list` redacts values.

Edit the decrypted YAML in `$EDITOR`:

```sh
go run ./cmd/dygo secrets edit --env development
```

Remove a secret:

```sh
go run ./cmd/dygo secrets remove --env development DATABASE_URL
go run ./cmd/dygo secrets remove --env development DATABASE_URL --yes
```

The command is `remove`, not `rm`.

Validate secrets and config references:

```sh
go run ./cmd/dygo secrets validate --env development
```

Rotate an environment key:

```sh
go run ./cmd/dygo secrets rotate-key --env development
```

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

## Manifest References

Manifests should reference secret names, not raw values.

```yaml
env:
  DATABASE_URL:
    secret: DATABASE_URL
```

`dygo secrets validate --env <environment>` checks existing YAML under `configs/` for this shape and fails when a referenced secret is missing.

## Production Keys

The production key path exists for consistency:

```txt
.dygo/secrets/keys/production.agekey
```

That does not mean every developer should have the production key.

For production deployments, prefer injecting the private key through CI or deployment secret storage. Use the local ignored path only when a production key is intentionally present on a machine.

## Placeholder Files

The repository can contain empty encrypted files for each environment. They make the expected layout explicit and keep the first setup path predictable.

If placeholder files already exist and you intentionally need to replace the local key, recipient, and empty encrypted file, run:

```sh
go run ./cmd/dygo secrets init --env development --force
```
