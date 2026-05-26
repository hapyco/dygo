package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateHookCommandCreatesScaffold(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIGoModule(t, root, "example.com/acme")
	writeCLIApp(t, filepath.Join(root, "apps", "sales"), "sales")
	writeCLIEntity(t, cliEntityPath(root, "sales", "lead"), `
label: Lead
fields:
  - name: title
    label: Title
    type: text
`)
	t.Chdir(root)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"generate", "hook", "sales/lead"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run(generate hook) error = %v, want nil", err)
	}
	for _, want := range []string{
		"generated hook for sales/lead",
		"hook: apps/sales/entities/lead/hooks.go (created)",
		"runner: cmd/dygo/main.go (updated)",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("generate hook stdout = %q, want substring %q", stdout.String(), want)
		}
	}
	for _, path := range []string{
		filepath.Join(root, "apps", "sales", "entities", "lead", "hooks.go"),
		filepath.Join(root, "cmd", "dygo", "main.go"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("Stat(%s) error = %v, want generated file", path, err)
		}
	}
}

func TestGenerateHookAliasCreatesScaffold(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIGoModule(t, root, "example.com/acme")
	writeCLIApp(t, filepath.Join(root, "apps", "sales"), "sales")
	writeCLIEntity(t, cliEntityPath(root, "sales", "lead"), `
label: Lead
fields:
  - name: title
    label: Title
    type: text
`)
	t.Chdir(root)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"g", "hook", "sales/lead"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run(g hook) error = %v, want nil", err)
	}
	if !strings.Contains(stdout.String(), "generated hook for sales/lead") {
		t.Fatalf("g hook stdout = %q, want generated output", stdout.String())
	}
}

func TestGenerateHookCommandWrapsGeneratorErrors(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIGoModule(t, root, "example.com/acme")
	writeCLIApp(t, filepath.Join(root, "apps", "sales"), "sales")
	t.Chdir(root)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"generate", "hook", "sales/lead"}, strings.NewReader(""), &stdout, &stderr)
	if err == nil {
		t.Fatal("Run(generate hook) error = nil, want unknown entity error")
	}
	for _, want := range []string{"generate hook", `entity "lead" not found in app "sales"`} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Run(generate hook) error = %q, want substring %q", err.Error(), want)
		}
	}
}

func TestOldHooksGenerateCommandIsRemoved(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIGoModule(t, root, "example.com/acme")
	t.Chdir(root)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"hooks", "generate", "sales", "lead"}, strings.NewReader(""), &stdout, &stderr)
	if err == nil {
		t.Fatal("Run(hooks generate) error = nil, want old command removed")
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("Run(hooks generate) error = %q, want unknown command", err.Error())
	}
}

func TestHookListAndValidateCommands(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIGoModule(t, root, "example.com/acme")
	writeCLIApp(t, filepath.Join(root, "apps", "sales"), "sales")
	writeCLIEntity(t, cliEntityPath(root, "sales", "lead"), `
label: Lead
fields:
  - name: title
    label: Title
    type: text
`)
	t.Chdir(root)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := Run(context.Background(), []string{"generate", "hook", "sales/lead"}, strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatalf("Run(generate hook) error = %v, want nil", err)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Run(context.Background(), []string{"hook", "list"}, strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatalf("Run(hook list) error = %v, want nil", err)
	}
	for _, want := range []string{"sales/lead", "apps/sales/entities/lead/hooks.go", "register:yes", "runner:wired"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("hook list stdout = %q, want substring %q", stdout.String(), want)
		}
	}

	stdout.Reset()
	stderr.Reset()
	if err := Run(context.Background(), []string{"hook", "validate"}, strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatalf("Run(hook validate) error = %v, want nil", err)
	}
	if stdout.String() != "hooks are valid\n" {
		t.Fatalf("hook validate stdout = %q, want valid output", stdout.String())
	}
}

func writeCLIGoModule(t *testing.T, root string, modulePath string) {
	t.Helper()

	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module "+modulePath+"\n\ngo 1.26.2\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(go.mod) error = %v", err)
	}
}
