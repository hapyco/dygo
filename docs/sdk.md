# App SDK

The App SDK is the public Go package app code can compile against.

```txt
pkg/sdk/     - Public app-facing Go API
internal/    - Private framework implementation
```

Use the App SDK from:

```txt
hooks        - Record lifecycle behavior
jobs         - Background work
pages        - Custom page actions
reports      - Custom report queries
workflows    - Future workflow actions
patches      - Future app migrations
```

## SDK Vs API

```txt
App SDK  - Go code imported by dygo apps
HTTP API - Network API used by clients
```

## Current Surface

```txt
Record hooks
Record data access
Project runner integration
```

```go
import "github.com/dygo-dev/dygo/pkg/sdk"
```

## Record Access

App code reads and writes metadata-backed Records through `dygo.Records`.

```go
record, err := dygo.Records.Get(ctx, "crm", "lead", 42)
created, err := dygo.Records.Create(ctx, "crm", "activity", input)
updated, err := dygo.Records.Update(ctx, "crm", "lead", 42, input)
err := dygo.Records.Delete(ctx, "crm", "lead", 42)
```

Record access uses app-scoped Entity identity:

```txt
<app>, <entity>
```

Do not use route slugs as SDK Entity identity.

## Future Surfaces

```txt
dygo.Records       - Metadata-backed Record access
dygo.Files         - Public/private file storage
dygo.Jobs          - Background job enqueueing
dygo.Permissions   - Permission checks
dygo.Actor         - Current user/session identity
dygo.Logger        - Structured app logging
dygo.Config        - App/runtime config reads
dygo.Secrets       - Controlled secret reads
dygo.Metadata      - Entity and Field metadata reads
```

## Runtime Rules

```txt
hooks    - Run inside the current Record transaction
jobs     - Run outside user requests
pages    - Run inside request context
reports  - Run inside report request context
```

App SDK code is trusted server-side code. It does not mean the same thing as a browser or HTTP client calling the dygo HTTP API.
