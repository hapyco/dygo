// Package registry discovers and validates dygo apps from standard app roots.
package registry

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hapyco/dygo/internal/app/manifest"
	"github.com/hapyco/dygo/internal/reserved"
)

var defaultRoots = []string{filepath.Join(".dygo", "apps"), "apps"}

// Registry discovers dygo apps under a project or framework root.
type Registry struct {
	root string
}

// New returns an app Registry rooted at root.
func New(root string) Registry {
	return Registry{root: root}
}

// Discover returns apps from the default dygo app roots.
func (r Registry) Discover() ([]manifest.LoadedApp, error) {
	var apps []manifest.LoadedApp
	for _, appRoot := range defaultRoots {
		discovered, err := manifest.Discover(filepath.Join(r.root, appRoot))
		if err != nil {
			return nil, fmt.Errorf("discover apps from %s: %w", appRoot, err)
		}
		apps = append(apps, discovered...)
	}

	sort.SliceStable(apps, func(i, j int) bool {
		if apps[i].Manifest.Name == apps[j].Manifest.Name {
			return apps[i].ManifestPath < apps[j].ManifestPath
		}
		return apps[i].Manifest.Name < apps[j].Manifest.Name
	})

	return apps, nil
}

// Validate discovers apps and validates the discovered app set.
func (r Registry) Validate() ([]manifest.LoadedApp, error) {
	apps, err := r.Discover()
	if err != nil {
		return nil, err
	}
	if err := manifest.ValidateSet(apps); err != nil {
		return nil, err
	}
	if err := r.validateProjectAppNames(apps); err != nil {
		return nil, err
	}
	return apps, nil
}

func (r Registry) validateProjectAppNames(apps []manifest.LoadedApp) error {
	if r.isFrameworkRoot() {
		return nil
	}

	projectAppsRoot := filepath.Join(r.root, "apps")
	var problems []string
	for _, app := range apps {
		if !isInsideDir(app.Dir, projectAppsRoot) || !reserved.IsApp(app.Manifest.Name) {
			continue
		}
		problems = append(problems, fmt.Sprintf("%s: app name %q is reserved for framework-managed apps", app.ManifestPath, app.Manifest.Name))
	}
	if len(problems) > 0 {
		return manifest.ValidationError{Problems: problems}
	}
	return nil
}

func (r Registry) isFrameworkRoot() bool {
	data, err := os.ReadFile(filepath.Join(r.root, "go.mod"))
	if err != nil || !strings.Contains(string(data), "module github.com/hapyco/dygo") {
		return false
	}
	for _, path := range []string{"apps", filepath.Join("internal", "cli")} {
		info, err := os.Stat(filepath.Join(r.root, path))
		if err != nil || !info.IsDir() {
			return false
		}
	}
	return true
}

func isInsideDir(path string, dir string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absDir, absPath)
	if err != nil {
		return false
	}
	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
