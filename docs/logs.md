# Logs

Logs are persisted framework and app diagnostic events.

dygo stores Logs as Core Records so operators, builders, and agents can inspect runtime behavior in Studio without leaving the app. Logs are intentionally generic: the same Entity should hold framework errors, app information, Job diagnostics, hook failures, HTTP request problems, and future operational events.

The Log Entity is a heavily dogfooded Core Entity. dygo should use it for its own runtime diagnostics before asking app developers to rely on it.

## Why Logs Exist

Activity answers "what happened to this Record?"

Logs answer "what happened inside the system?"

Use Activity for product timelines and Record history. Use Logs for diagnostics, troubleshooting, and operational visibility.

Examples:

- an app hook catches a validation service failure
- a Job retries after a third-party API timeout
- a user-facing request fails with an unexpected internal error
- dygo detects invalid metadata during runtime startup
- a developer records useful context while testing a feature

## Log Entity

The Core app owns a system Entity with key `log`, label `Log`, and Studio navigation label `Logs`.

The Entity should be append-only by default. App code and framework code create Logs; normal app workflows should not edit them.

Proposed fields:

| Field | Type | Required | Purpose |
| --- | --- | --- | --- |
| `type` | `select` | yes | Log type, matching the common logger idea of severity or level. Stored values should be `Debug`, `Info`, `Warning`, `Error`, and `Panic`. |
| `source` | `select` | yes | Runtime surface that wrote the Log. Stored values should be `Framework`, `SDK`, `HTTP`, `Job`, `Hook`, `CLI`, and `Studio`. |
| `app` | `link` | no | Optional Core App that produced the Log. This should be set explicitly by the SDK or framework when app context is known, not fetched from another field. |
| `title` | `text` | yes | Short human-facing summary. Simple SDK calls should map their message here. |
| `message` | `long-text` | no | Optional full detail for any Log type, including error text, stack traces, tracebacks, success details, or diagnostic notes. |
| `trace-id` | `text` | no | Correlation ID shared across request, Job, hook, and nested work. |
| `reference-entity` | `link` | no | Optional Core Entity reference for the related Record type. |
| `reference-record-id` | `bigint` | no | Optional internal Record ID for the related Record. |
| `reference-record-name` | `text` | no | Optional stable Record name for the related Record. |
| `actor` | `link` | no | Optional User that caused the event. |
| `metadata` | `json` | no | Structured attributes, tags, and context that do not deserve first-class fields yet. |

All Log `link` fields should use `foreign-key: false` so old Logs survive target deletion, the same pattern used by Activity history.

Do not use `fetch.from` for `app`. Logs often belong to an app without belonging to a specific Record, and an explicit producer app should not be overwritten by a reference-derived value.

Do not add custom indexes in the first pass. Logs should start with the normal Core Record storage shape, and dygo should add indexes later only after Studio usage shows a repeated access pattern that needs one.

## Field Review

Common loggers and observability tools converge on a small set of concepts:

- level or severity
- message or body
- timestamp
- source, logger, scope, or resource
- error or exception details
- trace correlation
- structured attributes, tags, or context
- user context for application errors

The proposed Log fields cover those concepts without making the Entity too wide:

| Common concept | dygo field |
| --- | --- |
| level or severity | `type` |
| message or body | `title`, optionally `message` |
| timestamp | Core Record `created-at` |
| source, logger, scope, or resource | `source`, optionally `metadata` |
| error, exception, stack trace, traceback, or longer detail | `message` |
| trace correlation | `trace-id` |
| structured attributes, tags, or context | `metadata` |
| user context | `actor`, optionally `metadata` |
| producing app | `app` |
| related object | `reference-entity`, `reference-record-id`, `reference-record-name` |

Do not add dedicated v1 fields for `span-id`, `environment`, `release`, `logger`, `request-path`, `status-code`, `fingerprint`, or breadcrumbs. Put those in `metadata` until dygo has a clear Studio workflow that needs first-class filtering or display.

## Types

Initial Log types:

```txt
Debug
Info
Warning
Error
Panic
```

These are the exact select values stored in the database. SDK helpers and constants should map to these title-case values so app code does not need to pass raw strings.

`Debug` is for local or temporary detail. It should be easy to disable when a writer does not need that level of detail.

`Info` is for useful operational breadcrumbs.

`Warning` is for recoverable problems that may need attention.

`Error` is for failed operations.

`Panic` is for recovered crashes or unrecoverable code paths that dygo managed to capture.

The type list can grow later, but v1 should stay small so Studio filters remain simple.

## Sources

Initial Log source values:

```txt
Framework
SDK
HTTP
Job
Hook
CLI
Studio
```

These are the exact select values stored in the database.

## SDK

The SDK should make writing Logs simple. App code should not need to know the Core Entity fields or call generic Record APIs directly for common cases.

Intended ergonomic shape:

