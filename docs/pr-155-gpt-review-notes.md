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

### 3. Query Params Are Loose

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

### 4. Frontend And Backend Query Limits Are Duplicated

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

This is drift-prone. Later, either expose these through metadata/config or add contract coverage.

Also, `2500` may be high for normal operational UI. A more conservative set could be:

- 20
- 50
- 100
- 250

Larger data flows may belong to export/reporting.

### 5. System Fields Are Still Split Between Backend And Studio

Backend system fields live in `storage_contract.go`.

Studio still has its own system-field helper for:

- `id`
- `name`
- `created-at`
- `updated-at`

Studio should not invent system field semantics. The backend metadata API should eventually return enough system field metadata that Studio keeps only UI presentation rules.

### 6. `MetadataFieldByName` Naming May Be Misleading

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

### 7. Route Reserved Slugs May Be Too Narrow

Route identity cleanup is good, but root route reservations should be stronger.

Candidate reserved slugs:

- `api`
- `assets`
- `health`
- `login`
- `logout`
- `setup`
- `settings`
- `me`
- `files`
- `jobs`
- `audit`
- `admin`
- `studio`

Root-mounted Studio makes route collisions expensive later.

### 8. Collection Folder Convention Needs Verification

The PR derives Entity identity from file path and uses computed `Entity.Name`.

Need to verify or finish the exact locked convention:

```txt
entities/<entity>.yml
entities/<entity>/<entity>.yml
entities/<entity>/<collection-entity>.yml
```

Rules to verify:

- no `entity.yml`
- no `_entity.yml`
- no `index.yml`
- no automatic prefixing
- any other `.yml` inside the folder is a collection Entity
- collection Entity must be referenced by exactly one parent collection field
- no reusable collection Entities in v1

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
