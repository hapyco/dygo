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
| `type` | `select` | yes | Log type such as `debug`, `info`, `warning`, `error`, or `panic`. |
| `source` | `select` | yes | Where the Log came from, such as `framework`, `sdk`, `http`, `job`, `hook`, `cli`, `worker`, or `studio`. |
| `title` | `text` | yes | Short human-facing summary. |
| `message` | `long-text` | no | Longer explanation or plain text detail. |
| `error` | `long-text` | no | Error string, stack trace, or traceback text. |
| `trace-id` | `text` | no | Correlation ID shared across request, Job, hook, and nested work. |
| `reference-entity` | `link` | no | Optional Core Entity reference for the related Record type. |
| `reference-record-id` | `bigint` | no | Optional internal Record ID for the related Record. |
| `reference-record-name` | `text` | no | Optional stable Record name for the related Record. |
| `actor` | `link` | no | Optional User that caused the event. |
| `metadata` | `json` | no | Structured context that does not deserve first-class fields yet. |

`reference-entity` and `actor` should use `foreign-key: false` so old Logs survive target deletion, the same pattern used by Activity history.

Expected indexes:

| Index | Fields | Purpose |
| --- | --- | --- |
| `by-type-created` | `type`, `created-at` | Filter recent errors, warnings, or info Logs. |
| `by-source-created` | `source`, `created-at` | Inspect one runtime surface. |
| `by-trace` | `trace-id` | Follow a request or Job across multiple Logs. |
| `by-reference` | `reference-entity`, `reference-record-id` | Show Logs related to one Record. |

## Types

Initial Log types:

```txt
debug
info
warning
error
panic
```

`debug` is for local or temporary detail. It should be easy to disable or retain for a shorter time.

`info` is for useful operational breadcrumbs.

`warning` is for recoverable problems that may need attention.

`error` is for failed operations.

`panic` is for recovered crashes or unrecoverable code paths that dygo managed to capture.

The type list can grow later, but v1 should stay small so Studio filters and retention policies remain simple.

## SDK

The SDK should make writing Logs simple. App code should not need to know the Core Entity fields or call generic Record APIs directly for common cases.

Intended ergonomic shape:

```go
sdk.Info(ctx, "Customer import started")
sdk.Error(ctx, "Customer import failed", err)
```

Structured options can add context without making the common path noisy:

```go
sdk.Error(ctx, "Customer import failed", err,
	sdk.WithTraceID(traceID),
	sdk.WithReference("crm", "customer", customerID),
	sdk.WithMetadata("batch", batchName),
)
```

Inside hooks and Jobs, dygo should attach context automatically when available:

- source: `hook` or `job`
- actor: current user when the operation came from a session
- trace ID: current request or Job trace
- reference: current Entity and Record when running inside a Record hook

The SDK should also expose a structured form for advanced cases:

```go
sdk.Log(ctx, sdk.LogEntry{
	Type:    sdk.LogError,
	Title:   "Payment sync failed",
	Error:   err,
	TraceID: traceID,
	Metadata: map[string]any{
		"provider": "stripe",
		"attempt":  3,
	},
})
```

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

## Retention

Logs can grow quickly. The first implementation should plan for retention even if cleanup lands later.

Suggested default retention:

| Type | Default retention |
| --- | --- |
| `debug` | 7 days |
| `info` | 30 days |
| `warning` | 90 days |
| `error` | 180 days |
| `panic` | 365 days |

Retention should eventually connect to the Core retention policy Entity rather than hard-coded cleanup rules.

## Privacy

Logs may contain sensitive data. The SDK and framework writers must avoid storing secrets, password values, session tokens, API keys, raw cookies, or full database URLs.

Rules:

- redact known secret values before writing a Log
- prefer stable IDs, names, and trace IDs over full payload dumps
- keep request bodies out of Logs by default
- treat `metadata` as operator-visible application data, not a private vault

If an error string contains a known secret or database URL, dygo should sanitize it before persistence.

## Boundaries

Logs are not Activity. They are not a compliance-grade Audit Log. They are not a replacement for local development console output.

Logs are persisted operational diagnostics. They should be queryable, filterable, and visible in Studio.

Activity stays focused on human-meaningful Record history.

Audit Log remains a future compliance/security feature with stricter immutability, access rules, and retention requirements.

Local console logging remains useful while developing, especially before the database is available.

## Implementation Order

Recommended first pass:

1. Add Core `log` Entity metadata.
2. Add the Log values to Core fixtures and permissions so system managers can read Logs.
3. Add SDK helpers for `Debug`, `Info`, `Warning`, `Error`, and structured `Log`.
4. Add framework writers for recovered panics and Job failures.
5. Expose Logs through the generic Record API and Studio list/detail UI.
6. Add retention cleanup after Core retention policy support exists.

