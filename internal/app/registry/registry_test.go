package registry

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hapyco/dygo/internal/app/manifest"
)

func TestDiscoverUsesDefaultRoots(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeManifest(t, filepath.Join(root, ".dygo", "apps", "core"), `
name: core
label: Core
version: 0.1.0
`)
	writeManifest(t, filepath.Join(root, "apps", "sales"), `
name: sales
label: Sales
version: 0.1.0
dependencies:
  - core
`)

	apps, err := New(root).Discover()
	if err != nil {
		t.Fatalf("Discover() error = %v, want nil", err)
	}
	if got := appNames(apps); strings.Join(got, ",") != "core,sales" {
		t.Fatalf("Discover() names = %#v, want [core sales]", got)
	}
}

func TestValidateAcceptsDependenciesAcrossRoots(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeManifest(t, filepath.Join(root, ".dygo", "apps", "core"), `
name: core
label: Core
version: 0.1.0
`)
	writeManifest(t, filepath.Join(root, "apps", "sales"), `
name: sales
label: Sales
version: 0.1.0
dependencies:
  - core
`)

	apps, err := New(root).Validate()
	if err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
	if got := appNames(apps); strings.Join(got, ",") != "core,sales" {
		t.Fatalf("Validate() names = %#v, want [core sales]", got)
	}
}

func TestValidateRejectsReservedProjectAppNames(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeManifest(t, filepath.Join(root, ".dygo", "apps", "core"), `
name: core
label: Core
version: 0.1.0
`)
	writeManifest(t, filepath.Join(root, "apps", "studio"), `
name: studio
label: Studio
version: 0.1.0
dependencies:
  - core
`)

	_, err := New(root).Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want reserved app name error")
	}
	if !strings.Contains(err.Error(), `app name "studio" is reserved`) {
		t.Fatalf("Validate() error = %q, want reserved app name", err.Error())
	}
}

func TestValidateRejectsDuplicateAppsAcrossRoots(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeManifest(t, filepath.Join(root, ".dygo", "apps", "core"), `
name: core
label: Core
version: 0.1.0
`)
	writeManifest(t, filepath.Join(root, "apps", "core"), `
name: core
label: Core Duplicate
version: 0.1.0
`)

	_, err := New(root).Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want duplicate app error")
	}
	if !strings.Contains(err.Error(), "duplicate app") {
		t.Fatalf("Validate() error = %q, want duplicate app", err.Error())
	}
}

func TestValidateRejectsMissingDependencies(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeManifest(t, filepath.Join(root, "apps", "sales"), `
name: sales
label: Sales
version: 0.1.0
dependencies:
  - core
`)

	_, err := New(root).Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want missing dependency error")
	}
	if !strings.Contains(err.Error(), "unknown app") {
		t.Fatalf("Validate() error = %q, want unknown app", err.Error())
	}
}

func TestValidateMissingRoots(t *testing.T) {
	t.Parallel()

	apps, err := New(filepath.Join(t.TempDir(), "missing")).Validate()
	if err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
	if len(apps) != 0 {
		t.Fatalf("Validate() len = %d, want 0", len(apps))
	}
}

func writeManifest(t *testing.T, dir string, body string) {
	t.Helper()

	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	path := filepath.Join(dir, "app.yml")
	if err := os.WriteFile(path, []byte(strings.TrimSpace(body)+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func appNames(apps []manifest.LoadedApp) []string {
	names := make([]string, 0, len(apps))
	for _, app := range apps {
		names = append(names, app.Manifest.Name)
	}
	return names
}
