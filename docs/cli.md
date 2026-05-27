# dygo CLI

This document describes the dygo CLI surface after the CLI cleanup work. Deferred commands and future extensions are listed separately.

## Root

- `dygo` - Shows the root help for the metadata-driven dygo platform CLI.
- `dygo new <name>` - Creates a new dygo project skeleton.
- `dygo upgrade` - Upgrades the current project files, assets, and dependencies when the project dygo version differs from the installed dygo binary.
- `dygo upgrade --check` - Checks whether the current project needs an upgrade without planning or writing changes.
- `dygo upgrade --to <version>` - Plans or applies a project upgrade to a specific dygo version.
- `dygo upgrade --dry-run` - Prints the project upgrade plan without writing or prompting.
- `dygo upgrade --yes` - Applies the project upgrade without an interactive prompt.
- `dygo version` - Prints the dygo version.
- `dygo completion <shell>` - Generates shell completion scripts for bash, zsh, fish, or PowerShell.
- `dygo doctor` - Diagnoses current project readiness.
- `dygo setup` - Runs the first-run project setup flow, including Administrator bootstrap until the UI wizard owns it.
- `dygo dev` - Runs the local development experience with backend, Studio dev server, proxying, and diagnostics.
- `dygo serve` - Starts the dygo server.

`dygo doctor` checks root/config, secrets, database connectivity, schema snapshot state, app and Entity metadata, route conflicts, fixture validity, hook wiring, generated project runner, Studio assets, and first-run setup state.

## Database

- `dygo db` - Groups database lifecycle commands.
- `dygo db check` - Checks PostgreSQL connectivity for the configured environment.
- `dygo db create` - Creates the configured PostgreSQL database.
- `dygo db drop` - Prints the drop target, prompts interactively, then drops the configured PostgreSQL database.
- `dygo db drop --yes` - Drops the configured PostgreSQL database without an interactive prompt.
- `dygo db migrate` - Ensures the configured database exists, prints the full migration plan, prompts interactively, then applies pre-sync patches, metadata sync, post-sync patches, fixtures, and schema dump.
- `dygo db migrate --yes` - Applies the full migration workflow without an interactive prompt.
- `dygo db migrate --dry-run` - Prints the full migration plan without writing; if the database is missing, reports that it would be created and exits before full planning.
- `dygo db prune` - Prints the metadata-orphaned schema cleanup plan, prompts interactively, then removes approved objects.
- `dygo db prune --yes` - Prints the cleanup plan and applies it without an interactive prompt.
- `dygo db prune --dry-run` - Previews metadata-orphaned schema cleanup without writing.
- `dygo db reset` - Prints the reset target, prompts interactively, then drops, creates, and migrates the configured PostgreSQL database.
- `dygo db reset --yes` - Drops, creates, and migrates the configured PostgreSQL database without an interactive prompt.
- `dygo db reset --dry-run` - Prints the reset target and planned steps without writing.

## Apps

- `dygo app` - Groups dygo app commands.
- `dygo app list` - Lists discovered apps, versions, labels, and install locations.
- `dygo app validate` - Validates app manifests, app paths, dependencies, and reserved app metadata.

## Entities

- `dygo entity` - Groups dygo Entity commands.
- `dygo entity list` - Lists discovered normal, single, and collection Entities grouped by app.
- `dygo entity validate` - Validates Entity metadata, collection metadata, route slugs, link targets, collection targets, field names, and hook file conventions.
- `dygo entity show <app>/<entity>` - Prints resolved metadata for one Entity, including source path, kind, route slug, storage table, fields, links, collections, and naming.
- `dygo entity graph` - Prints Entity link and collection relationships across discovered apps.
- `dygo entity graph <app>` - Prints Entity relationships for one app.
- `dygo entity graph <app>/<entity>` - Prints incoming and outgoing relationships for one Entity.

## Fixtures

