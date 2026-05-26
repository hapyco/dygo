// Package catalog loads app-owned dygo Entity metadata.
package catalog

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hapyco/dygo/internal/app/manifest"
	"github.com/hapyco/dygo/internal/entity/fieldtype"
	"github.com/hapyco/dygo/internal/entity/schema"
	"github.com/hapyco/dygo/internal/reserved"
	"github.com/hapyco/dygo/internal/shape"
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

// RouteSlug returns the globally unique Studio route slug for routeable Entities.
func (e LoadedEntity) RouteSlug() string {
	if !e.HasRouteSlug() {
		return ""
	}
	return e.Entity.EffectiveRouteSlug()
}

// HasRouteSlug reports whether the Entity owns public root route space.
func (e LoadedEntity) HasRouteSlug() bool {
	return !e.IsCollection()
}

// IsCollection reports whether the Entity is a collection row Entity.
func (e LoadedEntity) IsCollection() bool {
	return e.Entity.IsCollection
}

// EntityKey returns a stable key for an app-owned Entity identity.
func EntityKey(appName string, entityName string) string {
	return appName + "\x00" + entityName
}

// ReservedRootRouteSlugs returns route slugs reserved by Studio and HTTP handlers.
func ReservedRootRouteSlugs() []string {
	return reserved.Slugs()
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
		path := filepath.Join(entitiesDir, entry.Name())
		if !entry.IsDir() {
			continue
		}
		if entry.Name() == shape.CollectionDir {
			discovered, err := c.discoverCollectionFolder(app, path)
			if err != nil {
				return nil, err
			}
			entities = append(entities, discovered...)
			continue
		}
		discovered, err := c.discoverEntityFolder(app, entitiesDir, entry.Name())
		if err != nil {
			return nil, err
		}
		entities = append(entities, discovered...)
	}

	return entities, nil
}

func (c Catalog) discoverEntityFolder(app manifest.LoadedApp, entitiesDir string, folderName string) ([]LoadedEntity, error) {
	folderPath := filepath.Join(entitiesDir, folderName)
	entries, err := os.ReadDir(folderPath)
	if err != nil {
		return nil, fmt.Errorf("read entity folder for app %q from %s: %w", app.Manifest.Name, folderPath, err)
	}

	var entityPath string
	hasYAML := false
	hasBundleFile := false
	var entities []LoadedEntity
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yml" {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, fmt.Errorf("stat entity for app %q from %s: %w", app.Manifest.Name, filepath.Join(folderPath, entry.Name()), err)
		}
		if !info.Mode().IsRegular() {
			continue
		}
		if isEntityBundleMetadataFile(entry.Name()) {
			hasBundleFile = true
			continue
		}
		hasYAML = true
		path := filepath.Join(folderPath, entry.Name())
		if entry.Name() != shape.EntityMetadataFile {
			return nil, fmt.Errorf("%s is not a valid Entity bundle file; Entity metadata must be %s", appRelativePath(app.Dir, path), appRelativePath(app.Dir, filepath.Join(folderPath, shape.EntityMetadataFile)))
		}
		if entityPath != "" {
			return nil, fmt.Errorf("Entity %q is defined twice. Use either %s or %s.", folderName, appRelativePath(app.Dir, entityPath), appRelativePath(app.Dir, path))
		}
		entityPath = path
	}

	if entityPath == "" {
		if hasYAML || hasBundleFile {
			return nil, fmt.Errorf("%s requires Entity metadata file %s", appRelativePath(app.Dir, folderPath), appRelativePath(app.Dir, filepath.Join(folderPath, shape.EntityMetadataFile)))
		}
		return entities, nil
	}
	entity, err := c.loadEntityFile(app, entityPath, false)
	if err != nil {
		return nil, err
	}
	entities = append(entities, entity)

	return entities, nil
}

func (c Catalog) discoverCollectionFolder(app manifest.LoadedApp, folderPath string) ([]LoadedEntity, error) {
	entries, err := os.ReadDir(folderPath)
	if err != nil {
		return nil, fmt.Errorf("read collection entities for app %q from %s: %w", app.Manifest.Name, folderPath, err)
	}

	var entities []LoadedEntity
	for _, entry := range entries {
		path := filepath.Join(folderPath, entry.Name())
		if entry.IsDir() {
			entityPath := filepath.Join(path, shape.EntityMetadataFile)
			entity, err := c.loadEntityFile(app, entityPath, true)
			if err != nil {
				return nil, err
			}
			entities = append(entities, entity)
			continue
		}
		if filepath.Ext(entry.Name()) != ".yml" {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, fmt.Errorf("stat collection entity for app %q from %s: %w", app.Manifest.Name, path, err)
		}
		if !info.Mode().IsRegular() {
			continue
		}
		entity, err := c.loadEntityFile(app, path, true)
		if err != nil {
			return nil, err
		}
		entities = append(entities, entity)
	}
	return entities, nil
}

