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
	"github.com/dygo-dev/dygo/internal/reserved"
)

// LoadedEntity is one Entity loaded from an owning app.
type LoadedEntity struct {
	AppName          string
	AppDir           string
	Path             string
	Entity           schema.Entity
	CollectionParent string
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

// IsCollection reports whether the Entity is an owned collection row Entity.
func (e LoadedEntity) IsCollection() bool {
	return strings.TrimSpace(e.CollectionParent) != ""
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
	rootFiles := map[string]string{}
	for _, entry := range entries {
		path := filepath.Join(entitiesDir, entry.Name())
		if entry.IsDir() {
			discovered, err := c.discoverEntityFolder(app, entitiesDir, entry.Name(), rootFiles)
			if err != nil {
				return nil, err
			}
			entities = append(entities, discovered...)
			continue
		}
		if filepath.Ext(entry.Name()) != ".yml" {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, fmt.Errorf("stat entity for app %q from %s: %w", app.Manifest.Name, path, err)
		}
		if !info.Mode().IsRegular() {
			continue
		}
		entity, err := c.loadEntityFile(app, path, "")
		if err != nil {
			return nil, err
		}
		rootFiles[entity.Entity.Name] = path
		entities = append(entities, entity)
	}

	return entities, nil
}

func (c Catalog) discoverEntityFolder(app manifest.LoadedApp, entitiesDir string, folderName string, rootFiles map[string]string) ([]LoadedEntity, error) {
	folderPath := filepath.Join(entitiesDir, folderName)
	parentPath := filepath.Join(folderPath, folderName+".yml")
	entries, err := os.ReadDir(folderPath)
	if err != nil {
		return nil, fmt.Errorf("read entity folder for app %q from %s: %w", app.Manifest.Name, folderPath, err)
	}

	hasEntityFiles := false
	hasParent := false
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
		hasEntityFiles = true
		name := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		path := filepath.Join(folderPath, entry.Name())
		if name == folderName {
			hasParent = true
			if rootPath, ok := rootFiles[name]; ok {
				return nil, fmt.Errorf("Entity %q is defined twice. Use either %s or %s.", name, appRelativePath(app.Dir, rootPath), appRelativePath(app.Dir, path))
			}
			entity, err := c.loadEntityFile(app, path, "")
			if err != nil {
				return nil, err
			}
			entities = append(entities, entity)
			continue
		}
		entity, err := c.loadEntityFile(app, path, folderName)
		if err != nil {
			return nil, err
		}
		entities = append(entities, entity)
	}

	if hasEntityFiles && !hasParent {
		for _, entity := range entities {
			if entity.IsCollection() {
				return nil, fmt.Errorf("%s requires parent Entity file %s", appRelativePath(app.Dir, entity.Path), appRelativePath(app.Dir, parentPath))
			}
		}
		return nil, fmt.Errorf("%s requires parent Entity file %s", appRelativePath(app.Dir, folderPath), appRelativePath(app.Dir, parentPath))
	}

	return entities, nil
}

func (c Catalog) loadEntityFile(app manifest.LoadedApp, path string, collectionParent string) (LoadedEntity, error) {
	info, err := os.Stat(path)
	if err != nil {
		return LoadedEntity{}, fmt.Errorf("stat entity for app %q from %s: %w", app.Manifest.Name, path, err)
	}
	if !info.Mode().IsRegular() {
		return LoadedEntity{}, nil
	}
	entity, err := schema.LoadFile(path, c.fieldTypes)
	if err != nil {
		return LoadedEntity{}, fmt.Errorf("load entity for app %q from %s: %w", app.Manifest.Name, path, err)
	}
	entity.IsCollection = collectionParent != ""
	return LoadedEntity{
		AppName:          app.Manifest.Name,
		AppDir:           app.Dir,
		Path:             path,
		Entity:           entity,
		CollectionParent: collectionParent,
	}, nil
}

