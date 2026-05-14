package hookgen

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateCreatesHookRegisterAndRunner(t *testing.T) {
	root := newTestProject(t, "example.com/acme")
	writeTestApp(t, root, "sales", "")
	writeTestEntity(t, root, "sales", "lead")

	result, err := Generate(root, "sales", "lead")
	if err != nil {
		t.Fatalf("Generate() error = %v, want nil", err)
	}
	if !result.HookFileCreated || !result.RegisterFileWritten || !result.RunnerFileWritten {
		t.Fatalf("Generate() result = %+v, want created/written files", result)
	}

	hookSource := readTestFile(t, filepath.Join(root, "apps", "sales", "hooks", "lead.go"))
	for _, want := range []string{
		"func registerLeadHooks(registry sdk.RecordHookRegistry) error",
		`sdk.RecordBeforeCreate, "lead-before-save", beforeSaveLead`,
		`sdk.RecordBeforeUpdate, "lead-before-save", beforeSaveLead`,
		`sdk.RecordAfterCreate, "lead-after-save", afterSaveLead`,
		`sdk.RecordAfterUpdate, "lead-after-save", afterSaveLead`,
		"func beforeSaveLead(ctx context.Context, dygo sdk.RecordHook) error",
		"func afterSaveLead(ctx context.Context, dygo sdk.RecordHook) error",
	} {
		if !strings.Contains(hookSource, want) {
			t.Fatalf("hook source = %q, want substring %q", hookSource, want)
		}
	}

	registerSource := readTestFile(t, filepath.Join(root, "apps", "sales", "hooks", "register.go"))
	for _, want := range []string{generatedHeader, "func Register(registry sdk.RecordHookRegistry) error", "registerLeadHooks(registry)"} {
		if !strings.Contains(registerSource, want) {
			t.Fatalf("register source = %q, want substring %q", registerSource, want)
		}
	}

	runnerSource := readTestFile(t, filepath.Join(root, "cmd", "dygo", "main.go"))
	for _, want := range []string{
		generatedHeader,
		`saleshooks "example.com/acme/apps/sales/hooks"`,
		"dygoruntime.Run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr",
		"saleshooks.Register",
	} {
		if !strings.Contains(runnerSource, want) {
			t.Fatalf("runner source = %q, want substring %q", runnerSource, want)
		}
	}

	second, err := Generate(root, "sales", "lead")
	if err != nil {
		t.Fatalf("Generate() second error = %v, want nil", err)
	}
	if second.HookFileCreated || second.RegisterFileWritten || second.RunnerFileWritten {
		t.Fatalf("Generate() second result = %+v, want idempotent no-op", second)
	}
}

func TestGenerateReturnsUsefulLookupErrors(t *testing.T) {
	root := newTestProject(t, "example.com/acme")
	writeTestApp(t, root, "sales", "")
	writeTestEntity(t, root, "sales", "lead")

	tests := []struct {
		name       string
		appName    string
		entityName string
		want       string
	}{
		{name: "unknown app", appName: "support", entityName: "lead", want: `app "support" not found`},
		{name: "unknown entity", appName: "sales", entityName: "deal", want: `entity "deal" not found in app "sales"`},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			_, err := Generate(root, tt.appName, tt.entityName)
			if err == nil {
				t.Fatal("Generate() error = nil, want lookup error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Generate() error = %q, want substring %q", err.Error(), tt.want)
			}
		})
	}
}

func TestGenerateFailsSafelyForCustomRegister(t *testing.T) {
	root := newTestProject(t, "example.com/acme")
	writeTestApp(t, root, "sales", "")
	writeTestEntity(t, root, "sales", "lead")
	writeTestFile(t, filepath.Join(root, "apps", "sales", "hooks", "register.go"), "package hooks\n")

	_, err := Generate(root, "sales", "lead")
	if err == nil {
		t.Fatal("Generate() error = nil, want custom register error")
	}
	for _, want := range []string{"register.go exists and is not dygo-generated", "registerLeadHooks"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Generate() error = %q, want substring %q", err.Error(), want)
		}
	}
	assertPathMissing(t, filepath.Join(root, "apps", "sales", "hooks", "lead.go"))
}

func TestGenerateFailsSafelyForCustomRunner(t *testing.T) {
	root := newTestProject(t, "example.com/acme")
	writeTestApp(t, root, "sales", "")
	writeTestEntity(t, root, "sales", "lead")
	writeTestFile(t, filepath.Join(root, "cmd", "dygo", "main.go"), "package main\n")

	_, err := Generate(root, "sales", "lead")
	if err == nil {
		t.Fatal("Generate() error = nil, want custom runner error")
	}
	for _, want := range []string{"cmd/dygo/main.go exists and is not dygo-generated", `saleshooks "example.com/acme/apps/sales/hooks"`} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Generate() error = %q, want substring %q", err.Error(), want)
		}
	}
	assertPathMissing(t, filepath.Join(root, "apps", "sales", "hooks", "lead.go"))
}

