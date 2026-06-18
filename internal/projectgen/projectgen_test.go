package projectgen

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hapyco/dygo/internal/app/manifest"
	"github.com/hapyco/dygo/internal/config"
	"github.com/hapyco/dygo/internal/queues"
	"github.com/hapyco/dygo/internal/secrets"
)

func TestGenerateCreatesProjectSkeletonAndSecrets(t *testing.T) {
	parent := t.TempDir()
	repoRoot := repositoryRoot(t)

	result, err := Generate(context.Background(), Options{
		Name:          "My Company",
		ModulePath:    "example.com/my-company",
		WorkingDir:    parent,
		FrameworkRoot: repoRoot,
		SkipTidy:      true,
	})
	if err != nil {
		t.Fatalf("Generate() error = %v, want nil", err)
	}
	if result.Name != "my-company" || result.Label != "My Company" || result.ModulePath != "example.com/my-company" {
		t.Fatalf("Generate() result = %+v, want normalized name, label, module", result)
	}
	if result.TidyRun {
		t.Fatal("TidyRun = true, want false with SkipTidy")
	}

	root := filepath.Join(parent, "my-company")
	for _, path := range []string{
		"dygo.yml",
		"go.mod",
		"cmd/dygo/main.go",
		"apps/my-company/app.yml",
		"apps/my-company/entities",
		"apps/my-company/entities/_collections",
		"apps/my-company/jobs",
		"apps/my-company/jobs/_schedules.yml",
		"apps/my-company/pages",
		"apps/my-company/reports",
		"apps/my-company/access/_roles.yml",
		"config/queues.yml",
		"config/secrets/development.yml.age",
		"config/secrets/staging.yml.age",
		"config/secrets/production.yml.age",
		"db/schema.sql",
		"docs/index.md",
		".dygo/apps/studio",
		".dygo/files",
		".dygo/logs",
		".dygo/tmp",
		".dygo/secrets",
		".dygo/secrets/master.key",
		".gitignore",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(path))); err != nil {
			t.Fatalf("Stat(%s) error = %v, want generated path", path, err)
		}
	}

	if _, err := config.Load(root); err != nil {
		t.Fatalf("config.Load() error = %v, want generated config valid", err)
	}
	queueConfig, err := queues.Load(root)
	if err != nil {
		t.Fatalf("queues.Load() error = %v, want generated queue config valid", err)
	}
	if !queueConfig.Has("default") || len(queueConfig.Queues) != 1 || queueConfig.Queues[0].Concurrency != 4 {
		t.Fatalf("queue config = %+v, want default queue concurrency 4", queueConfig)
	}
	app, err := manifest.LoadFile(filepath.Join(root, "apps", "my-company", "app.yml"))
	if err != nil {
		t.Fatalf("manifest.LoadFile() error = %v, want generated app valid", err)
	}
	if app.Name != "my-company" || app.Label != "My Company" {
		t.Fatalf("app manifest = %+v, want generated app metadata", app)
	}

	store := secrets.NewStore(root)
	secret, err := store.Get(secrets.EnvironmentDevelopment, "DATABASE_URL")
	if err != nil {
		t.Fatalf("Get(development DATABASE_URL) error = %v, want seeded secret", err)
	}
	if secret.Value != "postgres://localhost/my_company_development?sslmode=disable" {
		t.Fatalf("development DATABASE_URL = %q, want default local URL", secret.Value)
	}
	if err := store.Validate(secrets.EnvironmentDevelopment); err != nil {
		t.Fatalf("Validate(development) error = %v, want generated dev secrets valid", err)
	}
	keyInfo, err := os.Stat(filepath.Join(root, ".dygo", "secrets", "master.key"))
	if err != nil {
		t.Fatalf("Stat(.dygo/secrets/master.key) error = %v", err)
	}
	if keyInfo.Mode().Perm() != 0o600 {
		t.Fatalf(".dygo/secrets/master.key mode = %v, want 0600", keyInfo.Mode().Perm())
	}

	assertContains(t, readFile(t, filepath.Join(root, ".gitignore")), ".dygo/secrets/master.key")
	assertContains(t, readFile(t, filepath.Join(root, ".gitignore")), ".dygo/")
	assertContains(t, readFile(t, filepath.Join(root, "cmd", "dygo", "main.go")), "dygoruntime.Run")
	readme := readFile(t, filepath.Join(root, "README.md"))
	assertContains(t, readme, "dygo db prepare")
	assertContains(t, readme, "dygo setup")
	assertContains(t, readme, "dygo dev")
}

func TestGenerateInstallsStudioCacheFromFrameworkBuild(t *testing.T) {
	parent := t.TempDir()
	frameworkRoot := t.TempDir()
	writeProjectgenFile(t, filepath.Join(frameworkRoot, "apps", "studio", "ui", "dist", "index.html"), "<html>studio</html>")
	writeProjectgenFile(t, filepath.Join(frameworkRoot, "apps", "studio", "ui", "dist", "assets", "index.js"), "console.log('studio')")

	result, err := Generate(context.Background(), Options{
		Name:          "studio-ready",
		WorkingDir:    parent,
		FrameworkRoot: frameworkRoot,
		SkipTidy:      true,
	})
	if err != nil {
		t.Fatalf("Generate() error = %v, want nil", err)
	}
	if !result.StudioCached || result.StudioSource != "framework Studio build" {
		t.Fatalf("Generate() result = %+v, want Studio cache from framework build", result)
	}
	root := filepath.Join(parent, "studio-ready")
	assertContains(t, readFile(t, filepath.Join(root, ".dygo", "apps", "studio", "ui", "dist", "index.html")), "studio")
	assertContains(t, readFile(t, filepath.Join(root, ".dygo", "apps", "studio", "ui", "dist", "assets", "index.js")), "console.log")
}

