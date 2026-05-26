# CLI And Directory Refactor Plan

This is the one-PR implementation plan for making the current codebase match the target CLI and directory shape.

Primary target references:

- `docs/cli.md` - Target command surface, flags, removal notes, and safety rules.
- `docs/dir.md` - Target generated project, runtime, and framework directory shape.

The PR should implement the committed target direction, not reopen naming decisions. Deferred items in `docs/cli.md` stay deferred.

## Success Definition

- `dygo --help` and subcommand help expose the non-deferred command surface from `docs/cli.md`.
- Generated projects from `dygo new <name>` follow the Project shape in `docs/dir.md`.
- App metadata discovery, validation, fixtures, hooks, generators, and doctor understand the finalized directory shape.
- Existing framework apps still load correctly after any required metadata moves.
- Old public command paths listed under Removal And Replacement Notes are removed or replaced.
- Runtime/database writes follow the interactive, `--yes`, and `--dry-run` rules in `docs/cli.md`.
- Tests cover command registration, target directory discovery, generated project output, and important write-safety behavior.

## Current State Snapshot

Already useful and should be reused:

- Root command construction: `internal/cli/root.go`.
- Project root discovery with `dygo.yml`: `internal/project/root.go`.
- Project generator foundation: `internal/projectgen/projectgen.go`.
- App manifest discovery and validation: `internal/app/manifest/manifest.go`, `internal/app/registry/registry.go`.
- Entity metadata loader and validator: `internal/entity/schema/schema.go`, `internal/entity/catalog/catalog.go`.
- Database lifecycle, metadata sync, patch, prune, and schema snapshot runtime: `internal/db/`.
- Fixture apply runtime: `internal/fixtures/fixtures.go`.
- Hook generator and runner wiring foundation: `internal/hookgen/hookgen.go`.
- Permission engine foundation: `internal/permissions/`.
- Secrets store supports nested dot-path reads internally: `internal/secrets/secrets.go`.
- Upgrade project runtime exists, but still includes binary self-upgrade concerns: `internal/upgrade/`.

Known mismatches against the target:

- CLI still registers old root-level or plural commands: `migrate`, `patches`, `schema`, `apps`, `entities`, `fixtures`, `hooks`, and `secrets`.
- `dygo db prepare`, `dygo db schema check`, and `dygo db schema dump` still exist publicly.
- `dygo setup admin` still exists; target is a single `dygo setup` flow.
- `dygo serve` still owns Studio dev proxying via `--studio-dev-url`; target moves local orchestration to `dygo dev`.
- `dygo upgrade` still has binary self-upgrade flags and behavior.
- Current generated projects use `configs/dygo.yaml`, `configs/secrets/`, `var/`, app-level `fixtures/`, app-level `hooks/`, app-level `permissions/`, and app-level `patches/`.
- Target generated projects use root `dygo.yml`, `config/`, `.dygo/`, and Entity bundles under `apps/<app>/entities/<entity>/`.
- Current Entity loader supports `entities/<entity>.yml`, `entities/<entity>/<entity>.yml`, and `entities/collections/<collection>.yml`.
- Target Entity shape is `entities/<entity>/entity.yml` and `entities/_collections/<collection>.yml` or `entities/_collections/<collection>/entity.yml`.
- Current fixture discovery loads app-level `fixtures/<entity>.yml`; target puts Entity fixtures in `entities/<entity>/fixtures.yml`.
- Current hook scaffolding creates `hooks/<entity>.go`; target puts generated Entity hook scaffolds in `entities/<entity>/hooks.go`.
- Route, permission, generate, entity graph/show, fixture export, hook list/validate/sync, and secret get commands are missing or incomplete.

## Implementation Principles

