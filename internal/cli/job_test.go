package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hapyco/dygo/internal/jobs"
	jobstore "github.com/hapyco/dygo/internal/jobs/store"
	"github.com/hapyco/dygo/internal/queues"
	"github.com/hapyco/dygo/internal/secrets"
)

func TestJobExecutionRunCommandEnqueuesExecution(t *testing.T) {
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

	fake := &fakeJobExecutionStore{
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
	withJobExecutionStore(t, fake)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"job", "execution", "run", "sales/send-email", "--payload", `{"email":"hi@example.com"}`, "--idempotency-key", "test-1"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run(job execution run) error = %v, want nil", err)
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
		t.Fatalf("job execution run stdout = %q, want queued output", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("job execution run stderr = %q, want empty", stderr.String())
	}
}

func TestJobExecRunAliasEnqueuesExecution(t *testing.T) {
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

	fake := &fakeJobExecutionStore{
		execution: jobstore.Execution{
			ID:      42,
			Name:    "manual-test",
			AppName: "sales",
			JobName: "send-email",
			Queue:   "default",
			Status:  jobs.StatusQueued,
		},
	}
	withJobExecutionStore(t, fake)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"job", "exec", "run", "sales/send-email"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run(job exec run) error = %v, want nil", err)
	}
	if fake.appName != "sales" || fake.jobName != "send-email" {
		t.Fatalf("job target = %s/%s, want sales/send-email", fake.appName, fake.jobName)
	}
	if string(fake.payload) != `{}` {
		t.Fatalf("payload = %s, want default JSON object", fake.payload)
	}
	if stdout.String() != "job execution queued: sales/send-email id=42 name=manual-test queue=default status=queued (development)\n" {
		t.Fatalf("job exec run stdout = %q, want queued output", stdout.String())
	}
}

func TestJobExecutionRunCommandRejectsInvalidPayloadBeforeConnecting(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIConfig(t, root)
	t.Chdir(root)

	fake := &fakeJobExecutionStore{}
	withJobExecutionStore(t, fake)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"job", "execution", "run", "sales/send-email", "--payload", "{"}, strings.NewReader(""), &stdout, &stderr)
	if err == nil {
		t.Fatal("Run(job execution run invalid payload) error = nil, want validation error")
	}
	if !strings.Contains(err.Error(), "job payload must be valid JSON") {
		t.Fatalf("Run(job execution run invalid payload) error = %q, want payload validation", err.Error())
	}
	if fake.opened != 0 {
		t.Fatalf("opened = %d, want no database connection", fake.opened)
	}
}

func TestJobExecutionRunCommandWrapsMissingRegisteredJob(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIConfig(t, root)
	writeCLIDatabaseSecret(t, root, secrets.EnvironmentDevelopment, "postgres://user:secret-password@localhost:5432/dygo")
	t.Chdir(root)

	fake := &fakeJobExecutionStore{enqueueErr: fmt.Errorf("job sales/send-email is not registered")}
	withJobExecutionStore(t, fake)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"job", "execution", "run", "sales/send-email"}, strings.NewReader(""), &stdout, &stderr)
	if err == nil {
		t.Fatal("Run(job execution run missing job) error = nil, want registration guidance")
	}
	if !strings.Contains(err.Error(), "run dygo db migrate") {
		t.Fatalf("Run(job execution run missing job) error = %q, want migrate guidance", err.Error())
	}
}

func TestJobExecutionListCommandPrintsEmptyResult(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIConfig(t, root)
	writeCLIDatabaseSecret(t, root, secrets.EnvironmentDevelopment, "postgres://user:secret-password@localhost:5432/dygo")
	t.Chdir(root)

	fake := &fakeJobExecutionStore{}
	withJobExecutionStore(t, fake)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"job", "execution", "list"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run(job execution list) error = %v, want nil", err)
	}
	if stdout.String() != "No job executions found (development).\n" {
		t.Fatalf("job execution list stdout = %q, want empty result", stdout.String())
	}
	if fake.listOptions.Limit != 20 {
		t.Fatalf("list limit = %d, want default 20", fake.listOptions.Limit)
	}
	if len(fake.queueConfigs) != 0 {
		t.Fatalf("queue config count = %d, want none for list", len(fake.queueConfigs))
	}
}

