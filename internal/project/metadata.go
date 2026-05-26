package project

import (
	"fmt"

	"github.com/hapyco/dygo/internal/app/manifest"
	"github.com/hapyco/dygo/internal/app/registry"
	"github.com/hapyco/dygo/internal/entity/catalog"
	"github.com/hapyco/dygo/internal/entity/fieldtype"
)

// Metadata is the validated app and Entity context for one dygo project.
type Metadata struct {
	Apps     []manifest.LoadedApp
	Entities []catalog.LoadedEntity
}

// LoadApps discovers and validates app manifests for a project root.
func LoadApps(root string) ([]manifest.LoadedApp, error) {
	apps, err := registry.New(root).Validate()
	if err != nil {
		return nil, fmt.Errorf("validate apps: %w", err)
	}
	return apps, nil
}

// LoadEntities validates Entity metadata for already loaded apps.
func LoadEntities(apps []manifest.LoadedApp) ([]catalog.LoadedEntity, error) {
	entities, err := catalog.New(apps, fieldtype.DefaultRegistry()).Validate()
	if err != nil {
		return nil, fmt.Errorf("validate entities: %w", err)
	}
	return entities, nil
}

// LoadMetadata loads the validated app and Entity metadata context for a project root.
func LoadMetadata(root string) (Metadata, error) {
	apps, err := LoadApps(root)
	if err != nil {
		return Metadata{}, err
	}
	entities, err := LoadEntities(apps)
	if err != nil {
		return Metadata{}, err
	}
	return Metadata{Apps: apps, Entities: entities}, nil
}
