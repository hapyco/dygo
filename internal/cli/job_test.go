package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hapyco/dygo/internal/jobs"
	jobstore "github.com/hapyco/dygo/internal/jobs/store"
	"github.com/hapyco/dygo/internal/queues"
	"github.com/hapyco/dygo/internal/secrets"
)

func TestJobRunCommandEnqueuesExecution(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIGoModule(t, root, "example.com/acme")
	writeCLIConfig(t, root)
	writeCLIDatabaseSecret(t, root, secrets.EnvironmentDevelopment, "postgres://user:secret-password@localhost:5432/dygo")
	writeCLIApp(t, filepath.Join(root, "apps", "sales"), "sales")
	writeCLIJob(t, filepath.Join(root, "apps", "sales", "jobs", "send-email", "job.yml"), `
label: Send Email
queue: default
timeout: 30s
`)
	t.Chdir(root)

	fake := &fakeJobRunEnqueuer{
		execution: jobstore.Execution{
			ID:       42,
			Name:     "manual-test",
			AppName:  "sales",
			JobName:  "send-email",
			Queue:    "default",
			Status:   jobs.StatusQueued,
			Priority: 0,
		},
	}
	withJobRunEnqueuer(t, fake)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"job", "run", "sales/send-email", "--payload", `{"email":"hi@example.com"}`, "--idempotency-key", "test-1"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run(job run) error = %v, want nil", err)
	}
	if fake.databaseURL != "postgres://user:secret-password@localhost:5432/dygo" {
		t.Fatalf("databaseURL = %q, want configured secret", fake.databaseURL)
	}
	if fake.closed != 1 {
		t.Fatalf("closed = %d, want 1", fake.closed)
	}
	if fake.appName != "sales" || fake.jobName != "send-email" {
		t.Fatalf("job target = %s/%s, want sales/send-email", fake.appName, fake.jobName)
	}
	if string(fake.payload) != `{"email":"hi@example.com"}` {
		t.Fatalf("payload = %s, want provided JSON", fake.payload)
	}
	if fake.options.IdempotencyKey != "test-1" {
		t.Fatalf("IdempotencyKey = %q, want test-1", fake.options.IdempotencyKey)
	}
	if stdout.String() != "job execution queued: sales/send-email id=42 name=manual-test queue=default status=queued (development)\n" {
		t.Fatalf("job run stdout = %q, want queued output", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("job run stderr = %q, want empty", stderr.String())
	}
}

func TestJobRunCommandRejectsInvalidPayloadBeforeConnecting(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIConfig(t, root)
	t.Chdir(root)

	fake := &fakeJobRunEnqueuer{}
	withJobRunEnqueuer(t, fake)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"job", "run", "sales/send-email", "--payload", "{"}, strings.NewReader(""), &stdout, &stderr)
	if err == nil {
		t.Fatal("Run(job run invalid payload) error = nil, want validation error")
	}
	if !strings.Contains(err.Error(), "job payload must be valid JSON") {
		t.Fatalf("Run(job run invalid payload) error = %q, want payload validation", err.Error())
	}
	if fake.opened != 0 {
		t.Fatalf("opened = %d, want no database connection", fake.opened)
	}
}

func TestJobRunCommandWrapsMissingRegisteredJob(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIConfig(t, root)
	writeCLIDatabaseSecret(t, root, secrets.EnvironmentDevelopment, "postgres://user:secret-password@localhost:5432/dygo")
	t.Chdir(root)

	fake := &fakeJobRunEnqueuer{err: fmt.Errorf("job sales/send-email is not registered")}
	withJobRunEnqueuer(t, fake)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"job", "run", "sales/send-email"}, strings.NewReader(""), &stdout, &stderr)
	if err == nil {
		t.Fatal("Run(job run missing job) error = nil, want registration guidance")
	}
	if !strings.Contains(err.Error(), "run dygo db migrate") {
		t.Fatalf("Run(job run missing job) error = %q, want migrate guidance", err.Error())
	}
}

func withJobRunEnqueuer(t *testing.T, fake *fakeJobRunEnqueuer) {
	t.Helper()
	previous := openJobRunEnqueuer
	openJobRunEnqueuer = func(_ context.Context, databaseURL string, queueConfig queues.Config) (jobRunEnqueuer, func(), error) {
		fake.opened++
		fake.databaseURL = databaseURL
		fake.queueConfig = queueConfig
		return fake, func() {
			fake.closed++
		}, nil
	}
	t.Cleanup(func() {
		openJobRunEnqueuer = previous
	})
}

type fakeJobRunEnqueuer struct {
	opened      int
	closed      int
	databaseURL string
	queueConfig queues.Config

	appName string
	jobName string
	payload json.RawMessage
	options jobstore.EnqueueOptions

	execution jobstore.Execution
	err       error
}

func (f *fakeJobRunEnqueuer) Enqueue(_ context.Context, appName string, jobName string, payload json.RawMessage, options jobstore.EnqueueOptions) (jobstore.Execution, error) {
	f.appName = appName
	f.jobName = jobName
	f.payload = payload
	f.options = options
	if f.err != nil {
		return jobstore.Execution{}, f.err
	}
	return f.execution, nil
}
