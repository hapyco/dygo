package jobs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hapyco/dygo/internal/app/manifest"
	"github.com/hapyco/dygo/internal/queues"
)

func TestCatalogValidateLoadsRegisteredJobs(t *testing.T) {
	root := t.TempDir()
	appDir := filepath.Join(root, "apps", "crm")
	writeJobTestFile(t, filepath.Join(appDir, "jobs", "send-email", "job.yml"), `label: Send Email
queue: email
timeout: 30s
retry:
  attempts: 3
`)

	loaded, err := New([]manifest.LoadedApp{{
		Dir: appDir,
		Manifest: manifest.Manifest{
			Name: "crm",
		},
	}}, queues.Config{Queues: []queues.Queue{
		{Name: "default", Concurrency: 4},
		{Name: "email", Concurrency: 2},
	}}).Validate()
	if err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("loaded jobs count = %d, want 1", len(loaded))
	}
	job := loaded[0].Job
	if job.Name != "send-email" || job.EffectiveQueue() != "email" || job.MaxAttempts() != 3 {
		t.Fatalf("loaded job = %+v, want send-email on email with retries", job)
	}
	retry := job.EffectiveRetry()
	if retry == nil || retry.InitialDelay != "10s" || retry.MaxDelay != "5m" {
		t.Fatalf("EffectiveRetry() = %+v, want default retry delays", retry)
	}
}

func TestCatalogValidateRejectsUnregisteredQueue(t *testing.T) {
	root := t.TempDir()
	appDir := filepath.Join(root, "apps", "crm")
	writeJobTestFile(t, filepath.Join(appDir, "jobs", "send-email", "job.yml"), `label: Send Email
queue: email
timeout: 30s
`)

	_, err := New([]manifest.LoadedApp{{
		Dir: appDir,
		Manifest: manifest.Manifest{
			Name: "crm",
		},
	}}, queues.Default()).Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want unregistered queue error")
	}
	if !strings.Contains(err.Error(), `references unregistered queue "email"`) {
		t.Fatalf("Validate() error = %q, want unregistered queue", err.Error())
	}
}

func TestDecodeRejectsInvalidJobMetadata(t *testing.T) {
	_, err := Decode([]byte(`label: Import
timeout: 0s
retry:
  attempts: 1
  initial-delay: 5m
  max-delay: 1m
`))
	if err == nil {
		t.Fatal("Decode() error = nil, want validation error")
	}
	for _, want := range []string{"timeout must be greater than 0", "retry.attempts must be at least 2", "retry.max-delay must be greater than or equal to retry.initial-delay"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Decode() error = %q, want %q", err.Error(), want)
		}
	}
}

func writeJobTestFile(t *testing.T, path string, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
}
