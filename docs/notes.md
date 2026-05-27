# Notes

These are internal working notes. Framework behavior is documented in the focused reference docs linked from [Documentation Index](index.md).

## Documentation

- Keep framework documentation in `/docs`, not GitHub Wiki.
- Keep docs versioned with code and reviewed in pull requests.
- Prefer concise reference docs over planning transcripts or product-thesis prose.
- A future docs website can publish these files, but website tooling should wait until the docs are stable.

## Historical CLI And Directory Refactor

`docs/cli.md` and `docs/dir.md` remain the source references for the target command surface and directory shape.

The refactor plan originally tracked these work areas:

- Centralize path constants and slash-target parsing.
- Use root `dygo.yml`, `config/`, `.dygo/`, and `db/schema.sql`.
- Use canonical Entity bundles at `apps/<app>/entities/<entity>/entity.yml`.
- Store Entity fixtures at `apps/<app>/entities/<entity>/fixtures.yml`.
- Use `dygo db migrate`, `dygo db prune`, singular command groups, and `--yes` / `--dry-run` write safety.
- Keep `dygo upgrade` project-only; update binaries out of band through installers.
- Put scaffolding under `dygo generate` and alias `dygo g`.
- Keep hook generation under `dygo generate hook`; use `dygo hook` for inspection and wiring maintenance.
- Keep route validation filesystem-backed.
- Keep permission CLI explicitly database-backed because it must use the runtime permission engine.
- Include route, fixture, hook, schema snapshot, config, secrets, database, Studio assets, and first-run setup checks in `dygo doctor`.

Historical deferred items:

- `dygo worker`
- `dygo scheduler`
- global `--json`
- smart shell completions
- full job, schedule, report, and custom page runtimes
- production secret providers such as KMS or Vault

## Reduction Scan

Static scan notes, not verified by tests in this pass.

High-confidence delete candidates:

- `apps/studio/ui/src/stores/index.ts`
- `apps/studio/ui/src/renderers/index.ts`
- `apps/studio/ui/src/shell/index.ts`
- `apps/studio/ui/src/design/atoms/Badge.vue`
- `apps/studio/ui/src/design/atoms/Divider.vue`
- `apps/studio/ui/src/design/molecules/FieldRow.vue`
- `apps/studio/ui/src/design/molecules/FormSection.vue`
- `apps/studio/ui/src/design/molecules/SearchBox.vue`
- `apps/studio/ui/src/design/molecules/RadioGroupField.vue`
- `apps/studio/ui/src/design/primitives/RadioGroup.vue`
- `internal/db/schema_inspect.go` unused `liveTable.HasIndex`

DRY candidates:

- Share fixture dependency sorting between validation and apply.
- Share nested fixture link decoding between validation and apply.
- Reduce repeated public identity and layout dispatch in Record CRUD.
- Table-drive generator command construction.
- Share field normalization and editor selection across record renderers.

Large files to watch:

- `internal/db/records.go`
- `internal/db/schema_plan.go`
- `internal/fixtures/fixtures.go`
- `internal/cli/db.go`
- `apps/studio/ui/src/design/organisms/DataTable.vue`
- `apps/studio/ui/src/pages/RecordFormPage.vue`
- `apps/studio/ui/src/renderers/records/RecordCollectionTable.vue`
- `internal/cli/root_test.go`
- `internal/db/records_test.go`

Naming notes:

- `internal/studio/assets.go` is really Studio bundle source resolution, cache installation, and static handler wiring.
- `internal/fixtures/fixtures.go` may read better split into discovery, validation, and apply files.
