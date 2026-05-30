package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
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

func TestWorkerRunOnceRecordsHandlerPanic(t *testing.T) {
	now := time.Date(2026, 5, 30, 10, 0, 0, 0, time.UTC)
	store := &fakeStore{
		claimed: []jobstore.Execution{{
			ID:       47,
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
				panic("nil template")
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
	if len(store.failures) != 1 || store.failures[0].id != 47 || store.failures[0].message != "panic: nil template" {
		t.Fatalf("failures = %#v, want panic recorded as handler failure", store.failures)
	}
}

func TestWorkerRunContinuousNotificationWakesBeforePoll(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	firstClaimed := make(chan struct{}, 1)
	store := &fakeStore{
		claimBatches: [][]jobstore.Execution{
			nil,
			{{
				ID:          45,
				AppName:     "crm",
				JobName:     "send-email",
				Queue:       "default",
				Attempts:    1,
				MaxAttempts: 1,
				Timeout:     time.Minute,
			}},
		},
		claimSignal: firstClaimed,
	}
	listener := newFakeListener()
	registry, err := NewRegistry([]sdk.JobRegistrar{
		func(registry sdk.JobRegistry) error {
			return registry.RegisterJob("crm", "send-email", func(context.Context, sdk.JobExecution) error {
				cancel()
				return nil
			})
		},
	})
	if err != nil {
		t.Fatalf("NewRegistry() error = %v, want nil", err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := (Worker{Store: store, Registry: registry}).Run(ctx, Options{
			Queues:               []Queue{{Name: "default", Concurrency: 1}},
			WorkerID:             "test-worker",
			PollInterval:         time.Hour,
			NotificationListener: listener,
		})
		done <- err
	}()

	select {
	case <-firstClaimed:
	case <-time.After(time.Second):
		t.Fatal("worker did not make initial claim")
	}
	listener.Notify("default")

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() error = %v, want nil", err)
		}
	case <-time.After(time.Second):
		t.Fatal("worker did not wake from notification")
	}
	if len(store.completed) != 1 || store.completed[0] != 45 {
		t.Fatalf("completed = %#v, want execution 45", store.completed)
	}
}

func TestWorkerRunContinuousUsesNextRunAfterBeforePoll(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	now := time.Now().UTC()
	store := &fakeStore{
		claimBatches: [][]jobstore.Execution{
			nil,
			{{
				ID:          46,
				AppName:     "crm",
				JobName:     "send-email",
				Queue:       "default",
				Attempts:    1,
				MaxAttempts: 1,
				Timeout:     time.Minute,
			}},
		},
		nextRunAfter: ptrTime(now.Add(20 * time.Millisecond)),
	}
	registry, err := NewRegistry([]sdk.JobRegistrar{
		func(registry sdk.JobRegistry) error {
			return registry.RegisterJob("crm", "send-email", func(context.Context, sdk.JobExecution) error {
				cancel()
				return nil
			})
		},
	})
	if err != nil {
		t.Fatalf("NewRegistry() error = %v, want nil", err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := (Worker{Store: store, Registry: registry}).Run(ctx, Options{
			Queues:       []Queue{{Name: "default", Concurrency: 1}},
			WorkerID:     "test-worker",
			PollInterval: time.Hour,
			Now:          func() time.Time { return time.Now().UTC() },
		})
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() error = %v, want nil", err)
		}
	case <-time.After(time.Second):
		t.Fatal("worker did not wake from next run_after")
	}
	if len(store.completed) != 1 || store.completed[0] != 46 {
		t.Fatalf("completed = %#v, want execution 46", store.completed)
	}
}

func TestWorkerRunContinuousExpiresActiveClaimAfterShutdownTimeout(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{})
	release := make(chan struct{})
	handlerDone := make(chan struct{})
	released := false
	defer func() {
		if !released {
			close(release)
		}
	}()
	store := &fakeStore{
		claimed: []jobstore.Execution{{
			ID:       48,
			AppName:  "crm",
			JobName:  "slow-import",
			Queue:    "default",
			Attempts: 1,
			Timeout:  time.Hour,
		}},
	}
	registry, err := NewRegistry([]sdk.JobRegistrar{
		func(registry sdk.JobRegistry) error {
			return registry.RegisterJob("crm", "slow-import", func(context.Context, sdk.JobExecution) error {
				defer close(handlerDone)
				close(started)
				<-release
				return nil
			})
		},
	})
	if err != nil {
		t.Fatalf("NewRegistry() error = %v, want nil", err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := (Worker{Store: store, Registry: registry}).Run(ctx, Options{
			Queues:          []Queue{{Name: "default", Concurrency: 1}},
			WorkerID:        "test-worker",
			PollInterval:    time.Hour,
			ShutdownTimeout: 20 * time.Millisecond,
		})
		done <- err
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("handler did not start")
	}
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() error = %v, want nil", err)
		}
	case <-time.After(time.Second):
		t.Fatal("worker did not finish after shutdown timeout")
	}
	if len(store.expired) != 1 || store.expired[0] != 48 {
		t.Fatalf("expired = %#v, want execution 48 claim shortened", store.expired)
	}
	close(release)
	released = true
	select {
	case <-handlerDone:
	case <-time.After(time.Second):
		t.Fatal("handler did not finish after release")
	}
}

