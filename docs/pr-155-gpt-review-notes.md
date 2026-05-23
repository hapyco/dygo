# PR 155 Decision Notes

Source: GPT-5.5 Pro review message shared by Tahseen.

Status: discussion notes only. These are not locked decisions yet.

PR: `Dogfood framework internals and remove duplication`

Branch: `codex/framework-dogfood-audit`

## Purpose

Working list of ambiguities, risks, and follow-up framework decisions from the review. Positive observations were removed so this file can be used as a decision checklist.

## Risks And Open Concerns

### 1. PR Size

The PR touches many surfaces:

- field types
- naming
- records
- auth
- fixtures
- patches
- schema prune
- Studio API
- Studio tests
- docs
- JSON schemas
- YAML parsing
- system writers
- permissions

The blast radius is high.

Future work should be split into smaller PRs, for example:

1. shared field type behavior
2. system writer dogfooding
3. YAML metadata helper
4. Studio API client cleanup
5. route identity cleanup
6. schema/patch contract cleanup

### 2. `SystemMutationOptions` Is Boolean-Heavy

Status: locked for implementation.

Current shape:

```go
type SystemMutationOptions struct {
    RunHooks      bool
    WriteActivity bool
    ReturnRecord  bool
    Bootstrap     bool
}
```

This works, but boolean combinations are easy to misuse.

Potentially invalid or confusing combinations:

- `RunHooks=true`, `ReturnRecord=false`
- `Bootstrap=true`, `WriteActivity=true`
- `RunHooks=false`, `WriteActivity=true`

Possible future shape:

```go
type SystemMutationPolicy string

const (
    SystemMutationBootstrap SystemMutationPolicy = "bootstrap"
    SystemMutationSilent    SystemMutationPolicy = "silent"
    SystemMutationFramework SystemMutationPolicy = "framework"
    SystemMutationFull      SystemMutationPolicy = "full"
)
```

Alternative: constructors like:

- `SystemMutationOptions.Bootstrap()`
- `SystemMutationOptions.Silent()`
- `SystemMutationOptions.FrameworkOnly()`

The current implementation validates at least one invalid combination, but the API shape remains easy to misuse.

Decision:

Replace the boolean-heavy mutation options with one named mutation policy. The goal is not to change runtime behavior; it is to make the allowed behavior explicit and prevent nonsensical combinations.

Preferred shape:

```go
type SystemMutationPolicy string

const (
    SystemMutationBootstrap SystemMutationPolicy = "bootstrap"
    SystemMutationSilent    SystemMutationPolicy = "silent"
    SystemMutationFramework SystemMutationPolicy = "framework"
    SystemMutationFull      SystemMutationPolicy = "full"
)
```

Policy meaning:

| Policy | Meaning |
| --- | --- |
| `bootstrap` | Core setup/bootstrap writes. No hooks, no activity. Used while metadata or schema state may still be coming online. |
| `silent` | Metadata-backed internal write with no hooks and no activity. Useful for Activity itself, patch ledger, and other system writes that must not recurse. |
| `framework` | Framework behavior only. Run framework hooks/activity as needed, but do not run app hooks. |
| `full` | Normal Record-like behavior. Use only when an internal write should behave like a public Record mutation. |

Implementation notes:

- Keep `ReturnRecord` separate only if the writer still needs a performance toggle.
- If `ReturnRecord` remains, prefer an explicit method split later, such as `InsertByIdentity` vs `InsertReturningByIdentity`, instead of packing more booleans into policy.
- Add tests for each policy and for invalid/default behavior.
- Replace call sites that currently pass `{Bootstrap: true}` or empty options with the named policy.

### 3. Query Params Are Loose

Status: locked for implementation.

`recordquery.FromValues` treats every unknown query param as a filter field, except reserved keys like:

- `limit`
- `offset`
- `sort`

This is lean but can collide with future query params:

- `view`
- `fields`
- `search`
- `q`
- `cursor`
- `group`
- `include`
- `expand`
- `debug`

