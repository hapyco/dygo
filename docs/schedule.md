# Schedules

This file tracks the recurring Schedule design and runtime behavior.

Related tasks:

- #214 Let Studio users schedule Jobs like "every Monday at 9 AM"
- #215 Let apps ship recurring schedules with code
- #32 Add scheduler behavior to workers

## Goals

- Apps can define recurring Job triggers in code-owned metadata.
- Schedules are synced into Core records so Studio and APIs can inspect them.
- `dygo worker` creates one Job Execution when a Schedule occurrence is due.
- Multiple workers can check due Schedules safely without duplicate executions.

## Non-Goals For MVP

- No separate `dygo scheduler` process.
- No `dygo scheduler` command in the MVP.
- No OS cron integration.
- No natural-language schedule syntax.
- No interval, one-time, business-calendar, or holiday rules.
- No Studio schedule UI in the first app-owned schedule batch.
- No schedule-specific CLI commands unless we decide they are needed later.

## Terms

`Schedule` is a recurring trigger rule.

`Job` is the app-defined background work type to run.

`Job Execution` is one durable queued occurrence created by a Schedule.

`Worker` is the long-running dygo process that checks due Schedules, enqueues Job Executions, and runs queued Job Executions.

## App File Shape

Schedules live in:

```txt
apps/<app>/jobs/_schedules.yml
```

Shape:

```yaml
schedules:
  - name: weekly-report
    label: Weekly Report
    cron: "0 9 * * MON"
    timezone: Asia/Karachi
    job: sales/send-report
    enabled: true
```

Generated empty file:

```yaml
schedules: []
```

Locked decisions:

- Keep the file name `_schedules.yml`.
- Use cron syntax for the MVP.
- Use the product word `Schedule`; cron is the first Schedule rule type.
- `name` is required and must be kebab-case.
- `label` is required.
- `cron` is required.
- `timezone` is required.
- `job` is required and must be the full `<app>/<job>` name.
- Do not add `payload` in the MVP. Create explicit Jobs when scheduled work needs different behavior.
- `enabled` defaults to `true`.
- File-backed Schedules sync into Core `schedule` records with `source=file`.
- Removing a file-backed Schedule retires the Core row instead of deleting it.
- Missing target Jobs fail Schedule validation and sync.
- Retired target Jobs fail Schedule validation and sync.
- Disabled target Jobs do not block Schedule sync. Disabling a Job is an operational pause, not metadata invalidation.

## Job Input

Schedules do not pass custom payloads in the MVP. The Job name should describe the scheduled work clearly:

```yaml
schedules:
  - name: weekly-report
    label: Weekly Report
    cron: "0 9 * * MON"
    timezone: Asia/Karachi
    job: sales/send-weekly-report
```

If a future product need appears for parameterized schedules, add payload later. For now, avoiding payload keeps app-owned schedules explicit and less abstract.

## Runtime Behavior

A Schedule never creates infinite future executions upfront.

When `next-run-at <= now`, a worker:

1. locks the due Schedule row with `FOR UPDATE SKIP LOCKED`
2. creates one Job Execution for that occurrence
3. uses a deterministic idempotency key
4. updates `last-run-at`
5. calculates and stores the next `next-run-at`

Idempotency key:

```txt
schedule:<app>/<schedule-name>:<due-time-rfc3339>
```

If the worker was down and missed multiple occurrences, the MVP creates one late execution, then advances to the next future cron time. It does not backfill every missed occurrence.

If the target Job is disabled when a Schedule becomes due, the worker records `last-error`, does not create a Job Execution, and advances to the next future cron time. Disabling a Job is treated as an operational pause, not a reason for metadata sync to fail.

Worker wake-up rules:

- Job Execution notification
- next Job Execution `run-after`
- next Schedule `next-run-at`
- 60s fallback poll

## Core Schedule Fields

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

Constraints:

- unique `(app, key)`

Indexes:

- `(enabled, retired, next-run-at)`
- `job`
- `source`

## Cron Parser

- Use `github.com/robfig/cron/v3` instead of hand-rolling cron parsing.
- Use only parser/schedule calculation APIs, not its in-process job runner.
- Accept standard 5-field cron only: minute, hour, day-of-month, month, day-of-week.
- Do not accept seconds fields or `@every` interval descriptors in the MVP.
- Allow named weekdays/months such as `MON` and `JAN`; `github.com/robfig/cron/v3` supports them directly.
- Do not add custom Dygo aliases or custom cron name translation.

## Testing And Diagnosis

Schedule timing is tested with injected worker time instead of waiting for wall-clock time.

Focused runtime tests prove that:

- `dygo worker --once` checks due Schedules before claiming Job Executions.
- the continuous worker wakes for the next Schedule `next-run-at`, not only the 60s fallback poll.
- cron calculation uses the Schedule timezone.
- Schedule occurrence idempotency keys use the due occurrence time.

Manual local check:

```sh
dygo db migrate
dygo worker --once
dygo job execution list
```

`dygo worker --once` creates executions for due Schedules, claims one available batch, persists the result, and exits.

## Open Questions

None.
