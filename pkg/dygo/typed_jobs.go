package dygo

import (
	"context"
	"encoding/json"
	"fmt"
)

// TypedJob binds a Job identity to the Go type used to enqueue and handle its payload.
// The durable payload remains JSON; callers must still validate business requirements.
type TypedJob[T any] struct {
	appName string
	jobName string
}

// NewTypedJob identifies a Job whose payload is represented by T.
func NewTypedJob[T any](appName, jobName string) TypedJob[T] {
	return TypedJob[T]{appName: appName, jobName: jobName}
}

// Handler adapts a typed handler to the existing JobFunc registration API.
// An absent payload is decoded as JSON null. A nil handler returns nil.
func (j TypedJob[T]) Handler(fn func(context.Context, JobExecution, T) error) JobFunc {
	if fn == nil {
		return nil
	}
	return func(ctx context.Context, execution JobExecution) error {
		var payload T
		raw := execution.Payload
		if len(raw) == 0 {
			raw = json.RawMessage("null")
		}
		if err := json.Unmarshal(raw, &payload); err != nil {
			return fmt.Errorf("decode job payload: %w", err)
		}
		return fn(ctx, execution, payload)
	}
}

// Register registers a typed handler with the existing Job registry.
func (j TypedJob[T]) Register(registry JobRegistry, fn func(context.Context, JobExecution, T) error) error {
	return registry.RegisterJob(j.appName, j.jobName, j.Handler(fn))
}

// Enqueue marshals payload as JSON and enqueues it using the existing JobData API.
func (j TypedJob[T]) Enqueue(ctx context.Context, jobs JobData, payload T, options EnqueueOptions) (JobExecution, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return JobExecution{}, fmt.Errorf("encode job payload: %w", err)
	}
	return jobs.Enqueue(ctx, j.appName, j.jobName, raw, options)
}
