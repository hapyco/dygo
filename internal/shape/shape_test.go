package shape

import "testing"

func TestParseAppRef(t *testing.T) {
	t.Parallel()

	ref, err := ParseAppRef("crm/lead")
	if err != nil {
		t.Fatalf("ParseAppRef() error = %v, want nil", err)
	}
	if ref.App != "crm" || ref.Name != "lead" {
		t.Fatalf("ParseAppRef() = %+v, want crm/lead", ref)
	}
}

func TestParseAppRefRejectsInvalidTargets(t *testing.T) {
	t.Parallel()

	for _, target := range []string{"lead", "crm/", "/lead", "crm/lead/extra", "CRM/lead", "crm/Lead"} {
		if _, err := ParseAppRef(target); err == nil {
			t.Fatalf("ParseAppRef(%q) error = nil, want error", target)
		}
	}
}

func TestCanonicalPaths(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		AppDir("crm"):                 "apps/crm",
		AppManifestPath("crm"):        "apps/crm/app.yml",
		EntityDir("lead"):             "entities/lead",
		EntityMetadataPath("lead"):    "entities/lead/entity.yml",
		EntityFixturesPath("lead"):    "entities/lead/fixtures.yml",
		CollectionMetadataPath("row"): "entities/_collections/row.yml",
	}
	for got, want := range tests {
		if got != want {
			t.Fatalf("path = %q, want %q", got, want)
		}
	}
}
