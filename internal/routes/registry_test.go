package routes

import (
	"reflect"
	"testing"

	"github.com/hapyco/dygo/internal/entity/catalog"
	"github.com/hapyco/dygo/internal/entity/schema"
)

func TestEntriesBuildsStableRouteRegistry(t *testing.T) {
	entities := []catalog.LoadedEntity{
		{
			AppName: "sales",
			Path:    "/project/apps/sales/entities/deal/entity.yml",
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
			Path:    "/project/apps/core/entities/settings/entity.yml",
			Entity: schema.Entity{
				Name:     "settings",
				Label:    "Settings",
				IsSingle: true,
			},
		},
	}

	got := Entries(entities)
	want := []Entry{
		{Slug: "sales-deal", AppName: "sales", EntityName: "deal", Kind: "normal", Path: "/project/apps/sales/entities/deal/entity.yml"},
		{Slug: "settings", AppName: "core", EntityName: "settings", Kind: "single", Path: "/project/apps/core/entities/settings/entity.yml"},
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
