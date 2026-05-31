# Schedules

Schedules are recurring trigger rules that create Job Executions. They do not run work directly; `dygo worker` checks due Schedules, enqueues normal Job Executions, and then processes queued work through the Jobs runtime.

## Terms

`Schedule` is a recurring trigger rule.

`Job` is the app-defined background work type to run.

`Job Execution` is one durable queued occurrence created by a Schedule.

`Worker` is the dygo process that checks due Schedules, creates Job Executions, claims queued Job Executions, and runs compiled Job handlers.

## App File Shape

App-owned schedules live in:

```txt
apps/<app>/jobs/_schedules.yml
```

Generated projects and generated apps start with an empty file:

```yaml
schedules: []
```

Example:

```yaml
schedules:
  - name: weekly-report
    label: Weekly Report
    description: Sends the weekly sales report.
    cron: "0 9 * * MON"
    timezone: Asia/Karachi
    job: sales/send-weekly-report
    enabled: true
```

## Field Reference

| Field | Required | Description |
| --- | --- | --- |
| `name` | yes | App-local Schedule key. Must be kebab-case. |
| `label` | yes | Human-facing label. |
| `description` | no | Human-facing explanation. |
| `cron` | yes | Standard 5-field cron expression. |
| `timezone` | yes | IANA timezone, such as `UTC` or `Asia/Karachi`. |
| `job` | yes | Target Job as `<app>/<job>`. |
| `enabled` | no | Whether the Schedule creates future executions. Defaults to `true`. |

Schedules do not accept custom payloads. If scheduled work needs different behavior, create explicit Jobs with clear names, such as `sales/send-weekly-report` and `sales/send-monthly-report`.

Parameterized Schedule payloads are coming soon only if a real product need appears.

## Cron Syntax

dygo uses `github.com/robfig/cron/v3` for parsing and schedule calculation. dygo does not use the package's in-process job runner.

Supported syntax:

```txt
minute hour day-of-month month day-of-week
```

Example:

```txt
0 9 * * MON
```

Rules:

- Use standard 5-field cron.
- Named weekdays and months, such as `MON` and `JAN`, are accepted by the cron parser.
- Do not include `CRON_TZ=` or `TZ=` in `cron`; use the separate `timezone` field.
- Seconds fields are not accepted.
- `@every` interval descriptors are not accepted.
- dygo does not add custom cron aliases or custom name translation.

## Metadata Sync

`dygo db migrate` syncs file-backed Schedules into Core `schedule` records with `source=file`.

Validation rules:

- Missing target Jobs fail Schedule validation and sync.
- Retired target Jobs fail Schedule validation and sync.
- Disabled target Jobs do not block Schedule sync.

Disabling a Job is an operational pause. The Schedule can remain valid, but due occurrences will not enqueue while the target Job is disabled.

If a file-backed Schedule is removed from `_schedules.yml`, migrate marks the Core `schedule` row as retired instead of deleting it. If the Schedule returns and `dygo db migrate` runs again, the row is un-retired.

Studio-created Schedules are coming soon. They will be stored directly in Core `schedule` records with `source=studio`. System-owned Schedules can use `source=system`.

## Runtime Behavior

A Schedule never creates infinite future executions upfront.

When a Schedule is due, the worker:

1. locks due Schedule rows with `FOR UPDATE SKIP LOCKED`
2. creates one Job Execution for each due occurrence it can process
3. uses a deterministic idempotency key
4. updates `last-run-at`
5. calculates and stores the next `next-run-at`

Schedule occurrence idempotency keys use the due occurrence time:

```txt
schedule:<app>/<schedule-name>:<due-time-rfc3339>
```

If the worker was down and missed multiple occurrences, dygo creates one late execution, then advances to the next future cron time. It does not backfill every missed occurrence.

If the target Job is disabled when a Schedule becomes due, the worker records `last-error`, does not create a Job Execution, and advances to the next future cron time.

## Worker Wake-Ups

Workers wake from the earliest of:

- Job Execution notification
- next Job Execution `run-after`
- next Schedule `next-run-at`
- fallback poll interval, default `60s`

PostgreSQL does not send a notification when a future timestamp becomes due, so the worker keeps a timer for the next `next-run-at`. Notifications remain a fast path; the database state remains the source of truth.

There is no separate `dygo scheduler` process or OS cron integration. Run `dygo worker` anywhere Schedules should create Job Executions.

## Core Schedule Fields

Core `schedule` records store:

- `app`: link to Core App
- `key`
- `source`: `file`, `studio`, or `system`
- `label`
- `description`
- `cron`
- `timezone`
- `job`: link to Core Job
- `job-app-name` and `job-name`: snapshots of the target Job identity
- `enabled`
- `retired`
- `next-run-at`
- `last-run-at`
- `last-error`
- `actor`: optional link to Core User

Constraints:

- unique `(app, key)`

Indexes:

- `(enabled, retired, next-run-at)`
- `job`
- `source`

## Manual Check

```sh
dygo db migrate
dygo worker --once
dygo job execution list
```

`dygo worker --once` checks due Schedules, claims one available Job Execution batch, persists the result, and exits.

## Coming Soon

- Studio UI for creating and editing Schedules.
- Natural-language schedule helpers.
- Interval, one-time, business-calendar, and holiday rules.
- Schedule-specific CLI commands if operators need them.