func TestGenerateHonorsCustomHooksPathAndKebabEntityName(t *testing.T) {
	root := newTestProject(t, "example.com/acme")
	writeTestApp(t, root, "sales", "paths:\n  hooks: custom-hooks\n")
	writeTestEntity(t, root, "sales", "lead-status")

	_, err := Generate(root, "sales", "lead-status")
	if err != nil {
		t.Fatalf("Generate() error = %v, want nil", err)
	}

	hookSource := readTestFile(t, filepath.Join(root, "apps", "sales", "custom-hooks", "lead-status.go"))
	for _, want := range []string{"registerLeadStatusHooks", "beforeSaveLeadStatus", "afterSaveLeadStatus"} {
		if !strings.Contains(hookSource, want) {
			t.Fatalf("hook source = %q, want substring %q", hookSource, want)
		}
	}
	runnerSource := readTestFile(t, filepath.Join(root, "cmd", "dygo", "main.go"))
	if !strings.Contains(runnerSource, `saleshooks "example.com/acme/apps/sales/custom-hooks"`) {
		t.Fatalf("runner source = %q, want custom hooks import", runnerSource)
	}
}

func TestGeneratePreservesExistingEntityHookFile(t *testing.T) {
	root := newTestProject(t, "example.com/acme")
	writeTestApp(t, root, "sales", "")
	writeTestEntity(t, root, "sales", "lead")
	existing := `package hooks

import "github.com/dygo-dev/dygo/pkg/sdk"

func registerLeadHooks(registry sdk.RecordHookRegistry) error {
	return nil
}
`
	hookPath := filepath.Join(root, "apps", "sales", "hooks", "lead.go")
	writeTestFile(t, hookPath, existing)

	result, err := Generate(root, "sales", "lead")
	if err != nil {
		t.Fatalf("Generate() error = %v, want nil", err)
	}
	if result.HookFileCreated {
		t.Fatal("HookFileCreated = true, want existing hook preserved")
	}
	if got := readTestFile(t, hookPath); got != existing {
		t.Fatalf("hook source changed:\n%s", got)
	}
	registerSource := readTestFile(t, filepath.Join(root, "apps", "sales", "hooks", "register.go"))
	if !strings.Contains(registerSource, "registerLeadHooks(registry)") {
		t.Fatalf("register source = %q, want existing hook registrar call", registerSource)
	}
}

func TestGenerateRunnerOrdersMultipleHookApps(t *testing.T) {
	root := newTestProject(t, "example.com/acme")
	writeTestApp(t, root, "support", "")
	writeTestEntity(t, root, "support", "ticket")
	writeTestApp(t, root, "sales", "")
	writeTestEntity(t, root, "sales", "lead")

	if _, err := Generate(root, "support", "ticket"); err != nil {
		t.Fatalf("Generate(support ticket) error = %v, want nil", err)
	}
	if _, err := Generate(root, "sales", "lead"); err != nil {
		t.Fatalf("Generate(sales lead) error = %v, want nil", err)
	}

	runnerSource := readTestFile(t, filepath.Join(root, "cmd", "dygo", "main.go"))
	assertBefore(t, runnerSource, `saleshooks "example.com/acme/apps/sales/hooks"`, `supporthooks "example.com/acme/apps/support/hooks"`)
	assertBefore(t, runnerSource, "saleshooks.Register", "supporthooks.Register")
}

func TestGenerateOutputCompilesInProject(t *testing.T) {
	repoRoot := repositoryRoot(t)
	root := newTestProjectWithBody(t, `module example.com/acme

go 1.26.2

require github.com/dygo-dev/dygo v0.0.0

replace github.com/dygo-dev/dygo => `+filepath.ToSlash(repoRoot)+`
`)
	copyTestFile(t, filepath.Join(repoRoot, "go.sum"), filepath.Join(root, "go.sum"))
	writeTestApp(t, root, "sales", "")
	writeTestEntity(t, root, "sales", "lead")

	if _, err := Generate(root, "sales", "lead"); err != nil {
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

func writeTestApp(t *testing.T, root string, name string, extra string) {
	t.Helper()
	body := "name: " + name + "\nlabel: " + strings.ToUpper(name[:1]) + name[1:] + "\nversion: 0.1.0\n"
	if strings.TrimSpace(extra) != "" {
		body += strings.TrimSpace(extra) + "\n"
	}
	writeTestFile(t, filepath.Join(root, "apps", name, "app.yml"), body)
}

func writeTestEntity(t *testing.T, root string, appName string, entityName string) {
	t.Helper()
	body := "name: " + entityName + "\nlabel: " + strings.ToUpper(entityName[:1]) + entityName[1:] + "\nfields:\n  - name: title\n    label: Title\n    type: text\n"
	writeTestFile(t, filepath.Join(root, "apps", appName, "entities", entityName+".yml"), body)
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

func assertBefore(t *testing.T, source string, first string, second string) {
	t.Helper()
	firstIndex := strings.Index(source, first)
	if firstIndex == -1 {
		t.Fatalf("source = %q, missing %q", source, first)
	}
	secondIndex := strings.Index(source, second)
	if secondIndex == -1 {
		t.Fatalf("source = %q, missing %q", source, second)
	}
	if firstIndex >= secondIndex {
		t.Fatalf("source order invalid: %q should appear before %q in\n%s", first, second, source)
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