- Keep this as one PR, but implement in reviewable commits by layer.
- Add central shape constants before changing many packages. Avoid scattering strings like `entities`, `_collections`, `entity.yml`, `fixtures.yml`, `hooks.go`, `dygo.yml`, and `config/secrets`.
- Do not keep compatibility readers for unreleased shapes; remove old readers and old public command paths as the final shape lands.
- Do not add global `--json` or smart completions in this PR. They are deferred in `docs/cli.md`.
- Do not implement worker or scheduler runtimes in this PR. They are deferred in `docs/cli.md`.
- Keep command output plain ASCII and stable for humans and agents.
- Use Cobra `RunE`, explicit `Args`, injected stdio, and project root discovery patterns already used in `internal/cli`.

## Breaking-Change Policy For This PR

- Generated output must use the canonical shape from `docs/dir.md`.
- Public docs must describe only the canonical shape.
- First-party metadata should be migrated to canonical paths in this PR.
- Old generated-project layouts and old public command paths should be removed instead of preserved.
- If old path forms are encountered, validation should fail with a clear final-shape error instead of loading them.

## Phase 1 - Centralize Target Shape

TODO:

- Add `internal/shape` for path and target parsing constants.
- Define canonical project paths:
  - `dygo.yml`
  - `config/secrets`
  - `.dygo/apps/studio`
  - `.dygo/files`
  - `.dygo/logs`
  - `.dygo/tmp`
  - `.dygo/secrets`
  - `db/schema.sql`
  - `apps/<app>/app.yml`
- Define canonical app paths:
  - `entities/<entity>/entity.yml`
  - `entities/<entity>/fixtures.yml`
  - `entities/<entity>/hooks.go`
  - `entities/<entity>/permissions.yml`
  - `entities/<entity>/views.yml`
  - `entities/_collections/<collection>.yml`
  - `entities/_collections/<collection>/entity.yml`
  - `jobs/<job>/job.yml`
  - `jobs/<job>/run.go`
  - `jobs/_schedules.yml`
  - `pages/<page>/page.yml`
  - `reports/<report>.yml`
  - `reports/<report>/report.yml`
  - `roles.yml`
- Add a shared parser for slash targets:
  - `<app>/<entity>`
  - `<app>/<collection>`
  - reject missing slash, empty segments, non-kebab names, and extra segments.
- Use the shared parser in future `generate`, `fixture export`, `entity show`, `entity graph`, `permission`, and hook commands.

References:

- `docs/dir.md`
- `docs/cli.md`
- `internal/project/root.go`
- `internal/app/manifest/manifest.go`
- `internal/entity/schema/schema.go`
- `internal/entity/catalog/catalog.go`

## Phase 2 - Align Project Config And Generated Project Shape

TODO:

- Implement `dygo.yml` as both root marker and project config source.
- Move runtime config loading from `configs/dygo.yaml` to root `dygo.yml`.
- Remove `configs/dygo.yaml` loading; generated output and the framework repo must use root `dygo.yml`.
- Update `config.FilePath` and `config.Load` in `internal/config/config.go`.
- Update `projectgen.Generate` to write the canonical target tree from `docs/dir.md`.
- Replace generated `configs/` and `var/` paths with `config/` and `.dygo/` paths.
- Update generated `.gitignore` for the new `.dygo/` and secret-key layout.
- Update generated README and printed next steps to:
  - `dygo db migrate`
  - `dygo setup`
  - `dygo dev`
- Remove old printed first-run commands:
  - `dygo db prepare`
  - `dygo fixtures apply`
  - `dygo setup admin`
  - `dygo serve` as the default development command.
- Update `internal/projectgen/projectgen_test.go`.
- Update docs that still mention old first-run paths.

References:

- `docs/dir.md`
- `docs/cli.md` Root, Database, Setup, and Removal And Replacement Notes
- `internal/projectgen/projectgen.go`
- `internal/projectgen/projectgen_test.go`
- `internal/config/config.go`
- `docs/config.md`
- `docs/database.md`
- `docs/secrets.md`

## Phase 3 - Align Entity And Collection Discovery

TODO:

