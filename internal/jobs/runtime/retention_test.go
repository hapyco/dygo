package runtime

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"
)

type retentionStore struct {
	*fakeStore
	queues []string
	cutoff time.Time
	limit  int
	err    error
}

func (s *retentionStore) PruneTerminal(_ context.Context, queues []string, cutoff time.Time, limit int) (int, error) {
	s.queues, s.cutoff, s.limit = queues, cutoff, limit
	return 0, s.err
}

func TestWorkerRetentionIsOptInAndDoesNotStopWorkOnError(t *testing.T) {
	now := time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)
	for _, days := range []int{0, 30} {
		store := &retentionStore{fakeStore: &fakeStore{}, err: errors.New("database unavailable")}
		_, err := (Worker{Store: store, Stderr: io.Discard}).Run(context.Background(), Options{
			Queues: []Queue{{Name: "default", Concurrency: 1}}, Once: true,
			RetentionDays: days, Now: func() time.Time { return now },
		})
		if err != nil {
			t.Fatalf("days=%d: worker stopped: %v", days, err)
		}
		if days == 0 && store.limit != 0 {
			t.Fatalf("disabled retention called store")
		}
		if days > 0 && (len(store.queues) != 1 || store.queues[0] != "default" || !store.cutoff.Equal(now.Add(-30*24*time.Hour)) || store.limit != 500) {
			t.Fatalf("retention args: %+v", store)
		}
		if len(store.calls) == 0 {
			t.Fatal("worker did not continue after cleanup")
		}
	}
}
