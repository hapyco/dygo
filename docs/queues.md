# Queues And Workers

Queues control how dygo workers select and run Job Executions. The first queue backend is PostgreSQL; dygo does not require Redis or an external queue service.

## Queue Configuration

Queues are registered in:

```txt
config/queues.yml
```

Generated projects include:

```yaml
queues:
  - name: default
    concurrency: 4
```

Most projects only need the `default` queue. Add custom queues when a class of work needs separate capacity or isolation, such as slow imports or urgent emails.

Rules:

- Queue names must be kebab-case.
- Every queue must set `concurrency` greater than `0`.
- A Job whose `queue` names an unregistered queue is invalid.
- A Job with no `queue` uses `default`.
- `dygo worker` with no `--queue` flags processes all registered queues.
- `dygo worker --queue email` processes only the registered `email` queue.
- Unknown queue flags fail worker startup.
- `dygo worker --concurrency <n>` overrides configured concurrency for the current worker process.

Rate limits, retention, and dead-letter policy are coming soon.

## Worker Command

Run workers for all registered queues:

```sh
dygo worker
```

Common flags:

```txt
--env development
--queue <name>
--concurrency <n>
--poll-interval 60s
--poll-only
--once
--shutdown-timeout 30s
```

`--once` checks due Schedules, recovers expired running executions, claims one available batch, runs it, persists success/failure/retry state, and exits cleanly. It is useful for tests, local debugging, and one-shot maintenance.

Normal command lifecycle output goes to stdout. Worker diagnostics and per-execution logs go to stderr.

## Claiming Work

Workers claim ready Job Executions from PostgreSQL with row locks:

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

Runtime rules:

- `locked-by` is a readable worker ID: `<hostname>:<pid>:<short-random>`.
- `locked-until` is set to `claimed-at + timeout`.
- The handler receives a context deadline based on the Job timeout.
- Success sets `status=succeeded`, `finished-at`, optional `result`, and clears lock fields.
- Failure before the final attempt sets `status=queued`, computes a future `run-after`, stores `error`, clears lock fields, and sends a queue notification.
- Final failure sets `status=failed`, `finished-at`, stores `error`, and clears lock fields.
- Timeout is recorded as a failure for the current attempt.
- Missing compiled handlers fail the execution immediately and are not retried.
- Each worker recovers expired `running` executions before claiming new work.

## Wake-Ups

Workers wake from the earliest of:

- PostgreSQL notification when a Job Execution is newly queued or requeued
- next queued Job Execution `run-after`
- next Schedule `next-run-at`
- fallback poll interval, default `60s`

Notifications wake workers quickly; the database eligibility rules remain the source of truth. Use `--poll-only` to disable notifications and rely on polling plus timers.

PostgreSQL does not notify again when a future timestamp becomes due, so workers keep timers for `run-after` and `next-run-at`.

## Production Runtime

Jobs and Schedules are not processed by the web server process. Production deployments that use Jobs or Schedules should run both:

```txt
web: dygo serve
worker: dygo worker
```

`dygo serve` handles HTTP and Studio traffic. `dygo worker` handles queued Job Executions and due Schedules. The deployment tool is responsible for process supervision, restarts, scaling, and log collection.

Running only `dygo serve` leaves Job Executions queued in PostgreSQL and due Schedules waiting in the database until a worker starts.

## Implementation Map

```txt
internal/queues              - config/queues.yml reader and validator
internal/jobs/store          - PostgreSQL enqueue, claim, complete, fail, retry, cancel, and Schedule enqueue behavior
internal/jobs/runtime        - registry and worker loop
internal/cli                 - worker command
```