- Update Entity discovery to treat `entities/<entity>/entity.yml` as the canonical normal Entity bundle.
- Update collection discovery to use `_collections`, not `collections`.
- Support both collection file forms:
  - `entities/_collections/<collection>.yml`
  - `entities/_collections/<collection>/entity.yml`
- Update path-derived naming so `entity.yml` derives the Entity name from the parent folder.
- Remove old flat Entity readers; direct `entities/*.yml`, `entities/<entity>/<entity>.yml`, and `entities/collections/` paths should fail with final-shape guidance.
- Migrate first-party Core metadata under `apps/core/entities/` to canonical bundle paths.
- Update catalog validation messages to mention `_collections` and `entity.yml`.
- Update tests in `internal/entity/catalog/` and `internal/entity/schema/`.
- Update stale docs that mention `entities/collections/` or `<entity>/<entity>.yml`.

References:

- `docs/dir.md`
- `docs/cli.md` Generate and Entities sections
- `internal/entity/catalog/catalog.go`
- `internal/entity/catalog/catalog_test.go`
- `internal/entity/schema/schema.go`
- `docs/entity-metadata.md`
- `docs/metadata-authoring.md`

## Phase 4 - Align Fixture Storage And Commands

TODO:

- Keep the existing fixture apply engine, but update discovery to find canonical Entity-bundle fixtures:
  - `apps/<app>/entities/<entity>/fixtures.yml`
- Remove app-level fixture discovery; generate, export, and load fixtures only from `entities/<entity>/fixtures.yml`.
- Add `dygo fixture` singular command group.
- Add `dygo fixture validate`.
- Add `dygo fixture apply`.
- Add `dygo fixture apply --yes`.
- Add `dygo fixture apply --dry-run`.
- Add `dygo fixture export <app>/<entity>`.
- Add `dygo fixture export <app>/<entity> --yes`.
- Add `dygo fixture export <app>/<entity> --include-links`.
- Add `dygo fixture export <app>/<entity> --dry-run`.
- Ensure normal `fixture apply` prints a plan and prompts before writes.
- Ensure `fixture apply --dry-run` never prompts or writes.
- Ensure fixture validation checks files, match fields, dependencies, references, and collection limitations.
- Keep the known collection fixture TODO visible until collection row fixture upsert is implemented.

References:

- `docs/cli.md` Fixtures section
- `docs/dir.md`
- `internal/fixtures/fixtures.go`
- `internal/fixtures/fixtures_test.go`
- `internal/cli/fixtures.go`
- `docs/fixtures.md`

## Phase 5 - Reshape Database And Migration CLI

TODO:

- Remove public top-level `dygo migrate`.
- Add `dygo db migrate`.
- Add `dygo db migrate --yes`.
- Add `dygo db migrate --dry-run`.
- Make `dygo db migrate` print the full plan, prompt, then apply:
  - pre-sync patches
  - metadata schema sync
  - post-sync patches
  - fixtures
  - schema dump
- Remove public `dygo db prepare`.
- Remove public `dygo db schema`.
- Remove public `dygo db schema dump`.
- Remove public `dygo db schema check`; fold snapshot checks into `dygo doctor`.
- Move top-level `dygo schema prune` to `dygo db prune`.
- Add `dygo db prune --yes`.
- Add `dygo db prune --dry-run`.
- Replace `--confirm <environment>/<database>` public UX with the `--yes` and interactive prompt model from `docs/cli.md`.
- Keep protected env destructive safeguards:
  - development can run destructive commands after prompt or `--yes`.
  - staging/production destructive commands require `--force` in addition to prompt or `--yes`.
- Update `dygo db drop`, `dygo db reset`, and `dygo db prune` to print plans before prompting.
- Update database docs and root/new generated output to use the final commands.

References:

