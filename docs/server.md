# Server

`dygo serve` starts the local dygo HTTP server.

The server address comes from:

```txt
configs/dygo.yaml
```

The default address is:

```txt
127.0.0.1:6790
```

`dygo serve` loads the development database credentials by default:

```sh
go run ./cmd/dygo serve
```

Use another encrypted environment with `--env`:

```sh
go run ./cmd/dygo serve --env staging
```

The server opens and pings PostgreSQL before it starts listening. It does not run `dygo migrate` automatically; run metadata sync before serving runtime metadata.

## Health

The first server surface is:

```txt
GET /health
```

It returns:

```txt
ok
```

This endpoint is intentionally small. It only confirms that the HTTP process is accepting requests.

## Auth API

Studio-oriented auth uses an HttpOnly `dygo_session` cookie:

```txt
POST /api/v1/auth/login
POST /api/v1/auth/logout
GET  /api/v1/auth/me
```

`POST /api/v1/auth/login` is public. Metadata and Record API routes require a valid session. Record API routes also require an allowed Entity permission.

## Metadata API

The first runtime API is read-only and powered by persisted Core metadata records:

```txt
GET /api/v1/apps
GET /api/v1/apps/{app}
GET /api/v1/entities
GET /api/v1/entities/{entity}/meta
```

Responses use stable JSON envelopes:

```json
{"data":[]}
```

Errors use:

```json
{"error":{"code":"not_found","message":"entity not found","details":{"entity":"lead"}}}
```

These endpoints are generic. dygo does not create per-Entity routes such as `/api/users` or `/api/leads`.

## Record API

The first Record API is also generic and metadata-powered:

```txt
GET    /api/v1/records/{entity}?limit=50&offset=0
GET    /api/v1/records/{entity}/{id}
POST   /api/v1/records/{entity}
PATCH  /api/v1/records/{entity}/{id}
DELETE /api/v1/records/{entity}/{id}
```

Record APIs read persisted Core metadata to map Entity names, Field names, and storage columns. Run `dygo migrate` before serving Records so metadata tables and Entity storage tables are in sync.

Record request bodies use a `data` envelope:

```json
{"data":{"email":"a@example.com","full-name":"A User"}}
```

Record responses use dygo metadata names, including system fields:

```json
{"data":{"id":1,"created-at":"2026-05-07T12:00:00Z","updated-at":"2026-05-07T12:00:00Z","email":"a@example.com"}}
```

Write-only fields such as `password` are accepted in create and update requests, but are not returned in responses.

List responses include pagination metadata:

```json
{"data":[],"meta":{"limit":50,"offset":0,"count":0}}
```

`PATCH` is the update operation and only changes fields provided in the request body. `DELETE` performs a hard delete in v1.

Record API permissions are checked through the single internal permission engine:

```txt
GET list/detail -> read
POST -> create
PATCH -> update
DELETE -> delete
```

Administrator users are allowed by the engine before flat role permissions are checked.

## Shutdown

Stop the server with `Ctrl-C`.

The CLI listens for interrupt and termination signals and asks the HTTP server to shut down cleanly.

## Boundaries

The current server includes health, session auth, read-only metadata APIs, generic Record CRUD APIs, and Entity permission enforcement for Records. The server does not include Studio rendering, per-Entity controllers, child table storage, row-level sharing rules, workflow hooks, or audit logging yet.
