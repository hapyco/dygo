package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	namegen "github.com/hapyco/dygo/internal/naming"
	"github.com/hapyco/dygo/internal/schedules"
	"github.com/jackc/pgx/v5"
)

const defaultScheduleClaimLimit = 20

type dueSchedule struct {
	ID        int64
	AppName   string
	Key       string
	Cron      string
	Timezone  string
	JobApp    string
	JobName   string
	NextRunAt time.Time
	Job       jobRecord
}

// RunDueSchedules creates Job Executions for due Schedules in the selected queues.
func (s Store) RunDueSchedules(ctx context.Context, queueNames []string, workerID string, now time.Time, limit int) (int, error) {
	queueNames, err := s.scheduleQueueNames(queueNames)
	if err != nil {
		return 0, err
	}
	if len(queueNames) == 0 {
		return 0, nil
	}
	if strings.TrimSpace(workerID) == "" {
		return 0, fmt.Errorf("worker id is required")
	}
	if limit < 1 {
		limit = defaultScheduleClaimLimit
	}
	now = now.UTC()

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin schedule run: %w", err)
	}
	defer tx.Rollback(ctx)

	due, err := findDueSchedules(ctx, tx, queueNames, now, limit)
	if err != nil {
		return 0, err
	}
	for _, schedule := range due {
		if err := runDueSchedule(ctx, tx, schedule, now); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit schedule run: %w", err)
	}
	return len(due), nil
}

// NextScheduleRunAt returns the earliest future Schedule occurrence for selected queues.
func (s Store) NextScheduleRunAt(ctx context.Context, queueNames []string, now time.Time) (*time.Time, error) {
	queueNames, err := s.scheduleQueueNames(queueNames)
	if err != nil {
		return nil, err
	}
	if len(queueNames) == 0 {
		return nil, nil
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin next schedule run lookup: %w", err)
	}
	defer tx.Rollback(ctx)

	var next sql.NullTime
	if err := tx.QueryRow(ctx, `
SELECT MIN(s.next_run_at)
FROM "schedule" s
JOIN "job" j ON j.id = s.job_id
WHERE s.enabled = true
  AND s.retired = false
  AND s.next_run_at IS NOT NULL
  AND s.next_run_at > $1
  AND j.queue = ANY($2)`, now.UTC(), queueNames).Scan(&next); err != nil {
		return nil, fmt.Errorf("query next schedule run: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit next schedule run lookup: %w", err)
	}
	if !next.Valid {
		return nil, nil
	}
	nextTime := next.Time.UTC()
	return &nextTime, nil
}

func (s Store) scheduleQueueNames(queueNames []string) ([]string, error) {
	queueNames = normalizeQueueNames(queueNames)
	if s.queues != nil {
		for _, queue := range queueNames {
			if !s.queues.Has(queue) {
				return nil, fmt.Errorf("queue %q is not registered", queue)
			}
		}
	}
	return queueNames, nil
}

func findDueSchedules(ctx context.Context, tx pgx.Tx, queueNames []string, now time.Time, limit int) ([]dueSchedule, error) {
	rows, err := tx.Query(ctx, `
SELECT
	s.id,
	a.name,
	s.key,
	s.cron,
	s.timezone,
	s.job_app_name,
	s.job_name,
	s.next_run_at,
	j.id,
	j.name,
	ja.name,
	j.key,
	j.source,
	j.label,
	COALESCE(j.description, ''),
	j.queue,
	j.timeout,
	j.retry,
	j.enabled,
	j.retired
FROM "schedule" s
JOIN "app" a ON a.id = s.app_id
JOIN "job" j ON j.id = s.job_id
JOIN "app" ja ON ja.id = j.app_id
WHERE s.enabled = true
  AND s.retired = false
  AND s.next_run_at IS NOT NULL
  AND s.next_run_at <= $1
  AND j.queue = ANY($2)
ORDER BY s.next_run_at ASC, s.id ASC
FOR UPDATE OF s SKIP LOCKED
LIMIT $3`, now, queueNames, limit)
	if err != nil {
		return nil, fmt.Errorf("query due schedules: %w", err)
	}
	defer rows.Close()

	var due []dueSchedule
	for rows.Next() {
		var schedule dueSchedule
		if err := rows.Scan(
			&schedule.ID,
			&schedule.AppName,
			&schedule.Key,
			&schedule.Cron,
			&schedule.Timezone,
			&schedule.JobApp,
			&schedule.JobName,
			&schedule.NextRunAt,
			&schedule.Job.ID,
			&schedule.Job.Name,
			&schedule.Job.AppName,
			&schedule.Job.Key,
			&schedule.Job.Source,
			&schedule.Job.Label,
			&schedule.Job.Description,
			&schedule.Job.Queue,
			&schedule.Job.Timeout,
			&schedule.Job.Retry,
			&schedule.Job.Enabled,
			&schedule.Job.Retired,
		); err != nil {
			return nil, fmt.Errorf("scan due schedule: %w", err)
		}
		due = append(due, schedule)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scan due schedules: %w", err)
	}
	return due, nil
}

