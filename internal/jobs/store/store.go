// Package store persists and claims Job Executions in PostgreSQL.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hapyco/dygo/internal/jobs"
	namegen "github.com/hapyco/dygo/internal/naming"
	"github.com/hapyco/dygo/internal/queues"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	executionNameLength = 16
	notificationChannel = "dygo_job_execution_queued"
)

// ErrClaimLost means a worker is trying to update an execution it no longer owns.
var ErrClaimLost = errors.New("job execution claim is no longer active")

// Beginner is the PostgreSQL behavior needed by Store.
type Beginner interface {
	Begin(context.Context) (pgx.Tx, error)
}

// Store manages durable Job Executions.
type Store struct {
	db     Beginner
	queues *queues.Config
}

// EnqueueOptions controls one enqueue call.
type EnqueueOptions struct {
	IdempotencyKey string
	Priority       int
	RunAfter       time.Time
}

// ListOptions controls Job Execution listing.
type ListOptions struct {
	Limit int
}

// Job is one registered background Job.
type Job struct {
	ID          int64
	Name        string
	AppName     string
	Key         string
	Source      string
	Label       string
	Description string
	Queue       string
	Timeout     string
	Retry       *jobs.Retry
	Enabled     bool
	Retired     bool
}

// Execution is one durable Job Execution row.
type Execution struct {
	ID             int64
	Name           string
	JobID          int64
	AppName        string
	JobName        string
	Queue          string
	Status         string
	Priority       int
	Payload        json.RawMessage
	Result         json.RawMessage
	Error          string
	Attempts       int
	MaxAttempts    int
	Retry          *jobs.Retry
	RunAfter       time.Time
	StartedAt      *time.Time
	FinishedAt     *time.Time
	LockedBy       string
	LockedUntil    *time.Time
	IdempotencyKey string
	Timeout        time.Duration
}

type jobRecord struct {
	ID          int64
	Name        string
	AppName     string
	Key         string
	Source      string
	Label       string
	Description string
	Queue       string
	Timeout     string
	Retry       []byte
	Enabled     bool
	Retired     bool
}

// New returns a Store backed by db.
func New(db Beginner, queueConfig ...queues.Config) (Store, error) {
	if db == nil {
		return Store{}, fmt.Errorf("job store database is required")
	}
	store := Store{db: db}
	if len(queueConfig) > 0 {
		cfg := queueConfig[0]
		if err := cfg.Validate(); err != nil {
			return Store{}, err
		}
		store.queues = &cfg
	}
	return store, nil
}