func TestJobExecutionListCommandPrintsRecentExecutions(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIConfig(t, root)
	writeCLIDatabaseSecret(t, root, secrets.EnvironmentDevelopment, "postgres://user:secret-password@localhost:5432/dygo")
	t.Chdir(root)

	runAfter := time.Date(2026, 5, 30, 10, 0, 0, 0, time.UTC)
	startedAt := time.Date(2026, 5, 30, 10, 1, 0, 0, time.UTC)
	finishedAt := time.Date(2026, 5, 30, 10, 2, 0, 0, time.UTC)
	fake := &fakeJobExecutionStore{
		listResult: []jobstore.Execution{
			{
				ID:          43,
				Name:        "newest",
				AppName:     "sales",
				JobName:     "send-email",
				Queue:       "default",
				Status:      jobs.StatusSucceeded,
				Attempts:    1,
				MaxAttempts: 3,
				RunAfter:    runAfter,
				StartedAt:   &startedAt,
				FinishedAt:  &finishedAt,
			},
			{
				ID:          42,
				Name:        "older",
				AppName:     "sales",
				JobName:     "sync-report",
				Queue:       "reports",
				Status:      jobs.StatusQueued,
				Attempts:    0,
				MaxAttempts: 1,
				RunAfter:    runAfter.Add(-time.Hour),
			},
		},
	}
	withJobExecutionStore(t, fake)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"job", "exec", "list", "--limit", "50"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run(job exec list) error = %v, want nil", err)
	}
	if fake.listOptions.Limit != 50 {
		t.Fatalf("list limit = %d, want 50", fake.listOptions.Limit)
	}
	output := stdout.String()
	for _, want := range []string{
		"ID  NAME    JOB                STATUS     QUEUE    ATTEMPTS  RUN_AFTER             STARTED_AT            FINISHED_AT",
		"43  newest  sales/send-email   succeeded  default  1/3       2026-05-30T10:00:00Z  2026-05-30T10:01:00Z  2026-05-30T10:02:00Z",
		"42  older   sales/sync-report  queued     reports  0/1       2026-05-30T09:00:00Z  -                     -",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("job execution list stdout = %q, want substring %q", output, want)
		}
	}
}

func TestJobExecutionShowCommandPrintsDetails(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIConfig(t, root)
	writeCLIDatabaseSecret(t, root, secrets.EnvironmentDevelopment, "postgres://user:secret-password@localhost:5432/dygo")
	t.Chdir(root)

	runAfter := time.Date(2026, 5, 30, 10, 0, 0, 0, time.UTC)
	startedAt := time.Date(2026, 5, 30, 10, 1, 0, 0, time.UTC)
	finishedAt := time.Date(2026, 5, 30, 10, 2, 0, 0, time.UTC)
	lockedUntil := time.Date(2026, 5, 30, 10, 5, 0, 0, time.UTC)
	fake := &fakeJobExecutionStore{
		getResult: jobstore.Execution{
			ID:             42,
			Name:           "manual-test",
			AppName:        "sales",
			JobName:        "send-email",
			Queue:          "default",
			Status:         jobs.StatusFailed,
			Priority:       5,
			Payload:        json.RawMessage(`{"email":"hi@example.com"}`),
			Result:         json.RawMessage(`{"sent":false}`),
			Error:          "smtp failed",
			Attempts:       3,
			MaxAttempts:    3,
			Retry:          &jobs.Retry{Attempts: 3, InitialDelay: "10s", MaxDelay: "5m"},
			RunAfter:       runAfter,
			StartedAt:      &startedAt,
			FinishedAt:     &finishedAt,
			LockedBy:       "worker-1",
			LockedUntil:    &lockedUntil,
			IdempotencyKey: "email:1",
			Timeout:        30 * time.Second,
		},
	}
	withJobExecutionStore(t, fake)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"job", "execution", "show", "42"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run(job execution show) error = %v, want nil", err)
	}
	if fake.getReference != "42" {
		t.Fatalf("get reference = %q, want 42", fake.getReference)
	}
	output := stdout.String()
	for _, want := range []string{
		"id: 42",
		"name: manual-test",
		"job: sales/send-email",
		"status: failed",
		"attempts: 3/3",
		"run-after: 2026-05-30T10:00:00Z",
		"started-at: 2026-05-30T10:01:00Z",
		"finished-at: 2026-05-30T10:02:00Z",
		"locked-by: worker-1",
		"locked-until: 2026-05-30T10:05:00Z",
		"idempotency-key: email:1",
		"timeout: 30s",
		"retry: attempts=3 initial-delay=10s max-delay=5m",
		`payload: {"email":"hi@example.com"}`,
		`result: {"sent":false}`,
		"error: smtp failed",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("job execution show stdout = %q, want substring %q", output, want)
		}
	}
}

