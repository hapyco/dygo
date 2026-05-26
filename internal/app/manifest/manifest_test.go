package manifest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadFile(t *testing.T) {
	t.Parallel()

	path := writeManifest(t, t.TempDir(), "app.yml", `
name: dygo-crm
label: CRM
version: 0.1.0
description: Customer relationship management
dependencies:
  - core
`)

	app, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile() error = %v, want nil", err)
	}

	if app.Name != "dygo-crm" {
		t.Fatalf("LoadFile().Name = %q, want %q", app.Name, "dygo-crm")
	}
	if app.Paths.Entities != "entities" {
		t.Fatalf("LoadFile().Paths.Entities = %q, want default entities", app.Paths.Entities)
	}
	if len(app.Dependencies) != 1 || app.Dependencies[0] != "core" {
		t.Fatalf("LoadFile().Dependencies = %#v, want [core]", app.Dependencies)
	}
}

func TestLoadFileWithExplicitPaths(t *testing.T) {
	t.Parallel()

	path := writeManifest(t, t.TempDir(), "app.yml", `
name: dygo-crm
label: CRM
version: 0.1.0
paths:
  entities: metadata/entities
  patches: data/patches
  docs: docs
  assets: assets
`)

	app, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile() error = %v, want nil", err)
	}
	if app.Paths.Entities != "metadata/entities" {
		t.Fatalf("LoadFile().Paths.Entities = %q, want explicit path", app.Paths.Entities)
	}
	if app.Paths.Patches != "data/patches" {
		t.Fatalf("LoadFile().Paths.Patches = %q, want explicit path", app.Paths.Patches)
	}
}

func TestLoadFileRejectsOldAppLevelEntityOwnedPaths(t *testing.T) {
	t.Parallel()

	path := writeManifest(t, t.TempDir(), "app.yml", `
name: dygo-crm
label: CRM
version: 0.1.0
paths:
  hooks: hooks
  fixtures: fixtures
  permissions: permissions
`)

	_, err := LoadFile(path)
	if err == nil {
		t.Fatal("LoadFile() error = nil, want old app-level path fields rejected")
	}
	for _, want := range []string{`field hooks not found`, `field fixtures not found`, `field permissions not found`} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("LoadFile() error = %q, want substring %q", err.Error(), want)
		}
	}
}

func TestLoadAppDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeManifest(t, dir, Filename, `
name: core
label: Core
version: 0.1.0
`)

	app, err := LoadAppDir(dir)
	if err != nil {
		t.Fatalf("LoadAppDir() error = %v, want nil", err)
	}
	if app.Dir != dir {
		t.Fatalf("LoadAppDir().Dir = %q, want %q", app.Dir, dir)
	}
	if app.ManifestPath != filepath.Join(dir, Filename) {
		t.Fatalf("LoadAppDir().ManifestPath = %q, want app.yml path", app.ManifestPath)
	}
	if app.Manifest.Name != "core" {
		t.Fatalf("LoadAppDir().Manifest.Name = %q, want core", app.Manifest.Name)
	}
}

func TestDiscover(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeManifest(t, filepath.Join(root, "z-app"), Filename, `
name: beta
label: Beta
version: 0.1.0
`)
	writeManifest(t, filepath.Join(root, "a-app"), Filename, `
name: alpha
label: Alpha
version: 0.1.0
`)
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("ignored"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.Mkdir(filepath.Join(root, "without-manifest"), 0o755); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}

	apps, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover() error = %v, want nil", err)
	}
	if got := appNames(apps); strings.Join(got, ",") != "alpha,beta" {
		t.Fatalf("Discover() app names = %#v, want [alpha beta]", got)
	}
}

func TestDiscoverMissingRoot(t *testing.T) {
	t.Parallel()

	apps, err := Discover(filepath.Join(t.TempDir(), "missing"))
	if err != nil {
		t.Fatalf("Discover() error = %v, want nil", err)
	}
	if len(apps) != 0 {
		t.Fatalf("Discover() len = %d, want 0", len(apps))
	}
}

