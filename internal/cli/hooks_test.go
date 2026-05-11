package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHooksGenerateCommandCreatesScaffold(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIGoModule(t, root, "example.com/acme")
	writeCLIApp(t, filepath.Join(root, "apps", "sales"), "sales")
	writeCLIEntity(t, filepath.Join(root, "apps", "sales", "entities", "lead.yml"), `
name: lead
label: Lead
fields:
  - name: title
    label: Title
    type: text
`)
	t.Chdir(root)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"hooks", "generate", "sales", "lead"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run(hooks generate) error = %v, want nil", err)
	}
	for _, want := range []string{
		"generated hooks for sales/lead",
		"hook: apps/sales/hooks/lead.go (created)",
		"register: apps/sales/hooks/register.go (updated)",
		"runner: cmd/dygo/main.go (updated)",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("hooks generate stdout = %q, want substring %q", stdout.String(), want)
		}
	}
	for _, path := range []string{
		filepath.Join(root, "apps", "sales", "hooks", "lead.go"),
		filepath.Join(root, "apps", "sales", "hooks", "register.go"),
		filepath.Join(root, "cmd", "dygo", "main.go"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("Stat(%s) error = %v, want generated file", path, err)
		}
	}
}

func TestHooksGenerateCommandWrapsGeneratorErrors(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIGoModule(t, root, "example.com/acme")
	writeCLIApp(t, filepath.Join(root, "apps", "sales"), "sales")
	t.Chdir(root)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"hooks", "generate", "sales", "lead"}, strings.NewReader(""), &stdout, &stderr)
	if err == nil {
		t.Fatal("Run(hooks generate) error = nil, want unknown entity error")
	}
	for _, want := range []string{"generate hooks", `entity "lead" not found in app "sales"`} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Run(hooks generate) error = %q, want substring %q", err.Error(), want)
		}
	}
}

func writeCLIGoModule(t *testing.T, root string, modulePath string) {
	t.Helper()

	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module "+modulePath+"\n\ngo 1.26.2\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(go.mod) error = %v", err)
	}
}