// Enqueue creates a queued Job Execution or returns the existing execution for the same idempotency key.
func (s Store) Enqueue(ctx context.Context, appName string, jobName string, payload json.RawMessage, options EnqueueOptions) (Execution, error) {
	if strings.TrimSpace(appName) == "" || strings.TrimSpace(jobName) == "" {
		return Execution{}, fmt.Errorf("job app and name are required")
	}
	if len(payload) > 0 && !json.Valid(payload) {
		return Execution{}, fmt.Errorf("job payload must be valid JSON")
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Execution{}, fmt.Errorf("begin job enqueue: %w", err)
	}
	defer tx.Rollback(ctx)

	job, err := loadJob(ctx, tx, appName, jobName)
	if err != nil {
		return Execution{}, err
	}
	if !job.Enabled {
		return Execution{}, fmt.Errorf("job %s/%s is disabled", appName, jobName)
	}
	if job.Retired {
		return Execution{}, fmt.Errorf("job %s/%s is retired", appName, jobName)
	}
	if s.queues != nil && !s.queues.Has(job.Queue) {
		return Execution{}, fmt.Errorf("job %s/%s references unregistered queue %q", appName, jobName, job.Queue)
	}
	timeout, err := time.ParseDuration(job.Timeout)
	if err != nil || timeout <= 0 {
		return Execution{}, fmt.Errorf("job %s/%s has invalid timeout %q", appName, jobName, job.Timeout)
	}
	retry, err := decodeRetry(job.Retry)
	if err != nil {
		return Execution{}, fmt.Errorf("job %s/%s retry metadata is invalid: %w", appName, jobName, err)
	}
	maxAttempts := 1
	if retry != nil {
		maxAttempts = retry.Attempts
	}
	idempotencyKey := strings.TrimSpace(options.IdempotencyKey)
	if idempotencyKey != "" {
		existing, ok, err := findExecutionByIdempotency(ctx, tx, job.ID, idempotencyKey)
		if err != nil {
			return Execution{}, err
		}
		if ok {
			if err := tx.Commit(ctx); err != nil {
				return Execution{}, fmt.Errorf("commit job enqueue: %w", err)
			}
			return existing, nil
		}
	}
	runAfter := options.RunAfter
	if runAfter.IsZero() {
		runAfter = time.Now().UTC()
	}

	name, err := namegen.Random(executionNameLength)
	if err != nil {
		return Execution{}, fmt.Errorf("generate job execution name: %w", err)
	}
	execution, inserted, err := insertExecution(ctx, tx, name, job, timeout, retry, maxAttempts, payloadOrNil(payload), runAfter, idempotencyKey, options.Priority)
	if err != nil {
		return Execution{}, fmt.Errorf("enqueue job %s/%s: %w", appName, jobName, err)
	}
	if inserted {
		if err := notifyQueue(ctx, tx, execution.Queue); err != nil {
			return Execution{}, fmt.Errorf("notify job queue %q: %w", execution.Queue, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Execution{}, fmt.Errorf("commit job enqueue: %w", err)
	}
	return execution, nil
}

// List returns recent Job Executions, newest first.
func (s Store) List(ctx context.Context, options ListOptions) ([]Execution, error) {
	limit := options.Limit
	if limit == 0 {
		limit = 20
	}
	if limit < 1 {
		return nil, fmt.Errorf("job execution list limit must be greater than 0")
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin job execution list: %w", err)
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `
SELECT `+executionSelectColumns+`
FROM "job_execution" e
JOIN "job" j ON j.id = e.job_id
ORDER BY e.id DESC
LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("query job executions: %w", err)
	}
	defer rows.Close()

	var executions []Execution
	for rows.Next() {
		execution, err := scanExecution(rows)
		if err != nil {
			return nil, fmt.Errorf("scan job execution: %w", err)
		}
		executions = append(executions, execution)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scan job executions: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit job execution list: %w", err)
	}
	return executions, nil
}

// ListJobs returns registered Jobs ordered by app and key.
func (s Store) ListJobs(ctx context.Context) ([]Job, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin job list: %w", err)
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `
SELECT `+jobSelectColumns+`
FROM "job" j
JOIN "app" a ON a.id = j.app_id
ORDER BY a.name ASC, j.key ASC`)
	if err != nil {
		return nil, fmt.Errorf("query jobs: %w", err)
	}
	defer rows.Close()

	var result []Job
	for rows.Next() {
		record, err := scanJobRecord(rows)
		if err != nil {
			return nil, fmt.Errorf("scan job: %w", err)
		}
		job, err := jobFromRecord(record)
		if err != nil {
			return nil, fmt.Errorf("decode job %s/%s: %w", record.AppName, record.Key, err)
		}
		result = append(result, job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scan jobs: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit job list: %w", err)
	}
	return result, nil
}

// GetJob returns one registered Job by app and key.
func (s Store) GetJob(ctx context.Context, appName string, jobName string) (Job, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Job{}, fmt.Errorf("begin job lookup: %w", err)
	}
	defer tx.Rollback(ctx)

	record, err := loadJob(ctx, tx, appName, jobName)
	if err != nil {
		return Job{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Job{}, fmt.Errorf("commit job lookup: %w", err)
	}
	return jobFromRecord(record)
}

// DisableJob prevents future Job Executions from being enqueued.
func (s Store) DisableJob(ctx context.Context, appName string, jobName string) (Job, error) {
	return s.setJobEnabled(ctx, appName, jobName, false)
}

// EnableJob allows future Job Executions to be enqueued when the Job is not retired.
func (s Store) EnableJob(ctx context.Context, appName string, jobName string) (Job, error) {
	return s.setJobEnabled(ctx, appName, jobName, true)
}

// Get returns one Job Execution by numeric id or execution name.
func (s Store) Get(ctx context.Context, reference string) (Execution, error) {
	reference = strings.TrimSpace(reference)
	if reference == "" {
		return Execution{}, fmt.Errorf("job execution id or name is required")
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Execution{}, fmt.Errorf("begin job execution lookup: %w", err)
	}
	defer tx.Rollback(ctx)

	execution, err := findExecutionByReference(ctx, tx, reference)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Execution{}, fmt.Errorf("job execution %q was not found", reference)
		}
		return Execution{}, fmt.Errorf("load job execution %q: %w", reference, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Execution{}, fmt.Errorf("commit job execution lookup: %w", err)
	}
	return execution, nil
}

// CancelQueued marks a queued Job Execution cancelled by id or name.
func (s Store) CancelQueued(ctx context.Context, reference string, now time.Time) (Execution, error) {
	reference = strings.TrimSpace(reference)
	execution, err := s.Get(ctx, reference)
	if err != nil {
		return Execution{}, err
	}
	if execution.Status != jobs.StatusQueued {
		return Execution{}, fmt.Errorf("job execution %q is %s; only queued executions can be cancelled", reference, execution.Status)
	}
	cancelled, err := s.Cancel(ctx, execution.ID, now)
	if err != nil {
		return Execution{}, err
	}
	if !cancelled {
		return Execution{}, fmt.Errorf("job execution %q is no longer queued", reference)
	}
	finishedAt := now.UTC()
	execution.Status = jobs.StatusCancelled
	execution.FinishedAt = &finishedAt
	return execution, nil
}

// Retry creates a new queued Job Execution from a failed execution's payload.
func (s Store) Retry(ctx context.Context, reference string, idempotencyKey string) (Execution, error) {
	reference = strings.TrimSpace(reference)
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" {
		return Execution{}, fmt.Errorf("manual retry requires an idempotency key")
	}
	execution, err := s.Get(ctx, reference)
	if err != nil {
		return Execution{}, err
	}
	if execution.Status != jobs.StatusFailed {
		return Execution{}, fmt.Errorf("job execution %q is %s; only failed executions can be retried", reference, execution.Status)
	}
	return s.Enqueue(ctx, execution.AppName, execution.JobName, execution.Payload, EnqueueOptions{
		IdempotencyKey: idempotencyKey,
	})
}

// RecoverExpired releases expired running executions for retry or final failure.
func (s Store) RecoverExpired(ctx context.Context, now time.Time) (int, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin job recovery: %w", err)
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `
SELECT id, attempts, max_attempts, retry
FROM "job_execution"
WHERE status = $1
  AND locked_until <= $2
FOR UPDATE SKIP LOCKED`, jobs.StatusRunning, now)
	if err != nil {
		return 0, fmt.Errorf("query expired job executions: %w", err)
	}
	defer rows.Close()

	type expired struct {
		id          int64
		attempts    int
		maxAttempts int
		retry       []byte
	}
	var expiredRows []expired
	for rows.Next() {
		var row expired
		if err := rows.Scan(&row.id, &row.attempts, &row.maxAttempts, &row.retry); err != nil {
			return 0, fmt.Errorf("scan expired job execution: %w", err)
		}
		expiredRows = append(expiredRows, row)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("scan expired job executions: %w", err)
	}
	for _, row := range expiredRows {
		retry, err := decodeRetry(row.retry)
		if err != nil {
			return 0, fmt.Errorf("decode retry for expired job execution %d: %w", row.id, err)
		}
		if err := retryOrFail(ctx, tx, row.id, row.attempts, row.maxAttempts, retry, "worker lease expired", now); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit job recovery: %w", err)
	}
	return len(expiredRows), nil
}

// Claim claims queued executions from selected queues.
func (s Store) Claim(ctx context.Context, queueNames []string, limit int, workerID string, now time.Time) ([]Execution, error) {
	queueNames = normalizeQueueNames(queueNames)
	if len(queueNames) == 0 || limit < 1 {
		return nil, nil
	}
	if strings.TrimSpace(workerID) == "" {
		return nil, fmt.Errorf("worker id is required")
	}
	if s.queues != nil {
		for _, queue := range queueNames {
			if !s.queues.Has(queue) {
				return nil, fmt.Errorf("queue %q is not registered", queue)
			}
		}
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin job claim: %w", err)
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `
SELECT e.id, j.timeout
FROM "job_execution" e
JOIN "job" j ON j.id = e.job_id
WHERE e.status = $1
  AND e.run_after <= $2
  AND e.queue = ANY($3)
ORDER BY e.priority DESC, e.run_after ASC, e.id ASC
FOR UPDATE SKIP LOCKED
LIMIT $4`, jobs.StatusQueued, now, queueNames, limit)
	if err != nil {
		return nil, fmt.Errorf("query queued job executions: %w", err)
	}
	defer rows.Close()

	type claimed struct {
		id      int64
		timeout time.Duration
	}
	var claimedRows []claimed
	for rows.Next() {
		var id int64
		var timeoutValue string
		if err := rows.Scan(&id, &timeoutValue); err != nil {
			return nil, fmt.Errorf("scan queued job execution: %w", err)
		}
		timeout, err := time.ParseDuration(timeoutValue)
		if err != nil || timeout <= 0 {
			return nil, fmt.Errorf("job execution %d has invalid timeout %q", id, timeoutValue)
		}
		claimedRows = append(claimedRows, claimed{id: id, timeout: timeout})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scan queued job executions: %w", err)
	}

	executions := make([]Execution, 0, len(claimedRows))
	for _, claimed := range claimedRows {
		row := tx.QueryRow(ctx, claimExecutionSQL, jobs.StatusRunning, now, now.Add(claimed.timeout), workerID, claimed.id)
		execution, err := scanExecution(row)
		if err != nil {
			return nil, fmt.Errorf("claim job execution %d: %w", claimed.id, err)
		}
		execution.Timeout = claimed.timeout
		executions = append(executions, execution)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit job claim: %w", err)
	}
	return executions, nil
}

// NextRunAfter returns the earliest future queued execution for selected queues.
func (s Store) NextRunAfter(ctx context.Context, queueNames []string, now time.Time) (*time.Time, error) {
	queueNames = normalizeQueueNames(queueNames)
	if len(queueNames) == 0 {
		return nil, nil
	}
	if s.queues != nil {
		for _, queue := range queueNames {
			if !s.queues.Has(queue) {
				return nil, fmt.Errorf("queue %q is not registered", queue)
			}
		}
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin next job run lookup: %w", err)
	}
	defer tx.Rollback(ctx)

	var next sql.NullTime
	if err := tx.QueryRow(ctx, `
SELECT MIN(run_after)
FROM "job_execution"
WHERE status = $1
  AND queue = ANY($2)
  AND run_after > $3`, jobs.StatusQueued, queueNames, now).Scan(&next); err != nil {
		return nil, fmt.Errorf("query next job run: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit next job run lookup: %w", err)
	}
	if !next.Valid {
		return nil, nil
	}
	nextTime := next.Time.UTC()
	return &nextTime, nil
}

// Complete marks a running execution succeeded when the worker still owns the claim.
func (s Store) Complete(ctx context.Context, execution Execution, result json.RawMessage, now time.Time) error {
	if len(result) > 0 && !json.Valid(result) {
		return fmt.Errorf("job result must be valid JSON")
	}
	tag, err := execWithStatus(ctx, s.db, `
UPDATE "job_execution"
SET status = $1,
	result = $2,
	finished_at = $3,
	locked_by = NULL,
	locked_until = NULL,
	updated_at = now()
WHERE id = $4
  AND status = $5
  AND locked_by = $6
  AND attempts = $7`, jobs.StatusSucceeded, payloadOrNil(result), now, execution.ID, jobs.StatusRunning, execution.LockedBy, execution.Attempts)
	if err != nil {
		return fmt.Errorf("complete job execution %d: %w", execution.ID, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("complete job execution %d: %w", execution.ID, ErrClaimLost)
	}
	return nil
}

// Fail records a failed attempt and retries when attempts remain and the worker still owns the claim.
func (s Store) Fail(ctx context.Context, execution Execution, message string, now time.Time) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin job failure: %w", err)
	}
	defer tx.Rollback(ctx)

	var attempts int
	var maxAttempts int
	var retryRaw []byte
	err = tx.QueryRow(ctx, `
SELECT attempts, max_attempts, retry
FROM "job_execution"
WHERE id = $1
  AND status = $2
  AND locked_by = $3
  AND attempts = $4
FOR UPDATE`, execution.ID, jobs.StatusRunning, execution.LockedBy, execution.Attempts).Scan(&attempts, &maxAttempts, &retryRaw)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("fail job execution %d: %w", execution.ID, ErrClaimLost)
		}
		return fmt.Errorf("load job execution %d for failure: %w", execution.ID, err)
	}
	retry, err := decodeRetry(retryRaw)
	if err != nil {
		return fmt.Errorf("decode retry for job execution %d: %w", execution.ID, err)
	}
	if err := retryOrFail(ctx, tx, execution.ID, attempts, maxAttempts, retry, message, now); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit job failure: %w", err)
	}
	return nil
}

// FailFinal marks a running execution failed without retry when the worker still owns the claim.
func (s Store) FailFinal(ctx context.Context, execution Execution, message string, now time.Time) error {
	tag, err := execWithStatus(ctx, s.db, claimedFinalFailureSQL, jobs.StatusFailed, strings.TrimSpace(message), now, execution.ID, execution.LockedBy, execution.Attempts)
	if err != nil {
		return fmt.Errorf("fail job execution %d: %w", execution.ID, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("fail job execution %d: %w", execution.ID, ErrClaimLost)
	}
	return nil
}

// ExpireClaim shortens an in-flight claim lease when a worker cannot finish shutdown cleanly.
func (s Store) ExpireClaim(ctx context.Context, execution Execution, now time.Time) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin job claim expiry: %w", err)
	}
	defer tx.Rollback(ctx)

	var queue string
	err = tx.QueryRow(ctx, `
UPDATE "job_execution"
SET locked_until = $1,
	updated_at = now()
WHERE id = $2
  AND status = $3
  AND locked_by = $4
  AND attempts = $5
RETURNING queue`, now, execution.ID, jobs.StatusRunning, execution.LockedBy, execution.Attempts).Scan(&queue)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("expire job execution %d claim: %w", execution.ID, ErrClaimLost)
		}
		return fmt.Errorf("expire job execution %d claim: %w", execution.ID, err)
	}
	if err := notifyQueue(ctx, tx, queue); err != nil {
		return fmt.Errorf("notify job queue %q: %w", queue, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit job claim expiry: %w", err)
	}
	return nil
}

// Cancel marks a queued execution cancelled.
func (s Store) Cancel(ctx context.Context, id int64, now time.Time) (bool, error) {
	tag, err := execWithStatus(ctx, s.db, `
UPDATE "job_execution"
SET status = $1,
	finished_at = $2,
	updated_at = now()
WHERE id = $3 AND status = $4`, jobs.StatusCancelled, now, id, jobs.StatusQueued)
	if err != nil {
		return false, fmt.Errorf("cancel job execution %d: %w", id, err)
	}
	return tag.RowsAffected() > 0, nil
}

const executionSelectColumns = `
e.id, e.name, e.job_id, e.app_name, e.job_name, e.queue, e.status, e.priority,
e.payload, e.result, COALESCE(e.error, ''), e.attempts, e.max_attempts, e.retry,
e.run_after, e.started_at, e.finished_at, COALESCE(e.locked_by, ''), e.locked_until,
COALESCE(e.idempotency_key, ''), j.timeout`

const claimExecutionSQL = `
UPDATE "job_execution" e
SET status = $1,
	started_at = $2,
	locked_until = $3,
	locked_by = $4,
	attempts = attempts + 1,
	updated_at = now()
FROM "job" j
WHERE e.id = $5
  AND e.job_id = j.id
RETURNING ` + executionSelectColumns

const finalFailureSQL = `
UPDATE "job_execution"
SET status = $1,
	error = $2,
	finished_at = $3,
	locked_by = NULL,
	locked_until = NULL,
	updated_at = now()
WHERE id = $4`

const claimedFinalFailureSQL = `
UPDATE "job_execution"
SET status = $1,
	error = $2,
	finished_at = $3,
	locked_by = NULL,
	locked_until = NULL,
	updated_at = now()
WHERE id = $4
  AND status = 'running'
  AND locked_by = $5
  AND attempts = $6`

const jobSelectColumns = `
j.id, j.name, a.name, j.key, j.source, j.label, COALESCE(j.description, ''),
j.queue, j.timeout, j.retry, j.enabled, j.retired`

func loadJob(ctx context.Context, tx pgx.Tx, appName string, jobName string) (jobRecord, error) {
	appName = strings.TrimSpace(appName)
	jobName = strings.TrimSpace(jobName)
	if appName == "" || jobName == "" {
		return jobRecord{}, fmt.Errorf("job app and name are required")
	}
	row := tx.QueryRow(ctx, `
SELECT `+jobSelectColumns+`
FROM "job" j
JOIN "app" a ON a.id = j.app_id
WHERE a.name = $1 AND j.key = $2`, appName, jobName)
	job, err := scanJobRecord(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return jobRecord{}, fmt.Errorf("job %s/%s is not registered", appName, jobName)
		}
		return jobRecord{}, fmt.Errorf("load job %s/%s: %w", appName, jobName, err)
	}
	return job, nil
}

func loadJobForUpdate(ctx context.Context, tx pgx.Tx, appName string, jobName string) (jobRecord, error) {
	appName = strings.TrimSpace(appName)
	jobName = strings.TrimSpace(jobName)
	if appName == "" || jobName == "" {
		return jobRecord{}, fmt.Errorf("job app and name are required")
	}
	row := tx.QueryRow(ctx, `
SELECT `+jobSelectColumns+`
FROM "job" j
JOIN "app" a ON a.id = j.app_id
WHERE a.name = $1 AND j.key = $2
FOR UPDATE OF j`, appName, jobName)
	job, err := scanJobRecord(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return jobRecord{}, fmt.Errorf("job %s/%s is not registered", appName, jobName)
		}
		return jobRecord{}, fmt.Errorf("load job %s/%s: %w", appName, jobName, err)
	}
	return job, nil
}

func (s Store) setJobEnabled(ctx context.Context, appName string, jobName string, enabled bool) (Job, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Job{}, fmt.Errorf("begin job update: %w", err)
	}
	defer tx.Rollback(ctx)

	record, err := loadJobForUpdate(ctx, tx, appName, jobName)
	if err != nil {
		return Job{}, err
	}
	if enabled && record.Retired {
		return Job{}, fmt.Errorf("job %s/%s is retired; restore its job.yml and run dygo db migrate before enabling it", record.AppName, record.Key)
	}
	if _, err := tx.Exec(ctx, `UPDATE "job" SET enabled = $1, updated_at = now() WHERE id = $2`, enabled, record.ID); err != nil {
		return Job{}, fmt.Errorf("update job %s/%s: %w", record.AppName, record.Key, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Job{}, fmt.Errorf("commit job update: %w", err)
	}
	record.Enabled = enabled
	return jobFromRecord(record)
}

func scanJobRecord(row scanner) (jobRecord, error) {
	var job jobRecord
	if err := row.Scan(
		&job.ID,
		&job.Name,
		&job.AppName,
		&job.Key,
		&job.Source,
		&job.Label,
		&job.Description,
		&job.Queue,
		&job.Timeout,
		&job.Retry,
		&job.Enabled,
		&job.Retired,
	); err != nil {
		return jobRecord{}, err
	}
	return job, nil
}

func jobFromRecord(record jobRecord) (Job, error) {
	retry, err := decodeRetry(record.Retry)
	if err != nil {
		return Job{}, err
	}
	return Job{
		ID:          record.ID,
		Name:        record.Name,
		AppName:     record.AppName,
		Key:         record.Key,
		Source:      record.Source,
		Label:       record.Label,
		Description: record.Description,
		Queue:       record.Queue,
		Timeout:     record.Timeout,
		Retry:       retry,
		Enabled:     record.Enabled,
		Retired:     record.Retired,
	}, nil
}

func insertExecution(ctx context.Context, tx pgx.Tx, name string, job jobRecord, timeout time.Duration, retry *jobs.Retry, maxAttempts int, payload json.RawMessage, runAfter time.Time, idempotencyKey string, priority int) (Execution, bool, error) {
	retryRaw, err := encodeRetry(retry)
	if err != nil {
		return Execution{}, false, err
	}
	var id int64
	err = tx.QueryRow(ctx, `
INSERT INTO "job_execution" (name, job_id, app_name, job_name, queue, status, priority, payload, attempts, max_attempts, retry, run_after, idempotency_key)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 0, $9, $10, $11, $12)
ON CONFLICT ON CONSTRAINT "job_execution_job_idempotency_key_key" DO NOTHING
RETURNING id`, name, job.ID, job.AppName, job.Key, job.Queue, jobs.StatusQueued, priority, payload, maxAttempts, retryRaw, runAfter, nullIfEmpty(idempotencyKey)).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) && idempotencyKey != "" {
		execution, ok, findErr := findExecutionByIdempotency(ctx, tx, job.ID, idempotencyKey)
		if findErr != nil {
			return Execution{}, false, findErr
		}
		if ok {
			return execution, false, nil
		}
	}
	if err != nil {
		return Execution{}, false, err
	}
	execution, err := findExecutionByID(ctx, tx, id)
	if err != nil {
		return Execution{}, false, err
	}
	execution.Timeout = timeout
	return execution, true, nil
}

func findExecutionByID(ctx context.Context, tx pgx.Tx, id int64) (Execution, error) {
	row := tx.QueryRow(ctx, `
SELECT `+executionSelectColumns+`
FROM "job_execution" e
JOIN "job" j ON j.id = e.job_id
WHERE e.id = $1`, id)
	return scanExecution(row)
}

func findExecutionByName(ctx context.Context, tx pgx.Tx, name string) (Execution, error) {
	row := tx.QueryRow(ctx, `
SELECT `+executionSelectColumns+`
FROM "job_execution" e
JOIN "job" j ON j.id = e.job_id
WHERE e.name = $1`, name)
	return scanExecution(row)
}

func findExecutionByReference(ctx context.Context, tx pgx.Tx, reference string) (Execution, error) {
	if id, err := strconv.ParseInt(reference, 10, 64); err == nil {
		if id < 1 {
			return Execution{}, fmt.Errorf("job execution id must be greater than 0")
		}
		return findExecutionByID(ctx, tx, id)
	}
	return findExecutionByName(ctx, tx, reference)
}

func findExecutionByIdempotency(ctx context.Context, tx pgx.Tx, jobID int64, idempotencyKey string) (Execution, bool, error) {
	row := tx.QueryRow(ctx, `
SELECT `+executionSelectColumns+`
FROM "job_execution" e
JOIN "job" j ON j.id = e.job_id
WHERE e.job_id = $1 AND e.idempotency_key = $2
ORDER BY e.id ASC
LIMIT 1`, jobID, idempotencyKey)
	execution, err := scanExecution(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Execution{}, false, nil
		}
		return Execution{}, false, fmt.Errorf("find existing job execution: %w", err)
	}
	return execution, true, nil
}

type scanner interface {
	Scan(...any) error
}

func scanExecution(row scanner) (Execution, error) {
	var execution Execution
	var payload []byte
	var result []byte
	var retryRaw []byte
	var startedAt sql.NullTime
	var finishedAt sql.NullTime
	var lockedUntil sql.NullTime
	var timeoutValue string
	if err := row.Scan(
		&execution.ID,
		&execution.Name,
		&execution.JobID,
		&execution.AppName,
		&execution.JobName,
		&execution.Queue,
		&execution.Status,
		&execution.Priority,
		&payload,
		&result,
		&execution.Error,
		&execution.Attempts,
		&execution.MaxAttempts,
		&retryRaw,
		&execution.RunAfter,
		&startedAt,
		&finishedAt,
		&execution.LockedBy,
		&lockedUntil,
		&execution.IdempotencyKey,
		&timeoutValue,
	); err != nil {
		return Execution{}, err
	}
	execution.Payload = rawMessageOrNil(payload)
	execution.Result = rawMessageOrNil(result)
	retry, err := decodeRetry(retryRaw)
	if err != nil {
		return Execution{}, err
	}
	execution.Retry = retry
	if startedAt.Valid {
		execution.StartedAt = &startedAt.Time
	}
	if finishedAt.Valid {
		execution.FinishedAt = &finishedAt.Time
	}
	if lockedUntil.Valid {
		execution.LockedUntil = &lockedUntil.Time
	}
	timeout, err := time.ParseDuration(timeoutValue)
	if err != nil || timeout <= 0 {
		return Execution{}, fmt.Errorf("invalid timeout %q", timeoutValue)
	}
	execution.Timeout = timeout
	return execution, nil
}

func retryOrFail(ctx context.Context, tx pgx.Tx, id int64, attempts int, maxAttempts int, retry *jobs.Retry, message string, now time.Time) error {
	message = strings.TrimSpace(message)
	if attempts < maxAttempts {
		delay, err := retryDelay(retry, attempts)
		if err != nil {
			return fmt.Errorf("compute retry delay for job execution %d: %w", id, err)
		}
		var queue string
		if err := tx.QueryRow(ctx, `
UPDATE "job_execution"
SET status = $1,
	error = $2,
	run_after = $3,
	locked_by = NULL,
	locked_until = NULL,
	updated_at = now()
WHERE id = $4
RETURNING queue`, jobs.StatusQueued, message, now.Add(delay), id).Scan(&queue); err != nil {
			return fmt.Errorf("queue retry for job execution %d: %w", id, err)
		}
		if err := notifyQueue(ctx, tx, queue); err != nil {
			return fmt.Errorf("notify job queue %q: %w", queue, err)
		}
		return nil
	}
	if _, err := tx.Exec(ctx, finalFailureSQL, jobs.StatusFailed, message, now, id); err != nil {
		return fmt.Errorf("fail job execution %d: %w", id, err)
	}
	return nil
}

func retryDelay(retry *jobs.Retry, attempt int) (time.Duration, error) {
	if retry == nil {
		return 0, nil
	}
	initial, err := time.ParseDuration(retry.InitialDelay)
	if err != nil {
		return 0, err
	}
	maxDelay, err := time.ParseDuration(retry.MaxDelay)
	if err != nil {
		return 0, err
	}
	delay := initial
	for i := 1; i < attempt; i++ {
		delay *= 2
		if delay >= maxDelay {
			return maxDelay, nil
		}
	}
	if delay > maxDelay {
		return maxDelay, nil
	}
	return delay, nil
}

func decodeRetry(raw []byte) (*jobs.Retry, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var retry jobs.Retry
	if err := json.Unmarshal(raw, &retry); err != nil {
		return nil, err
	}
	return &retry, nil
}

func encodeRetry(retry *jobs.Retry) ([]byte, error) {
	if retry == nil {
		return nil, nil
	}
	return json.Marshal(retry)
}

func execWithStatus(ctx context.Context, db Beginner, sql string, args ...any) (pgconn.CommandTag, error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return pgconn.CommandTag{}, err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, sql, args...)
	if err != nil {
		return pgconn.CommandTag{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return pgconn.CommandTag{}, err
	}
	return tag, nil
}

func notifyQueue(ctx context.Context, tx pgx.Tx, queue string) error {
	_, err := tx.Exec(ctx, `SELECT pg_notify($1, $2)`, notificationChannel, queue)
	return err
}

func payloadOrNil(payload json.RawMessage) json.RawMessage {
	if len(payload) == 0 {
		return nil
	}
	return payload
}

func rawMessageOrNil(raw []byte) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	return append(json.RawMessage(nil), raw...)
}

func nullIfEmpty(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func normalizeQueueNames(names []string) []string {
	seen := map[string]struct{}{}
	normalized := make([]string, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		normalized = append(normalized, name)
	}
	return normalized
}