type fakeStore struct {
	mu            sync.Mutex
	claimed       []jobstore.Execution
	claimBatches  [][]jobstore.Execution
	claimSignal   chan struct{}
	nextRunAfter  *time.Time
	completed     []int64
	failures      []fakeFailure
	finalFailures []fakeFailure
	expired       []int64
}

type fakeFailure struct {
	id      int64
	message string
}

func (s *fakeStore) RecoverExpired(context.Context, time.Time) (int, error) {
	return 0, nil
}

func (s *fakeStore) Claim(_ context.Context, _ []string, _ int, _ string, _ time.Time) ([]jobstore.Execution, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.claimSignal != nil {
		select {
		case s.claimSignal <- struct{}{}:
		default:
		}
	}
	if len(s.claimBatches) > 0 {
		claimed := s.claimBatches[0]
		s.claimBatches = s.claimBatches[1:]
		return claimed, nil
	}
	claimed := s.claimed
	s.claimed = nil
	return claimed, nil
}

func (s *fakeStore) NextRunAfter(context.Context, []string, time.Time) (*time.Time, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.nextRunAfter, nil
}

func (s *fakeStore) Complete(_ context.Context, execution jobstore.Execution, _ json.RawMessage, _ time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.completed = append(s.completed, execution.ID)
	return nil
}

func (s *fakeStore) Fail(_ context.Context, execution jobstore.Execution, message string, _ time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failures = append(s.failures, fakeFailure{id: execution.ID, message: message})
	return nil
}

func (s *fakeStore) FailFinal(_ context.Context, execution jobstore.Execution, message string, _ time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.finalFailures = append(s.finalFailures, fakeFailure{id: execution.ID, message: message})
	return nil
}

func (s *fakeStore) ExpireClaim(_ context.Context, execution jobstore.Execution, _ time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expired = append(s.expired, execution.ID)
	return nil
}

func (s *fakeStore) Enqueue(context.Context, string, string, json.RawMessage, jobstore.EnqueueOptions) (jobstore.Execution, error) {
	return jobstore.Execution{}, nil
}

func ptrTime(value time.Time) *time.Time {
	return &value
}

type fakeListener struct {
	notifications chan string
	closeOnce     sync.Once
}

func newFakeListener() *fakeListener {
	return &fakeListener{notifications: make(chan string, 1)}
}

func (l *fakeListener) Notify(queue string) {
	l.notifications <- queue
}

func (l *fakeListener) Wait(ctx context.Context) (string, error) {
	select {
	case queue := <-l.notifications:
		return queue, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func (l *fakeListener) Close() {
	l.closeOnce.Do(func() {
		close(l.notifications)
	})
}