func TestGenerateDefaultsModuleToName(t *testing.T) {
	parent := t.TempDir()

	_, err := Generate(context.Background(), Options{
		Name:          "acme-ops",
		WorkingDir:    parent,
		FrameworkRoot: repositoryRoot(t),
		SkipTidy:      true,
	})
	if err != nil {
		t.Fatalf("Generate() error = %v, want nil", err)
	}
	goMod := readFile(t, filepath.Join(parent, "acme-ops", "go.mod"))
	assertContains(t, goMod, "module acme-ops")
	assertContains(t, goMod, "require github.com/hapyco/dygo v0.0.0")
	assertContains(t, goMod, "replace github.com/hapyco/dygo => "+filepath.ToSlash(repositoryRoot(t)))
}

func TestGenerateUsesReleaseDygoVersionWithoutLocalReplace(t *testing.T) {
	parent := t.TempDir()

	_, err := Generate(context.Background(), Options{
		Name:        "release-app",
		WorkingDir:  parent,
		DygoVersion: "v1.2.3",
		SkipTidy:    true,
	})
	if err != nil {
		t.Fatalf("Generate() error = %v, want nil", err)
	}
	goMod := readFile(t, filepath.Join(parent, "release-app", "go.mod"))
	assertContains(t, goMod, "require github.com/hapyco/dygo v1.2.3")
	if strings.Contains(goMod, "replace github.com/hapyco/dygo") {
		t.Fatalf("go.mod = %q, want release dependency without local replace", goMod)
	}
}

func TestGenerateRejectsExistingTargetAndInvalidNames(t *testing.T) {
	parent := t.TempDir()
	repoRoot := repositoryRoot(t)
	if err := os.Mkdir(filepath.Join(parent, "acme"), 0o755); err != nil {
		t.Fatalf("Mkdir(acme) error = %v", err)
	}

	tests := []struct {
		name string
		opts Options
		want string
	}{
		{name: "existing", opts: Options{Name: "acme", WorkingDir: parent, FrameworkRoot: repoRoot, SkipTidy: true}, want: "already exists"},
		{name: "empty", opts: Options{Name: "   ", WorkingDir: parent, FrameworkRoot: repoRoot, SkipTidy: true}, want: "project name is required"},
		{name: "digit", opts: Options{Name: "123 app", WorkingDir: parent, FrameworkRoot: repoRoot, SkipTidy: true}, want: "must start with a letter"},
		{name: "reserved app", opts: Options{Name: "Core", WorkingDir: parent, FrameworkRoot: repoRoot, SkipTidy: true}, want: `app name "core" is reserved`},
		{name: "bad module", opts: Options{Name: "valid", ModulePath: "bad module", WorkingDir: parent, FrameworkRoot: repoRoot, SkipTidy: true}, want: "must not contain whitespace"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			_, err := Generate(context.Background(), tt.opts)
			if err == nil {
				t.Fatal("Generate() error = nil, want validation error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Generate() error = %q, want substring %q", err.Error(), tt.want)
			}
		})
	}
}

func TestGenerateRunsTidyAndGeneratedProjectCompiles(t *testing.T) {
	parent := t.TempDir()
	root := repositoryRoot(t)

	result, err := Generate(context.Background(), Options{
		Name:          "compile-check",
		WorkingDir:    parent,
		FrameworkRoot: root,
	})
	if err != nil {
		t.Fatalf("Generate() error = %v, want nil", err)
	}
	if !result.TidyRun {
		t.Fatal("TidyRun = false, want go mod tidy to run by default")
	}
	if _, err := os.Stat(filepath.Join(result.Path, "go.sum")); err != nil {
		t.Fatalf("Stat(go.sum) error = %v, want tidy-created go.sum", err)
	}
	runGoCommand(t, result.Path, "test", "./...")
}

func TestNormalizeName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "My Company", want: "my-company"},
		{input: "acme_ops", want: "acme-ops"},
		{input: "  ACME---Ops  ", want: "acme-ops"},
		{input: "Café CRM", want: "caf-crm"},
	}
	for _, tt := range tests {
		got, err := NormalizeName(tt.input)
		if err != nil {
			t.Fatalf("NormalizeName(%q) error = %v, want nil", tt.input, err)
		}
		if got != tt.want {
			t.Fatalf("NormalizeName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	return string(data)
}

func assertContains(t *testing.T, source string, want string) {
	t.Helper()
	if !strings.Contains(source, want) {
		t.Fatalf("source = %q, want substring %q", source, want)
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse --show-toplevel error = %v", err)
	}
	return strings.TrimSpace(string(output))
}

func runGoCommand(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go %s error = %v\n%s", strings.Join(args, " "), err, string(output))
	}
}

func writeProjectgenFile(t *testing.T, path string, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
}