- `docs/cli.md` Database and Environment Safety Rule
- `internal/cli/db.go`
- `internal/cli/migrate.go`
- `internal/cli/schema.go`
- `internal/cli/patches.go`
- `internal/db/admin.go`
- `internal/db/migrate.go`
- `internal/db/schema.go`
- `internal/db/schema_prune.go`
- `docs/database.md`

## Phase 6 - Reshape Root, Dev, Serve, Setup, Upgrade, And Secrets

TODO:

- Keep root command help behavior.
- Keep `dygo new <name>`.
- Keep `dygo version`.
- Document or rely on Cobra's generated `dygo completion <shell>`.
- Change `dygo upgrade` to project-only behavior.
- Remove `dygo upgrade --cli-only`.
- Remove `dygo upgrade --project-only`.
- Remove `dygo upgrade --install-dir`.
- Keep:
  - `dygo upgrade --check`
  - `dygo upgrade --to <version>`
  - `dygo upgrade --dry-run`
  - `dygo upgrade --yes`
- Remove binary self-upgrade execution from `internal/upgrade.Run`; binary updates stay in installer scripts.
- Add `dygo dev` for local development orchestration.
- Move Studio/Vite dev proxy behavior from `dygo serve` to `dygo dev`.
- Remove `dygo serve --studio-dev-url` or expose the override only on `dygo dev`.
- Keep `dygo serve` as runtime server only.
- Collapse `dygo setup admin` into `dygo setup`.
- Keep setup interactive and move current admin bootstrap flags to `dygo setup` while this command owns Administrator bootstrap: `--env`, `--email`, `--full-name`, and `--password-stdin`.
- Rename `dygo secrets` to `dygo secret`.
- Move encrypted secret files and private key storage to the target paths from `docs/dir.md`.
- Add `dygo secret get <name>`.
- Add `dygo secret get <name> --env <environment>`.
- Remove `dygo secret init --force`.
- Replace `dygo secret rotate-key --confirm` with interactive default plus `--yes`.
- Preserve dot-path secret lookup through `internal/secrets.Store.Get`.

References:

- `docs/cli.md` Root, Secrets, Out Of Band Binary Updates, and Removal And Replacement Notes
- `internal/cli/root.go`
- `internal/cli/new.go`
- `internal/cli/upgrade.go`
- `internal/upgrade/upgrade.go`
- `internal/upgrade/project.go`
- `internal/upgrade/install.go`
- `internal/cli/setup.go`
- `internal/cli/secrets.go`
- `internal/secrets/secrets.go`
- `internal/studio/assets.go`
- `docs/installation.md`
- `docs/server.md`
- `docs/secrets.md`

## Phase 7 - Singular App And Entity Commands

TODO:

- Replace `dygo apps` with `dygo app`.
- Replace `dygo entities` with `dygo entity`.
- Keep:
  - `dygo app list`
  - `dygo app validate`
  - `dygo entity list`
  - `dygo entity validate`
- Add `dygo entity show <app>/<entity>`.
- Add `dygo entity graph`.
- Add `dygo entity graph <app>`.
- Add `dygo entity graph <app>/<entity>`.
- Make graph output useful to agents:
  - outgoing links
  - incoming links
  - collection fields
  - collection row ownership
  - route slug where relevant
- Keep validation output stable and actionable.

References:

- `docs/cli.md` Apps and Entities sections
- `internal/cli/apps.go`
- `internal/cli/entities.go`
- `internal/project/metadata.go`
- `internal/entity/catalog/catalog.go`

## Phase 8 - Generate Command Group

TODO:

- Add `dygo generate` command group and `dygo g` alias.
- Add `dygo generate app <app>`.
- Add `dygo generate entity <app>/<entity>`.
- Add `dygo generate collection <app>/<collection>`.
- Add `dygo generate hook <app>/<entity>`.
- Add `dygo generate fixture <app>/<entity>`.
- Add `dygo generate test <app>/<entity>`.
- Implement flags:
  - `--dry-run`
  - `--force`
  - `--no-hook`
  - `--no-fixture`
  - `--no-test`
