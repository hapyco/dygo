package cli

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAppInstallClonesValidatesAndInstallsRepository(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIGoModule(t, root, "example.com/acme")
	writeCLIApp(t, filepath.Join(root, ".dygo", "apps", "core"), "core")
	writeCLIAppWithBody(t, filepath.Join(root, "apps", "crm"), `
name: crm
label: CRM
version: 0.1.0
dependencies: [core]
`)
	repository := writeGitAppRepository(t, `
name: sales
label: Sales
version: 0.1.0
dependencies: [crm]
`)
	if err := os.WriteFile(filepath.Join(repository, "README.md"), []byte("sales\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	commitGitAppRepository(t, repository)
	t.Chdir(root)

	var stdout bytes.Buffer
	if err := Run(context.Background(), []string{"app", "install", repository, "--yes"}, strings.NewReader(""), &stdout, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"app: sales 0.1.0",
		"destination: apps/sales (create)",
		"prepare order: core -> crm -> sales",
		"App installed: sales 0.1.0",
		"Next: build the project runner, then run dygo db migrate.",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout = %q, want %q", stdout.String(), want)
		}
	}
	for _, path := range []string{
		filepath.Join(root, "apps", "sales", "app.yml"),
		filepath.Join(root, "apps", "sales", "README.md"),
		filepath.Join(root, "cmd", "dygo", "main.go"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("installed file %s: %v", path, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "apps", "sales", ".git")); !os.IsNotExist(err) {
		t.Fatalf("installed Git metadata stat = %v, want missing", err)
	}
}

func TestAppInstallDryRunLeavesProjectUnchanged(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIGoModule(t, root, "example.com/acme")
	repository := writeGitAppRepository(t, "name: sales\nlabel: Sales\nversion: 0.1.0\n")
	commitGitAppRepository(t, repository)
	t.Chdir(root)

	var stdout bytes.Buffer
	if err := Run(context.Background(), []string{"app", "install", repository, "--dry-run"}, strings.NewReader(""), &stdout, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "dry-run: no project files or database changes") {
		t.Fatalf("stdout = %q", stdout.String())
	}
	for _, path := range []string{filepath.Join(root, "apps", "sales"), filepath.Join(root, "cmd", "dygo", "main.go")} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("dry-run path %s stat = %v, want missing", path, err)
		}
	}
}

func TestAppInstallRejectsInvalidRepository(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIGoModule(t, root, "example.com/acme")
	repository := writeGitAppRepository(t, "")
	if err := os.WriteFile(filepath.Join(repository, "README.md"), []byte("not an App\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	commitGitAppRepository(t, repository)
	t.Chdir(root)

	err := Run(context.Background(), []string{"app", "install", repository, "--yes"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "validate cloned App") {
		t.Fatalf("error = %v, want cloned App validation error", err)
	}
}

func TestAppInstallRejectsMissingDependencyWithoutWriting(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIGoModule(t, root, "example.com/acme")
	repository := writeGitAppRepository(t, `
name: sales
label: Sales
version: 0.1.0
dependencies: [crm]
`)
	commitGitAppRepository(t, repository)
	t.Chdir(root)

	err := Run(context.Background(), []string{"app", "install", repository, "--yes"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), `depends on unknown app "crm"`) {
		t.Fatalf("error = %v, want missing dependency", err)
	}
	if _, statErr := os.Stat(filepath.Join(root, "apps", "sales")); !os.IsNotExist(statErr) {
		t.Fatalf("App destination stat = %v, want missing", statErr)
	}
}

func TestAppInstallRemovesSourceWhenProjectValidationFails(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIGoModule(t, root, "example.com/acme")
	repository := writeGitAppRepository(t, "name: sales\nlabel: Sales\nversion: 0.1.0\n")
	entityDir := filepath.Join(repository, "entities", "lead")
	if err := os.MkdirAll(entityDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(entityDir, "lead.entity.yml"), []byte("name: [\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	commitGitAppRepository(t, repository)
	t.Chdir(root)

	err := Run(context.Background(), []string{"app", "install", repository, "--yes"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "prepare project runner") {
		t.Fatalf("error = %v, want project validation error", err)
	}
	if _, statErr := os.Stat(filepath.Join(root, "apps", "sales")); !os.IsNotExist(statErr) {
		t.Fatalf("App destination stat = %v, want rollback", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(root, "cmd", "dygo", "main.go")); !os.IsNotExist(statErr) {
		t.Fatalf("runner stat = %v, want missing", statErr)
	}
}

func writeGitAppRepository(t *testing.T, manifest string) string {
	t.Helper()
	repository := t.TempDir()
	if strings.TrimSpace(manifest) != "" {
		if err := os.WriteFile(filepath.Join(repository, "app.yml"), []byte(strings.TrimSpace(manifest)+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runGit(t, repository, "init", "--quiet")
	return repository
}

func commitGitAppRepository(t *testing.T, repository string) {
	t.Helper()
	runGit(t, repository, "add", ".")
	runGit(t, repository, "-c", "user.name=dygo", "-c", "user.email=dygo@example.invalid", "commit", "--quiet", "-m", "initial")
}

func runGit(t *testing.T, repository string, args ...string) {
	t.Helper()
	commandArgs := append([]string{"-C", repository}, args...)
	if output, err := exec.Command("git", commandArgs...).CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, output)
	}
}

func TestAppInstallCancelsRunningCloneAndCleansUp(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIGoModule(t, root, "example.com/acme")
	t.Chdir(root)
	checkoutDir := t.TempDir()
	t.Setenv("TMPDIR", checkoutDir)
	t.Setenv("TMP", checkoutDir)

	requested := make(chan struct{}, 1)
	releaseResponse := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case requested <- struct{}{}:
		default:
		}
		select {
		case <-r.Context().Done():
		case <-releaseResponse:
		}
	}))
	defer server.Close()
	defer close(releaseResponse)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var stdout bytes.Buffer
	stderr := newProgressBuffer()
	finished := make(chan error, 1)
	go func() {
		finished <- Run(ctx, []string{"app", "install", server.URL + "/app.git", "--yes"}, strings.NewReader(""), &stdout, stderr)
	}()
	select {
	case <-requested:
	case err := <-finished:
		t.Fatalf("clone stopped before contacting repository: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("clone did not contact repository")
	}
	stderr.wait(t)
	cancel()
	select {
	case err := <-finished:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("clone error = %v, want cancellation", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("app install ignored command cancellation")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want no plan/result for cancelled clone", stdout.String())
	}
	if got := stderr.String(); got != "Cloning and validating App repository...\n" {
		t.Fatalf("stderr = %q, want bounded plain clone status", got)
	}
	entries, err := os.ReadDir(checkoutDir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("temporary checkouts = %v, error = %v, want cleanup", entries, err)
	}
	if _, err := os.Stat(filepath.Join(root, "apps")); !os.IsNotExist(err) {
		t.Fatalf("apps directory stat = %v, want no installation after cancellation", err)
	}
}
