package registry

import (
	"path/filepath"
	"testing"
)

func TestRepositoryAppsValidate(t *testing.T) {
	appsRoot := filepath.Join("..", "..", "..")
	apps, err := New(appsRoot).Validate()
	if err != nil {
		t.Fatalf("Validate(repository apps) error = %v, want nil", err)
	}

	wantApps := map[string]struct{}{
		"core":   {},
		"studio": {},
	}
	for _, app := range apps {
		if _, ok := wantApps[app.Manifest.Name]; !ok {
			continue
		}
		delete(wantApps, app.Manifest.Name)
	}
	if len(wantApps) > 0 {
		t.Fatalf("Validate(repository apps) missing built-in apps: %#v", wantApps)
	}
}
