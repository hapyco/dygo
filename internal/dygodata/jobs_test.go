package dygodata

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	jobstore "github.com/hapyco/dygo/internal/jobs/store"
	"github.com/hapyco/dygo/pkg/dygo"
)

func TestJobDataEnqueueMapsSDKOptions(t *testing.T) {
	runAfter := time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)
	store := &fakeJobEnqueuer{
		execution: jobstore.Execution{
			ID:       42,
			AppName:  "crm",
			JobName:  "send-email",
			Queue:    "default",
			Attempts: 0,
			Payload:  json.RawMessage(`{"email":"a@example.com"}`),
		},
	}

	execution, err := NewJobData(store).Enqueue(context.Background(), "crm", "send-email", json.RawMessage(`{"email":"a@example.com"}`), dygo.EnqueueOptions{
		IdempotencyKey: "email:1",
		Priority:       10,
		RunAfter:       runAfter,
	})
	if err != nil {
		t.Fatalf("Enqueue() error = %v, want nil", err)
	}
	if store.appName != "crm" || store.jobName != "send-email" || string(store.payload) != `{"email":"a@example.com"}` {
		t.Fatalf("store call = %s/%s %s, want crm/send-email payload", store.appName, store.jobName, store.payload)
	}
	if store.options.IdempotencyKey != "email:1" || store.options.Priority != 10 || !store.options.RunAfter.Equal(runAfter) {
		t.Fatalf("store options = %+v, want SDK enqueue options", store.options)
	}
	if execution.ID != 42 || execution.AppName != "crm" || execution.JobName != "send-email" || execution.Queue != "default" || string(execution.Payload) != `{"email":"a@example.com"}` || execution.Jobs == nil {
		t.Fatalf("SDK execution = %+v, want mapped execution with Jobs service", execution)
	}
}

type fakeJobEnqueuer struct {
	appName   string
	jobName   string
	payload   json.RawMessage
	options   jobstore.EnqueueOptions
	execution jobstore.Execution
}

func (s *fakeJobEnqueuer) Enqueue(_ context.Context, appName string, jobName string, payload json.RawMessage, options jobstore.EnqueueOptions) (jobstore.Execution, error) {
	s.appName = appName
	s.jobName = jobName
	s.payload = payload
	s.options = options
	return s.execution, nil
}
