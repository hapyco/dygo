package manifest

import (
	"path/filepath"
	"testing"
)

func TestFrameworkAppsDiscoverCore(t *testing.T) {
	appsRoot := filepath.Join("..", "..", "..", "apps")
	apps, err := Discover(appsRoot)
	if err != nil {
		t.Fatalf("Discover(%q) error = %v, want nil", appsRoot, err)
	}
	if err := ValidateSet(apps); err != nil {
		t.Fatalf("ValidateSet(framework apps) error = %v, want nil", err)
	}

	for _, app := range apps {
		if app.Manifest.Name == "core" {
			if app.Manifest.Version == "" {
				t.Fatal("core app version is empty")
			}
			return
		}
	}

	t.Fatalf("Discover(%q) did not find core app", appsRoot)
}
