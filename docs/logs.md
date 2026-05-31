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

The Core app owns a system Entity named `log`.

The Entity should be append-only by default. App code and framework code create Logs; normal app workflows should not edit them.

Proposed fields:

| Field | Type | Required | Purpose |
| --- | --- | --- | --- |
| `type` | `select` | yes | Log type, matching the common logger idea of severity or level. Stored values should be `Debug`, `Info`, `Warning`, `Error`, and `Panic`. |
| `source` | `select` | yes | Runtime surface that wrote the Log, such as `framework`, `sdk`, `http`, `job`, `hook`, `cli`, or `studio`. |
| `title` | `text` | yes | Short human-facing summary. Simple SDK calls should map their message here. |
| `message` | `long-text` | no | Optional longer body or explanation when the title is not enough. |
| `error` | `long-text` | no | Error string plus stack trace or traceback text when available. |
| `trace-id` | `text` | no | Correlation ID shared across request, Job, hook, and nested work. |
| `reference-entity` | `link` | no | Optional Core Entity reference for the related Record type. |
| `reference-record-id` | `bigint` | no | Optional internal Record ID for the related Record. |
| `reference-record-name` | `text` | no | Optional stable Record name for the related Record. |
| `actor` | `link` | no | Optional User that caused the event. |
| `metadata` | `json` | no | Structured attributes, tags, and context that do not deserve first-class fields yet. |

All Log `link` fields should use `foreign-key: false` so old Logs survive target deletion, the same pattern used by Activity history.

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
| error, exception, stack trace, or traceback | `error` |
| trace correlation | `trace-id` |
| structured attributes, tags, or context | `metadata` |
| user context | `actor`, optionally `metadata` |
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

Structured options can add context without making the common path noisy:

```go
dygo.Error(ctx, "Customer import failed", err,
	dygo.WithTraceID(traceID),
	dygo.WithReference("crm", "customer", customerID),
	dygo.WithMetadata("batch", batchName),
)
```

Inside hooks and Jobs, dygo should attach context automatically when available:

- source: `hook` or `job`
- actor: current user when the operation came from a session
- trace ID: current request or Job trace
- reference: current Entity and Record when running inside a Record hook

The SDK should also expose a structured form for advanced cases:

```go
dygo.Log(ctx, dygo.LogEntry{
	Type:    dygo.TypeError,
	Title:   "Payment sync failed",
	Error:   err,
	TraceID: traceID,
	Metadata: map[string]any{
		"provider": "stripe",
		"attempt":  3,
	},
})
```

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
- show `error` and `metadata` in readable code-style blocks
- provide copy buttons for error text, trace ID, and metadata

The screenshot reference from Frappe's Error Log is useful for the detail page shape: top-level reference fields, a short title, a large error/traceback block, trace ID, and structured metadata.

Later Studio can add dedicated Log screens, but the first version should work through the generic metadata-driven Record UI.

## Boundaries

Logs are not Activity. They are not a compliance-grade Audit Log. They are not a replacement for local development console output.

Logs are persisted operational diagnostics. They should be queryable, filterable, and visible in Studio.

Activity stays focused on human-meaningful Record history.

Audit Log remains a future compliance/security feature with stricter immutability, access rules, and lifecycle requirements.

Local console logging remains useful while developing, especially before the database is available.

## Implementation Order

Recommended first pass:

1. Add Core `log` Entity metadata.
2. Add the Log values to Core fixtures and permissions so system managers can read Logs.
3. Add SDK helpers for `Debug`, `Info`, `Warning`, `Error`, and structured `Log`.
4. Add framework writers for recovered panics and Job failures.
5. Expose Logs through the generic Record API and Studio list/detail UI.
