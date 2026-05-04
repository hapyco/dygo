// Package registry discovers and validates dygo apps from standard app roots.
package registry

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/dygo-dev/dygo/internal/app/manifest"
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
	return apps, nil
}
