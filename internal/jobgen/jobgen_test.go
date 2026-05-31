package jobgen

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hapyco/dygo/internal/hookgen"
)

func TestGenerateCreatesJobRunAndRunner(t *testing.T) {
	root := newTestProject(t, "example.com/acme")
	writeTestApp(t, root, "sales")

	result, err := Generate(root, "sales", "send-email")
	if err != nil {
		t.Fatalf("Generate() error = %v, want nil", err)
	}
	if !result.JobFileCreated || !result.RunFileCreated || !result.RunnerFileWritten {
		t.Fatalf("Generate() result = %+v, want created files and runner", result)
	}

	jobSource := readTestFile(t, filepath.Join(root, "apps", "sales", "jobs", "send-email", "job.yml"))
	for _, want := range []string{"label: Send Email", "queue: default", "timeout: 30s", "retry:\n  attempts: 3"} {
		if !strings.Contains(jobSource, want) {
			t.Fatalf("job.yml = %q, want substring %q", jobSource, want)
		}
	}

	runSource := readTestFile(t, filepath.Join(root, "apps", "sales", "jobs", "send-email", "run.go"))
	for _, want := range []string{"package job", "func Run(ctx context.Context, job dygo.JobExecution) error", "TODO(sales/send-email): implement job behavior"} {
		if !strings.Contains(runSource, want) {
			t.Fatalf("run.go = %q, want substring %q", runSource, want)
		}
	}

	runnerSource := readTestFile(t, filepath.Join(root, "cmd", "dygo", "main.go"))
	for _, want := range []string{
		`salessendemailjob "example.com/acme/apps/sales/jobs/send-email"`,
		"Jobs: []dygo.JobRegistrar{",
		`return registry.RegisterJob("sales", "send-email", salessendemailjob.Run)`,
	} {
		if !strings.Contains(runnerSource, want) {
			t.Fatalf("runner source = %q, want substring %q", runnerSource, want)
		}
	}
}

func TestGenerateDryRunDoesNotWrite(t *testing.T) {
	root := newTestProject(t, "example.com/acme")
	writeTestApp(t, root, "sales")

	result, err := GenerateWithOptions(GenerateOptions{Root: root, AppName: "sales", JobName: "send-email", DryRun: true})
	if err != nil {
		t.Fatalf("GenerateWithOptions(dry-run) error = %v, want nil", err)
	}
	if result.JobFileStatus != "would create" || result.RunFileStatus != "would create" || result.RunnerFileStatus != "would create" {
		t.Fatalf("GenerateWithOptions(dry-run) result = %+v, want would create statuses", result)
	}
	assertPathMissing(t, filepath.Join(root, "apps", "sales", "jobs", "send-email", "job.yml"))
	assertPathMissing(t, filepath.Join(root, "apps", "sales", "jobs", "send-email", "run.go"))
	assertPathMissing(t, filepath.Join(root, "cmd", "dygo", "main.go"))
}

func TestGeneratePreservesExistingRunFunction(t *testing.T) {
	root := newTestProject(t, "example.com/acme")
	writeTestApp(t, root, "sales")
	writeTestFile(t, filepath.Join(root, "apps", "sales", "jobs", "send-email", "job.yml"), `label: Send Email
queue: default
timeout: 30s
`)
	runPath := filepath.Join(root, "apps", "sales", "jobs", "send-email", "run.go")
	existing := `package job

import (
	"context"

	"github.com/hapyco/dygo/pkg/dygo"
)

func Run(ctx context.Context, job dygo.JobExecution) error {
	return nil
}
`
	writeTestFile(t, runPath, existing)

	result, err := Generate(root, "sales", "send-email")
	if err != nil {
		t.Fatalf("Generate() error = %v, want nil", err)
	}
	if result.RunFileStatus != "existing" || result.RunFileCreated {
		t.Fatalf("Generate() result = %+v, want existing run file", result)
	}
	if got := readTestFile(t, runPath); got != existing {
		t.Fatalf("run.go changed:\n%s", got)
	}
	runnerSource := readTestFile(t, filepath.Join(root, "cmd", "dygo", "main.go"))
	if !strings.Contains(runnerSource, `return registry.RegisterJob("sales", "send-email", salessendemailjob.Run)`) {
		t.Fatalf("runner source = %q, want job registration", runnerSource)
	}
}

