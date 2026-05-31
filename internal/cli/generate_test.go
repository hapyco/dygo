package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateEntityCreatesStandardBundle(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIGoModule(t, root, "example.com/acme")
	writeCLIApp(t, filepath.Join(root, "apps", "sales"), "sales")
	t.Chdir(root)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"generate", "entity", "sales/lead"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run(generate entity) error = %v, want nil", err)
	}
	for _, want := range []string{
		"generated entity sales/lead",
		"file: apps/sales/entities/lead/entity.yml (created)",
		"file: apps/sales/entities/lead/fixtures.yml (created)",
		"file: apps/sales/entities/lead/hooks_test.go (created)",
		"hook: apps/sales/entities/lead/hooks.go (created)",
		"runner: cmd/dygo/main.go (created)",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("generate entity stdout = %q, want substring %q", stdout.String(), want)
		}
	}

	for _, path := range []string{
		"apps/sales/entities/lead/entity.yml",
		"apps/sales/entities/lead/fixtures.yml",
		"apps/sales/entities/lead/hooks.go",
		"apps/sales/entities/lead/hooks_test.go",
		"cmd/dygo/main.go",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(path))); err != nil {
			t.Fatalf("Stat(%s) error = %v, want generated file", path, err)
		}
	}

	stdout.Reset()
	stderr.Reset()
	if err := Run(context.Background(), []string{"entity", "validate"}, strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatalf("Run(entity validate) error = %v, want generated metadata valid", err)
	}
	stdout.Reset()
	stderr.Reset()
	if err := Run(context.Background(), []string{"fixture", "validate"}, strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatalf("Run(fixture validate) error = %v, want generated fixture valid", err)
	}
	stdout.Reset()
	stderr.Reset()
	if err := Run(context.Background(), []string{"hook", "validate"}, strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatalf("Run(hook validate) error = %v, want generated hook wiring valid", err)
	}
}

func TestGenerateEntityDryRunDoesNotWrite(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIApp(t, filepath.Join(root, "apps", "sales"), "sales")
	t.Chdir(root)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"generate", "entity", "sales/lead", "--dry-run"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run(generate entity --dry-run) error = %v, want nil", err)
	}
	for _, want := range []string{
		"file: apps/sales/entities/lead/entity.yml (would create)",
		"hook: apps/sales/entities/lead/hooks.go (would create)",
		"runner: cmd/dygo/main.go (would update)",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("generate entity dry-run stdout = %q, want substring %q", stdout.String(), want)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "apps", "sales", "entities", "lead", "entity.yml")); !os.IsNotExist(err) {
		t.Fatalf("dry-run entity stat error = %v, want missing generated file", err)
	}
}

func TestGenerateEntityForcePreservesExistingHookFile(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIGoModule(t, root, "example.com/acme")
	writeCLIApp(t, filepath.Join(root, "apps", "sales"), "sales")
	t.Chdir(root)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := Run(context.Background(), []string{"generate", "entity", "sales/lead"}, strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatalf("Run(generate entity) error = %v, want nil", err)
	}

	hookPath := filepath.Join(root, "apps", "sales", "entities", "lead", "hooks.go")
	existing := `package hooks

import "github.com/hapyco/dygo/pkg/sdk"

func Register(registry sdk.RecordHookRegistry) error {
	return nil
}
`
	if err := os.WriteFile(hookPath, []byte(existing), 0o644); err != nil {
		t.Fatalf("WriteFile(hooks.go) error = %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Run(context.Background(), []string{"generate", "entity", "sales/lead", "--force"}, strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatalf("Run(generate entity --force) error = %v, want nil", err)
	}
	if got := readCLIFile(t, hookPath); got != existing {
		t.Fatalf("hooks.go changed after generate entity --force:\n%s", got)
	}
	if !strings.Contains(stdout.String(), "hook: apps/sales/entities/lead/hooks.go (existing)") {
		t.Fatalf("generate entity --force stdout = %q, want hook existing", stdout.String())
	}
}

func TestGenerateJobCommandCreatesScaffold(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIGoModule(t, root, "example.com/acme")
	writeCLIApp(t, filepath.Join(root, "apps", "sales"), "sales")
	t.Chdir(root)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"generate", "job", "sales/send-email"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run(generate job) error = %v, want nil", err)
	}
	for _, want := range []string{
		"generated job for sales/send-email",
		"job: apps/sales/jobs/send-email/job.yml (created)",
		"run: apps/sales/jobs/send-email/run.go (created)",
		"runner: cmd/dygo/main.go (created)",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("generate job stdout = %q, want substring %q", stdout.String(), want)
		}
	}
	for _, path := range []string{
		"apps/sales/jobs/send-email/job.yml",
		"apps/sales/jobs/send-email/run.go",
		"cmd/dygo/main.go",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(path))); err != nil {
			t.Fatalf("Stat(%s) error = %v, want generated file", path, err)
		}
	}
}