- `dygo fixture` - Groups app-owned fixture Record commands.
- `dygo fixture apply` - Prints the fixture apply plan, prompts interactively, then applies app-owned fixture Records.
- `dygo fixture apply --yes` - Applies app-owned fixture Records without an interactive prompt.
- `dygo fixture apply --dry-run` - Prints the fixture apply plan without writing or prompting.
- `dygo fixture validate` - Validates fixture files, match fields, dependencies, and references without connecting to the database when possible.
- `dygo fixture export <app>/<entity>` - Prints the fixture export plan, reports unresolved link dependencies, prompts interactively, then writes fixture files.
- `dygo fixture export <app>/<entity> --yes` - Exports selected Records without an interactive prompt.
- `dygo fixture export <app>/<entity> --include-links` - Exports selected Records and their linked fixture dependencies.
- `dygo fixture export <app>/<entity> --dry-run` - Prints the fixture export plan without writing or prompting.

## Hooks

- `dygo hook` - Groups hook inspection and maintenance commands.
- `dygo hook list` - Lists discovered hook packages, Entity hook files, runner wiring status, and compiled hook registrations when available.
- `dygo hook validate` - Validates hook file conventions, Entity references, duplicate compiled hook IDs when available, generated registrars, and runner wiring.
- `dygo hook sync` - Updates generated project runner wiring for discovered app hook packages without creating hook files.
- `dygo hook sync --dry-run` - Prints runner wiring changes without writing.
- `dygo hook sync --force` - Overwrites dygo-generated runner wiring only; custom runner files still fail.

## Generate

`dygo g` is an alias for `dygo generate`.

- `dygo generate` - Groups source scaffolding commands.
- `dygo generate app <app>` - Generates a new app skeleton.
- `dygo generate entity <app>/<entity>` - Generates the standard Entity bundle.
- `dygo generate collection <app>/<collection>` - Generates reusable collection row Entity metadata.
- `dygo generate hook <app>/<entity>` - Adds Entity hook scaffolding and project runner wiring to an existing Entity.
- `dygo generate fixture <app>/<entity>` - Adds a fixture skeleton to an existing Entity.
- `dygo generate test <app>/<entity>` - Adds Go test boilerplate for an existing Entity.

Generated files are valid boilerplate, not empty placeholders. Generators do not overwrite custom files. `--force` overwrites dygo-generated files only.

Collection generators create metadata only. Collection rows do not get fixture skeletons, route metadata, standalone permissions, or hooks by default; parent Entity fixtures and hooks own collection row usage. The intended collection file convention is `entities/_collections/<collection>.yml`.

`dygo generate entity` composes the narrower generators for the standard Entity bundle. In v1 it creates Entity metadata, hook scaffold, fixture skeleton, test boilerplate, and runner wiring unless skipped by flags.

- `dygo generate entity <app>/<entity> --dry-run` - Prints files that would be created or updated without writing.
- `dygo generate entity <app>/<entity> --force` - Overwrites dygo-generated files only; custom files still fail.
- `dygo generate entity <app>/<entity> --no-hook` - Skips hook scaffolding and runner wiring in the standard Entity bundle.
- `dygo generate entity <app>/<entity> --no-fixture` - Skips fixture skeleton creation in the standard Entity bundle.
- `dygo generate entity <app>/<entity> --no-test` - Skips Go test boilerplate in the standard Entity bundle.
- `dygo generate app <app> --dry-run` - Prints app skeleton files that would be created or updated without writing.
- `dygo generate app <app> --force` - Overwrites dygo-generated app skeleton files only; custom files still fail.
- `dygo generate collection <app>/<collection> --dry-run` - Prints collection metadata files that would be created or updated without writing.
- `dygo generate collection <app>/<collection> --force` - Overwrites dygo-generated collection metadata only; custom files still fail.
- `dygo generate hook <app>/<entity> --dry-run` - Prints hook scaffold and runner wiring changes without writing.
- `dygo generate hook <app>/<entity> --force` - Overwrites dygo-generated hook scaffolding only; custom hook files still fail.
- `dygo generate fixture <app>/<entity> --dry-run` - Prints fixture skeleton files that would be created or updated without writing.
- `dygo generate fixture <app>/<entity> --force` - Overwrites dygo-generated fixture skeletons only; custom files still fail.
- `dygo generate test <app>/<entity> --dry-run` - Prints Go test files that would be created or updated without writing.
- `dygo generate test <app>/<entity> --force` - Overwrites dygo-generated Go test files only; custom files still fail.