- Use embedded templates under `internal/generate/templates/`.
- Do not write runtime template files from disk at command execution time.
- Ensure generated files are valid boilerplate, not empty placeholders.
- Ensure generators are non-interactive:
  - write when no conflicts
  - skip unchanged generated files
  - fail on custom-file conflicts
  - overwrite dygo-generated files only with `--force`
- Move hook scaffold entrypoint from `dygo hooks generate <app> <entity>` to `dygo generate hook <app>/<entity>`.
- Update hookgen output paths to Entity bundles:
  - `entities/<entity>/hooks.go`
  - generated runner wiring remains `cmd/dygo/main.go`
- Ensure `dygo generate entity` composes narrower generators.

References:

- `docs/cli.md` Generate section
- `docs/dir.md`
- `internal/hookgen/hookgen.go`
- `internal/cli/hooks.go`
- `internal/projectgen/projectgen.go`

## Phase 9 - Hook Inspection And Maintenance Commands

TODO:

- Replace `dygo hooks` with `dygo hook`.
- Remove `dygo hooks generate`.
- Add `dygo hook list`.
- Add `dygo hook validate`.
- Add `dygo hook sync`.
- Add `dygo hook sync --dry-run`.
- Add `dygo hook sync --force`.
- Make `hook list` show:
  - discovered hook files
  - app/entity ownership
  - runner wiring status
  - compiled registrations when available
- Make `hook validate` check:
  - hook file conventions
  - Entity references
  - duplicate compiled hook IDs when available
  - generated registrars
  - runner wiring
- Make `hook sync` update only generated runner wiring and never create new hook files.
- Include hook validation in `dygo doctor`.

References:

- `docs/cli.md` Hooks section
- `internal/cli/hooks.go`
- `internal/hookgen/hookgen.go`
- `internal/hooks/record_hooks.go`
- `pkg/sdk/runtime/runtime.go`

## Phase 10 - Routes And Permissions Commands

TODO:

- Add route registry package if current Entity validation is not enough as a reusable abstraction.
- Add `dygo route`.
- Add `dygo route list`.
- Add `dygo route validate`.
- Add `dygo route resolve <path>`.
- Add `dygo route resolve <method> <path>`.
- Add `dygo route reserved`.
- Route output should explain owners, route kind, effective slug, action, and required permission where method-aware.
- Add `dygo permission`.
- Add `dygo permission list`.
- Add `dygo permission list <app>/<entity>`.
- Add `dygo permission check <app>/<entity> <action> --user <email-or-id>`.
- Add `dygo permission check <app>/<entity> <action> --role <role>`.
- Add `dygo permission explain <app>/<entity> <action> --user <email-or-id>`.
- Add `dygo permission explain <app>/<entity> <action> --role <role>`.
- Permission commands should default to `--env development`.
- Permission commands must use the same internal permission engine as API and Studio behavior.
- Keep permission CLI explicitly database-backed; do not introduce a broad general-purpose DB record CLI in this PR.

References:

- `docs/cli.md` Routes and Permissions sections
- `internal/entity/catalog/catalog.go`
- `internal/reserved/reserved.go`
- `internal/permissions/permissions.go`
- `internal/db/metadata_reader.go`
- `internal/db/records.go`
- `docs/records.md`

## Phase 11 - Doctor Coverage

TODO:

- Keep existing doctor checks:
  - project root
  - Go toolchain
  - app manifests
  - Entity metadata
  - config
  - secrets layout
  - runtime database
  - Core fixtures
  - Administrator account
- Add or update checks for:
  - root `dygo.yml` config shape
  - `config/secrets` or final secret path
  - schema snapshot missing or stale
  - route conflicts and reserved route usage
  - fixture validity
  - hook file conventions
  - generated runner wiring
  - Studio cached/bundled assets
  - first-run setup state
