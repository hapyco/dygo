// Package catalog loads app-owned dygo Entity metadata.
package catalog

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dygo-dev/dygo/internal/app/manifest"
	"github.com/dygo-dev/dygo/internal/entity/fieldtype"
	"github.com/dygo-dev/dygo/internal/entity/schema"
)

// LoadedEntity is one Entity loaded from an owning app.
type LoadedEntity struct {
	AppName string
	AppDir  string
	Path    string
	Entity  schema.Entity
}

// Catalog loads Entity metadata from a set of discovered apps.
type Catalog struct {
	apps       []manifest.LoadedApp
	fieldTypes fieldtype.Registry
}

// New returns an Entity Catalog for the given loaded apps and field type registry.
func New(apps []manifest.LoadedApp, fieldTypes fieldtype.Registry) Catalog {
	copied := make([]manifest.LoadedApp, len(apps))
	copy(copied, apps)

	return Catalog{
		apps:       copied,
		fieldTypes: fieldTypes,
	}
}

// Discover loads Entity metadata files from app-owned entities directories.
func (c Catalog) Discover() ([]LoadedEntity, error) {
	var entities []LoadedEntity
	for _, app := range c.apps {
		discovered, err := c.discoverApp(app)
		if err != nil {
			return nil, err
		}
		entities = append(entities, discovered...)
	}

	sortEntities(entities)
	return entities, nil
}

// Validate discovers entities and validates app-level Entity catalog rules.
func (c Catalog) Validate() ([]LoadedEntity, error) {
	entities, err := c.Discover()
	if err != nil {
		return nil, err
	}
	if err := validateCatalog(entities); err != nil {
		return nil, err
	}
	return entities, nil
}

func (c Catalog) discoverApp(app manifest.LoadedApp) ([]LoadedEntity, error) {
	entitiesDir := filepath.Join(app.Dir, filepath.FromSlash(app.Manifest.Paths.WithDefaults().Entities))
	entries, err := os.ReadDir(entitiesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read entities for app %q from %s: %w", app.Manifest.Name, entitiesDir, err)
	}

	var entities []LoadedEntity
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yml" {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			return nil, fmt.Errorf("stat entity for app %q from %s: %w", app.Manifest.Name, filepath.Join(entitiesDir, entry.Name()), err)
		}
		if !info.Mode().IsRegular() {
			continue
		}

		path := filepath.Join(entitiesDir, entry.Name())
		entity, err := schema.LoadFile(path, c.fieldTypes)
		if err != nil {
			return nil, fmt.Errorf("load entity for app %q from %s: %w", app.Manifest.Name, path, err)
		}
		entities = append(entities, LoadedEntity{
			AppName: app.Manifest.Name,
			AppDir:  app.Dir,
			Path:    path,
			Entity:  entity,
		})
	}

	return entities, nil
}

func validateCatalog(entities []LoadedEntity) error {
	var problems []string
	seen := map[string]string{}
	for _, entity := range entities {
		key := entity.AppName + "\x00" + entity.Entity.Name
		if previousPath, ok := seen[key]; ok {
			problems = append(problems, entityDiagnostic(entity, fmt.Sprintf("app %q has duplicate entity %q in %s and %s", entity.AppName, entity.Entity.Name, previousPath, entity.Path)))
		} else {
			seen[key] = entity.Path
		}
	}

	validateFieldTargets(entities, &problems)

	if len(problems) > 0 {
		return ValidationError{Problems: problems}
	}
	return nil
}

func validateFieldTargets(entities []LoadedEntity, problems *[]string) {
	targets := map[string][]LoadedEntity{}
	for _, entity := range entities {
		targets[entity.Entity.Name] = append(targets[entity.Entity.Name], entity)
	}

	for _, entity := range entities {
		for _, field := range entity.Entity.Fields {
			if field.Type != "link" && field.Type != "child-table" {
				continue
			}
			targetName := field.Options.Entity
			if targetName == "" {
				continue
			}

			matches := targets[targetName]
			if len(matches) == 0 {
				*problems = append(*problems, fieldDiagnostic(entity, field, fmt.Sprintf("references unknown entity target %q", targetName)))
				continue
			}
			if len(matches) > 1 {
				*problems = append(*problems, fieldDiagnostic(entity, field, fmt.Sprintf("references ambiguous entity target %q found in %s", targetName, targetLocations(matches))))
			}
		}
	}
}

func sortEntities(entities []LoadedEntity) {
	sort.SliceStable(entities, func(i, j int) bool {
		if entities[i].AppName != entities[j].AppName {
			return entities[i].AppName < entities[j].AppName
		}
		if entities[i].Entity.Name != entities[j].Entity.Name {
			return entities[i].Entity.Name < entities[j].Entity.Name
		}
		return entities[i].Path < entities[j].Path
	})
}

// ValidationError reports one or more Entity catalog validation problems.
type ValidationError struct {
	Problems []string
}

func (e ValidationError) Error() string {
	return "entity catalog validation failed: " + strings.Join(e.Problems, "; ")
}

func entityDiagnostic(entity LoadedEntity, message string) string {
	return fmt.Sprintf("%s: %s", location(entity.Path, entity.Entity.Line), message)
}

func fieldDiagnostic(entity LoadedEntity, field schema.Field, message string) string {
	line := field.Line
	if line == 0 {
		line = entity.Entity.Line
	}
	fieldName := field.Name
	if fieldName == "" {
		fieldName = "<missing>"
	}
	return fmt.Sprintf("%s: app %q entity %q field %q: %s", location(entity.Path, line), entity.AppName, entity.Entity.Name, fieldName, message)
}

func location(path string, line int) string {
	if line == 0 {
		return path
	}
	return fmt.Sprintf("%s:%d", path, line)
}

func targetLocations(entities []LoadedEntity) string {
	locations := make([]string, 0, len(entities))
	for _, entity := range entities {
		locations = append(locations, fmt.Sprintf("%s/%s at %s", entity.AppName, entity.Entity.Name, location(entity.Path, entity.Entity.Line)))
	}
	sort.Strings(locations)
	return strings.Join(locations, ", ")
}
