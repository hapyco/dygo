# App SDK

The App SDK is the Go package app code compiles against:

```go
import "github.com/hapyco/dygo/pkg/dygo"
```

Everything under `internal/` is private framework implementation. App-owned hooks and Jobs should only depend on `pkg/dygo` and normal Go packages.

The supported public package is `pkg/dygo`.

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
- Job enqueueing, queued cancel, and failed retry from hooks, Jobs, and Entity actions
- best-effort and strict persisted Log helpers
- durable in-app notifications with optional email delivery
- project runner integration types

## Tree Records

Use `records.Tree(app, entity)` for an Entity with Tree metadata. The returned `TreeData` retains the caller's actor and transaction. Use it from the same Record service supplied to Hooks, actions, or Jobs.

```go
tree := hook.Records.Tree("hr", "department")
children, err := tree.Children(ctx, departmentID, dygo.RecordListParams{Limit: 20})
```

`Roots`, `Children`, and `Descendants` return paginated nodes. `Ancestors` returns root-to-parent order; `Path` includes the selected Record. `Search` accepts `TreeSearchParams`, which embeds Record list filters and pagination. It returns matching nodes and their readable ancestor context. `ExcludeSubtree` excludes an anchor and its descendants, for example when selecting a new parent.

Anchors use internal numeric Record IDs, as other SDK methods do. `Move(ctx, recordID, parentName)` accepts a parent Record name; a nil parent makes the Record a root. Create roots and children with ordinary `RecordData.Create`. Moves, creates, and deletes use the same validation, permission, Hook, Activity, and transaction rules as other Record writes.

Use `AsActor` for user-scoped access. Tree access does not inherit permissions from parents. A full path or ancestor request fails if its chain is not readable. Search can return an independently readable match with `PathUnavailable` instead of exposing hidden ancestors. Child indicators describe readable children only.

## Record Hooks

Record hooks register functions for Entity lifecycle events:

