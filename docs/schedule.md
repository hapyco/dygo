# Schedules

This file tracks the recurring Schedule decisions before implementation.

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

Proposed shape:

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

Proposed idempotency key:

```txt
schedule:<app>/<schedule-name>:<due-time-rfc3339>
```

If the worker was down and missed multiple occurrences, the MVP creates one late execution, then advances to the next future cron time. It does not backfill every missed occurrence.

Worker wake-up rules:

- Job Execution notification
- next Job Execution `run-after`
- next Schedule `next-run-at`
- 60s fallback poll

## Core Schedule Fields

Proposed Core `schedule` fields:

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

Current repo state: there is no cron parser dependency in `go.mod`.

Recommendation:

- Use a small parser dependency instead of hand-rolling cron parsing.
- Prefer `github.com/robfig/cron/v3` for MVP.
- Use only parser/schedule calculation APIs, not its in-process job runner.
- Accept standard 5-field cron only: minute, hour, day-of-month, month, day-of-week.
- Do not accept seconds fields or `@every` interval descriptors in the MVP.
- Allow named weekdays/months such as `MON` only if the selected cron parser supports them directly.
- Do not add custom Dygo aliases or custom cron name translation.

## Implementation Plan

1. Add Core `schedule` entity metadata.
2. Add `internal/schedules` parser and validation for `_schedules.yml`.
3. Add schedule schema for editor validation.
4. Sync file-backed Schedules during `dygo db migrate`.
5. Add schedule store methods for claiming due Schedules and advancing `next-run-at`.
6. Extend `dygo worker` to check due Schedules and enqueue Job Executions.
7. Update docs and todo status.

## Open Questions

None.
