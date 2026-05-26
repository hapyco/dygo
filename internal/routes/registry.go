// Package routes builds the framework route registry from discovered Entity metadata.
package routes

import (
	"sort"

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
