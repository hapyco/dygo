# Records

Records are saved data created from an Entity.

dygo's first Record runtime is generic. It uses persisted Core metadata to map Entity route slugs, app-scoped Entity identity, and Field names to PostgreSQL tables and columns, so the framework does not need handwritten handlers for each Entity.

Run metadata sync before using the Record API:

```sh
go run ./cmd/dygo migrate
go run ./cmd/dygo serve
```

Record API routes require a valid `dygo_session` cookie from the auth API and an allowed Entity permission.

`{entity}` is the Entity route slug. It defaults to Entity `name`, but apps can set `route.slug` to keep URLs stable when multiple apps define the same Entity name.

## API

```txt
GET    /api/v1/records/{entity}?limit=50&offset=0&status=Open&sort=-created-at,name
GET    /api/v1/records/{entity}/{id}
GET    /api/v1/records/{entity}/name/{name}
GET    /api/v1/records/{entity}/single
GET    /api/v1/records/{entity}/{id}/activity?limit=50&offset=0
POST   /api/v1/records/{entity}
PATCH  /api/v1/records/{entity}/{id}
PATCH  /api/v1/records/{entity}/single
DELETE /api/v1/records/{entity}/{id}
```

List endpoints default to `limit=50` and `offset=0`. The maximum limit is `2500`. Records are ordered by `id ASC` unless `sort` is provided.

Use `GET /api/v1/records/{entity}/name/{name}` to read exactly one Record by its stable system `name`. URL-encode `{name}` as a path segment.

For Entities marked `is-single: true`, use `GET /api/v1/records/{entity}/single` and `PATCH /api/v1/records/{entity}/single`. dygo owns the singleton Record name and seeds the one allowed row during metadata sync. Normal list, create, and delete operations return `invalid_request` for Single Entities.

Exact filters use direct Field query params:

```txt
GET /api/v1/records/lead?status=Open&enabled=true
```

Filters support visible DB-backed Fields and system fields: `id`, `name`, `created-at`, and `updated-at`. The reserved query params `limit`, `offset`, and `sort` cannot be used as HTTP filter names in v1. Write-only fields such as `password` and non-storage fields such as `child-table` cannot be filtered.

Sorting uses a comma-separated `sort` value. Prefix a field with `-` for descending order:

```txt
GET /api/v1/records/lead?sort=-created-at,name
```

dygo appends `id ASC` as a deterministic tie-breaker unless `id` is already in the sort list. `meta.count` is the number of Records in the returned page, not a total matching row count.

Create and update bodies use a `data` envelope:

```json
{"data":{"email":"a@example.com","full-name":"A User"}}
```

Responses also use envelopes:

```json
{"data":{"id":1,"name":"a@example.com","email":"a@example.com"}}
```

```json
{"data":[],"meta":{"limit":50,"offset":0,"count":0}}
```

Errors use the same API error shape as the metadata API:

```json
{"error":{"code":"validation_error","message":"required field is missing","details":{"field":"email"}}}
```

## Field Names

JSON uses dygo metadata names, not SQL column names.

Examples:

```txt
full-name -> "full-name"
started-at -> "started-at"
user link field -> "user"
```

System fields are returned as:

```txt
id
name
created-at
updated-at
```

`id` is dygo's internal numeric identity. `name` is the stable Record identifier generated from Entity `naming` metadata. System fields cannot be written in update request bodies. Create request bodies may include `name` only when the Entity explicitly uses `naming.strategy: field` with `field: name`; otherwise dygo generates `name` during create.

## Supported Fields

Record runtime v1 supports DB-backed fields:

```txt
text
email
phone
password
long-text
int
bigint
decimal
currency
boolean
date
datetime
time
select
link
attachment
json
```

`password` fields accept plaintext strings in create and update requests, hash them before storage, and are never returned in list or detail responses.

`child-table` is not writable through Record APIs yet.

## Activity History

Generic Record mutations write append-only `activity` Records in the same transaction:

```txt
create -> Activity with a visible Record snapshot
update -> Activity with field-level changes
delete -> Activity with the deleted visible Record snapshot
```

Write-only fields such as `password` are recorded by field name only with `redacted: true`; their values are not stored in Activity. Activity is the product timeline/history stream for v1, not a compliance-grade audit log.

Scoped Activity can be read for one Entity/Record pair:

```txt
GET /api/v1/records/{entity}/{id}/activity?limit=50&offset=0
```

The endpoint returns newest-first Activity ordered by `created-at DESC, id DESC`:

```json
{"data":[],"meta":{"limit":50,"offset":0,"count":0}}
```

Activity lookup uses the target Entity and Record ID, so history can still be read after the live Record row has been deleted. It requires authentication and `read` permission on the target Entity with the target Record ID. It does not require generic `activity` Entity permission.

Activity items include `id`, `created-at`, `entity`, `record-id`, `kind`, `operation`, `status`, `title`, `message`, `actor`, `changes`, `snapshot`, and `details`. `actor` is `null` when no user caused the change, otherwise it contains `id`, `email`, and `full-name`.

## Boundaries

`PATCH` is dygo's update operation and only changes fields included in the request body. `PUT` is not part of v1.

`DELETE` performs a hard delete in v1.

This layer requires authentication and checks Entity permissions through the single internal permission engine.

Permission mapping:

```txt
list/detail -> read
create -> create
update -> update
delete -> delete
```

Administrator users are privileged through the permission engine. Sharing, row-level filtering, owner rules, field-level permissions, advanced list operators, saved views, and Studio list UI are future layers on the same engine.
