package dygodata

import (
	"context"
	"encoding/json"

	jobstore "github.com/hapyco/dygo/internal/jobs/store"
	"github.com/hapyco/dygo/pkg/dygo"
)

type jobEnqueuer interface {
	Enqueue(context.Context, string, string, json.RawMessage, jobstore.EnqueueOptions) (jobstore.Execution, error)
}

// JobData exposes durable Job enqueueing through the public SDK.
type JobData struct {
	store jobEnqueuer
}

// NewJobData returns dygo JobData backed by a durable Job store.
func NewJobData(store jobEnqueuer) JobData {
	return JobData{store: store}
}

// NewJobDataFromBeginner returns dygo JobData backed by the current transaction or pool.
func NewJobDataFromBeginner(beginner jobstore.Beginner) (JobData, error) {
	store, err := jobstore.New(beginner)
	if err != nil {
		return JobData{}, err
	}
	return NewJobData(store), nil
}

// Enqueue creates one durable Job Execution.
func (d JobData) Enqueue(ctx context.Context, appName string, jobName string, payload json.RawMessage, options dygo.EnqueueOptions) (dygo.JobExecution, error) {
	execution, err := d.store.Enqueue(ctx, appName, jobName, payload, jobstore.EnqueueOptions{
		IdempotencyKey: options.IdempotencyKey,
		Priority:       options.Priority,
		RunAfter:       options.RunAfter,
	})
	if err != nil {
		return dygo.JobExecution{}, err
	}
	return dygo.JobExecution{
		ID:      execution.ID,
		AppName: execution.AppName,
		JobName: execution.JobName,
		Queue:   execution.Queue,
		Attempt: execution.Attempts,
		Payload: execution.Payload,
		Jobs:    d,
	}, nil
}