Generators are non-interactive by default. They write when there are no conflicts, skip unchanged generated files, and fail on custom-file conflicts with a clear message.

Generator boilerplate should live as embedded templates under `internal/generate/templates/`. The dygo binary should embed these templates at build time instead of reading template files from disk at runtime.

## Routes

- `dygo route` - Groups route registry inspection and validation commands.
- `dygo route list` - Lists routeable Entities, effective slugs, owners, and reserved root slugs.
- `dygo route validate` - Validates route slug conflicts, reserved root slugs, invalid slug syntax, and non-routeable collection usage.
- `dygo route resolve <path>` - Explains which Studio, API, or Entity route would handle a path.
- `dygo route resolve <method> <path>` - Explains which route, action, and permission a request would use.
- `dygo route reserved` - Lists framework-reserved route slugs.

## Permissions*

- `dygo permission` - Groups live permission inspection commands.
- `dygo permission list` - Lists live role permission grants from the configured database.
- `dygo permission list <app>/<entity>` - Lists live role permission grants for one Entity.
- `dygo permission check <app>/<entity> <action> --user <email-or-id>` - Checks live database permissions and prints `allow` or `deny`.
- `dygo permission check <app>/<entity> <action> --role <role>` - Checks whether one live role grants the action and prints `allow` or `deny`.
- `dygo permission explain <app>/<entity> <action> --user <email-or-id>` - Explains the live permission decision for one user.
- `dygo permission explain <app>/<entity> <action> --role <role>` - Explains the live permission decision for one role.

Permission commands default to `--env development` and read live Core permission Records from the configured database. The CLI should share the same internal permission verification methods used by the server/runtime so command answers match API and Studio authorization behavior.

## Secrets

- `dygo secret` - Groups encrypted dygo secret commands.
- `dygo secret init` - Initializes the root master key and encrypted environment secret files.
- `dygo secret get <name>` - Prints one decrypted development secret value to stdout for scripts.
- `dygo secret get <name> --env <environment>` - Prints one decrypted secret value for `development`, `staging`, or `production`.
- `dygo secret edit` - Opens decrypted development secrets in an editor, then validates and re-encrypts them.
- `dygo secret edit --env <environment>` - Opens decrypted secrets for `development`, `staging`, or `production`.
- `dygo secret validate` - Validates development secrets and config references.
- `dygo secret validate --env <environment>` - Validates encrypted secrets and config references for `development`, `staging`, or `production`.
- `dygo secret rotate-key` - Prints the rotation plan, prompts interactively, then rotates `.dygo/secrets/master.key` and re-encrypts all environment secret files.
- `dygo secret rotate-key --yes` - Rotates `.dygo/secrets/master.key` and re-encrypts all environment secret files without an interactive prompt.

Secret names support root keys and dot-separated YAML paths, such as `DATABASE_URL` or `database.url`. `dygo secret get` prints only the raw value to stdout; errors and diagnostics go to stderr.

## Deferred CLI Surface

- `dygo worker` - Defer until the durable job runtime is designed and implemented.
- `dygo scheduler` - Defer until schedule metadata and recurring job runtime exist.
- Global `--json` - Defer until dygo has a consistent output contract for command results, validation errors, dry-run plans, prompts, redaction, and streaming commands.
- Smart shell completions - Defer until command structure is implemented; start with filesystem/static completions for `--env`, `<app>`, `<app>/<entity>`, hook events, and completion shells.

## Out Of Band Binary Updates

