// Package routes builds the framework route registry from discovered Entity metadata.
package routes

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hapyco/dygo/internal/entity/catalog"
)

// Entry describes one Entity-owned route.
type Entry struct {
	Slug       string
	AppName    string
	EntityName string
	Kind       string
	Path       string
}

// RegistryEntry is one owner claim in public root route space.
type RegistryEntry struct {
	Path   string
	Kind   string
	Owner  string
	Source string
}

// ValidationResult summarizes the static route registry.
type ValidationResult struct {
	ReservedRoutes int
	EntityRoutes   int
	Conflicts      int
}

// ValidationError reports public route ownership conflicts.
type ValidationError struct {
	Problems []string
}

func (e ValidationError) Error() string {
	return "route validation failed: " + strings.Join(e.Problems, "; ")
}

// Entries returns routeable Entity routes in stable slug order.
func Entries(entities []catalog.LoadedEntity) []Entry {
	routes := make([]Entry, 0, len(entities))
	for _, entity := range entities {
		if !entity.HasRouteSlug() {
			continue
		}
		routes = append(routes, Entry{
			Slug:       entity.RouteSlug(),
			AppName:    entity.AppName,
			EntityName: entity.Entity.Name,
			Kind:       entity.Kind(),
			Path:       entity.Path,
		})
	}
	sort.SliceStable(routes, func(i, j int) bool {
		if routes[i].Slug != routes[j].Slug {
			return routes[i].Slug < routes[j].Slug
		}
		return routes[i].Path < routes[j].Path
	})
	return routes
}

// Registry returns framework-reserved and Entity-owned root route claims.
func Registry(entities []catalog.LoadedEntity) []RegistryEntry {
	entityRoutes := Entries(entities)
	routes := make([]RegistryEntry, 0, len(ReservedSlugs())+len(entityRoutes))
	for _, slug := range ReservedSlugs() {
		routes = append(routes, RegistryEntry{
			Path:  "/" + slug,
			Kind:  "reserved",
			Owner: "framework reserved route",
		})
	}
	for _, route := range entityRoutes {
		routes = append(routes, RegistryEntry{
			Path:   "/" + route.Slug,
			Kind:   "entity",
			Owner:  fmt.Sprintf("entity %s/%s", route.AppName, route.EntityName),
			Source: route.Path,
		})
	}
	sort.SliceStable(routes, func(i, j int) bool {
		if routes[i].Path != routes[j].Path {
			return routes[i].Path < routes[j].Path
		}
		if routes[i].Kind != routes[j].Kind {
			return routes[i].Kind < routes[j].Kind
		}
		return routes[i].Owner < routes[j].Owner
	})
	return routes
}

// Validate checks the static public route registry for ownership conflicts.
func Validate(entities []catalog.LoadedEntity) (ValidationResult, error) {
	entityRoutes := Entries(entities)
	result := ValidationResult{
		ReservedRoutes: len(ReservedSlugs()),
		EntityRoutes:   len(entityRoutes),
	}
	claims := map[string][]RegistryEntry{}
	for _, route := range Registry(entities) {
		claims[route.Path] = append(claims[route.Path], route)
	}

	var paths []string
	for routePath, owners := range claims {
		if len(owners) > 1 {
			paths = append(paths, routePath)
		}
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		return result, nil
	}

	problems := make([]string, 0, len(paths))
	for _, routePath := range paths {
		owners := make([]string, len(claims[routePath]))
		for index, claim := range claims[routePath] {
			owners[index] = claim.Owner
		}
		problems = append(problems, fmt.Sprintf("%s claimed by %s", routePath, strings.Join(owners, " and ")))
	}
	result.Conflicts = len(problems)
	return result, ValidationError{Problems: problems}
}

// ReservedSlugs returns framework-reserved root route slugs without a leading slash.
func ReservedSlugs() []string {
	return catalog.ReservedRootRouteSlugs()
}

// PrefixedReservedSlugs returns framework-reserved root route slugs with a leading slash.
func PrefixedReservedSlugs() []string {
	slugs := ReservedSlugs()
	for index := range slugs {
		slugs[index] = "/" + slugs[index]
	}
	return slugs
}

// IsReservedSlug reports whether slug is reserved by the framework at root route space.
func IsReservedSlug(slug string) bool {
	for _, reserved := range ReservedSlugs() {
		if slug == reserved {
			return true
		}
	}
	return false
}
