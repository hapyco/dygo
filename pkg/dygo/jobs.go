package dygo

import (
	"context"
	"encoding/json"
	"time"
)

// JobExecution is one durable queued occurrence of a Job.
type JobExecution struct {
	ID      int64
	AppName string
	JobName string
	Queue   string
	Attempt int
	Payload json.RawMessage

	Records RecordData
	Jobs    JobData
}

// JobFunc handles one Job Execution.
type JobFunc func(context.Context, JobExecution) error

// JobRegistry is the public app-facing Job registration API.
type JobRegistry interface {
	RegisterJob(appName string, jobName string, fn JobFunc) error
}

// JobRegistrar registers compiled app Jobs with dygo.
type JobRegistrar func(JobRegistry) error

// EnqueueOptions controls one Job Execution enqueue call.
type EnqueueOptions struct {
	IdempotencyKey string
	Priority       int
	RunAfter       time.Time
}

// JobData gives app code access to durable Jobs.
type JobData interface {
	Enqueue(ctx context.Context, appName string, jobName string, payload json.RawMessage, options EnqueueOptions) (JobExecution, error)
}
