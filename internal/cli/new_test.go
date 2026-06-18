package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/hapyco/dygo/internal/secrets"
)

func TestNewProjectCommandCreatesProject(t *testing.T) {
	root := t.TempDir()
	writeCLIFrameworkRootForNew(t, root)
	t.Chdir(root)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"new", "My Company", "--module", "example.com/my-company", "--skip-tidy"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run(new) error = %v, want nil", err)
	}

	for _, want := range []string{
		"created dygo project: my-company",
		"path: my-company",
		"module: example.com/my-company",
		"secrets: initialized",
		"dependencies: tidy skipped",
		"studio: cached from framework Studio build",
		"dygo db prepare",
		"dygo setup",
		"dygo dev",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("new stdout = %q, want substring %q", stdout.String(), want)
		}
	}
	if stderr.String() != "" {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	projectRoot := filepath.Join(root, "my-company")
	for _, path := range []string{
		"dygo.yml",
		"go.mod",
		"cmd/dygo/main.go",
		"apps/my-company/app.yml",
		"apps/my-company/jobs/_schedules.yml",
		"config/secrets/development.yml.age",
		"config/secrets/staging.yml.age",
		"config/secrets/production.yml.age",
		".dygo/secrets/master.key",
		".dygo/apps/studio/ui/dist/index.html",
	} {
		if _, err := os.Stat(filepath.Join(projectRoot, filepath.FromSlash(path))); err != nil {
			t.Fatalf("Stat(%s) error = %v, want generated path", path, err)
		}
	}

	store := secrets.NewStore(projectRoot)
	secret, err := store.Get(secrets.EnvironmentDevelopment, "DATABASE_URL")
	if err != nil {
		t.Fatalf("Get(development DATABASE_URL) error = %v, want seeded secret", err)
	}
	if secret.Value != "postgres://localhost/my_company_development?sslmode=disable" {
		t.Fatalf("development DATABASE_URL = %q, want generated local URL", secret.Value)
	}
}

func TestNewProjectCommandRejectsExistingProject(t *testing.T) {
	root := t.TempDir()
	writeCLIFrameworkRootForNew(t, root)
	if err := os.Mkdir(filepath.Join(root, "acme"), 0o755); err != nil {
		t.Fatalf("Mkdir(acme) error = %v", err)
	}
	t.Chdir(root)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"new", "acme", "--skip-tidy"}, strings.NewReader(""), &stdout, &stderr)
	if err == nil {
		t.Fatal("Run(new existing) error = nil, want error")
	}
	for _, want := range []string{"create project", "already exists"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Run(new existing) error = %q, want substring %q", err.Error(), want)
		}
	}
}

func TestNewProjectCommandRejectsReservedAppName(t *testing.T) {
	root := t.TempDir()
	writeCLIFrameworkRootForNew(t, root)
	t.Chdir(root)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"new", "Core", "--skip-tidy"}, strings.NewReader(""), &stdout, &stderr)
	if err == nil {
		t.Fatal("Run(new Core) error = nil, want reserved app name error")
	}
	if !strings.Contains(err.Error(), `app name "core" is reserved`) {
		t.Fatalf("Run(new Core) error = %q, want reserved app name", err.Error())
	}
}

func TestDygoVersionForNewUsesBuildInfoReleaseVersion(t *testing.T) {
	oldReadBuildInfo := readBuildInfo
	readBuildInfo = func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{Main: debug.Module{Version: "v1.2.3"}}, true
	}
	defer func() {
		readBuildInfo = oldReadBuildInfo
	}()

	if got := dygoVersionForNew(); got != "v1.2.3" {
		t.Fatalf("dygoVersionForNew() = %q, want build info version", got)
	}
}

func writeCLIFrameworkRootForNew(t *testing.T, root string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Join(root, "apps"), 0o755); err != nil {
		t.Fatalf("MkdirAll(apps) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "internal", "cli"), 0o755); err != nil {
		t.Fatalf("MkdirAll(internal/cli) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module github.com/hapyco/dygo\n\ngo 1.26.2\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(go.mod) error = %v", err)
	}
	writeCLIFrameworkFileForNew(t, filepath.Join(root, "apps", "studio", "ui", "dist", "index.html"), "<html>studio</html>")
	writeCLIFrameworkFileForNew(t, filepath.Join(root, "apps", "studio", "ui", "dist", "assets", "index.js"), "console.log('studio')")
}

func writeCLIFrameworkFileForNew(t *testing.T, path string, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
}
