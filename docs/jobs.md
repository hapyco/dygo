# Jobs And Queues

This file tracks the durable Jobs, Queues, and worker decisions.

Related tasks:

- #138 Design durable Jobs and Queues architecture
- #139 Add Core Job metadata and Postgres queue storage
- #141 Add SDK and internal Jobs APIs
- #140 Implement Job worker runtime, claiming, retries, and timeouts
- #31 Add worker command
- #142 Add Job operations visibility and retry controls

## Goals

- App code can enqueue durable background work.
- App code can define typed job runners in app-owned Go files.
- Workers can claim work safely from PostgreSQL with multiple worker processes.
- Failed jobs retry with bounded attempts and visible terminal state.
- Job state is visible through Core records and can later power Studio operations.

## Non-Goals For This Batch

- Studio retry/cancel/detail screens; keep them for #216, #217, and #218.
- Importer-specific jobs; keep them for #143.
- External queue backends. PostgreSQL is the first durable backend.
- Dynamic loading of Go source files. Jobs are compiled into the project runner, like hooks.

## Terms

`Job` is the app-defined background work type, such as `crm/send-welcome-email`.

`Job Execution` is one durable queued occurrence of a Job.

`Attempt` is one try inside a Job Execution. Retries create more attempts, not more Job Executions.

`Worker` is a long-running dygo process that claims and runs Job Executions.

`Queue` is a named lane used by workers to select work. The default queue is `default`.

## App File Shape

Jobs live in the existing app shape:

```txt
apps/<app>/jobs/<job>/job.yml
apps/<app>/jobs/<job>/run.go
apps/<app>/jobs/_schedules.yml
```

Recurring app-owned schedules are defined in `_schedules.yml`; `dygo worker` turns due Schedule occurrences into normal Job Executions.

Create the happy-path Job scaffold with:

```sh
dygo generate job crm/send-welcome-email
```

The command creates `job.yml`, creates a starter `run.go`, and updates the generated project runner so the Job is registered automatically.

Queue one execution manually for testing with:

```sh
dygo job execution run crm/send-welcome-email --payload '{}'
dygo worker --once
```

`job execution run` creates a durable Job Execution in the database. It does not run the handler inline; a worker process still claims and executes it. The Job must be synced into Core records first with `dygo db migrate`. `dygo job exec run` is the short alias.

Inspect and control executions from the database with:

```sh
dygo job execution list
dygo job execution show <id-or-name>
dygo job execution cancel <id-or-name>
dygo job execution retry <id-or-name> --idempotency-key <key>
```

`list`, `show`, `cancel`, and `retry` read the selected environment database. `cancel` is queued-only in the MVP. `retry` is failed-only, copies the failed execution payload, and requires a new caller-provided idempotency key.

Inspect and control registered Jobs with:

```sh
dygo job list
dygo job show crm/send-welcome-email
dygo job disable crm/send-welcome-email
dygo job enable crm/send-welcome-email
```

`job list` shows registered Jobs, not executions. `disable` and `enable` only update the human-controlled `enabled` state. They do not cancel queued or running executions. `enable` fails for retired Jobs; restoring the `job.yml` and running `dygo db migrate` is what un-retires a file-backed Job.

Proposed `job.yml` shape:

```yaml
label: Send Welcome Email
description: Sends the first welcome email to a new contact.
queue: default
timeout: 30s
retry:
  attempts: 3
```

Locked decisions:

- Job key comes from the bundle folder name: `apps/<app>/jobs/<job>/job.yml`. There is no `name` field in `job.yml`.
- `source`, `enabled`, and `retired` are Core `job` fields, not `job.yml` keys.
- Job key must be kebab-case.
- `label` is required.
- `description` is optional.
- Use kebab-case metadata keys, matching the rest of dygo metadata.
- Store durations as strings accepted by Go duration parsing, such as `30s` and `5m`.
- Treat missing `queue` as `default`.
- Require `queue` to reference a queue registered in `config/queues.yml`.
- Require `timeout`; it must be a positive duration.
- Treat missing `retry` as no retries, one attempt only.
- If `retry` exists, `retry.attempts` is required and must be at least `2`.
- `retry.attempts` is total attempts, including the first try.
- `retry.initial-delay` is optional; default `10s`.
- `retry.max-delay` is optional; default `5m`.
- Retry delays must be positive, and `retry.max-delay` must be greater than or equal to `retry.initial-delay`.
- Do not include a `strategy` field in the MVP. Exponential is the only retry behavior.
- Payloads are JSON. Payload may be empty, but when provided it must be valid JSON.
- Defer payload schema validation. The first implementation stores JSON payloads and lets handler code validate and decode them.

