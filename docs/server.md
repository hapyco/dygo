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

These endpoints are generic. dygo does not create per-Entity routes such as `/api/users` or `/api/leads`. Future Record APIs should use one metadata-powered runtime surface such as `/api/v1/records/{entity}`.

## Shutdown

Stop the server with `Ctrl-C`.

The CLI listens for interrupt and termination signals and asks the HTTP server to shut down cleanly.

## Boundaries

The current server includes health and read-only metadata APIs. It does not include authentication, permission enforcement, Studio rendering, Record CRUD, or per-Entity controllers yet.
