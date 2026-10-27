package dygo_test

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/hapyco/dygo/pkg/dygo"
)

type welcomePayload struct {
	ContactID string `json:"contact_id"`
}

type jobRegistry struct {
	app, job string
	fn       dygo.JobFunc
}

func (r *jobRegistry) RegisterJob(app, job string, fn dygo.JobFunc) error {
	r.app, r.job, r.fn = app, job, fn
	return nil
}

type jobData struct {
	app, job string
	payload  json.RawMessage
	options  dygo.EnqueueOptions
	called   bool
}

func (d *jobData) Enqueue(_ context.Context, app, job string, payload json.RawMessage, options dygo.EnqueueOptions) (dygo.JobExecution, error) {
	d.app, d.job, d.payload, d.options, d.called = app, job, payload, options, true
	return dygo.JobExecution{ID: 42, Payload: payload}, nil
}

func (*jobData) CancelQueued(context.Context, string) (dygo.JobExecution, error) {
	panic("unexpected call")
}

func (*jobData) Retry(context.Context, string, string) (dygo.JobExecution, error) {
	panic("unexpected call")
}

func TestTypedJobEnqueueAndHandle(t *testing.T) {
	job := dygo.NewTypedJob[welcomePayload]("crm", "welcome")
	store := &jobData{}
	options := dygo.EnqueueOptions{IdempotencyKey: "contact:42", Priority: 2}
	execution, err := job.Enqueue(context.Background(), store, welcomePayload{ContactID: "42"}, options)
	if err != nil {
		t.Fatal(err)
	}
	if store.app != "crm" || store.job != "welcome" || store.options != options || string(store.payload) != `{"contact_id":"42"}` {
		t.Fatalf("unexpected enqueue: %#v", store)
	}
	registry := &jobRegistry{}
	called := false
	if err := job.Register(registry, func(_ context.Context, got dygo.JobExecution, payload welcomePayload) error {
		called = true
		if got.ID != 42 || payload.ContactID != "42" {
			t.Fatalf("execution = %#v, payload = %#v", got, payload)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if registry.app != "crm" || registry.job != "welcome" {
		t.Fatalf("registration = %s/%s", registry.app, registry.job)
	}
	if err := registry.fn(context.Background(), execution); err != nil || !called {
		t.Fatalf("handler called = %v, error = %v", called, err)
	}
}

func TestTypedJobPayloadErrors(t *testing.T) {
	store := &jobData{}
	job := dygo.NewTypedJob[float64]("app", "job")
	if _, err := job.Enqueue(context.Background(), store, math.NaN(), dygo.EnqueueOptions{}); err == nil || !strings.Contains(err.Error(), "encode job payload") {
		t.Fatalf("encoding error = %v", err)
	}
	if store.called {
		t.Fatal("enqueue called after encoding failed")
	}
	called := false
	handler := job.Handler(func(context.Context, dygo.JobExecution, float64) error {
		called = true
		return nil
	})
	if err := handler(context.Background(), dygo.JobExecution{Payload: json.RawMessage(`"not a number"`)}); err == nil || !strings.Contains(err.Error(), "decode job payload") {
		t.Fatalf("decoding error = %v", err)
	}
	if called {
		t.Fatal("handler called after decoding failed")
	}
}

func TestTypedJobAbsentPayloadAndHandlerError(t *testing.T) {
	job := dygo.NewTypedJob[*welcomePayload]("app", "job")
	want := errors.New("handler failure")
	handler := job.Handler(func(_ context.Context, _ dygo.JobExecution, payload *welcomePayload) error {
		if payload != nil {
			t.Fatalf("absent payload = %#v, want nil", payload)
		}
		return want
	})
	if err := handler(context.Background(), dygo.JobExecution{}); !errors.Is(err, want) {
		t.Fatalf("handler error = %v", err)
	}
	if job.Handler(nil) != nil {
		t.Fatal("nil typed handler must remain nil for registry validation")
	}
}