func runDueSchedule(ctx context.Context, tx pgx.Tx, schedule dueSchedule, now time.Time) error {
	nextRunAt, err := schedules.NextRunAt(schedule.Cron, schedule.Timezone, now)
	if err != nil {
		return fmt.Errorf("compute next schedule run for %s/%s: %w", schedule.AppName, schedule.Key, err)
	}

	if message := scheduleJobBlocker(schedule.Job); message != "" {
		return updateScheduleFailure(ctx, tx, schedule.ID, message, nextRunAt)
	}
	timeout, err := time.ParseDuration(schedule.Job.Timeout)
	if err != nil || timeout <= 0 {
		return updateScheduleFailure(ctx, tx, schedule.ID, fmt.Sprintf("job %s/%s has invalid timeout %q", schedule.Job.AppName, schedule.Job.Key, schedule.Job.Timeout), nextRunAt)
	}
	retry, err := decodeRetry(schedule.Job.Retry)
	if err != nil {
		return updateScheduleFailure(ctx, tx, schedule.ID, fmt.Sprintf("job %s/%s retry metadata is invalid: %v", schedule.Job.AppName, schedule.Job.Key, err), nextRunAt)
	}
	maxAttempts := 1
	if retry != nil {
		maxAttempts = retry.Attempts
	}

	name, err := namegen.Random(executionNameLength)
	if err != nil {
		return fmt.Errorf("generate scheduled job execution name: %w", err)
	}
	idempotencyKey := scheduleIdempotencyKey(schedule.AppName, schedule.Key, schedule.NextRunAt)
	execution, inserted, err := insertExecution(ctx, tx, name, schedule.Job, timeout, retry, maxAttempts, nil, schedule.NextRunAt.UTC(), idempotencyKey, 0)
	if err != nil {
		return fmt.Errorf("enqueue scheduled job %s/%s for schedule %s/%s: %w", schedule.Job.AppName, schedule.Job.Key, schedule.AppName, schedule.Key, err)
	}
	if inserted {
		if err := notifyQueue(ctx, tx, execution.Queue); err != nil {
			return fmt.Errorf("notify job queue %q: %w", execution.Queue, err)
		}
	}
	return updateScheduleSuccess(ctx, tx, schedule.ID, schedule.NextRunAt.UTC(), nextRunAt)
}

func scheduleJobBlocker(job jobRecord) string {
	if !job.Enabled {
		return fmt.Sprintf("job %s/%s is disabled", job.AppName, job.Key)
	}
	if job.Retired {
		return fmt.Sprintf("job %s/%s is retired", job.AppName, job.Key)
	}
	return ""
}

func scheduleIdempotencyKey(appName string, scheduleName string, dueAt time.Time) string {
	return "schedule:" + appName + "/" + scheduleName + ":" + dueAt.UTC().Format(time.RFC3339)
}

func updateScheduleSuccess(ctx context.Context, tx pgx.Tx, id int64, lastRunAt time.Time, nextRunAt time.Time) error {
	if _, err := tx.Exec(ctx, `
UPDATE "schedule"
SET last_run_at = $2,
	last_error = NULL,
	next_run_at = $3,
	updated_at = now()
WHERE id = $1`, id, lastRunAt, nextRunAt); err != nil {
		return fmt.Errorf("update schedule %d after run: %w", id, err)
	}
	return nil
}

func updateScheduleFailure(ctx context.Context, tx pgx.Tx, id int64, message string, nextRunAt time.Time) error {
	if _, err := tx.Exec(ctx, `
UPDATE "schedule"
SET last_error = $2,
	next_run_at = $3,
	updated_at = now()
WHERE id = $1`, id, strings.TrimSpace(message), nextRunAt); err != nil {
		return fmt.Errorf("update schedule %d after failure: %w", id, err)
	}
	return nil
}