func TestGenerateJobDryRunDoesNotWrite(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIGoModule(t, root, "example.com/acme")
	writeCLIApp(t, filepath.Join(root, "apps", "sales"), "sales")
	t.Chdir(root)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"generate", "job", "sales/send-email", "--dry-run"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run(generate job --dry-run) error = %v, want nil", err)
	}
	for _, want := range []string{
		"generated job for sales/send-email",
		"job: apps/sales/jobs/send-email/job.yml (would create)",
		"run: apps/sales/jobs/send-email/run.go (would create)",
		"runner: cmd/dygo/main.go (would create)",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("generate job --dry-run stdout = %q, want substring %q", stdout.String(), want)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "apps", "sales", "jobs", "send-email", "job.yml")); !os.IsNotExist(err) {
		t.Fatalf("dry-run job stat error = %v, want missing generated file", err)
	}
}

func TestGenerateCollectionFixtureAndTestCommands(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIApp(t, filepath.Join(root, "apps", "sales"), "sales")
	writeCLIEntity(t, cliEntityPath(root, "sales", "lead"), `
label: Lead
fields:
  - name: title
    label: Title
    type: text
`)
	t.Chdir(root)

	commands := [][]string{
		{"generate", "collection", "sales/line-item"},
		{"generate", "fixture", "sales/lead"},
		{"generate", "test", "sales/lead"},
	}
	for _, args := range commands {
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		if err := Run(context.Background(), args, strings.NewReader(""), &stdout, &stderr); err != nil {
			t.Fatalf("Run(%v) error = %v, want nil", args, err)
		}
	}
	for _, path := range []string{
		"apps/sales/entities/_collections/line-item.yml",
		"apps/sales/entities/lead/fixtures.yml",
		"apps/sales/entities/lead/hooks_test.go",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(path))); err != nil {
			t.Fatalf("Stat(%s) error = %v, want generated file", path, err)
		}
	}
}

func TestGenerateAppForceOnlyOverwritesGeneratedFiles(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	t.Chdir(root)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := Run(context.Background(), []string{"generate", "app", "sales"}, strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatalf("Run(generate app) error = %v, want nil", err)
	}
	if _, err := os.Stat(filepath.Join(root, "apps", "sales", "jobs", "_schedules.yml")); err != nil {
		t.Fatalf("Stat(app schedules) error = %v, want generated schedules file", err)
	}
	schedulesPath := filepath.Join(root, "apps", "sales", "jobs", "_schedules.yml")
	if !strings.Contains(readCLIFile(t, schedulesPath), "Code generated by dygo generate app") {
		t.Fatalf("_schedules.yml missing generated marker")
	}

	appPath := filepath.Join(root, "apps", "sales", "app.yml")
	if err := os.WriteFile(appPath, []byte("# Code generated by dygo generate app; DO NOT EDIT.\nname: sales\nlabel: Broken\nversion: 0.1.0\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(app.yml) error = %v", err)
	}
	if err := os.WriteFile(schedulesPath, []byte("# Code generated by dygo generate app; DO NOT EDIT.\nschedules:\n  - name: old\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(_schedules.yml) error = %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	err := Run(context.Background(), []string{"generate", "app", "sales"}, strings.NewReader(""), &stdout, &stderr)
	if err == nil {
		t.Fatal("Run(generate app) error = nil, want force requirement")
	}
	if !strings.Contains(err.Error(), "rerun with --force") {
		t.Fatalf("Run(generate app) error = %q, want force requirement", err.Error())
	}

	stdout.Reset()
	stderr.Reset()
	if err := Run(context.Background(), []string{"generate", "app", "sales", "--force"}, strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatalf("Run(generate app --force) error = %v, want nil", err)
	}
	if !strings.Contains(readCLIFile(t, appPath), "label: Sales") {
		t.Fatalf("app.yml was not restored by --force")
	}
	if !strings.Contains(readCLIFile(t, schedulesPath), "schedules: []") {
		t.Fatalf("_schedules.yml was not restored by --force")
	}

	if err := os.WriteFile(appPath, []byte("name: sales\nlabel: Custom\nversion: 0.1.0\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(custom app.yml) error = %v", err)
	}
	stdout.Reset()
	stderr.Reset()
	err = Run(context.Background(), []string{"generate", "app", "sales", "--force"}, strings.NewReader(""), &stdout, &stderr)
	if err == nil {
		t.Fatal("Run(generate app --force custom) error = nil, want custom-file refusal")
	}
	if !strings.Contains(err.Error(), "exists and is not dygo-generated") {
		t.Fatalf("Run(generate app --force custom) error = %q, want custom-file refusal", err.Error())
	}
}

func TestGenerateAppRejectsReservedAppName(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	t.Chdir(root)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"generate", "app", "studio"}, strings.NewReader(""), &stdout, &stderr)
	if err == nil {
		t.Fatal("Run(generate app studio) error = nil, want reserved app name error")
	}
	if !strings.Contains(err.Error(), `app name "studio" is reserved`) {
		t.Fatalf("Run(generate app studio) error = %q, want reserved app name", err.Error())
	}
}

func readCLIFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	return string(data)
}