Possible future directions:

```txt
?filter[status]=open
?filter[owner]=123
?sort=-created-at
```

Or keep short syntax for now, but reserve a clear list of query keys.

Decision:

Keep the short Record query syntax:

```txt
?status=open
?owner=123
?sort=-created-at
```

Do not switch to `filter[...]` as the canonical API.

Create one framework-owned reserved words registry:

```txt
internal/reserved/
  words.yml
  reserved.go
  reserved_test.go
```

Use terse YAML categories:

```yaml
slugs:
  - api
  - assets
  - health
  - login
  - logout
  - setup
  - settings
  - me
  - files
  - jobs
  - audit
  - admin
  - studio

fields:
  - id
  - name
  - created-at
  - updated-at
  - limit
  - offset
  - sort
  - view
  - fields
  - search
  - q
  - cursor
  - group
  - include
  - expand
  - debug

queries:
  - limit
  - offset
  - sort
  - view
  - fields
  - search
  - q
  - cursor
  - group
  - include
  - expand
  - debug

entities:
  - app
  - entity
  - field
  - record
```

Expected package API:

```go
reserved.IsSlug(value)
reserved.IsField(value)
reserved.IsQuery(value)
reserved.IsEntity(value)

reserved.Slugs()
reserved.Fields()
reserved.Queries()
reserved.Entities()
```

Implementation notes:

- Use `internal/reserved/words.yml` as the single framework source for reserved words.
- Reject route slugs that collide with reserved `slugs`.
- Reject authored field names that collide with reserved `fields`.
- `fields` should include reserved query keys so short query syntax cannot collide with future framework query parameters.
- Use `queries` in `recordquery.FromValues` instead of scattered reserved query constants.
- Use the reserved registry from schema validation and JSON schema contract tests.
- Keep the registry framework-owned, not app-configurable.

### 4. Frontend And Backend Query Limits Are Duplicated

Status: locked for implementation.

Backend:

```txt
DefaultLimit = 50
MaxLimit = 2500
```

Frontend:

```txt
recordListDefaultLimit = 50
recordListMaxLimit = 2500
recordListPageSizes = [20, 100, 500, recordListMaxLimit]
```

This is drift-prone. Backend should own the Record list pagination policy, and Studio should read it from the backend instead of hardcoding matching constants.

Decision:

```yaml
record-list:
  default-limit: 50
  max-limit: 2500
  page-sizes: [20, 100, 500, 2500]
```

API behavior:

- `limit` remains a query parameter.
- Missing `limit` uses `default-limit`.
- `limit > max-limit` rejects with `invalid_request`; do not silently clamp.
- API clients may request any positive `limit` up to `max-limit`; they do not have to use one of the configured `page-sizes`.
- Studio only offers backend-provided `page-sizes`.

Contract:

- Backend is the source of truth for Record list pagination policy.
- Frontend should not hardcode page sizes after implementation.
- `page-sizes` are UI choices.
- `max-limit` is the API safety cap.
- `default-limit` is the default for API and Studio list views.
- Expose through platform metadata/config, likely a future `GET /api/v1/platform` endpoint.

### 5. System Fields Are Still Split Between Backend And Studio

Status: locked for implementation.

Backend system fields live in `storage_contract.go`.

Studio still has its own system-field helper for:

- `id`
- `name`
- `created-at`
- `updated-at`

Studio should not invent system field semantics. The backend metadata API should eventually return enough system field metadata that Studio keeps only UI presentation rules.

Decision:

Every Record table gets these framework-owned system fields automatically:

```txt
id
name
created-at
updated-at
```

They are not declared in Entity YAML and are not stored as normal authored `fields`.

Backend is the source of truth for:

- which system fields exist
- their labels
- their types
- their value kinds
- their storage/list behavior
- their Studio editor/display hints

Backend exposes them through metadata as:

```txt
system-fields
```

Studio should consume `system-fields` from backend metadata.