- `curl -fsSL https://dygo.dev/install | sh` - Installs or updates the dygo binary outside the workspace CLI.
- `brew upgrade dygo` - Future package-manager-owned binary update path.
- `go install github.com/hapyco/dygo/cmd/dygo@latest` - Future Go toolchain-owned binary update path if dygo supports it.

Normal update flow:

```sh
curl -fsSL https://dygo.dev/install | sh
dygo upgrade
```

Until `dygo.dev/install` is wired:

```sh
curl -fsSL https://raw.githubusercontent.com/hapyco/dygo/main/scripts/install.sh | sh
dygo upgrade
```

## Removal And Replacement Notes

- Remove `dygo schema` - Prune moves to `dygo db prune`, and schema snapshot checking moves into `dygo doctor`.
- Replace `dygo schema prune` with `dygo db prune` - Prune is a database cleanup action; help text should clarify that it removes metadata-orphaned schema objects.
- Replace `dygo migrate` with `dygo db migrate` - Metadata schema sync is a database operation and belongs under the database command group.
- Replace `dygo migrate plan` with `dygo db migrate --dry-run` - Planning is the same migration workflow in preview mode, not a separate command group.
- Remove `dygo db prepare` - It only combines `dygo db create` and metadata sync; the explicit flow is clearer.
- Remove `dygo setup admin` - First-run setup should live behind `dygo setup`; the future path is a UI wizard rather than many setup subcommands.
- Remove public `dygo db schema` commands - `dygo db migrate` and `dygo db prune` refresh `db/schema.sql`, and `dygo doctor` should report whether the schema snapshot is missing or out of date.
- Remove public `dygo db schema dump` - Manual dumping can hide drift by making the snapshot match an unintended database state.
- Remove public `dygo patch` commands - Patches are part of the `dygo db migrate` workflow. Add direct patch controls later only if debugging or recovery needs a lower-level expert command.
- Replace plural command groups with singular command groups - `dygo apps` becomes `dygo app`, `dygo entities` becomes `dygo entity`, `dygo fixtures` becomes `dygo fixture`, `dygo hooks` becomes `dygo hook`, and `dygo secrets` becomes `dygo secret`.
- Replace `dygo hooks generate <app> <entity>` with `dygo generate hook <app>/<entity>` - Source scaffolding belongs under `dygo generate`, and app/entity targets use slash identity.
- Use `dygo generate` as the home for source scaffolding - Avoid per-resource `generate` subcommands such as `dygo hook generate`.
- Do not add `dygo generate resource` - `dygo generate entity` owns the standard Entity bundle.
- Keep explicit `dygo fixture` commands - `dygo db migrate` applies fixtures during the normal app-state workflow, while `dygo fixture apply`, `validate`, and `export` support app-author tooling and debugging.
- Remove `dygo upgrade --cli-only` - Binary updates are handled out of band through installer, package manager, or Go toolchain.
- Remove `dygo upgrade --install-dir` - Install location belongs to the installer, not the project upgrade command.
- Remove `dygo upgrade --project-only` - `dygo upgrade` is project-only by definition.
- Remove `dygo secret init --force` - Key replacement belongs to `dygo secret rotate-key`; init should only create missing secret infrastructure.
- Move `dygo serve --studio-dev-url` to `dygo dev` if the override is still needed - Studio/Vite proxying is local development orchestration, not runtime serving.

## Environment Safety Rule

- `--env` defaults to `development`.
- Runtime and database write commands print the plan and prompt interactively by default when the action benefits from review.
- `--yes` skips runtime write prompts for agents, scripts, and CI.
- `--dry-run` prints the same runtime write plan and exits without writing or prompting.
- Generator and scaffold writes are non-interactive by default: they write when there are no conflicts, support `--dry-run` for previews, support `--force` for dygo-generated files only, and fail on custom-file conflicts.
- Destructive commands can run in `development`.
- Protected environments such as `staging` and `production` block destructive commands unless `--force` is passed.
- Read-only commands such as `--dry-run`, `check`, and validation commands never need `--force`.