## Execution Sources

All Job Executions are durable database rows, regardless of where the work definition comes from.

Code-backed jobs:

```txt
apps/crm/jobs/send-email/job.yml   - App-owned Job metadata
apps/crm/jobs/send-email/run.go    - App-owned Run function compiled into the project runner
Core job row                       - Synced runtime metadata
Core job-execution row             - Queued durable work
dygo worker                        - Claims the row and calls the compiled handler
```

System-backed jobs:

```txt
Studio action                      - User configures work in the UI
Core job row                       - First-party system Job, such as studio/send-report
Core job-execution row             - Queued durable work with user-selected payload
dygo worker                        - Claims the row and calls the built-in handler
```

Studio should not create arbitrary Go code. Studio-created work stores configuration in Core records and executes through approved built-in handlers.

Future metadata-backed automation can dogfood the same queue:

```txt
Core automation definition         - DB-stored action graph or workflow steps
Core job row                       - Built-in automation runner Job
Core job-execution row             - Queued durable work
dygo worker                        - Runs the automation handler, which interprets the DB definition
```

This lets users build complex jobs in Studio from approved steps, while workers still execute known compiled handlers.

## Queue Configuration

Queues are registered explicitly in project config:

```txt
config/queues.yml
```

Generated projects include the default queue:

```yaml
queues:
  - name: default
    concurrency: 4
```

Job metadata references a registered queue:

```yaml
queue: default
```

Locked decisions:

- `config/queues.yml` is the queue registry for the project.
- `default` is generated automatically and should be present in every project.
- Generated `default` queue includes `concurrency: 4`.
- Workers read queue concurrency from `config/queues.yml`.
- `--concurrency` overrides queue config for the current worker process when explicitly passed.
- Most Jobs should use the default queue. Custom queues are an optional routing tool for work that needs separate handling.
- A Job whose `queue` is missing uses `default`.
- A Job whose `queue` names an unregistered queue is invalid.
- `dygo worker` with no `--queue` flags processes all registered queues.
- `dygo worker --queue email` processes only registered queue `email`; unknown queue flags fail at startup.
- Queue validation belongs in job metadata validation, `dygo doctor`, and worker queue flag checks.
- Defer other queue-specific settings such as rate limits, retention, and dead-letter policy.
- Keep queue configuration opinionated: fewer choices, strong defaults, and minimal knobs in the MVP.

## Core Metadata

Add Core `Job` metadata so dygo can persist discovered job definitions next to App, Entity, Field, Permission, and Patch metadata.

Each discovered `apps/<app>/jobs/<job>/job.yml` syncs into one Core `job` record.

`dygo db migrate` owns Job metadata sync. It reads and validates `apps/*/jobs/*/job.yml`, validates referenced queues against `config/queues.yml`, and upserts Core `job` records with `source=file`. Enqueue and worker runtime use synced Core records rather than scanning `job.yml` files on each operation.

If a file-backed `job.yml` disappears, migrate marks that Core `job` as retired instead of deleting it. Old Job Executions stay inspectable. If the file comes back, migrate clears the retired flag.

Proposed Core `job` fields:

- `app` link to Core App
- `key` text, app-scoped job key
- `source` select: `file`, `studio`, `system`
- `label` text
- `description` long text
- `queue` text, default `default`
- `timeout` text
- `retry` json
- `enabled` boolean, default true
- `retired` boolean, default false

Add Core `Job Execution` storage for durable queue state.

Proposed Core `job-execution` fields:

- `job` link to Core Job
- `app-name` text snapshot
- `job-name` text snapshot
- `queue` text
- `status` select: `queued`, `running`, `succeeded`, `failed`, `cancelled`
- `priority` integer, default 0
- `payload` json
- `result` json
- `error` long text
- `attempts` integer, default 0
- `max-attempts` integer
- `retry` json
- `run-after` datetime
- `started-at` datetime
- `finished-at` datetime
- `locked-by` text
- `locked-until` datetime
- `idempotency-key` text
- `actor` optional link to Core User

Add Core `Schedule` metadata for recurring work.

Core `schedule` fields:

- `app` link to Core App
- `key` text
- `source` select: `file`, `studio`, `system`
- `label` text
- `description` long text
- `cron` text
- `timezone` text
- `job` link to Core Job
- `job-app-name` text snapshot
- `job-name` text snapshot
- `enabled` boolean, default true
- `retired` boolean, default false
- `next-run-at` datetime
- `last-run-at` datetime
- `last-error` long text
- `actor` optional link to Core User

Schedules create Job Executions; workers do not run schedules directly.

Locked decisions:

- Use `Job Execution` everywhere as the official concept name. Use `run` as a verb, such as "run a job execution".
- Keep attempt history out of the MVP. Store the latest error on `job-execution`; add `job-attempt` later if Studio needs detailed timelines.
- Do not add a separate `job-attempt` entity in the MVP. `job-execution.attempts` and latest `error` are enough for the first operational view.
- Use Core record tables for `job` and `job-execution` so Studio and APIs can inspect them through normal metadata paths.
- Keep `app-name` and `job-name` snapshots on `job-execution` so old queued executions remain understandable after metadata changes.
- `job-execution.max-attempts` snapshots the Job's total allowed attempts at enqueue time. It is `1` when `retry` is missing.
- `job-execution.retry` snapshots retry delay settings at enqueue time so later Job metadata changes do not change already queued executions.
- Every Job must define `timeout`. Workers use it to set each execution lock and handler deadline.
- Workers generate a readable ID on startup for `locked-by`: `<hostname>:<pid>:<short-random>`.
- `locked-until` is set to `claimed-at + timeout`.
- Store Studio-created schedules in Core `schedule` records, not `_schedules.yml`.
- Keep `_schedules.yml` for app-defined schedules shipped with code.
- File-backed Schedules sync with `source=file`; removing a file-backed Schedule retires the Core row instead of deleting it.
- Schedule occurrences use deterministic idempotency keys shaped as `schedule:<app>/<schedule-name>:<due-time-rfc3339>`.
- Enforce `idempotency-key` uniqueness per Job when present. Enqueueing the same Job with the same key returns the existing Job Execution instead of creating duplicate work.
- `idempotency-key` is supplied at enqueue time, not in `job.yml`. It identifies the upstream cause or work item, such as `email:<email-id>`, `import:<import-id>`, `webhook-event:<event-id>`, or `schedule-occurrence:<occurrence-id>`.
- The uniqueness scope is `job` plus `idempotency-key`. dygo enforces uniqueness; app and system code choose keys that represent the correct business cause.
- Each Job Execution stores the Job identity and the caller-provided `idempotency-key`. Whoever enqueues the Job is responsible for making the key unique for that Job when duplicate prevention matters.
- Good idempotency keys can include timestamps, stored UUIDs, Record IDs, provider event IDs, or schedule occurrence IDs. The key must be stable for the same intended work and different for new intended work.
- Disabled or retired Jobs cannot create new Job Executions. `enabled` is the human pause switch; `retired` is dygo's lifecycle state for file-backed Jobs whose `job.yml` was removed. Workers still run already-created Job Executions unless those executions are cancelled separately.
- `dygo job disable` sets `enabled=false`; `dygo job enable` sets `enabled=true` only when `retired=false`.
- Enqueue requires a synced Core `job` record. Unknown app/job targets fail with an error such as `job crm/send-email is not registered`.
- Enqueue validation errors include unknown Job, disabled Job, retired Job, and invalid payload JSON. Unregistered queues in Job metadata are caught by Job metadata validation, `dygo doctor`, and `dygo db migrate`; unknown `dygo worker --queue` flags fail at worker startup. Idempotency duplicates return the existing Job Execution instead of failing.
- MVP handlers return `error` only. Job Execution keeps nullable `result` JSON as reserved storage for future system/API use, but app SDK code does not write structured results in the first batch.
- Jobs that produce durable output should create normal Records or files and rely on those as the real output.
- Priority belongs to Job Executions, not `job.yml`, in the MVP. It defaults to `0`; callers may enqueue with a nonzero priority, and workers claim higher priority executions first.
- MVP Job Execution statuses are `queued`, `running`, `succeeded`, `failed`, and `cancelled`.
- Do not add a separate `retrying` status. Retried executions return to `queued` with a future `run-after` and `attempts > 0`.
- Cancellation is queued-only in the MVP. A `queued` execution can become `cancelled`; a `running` execution is allowed to finish or timeout.
- Running cancellation can be added later with cooperative handler cancellation and stronger runtime controls.
- Store execution lifecycle timestamps on `job-execution`: `run-after`, `started-at`, and `finished-at`.