```go
dygo.Debug(ctx, "Resolved pricing rule")
dygo.Info(ctx, "Customer import started")
dygo.Warning(ctx, "Stripe rate limit hit")
dygo.Error(ctx, "Customer import failed", err)
dygo.Panic(ctx, "Recovered hook panic", recovered)
```

The public package name should be `dygo`, not `sdk`, once the public SDK import path is renamed.

The helper functions are best-effort and should not return errors. Logging should add visibility without forcing every app call site into log-persistence error handling.

Structured options can add context without making the common path noisy:

```go
dygo.Error(ctx, "Customer import failed", err,
	dygo.WithTraceID(traceID),
	dygo.WithReference("crm", "customer", customerID),
	dygo.WithMetadata("batch", batchName),
)
```

When a helper is called, dygo should fill the obvious fields from the function and from `ctx`:

| Field | Filled how |
| --- | --- |
| `type` | From the helper or `LogEntry.Type`. For example, `dygo.Error` writes `Error`. |
| `source` | From the current runtime context when available: `Hook`, `Job`, `HTTP`, `CLI`, `Studio`, or `Framework`. Use `SDK` as the fallback for direct SDK calls without a richer source. |
| `app` | From hook or Job context when app-owned code is running, or from explicit options. |
| `title` | From the helper title argument or `LogEntry.Title`. |
| `message` | From error detail, stack or traceback text, explicit message/detail options, or `LogEntry.Message`. `dygo.Error(ctx, title, err)` should write `err.Error()` here when `err` is present. |
| `trace-id` | From `ctx` when the current request, Job, or hook carries trace context, or from explicit options. The helper should not create a different trace ID per log call. |
| `reference-entity` | From Record hook context when available, or from explicit options. |
| `reference-record-id` | From Record hook context when available, or from explicit options. |
| `reference-record-name` | From the current Record name when available, or from explicit options. |
| `actor` | From current user/session context when available, or from explicit options. |
| `metadata` | Only caller-provided structured context or `LogEntry.Metadata`. |

Inside hooks and Jobs, dygo should attach context automatically when available:

- source: `Hook` or `Job`
- actor: current user when the operation came from a session
- trace ID: current request or Job trace
- reference: current Entity and Record when running inside a Record hook

The SDK should also expose a structured form for advanced cases:

```go
err := dygo.Log(ctx, dygo.LogEntry{
	Type:    dygo.TypeError,
	Title:   "Payment sync failed",
	Message: err.Error(),
	TraceID: traceID,
	Metadata: map[string]any{
		"provider": "stripe",
		"attempt":  3,
	},
})
```

Unlike the helper functions, `dygo.Log` should return an error so framework code and strict app code can react when Log persistence fails.

The structured `LogEntry.Type` constants should be named after the field, not the Entity:

```go
dygo.TypeDebug
dygo.TypeInfo
dygo.TypeWarning
dygo.TypeError
dygo.TypePanic
```

This keeps the common helpers short (`dygo.Error`) while avoiding a Go package namespace conflict with the type constants.

## Framework Use

dygo should dogfood Logs in framework code.

Good first framework writers:

- recovered panics in HTTP handlers, Jobs, and hooks
- Job handler failures and retry decisions
- metadata validation failures at runtime startup
- Studio API failures that are useful to inspect later
- secret/config loading failures after redacting sensitive values

Logs must not replace returned errors. A function should still return the correct error to its caller. Logging is additional visibility.

## Studio

Studio should treat Logs as a normal system Entity first:

- list Logs newest-first
- filter by `type`, `source`, `trace-id`, and reference fields
- show `message` and `metadata` in readable code-style blocks
- provide copy buttons for message text, trace ID, and metadata

The screenshot reference from Frappe's Error Log is useful for the detail page shape: top-level reference fields, a short title, a large message block for error or diagnostic detail, trace ID, and structured metadata.

Later Studio can add dedicated Log screens, but the first version should work through the generic metadata-driven Record UI.

## Permissions

Do not add Log-specific permissions in v1. Existing admin/system access is enough for the first implementation. Log permissions should be handled later with the broader permissions work.

## Boundaries

Logs are not Activity. They are not a compliance-grade Audit Log. They are not a replacement for local development console output.

Logs are persisted operational diagnostics. They should be queryable, filterable, and visible in Studio.

Activity stays focused on human-meaningful Record history.

Audit Log remains a future compliance/security feature with stricter immutability, access rules, and lifecycle requirements.

Local console logging remains useful while developing, especially before the database is available.

## Implementation Order

Recommended first pass:

1. Rename the public SDK package and import path from `pkg/sdk` to `pkg/dygo`, without keeping a compatibility shim.
2. Add Core `log` Entity metadata.
3. Add SDK helpers for `Debug`, `Info`, `Warning`, `Error`, and structured `Log`.
4. Dogfood Logs with recovered panic writers first.
5. Expose Logs through the generic Record API and Studio list/detail UI.