Studio should not own a duplicated list of system fields.

Studio may keep local presentation choices only, for example:

- hide `id` from normal forms/lists
- show `name` as record identity
- place timestamps in list/detail UI

Do not add new metadata props right now:

```txt
system
writable
sortable
filterable
write-policy
```

Use existing resolved metadata only:

```txt
stored
write-only
listable
name-renderable
value-kind
studio.editor
studio.display
```

Future direction:

Permissions and backend validation decide what can be written. Studio should not infer write rules by inventing field semantics locally.

### 6. `MetadataFieldByName` Naming May Be Misleading

Status: locked for implementation.

`MetadataFieldByName` returns authored fields or a supported system field, but the review notes that it appears to special-case only `name`.

Backend storage contract defines system fields:

- `id`
- `name`
- `created-at`
- `updated-at`

If only `name` is matchable, the function name should be sharper.

Possible names:

- `MatchableMetadataFieldByName`
- `RecordMatchFieldByName`
- `MetadataOrNameFieldByName`

Otherwise a caller may assume `id`, `created-at`, and `updated-at` are supported through the same path.

Decision:

This is a naming and contract clarity cleanup, not a behavior change.

Fixtures should continue to match by stable public identity:

```yaml
match: [name]
```

or by unique authored fields / unique authored-field constraints.

Fixtures should not match by `id`, because `id` is an internal database-generated value and does not exist before insert. Fixtures should also not treat `created-at` or `updated-at` as normal addressable fields.

Rename:

```go
MetadataFieldByName
```

to:

```go
RecordAddressableFieldByName
```

Contract:

```txt
MetadataFieldsByName = authored Entity fields only.
RecordAddressableFieldByName = authored fields plus the public system identity field `name`.
system-fields metadata = full backend-described system fields for Studio/SDK display.
```

Implementation notes:

- Keep `name` as the only system field accepted by fixture/match helpers.
- Do not allow `id`, `created-at`, or `updated-at` through this helper.
- Do not broaden fixture behavior in this cleanup.
- Future importer/update flows may define a separate broader lookup that supports `id`, `name`, unique authored fields, and unique constraints.

### 7. Route Reserved Slugs May Be Too Narrow

Status: locked for implementation.

Route identity cleanup is good, but root route reservations should be stronger.

Decision:

Only reserve root slugs that dygo owns as concrete framework routes or near-term framework routes.

Reserved root slugs:

- `api`
- `health`
- `login`
- `logout`
- `setup`
- `me`
- `assets`

Do not reserve these yet:

- `settings`
- `files`
- `jobs`
- `audit`
- `admin`
- `studio`

Those are product/module concepts. Reserving them too early blocks app authors from using common Entity route slugs before dygo actually owns those root routes.

Rules:

- Normal and Single Entities cannot use reserved root slugs.
- Collection Entities have no slug, so this check does not apply to them.
- App authors can still use these as Entity keys; they just need a different public route slug.

Implementation notes:

- Store this list once in `internal/reserved/words.yml` under `slugs`.
- Backend catalog validation should use `reserved.IsSlug`.
- Remove backend hardcoded `rootReservedSlugs`.
- Remove or generate Studio's duplicated `rootReservedSlugs`.
- Studio can keep `entityChildReservedSlugs = ["new"]` separately because that protects `/:entity/new`, not a root route.

### 8. Collection Folder Convention Needs Verification

Status: locked for implementation.

The PR derives Entity identity from file path and uses computed `Entity.Name`.

Need to verify or finish the exact locked convention:

```txt
entities/<entity>.yml
entities/<entity>/<entity>.yml
entities/<entity>/<collection-entity>.yml
```

Rules to verify:

- no automatic prefixing
- any other `.yml` inside the folder is a collection Entity
- collection Entity must be referenced by exactly one parent collection field
- no reusable collection Entities in v1

Decision:

Filename always defines Entity key. There are no special filenames.

Examples:

```txt
entities/invoice.yml              -> invoice
entities/invoice/invoice.yml      -> invoice
entities/invoice/invoice-item.yml -> invoice-item
entities/invoice/index.yml        -> index
entities/invoice/entity.yml       -> entity
```

Do not reject `entity.yml`, `_entity.yml`, or `index.yml` just because of the filename. If a filename is otherwise valid, it defines that Entity key.

Simple and folder parent forms are equivalent:

```txt
entities/invoice.yml
entities/invoice/invoice.yml
```

Having both should still fail as duplicate parent definitions.

Inside an Entity folder:

- self-named `.yml` file is the parent Entity
- every other `.yml` file is a collection Entity
- collection files require the self-named parent file
- no automatic prefixing

Collection usage rules:

- Collection Entity must be referenced exactly once by a `type: collection` field in its parent.
- Reject unused collection Entities.
- Reject the same collection Entity referenced by more than one field.
- Reject collection fields targeting normal Entities.
- Reject collection fields targeting Single Entities.
- Reject collection fields targeting collection Entities owned by another parent.
- Reject link fields targeting collection Entities.
- No reusable collection Entities in v1.

Routeability:

Collection Entities are non-routeable.

That means:

- no public route slug for collection Entities
- no Studio nav entry
- no direct metadata lookup by slug
- no normal record endpoints by public slug
- internal lookup by `{app, key}` still works

Implementation implications:

- `entity.slug` should be nullable for collection Entities.
- Studio metadata type should allow `slug: string | null`.
- Route slug uniqueness should ignore collection Entities without a slug.
- Core `entity.slug` should remain unique when present, but no longer be required.
- Framework unique semantics should follow PostgreSQL defaults: unique fields and unique constraints allow multiple `NULL` values unless the participating fields are required.
- Do not add custom uniqueness handling for nullable `slug`; a normal unique constraint is enough.
- Remove the existing folder special-filename rejection for `entity.yml`, `_entity.yml`, and `index.yml`.

### 9. Field Type Behavior Could Become A Dumping Ground

The field type behavior contract is powerful, but it could grow into a bag of cross-layer flags.

If it grows too much later, split into focused behavior groups:

- `StorageBehavior`
- `APIBehavior`
- `StudioBehavior`
- `NamingBehavior`
- `ValidationBehavior`

Do not split prematurely. Just watch the shape.

### 10. `corevalues` Must Stay Tied To Core Metadata

`corevalues` centralizes select values for:

- app status
- session status
- activity kind
- activity operation
- activity status

Go constants and Core Entity YAML select options must stay aligned through contract tests.

## Pre-Merge Checks To Decide

1. Path-derived Entity naming convention coverage.
2. `is-collection` and folder-implied collection inference.
3. Route reserved slugs.
4. `MetadataFieldByName` naming and behavior.
5. `SystemMutationOptions` invalid-combination tests or stronger API shape.

## Watch List

- large PR size
- field type behavior breadth
- frontend/backend query limit duplication
- generic URL filter syntax
- Studio hardcoded system fields

## Pending Broader Framework Decisions

These are broader framework areas not solved by this PR.

### 1. Singular CLI Commands

Move toward singular commands:

```txt
dygo app
dygo entity
dygo fixture
dygo secret
dygo hook
```

Instead of:

```txt
dygo apps
dygo entities
dygo fixtures
dygo secrets
dygo hooks
```

### 2. `dygo dev` vs `dygo serve`

Separate development orchestration from runtime serving:

```txt
dygo dev
dygo serve
```

### 3. Route Registry

Build:

```txt
dygo route list
dygo route validate
dygo route resolve /lead
```

Root-mounted Studio needs this.

### 4. Studio Record Route Should Use Record Name

Move from:

```txt
/:entity/:id
```

to:

```txt
/:entity/:name
```

or at least plan that migration.

### 5. Permission Explain

Add:

```txt
dygo permission check
dygo permission explain
```

This will matter once field-level and row-level rules arrive.
