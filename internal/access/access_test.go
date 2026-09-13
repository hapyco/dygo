package access

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hapyco/dygo/internal/entity/catalog"
	"github.com/hapyco/dygo/internal/entity/schema"
	"github.com/hapyco/dygo/internal/permissions"
)

func TestLoadPagePolicyFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "apps", "studio", "access", "home.page.access.yml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("policy:\n  - role: studio-member\n    can: [read]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := loadPolicyFile(root, "studio", "studio", "", "home", path)
	if err != nil {
		t.Fatalf("loadPolicyFile() error = %v, want nil", err)
	}
	if file.Entity != "" || file.Page != "home" || file.TargetApp != "studio" || len(file.Items) != 1 {
		t.Fatalf("page policy file = %+v, want studio/home page policy", file)
	}
}

func TestValidatePagePolicy(t *testing.T) {
	plan := Plan{
		Roles: []Role{{App: "studio", Name: "studio-member", Label: "Studio Member"}},
		Policies: []PolicyFile{{
			ContributorApp: "studio",
			TargetApp:      "studio",
			Page:           "home",
			Items:          []PolicyItem{{Role: "studio-member", Can: []permissions.Action{permissions.ActionRead}}},
		}},
	}
	if err := ValidateWithPages(&plan, nil, nil, nil); err != nil {
		t.Fatalf("ValidateWithPages() error = %v, want nil for page policy", err)
	}
	if len(plan.Grants) != 1 || plan.Grants[0].Entity != "" || plan.Grants[0].Page != "home" {
		t.Fatalf("page grants = %+v, want one home page grant", plan.Grants)
	}
}

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
	if err := ValidateWithPages(&duplicate, entities, nil, nil); err == nil || !strings.Contains(err.Error(), "duplicate policy") {
		t.Fatalf("ValidateWithPages(duplicate) error = %v, want duplicate policy error", err)
	}

	override := Plan{
		Roles: roles,
		Policies: []PolicyFile{
			policyFile("core.access.yml", "core", "sales", "invoice", PolicyItem{Role: "sales-manager", Can: []permissions.Action{permissions.ActionRead}}),
			policyFile("sales.access.yml", "sales", "sales", "invoice", PolicyItem{Role: "sales-manager", Can: []permissions.Action{permissions.ActionUpdate}, Override: true}),
		},
	}
	if err := ValidateWithPages(&override, entities, nil, nil); err != nil {
		t.Fatalf("ValidateWithPages(override) error = %v, want nil", err)
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