## PostgreSQL Queue Semantics

Workers claim work from PostgreSQL using row locks:

```sql
SELECT id
FROM job_execution
WHERE status = 'queued'
  AND run_after <= now()
  AND queue = ANY($1)
ORDER BY priority DESC, run_after ASC, id ASC
FOR UPDATE SKIP LOCKED
LIMIT $2
```

The claim transaction updates matching rows to `running`, sets `locked-by`, sets `locked-until`, increments `attempts`, and returns the claimed executions.

Locked decisions:

- Use `FOR UPDATE SKIP LOCKED` for multi-worker safety.
- A job execution is eligible when `status = queued` and `run-after <= now()`.
- `locked-until` is the crash recovery lease. If a worker dies, another worker can recover the execution after the lease expires.
- The worker gives each execution a context deadline based on the job timeout.
- On success, set `status = succeeded`, `finished-at`, `result`, and clear lock fields.
- On failure before `max-attempts`, set `status = queued`, compute a future `run-after`, store `error`, and clear lock fields.
- On final failure, set `status = failed`, `finished-at`, store `error`, and clear lock fields.
- On timeout, treat the execution's current attempt as failed with error kind `timeout`.
- If no compiled handler is registered for the Job, mark the execution `failed` immediately with a missing-handler error. Do not retry missing handlers.
- Each worker runs a recovery step before claiming new work. The recovery step finds `running` executions with `locked-until` in the past, then retries or fails them based on remaining attempts.
- Jobs that should not run after becoming stale should validate freshness in their handler or use an idempotency key tied to the business event. Future metadata can add an explicit stale-after setting if this becomes common.
- Defer a first-class `stale-after` field. It is too much policy for the MVP; handlers own business freshness checks.

Worker wake-up decisions:

- Send a PostgreSQL notification when a Job Execution is newly queued or requeued for retry.
- Notification payload is the queue name, such as `default`.
- Notifications only wake workers; the database eligibility rule above remains the source of truth.
- Keep a 60s fallback poll so workers keep running if notifications are unavailable or missed.
- Keep a next `run_after` timer because PostgreSQL does not notify again when a future timestamp becomes due.

## Retry Policy

Default behavior:

- missing `retry`: one attempt only, no retries
- `retry.attempts`: total attempts, including the first try
- `retry.initial-delay`: first retry delay, default `10s`
- `retry.max-delay`: exponential retry delay cap, default `5m`

The MVP retry strategy is exponential only and is not configurable in `job.yml`.

The retry delay is based on the failed attempt number:

```txt
delay = retry.initial-delay * 2^(failed attempt - 1)
delay = min(delay, retry.max-delay)
```

Jitter can be added later; keep the first version deterministic unless we see contention.

Locked decisions:

- `attempts` increments when a worker claims an execution.
- An execution with `attempts == max-attempts` after failure becomes terminal `failed`.
- Retry state is represented as `queued` with a future `run-after`, not a separate `retrying` status.

## SDK Shape

The public SDK should expose enqueueing and job registration without exposing PostgreSQL details.

Proposed app-facing types:

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

Proposed enqueue API:

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

Locked decisions:

- Keep the first SDK payload as `json.RawMessage`; typed helpers can come later.
- Give jobs access to `Records` and `Jobs` so a job can read/write Records and enqueue follow-up work.
- Generated project runners wire `apps/<app>/jobs/<job>/run.go` automatically when it exposes `Run(ctx context.Context, job sdk.JobExecution) error`.
- Custom project runners can still register compiled jobs through `pkg/sdk/runtime.Options`, parallel to Record hook registrars.
- App identity for SDK calls remains `<app>, <job>`, not route or label.
- Enqueue options stay small in the MVP: `idempotency-key`, `priority`, and `run-after`.
- `run-after` schedules one Job Execution for a future time. Recurring work belongs to Schedule metadata, not enqueue options.
- Do not allow enqueue-time queue, timeout, or retry overrides in the MVP; those come from `job.yml`.

## Internal Packages

Proposed package split:

```txt
internal/jobs                - job.yml reader, validator, and shared Job metadata types
internal/jobs/store          - PostgreSQL enqueue, claim, complete, fail
internal/jobs/runtime        - registry and worker loop
internal/jobgen              - Job scaffold and runner wiring generator
internal/runnergen           - shared generated project runner renderer
internal/cli                 - dygo worker command
pkg/sdk                      - public job types and registration API
pkg/sdk/runtime              - project runner options for compiled jobs
```

Locked decisions:

- Keep blocking operations context-aware.
- Keep the store interface small enough for worker tests without a live database.
- Put PostgreSQL-specific SQL behind the store package.

## Worker Command

Add:

```txt
dygo worker
```

Proposed flags:

- `--env development`
- `--queue <name>`, repeatable
- `--concurrency <n>`
- `--poll-interval 60s`
- `--poll-only`
- `--once`
- `--shutdown-timeout 30s`

Behavior:

- Use the same project root and database secret resolution as `dygo serve`, `dygo dev`, and `dygo db`.
- Require a migrated database with Core job tables present. `dygo worker` does not auto-migrate; if the schema is missing, fail with guidance to run `dygo db migrate`.
- Start worker pools from effective queue concurrency.
- If `--concurrency` is omitted, use each queue's `concurrency` from `config/queues.yml`.
- If `--concurrency` is explicitly passed, use that value for each selected queue in this worker process.
- With no `--queue` flags, process all registered queues until the process context is cancelled.
- With one or more `--queue` flags, process only those registered queues.
- Listen for PostgreSQL Job Execution notifications by default, then claim ready rows from the database.
- Use `--poll-interval` as the fallback polling interval when notifications are missed or unavailable.
- Use `--poll-only` to disable notifications and rely on polling only.
- Keep one next `run_after` timer and one next Schedule `next-run-at` timer per queue so delayed retries, future executions, and recurring Schedules wake when due instead of waiting for the fallback poll.
- Finish in-flight jobs during graceful shutdown until `--shutdown-timeout`.
- With `--once`, create executions for due Schedules, claim and run currently available Job Executions, then exit. This is useful for tests, local debugging, and one-shot maintenance.
- `--once` does one batch only: connect to the database, check due Schedules, recover expired running executions, claim up to the effective concurrency for the selected queues, run them, persist success/failure/retry state, and exit cleanly. If no executions are available, exit cleanly.

Output decision:

- Normal command lifecycle output goes to stdout.
- Worker diagnostics and per-execution logs go to stderr.
- Keep output plain ASCII and parseable.

## Production Runtime

Jobs are not processed by the web server process. Production deployments that use Jobs should run both:

```txt
web: dygo serve
worker: dygo worker
```

`dygo serve` handles HTTP and Studio traffic. `dygo worker` handles queued Job Executions. The deployment tool is responsible for running, supervising, restarting, scaling, and collecting logs for those processes.

Running only `dygo serve` leaves Job Executions queued in PostgreSQL until a worker process starts.

## Implementation Order

1. Add `config/queues.yml` scaffold, reader, and validator with generated `default` queue.
2. Add metadata reader/validator for `job.yml`, including registered queue checks.
3. Add Core `job` and `job-execution` entities and schema snapshot updates.
4. Add PostgreSQL queue store with enqueue, claim, complete, and fail operations.
5. Add SDK job registration and enqueue API.
6. Add worker runtime loop with concurrency, timeouts, retries, and shutdown.
7. Add `dygo worker` command.
8. Update docs and todo status after the implementation is verified.

## Remaining Details

- No open product decisions for the first implementation batch.
