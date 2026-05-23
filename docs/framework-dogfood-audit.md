# Framework Dogfooding Audit

Status: implementation pass complete
Date started: 2026-05-23

## Purpose

Audit dygo for places where framework internals bypass framework-owned primitives instead of dogfooding them. The goal is to reduce one-off code, make behavior reusable, and keep Core/Studio built on the same contracts that app developers use.

## Review Lens

- A framework feature should have one reusable implementation path.
- Internal system writers should declare intent through Entity metadata where possible.
- Bootstrap-only exceptions should be explicit, small, and isolated.
- Core metadata should not duplicate naming, validation, routing, permissions, schema, or field-type rules in ad hoc code.
- Studio should render and navigate from metadata contracts instead of hardcoding Core assumptions.

## External Framework Notes

These are not requirements for dygo, but they help calibrate the direction.

- Frappe treats DocType metadata as the central model contract that drives schema, UI, and runtime behavior. Its naming docs also show naming as a framework-level concept with multiple strategies, not something each subsystem reimplements. Sources: [DocType overview](https://frappe.io/framework/doctype), [Naming](https://docs.frappe.io/framework/user/en/basics/doctypes/naming).
- Django model fields carry cross-layer meaning: database storage, validation, default form widgets, choices, and custom field extension points. This supports a richer dygo field-type capability model instead of scattered `switch field.Type` blocks. Source: [Django model field reference](https://docs.djangoproject.com/en/6.0/ref/models/fields/).
- Rails Active Record exposes lifecycle callbacks through the record model itself. The lesson for dygo is not to copy callbacks, but to make lifecycle behavior a first-class framework path and make any bypass explicit. Source: [Rails Active Record callbacks](https://guides.rubyonrails.org/active_record_callbacks.html).

## Finding Summary

| ID | Area | Severity | Status | Summary |
| --- | --- | --- | --- | --- |
| FD-001 | Record naming | High | Done | Naming strategy execution now lives in `internal/naming`, with DB records, metadata sync, patch ledger, and Activity creation calling the shared executor. |
| FD-002 | Field types | High | Done | Field type behavior now lives behind registry capabilities consumed by schema sync, Record runtime, fixtures, naming, metadata APIs, and Studio hints. |
| FD-003 | Internal system writers | High | Done | `SystemRecordWriter` now backs Activity, patch-run, login session, and first-admin writes; metadata/single direct SQL is isolated as bootstrap. |
| FD-004 | Metadata contracts | Medium | Done | Backend metadata JSON contract tests lock exact API keys and legacy-key absence; Studio types are aligned with those keys. |
| FD-005 | Studio routing identity | Medium | Done | Studio route lookup, navigation, and metadata caching now use route `slug` only instead of treating Entity `key` as a route fallback. |
| FD-006 | Fixture match/link semantics | Medium | Done | Fixture match validation and link target resolution now use shared DB metadata contract helpers before dogfooding RecordStore writes. |
| FD-007 | Single Entities | Medium | Done | The singleton Record name rule now has one `SingleRecordName` policy helper used by schema constraints, seeding, runtime reads, and tests. |
| FD-008 | Permissions | Medium | Done | Permission actions now run through one runtime action registry with metadata validation instead of a separate action-to-column switch. |
| FD-009 | API clients/errors | Low | Done | Studio API modules use one envelope-aware client, and server handlers share one JSON error-envelope writer. |
| FD-010 | Authoring schemas | Medium | Done | JSON Schema authoring contracts now have tests against runtime field types, naming strategies, check operators, constraint types, route reservations, and name patterns. |
| FD-011 | Project loading | Medium | Done | Commands and subsystems now share one project metadata loader for app and Entity validation context. |
| FD-012 | YAML decoding | Low | Done | A shared YAML metadata helper now covers duplicate-key/document helpers for app manifests, Entity schemas, config, fixtures, patches, and secrets. |
| FD-013 | Patches | Medium | Done | Patch operation names, allowed fields, and schema branches now share one operation registry. |
| FD-014 | Hook events | Medium | Done | Record hook events now come from one shared event catalog consumed by SDK constants, DB validation, input-mutation policy, and adapter mapping. |
| FD-015 | Doctor checks | Low | Done | `dygo doctor` now consumes Core runtime readiness checks from a framework health package instead of owning SQL checks. |
| FD-016 | Defensive tests | Medium | Done | Brittle implementation-shape assertions were relaxed or moved to contract-level checks around shared primitives. |
| FD-017 | Studio verification | Low | Done | Studio now has focused Node tests for slug route identity and metadata-driven list columns. |
| FD-018 | Record list query contract | Medium | Done | Record list pagination, sort, filter parsing, and Studio serialization now use shared query helpers. |
| FD-019 | Core select values | Medium | Done | Runtime services use `corevalues` constants that are contract-tested against Core Entity `select` metadata. |
| FD-020 | Hook Record access | Medium | Done | SDK hook `Records` calls use RecordStore, but the hook/activity policy is implicit and different from the outer mutation path. |
| FD-021 | Terminology/docs drift | Low | Done | Public docs now distinguish Entity `key`, Record `name`, and route `slug` in identity-sensitive sections. |
| FD-022 | Storage and system fields | High | Done | Storage names and system Record fields now live in one backend contract and flow to Studio list/form behavior through metadata. |
| FD-023 | Schema prune boundary | High | Done | Schema prune now has a clear managed-schema rule: metadata is source of truth, and explicit prune removes metadata-orphaned objects. |

## Findings

### FD-001: Naming Strategy Execution Is Not Truly Global

Severity: High
Status: Done

Evidence:

- `internal/db/records.go` has `RecordStore.generateRecordName`, but it only works from a persisted `recordLayout` and normal `RecordInput`.
- `internal/db/naming.go` has template rendering and link-name resolution, but `templateRecordName` is a `RecordStore` method tied to database record creation.
- `internal/db/naming.go` writes Core `naming-series` rows directly in `seriesRecordName`, even though `apps/core/entities/naming-series.yml` declares `naming.strategy: field` on `key`.
- `internal/db/metadata_records.go` now calls a separate helper for metadata row names.
- `internal/db/patch_ledger.go` now reads `core.patch-run` naming metadata, but still needs custom glue to render the name outside the normal record path.

Why this matters:

The YAML now correctly declares:

```yaml
naming:
  strategy: template
  template: "{entity}.{field-name}"
```

But the actual strategy executor is not reusable as a framework primitive. Internal code cannot simply say "this Entity has naming metadata; generate the name." That forced extra code instead of reducing it.

Desired direction:

- Extract a reusable naming engine from `RecordStore.generateRecordName`.
- Keep strategy dispatch in one place.
- Accept a small resolver interface/function for token values and link values.
- Let normal Record creation, metadata sync, patch ledger, admin setup, activity logging, and any future internal system writer call the same naming engine.

Potential shape:

```go
type NameValueResolver interface {
	Value(ctx context.Context, token string) (string, error)
}

func GenerateName(ctx context.Context, naming NamingPlan, resolver NameValueResolver) (string, error)
```

Resolution:

- Added a reusable naming engine in `internal/naming` for `random`, `field`, `series`, and `template` strategies.
- Kept DB-specific behavior behind small adapters: Record field/link value resolution and series counter persistence.
- Replaced metadata and patch-run deterministic name rendering with the shared naming executor.
- Relaxed JSON persistence tests to assert semantic naming payloads instead of PostgreSQL/display key order.

Notes:

- `random` and `series` now run through the shared executor; `series` keeps the database increment behind a `SeriesCounter` adapter.
- `field` and `template` now share resolver code through the same `ValueResolver` interface.
- Link rendering should remain a resolver concern so metadata sync can provide `entity = core.user` without needing a persisted link lookup during bootstrap.
- Direct Core table writes are still tracked separately under FD-003.

### FD-002: Field Type Behavior Is Not Centralized

Severity: High
Status: Done

Evidence:

- `internal/entity/fieldtype/registry.go` defines a field type registry, but `Definition` only knows labels, allowed metadata flags, and option validation.
- `internal/db/schema_plan.go` maps field types to storage columns in `columnForField` and `columnType`.
- `internal/db/records.go` separately maps field types for runtime write conversion, placeholder casts, read normalization, list filtering, password hashing, and write-only behavior.
- `internal/db/naming.go` separately decides which field types can participate in `field` and `template` naming.
- `internal/fixtures/fixtures.go` separately treats `link`, `collection`, system `name`, and unique match fields.
- `apps/studio/ui/src/pages/RecordFormPage.vue` and `apps/studio/ui/src/renderers/records/RecordFormRenderer.vue` separately decide submit conversion, defaults, input widgets, and readonly behavior by `field.type`.
- `apps/studio/ui/src/renderers/records/RecordFormRenderer.vue` currently renders `link` as a number input, and `apps/studio/ui/src/renderers/records/columns.ts` renders all list columns as text. That is a UI symptom of the metadata API not exposing field editor/display behavior.

Why this matters:

Adding or changing a field type now requires edits across backend schema planning, Record runtime, fixture import, naming, and Studio. The framework already has a field type registry, but the registry is not yet the framework source of truth for field behavior.

Desired direction:

- Expand the field type contract, or add a runtime field behavior registry beside `fieldtype`.
- Centralize these concerns per type:
  - DB column suffix and SQL type.
  - DB placeholder cast.
  - API input coercion and validation.
  - read normalization.
  - whether it is writable, write-only, list-filterable, sortable, name-renderable, unique-capable, and index-capable.
  - Studio editor hint and display hint.
- Let schema sync, RecordStore, fixtures, naming, and Studio metadata consume the same capability model.

Potential shape:

```go
type FieldRuntime struct {
	Type string
	Storage FieldStorage
	CoerceInput func(FieldContext, json.RawMessage) (any, error)
	NormalizeOutput func(any) any
	CanRenderName bool
	WriteOnly bool
	Listable bool
	StudioEditor string
}
```

Notes:

- This does not mean every field type needs a heavy object. Simple scalar helpers can still generate most definitions.
- Studio does not need Go code. The metadata API can expose renderer/input hints derived from the same Go registry.

Resolution:

- Expanded `fieldtype.Definition` with behavior capabilities for storage, SQL type, placeholder cast, value kind, write-only/listable/name-renderable/checkable flags, and Studio editor/display hints.
- Refactored schema validation, schema planning, Record layout, Record list/input/name conversion, fixture storage checks, and metadata API field responses to consume those capabilities instead of local type switches.
- Updated Studio metadata types, form conversion, form rendering, and list columns to consume metadata-provided value and Studio hints.

### FD-003: Internal System Writers Bypass A Shared Mutation Path

Severity: High
Status: Done

Original evidence:

- `internal/auth/service.go` inserted `session` rows directly and generated session names with `naming.Random`.
- `internal/auth/service.go` inserted or updated the admin `user` row directly during setup.
- `internal/db/records_activity.go` inserts `activity` rows directly and generates names directly.
- `internal/db/patch_ledger.go` writes `patch_run` rows directly, now with custom naming glue.
- `internal/db/metadata_records.go` upserts `app`, `entity`, `field`, `index`, and `constraint` records directly.
- `internal/db/schema.go` seeds single Entity rows with direct SQL.

Why this matters:

Some of these are legitimate bootstrap or lifecycle code paths, but they all need some subset of the same framework behavior: naming, defaults, validation, type coercion, system field rules, activity suppression, hook suppression, and permission bypass. Today each path chooses those behaviors manually.

Desired direction:

- Keep normal `RecordStore` as the public/runtime Record API.
- Extract a lower-level system mutation engine used by `RecordStore` and internal writers.
- Make bypasses explicit with options instead of implicit direct SQL.

Potential shape:

```go
type SystemMutationOptions struct {
	RunHooks bool
	WriteActivity bool
	CheckPermissions bool
	AllowSystemFields bool
	Bootstrap bool
}

type SystemRecordWriter interface {
	InsertByIdentity(ctx context.Context, app string, entity string, input RecordInput, opts SystemMutationOptions) (Record, error)
	UpsertByIdentity(ctx context.Context, app string, entity string, match RecordInput, input RecordInput, opts SystemMutationOptions) (Record, error)
}
```

Notes:

- Metadata sync may still need a bootstrap writer because Core metadata is being created before metadata can be fully queried.
- Activity writing must suppress recursive Activity hooks, but it can still reuse naming and field coercion.
- Single Entity seeding can reuse defaults and name generation once there is a system writer that can run without hooks/activity.

Resolution:

- Added `SystemRecordWriter` and explicit `SystemMutationOptions` for internal metadata-backed writes.
- Extracted shared insert SQL/mutation execution so normal Record creation and system writes use the same naming, validation, defaults, field coercion, and random collision retry path.
- Moved Activity creation to `SystemRecordWriter` with hooks/activity suppressed, removing its custom direct insert and custom name generation.
- Moved patch-run ledger insertion to `SystemRecordWriter`, keeping duplicate/checksum checks local but letting `core.patch-run` metadata own naming and field coercion.
- Moved runtime login session creation behind an `auth.SessionWriter`; the HTTP server wires it to `SystemRecordWriter` for `core.session`, so session names, select validation, datetime coercion, and defaults come from Entity metadata.
- Moved first-admin user creation behind an `auth.AdminWriter`; the CLI setup command wires it to `SystemRecordWriter` for `core.user`, so user naming, password hashing, boolean coercion, and upsert writes use Entity metadata.
- Kept metadata sync and single Entity seeding as explicit bootstrap direct SQL. Those paths run before or while the framework metadata they need is being established, so they are intentionally small bootstrap exceptions rather than hidden runtime mutation paths.

### FD-004: Metadata API Contracts Are Hand-Copied

Severity: Medium
Status: Done

Evidence:

- `internal/db/metadata_reader.go` defines the Go JSON DTOs for apps, entities, fields, indexes, and constraints.
- `apps/studio/ui/src/features/metadata/metadata.api.ts` manually mirrors those DTOs.
- `internal/server/server.go` exposes those DTOs directly through JSON envelopes.
- `schemas/entity.schema.json` validates authored YAML, which is a different contract from persisted/API metadata.

Why this matters:

Renames like `route-slug` to `slug`, `name` to `key`, and `is-single` add risk because the backend, API, and Studio can drift independently. The fallback code in Studio is a symptom of this drift.

Desired direction:

- Generate Studio metadata types from the backend API contract, or generate both Go and TypeScript DTOs from a small shared schema.
- Keep authored YAML schema separate from runtime metadata API schema.
- Add API contract tests for exact metadata response field names.

Resolution:

- Added backend metadata JSON contract coverage for exact API field names including `key`, `slug`, `is-single`, `is-collection`, field behavior keys, and embedded app metadata.
- Asserted legacy drift keys such as `route-slug`, `route_slug`, `is_single`, and `is_collection` are absent from runtime metadata JSON.
- Kept generation as a later improvement; the current backend DTOs remain the API contract with focused drift tests.

### FD-005: Studio Route Identity Still Has Compatibility Fallbacks

Severity: Medium
Status: Done

Evidence:

- `apps/studio/ui/src/stores/metadata.store.ts` resolves route entities with `entity.slug === slug || entity.key === slug`.
- `apps/studio/ui/src/stores/metadata.store.ts` caches metadata under requested key, `meta.name`, `meta.key`, and `meta.slug`.
- `apps/studio/ui/src/app/App.vue` uses `entity.slug || entity.key` for navigation.

Why this matters:

The current backend contract says public record routes use `slug`, while `key` is app-local identity. Keeping `key` as a transparent fallback makes it harder to detect slug problems and can hide collisions or stale metadata during development.

Desired direction:

- Use `slug` only for Studio routes and `/api/v1/records/{entity}` calls.
- Keep `key` visible as metadata, not as route identity.
- If compatibility is needed temporarily, isolate it in one explicit route-compat helper and remove it before v1.

Resolution:

- Removed `entity.key` fallback from Studio route lookup and sidebar navigation.
- Renamed metadata store caches from key-based to slug-based state and cache only requested slug plus returned `meta.slug`.

### FD-006: Fixtures Mostly Dogfood Records, But Duplicate Match And Link Rules

Severity: Medium
Status: Done

Evidence:

- `internal/fixtures/fixtures.go` correctly calls `FindRecord`, `CreateRecord`, and `UpdateRecord`.
- The same file still synthesizes the system `name` field in `fixtureField`.
- It independently validates fixture match fields against unique fields and unique constraints.
- It independently parses link field `options` and resolves linked records by fixture match references.

Why this matters:

Fixture import is close to the desired shape, but the edge semantics around system fields, unique matchability, and link target resolution are still local to fixtures. That can drift from RecordStore and metadata behavior as matching, linking, or app-scoped Entity identity evolves.

Desired direction:

- Keep fixture writes through RecordStore.
- Move match validation and link resolution into reusable metadata/record helpers.
- Let fixtures call a framework-level "find by declared match" helper instead of reproducing uniqueness checks.

Resolution:

- Added DB metadata contract helpers for field lookup, stored-field checks, link target resolution, and unique-backed record match validation.
- Refactored fixtures to use those helpers for fixture match rules, dependency ordering, link reference resolution, and system `name` handling.
- Kept fixture writes through `FindRecord`, `CreateRecord`, and `UpdateRecord`.

### FD-007: Single Entity Naming Policy Is Repeated

Severity: Medium
Status: Done

Evidence:

- `internal/db/schema_plan.go` creates a check constraint that enforces `name = <entity-key>` for single Entities.
- `internal/db/schema.go` seeds the singleton row using the same fixed name rule.
- `internal/db/records.go` reads and updates single Entities by looking up the fixed name.
- Tests assert the same convention in multiple packages.

Why this matters:

The rule itself is good: a single Entity has exactly one framework-owned Record, and its system `name` is the Entity key. The problem is that the rule is encoded in several places. A later change to single naming, app-qualified names, or collection-owned single records would need synchronized edits across schema sync, migration seed, RecordStore, and tests.

Desired direction:

- Add one tiny policy helper and use it everywhere:

```go
func SingleRecordName(entityKey string) string
```

- Keep this helper close to Record identity/naming code, not hidden inside schema planning.
- Use it from:
  - single Entity check constraint generation
  - migration-time seed
  - `GetSingleRecord`
  - `UpdateSingleRecord`
  - tests and error details

Resolution:

- Added `SingleRecordName` beside DB naming policy.
- Used it from single Entity check constraint generation, migration-time seeding, and singleton read path.
- Added a direct policy test so future naming changes update the helper first.

### FD-008: Permission Actions Are Declared Twice

Severity: Medium
Status: Done

Evidence:

- `apps/core/entities/permission.yml` defines boolean fields named `read`, `create`, `update`, `delete`, `export`, and `print`.
- `internal/permissions/permissions.go` defines the same actions as Go constants.
- `internal/permissions/permissions.go` maps each action to a SQL column in `actionColumn`.

Why this matters:

Permissions are framework-owned, so some Go-level knowledge is expected. Still, adding or renaming an action means updating Core metadata and the Go switch together. This is a smaller version of the field-type problem: metadata and runtime behavior can drift.

Desired direction:

- Keep the supported permission actions in one action registry.
- Use that registry to:
  - validate permission requests
  - map safe action names to SQL columns
  - validate that `core.permission` metadata contains the required boolean fields
- Do not dynamically trust arbitrary field names from metadata for SQL. The registry should stay explicit and safe.

Resolution:

- Added a permission action registry that owns supported actions and safe SQL columns.
- Refactored request validation and column lookup through the registry.
- Added metadata validation for the `core.permission` boolean action fields.

### FD-009: API Envelope And Error Handling Are Repeated

Severity: Low
Status: Done

Evidence:

- `apps/studio/ui/src/features/auth/auth.api.ts`, `metadata.api.ts`, and `records.api.ts` each define their own error envelope shape, parse JSON helper, error class, and code-to-message switch.
- `internal/server/server.go` has separate auth, metadata, record, and permission error writers with similar envelope construction and redaction behavior.

Why this matters:

This is not as serious as naming or field types, but it is the kind of drift that makes a framework feel less coherent. Error envelope shape and redaction rules are framework contracts. They should be boring and reusable.

Desired direction:

- Add one Studio API client helper for:
  - `data` and `list` envelopes
  - parse failures
  - consistent credential handling
  - domain-specific message mapping as a small callback
- Add a server-side error response helper that each domain can feed with `{code, message, details, safe}` rather than recreating envelope writing.
- Keep domain-specific status mapping explicit; just share the common envelope and redaction mechanics.

Resolution:

- Added a shared Studio API client helper for data/list envelopes, parse failures, included credentials, and domain-specific message mapping.
- Refactored auth, metadata, and record Studio API modules through the helper while preserving their domain error classes.
- Added focused API helper tests and excluded `*.test.ts` files from the production UI typecheck.
- Added a server-side error-envelope helper used by auth, metadata, permission, and record error writers.

### FD-010: Authored Metadata JSON Schemas Drift From Go Validators

Severity: Medium
Status: Done

Evidence:

- `schemas/entity.schema.json`, `schemas/app.schema.json`, and `schemas/fixture.schema.json` duplicate the kebab-case name regex.
- `internal/entity/fieldtype/names.go` also owns the same metadata name regex.
- `internal/app/manifest/manifest.go` defines its own `kebabNamePattern` and semver-like version pattern.
- `schemas/entity.schema.json` duplicates field type names, naming strategy names, route reserved slugs, and option shapes that are also enforced in Go.
- `schemas/schema_test.go` only checks that schema files are valid JSON and referenced by VS Code; it does not compare schema semantics to Go validators.

Why this matters:

JSON Schemas are editor tooling, so Go should remain authoritative. But when the schemas repeat the same rules by hand, developers can get green editor feedback for metadata that Go rejects, or red editor feedback for metadata that Go accepts.

Desired direction:

- Centralize metadata syntax constants in Go:
  - name regex
  - route reserved slugs
  - field type names
  - naming strategy names
  - app manifest path fields
- Either generate JSON Schemas from those definitions or add contract tests that compare schemas against the Go registries.
- Keep authored YAML schemas separate from persisted metadata/API schemas, but make both generated or contract-tested.

Resolution:

- Added runtime contract helpers for field type names, naming strategies, check operators, constraint types, and reserved root route slugs.
- Added `schemas` package tests that parse `entity.schema.json`, `app.schema.json`, and `fixture.schema.json` and compare their duplicated authoring enums/patterns against the Go runtime contracts.
- Kept JSON Schemas hand-authored for now, but made drift visible in focused tests.

### FD-011: App And Entity Catalog Loading Is Repeated Across Subsystems

Severity: Medium
Status: Done

Evidence:

- `internal/db/schema.go` has `loadMetadataCatalog(root)` for app/entity validation before schema sync.
- `internal/cli/entities.go`, `internal/cli/apps.go`, and `internal/cli/doctor.go` each call app registry and Entity catalog validation directly.
- `internal/fixtures/fixtures.go` discovers apps separately before fixture loading.
- `internal/hookgen/hookgen.go` separately loads apps and entities for hook generation.

Why this matters:

The load sequence is becoming framework infrastructure: discover project root, load app manifests, order/validate dependencies, build field type registry, load entities, validate catalog-level rules. Repeating it makes it harder to add new project-level rules such as app type, enabled apps, generated apps, feature flags, or custom field registries.

Desired direction:

- Add one project metadata loader, for example:

```go
type ProjectMetadata struct {
	Root string
	Apps []manifest.LoadedApp
	Entities []catalog.LoadedEntity
	FieldTypes fieldtype.Registry
}

func LoadProjectMetadata(root string, opts ProjectMetadataOptions) (ProjectMetadata, error)
```

- Let CLI, schema sync, fixtures, hookgen, and doctor share it.
- Keep lower-level packages (`manifest`, `catalog`) small and reusable; the new loader should orchestrate, not absorb everything.

Resolution:

- Added `internal/project.Metadata`, `LoadApps`, `LoadEntities`, and `LoadMetadata` as the shared project metadata loading path.
- Routed schema sync, schema prune, patch planning, fixture apply, hook generation, doctor, `apps`, and `entities` commands through the shared loader.
- Left low-level app registry and Entity catalog tests direct, since those packages are the primitives behind the project loader.

### FD-012: YAML Decoding And Duplicate-Key Rejection Are Reimplemented

Severity: Low
Status: Done

Evidence:

- `internal/app/manifest/manifest.go`, `internal/entity/schema/schema.go`, `internal/config/config.go`, `internal/fixtures/fixtures.go`, and `internal/patches/patches.go` each have their own duplicate-key traversal or YAML document decoding logic.
- Error text differs by package, and some decoders use `KnownFields(true)` while others manually parse mappings.

Why this matters:

The duplication is not dangerous yet, but it is boilerplate framework code. Metadata parsing should feel consistent across app manifests, Entity YAML, fixtures, patches, config, and future job/import definitions.

Desired direction:

- Extract a small internal YAML metadata decoder package:
  - single-document enforcement
  - duplicate-key rejection
  - known-field decode helper
  - mapping/sequence/scalar helpers with line-aware errors
- Keep domain validation in each package.
- Use consistent source-location formatting.

Progress:

- Added `internal/yamlmeta` for YAML syntax-tree parsing, duplicate-key walking, top-level mapping extraction, value mapping checks, and string scalar helpers.
- Refactored app manifest, Entity schema, runtime config, fixtures, and patches to use the shared duplicate-key/document helpers.
- Preserved each caller's existing user-facing error wording while removing repeated tree-walk code.
- Refactored secret document duplicate-key validation to use `internal/yamlmeta` too, while preserving the secret-specific duplicate path format.

### FD-013: Patch Operation Contracts Are Split Between Packages

Severity: Medium
Status: Done

Evidence:

- `internal/patches/patches.go` has the patch operation type registry used during decode.
- `internal/db/patch_plan.go` defines a second set of patch operation constants and the planner switch.
- Required fields for each operation live in planner methods such as `planRenameField`, `planBackfillField`, and `planSQL`, not in a shared operation definition.
- There is no patch JSON Schema in `schemas/`, even though patches are authored metadata.

Why this matters:

Patches are part of dygo's framework-level schema evolution story. If operation names, field requirements, and planner behavior drift, users will get late errors or inconsistent tooling. This will get worse when patches grow beyond DB schema operations.

Desired direction:

- Introduce a patch operation registry with:
  - operation type
  - required fields
  - optional fields
  - value kinds
  - planner callback
  - schema/export metadata
- Let `internal/patches` use it for decode validation.
- Let `internal/db` register or consume DB patch operation planners.
- Generate or contract-test a `schemas/patch.schema.json`.

Resolution:

- Added a patch operation registry in `internal/patches` for v1 operation names, required fields, optional fields, and supported phases.
- Replaced DB patch operation string constants and allowed-field lists with the shared patch registry.
- Added `schemas/patch.schema.json`, wired it into VS Code YAML schema settings, and contract-tested its operation branches against the Go registry.

### FD-014: Record Hook Events Are Declared In Two Layers

Severity: Medium
Status: Done

Evidence:

- `pkg/sdk/hooks.go` defines public `RecordHookEvent` constants such as `before-create`, `after-update`, and `after-delete`.
- `internal/db/record_hooks.go` defines an internal `RecordHookEvent` type with the same event strings.
- `internal/hooks/record_hooks.go` maps SDK events to DB events through `recordHookEvent`.
- `internal/hooks/record_hooks_test.go` parses the SDK package with `go/parser` and `go/ast` to detect SDK hook constants that are not covered by the mapping table.

Why this matters:

The public SDK should remain decoupled from internal DB implementation types. The issue is not the adapter itself; it is that the event catalog has no first-class home. The AST test is a useful guard, but it exists because the framework has two hand-maintained hook event lists.

Desired direction:

- Add one hook event registry or generated contract used by:
  - SDK constant generation or verification
  - internal DB hook validation
  - adapter mapping
  - hook generator templates
- Keep SDK and internal types separate if that is useful for compatibility, but make the list of supported events a single source of truth.
- Move "which hook events can mutate input" into the same event definition instead of a separate switch in `recordHookEventMutatesTargetInput`.

Potential shape:

```go
type RecordHookEventSpec struct {
	Name string
	MutatesInput bool
	Operations []string
}
```

Resolution:

- Added a shared `internal/hookevents` catalog for supported Record hook events and input-mutation capability.
- Derived SDK event constants and supported event list from the shared catalog.
- Refactored DB event validation and input cloning policy through the same catalog.
- Replaced the SDK-to-DB mapping switch with a catalog membership check and direct type conversion.
- Removed the AST-based coverage test in favor of direct catalog contract tests.

### FD-015: Doctor Uses Hand-Coded Core Readiness Knowledge

Severity: Low
Status: Done

Evidence:

- `internal/cli/doctor.go` checks Core fixture readiness with direct SQL against `"role"` and `"permission"`.
- `internal/cli/doctor.go` checks administrator setup with direct SQL against `"user"` and `administrator`.
- `internal/cli/root_test.go` asserts exact doctor output such as `2 roles and 17 permissions ready`.

Why this matters:

`dygo doctor` should stay practical and direct, so this is not urgent. But the checks encode Core app assumptions in CLI code. As Core fixtures, permissions, or setup requirements grow, doctor will need more hardcoded SQL unless apps can expose readiness checks through framework metadata.

Desired direction:

- Keep built-in doctor checks for project root, config, secrets, and database reachability.
- Move app/runtime readiness into small reusable health check contracts.
- Let Core register checks such as:
  - required fixture records exist
  - administrator exists
  - metadata schema is synced
- Consider allowing future apps to register their own doctor checks without editing CLI core.

Resolution:

- Added `internal/health` with Core runtime readiness checks for fixtures and Administrator setup.
- Moved the direct role, permission, and administrator SQL out of `internal/cli/doctor.go`.
- Kept doctor responsible for orchestration and status rendering only.

### FD-016: Some Defensive Tests Are Symptoms Of Missing Primitives

Severity: Medium
Status: Done

Evidence:

- `internal/hooks/record_hooks_test.go` uses AST parsing to ensure every SDK hook event is mapped to an internal DB hook event.
- `internal/db/metadata_records_test.go` has tests such as `TestBuildMetadataRecordsUsesCoreMetadataNamingTemplates` that assert metadata sync manually applies naming templates for Core metadata records.
- `internal/db/metadata_records_test.go` includes `TestEntityNamingJSONOrdersStrategyFirst`, but Postgres `jsonb` will canonicalize key order when stored; this test protects display JSON, not a framework invariant.
- `internal/permissions/permissions_test.go` checks exact SQL join fragments in `Checker.Check`, including aliases and join shape.
- `internal/server/server_test.go` often checks raw JSON substrings instead of decoding the response envelope and asserting fields.
- `internal/db/schema_test.go` and `internal/db/patch_plan_test.go` assert SQL strings heavily. Some of this is valid because patch and schema plans are review surfaces, but not every internal SQL formatting detail should be treated the same way.
- `internal/db/patch_apply_test.go` asserts exact fake transaction events such as `queryrow:naming` and SQL substring routing in the fake transaction.
- `internal/cli/root_test.go` asserts exact doctor counts and prose fragments for implementation-specific health checks.

Why this matters:

These tests are not "bad" in isolation. They are compensating for missing reusable contracts. The cost is that tests lock down glue code and output details, so simplification work becomes harder: deleting duplicate code often requires rewriting tests that only existed to protect that duplicate code.

What to keep:

- Keep behavior tests that protect public contracts:
  - record names generated from Entity naming metadata
  - hooks fire in documented order
  - patches are applied idempotently and safely
  - doctor reports actionable failures
  - patch plan SQL shown to users for review
  - schema blockers for unsafe conversion, extra objects, type drift, and destructive changes

What to relax or reshape:

- AST hook mapping coverage can become a registry contract test.
- Metadata naming template tests can become naming engine tests plus one metadata integration test.
- JSON key-order tests should move to UI display formatting if the UI truly needs strategy-first rendering. Otherwise decode JSON and assert semantic fields.
- Permission tests should verify decisions and safe action-column mapping, not full SQL join structure.

Progress:

- Replaced the AST hook mapping coverage with shared hook event catalog tests under FD-014.
- Relaxed metadata naming JSON tests to decode semantic naming payloads instead of asserting object key order.
- Relaxed permission checker tests to assert decisions, query args, and safe action-column mapping instead of the full join shape.
- Added shared Studio and server API envelope helpers under FD-009, with focused helper tests replacing repeated parse/envelope checks.
- Added shared Record list query contract tests under FD-018 instead of repeating parser/normalizer expectations in every layer.
- Relaxed patch-apply fake transaction assertions to require the observable patch-run ledger write without locking down internal metadata lookup order.

Audit rule for tests:

If a test's main value is "make sure this second implementation stays aligned with the first," treat the second implementation as the smell. After extracting the shared primitive, keep one focused primitive test and one integration smoke test.

Test simplification targets:

| Test area | Current value | Keep | Relax after simplification |
| --- | --- | --- | --- |
| `internal/db/metadata_records_test.go` naming-template tests | Catches metadata record naming regressions. | One integration test that Core metadata records are named correctly. | Detailed per-record naming template assertions once the global naming engine has direct tests. |
| `internal/db/metadata_records_test.go` JSON ordering test | Improves display readability before `jsonb` storage. | Semantic JSON assertions for naming fields. | Exact object key order unless a UI formatter explicitly owns that display contract. |
| `internal/db/records_test.go` list SQL tests | Protects pagination, sorting, filtering, write-only exclusion, and system filters. | Contract tests for list behavior and safe SQL argument binding. | Exact `WHERE`/`ORDER BY` fragments after a reusable query planner/codec owns the syntax. |
| `internal/server/server_test.go` raw JSON substring tests | Protects status mapping and redaction. | Status codes, stable error codes, redaction, and permission-before-store behavior. | String containment checks once shared envelope decode helpers assert response objects. |
| `internal/db/schema_prune_test.go` public-schema drop expectations | Protects current explicit prune behavior. | Preview/confirm flow, no `CASCADE`, blockers before execution. | Exact operation ordering if prune later has a richer planner. |
| `internal/hooks/record_hooks_test.go` AST event mapping | Prevents SDK/internal hook event drift. | Supported event coverage. | AST parsing once hook events come from one registry or generated contract. |

### FD-017: Studio Has Build-Only Verification

Severity: Low
Status: Done

Evidence:

- `apps/studio/ui/package.json` has `dev`, `build`, `build:embed`, and `preview`, but no test command.
- There are no `*.test.ts`, `*.spec.ts`, or component tests under `apps/studio/ui/src`.
- The most drift-prone Studio code is metadata-driven:
  - `metadata.store.ts` route/meta caching and slug fallback
  - `RecordFormPage.vue` form input conversion
  - `RecordFormRenderer.vue` field-type widget selection
  - `RecordListRenderer.vue` column behavior

Why this matters:

Backend tests are heavy in places where implementation can change, while Studio has no focused tests around the metadata contract most likely to drift. `npm run build` catches type errors, but not routing behavior, field conversion, or renderer choices.

Desired direction:

- Do not add a large frontend test suite yet.
- After the API helper and field renderer contract are simplified, add a small Vitest setup with focused tests for:
  - metadata store uses `slug` as route identity
  - single Entity route opens form mode directly
  - field renderer picks editor/display hints from metadata

Resolution:

- Added a lightweight `npm test` script using Node's built-in test runner for pure TypeScript tests, avoiding a new frontend test framework dependency.
- Extracted Studio metadata route identity helpers and covered slug-only lookup/cache behavior.
- Added focused list-column tests for metadata-provided display hints, listability, write-only exclusion, and system `name` precedence.
  - API client decodes `data`, `list`, and error envelopes consistently
- Keep browser verification for larger UI changes.

### FD-018: Record List Query Contract Is Scattered

Severity: Medium
Status: Done

Evidence:

- `internal/db/records.go` owns `RecordListParams`, `defaultRecordLimit`, `maxRecordLimit`, and filter/sort validation in `normalizeRecordListParams` and `recordLayout.listQuery`.
- `internal/server/server.go` independently parses the HTTP query grammar in `recordListParams`, `recordPaginationParams`, and `recordSortParams`.
- `internal/server/server.go` repeats the `2500` limit rule in HTTP validation instead of using the DB/query contract.
- `apps/studio/ui/src/features/records/records.api.ts` independently serializes list params into `limit`, `offset`, `sort=-field`, and arbitrary filter query params.
- `apps/studio/ui/src/stores/records.store.ts` hardcodes page sizes `[20, 100, 500, 2500]`.
- `pkg/sdk/hooks.go` exposes a separate `RecordListParams` shape, and `internal/hooks/record_hooks.go` converts it into `db.RecordListParams`.

Why this matters:

Listing Records is a framework-level API contract. The current shape works, but the contract is scattered: changing max page size, sort grammar, filter operators, cursor pagination, or future typed filters means touching DB code, server parsing, Studio request building, and SDK adapters separately.

Desired direction:

- Add a small record query contract package used by DB, server, SDK adapters, and tests.
- Keep HTTP parsing separate from DB SQL planning, but make it a codec over the same contract:

```go
type RecordQuerySpec struct {
	DefaultLimit int
	MaxLimit int
	SupportedSortSyntax []string
	Filters RecordFilterSpec
}
```

- Expose the effective list/query capabilities through metadata or a small constants endpoint so Studio does not hardcode page-size limits.
- Let SDK `RecordListParams` stay public, but contract-test it against the internal query model or generate the adapter mapping.

Notes:

- This should not block the current simple list API.
- It becomes more important before adding advanced filters, import preview matching, reports, or background-job list views.

Resolution:

- Added `internal/recordquery` with shared pagination bounds, HTTP query decoding, sort parsing, deterministic filter ordering, and contract tests.
- Refactored DB list normalization and server list/activity parsing through the shared query contract.
- Added Studio record query helpers and tests for list query serialization and allowed page sizes.

### FD-019: Core Select Values Are Declared In YAML And Repeated In Go

Severity: Medium
Status: Done

Original evidence:

- `apps/core/entities/app.yml` declares app `status` values such as `installed`, `active`, `disabled`, `pending-install`, `pending-upgrade`, and `failed`.
- `internal/db/metadata_records.go` writes app metadata records with status `"active"` directly.
- `apps/core/entities/session.yml` declares session `status` values `active`, `expired`, and `revoked`.
- `internal/auth/service.go` hardcoded local `active` and `revoked` session constants.
- `apps/core/entities/activity.yml` declares activity `kind`, `operation`, and `status` select values.
- `internal/db/records_activity.go` inserts activity rows with literal values such as `"record"` and `"success"`, while `activityTitle` separately switches on operation strings.

Why this matters:

These values are framework domain contracts. The metadata needs them so schema, validation, Studio forms, and fixtures understand allowed values. Runtime services also need them so code is readable and safe. Today both layers are hand-maintained, so a future state such as a new session status or job activity operation can drift between YAML and Go.

Desired direction:

- Keep the user-facing allowed values in Entity metadata.
- Add small domain registries for Core-owned enum-like values:
  - app status
  - session status
  - activity kind/status/operation
  - future job/import states
- Add contract tests that verify Core Entity `select` options contain the registry values.
- Use registry constants from runtime services instead of raw string literals.

Potential shape:

```go
type DomainValue struct {
	Value string
	Label string
}

var SessionStatuses = DomainValues{
	Active: "active",
	Expired: "expired",
	Revoked: "revoked",
}
```

Notes:

- Full generation from YAML is probably too much for v1.
- A registry plus metadata contract tests is enough to prevent drift without making bootstrap harder.

Resolution:

- Added `internal/corevalues` with named Core app status, session status, and activity kind/operation/status values.
- Refactored metadata sync, auth sessions, and Record Activity writes to consume Core value constants instead of local status/kind literals.
- Added contract tests that load Core Entity YAML and verify select options match the runtime value registries.

### FD-020: Hook-Scoped Record Access Has Implicit Mutation Semantics

Severity: Medium
Status: Done

Evidence:

- `pkg/sdk/hooks.go` exposes `RecordHook.Records` as transactional access to metadata-backed Records.
- `internal/hooks/record_hooks.go` implements that service by constructing `db.NewRecordStore(d.queryer)`.
- `db.NewRecordStore` installs the default framework hook registry, not the compiled app hook registry used by the outer mutation.
- That means app hook code can create/update/delete records, but the follow-on hook behavior is not obviously the same as normal API/fixture mutation behavior. If this is intentional to avoid recursive hook loops, the policy is hidden in construction code rather than in a framework contract.

Why this matters:

Hooks are a core extension point. App authors will reasonably expect `hook.Records.Create` to use the same framework behavior as normal Record writes unless dygo clearly declares a different policy. Hidden differences around app hooks, activity writes, permissions, and recursion are exactly the kind of one-off behavior that later becomes hard to explain.

Desired direction:

- Make hook-scoped Record mutation policy explicit.
- Decide whether hook-created Records should:
  - run framework hooks only
  - run all app hooks
  - suppress hooks entirely
  - suppress only recursive hooks for the same target
- Represent that decision in one place, for example as `RecordMutationOptions` or a hook-scoped system writer, instead of relying on `NewRecordStore` defaults.
- Add tests around observable hook behavior, not only around the adapter wiring.

Resolution:

- Added `RecordMutationHookPolicy` with explicit framework-only and no-hook modes.
- Routed SDK hook `RecordData` writes through the framework-only policy so Activity remains active while app hooks do not re-enter.
- Routed `SystemRecordWriter` construction through the no-hook policy instead of passing nil hooks directly.
- Documented the hook write policy on the SDK `RecordData` interface and added policy coverage for suppressing framework Activity.

### FD-021: Entity Identity Terminology Still Drifts In Docs

Severity: Low
Status: Done

Evidence:

- `README.md` says `{entity}` is the route slug "defaulting to Entity `name`."
- `docs/entity-metadata.md` says `route.slug` defaults to Entity `name`.
- `docs/entity-metadata.md` and `docs/database.md` say Single Entity system `name` is fixed to the Entity name.
- The current code and Core metadata now distinguish:
  - `key`: app-local file-derived Entity key
  - `name`: qualified persisted Core Entity record name, such as `core.user`
  - `slug`: globally unique route identity

Why this matters:

The model itself is good, but the wording is easy to regress because "name" used to mean several things. Docs are part of the framework contract for app authors and future agents. If docs say "name" where the system means `key`, developers will reintroduce app-scoped/global identity confusion in code and tests.

Desired direction:

- Update docs and examples to use:
  - Entity `key` for file-derived local identity
  - Entity Record `name` for qualified Core metadata rows
  - route `slug` for URLs and API paths
  - Record `name` for per-table stable business/system identifiers
- Add a short terminology table to `docs/entity-metadata.md` or `docs/nomenclature.md`.
- Consider a small docs search checklist for metadata renames before merge.

Resolution:

- Updated README and docs for API routes, metadata authoring, schema sync, fixtures, patches, hooks, and validation wording.
- Replaced stale "Entity name" wording with "Entity key" where the file-derived identity is meant.
- Kept Record `name` wording for saved row identity generated by Entity `naming`.

### FD-022: Storage Names And System Record Fields Are Repeated

Severity: High
Status: Done

Evidence:

- `internal/db/schema_plan.go` owns `entityTableName`, `tableName`, `storageName`, `columnForField`, and `createTableSQL`.
- `internal/db/records.go` separately maps persisted metadata to runtime layout with `entityTableName`, `recordColumnForField`, `systemRecordListField`, and `isSystemRecordField`.
- `internal/db/patch_plan.go` uses table/column helpers but also has its own `isSystemColumn` and live-column inference candidates.
- `internal/db/schema_prune_test.go`, `internal/db/schema_test.go`, and `internal/db/records_test.go` repeat system columns such as `id`, `name`, `created_at`, and `updated_at`.
- `apps/studio/ui/src/pages/RecordFormPage.vue` skips `id`, `created-at`, and `updated-at` during submit.
- `apps/studio/ui/src/renderers/records/RecordFormRenderer.vue` hides `id`, `created-at`, and `updated-at`.
- `apps/studio/ui/src/renderers/records/columns.ts` manually adds `name`, `created-at`, and `updated-at` list columns.

Why this matters:

The table name, storage column, and system field rules are as fundamental as naming strategies. They define how metadata becomes PostgreSQL, how Records are read and written, how patches target old data, how prune detects drift, and how Studio decides what to show or hide. Any future change to system fields, app-qualified table names, collection table storage, audit columns, soft delete, or custom storage suffixes would require edits across several packages and Vue files.

Desired direction:

- Introduce one storage contract for metadata-backed Records:

```go
type StorageContract struct {
	TableName(app string, entity string) string
	FieldColumn(field FieldRef) (ColumnRef, error)
	SystemFields() []SystemField
	IsSystemField(name string) bool
}
```

- Use it from schema planning, Record runtime, patch planning, prune, fixtures, and tests.
- Expose system field metadata through the metadata API so Studio can render, hide, list, and sort system fields from metadata instead of hardcoding `id`, `created-at`, and `updated-at`.
- Keep SQL quoting and SQL type mapping explicit and safe; the goal is one contract, not dynamic SQL from arbitrary metadata.

Notes:

- This is adjacent to FD-002 but not the same. FD-002 is about per-field type behavior. This finding is about framework-level storage identity and system fields.
- This should probably land near the naming engine work because both are Record identity primitives.

Resolution:

- Added a DB storage/system-field contract that owns system field names, storage columns, labels, listability, name-rendering, and Studio display hints.
- Refactored schema planning, Record runtime, patch planning, and metadata API system field output through the shared contract.
- Exposed `system-fields` on Entity metadata and updated Studio list columns to consume system field metadata when available.
- Added a Studio system-field helper for remaining form/submit behavior so `id`, `created-at`, and `updated-at` are no longer repeated in multiple UI files.

### FD-023: Schema Prune Needs A Managed-Schema Boundary

Severity: High
Status: Done

Evidence:

- `internal/db/schema_prune.go` builds desired tables from currently loaded Entity metadata and live tables from the PostgreSQL public schema.
- `internal/db/schema_prune.go` plans `DROP TABLE` for every live table absent from desired metadata.
- `internal/db/schema_prune.go` plans drops for extra columns, indexes, and constraints based on metadata absence.
- `internal/db/schema_prune_test.go` explicitly expects a public-schema table such as `old_import` to be dropped when no loaded Entity declares it.
- `docs/database.md` and `docs/patches.md` must clearly document that dygo's managed schema is metadata-owned; otherwise patch-created or manually created objects can surprise operators during prune.

Why this matters:

The explicit preview/confirm flow is good, but the contract must be clear. dygo should either own a schema fully or keep unmanaged objects somewhere else. A halfway per-object ownership flag without a ledger makes prune unpredictable and can hide the real framework rule.

Desired direction:

- Treat dygo's managed PostgreSQL schema as metadata-owned.
- Let explicit `schema prune` remove objects present in that managed schema but absent from loaded Entity metadata:
  - tables
  - columns
  - indexes
  - constraints
- Keep `dygo migrate` additive and safe; only `schema prune --confirm` performs destructive metadata-orphan cleanup.
- Do not keep long-lived unmanaged tables or columns in dygo's managed schema. Model them as metadata, clean them up in patches, or place them in another PostgreSQL schema.
- Keep generated prune SQL quoted and without `CASCADE` so hidden dependencies fail instead of widening the blast radius.

Resolution:

- Removed the unused live-object ownership flags from schema inspection.
- Restored prune planning for metadata-orphaned tables, columns, indexes, and constraints in the managed schema.
- Kept prune explicit: preview by default, confirmed transaction for destructive cleanup, no `CASCADE`.
- Updated docs to state that patch-created permanent objects must either become metadata or live outside dygo's managed schema.
- Updated tests so prune behavior is driven by metadata absence, not synthetic ownership flags.

## Future Dogfooding Watchlist

- Collection storage and inline table rendering once that implementation starts.
- Background jobs/queueing later: design it as framework primitives from day one, not separate internal tables with separate naming.
- Importer: design import matching, defaults, links, validation, and dry-run around the same RecordStore/system mutation primitives instead of another fixture-like path.
- Sharing rules: keep them on the same permission/action and system mutation primitives when record sharing lands.
- Managed-schema escape hatches: add documented support for ignored objects or alternate PostgreSQL schemas if long-lived unmanaged database objects become necessary.

## Verification

- No `Open` or `In Progress` audit rows remain.
- `go test ./...` passes.
- `go vet ./...` passes.
- `npm test` passes in `apps/studio/ui`.
- `npm run build` passes in `apps/studio/ui`.
- `git diff --check` passes.
- Existing production Go files moved down by 399 net lines, and existing production Studio files moved down by 152 net lines. The total repository diff is larger because this pass added the audit document, shared contract modules, and contract tests.

## Completed Pass Order

1. Extract the naming engine first. It directly addresses the current `metadata_records.go` concern and should remove code.
2. Extract the storage/system-field contract. Table names, system fields, and column names are used by schema sync, Record runtime, patches, prune, and Studio.
3. Extract a shared field runtime contract. This prevents every new field type from multiplying switches.
4. Introduce an internal system record writer/mutation planner. Use it for Activity, patch ledger, and runtime session creation while keeping bootstrap exceptions explicit.
5. Make schema prune's managed-schema contract explicit: metadata is source of truth, and confirmed prune removes metadata-orphaned objects.
6. Add a project metadata loader and shared YAML metadata decoder. This is a modest cleanup that removes repeated orchestration across CLI, schema sync, fixtures, hookgen, and config readers.
7. Centralize patch and hook event registries. Keep public/internal type boundaries where useful, but stop maintaining operation/event lists twice.
8. Centralize the Record list query contract before adding richer filters, reports, or importer previews.
9. Add Core domain registries for select values used by runtime services, then contract-test them against Core metadata.
10. Clarify hook-scoped Record mutation semantics before encouraging app developers to create/update Records from hooks.
11. Generate or validate authored schemas and metadata API types before the next metadata rename.
12. Tighten Studio to `slug` routes only after backend contract tests are in place.
13. Clean up Entity identity terminology in docs immediately, because it is cheap and prevents recurring confusion.
14. Relax brittle implementation tests as each shared primitive lands; do not do a separate test-cleanup sweep without changing the underlying duplicated code.