func TestGenerateFailsForExistingRunWithoutRunFunction(t *testing.T) {
	root := newTestProject(t, "example.com/acme")
	writeTestApp(t, root, "sales")
	writeTestFile(t, filepath.Join(root, "apps", "sales", "jobs", "send-email", "run.go"), "package job\n")

	_, err := Generate(root, "sales", "send-email")
	if err == nil {
		t.Fatal("Generate() error = nil, want missing Run error")
	}
	if !strings.Contains(err.Error(), "does not expose Run(ctx context.Context, job dygo.JobExecution) error") {
		t.Fatalf("Generate() error = %q, want missing Run message", err.Error())
	}
	assertPathMissing(t, filepath.Join(root, "cmd", "dygo", "main.go"))
}

func TestHookGenerationPreservesJobRunnerWiring(t *testing.T) {
	root := newTestProject(t, "example.com/acme")
	writeTestApp(t, root, "sales")
	writeTestEntity(t, root, "sales", "lead")

	if _, err := Generate(root, "sales", "send-email"); err != nil {
		t.Fatalf("Generate(job) error = %v, want nil", err)
	}
	if _, err := hookgen.Generate(root, "sales", "lead"); err != nil {
		t.Fatalf("Generate(hook) error = %v, want nil", err)
	}

	runnerSource := readTestFile(t, filepath.Join(root, "cmd", "dygo", "main.go"))
	for _, want := range []string{
		"RecordHooks: []dygo.RecordHookRegistrar{",
		"Jobs: []dygo.JobRegistrar{",
		"salesleadhooks.Register",
		`return registry.RegisterJob("sales", "send-email", salessendemailjob.Run)`,
	} {
		if !strings.Contains(runnerSource, want) {
			t.Fatalf("runner source = %q, want substring %q", runnerSource, want)
		}
	}
}

func TestGenerateOutputCompilesInProject(t *testing.T) {
	repoRoot := repositoryRoot(t)
	root := newTestProjectWithBody(t, `module example.com/acme

go 1.26.2

require github.com/hapyco/dygo v0.0.0

replace github.com/hapyco/dygo => `+filepath.ToSlash(repoRoot)+`
`)
	copyTestFile(t, filepath.Join(repoRoot, "go.sum"), filepath.Join(root, "go.sum"))
	writeTestApp(t, root, "sales")

	if _, err := Generate(root, "sales", "send-email"); err != nil {
		t.Fatalf("Generate() error = %v, want nil", err)
	}

	runGoCommand(t, root, "mod", "tidy")
	runGoCommand(t, root, "test", "./...")
}

func newTestProject(t *testing.T, modulePath string) string {
	t.Helper()
	return newTestProjectWithBody(t, "module "+modulePath+"\n\ngo 1.26.2\n")
}

func newTestProjectWithBody(t *testing.T, goMod string) string {
	t.Helper()
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "dygo.yml"), "name: test\n")
	writeTestFile(t, filepath.Join(root, "go.mod"), goMod)
	return root
}

func writeTestApp(t *testing.T, root string, name string) {
	t.Helper()
	body := "name: " + name + "\nlabel: " + strings.ToUpper(name[:1]) + name[1:] + "\nversion: 0.1.0\n"
	writeTestFile(t, filepath.Join(root, "apps", name, "app.yml"), body)
}

func writeTestEntity(t *testing.T, root string, appName string, entityName string) {
	t.Helper()
	body := "label: " + strings.ToUpper(entityName[:1]) + entityName[1:] + "\nname:\n  strategy: random\nfields:\n  - name: title\n    label: Title\n    type: text\n"
	writeTestFile(t, filepath.Join(root, "apps", appName, "entities", entityName, "entity.yml"), body)
}

func writeTestFile(t *testing.T, path string, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	return string(data)
}

func copyTestFile(t *testing.T, source string, destination string) {
	t.Helper()
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", source, err)
	}
	writeTestFile(t, destination, string(data))
}

func assertPathMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("%s exists, want missing", path)
	} else if !os.IsNotExist(err) {
		t.Fatalf("Stat(%s) error = %v, want missing", path, err)
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse --show-toplevel error = %v", err)
	}
	return string(bytes.TrimSpace(output))
}

func runGoCommand(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go %s in generated project error = %v\n%s", strings.Join(args, " "), err, string(output))
	}
}
