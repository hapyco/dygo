package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverRootFindsDygoMarkerFromNestedDirectory(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, MarkerFile), "name: test\n")
	nested := filepath.Join(root, "apps", "sales", "entities")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("MkdirAll(nested) error = %v", err)
	}

	discovered, err := DiscoverRoot(nested)
	if err != nil {
		t.Fatalf("DiscoverRoot() error = %v, want nil", err)
	}
	if discovered.Path != root {
		t.Fatalf("DiscoverRoot().Path = %q, want %q", discovered.Path, root)
	}
	if discovered.Marker != MarkerFile {
		t.Fatalf("DiscoverRoot().Marker = %q, want %q", discovered.Marker, MarkerFile)
	}
}

func TestDiscoverRootAcceptsFileStartPath(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, MarkerFile), "name: test\n")
	start := filepath.Join(root, "apps", "sales", "entities", "lead.yml")
	writeFile(t, start, "name: lead\n")

	discovered, err := DiscoverRoot(start)
	if err != nil {
		t.Fatalf("DiscoverRoot(file) error = %v, want nil", err)
	}
	if discovered.Path != root {
		t.Fatalf("DiscoverRoot(file).Path = %q, want %q", discovered.Path, root)
	}
}

func TestDiscoverRootFindsFrameworkRepositoryRoot(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module github.com/hapyco/dygo\n")
	writeFile(t, filepath.Join(root, MarkerFile), "name: dygo\n")
	for _, dir := range []string{"apps", filepath.Join("internal", "cli")} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatalf("MkdirAll(%s) error = %v", dir, err)
		}
	}

	discovered, err := DiscoverRoot(filepath.Join(root, "internal", "cli"))
	if err != nil {
		t.Fatalf("DiscoverRoot(framework) error = %v, want nil", err)
	}
	if discovered.Path != root {
		t.Fatalf("DiscoverRoot(framework).Path = %q, want %q", discovered.Path, root)
	}
	if discovered.Marker != "framework-repo" {
		t.Fatalf("DiscoverRoot(framework).Marker = %q, want framework-repo", discovered.Marker)
	}
}

func TestDiscoverRootDoesNotUseConfigAsFrameworkSignal(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module github.com/hapyco/dygo\n")
	for _, dir := range []string{"apps", "config"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatalf("MkdirAll(%s) error = %v", dir, err)
		}
	}

	_, err := DiscoverRoot(filepath.Join(root, "config"))
	if err == nil {
		t.Fatal("DiscoverRoot() error = nil, want missing root because config is not a project shape signal")
	}
	if !strings.Contains(err.Error(), "no dygo project root found") {
		t.Fatalf("DiscoverRoot() error = %q, want missing root", err.Error())
	}
}

func TestDiscoverRootRejectsMissingRoot(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "not", "dygo")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("MkdirAll(nested) error = %v", err)
	}

	_, err := DiscoverRoot(nested)
	if err == nil {
		t.Fatal("DiscoverRoot() error = nil, want missing root error")
	}
	if !strings.Contains(err.Error(), "no dygo project root found") {
		t.Fatalf("DiscoverRoot() error = %q, want missing root", err.Error())
	}
}

func TestDiscoverRootRejectsDirectoryMarker(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, MarkerFile), 0o755); err != nil {
		t.Fatalf("Mkdir(dygo.yml) error = %v", err)
	}

	_, err := DiscoverRoot(root)
	if err == nil {
		t.Fatal("DiscoverRoot() error = nil, want directory marker error")
	}
	if !strings.Contains(err.Error(), "must be a file") {
		t.Fatalf("DiscoverRoot() error = %q, want file marker error", err.Error())
	}
}

func writeFile(t *testing.T, path string, body string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
}
