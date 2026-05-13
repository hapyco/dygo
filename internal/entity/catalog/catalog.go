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

// Key returns the stable internal Entity identity.
func (e LoadedEntity) Key() string {
	return EntityKey(e.AppName, e.Entity.Name)
}

// RouteSlug returns the globally unique Studio route slug for the Entity.
func (e LoadedEntity) RouteSlug() string {
	return e.Entity.EffectiveRouteSlug()
}

// EntityKey returns a stable key for an app-owned Entity identity.
func EntityKey(appName string, entityName string) string {
	return appName + "\x00" + entityName
}

var rootReservedSlugs = map[string]struct{}{
	"api":    {},
	"assets": {},
	"health": {},
	"login":  {},
	"logout": {},
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
	if err := validateCatalog(c.apps, entities); err != nil {
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

func validateCatalog(apps []manifest.LoadedApp, entities []LoadedEntity) error {
	var problems []string
	seenIdentities := map[string]LoadedEntity{}
	seenRouteSlugs := map[string]LoadedEntity{}
	for _, entity := range entities {
		if previous, ok := seenIdentities[entity.Key()]; ok {
			problems = append(problems, entityDiagnostic(entity, fmt.Sprintf("app %q entity %q duplicates Entity identity from %s", entity.AppName, entity.Entity.Name, location(previous.Path, previous.Entity.Line))))
		} else {
			seenIdentities[entity.Key()] = entity
		}

		routeSlug := entity.RouteSlug()
		if isReservedRootSlug(routeSlug) {
			problems = append(problems, entityDiagnostic(entity, fmt.Sprintf("app %q entity %q uses reserved root route slug %q; set route.slug to a non-reserved stable slug", entity.AppName, entity.Entity.Name, routeSlug)))
		}
		if previous, ok := seenRouteSlugs[routeSlug]; ok {
			problems = append(problems, entityDiagnostic(entity, fmt.Sprintf("app %q entity %q route slug %q conflicts with app %q entity %q at %s; set route.slug to a stable unique slug such as %q", entity.AppName, entity.Entity.Name, routeSlug, previous.AppName, previous.Entity.Name, location(previous.Path, previous.Entity.Line), suggestedRouteSlug(entity))))
		} else {
			seenRouteSlugs[routeSlug] = entity
		}
	}

	validateFieldTargets(entities, &problems)
	if err := validateHookFiles(apps, entities, &problems); err != nil {
		return err
	}

	if len(problems) > 0 {
		return ValidationError{Problems: problems}
	}
	return nil
}

func validateFieldTargets(entities []LoadedEntity, problems *[]string) {
	targets := newTargetIndex(entities)

	for _, entity := range entities {
		for _, field := range entity.Entity.Fields {
			if field.Type != "link" && field.Type != "child-table" {
				continue
			}
			targetName := field.Options.Entity
			if targetName == "" {
				continue
			}

			if _, err := targets.resolve(entity, field.Options.App, targetName); err != nil {
				*problems = append(*problems, fieldDiagnostic(entity, field, err.Error()))
				continue
			}
		}
	}
}

func validateHookFiles(apps []manifest.LoadedApp, entities []LoadedEntity, problems *[]string) error {
	entitiesByApp := map[string]map[string]struct{}{}
	for _, entity := range entities {
		if entitiesByApp[entity.AppName] == nil {
			entitiesByApp[entity.AppName] = map[string]struct{}{}
		}
		entitiesByApp[entity.AppName][entity.Entity.Name] = struct{}{}
	}

	for _, app := range apps {
		hooksDir := filepath.Join(app.Dir, filepath.FromSlash(app.Manifest.Paths.WithDefaults().Hooks))
		entries, err := os.ReadDir(hooksDir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("read hooks for app %q from %s: %w", app.Manifest.Name, hooksDir, err)
		}
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".go" {
				continue
			}
			info, err := entry.Info()
			if err != nil {
				return fmt.Errorf("stat hook for app %q from %s: %w", app.Manifest.Name, filepath.Join(hooksDir, entry.Name()), err)
			}
			if !info.Mode().IsRegular() {
				continue
			}
			if entry.Name() == "register.go" || strings.HasSuffix(entry.Name(), "_test.go") {
				continue
			}
			entityName := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
			if _, ok := entitiesByApp[app.Manifest.Name][entityName]; ok {
				continue
			}
			*problems = append(*problems, hookDiagnostic(app, filepath.Join(hooksDir, entry.Name()), entityName))
		}
	}
	return nil
}

func isReservedRootSlug(slug string) bool {
	_, ok := rootReservedSlugs[strings.ToLower(strings.TrimSpace(slug))]
	return ok
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

type targetIndex struct {
	byIdentity map[string]LoadedEntity
	byName     map[string][]LoadedEntity
}

func newTargetIndex(entities []LoadedEntity) targetIndex {
	index := targetIndex{
		byIdentity: map[string]LoadedEntity{},
		byName:     map[string][]LoadedEntity{},
	}
	for _, entity := range entities {
		index.byIdentity[entity.Key()] = entity
		index.byName[entity.Entity.Name] = append(index.byName[entity.Entity.Name], entity)
	}
	return index
}

func (i targetIndex) resolve(owner LoadedEntity, appName string, entityName string) (LoadedEntity, error) {
	if strings.TrimSpace(appName) != "" {
		target, ok := i.byIdentity[EntityKey(appName, entityName)]
		if !ok {
			return LoadedEntity{}, fmt.Errorf("references unknown entity target %q in app %q", entityName, appName)
		}
		return target, nil
	}
	if target, ok := i.byIdentity[EntityKey(owner.AppName, entityName)]; ok {
		return target, nil
	}
	matches := i.byName[entityName]
	switch len(matches) {
	case 0:
		return LoadedEntity{}, fmt.Errorf("references unknown entity target %q", entityName)
	case 1:
		return matches[0], nil
	default:
		apps := make([]string, 0, len(matches))
		for _, match := range matches {
			apps = append(apps, match.AppName)
		}
		sort.Strings(apps)
		return LoadedEntity{}, fmt.Errorf("references ambiguous entity target %q in apps %s; set options.app", entityName, strings.Join(apps, ", "))
	}
}

func suggestedRouteSlug(entity LoadedEntity) string {
	if strings.TrimSpace(entity.AppName) == "" {
		return entity.Entity.Name
	}
	return entity.AppName + "-" + entity.Entity.Name
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

func hookDiagnostic(app manifest.LoadedApp, path string, entityName string) string {
	return fmt.Sprintf("%s: app %q hook file %q does not match a known Entity name in the same app", location(path, 0), app.Manifest.Name, entityName)
}

func location(path string, line int) string {
	if line == 0 {
		return path
	}
	return fmt.Sprintf("%s:%d", path, line)
}
