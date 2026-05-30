// Package store persists and claims Job Executions in PostgreSQL.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
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
)

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
	ID      int64
	AppName string
	Key     string
	Queue   string
	Timeout string
	Retry   []byte
	Enabled bool
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
	execution, err := insertExecution(ctx, tx, name, job, timeout, retry, maxAttempts, payloadOrNil(payload), runAfter, idempotencyKey, options.Priority)
	if err != nil {
		return Execution{}, fmt.Errorf("enqueue job %s/%s: %w", appName, jobName, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Execution{}, fmt.Errorf("commit job enqueue: %w", err)
	}
	return execution, nil
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

// Complete marks a running execution succeeded.
func (s Store) Complete(ctx context.Context, id int64, result json.RawMessage, now time.Time) error {
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
WHERE id = $4 AND status = $5`, jobs.StatusSucceeded, payloadOrNil(result), now, id, jobs.StatusRunning)
	if err != nil {
		return fmt.Errorf("complete job execution %d: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("job execution %d is not running", id)
	}
	return nil
}

// Fail records a failed attempt and retries when attempts remain.
func (s Store) Fail(ctx context.Context, id int64, message string, now time.Time) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin job failure: %w", err)
	}
	defer tx.Rollback(ctx)

	var attempts int
	var maxAttempts int
	var retryRaw []byte
	err = tx.QueryRow(ctx, `SELECT attempts, max_attempts, retry FROM "job_execution" WHERE id = $1 AND status = $2 FOR UPDATE`, id, jobs.StatusRunning).Scan(&attempts, &maxAttempts, &retryRaw)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("job execution %d is not running", id)
		}
		return fmt.Errorf("load job execution %d for failure: %w", id, err)
	}
	retry, err := decodeRetry(retryRaw)
	if err != nil {
		return fmt.Errorf("decode retry for job execution %d: %w", id, err)
	}
	if err := retryOrFail(ctx, tx, id, attempts, maxAttempts, retry, message, now); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit job failure: %w", err)
	}
	return nil
}

// FailFinal marks a running execution failed without retry.
func (s Store) FailFinal(ctx context.Context, id int64, message string, now time.Time) error {
	tag, err := execWithStatus(ctx, s.db, finalFailureSQL, jobs.StatusFailed, strings.TrimSpace(message), now, id)
	if err != nil {
		return fmt.Errorf("fail job execution %d: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("job execution %d was not updated", id)
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

func loadJob(ctx context.Context, tx pgx.Tx, appName string, jobName string) (jobRecord, error) {
	var job jobRecord
	err := tx.QueryRow(ctx, `
SELECT j.id, a.name, j.key, j.queue, j.timeout, j.retry, j.enabled
FROM "job" j
JOIN "app" a ON a.id = j.app_id
WHERE a.name = $1 AND j.key = $2`, appName, jobName).Scan(&job.ID, &job.AppName, &job.Key, &job.Queue, &job.Timeout, &job.Retry, &job.Enabled)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return jobRecord{}, fmt.Errorf("job %s/%s is not registered", appName, jobName)
		}
		return jobRecord{}, fmt.Errorf("load job %s/%s: %w", appName, jobName, err)
	}
	return job, nil
}

func insertExecution(ctx context.Context, tx pgx.Tx, name string, job jobRecord, timeout time.Duration, retry *jobs.Retry, maxAttempts int, payload json.RawMessage, runAfter time.Time, idempotencyKey string, priority int) (Execution, error) {
	retryRaw, err := encodeRetry(retry)
	if err != nil {
		return Execution{}, err
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
			return Execution{}, findErr
		}
		if ok {
			return execution, nil
		}
	}
	if err != nil {
		return Execution{}, err
	}
	execution, err := findExecutionByID(ctx, tx, id)
	if err != nil {
		return Execution{}, err
	}
	execution.Timeout = timeout
	return execution, nil
}

func findExecutionByID(ctx context.Context, tx pgx.Tx, id int64) (Execution, error) {
	row := tx.QueryRow(ctx, `
SELECT `+executionSelectColumns+`
FROM "job_execution" e
JOIN "job" j ON j.id = e.job_id
WHERE e.id = $1`, id)
	return scanExecution(row)
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
		if _, err := tx.Exec(ctx, `
UPDATE "job_execution"
SET status = $1,
	error = $2,
	run_after = $3,
	locked_by = NULL,
	locked_until = NULL,
	updated_at = now()
WHERE id = $4`, jobs.StatusQueued, message, now.Add(delay), id); err != nil {
			return fmt.Errorf("queue retry for job execution %d: %w", id, err)
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
