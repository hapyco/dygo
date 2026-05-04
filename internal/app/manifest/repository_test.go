package manifest

import (
	"path/filepath"
	"testing"
)

func TestFrameworkAppsDiscoverBuiltInApps(t *testing.T) {
	appsRoot := filepath.Join("..", "..", "..", "apps")
	apps, err := Discover(appsRoot)
	if err != nil {
		t.Fatalf("Discover(%q) error = %v, want nil", appsRoot, err)
	}
	if err := ValidateSet(apps); err != nil {
		t.Fatalf("ValidateSet(framework apps) error = %v, want nil", err)
	}

	wantApps := map[string]struct{}{
		"core":   {},
		"studio": {},
	}
	for _, app := range apps {
		if _, ok := wantApps[app.Manifest.Name]; !ok {
			continue
		}
		if app.Manifest.Version == "" {
			t.Fatalf("%s app version is empty", app.Manifest.Name)
		}
		delete(wantApps, app.Manifest.Name)
	}

	if len(wantApps) > 0 {
		t.Fatalf("Discover(%q) missing built-in apps: %#v", appsRoot, wantApps)
	}
}