func isEntityBundleMetadataFile(name string) bool {
	switch name {
	case shape.EntityFixturesFile, shape.EntityPermissionsFile, shape.EntityViewsFile:
		return true
	default:
		return false
	}
}

func (c Catalog) loadEntityFile(app manifest.LoadedApp, path string, isCollection bool) (LoadedEntity, error) {
	info, err := os.Stat(path)
	if err != nil {
		return LoadedEntity{}, fmt.Errorf("stat entity for app %q from %s: %w", app.Manifest.Name, path, err)
	}
	if !info.Mode().IsRegular() {
		return LoadedEntity{}, nil
	}
	entity, err := schema.LoadFileWithOptions(path, c.fieldTypes, schema.LoadOptions{IsCollection: isCollection})
	if err != nil {
		return LoadedEntity{}, fmt.Errorf("load entity for app %q from %s: %w", app.Manifest.Name, path, err)
	}
	return LoadedEntity{
		AppName: app.Manifest.Name,
		AppDir:  app.Dir,
		Path:    path,
		Entity:  entity,
	}, nil
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

		if entity.IsCollection() && strings.TrimSpace(entity.Entity.Route.Slug) != "" {
			problems = append(problems, entityDiagnostic(entity, fmt.Sprintf("collection Entity %q cannot define route.slug; collection Entities are not routeable", entity.Entity.Name)))
		}
		if entity.HasRouteSlug() {
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
			if field.Type != "link" && field.Type != "collection" {
				continue
			}
			targetName := field.Options.Entity
			if targetName == "" {
				continue
			}

			target, err := targets.resolve(entity, field.Options.App, targetName)
			if err != nil {
				*problems = append(*problems, fieldDiagnostic(entity, field, err.Error()))
				continue
			}
			if target.Entity.IsSingle {
				if field.Type == "collection" {
					*problems = append(*problems, fieldDiagnostic(entity, field, fmt.Sprintf("targets single Entity %q, but collection fields must target a collection Entity", target.Entity.Name)))
				} else {
					*problems = append(*problems, fieldDiagnostic(entity, field, fmt.Sprintf("links to single Entity %q; single Entities cannot be link targets", target.Entity.Name)))
				}
				continue
			}
			if field.Type == "link" && target.IsCollection() {
				*problems = append(*problems, fieldDiagnostic(entity, field, fmt.Sprintf("links to collection Entity %q; collection Entities cannot be link targets", target.Entity.Name)))
				continue
			}
			if field.Type == "collection" {
				if !target.IsCollection() {
					*problems = append(*problems, fieldDiagnostic(entity, field, fmt.Sprintf("targets Entity %q, but collection fields must target a collection Entity", target.Entity.Name)))
					continue
				}
			}
		}
	}
}

func validateHookFiles(apps []manifest.LoadedApp, entities []LoadedEntity, problems *[]string) error {
	for _, entity := range entities {
		if entity.IsCollection() {
			continue
		}
		hookPath := filepath.Join(filepath.Dir(entity.Path), shape.EntityHooksFile)
		info, err := os.Stat(hookPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("stat hook file for app %q Entity %q from %s: %w", entity.AppName, entity.Entity.Name, hookPath, err)
		}
		if info.IsDir() || !info.Mode().IsRegular() {
			*problems = append(*problems, hookDiagnostic(entity.AppName, hookPath, "must be a regular file"))
		}
	}

	return nil
}

func isReservedRootSlug(slug string) bool {
	return reserved.IsSlug(slug)
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

func appRelativePath(appDir string, path string) string {
	relative, err := filepath.Rel(appDir, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(relative)
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

func hookDiagnostic(appName string, path string, message string) string {
	return fmt.Sprintf("%s: app %q hook file %s", location(path, 0), appName, message)
}

func location(path string, line int) string {
	if line == 0 {
		return path
	}
	return fmt.Sprintf("%s:%d", path, line)
}