func validateCatalog(apps []manifest.LoadedApp, entities []LoadedEntity) error {
	var problems []string
	seenIdentities := map[string]LoadedEntity{}
	seenRouteSlugs := map[string]LoadedEntity{}
	for _, entity := range entities {
		if previous, ok := seenIdentities[entity.Key()]; ok {
			if simpleFolderDuplicate(entity, previous) {
				simplePath, folderPath := duplicateParentPaths(entity, previous)
				problems = append(problems, fmt.Sprintf("Entity %q is defined twice. Use either %s or %s.", entity.Entity.Name, simplePath, folderPath))
			} else {
				problems = append(problems, entityDiagnostic(entity, fmt.Sprintf("app %q entity %q duplicates Entity identity from %s", entity.AppName, entity.Entity.Name, location(previous.Path, previous.Entity.Line))))
			}
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
	collectionRefs := map[string][]collectionReference{}

	for _, entity := range entities {
		for _, field := range entity.Entity.Fields {
			if field.Type != "link" && field.Type != "collection" {
				continue
			}
			targetName := field.Options.Entity
			if targetName == "" {
				continue
			}
			if field.Type == "collection" && strings.TrimSpace(field.Options.App) != "" && field.Options.App != entity.AppName {
				*problems = append(*problems, fieldDiagnostic(entity, field, "collection fields must target a collection Entity in the same app"))
				continue
			}

			target, err := targets.resolve(entity, field.Options.App, targetName)
			if err != nil {
				*problems = append(*problems, fieldDiagnostic(entity, field, err.Error()))
				continue
			}
			if target.Entity.IsSingle {
				if field.Type == "collection" {
					*problems = append(*problems, fieldDiagnostic(entity, field, fmt.Sprintf("targets single Entity %q, but collection fields must target a collection Entity defined in this Entity folder", target.Entity.Name)))
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
					*problems = append(*problems, fieldDiagnostic(entity, field, fmt.Sprintf("targets Entity %q, but collection fields must target a collection Entity defined in this Entity folder", target.Entity.Name)))
					continue
				}
				if target.AppName != entity.AppName || target.CollectionParent != entity.Entity.Name {
					*problems = append(*problems, fieldDiagnostic(entity, field, fmt.Sprintf("targets collection Entity %q owned by %q; expected collection Entity owned by %q", target.Entity.Name, target.CollectionParent, entity.Entity.Name)))
					continue
				}
				collectionRefs[target.Key()] = append(collectionRefs[target.Key()], collectionReference{Owner: entity, Field: field})
			}
		}
	}

	for _, entity := range entities {
		if !entity.IsCollection() {
			continue
		}
		refs := collectionRefs[entity.Key()]
		switch len(refs) {
		case 0:
			*problems = append(*problems, fmt.Sprintf("collection Entity %q is defined under %s but is not used by any collection field in %s", entity.Entity.Name, entity.CollectionParent, parentEntityPath(entity)))
		case 1:
			continue
		default:
			*problems = append(*problems, fmt.Sprintf("collection Entity %q is referenced by more than one collection field in %s", entity.Entity.Name, parentEntityPath(entity)))
		}
	}
}

type collectionReference struct {
	Owner LoadedEntity
	Field schema.Field
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

func simpleFolderDuplicate(left LoadedEntity, right LoadedEntity) bool {
	return isSimpleFolderPair(left, right) || isSimpleFolderPair(right, left)
}

func isSimpleFolderPair(simple LoadedEntity, folder LoadedEntity) bool {
	return !simple.IsCollection() &&
		!folder.IsCollection() &&
		filepath.Base(simple.Path) == simple.Entity.Name+".yml" &&
		filepath.Base(filepath.Dir(folder.Path)) == folder.Entity.Name &&
		filepath.Base(folder.Path) == folder.Entity.Name+".yml"
}

func duplicateParentPaths(left LoadedEntity, right LoadedEntity) (string, string) {
	if isFolderParent(left) {
		return appRelativePath(right.AppDir, right.Path), appRelativePath(left.AppDir, left.Path)
	}
	return appRelativePath(left.AppDir, left.Path), appRelativePath(right.AppDir, right.Path)
}

func isFolderParent(entity LoadedEntity) bool {
	return !entity.IsCollection() &&
		filepath.Base(filepath.Dir(entity.Path)) == entity.Entity.Name &&
		filepath.Base(entity.Path) == entity.Entity.Name+".yml"
}

func parentEntityPath(entity LoadedEntity) string {
	return appRelativePath(entity.AppDir, filepath.Join(filepath.Dir(entity.Path), entity.CollectionParent+".yml"))
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

func hookDiagnostic(app manifest.LoadedApp, path string, entityName string) string {
	return fmt.Sprintf("%s: app %q hook file %q does not match a known Entity name in the same app", location(path, 0), app.Manifest.Name, entityName)
}

func location(path string, line int) string {
	if line == 0 {
		return path
	}
	return fmt.Sprintf("%s:%d", path, line)
}
