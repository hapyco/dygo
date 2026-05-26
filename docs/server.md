# Server

`dygo serve` starts the local dygo HTTP server.

The server address comes from:

```txt
dygo.yml
```

The default address is:

```txt
127.0.0.1:6790
```

`dygo dev` loads the development database credentials by default and starts the local development experience:

```sh
go run ./cmd/dygo dev
```

In a source checkout with `apps/studio/ui/package.json`, `dygo dev` starts Studio's development asset server internally and proxies Studio pages through dygo. The browser-facing address stays `http://127.0.0.1:6790/`, so Studio and `/api/v1/...` share one origin during development.

Generated projects serve Studio from `.dygo/apps/studio/ui/dist` when that cache exists. Release builds also include bundled Studio assets, and `dygo new` / `dygo upgrade` refresh the generated-project cache when the running dygo binary has those assets.

`dygo serve` starts the runtime server and uses generated-project or bundled Studio assets. If no generated-project cache or bundled release assets are available, `dygo serve` exits before listening instead of serving an API-only site.

Use another encrypted environment with `--env`:

```sh
go run ./cmd/dygo serve --env staging
```

Use `dygo dev --studio-dev-url` only when the Studio asset server is already running somewhere else:

```sh
go run ./cmd/dygo dev --studio-dev-url http://127.0.0.1:6791
```

The server opens and pings PostgreSQL before it starts listening. It does not run `dygo db migrate` automatically; run metadata sync before serving runtime metadata.

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

`POST /api/v1/auth/login` is public. Metadata and Record API routes require a valid session and use the same permission engine. Metadata list endpoints filter unreadable Entities; Record API routes require the relevant Entity action.

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

`{entity}` in metadata and Record routes is the Entity slug.

Metadata visibility is permission-aware. `GET /api/v1/entities` returns only Entities the current user can read, while `GET /api/v1/entities/{entity}/meta` returns `403 forbidden` for a known Entity the user cannot read. App metadata is visible when the user can read Core `app` metadata or at least one Entity owned by that App.

## Record API

The first Record API is also generic and metadata-powered:

```txt
GET    /api/v1/records/{entity}?limit=50&offset=0&status=Open&sort=-created-at,name
GET    /api/v1/records/{entity}/{id}
GET    /api/v1/records/{entity}/name/{name}
GET    /api/v1/records/{entity}/{id}/activity?limit=50&offset=0
POST   /api/v1/records/{entity}
PATCH  /api/v1/records/{entity}/{id}
DELETE /api/v1/records/{entity}/{id}
```

Record APIs read persisted Core metadata to map Entity slugs, Field names, and storage columns. `{entity}` is the slug, defaulting to the file-derived Entity key. Run `dygo db migrate` before serving Records so metadata tables and Entity storage tables are in sync.

`GET /api/v1/records/{entity}/name/{name}` returns one Record by system `name`; URL-encode `{name}` as a path segment.

Record request bodies use a `data` envelope:

```json
{"data":{"email":"a@example.com","full-name":"A User"}}
```

Record responses use dygo metadata names, including system fields:

```json
{"data":{"id":1,"name":"a@example.com","created-at":"2026-05-07T12:00:00Z","updated-at":"2026-05-07T12:00:00Z","email":"a@example.com"}}
```

Write-only fields such as `password` are accepted in create and update requests, but are not returned in responses.

List responses include pagination metadata:

```json
{"data":[],"meta":{"limit":50,"offset":0,"count":0}}
```

Record lists support exact filters with direct Field query params and multi-field sorting through `sort`. `limit`, `offset`, and `sort` are reserved query params. Sorting uses `-field` for descending order and appends `id ASC` as a deterministic tie-breaker. Advanced operators, saved views, field-level permissions, row-level filters, and Studio list UI are future layers.

`PATCH` is the update operation and only changes fields provided in the request body. `DELETE` performs a hard delete in v1.

Scoped Record Activity is read through `GET /api/v1/records/{entity}/{id}/activity`. It returns newest-first Activity for the target Entity/Record pair and does not require the live Record row to still exist.

Record API permissions are checked through the single internal permission engine:

```txt
GET list/detail -> read
GET activity -> read
POST -> create
PATCH -> update
DELETE -> delete
```

Administrator users are allowed by the engine before flat role permissions are checked.

## Shutdown

Stop the server with `Ctrl-C`.

The CLI listens for interrupt and termination signals, asks the HTTP server to shut down cleanly, and stops the auto-started Studio dev server when one was started by `dygo dev`.

## Boundaries

The current server includes health, session auth, read-only metadata APIs, generic Record CRUD APIs with parent-owned collection rows, Entity permission enforcement for Records, static serving for generated-project or bundled Studio assets, and a development proxy for Studio when run through `dygo dev`. The server does not include per-Entity controllers, row-level sharing rules, workflow hooks, or audit logging yet.
