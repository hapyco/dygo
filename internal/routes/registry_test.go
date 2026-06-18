package routes

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/hapyco/dygo/internal/entity/catalog"
	"github.com/hapyco/dygo/internal/entity/schema"
)

func TestEntriesBuildsStableRouteRegistry(t *testing.T) {
	entities := []catalog.LoadedEntity{
		{
			AppName: "sales",
			Path:    "/project/apps/sales/entities/deal/deal.entity.yml",
			Entity: schema.Entity{
				Name:  "deal",
				Label: "Deal",
				Route: schema.Route{Slug: "sales-deal"},
			},
		},
		{
			AppName: "sales",
			Path:    "/project/apps/sales/entities/_collections/deal-row.yml",
			Entity: schema.Entity{
				Name:         "deal-row",
				Label:        "Deal Row",
				IsCollection: true,
			},
		},
		{
			AppName: "core",
			Path:    "/project/apps/core/entities/settings/settings.entity.yml",
			Entity: schema.Entity{
				Name:     "settings",
				Label:    "Settings",
				IsSingle: true,
			},
		},
	}

	got := Entries(entities)
	want := []Entry{
		{Slug: "sales-deal", AppName: "sales", EntityName: "deal", Kind: "normal", Path: "/project/apps/sales/entities/deal/deal.entity.yml"},
		{Slug: "settings", AppName: "core", EntityName: "settings", Kind: "single", Path: "/project/apps/core/entities/settings/settings.entity.yml"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Entries() = %+v, want %+v", got, want)
	}
}

func TestReservedSlugs(t *testing.T) {
	if !IsReservedSlug("api") {
		t.Fatal("IsReservedSlug(api) = false, want true")
	}
	if IsReservedSlug("lead") {
		t.Fatal("IsReservedSlug(lead) = true, want false")
	}
	if got := PrefixedReservedSlugs(); len(got) == 0 || got[0][0] != '/' {
		t.Fatalf("PrefixedReservedSlugs() = %#v, want slash-prefixed values", got)
	}
}

func TestRegistryIncludesReservedAndEntityRoutes(t *testing.T) {
	entities := []catalog.LoadedEntity{
		loadedRouteEntity("sales", "lead", "sales-lead"),
	}

	got := Registry(entities)
	if len(got) != len(ReservedSlugs())+1 {
		t.Fatalf("Registry() returned %d routes, want %d", len(got), len(ReservedSlugs())+1)
	}
	if !containsRegistryEntry(got, RegistryEntry{Path: "/api", Kind: "reserved", Owner: "framework reserved route"}) {
		t.Fatalf("Registry() = %+v, want /api reserved route", got)
	}
	if !containsRegistryEntry(got, RegistryEntry{Path: "/sales-lead", Kind: "entity", Owner: "entity sales/lead", Source: "/project/apps/sales/entities/lead/lead.entity.yml"}) {
		t.Fatalf("Registry() = %+v, want sales lead entity route", got)
	}
}

func TestValidatePassesStaticRegistry(t *testing.T) {
	result, err := Validate([]catalog.LoadedEntity{
		loadedRouteEntity("sales", "lead", "sales-lead"),
	})
	if err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
	if result.ReservedRoutes != len(ReservedSlugs()) {
		t.Fatalf("ReservedRoutes = %d, want %d", result.ReservedRoutes, len(ReservedSlugs()))
	}
	if result.EntityRoutes != 1 {
		t.Fatalf("EntityRoutes = %d, want 1", result.EntityRoutes)
	}
	if result.Conflicts != 0 {
		t.Fatalf("Conflicts = %d, want 0", result.Conflicts)
	}
}

func TestValidateReportsReservedRouteConflict(t *testing.T) {
	_, err := Validate([]catalog.LoadedEntity{
		loadedRouteEntity("sales", "login", "login"),
	})
	assertRouteValidationError(t, err, "/login claimed by entity sales/login and framework reserved route")
}

func TestValidateReportsDuplicateEntityRouteConflict(t *testing.T) {
	_, err := Validate([]catalog.LoadedEntity{
		loadedRouteEntity("sales", "customer", "customer"),
		loadedRouteEntity("support", "customer", "customer"),
	})
	assertRouteValidationError(t, err, "/customer claimed by entity sales/customer and entity support/customer")
}

func loadedRouteEntity(appName string, entityName string, routeSlug string) catalog.LoadedEntity {
	return catalog.LoadedEntity{
		AppName: appName,
		Path:    "/project/apps/" + appName + "/entities/" + entityName + "/" + entityName + ".entity.yml",
		Entity: schema.Entity{
			Name:  entityName,
			Label: entityName,
			Route: schema.Route{Slug: routeSlug},
		},
	}
}

func containsRegistryEntry(entries []RegistryEntry, want RegistryEntry) bool {
	for _, entry := range entries {
		if entry == want {
			return true
		}
	}
	return false
}

func assertRouteValidationError(t *testing.T, err error, want string) {
	t.Helper()

	if err == nil {
		t.Fatal("Validate() error = nil, want route validation error")
	}
	var validation ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("Validate() error = %T %v, want ValidationError", err, err)
	}
	if len(validation.Problems) != 1 {
		t.Fatalf("Problems = %#v, want one problem", validation.Problems)
	}
	if !strings.Contains(validation.Problems[0], want) {
		t.Fatalf("problem = %q, want substring %q", validation.Problems[0], want)
	}
}
