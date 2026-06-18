package access

import (
	"strings"
	"testing"

	"github.com/hapyco/dygo/internal/entity/catalog"
	"github.com/hapyco/dygo/internal/entity/schema"
	"github.com/hapyco/dygo/internal/permissions"
)

func TestValidatePolicyOverrideResolution(t *testing.T) {
	entities := []catalog.LoadedEntity{{AppName: "sales", Entity: schema.Entity{Name: "invoice"}}}
	roles := []Role{{App: "sales", Name: "sales-manager", Label: "Sales Manager"}}

	duplicate := Plan{
		Roles: roles,
		Policies: []PolicyFile{
			policyFile("core.access.yml", "core", "sales", "invoice", PolicyItem{Role: "sales-manager", Can: []permissions.Action{permissions.ActionRead}}),
			policyFile("sales.access.yml", "sales", "sales", "invoice", PolicyItem{Role: "sales-manager", Can: []permissions.Action{permissions.ActionUpdate}}),
		},
	}
	if err := Validate(&duplicate, entities, nil); err == nil || !strings.Contains(err.Error(), "duplicate policy") {
		t.Fatalf("Validate(duplicate) error = %v, want duplicate policy error", err)
	}

	override := Plan{
		Roles: roles,
		Policies: []PolicyFile{
			policyFile("core.access.yml", "core", "sales", "invoice", PolicyItem{Role: "sales-manager", Can: []permissions.Action{permissions.ActionRead}}),
			policyFile("sales.access.yml", "sales", "sales", "invoice", PolicyItem{Role: "sales-manager", Can: []permissions.Action{permissions.ActionUpdate}, Override: true}),
		},
	}
	if err := Validate(&override, entities, nil); err != nil {
		t.Fatalf("Validate(override) error = %v, want nil", err)
	}
	if len(override.Grants) != 1 || len(override.Grants[0].Can) != 1 || override.Grants[0].Can[0] != permissions.ActionUpdate {
		t.Fatalf("override grants = %+v, want update grant", override.Grants)
	}
}

func policyFile(path string, contributor string, targetApp string, entity string, item PolicyItem) PolicyFile {
	item.Path = path
	return PolicyFile{
		ContributorApp: contributor,
		TargetApp:      targetApp,
		Entity:         entity,
		Path:           path,
		ProjectPath:    path,
		Items:          []PolicyItem{item},
	}
}
