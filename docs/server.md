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

## Shutdown

Stop the server with `Ctrl-C`.

The CLI listens for interrupt and termination signals and asks the HTTP server to shut down cleanly.

## Boundaries

This server skeleton does not include database access, authentication, Studio rendering, metadata APIs, or Record APIs yet.