func TestJobExecutionShowCommandAcceptsExecutionName(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIConfig(t, root)
	writeCLIDatabaseSecret(t, root, secrets.EnvironmentDevelopment, "postgres://user:secret-password@localhost:5432/dygo")
	t.Chdir(root)

	fake := &fakeJobExecutionStore{getResult: jobstore.Execution{ID: 42, Name: "manual-test", AppName: "sales", JobName: "send-email", Status: jobs.StatusQueued, Queue: "default", MaxAttempts: 1}}
	withJobExecutionStore(t, fake)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"job", "exec", "show", "manual-test"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run(job exec show name) error = %v, want nil", err)
	}
	if fake.getReference != "manual-test" {
		t.Fatalf("get reference = %q, want manual-test", fake.getReference)
	}
}

func TestJobExecutionCancelCommandCancelsQueuedExecution(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIConfig(t, root)
	writeCLIDatabaseSecret(t, root, secrets.EnvironmentDevelopment, "postgres://user:secret-password@localhost:5432/dygo")
	t.Chdir(root)

	fake := &fakeJobExecutionStore{
		cancelResult: jobstore.Execution{ID: 42, Name: "manual-test", Status: jobs.StatusCancelled},
	}
	withJobExecutionStore(t, fake)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"job", "execution", "cancel", "42"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run(job execution cancel) error = %v, want nil", err)
	}
	if fake.cancelReference != "42" {
		t.Fatalf("cancel reference = %q, want 42", fake.cancelReference)
	}
	if stdout.String() != "job execution cancelled: id=42 name=manual-test status=cancelled (development)\n" {
		t.Fatalf("job execution cancel stdout = %q, want cancelled output", stdout.String())
	}
}

func TestJobExecutionCancelCommandRejectsNonQueuedExecution(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIConfig(t, root)
	writeCLIDatabaseSecret(t, root, secrets.EnvironmentDevelopment, "postgres://user:secret-password@localhost:5432/dygo")
	t.Chdir(root)

	fake := &fakeJobExecutionStore{cancelErr: fmt.Errorf(`job execution "42" is running; only queued executions can be cancelled`)}
	withJobExecutionStore(t, fake)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"job", "exec", "cancel", "42"}, strings.NewReader(""), &stdout, &stderr)
	if err == nil {
		t.Fatal("Run(job exec cancel running) error = nil, want queued-only failure")
	}
	if !strings.Contains(err.Error(), "only queued executions can be cancelled") {
		t.Fatalf("Run(job exec cancel running) error = %q, want queued-only failure", err.Error())
	}
}

func TestJobExecutionRetryCommandRequiresIdempotencyKey(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIConfig(t, root)
	writeCLIDatabaseSecret(t, root, secrets.EnvironmentDevelopment, "postgres://user:secret-password@localhost:5432/dygo")
	t.Chdir(root)

	fake := &fakeJobExecutionStore{}
	withJobExecutionStore(t, fake)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"job", "execution", "retry", "42"}, strings.NewReader(""), &stdout, &stderr)
	if err == nil {
		t.Fatal("Run(job execution retry without key) error = nil, want required key failure")
	}
	if !strings.Contains(err.Error(), "--idempotency-key is required") {
		t.Fatalf("Run(job execution retry without key) error = %q, want required key failure", err.Error())
	}
	if fake.opened != 0 {
		t.Fatalf("opened = %d, want no database connection", fake.opened)
	}
}

func TestJobExecutionRetryCommandRejectsNonFailedExecution(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIConfig(t, root)
	writeCLIDatabaseSecret(t, root, secrets.EnvironmentDevelopment, "postgres://user:secret-password@localhost:5432/dygo")
	t.Chdir(root)

	fake := &fakeJobExecutionStore{retryErr: fmt.Errorf(`job execution "42" is succeeded; only failed executions can be retried`)}
	withJobExecutionStore(t, fake)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"job", "exec", "retry", "42", "--idempotency-key", "manual-retry:42"}, strings.NewReader(""), &stdout, &stderr)
	if err == nil {
		t.Fatal("Run(job exec retry succeeded) error = nil, want failed-only error")
	}
	if !strings.Contains(err.Error(), "only failed executions can be retried") {
		t.Fatalf("Run(job exec retry succeeded) error = %q, want failed-only error", err.Error())
	}
}

