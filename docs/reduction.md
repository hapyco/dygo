# Reduction Scan

Static scan only. I verified the delete candidates with repo-wide reference searches and file inventory. No builds or tests were run.

## Safe Deletes

- `apps/studio/ui/src/stores/index.ts` - unused barrel. Current imports go straight to the concrete store files, so this file is not part of the live import graph.
- `apps/studio/ui/src/renderers/index.ts` - unused barrel. The app imports `@/renderers/records` directly.
- `apps/studio/ui/src/shell/index.ts` - unused barrel. Shell components are imported directly from their files.
- `apps/studio/ui/src/design/atoms/Badge.vue` - no current references outside the design barrel. If you remove it, also remove `BadgeVariant` from `apps/studio/ui/src/design/types.ts:9` and the `Badge` export from `apps/studio/ui/src/design/index.ts:16`.
- `apps/studio/ui/src/design/atoms/Divider.vue` - no current references outside the design barrel.
- `apps/studio/ui/src/design/molecules/FieldRow.vue` - no current references outside the design barrel.
- `apps/studio/ui/src/design/molecules/FormSection.vue` - no current references outside the design barrel.
- `apps/studio/ui/src/design/molecules/SearchBox.vue` - no current references outside the design barrel.
- `apps/studio/ui/src/design/molecules/RadioGroupField.vue` and `apps/studio/ui/src/design/primitives/RadioGroup.vue` - the field wrapper and the primitive are currently orphaned.
- `internal/db/schema_inspect.go:26-29` - unused `liveTable.HasIndex` method. I found no call sites outside the method definition itself.
- Remove the corresponding dead exports from `apps/studio/ui/src/design/index.ts:2,16,19,26,33-37`.

## DRY Reduction

- `internal/fixtures/fixtures.go:492-557` and `732-802` - validation and apply both implement the same dependency/topological sort shape. One shared topo-sort helper would remove duplicated loops.
- `internal/fixtures/fixtures.go:445-480` and `853-904` - link validation and link resolution both decode nested link references, enforce depth limits, and recurse over match fields. This is duplicated logic with two slightly different call sites.
- `internal/db/records.go:138-682` - the list/get/find/create/update/delete APIs all repeat the same public -> identity -> layout dispatch pattern. This is the biggest Go-side reduction target.
- `internal/cli/generate.go:36-247` - the generate subcommands are mostly the same command skeleton with different scaffold calls. A small command-spec table would trim a lot of boilerplate.
- `internal/cli/root.go:79-109` - the runner wrapper stack is thin but repetitive. If you want fewer lines, replace the layered wrappers with one options struct and one execution path.
- `apps/studio/ui/src/renderers/records/RecordFormRenderer.vue:41-120` and `apps/studio/ui/src/renderers/records/RecordCollectionTable.vue:65-181` - the two record renderers duplicate field normalization, editor selection, text conversion, boolean conversion, input type selection, and select-option extraction. That logic should be shared in a small helper module.

## Extra Defensive Code

- `internal/studio/assets.go:187-233` - the temp-dir + backup rename + rollback path is safe but very defensive. If cache replacement does not need rollback semantics, this is the easiest place to simplify.
- `internal/cli/root.go:364-461` - the Studio dev-server launcher polls readiness, captures bounded output, and preserves stop-state detail. Good operationally, but it is a lot of machinery for one command path.
- `internal/fixtures/fixtures.go:445-480, 853-904` - the recursion depth guard and repeated null checks are defensive against cyclic link graphs. Keep them only if cyclic or user-authored nested link fixtures are expected.

## Large Files

- `internal/db/records.go` - 1,784 lines.
- `internal/db/schema_plan.go` - 1,109 lines.
- `internal/fixtures/fixtures.go` - 1,102 lines.
- `internal/cli/db.go` - 629 lines.
- `apps/studio/ui/src/design/organisms/DataTable.vue` - 731 lines.
- `apps/studio/ui/src/pages/RecordFormPage.vue` - 651 lines.
- `apps/studio/ui/src/renderers/records/RecordCollectionTable.vue` - 428 lines.
- `internal/cli/root.go` - 488 lines, but unusually dense for a CLI root file.
- `internal/cli/root_test.go` - 2,796 lines.
- `internal/db/records_test.go` - 2,249 lines.

## Naming Notes

- `internal/studio/assets.go` is too generic for what it does. It is really Studio bundle source resolution, cache installation, and static handler wiring.
- `internal/fixtures/fixtures.go` is also broad. A split into `discover.go`, `validate.go`, and `apply.go` would read better.
- I left the tracked Studio placeholder files alone because they look like embed sentinels for clean checkouts rather than dead runtime code.

## Bottom Line

The highest-confidence deletions are the unused UI barrels and the unused design-system components. Everything else in this scan is a reduction or simplification opportunity, not a proven dead-code removal.
