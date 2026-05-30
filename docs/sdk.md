# App SDK

The App SDK is the public Go package app code can compile against.

```txt
pkg/dygo/     - Future public app-facing Go API
pkg/sdk/      - Current public app-facing Go API
internal/     - Private framework implementation
```

`pkg/dygo` is the intended public package name. It keeps app code readable without import aliases:

```go
import "github.com/hapyco/dygo/pkg/dygo"

func BeforeCreate(ctx context.Context, hook dygo.RecordHook) error {
	return nil
}
```

This document is named `sdk.md` for now because it defines the app developer SDK sprint. The public concept can be called the Go API in user-facing docs. HTTP API documentation should live separately.

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
Go API   - Go code imported by dygo apps
HTTP API - Network API used by clients
```

## Current Surface

```txt
Record hooks
Record data access
Job handlers
Job enqueueing
Project runner integration
```

```go
import "github.com/hapyco/dygo/pkg/sdk"
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

## Jobs

Generated Job files expose one `Run` function:

```go
func Run(ctx context.Context, job sdk.JobExecution) error {
    return nil
}
```

Job handlers and transactional Record hooks can enqueue durable background work:

```go
execution, err := dygo.Jobs.Enqueue(ctx, "crm", "send-welcome-email", payload, sdk.EnqueueOptions{
    IdempotencyKey: "email:welcome:contact-42",
    Priority:       0,
    RunAfter:       time.Now().Add(10 * time.Minute),
})
```

Job access uses app-scoped Job identity:

```txt
<app>, <job>
```

Do not use labels or routes as SDK Job identity.

## Future Surfaces

```txt
dygo.Records       - Metadata-backed Record access
dygo.Files         - Public/private file storage
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
