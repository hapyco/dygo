# Jobs

dygo Jobs run durable background work through PostgreSQL-backed Job Executions. Jobs are defined by app metadata and compiled Go handlers; workers claim queued executions from the database and run them safely across one or more worker processes.

## Terms

`Job` is the app-defined background work type, such as `crm/send-welcome-email`.

`Job Execution` is one durable queued occurrence of a Job.

`Attempt` is one try inside a Job Execution. Retries create more attempts on the same Job Execution, not a new Job Execution.

`Worker` is a long-running dygo process that claims and runs Job Executions.

`Queue` is a named lane used by workers to select work. Most projects use the generated `default` queue. See [Queues And Workers](queues.md) for queue config and worker runtime details.

## Define A Job

Jobs live inside app bundles:

```txt
apps/<app>/jobs/<job>/job.yml
apps/<app>/jobs/<job>/run.go
```

Create the standard scaffold with:

```sh
dygo generate job crm/send-welcome-email
```

The generator creates `job.yml`, creates a starter `run.go`, and updates the generated project runner so the Job is registered automatically. `run.go` is developer-owned after creation; dygo will not overwrite custom Job logic.

Generated Job handlers expose one `Run` function:

```go
package job

import (
	"context"

	"github.com/hapyco/dygo/pkg/dygo"
)

func Run(ctx context.Context, job dygo.JobExecution) error {
	return nil
}
```

Generated runner wiring registers the handler by app and Job key:

```go
registry.RegisterJob("crm", "send-welcome-email", crmsendwelcomeemailjob.Run)
```

Custom project runners can register Jobs through `pkg/dygo/runtime.Options`, parallel to Record hook registrars.

## job.yml Reference

The Job key comes from the bundle folder name. There is no `name` field in `job.yml`.

```yaml
label: Send Welcome Email
description: Sends the first welcome email to a new contact.
queue: default
timeout: 30s
retry:
  attempts: 3
```

| Field | Required | Description |
| --- | --- | --- |
| `label` | yes | Human-facing Job label. |
| `description` | no | Human-facing explanation of the Job. |
| `queue` | no | Registered queue name. Missing `queue` uses `default`. |
| `timeout` | yes | Positive Go duration string used for the handler deadline and worker lease. |
| `retry` | no | Retry settings. Missing `retry` means one attempt only. |
| `retry.attempts` | yes when `retry` exists | Total attempts, including the first try. Must be at least `2`. |
| `retry.initial-delay` | no | First retry delay. Defaults to `10s`. |
| `retry.max-delay` | no | Exponential retry cap. Defaults to `5m`. |

Rules:

- Job keys and queue names use kebab-case.
- Durations use Go duration syntax, such as `30s`, `5m`, or `1h30m`.
- `retry.max-delay` must be greater than or equal to `retry.initial-delay`.
- Retry strategy is exponential and is not configurable in `job.yml`.
- Payloads are JSON. dygo stores them as JSON and handler code validates the business shape.
- `source`, `enabled`, and `retired` are Core `job` fields, not `job.yml` keys.

## Queues

Job metadata references a registered queue. Missing `queue` uses `default`.

```yaml
queue: default
```

Queue names and worker concurrency are configured in `config/queues.yml`. Most Jobs should use the generated `default` queue; custom queues are an optional routing tool for work that needs separate capacity. See [Queues And Workers](queues.md).

## Sync And Lifecycle

`dygo db migrate` syncs file-backed Jobs into Core `job` records. It reads `apps/*/jobs/*/job.yml`, validates referenced queues against `config/queues.yml`, and upserts one Core `job` row with `source=file`.

Enqueueing and workers use synced Core rows. They do not scan `job.yml` files on each operation.

If a file-backed `job.yml` disappears, migrate marks the Core `job` as retired instead of deleting it. Old Job Executions stay inspectable. If the file returns and `dygo db migrate` runs again, the Job is un-retired.

Job lifecycle fields:

- `source`: `file`, `studio`, or `system`
- `enabled`: human pause switch; disabled Jobs cannot create new Job Executions
- `retired`: lifecycle state for file-backed Jobs whose `job.yml` was removed

`file` is the current app-owned metadata source. `studio` and `system` are reserved for Studio-configured and first-party system Jobs.

`dygo job disable <app>/<job>` sets `enabled=false`. `dygo job enable <app>/<job>` sets `enabled=true` only when `retired=false`.

## Run And Inspect Jobs

Queue one execution manually:

```sh
dygo job execution run crm/send-welcome-email --payload '{}'
```

Short alias:

```sh
dygo job exec run crm/send-welcome-email --payload '{}'
```