func TestJobExecutionRetryCommandQueuesNewExecution(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIConfig(t, root)
	writeCLIDatabaseSecret(t, root, secrets.EnvironmentDevelopment, "postgres://user:secret-password@localhost:5432/dygo")
	t.Chdir(root)

	fake := &fakeJobExecutionStore{
		retryResult: jobstore.Execution{
			ID:      43,
			Name:    "manual-retry",
			AppName: "sales",
			JobName: "send-email",
			Queue:   "default",
			Status:  jobs.StatusQueued,
		},
	}
	withJobExecutionStore(t, fake)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"job", "execution", "retry", "42", "--idempotency-key", "manual-retry:42"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run(job execution retry) error = %v, want nil", err)
	}
	if fake.retryReference != "42" || fake.retryIdempotencyKey != "manual-retry:42" {
		t.Fatalf("retry call = ref %q key %q, want 42 manual-retry:42", fake.retryReference, fake.retryIdempotencyKey)
	}
	if len(fake.queueConfigs) != 1 {
		t.Fatalf("queue config count = %d, want retry to load queue config", len(fake.queueConfigs))
	}
	if stdout.String() != "job execution queued: sales/send-email id=43 name=manual-retry queue=default status=queued (development)\n" {
		t.Fatalf("job execution retry stdout = %q, want queued output", stdout.String())
	}
}

func withJobExecutionStore(t *testing.T, fake *fakeJobExecutionStore) {
	t.Helper()
	previous := openJobExecutionStore
	openJobExecutionStore = func(_ context.Context, databaseURL string, queueConfig ...queues.Config) (jobExecutionStore, func(), error) {
		fake.opened++
		fake.databaseURL = databaseURL
		fake.queueConfigs = queueConfig
		return fake, func() {
			fake.closed++
		}, nil
	}
	t.Cleanup(func() {
		openJobExecutionStore = previous
	})
}

type fakeJobExecutionStore struct {
	opened       int
	closed       int
	databaseURL  string
	queueConfigs []queues.Config

	appName string
	jobName string
	payload json.RawMessage
	options jobstore.EnqueueOptions

	listOptions jobstore.ListOptions
	listResult  []jobstore.Execution
	listErr     error

	getReference string
	getResult    jobstore.Execution
	getErr       error

	cancelReference string
	cancelResult    jobstore.Execution
	cancelErr       error

	retryReference      string
	retryIdempotencyKey string
	retryResult         jobstore.Execution
	retryErr            error

	execution  jobstore.Execution
	enqueueErr error
}

func (f *fakeJobExecutionStore) Enqueue(_ context.Context, appName string, jobName string, payload json.RawMessage, options jobstore.EnqueueOptions) (jobstore.Execution, error) {
	f.appName = appName
	f.jobName = jobName
	f.payload = payload
	f.options = options
	if f.enqueueErr != nil {
		return jobstore.Execution{}, f.enqueueErr
	}
	return f.execution, nil
}

func (f *fakeJobExecutionStore) List(_ context.Context, options jobstore.ListOptions) ([]jobstore.Execution, error) {
	f.listOptions = options
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.listResult, nil
}

func (f *fakeJobExecutionStore) Get(_ context.Context, reference string) (jobstore.Execution, error) {
	f.getReference = reference
	if f.getErr != nil {
		return jobstore.Execution{}, f.getErr
	}
	return f.getResult, nil
}

func (f *fakeJobExecutionStore) CancelQueued(_ context.Context, reference string, _ time.Time) (jobstore.Execution, error) {
	f.cancelReference = reference
	if f.cancelErr != nil {
		return jobstore.Execution{}, f.cancelErr
	}
	return f.cancelResult, nil
}

func (f *fakeJobExecutionStore) Retry(_ context.Context, reference string, idempotencyKey string) (jobstore.Execution, error) {
	f.retryReference = reference
	f.retryIdempotencyKey = idempotencyKey
	if f.retryErr != nil {
		return jobstore.Execution{}, f.retryErr
	}
	return f.retryResult, nil
}
