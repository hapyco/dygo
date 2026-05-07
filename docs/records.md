# Records

Records are saved data created from an Entity.

dygo's first Record runtime is generic. It uses persisted Core metadata to map Entity names and Field names to PostgreSQL tables and columns, so the framework does not need handwritten handlers for each Entity.

Run metadata sync before using the Record API:

```sh
go run ./cmd/dygo migrate
go run ./cmd/dygo serve
```

## API

```txt
GET    /api/v1/records/{entity}?limit=50&offset=0
GET    /api/v1/records/{entity}/{id}
POST   /api/v1/records/{entity}
PATCH  /api/v1/records/{entity}/{id}
DELETE /api/v1/records/{entity}/{id}
```

List endpoints default to `limit=50` and `offset=0`. The maximum limit is `100`. Records are ordered by `id ASC`.

Create and update bodies use a `data` envelope:

```json
{"data":{"email":"a@example.com","full-name":"A User"}}
```

Responses also use envelopes:

```json
{"data":{"id":1,"email":"a@example.com"}}
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
created-at
updated-at
```

System fields cannot be written in create or update request bodies.

## Supported Fields

Record runtime v1 supports DB-backed fields:

```txt
text
email
phone
long-text
int
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

`child-table` is not writable through Record APIs yet.

## Boundaries

`PATCH` is dygo's update operation and only changes fields included in the request body. `PUT` is not part of v1.

`DELETE` performs a hard delete in v1.

This layer does not enforce permissions yet. The internal permission engine exists, but guarding these APIs is a follow-up task.
