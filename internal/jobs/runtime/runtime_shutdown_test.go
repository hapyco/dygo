package runtime

import (
	"context"
	"errors"
	"testing"
	"time"

	jobstore "github.com/hapyco/dygo/internal/jobs/store"
	"github.com/hapyco/dygo/pkg/dygo"
)

func TestWorkerFailureStopsSiblingClaimsBeforeDrain(t *testing.T) {
	for _, slowExpiry := range []bool{false, true} {
		name := "handler-drain"
		if slowExpiry {
			name = "handler-drain-and-slow-expiry"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			store := &siblingShutdownStore{
				fakeStore: &fakeStore{}, failure: errors.New("failed queue database error"), slowExpiry: slowExpiry,
				failedStarted: make(chan struct{}), failedCancelled: make(chan struct{}), siblingWaiting: make(chan struct{}),
				siblingClaimed: make(chan struct{}, 1), siblingStarted: make(chan struct{}, 1), expiryStarted: make(chan struct{}),
			}
			release := make(chan struct{})
			handlerFinished := make(chan struct{})
			defer func() {
				close(release)
				select {
				case <-handlerFinished:
				case <-time.After(time.Second):
					t.Error("failed queue handler did not finish after release")
				}
			}()
			registry, err := NewRegistry([]dygo.JobRegistrar{func(registry dygo.JobRegistry) error {
				if err := registry.RegisterJob("audit", "blocked", func(ctx context.Context, _ dygo.JobExecution) error {
					defer close(handlerFinished)
					close(store.failedStarted)
					<-ctx.Done()
					close(store.failedCancelled)
					<-release
					return ctx.Err()
				}); err != nil {
					return err
				}
				return registry.RegisterJob("audit", "sibling", func(ctx context.Context, _ dygo.JobExecution) error {
					store.siblingStarted <- struct{}{}
					<-ctx.Done()
					return ctx.Err()
				})
			}})
			if err != nil {
				t.Fatal(err)
			}
			_, err = (Worker{Store: store, Registry: registry}).Run(ctx, Options{
				Queues:   []Queue{{Name: "failed", Concurrency: 1}, {Name: "sibling", Concurrency: 1}},
				WorkerID: "shutdown-test", PollInterval: time.Hour, ShutdownTimeout: 20 * time.Millisecond,
			})
			// The sibling has no work to drain and returns context.Canceled before
			// the failed queue finishes cleanup. It must not replace this error.
			if !errors.Is(err, store.failure) {
				t.Fatalf("Run() error = %v, want originating database failure", err)
			}
			select {
			case <-store.siblingClaimed:
				t.Error("sibling claimed new work after the failed queue began draining")
			default:
			}
			select {
			case <-store.siblingStarted:
				t.Error("sibling started a new handler after the failed queue began draining")
			default:
			}
			select {
			case <-store.expiryStarted:
			default:
				t.Error("failed queue did not attempt bounded expiry of its active claim")
			}
		})
	}
}

type siblingShutdownStore struct {
	*fakeStore
	failure                                        error
	slowExpiry                                     bool
	failedClaims, siblingClaims                    int
	failedStarted, failedCancelled, siblingWaiting chan struct{}
	siblingClaimed, siblingStarted, expiryStarted  chan struct{}
}

func (s *siblingShutdownStore) Claim(ctx context.Context, queues []string, _ int, _ string, _ time.Time) ([]jobstore.Execution, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if queues[0] == "failed" {
		s.failedClaims++
		if s.failedClaims == 1 {
			return []jobstore.Execution{{ID: 101, AppName: "audit", JobName: "blocked", Queue: "failed", Attempts: 1, Timeout: time.Hour}}, nil
		}
		return nil, nil
	}
	s.siblingClaims++
	if s.siblingClaims == 2 {
		// The first batch is empty; its wake-up waits for failed-queue cleanup.
		// This is a new Claim invocation, not a request already in flight.
		s.siblingClaimed <- struct{}{}
		return []jobstore.Execution{{ID: 102, AppName: "audit", JobName: "sibling", Queue: "sibling", Attempts: 1, Timeout: time.Hour}}, nil
	}
	return nil, nil
}

func (s *siblingShutdownStore) NextRunAfter(ctx context.Context, queues []string, _ time.Time) (*time.Time, error) {
	if queues[0] == "failed" {
		<-s.failedStarted
		<-s.siblingWaiting
		return nil, s.failure
	}
	s.mu.Lock()
	claims := s.siblingClaims
	s.mu.Unlock()
	if claims == 1 {
		close(s.siblingWaiting)
		select {
		case <-s.failedCancelled:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		due := time.Time{}
		return &due, nil
	}
	return nil, nil
}

func (s *siblingShutdownStore) ExpireClaim(ctx context.Context, execution jobstore.Execution, now time.Time) error {
	if execution.ID == 101 {
		close(s.expiryStarted)
		if s.slowExpiry {
			<-ctx.Done()
			return ctx.Err()
		}
	}
	return s.fakeStore.ExpireClaim(ctx, execution, now)
}