```go
func Register(registry dygo.RecordHookRegistry) error {
	return registry.RegisterEntity("crm", "contact", dygo.RecordAfterCreate, "send-welcome", SendWelcome)
}

func SendWelcome(ctx context.Context, hook dygo.RecordHook) error {
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

Hooks receive `dygo.RecordHook`, which includes the Entity identity, current input, old/new Record snapshots, changes, and SDK services.

## Record Access

Hooks read and write metadata-backed Records through `hook.Records`:

```go
record, err := hook.Records.Get(ctx, "crm", "contact", 42)
created, err := hook.Records.Create(ctx, "crm", "activity", dygo.RecordInput{
	"subject": json.RawMessage(`"Welcome"`),
})
updated, err := hook.Records.Update(ctx, "crm", "contact", 42, dygo.RecordInput{
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

`RecordData` also provides permission-aware `Count`, `Exists`, `Aggregate`,
`GroupBy`, relationship filters, ordered `Lock`, and `Transaction`. Use
`AsActor` for user access. Use `AsPrivate(actor, reason)` for an Entity whose
metadata declares `is-private: true`; dygo reads `private-owner-field` and
automatically constrains every operation to that actor's owner Record. Use
`AsSystem` only for trusted system Entities and with a non-empty audit reason.

Entity actions receive the actor and transaction-scoped Records, Jobs, Files,
Timeline, and Notifications services. Register one action on one Entity with
`EntityActionRegistry.RegisterEntity`. Optional `confirm` text asks Studio to
confirm before running the action. Set `danger` when the confirm dialog is
destructive.

`FileData` uploads, attaches, opens, and removes private files. `TimelineData`
adds append-only comments and events to Core Activity.

## Jobs

Generated Job files expose one `Run` function:

```go
func Run(ctx context.Context, job dygo.JobExecution) error {
	return nil
}
```

Job handlers and transactional Record hooks can enqueue durable background work:

```go
execution, err := job.Jobs.Enqueue(ctx, "crm", "send-welcome-email", payload, dygo.EnqueueOptions{
	IdempotencyKey: "email:welcome:contact-42",
	Priority:       0,
	RunAfter:       time.Now().Add(10 * time.Minute),
})
```

Inside a Record hook, use `hook.Jobs.Enqueue` with the same arguments.

For a shared, compile-time payload type, define a `TypedJob` in the Job package:

```go
type WelcomePayload struct {
	ContactID string `json:"contact_id"`
}

var WelcomeJob = dygo.NewTypedJob[WelcomePayload]("crm", "send-welcome-email")

var handleWelcome = WelcomeJob.Handler(func(ctx context.Context, execution dygo.JobExecution, payload WelcomePayload) error {
	if payload.ContactID == "" {
		return fmt.Errorf("contact ID is required")
	}
	// Perform the work using execution services and payload.ContactID.
	return nil
})

// The generated runner discovers this Run function as usual.
func Run(ctx context.Context, execution dygo.JobExecution) error {
	return handleWelcome(ctx, execution)
}
```

Call `jobpkg.WelcomeJob.Enqueue(ctx, jobs, jobpkg.WelcomePayload{ContactID: "42"}, options)` from app code to enqueue that same Job with the typed payload. Custom runners can instead call `WelcomeJob.Register(registry, handler)`. Both paths use the existing Job registry and `JobData` interface; raw JSON handlers and enqueue calls continue to work.

The wrapper encodes and decodes standard JSON. A missing payload is decoded as JSON `null`; decode errors are returned as handler errors and follow the normal retry policy. Typed enqueueing catches Go type mismatches at compile time, but CLI, Schedule, Studio, and raw SDK inputs can still supply different JSON. Handlers must validate business requirements and account for payload changes across versions.

`JobData` can also cancel a queued Job Execution or retry a failed one by numeric id or execution name:

```go
cancelled, err := call.Jobs.CancelQueued(ctx, "42")
retried, err := call.Jobs.Retry(ctx, "42", "manual-retry:welcome:42")
```

Cancel only works while the execution is `queued`. Retry copies the failed payload into a new queued execution. Retry requires an idempotency key.

Job access uses app-scoped Job identity:

```txt
<app>, <job>
```

Do not use labels or routes as SDK Job identity.

## Logs

App code can write persisted diagnostic Logs through package helpers:

```go
dygo.Info(ctx, "Customer import started")
dygo.Error(ctx, "Customer import failed", err)
```

The helper functions are best-effort. Use `dygo.Log(ctx, dygo.LogEntry{...})` when code needs to handle persistence errors. See [Logs](logs.md) for the Log Entity contract and field mapping.

## Notifications

Actions, Hooks, and Jobs send user notifications through their `Notifications` service:

```go
_, err := call.Notifications.Send(ctx, dygo.NotificationMessage{
    Recipient:      "person@example.com",
    Title:          "Leave approved",
    Message:        "Your leave request was approved.",
    DeepLink:       "/hr-leave-request/HRL-2026-00001",
    Email:          true,
    IdempotencyKey: "hr:leave-approved:HRL-2026-00001",
})
```

`Recipient` is the Core User Record name. The idempotency key is unique for that recipient. `Send` creates the in-app Notification and, when requested, enqueues the Core email Job in the current transaction. SMTP delivery and retries happen after commit. Email failure does not remove the in-app Notification.

## Runtime Rules

```txt
hooks   - run inside the current Record transaction
jobs    - run outside user requests
pages   - metadata contract; rendering remains framework-owned
reports - coming soon
```

## Coming Soon

Planned SDK surfaces include:

```txt
dygo.Config        - app/runtime config reads
dygo.Secrets       - controlled secret reads
dygo.Metadata      - Entity, Field, and Page metadata reads
```

## Read a Record secret

Use `RecordData.DecryptSecret` only in explicit system code:

```go
value, err := hook.Records.AsSystem("send integration request").DecryptSecret(
    ctx, "crm", "connection", hook.RecordID, "token",
)
```

Ordinary SDK access and `AsActor`, including Administrator access, cannot decrypt
secrets. `AsSystem` requires a non-empty reason. Core and Business Apps use this
same SDK operation. Collection decryption uses the child Entity and saved row ID.
`errors.Is(err, dygo.ErrSecretUnset)` identifies an unset secret.

Decryption uses the current transaction. It records the target, field, reason,
actor context, and outcome through the framework Log writer. Audit write failure
prevents returning plaintext. The public contextual Log writer cannot replace
this security sink. Audit writes use the SDK's current queryer, so Hook
decryption uses the active Record transaction and does not require another pool
checkout.

The returned string is plaintext. Do not log it, add it to Job payloads, return it
through an Action response, or include it in errors. Use it only for the intended
server-side operation.

`RecordData.SecretStatus(ctx, app, entity, recordID)` returns `dygo.SecretStatus`
with presence maps, applying the SDK's normal read scope. Generic reads remain
write-only even in system mode.

## Trusted system Record writes

System Entities remain read-only through ordinary CRUD, Studio, fixtures, and
imports, including Administrator and `AsSystem` access. Trusted App code uses
`Records.System(reason)` to maintain its own system Entities:

```go
writer := hook.Records.System("refresh integration state")
record, err := writer.Upsert(ctx, "integration-state",
    dygo.RecordInput{"external-id": json.RawMessage(`"customer-42"`)},
    dygo.RecordInput{"status": json.RawMessage(`"ready"`)},
)
```

The writer provides `Create`, `Update`, `Delete`, and `Upsert`. Each accepts an
Entity key; the App is bound to the Hook, Entity action, or Job registration.
Pass the writer to App services through their caller. Empty reasons, absent App
scope, non-system Entities, and foreign App targets are rejected. Business Apps
cannot write Core system Entities through this API.

`Transaction`, `AsActor`, and `AsSystem` preserve the bound App. `AsSystem` changes
ordinary permission access and audit attribution; it does not grant system
Entity writes. The trusted writer uses the caller's transaction, records its
reason and available actor attribution in Activity, and runs Record validation,
naming, constraints, and secret handling. Framework hooks run; Business App
hooks do not re-enter.

Upsert matches must identify a metadata-defined unique key. Match values become
part of create input; conflicting input values are rejected. Concurrent insert
conflicts return the normal Record constraint error for the caller to handle.

App scope protects against accidental cross-App writes. Compiled App code remains
trusted in-process Go code. Bootstrap and silent writer policies stay internal.
See [structured system Record patches](patches.md) for the same capability in
App-owned post-sync patches.