- Ensure doctor messages reference final commands:
  - `dygo db migrate`
  - `dygo fixture apply`
  - `dygo setup`
  - `dygo dev`

References:

- `docs/cli.md` Root doctor note
- `internal/cli/doctor.go`
- `internal/cli/root_test.go`
- `internal/studio/assets.go`

## Phase 12 - Documentation Cleanup

TODO:

- Update docs that still use old command paths:
  - `dygo apps`
  - `dygo entities`
  - `dygo fixtures`
  - `dygo hooks`
  - `dygo secrets`
  - `dygo migrate`
  - `dygo migrate plan`
  - `dygo schema prune`
  - `dygo db prepare`
  - `dygo db schema dump`
  - `dygo setup admin`
- Update docs that still use old directory paths:
  - `configs/dygo.yaml`
  - `configs/secrets`
  - `var/`
  - `entities/<entity>.yml`
  - `entities/<entity>/<entity>.yml`
  - `entities/collections/`
  - `fixtures/<entity>.yml`
  - `hooks/<entity>.go`
- Keep `docs/cli.md` and `docs/dir.md` as the source references for final shape.
- Update generated project README text.
- Update `README.md` CLI and Project Shape sections after code is implemented.

References:

- `README.md`
- `docs/index.md`
- `docs/database.md`
- `docs/secrets.md`
- `docs/server.md`
- `docs/installation.md`
- `docs/entity-metadata.md`
- `docs/fixtures.md`
- `docs/record-hooks.md`

## Phase 13 - Tests And Verification Gates

TODO:

- Update CLI registration tests so old public paths fail and new paths exist.
- Add `dygo --help` and relevant subcommand help assertions.
- Add generated project shape assertions for `docs/dir.md` paths.
- Add config loading tests for root `dygo.yml`.
- Add Entity discovery tests for:
  - `entities/<entity>/entity.yml`
  - `_collections/<collection>.yml`
  - `_collections/<collection>/entity.yml`
  - rejection of old flat Entity and `entities/collections/` paths.
- Add fixture discovery tests for Entity-bundle fixtures.
- Add generator dry-run, force, and conflict tests.
- Add interactive write tests for `--yes` and `--dry-run` behavior.
- Add route and permission CLI tests with fake services where possible.
- Add doctor tests for new checks and final command recommendations.

Run before finalizing the PR:

```sh
go test ./internal/cli
go test ./internal/projectgen
go test ./internal/entity/...
go test ./internal/fixtures
go test ./internal/hookgen
go test ./internal/secrets
go test ./internal/upgrade
go test ./...
go vet ./...
```

Run if the PR changes runtime concurrency, hooks, or database transaction behavior:

```sh
go test -race ./...
```

Manual CLI help check before PR:

```sh
go run ./cmd/dygo --help
go run ./cmd/dygo db --help
go run ./cmd/dygo generate --help
go run ./cmd/dygo hook --help
go run ./cmd/dygo fixture --help
go run ./cmd/dygo secret --help
```

## Deferred Explicitly Out Of This PR

- `dygo worker`.
- `dygo scheduler`.
- Global `--json`.
- Smart shell completions beyond Cobra's generated `completion` command.
- Full job runtime, schedule runtime, report runtime, and custom page runtime.
- Production secret provider architecture such as KMS, Vault, or per-environment recipients.

## Target Interpretation For Implementation

The target docs should be treated as finalized. Where older docs or code disagree, implement this interpretation:

- `docs/dir.md` wins for generated project paths: use `config/`, `.dygo/secrets`, and `.dygo/*` runtime state instead of `configs/`, root `master.key`, or `var/`.
- `docs/dir.md` wins for Entity-owned files: hooks and fixtures live inside Entity bundles, not app-level `hooks/` and `fixtures/` directories.
- `docs/cli.md` wins for public command names and flags: singular nouns, `dygo db migrate`, `--yes`, `--dry-run`, project-only `dygo upgrade`, and no public `dygo patch` group.
