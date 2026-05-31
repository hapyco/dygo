# App SDK

The App SDK is the Go package app code compiles against:

```go
import "github.com/hapyco/dygo/pkg/sdk"
```

Everything under `internal/` is private framework implementation. App-owned hooks and Jobs should only depend on `pkg/sdk` and normal Go packages.

The current supported package is `pkg/sdk`. A shorter `pkg/dygo` import path is coming soon if the public Go API graduates from the current SDK shape.

## SDK Vs HTTP API

```txt
Go SDK   - Go code imported by dygo apps
HTTP API - Network API used by clients and Studio
```

App SDK code is trusted server-side code. It does not run the same permission path as a browser or HTTP client calling the dygo HTTP API.

## Current Surface

The current SDK exposes:

- Record lifecycle hook types and registration
- transactional Record reads and writes inside hooks
- durable Job handler types and registration
- Job enqueueing from hooks and Jobs
- project runner integration types

## Record Hooks

Record hooks register functions for Entity lifecycle events:

```go
func Register(registry sdk.RecordHookRegistry) error {
	return registry.RegisterEntity("crm", "contact", sdk.RecordAfterCreate, "send-welcome", SendWelcome)
}

func SendWelcome(ctx context.Context, hook sdk.RecordHook) error {
	return nil
}
```

Supported events:

```txt
before-validate
validate
before-create
after-create
before-update
after-update
before-delete
after-delete
```

Hooks receive `sdk.RecordHook`, which includes the Entity identity, current input, old/new Record snapshots, changes, and SDK services.

## Record Access

Hooks read and write metadata-backed Records through `hook.Records`:

```go
record, err := hook.Records.Get(ctx, "crm", "contact", 42)
created, err := hook.Records.Create(ctx, "crm", "activity", sdk.RecordInput{
	"subject": json.RawMessage(`"Welcome"`),
})
updated, err := hook.Records.Update(ctx, "crm", "contact", 42, sdk.RecordInput{
	"status": json.RawMessage(`"Active"`),
})
err := hook.Records.Delete(ctx, "crm", "contact", 42)
```

Record access uses app-scoped Entity identity:

```txt
<app>, <entity>
```

Do not use route slugs as SDK Entity identity.

Hook Record writes run dygo framework hooks, such as Activity, but do not re-enter app hooks.

## Jobs

Generated Job files expose one `Run` function:

```go
func Run(ctx context.Context, job sdk.JobExecution) error {
	return nil
}
```

Job handlers and transactional Record hooks can enqueue durable background work:

```go
execution, err := job.Jobs.Enqueue(ctx, "crm", "send-welcome-email", payload, sdk.EnqueueOptions{
	IdempotencyKey: "email:welcome:contact-42",
	Priority:       0,
	RunAfter:       time.Now().Add(10 * time.Minute),
})
```

Inside a Record hook, use `hook.Jobs.Enqueue` with the same arguments.

Job access uses app-scoped Job identity:

```txt
<app>, <job>
```

Do not use labels or routes as SDK Job identity.

## Runtime Rules

```txt
hooks   - run inside the current Record transaction
jobs    - run outside user requests
pages   - coming soon
reports - coming soon
```

## Coming Soon

Planned SDK surfaces include:

```txt
dygo.Files         - public/private file storage
dygo.Permissions   - permission checks
dygo.Actor         - current user/session identity
dygo.Logger        - structured app logging
dygo.Config        - app/runtime config reads
dygo.Secrets       - controlled secret reads
dygo.Metadata      - Entity and Field metadata reads
```
