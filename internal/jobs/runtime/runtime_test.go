package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	jobstore "github.com/hapyco/dygo/internal/jobs/store"
	"github.com/hapyco/dygo/pkg/sdk"
)

func TestWorkerRunOnceCompletesRegisteredJob(t *testing.T) {
	now := time.Date(2026, 5, 30, 10, 0, 0, 0, time.UTC)
	store := &fakeStore{
		claimed: []jobstore.Execution{{
			ID:          42,
			AppName:     "crm",
			JobName:     "send-email",
			Queue:       "default",
			Attempts:    1,
			MaxAttempts: 3,
			Timeout:     time.Minute,
			Payload:     json.RawMessage(`{"email":"a@example.com"}`),
		}},
	}
	registry, err := NewRegistry([]sdk.JobRegistrar{
		func(registry sdk.JobRegistry) error {
			return registry.RegisterJob("crm", "send-email", func(_ context.Context, execution sdk.JobExecution) error {
				if execution.ID != 42 || execution.AppName != "crm" || execution.JobName != "send-email" || execution.Attempt != 1 {
					t.Fatalf("job execution = %+v, want claimed execution context", execution)
				}
				if string(execution.Payload) != `{"email":"a@example.com"}` {
					t.Fatalf("payload = %s, want JSON payload", execution.Payload)
				}
				if execution.Records == nil || execution.Jobs == nil {
					t.Fatalf("execution services = Records:%v Jobs:%v, want both", execution.Records, execution.Jobs)
				}
				return nil
			})
		},
	})
	if err != nil {
		t.Fatalf("NewRegistry() error = %v, want nil", err)
	}

	result, err := (Worker{Store: store, Registry: registry}).Run(context.Background(), Options{
		Queues:   []Queue{{Name: "default", Concurrency: 1}},
		WorkerID: "test-worker",
		Once:     true,
		Now:      func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if result.Claimed != 1 || result.Succeeded != 1 || result.Failed != 0 {
		t.Fatalf("result = %+v, want one succeeded execution", result)
	}
	if len(store.completed) != 1 || store.completed[0] != 42 {
		t.Fatalf("completed = %#v, want execution 42", store.completed)
	}
}

func TestWorkerRunOnceFailsMissingHandlerWithoutRetry(t *testing.T) {
	now := time.Date(2026, 5, 30, 10, 0, 0, 0, time.UTC)
	store := &fakeStore{
		claimed: []jobstore.Execution{{
			ID:       43,
			AppName:  "crm",
			JobName:  "missing",
			Queue:    "default",
			Attempts: 1,
			Timeout:  time.Minute,
		}},
	}
	registry, err := NewRegistry(nil)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v, want nil", err)
	}

	result, err := (Worker{Store: store, Registry: registry}).Run(context.Background(), Options{
		Queues:   []Queue{{Name: "default", Concurrency: 1}},
		WorkerID: "test-worker",
		Once:     true,
		Now:      func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if result.Claimed != 1 || result.Failed != 1 {
		t.Fatalf("result = %+v, want one failed execution", result)
	}
	if len(store.finalFailures) != 1 || store.finalFailures[0].id != 43 || !strings.Contains(store.finalFailures[0].message, "missing job handler") {
		t.Fatalf("final failures = %#v, want missing handler failure", store.finalFailures)
	}
	if len(store.failures) != 0 {
		t.Fatalf("failures = %#v, want no retryable failure", store.failures)
	}
}

func TestWorkerRunOnceRecordsHandlerFailure(t *testing.T) {
	now := time.Date(2026, 5, 30, 10, 0, 0, 0, time.UTC)
	store := &fakeStore{
		claimed: []jobstore.Execution{{
			ID:       44,
			AppName:  "crm",
			JobName:  "send-email",
			Queue:    "default",
			Attempts: 1,
			Timeout:  time.Minute,
		}},
	}
	registry, err := NewRegistry([]sdk.JobRegistrar{
		func(registry sdk.JobRegistry) error {
			return registry.RegisterJob("crm", "send-email", func(context.Context, sdk.JobExecution) error {
				return errors.New("smtp unavailable")
			})
		},
	})
	if err != nil {
		t.Fatalf("NewRegistry() error = %v, want nil", err)
	}

	result, err := (Worker{Store: store, Registry: registry}).Run(context.Background(), Options{
		Queues:   []Queue{{Name: "default", Concurrency: 1}},
		WorkerID: "test-worker",
		Once:     true,
		Now:      func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if result.Claimed != 1 || result.Failed != 1 {
		t.Fatalf("result = %+v, want one failed execution", result)
	}
	if len(store.failures) != 1 || store.failures[0].id != 44 || store.failures[0].message != "smtp unavailable" {
		t.Fatalf("failures = %#v, want handler error", store.failures)
	}
}

type fakeStore struct {
	claimed       []jobstore.Execution
	completed     []int64
	failures      []fakeFailure
	finalFailures []fakeFailure
}

type fakeFailure struct {
	id      int64
	message string
}

func (s *fakeStore) RecoverExpired(context.Context, time.Time) (int, error) {
	return 0, nil
}

func (s *fakeStore) Claim(_ context.Context, _ []string, _ int, _ string, _ time.Time) ([]jobstore.Execution, error) {
	claimed := s.claimed
	s.claimed = nil
	return claimed, nil
}

func (s *fakeStore) Complete(_ context.Context, id int64, _ json.RawMessage, _ time.Time) error {
	s.completed = append(s.completed, id)
	return nil
}

func (s *fakeStore) Fail(_ context.Context, id int64, message string, _ time.Time) error {
	s.failures = append(s.failures, fakeFailure{id: id, message: message})
	return nil
}

func (s *fakeStore) FailFinal(_ context.Context, id int64, message string, _ time.Time) error {
	s.finalFailures = append(s.finalFailures, fakeFailure{id: id, message: message})
	return nil
}

func (s *fakeStore) Enqueue(context.Context, string, string, json.RawMessage, jobstore.EnqueueOptions) (jobstore.Execution, error) {
	return jobstore.Execution{}, nil
}