This creates a durable Job Execution in the database. It does not run the handler inline. Start a worker to process queued work:

```sh
dygo worker --once
```

Registered Job commands:

```sh
dygo job list
dygo job show crm/send-welcome-email
dygo job disable crm/send-welcome-email
dygo job enable crm/send-welcome-email
```

Job Execution commands:

```sh
dygo job execution list
dygo job execution show <id-or-name>
dygo job execution cancel <id-or-name>
dygo job execution retry <id-or-name> --idempotency-key <key>
```

`list`, `show`, `cancel`, and `retry` read the selected environment database. `cancel` only works for queued executions. `retry` only works for failed executions, copies the failed execution payload, and requires a new caller-provided idempotency key.

All Job commands default to `--env development`.

## Job Execution Data

Core `job-execution` rows store durable queue state:

- `job`: link to Core Job
- `app-name` and `job-name`: snapshots of the enqueued Job identity
- `queue`
- `status`: `queued`, `running`, `succeeded`, `failed`, or `cancelled`
- `priority`: higher numbers are claimed first; default `0`
- `payload`: JSON input
- `result`: nullable JSON reserved for future system/API use
- `error`: latest failure text
- `attempts`: attempts already claimed
- `max-attempts`: total allowed attempts, snapshotted from Job metadata
- `retry`: retry delay settings, snapshotted from Job metadata
- `run-after`
- `started-at`
- `finished-at`
- `locked-by`
- `locked-until`
- `idempotency-key`
- `actor`: optional link to Core User

Handlers return `error` only. Jobs that produce durable output should write normal Records or files and use those as the real output.

## Idempotency

`idempotency-key` is supplied when creating a Job Execution. It identifies the upstream cause or work item, such as:

```txt
email:<email-id>
import:<import-id>
webhook-event:<event-id>
schedule:<app>/<schedule-name>:<due-time-rfc3339>
```

dygo enforces uniqueness by Job plus idempotency key. Enqueueing the same Job with the same key returns the existing Job Execution instead of creating duplicate work.

Use a stable key for the same intended work and a different key for new intended work. Good keys can include timestamps, stored UUIDs, Record IDs, provider event IDs, or Schedule occurrence times.

## Worker Runtime

Workers claim queued Job Executions, enforce timeouts, recover expired locks, retry failures, and wake through PostgreSQL notifications plus timers. See [Queues And Workers](queues.md) for the worker runtime reference.

Stale business work is the handler's responsibility. A first-class `stale-after` field is coming soon only if this becomes common.

## Retry Policy

Missing `retry` means one attempt only.

When retry is configured:

```txt
delay = retry.initial-delay * 2^(failed attempt - 1)
delay = min(delay, retry.max-delay)
```

`attempts` increments when a worker claims an execution. An execution with `attempts == max-attempts` after failure becomes terminal `failed`. Retried executions return to `queued` with a future `run-after`; there is no separate `retrying` status.

Jitter is coming soon if retry contention becomes a problem.

## SDK

The public SDK exposes Job registration and enqueueing without exposing PostgreSQL details.

```go
type JobExecution struct {
	ID       int64
	AppName  string
	JobName  string
	Queue    string
	Attempt  int
	Payload  json.RawMessage
	Records  RecordData
	Jobs     JobData
}

type JobFunc func(context.Context, JobExecution) error

type JobRegistry interface {
	RegisterJob(appName string, jobName string, fn JobFunc) error
}

type JobRegistrar func(JobRegistry) error
```

Enqueue API:

```go
type EnqueueOptions struct {
	IdempotencyKey string
	Priority       int
	RunAfter       time.Time
}

type JobData interface {
	Enqueue(ctx context.Context, appName string, jobName string, payload json.RawMessage, options EnqueueOptions) (JobExecution, error)
}
```

App identity for SDK calls is `<app>, <job>`, not route, label, or display name. Enqueue options are intentionally small: idempotency key, priority, and `run-after`. Queue, timeout, and retry settings come from `job.yml`.

## Implementation Map

```txt
internal/jobs                - job.yml reader, validator, and shared Job metadata types
internal/jobgen              - Job scaffold and runner wiring generator
internal/runnergen           - shared generated project runner renderer
internal/cli                 - Job commands
pkg/dygo                      - public Job types and registration API
pkg/dygo/runtime              - project runner options for compiled Jobs
```

## Coming Soon

- Studio-native Job detail, retry, and cancel screens.
- Job-backed importer foundation.
- Retention policy for old succeeded and failed executions.
- Optional stale-work metadata if handler-level freshness checks become repetitive.
- Queue-level rate limits or dead-letter policy.