func TestLoadFileRejectsInvalidManifests(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		body      string
		wantError string
	}{
		{
			name: "missing required fields",
			body: `
description: Missing fields
`,
			wantError: "name is required",
		},
		{
			name: "invalid app name",
			body: `
name: dygo_crm
label: CRM
version: 0.1.0
`,
			wantError: "must be kebab-case",
		},
		{
			name: "invalid version",
			body: `
name: dygo-crm
label: CRM
version: 1.0
`,
			wantError: "semantic version",
		},
		{
			name: "unknown field",
			body: `
name: dygo-crm
label: CRM
version: 0.1.0
unknown: true
`,
			wantError: "field unknown not found",
		},
		{
			name: "duplicate yaml key",
			body: `
name: dygo-crm
name: duplicate
label: CRM
version: 0.1.0
`,
			wantError: "duplicate key",
		},
		{
			name: "invalid dependency name",
			body: `
name: dygo-crm
label: CRM
version: 0.1.0
dependencies:
  - Core
`,
			wantError: "dependency",
		},
		{
			name: "duplicate dependency",
			body: `
name: dygo-crm
label: CRM
version: 0.1.0
dependencies:
  - core
  - core
`,
			wantError: "duplicate dependency",
		},
		{
			name: "absolute path",
			body: `
name: dygo-crm
label: CRM
version: 0.1.0
paths:
  entities: /tmp/entities
`,
			wantError: "must be relative",
		},
		{
			name: "unclean path",
			body: `
name: dygo-crm
label: CRM
version: 0.1.0
paths:
  entities: entities/../entities
`,
			wantError: "must be clean",
		},
		{
			name: "traversal path",
			body: `
name: dygo-crm
label: CRM
version: 0.1.0
paths:
  entities: ../entities
`,
			wantError: "must stay inside",
		},
		{
			name: "backslash path",
			body: `
name: dygo-crm
label: CRM
version: 0.1.0
paths:
  entities: metadata\entities
`,
			wantError: "forward slashes",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := writeManifest(t, t.TempDir(), "app.yml", tt.body)
			_, err := LoadFile(path)
			if err == nil {
				t.Fatal("LoadFile() error = nil, want error")
			}
			if !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("LoadFile() error = %q, want substring %q", err.Error(), tt.wantError)
			}
		})
	}
}

func TestValidateSet(t *testing.T) {
	t.Parallel()

	apps := []LoadedApp{
		{
			ManifestPath: "/apps/core/app.yml",
			Manifest: Manifest{
				Name:    "core",
				Label:   "Core",
				Version: "0.1.0",
				Paths:   DefaultPaths(),
			},
		},
		{
			ManifestPath: "/apps/dygo-crm/app.yml",
			Manifest: Manifest{
				Name:         "dygo-crm",
				Label:        "CRM",
				Version:      "0.1.0",
				Dependencies: []string{"core"},
				Paths:        DefaultPaths(),
			},
		},
	}

	if err := ValidateSet(apps); err != nil {
		t.Fatalf("ValidateSet() error = %v, want nil", err)
	}
}

func TestValidateSetRejectsInvalidSets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		apps      []LoadedApp
		wantError string
	}{
		{
			name: "duplicate app",
			apps: []LoadedApp{
				{
					ManifestPath: "/apps/one/app.yml",
					Manifest:     Manifest{Name: "core", Label: "Core", Version: "0.1.0", Paths: DefaultPaths()},
				},
				{
					ManifestPath: "/apps/two/app.yml",
					Manifest:     Manifest{Name: "core", Label: "Core Again", Version: "0.1.0", Paths: DefaultPaths()},
				},
			},
			wantError: "duplicate app",
		},
		{
			name: "unresolved dependency",
			apps: []LoadedApp{
				{
					ManifestPath: "/apps/crm/app.yml",
					Manifest: Manifest{
						Name:         "dygo-crm",
						Label:        "CRM",
						Version:      "0.1.0",
						Dependencies: []string{"core"},
						Paths:        DefaultPaths(),
					},
				},
			},
			wantError: "unknown app",
		},
		{
			name: "self dependency",
			apps: []LoadedApp{
				{
					ManifestPath: "/apps/core/app.yml",
					Manifest: Manifest{
						Name:         "core",
						Label:        "Core",
						Version:      "0.1.0",
						Dependencies: []string{"core"},
						Paths:        DefaultPaths(),
					},
				},
			},
			wantError: "cannot depend on itself",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateSet(tt.apps)
			if err == nil {
				t.Fatal("ValidateSet() error = nil, want error")
			}
			if !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("ValidateSet() error = %q, want substring %q", err.Error(), tt.wantError)
			}
		})
	}
}

func writeManifest(t *testing.T, dir string, name string, body string) string {
	t.Helper()

	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(strings.TrimSpace(body)+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}

func appNames(apps []LoadedApp) []string {
	names := make([]string, 0, len(apps))
	for _, app := range apps {
		names = append(names, app.Manifest.Name)
	}
	return names
}
