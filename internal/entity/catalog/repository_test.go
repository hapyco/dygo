package catalog

import (
	"path/filepath"
	"sort"
	"strings"
	"testing"

	appregistry "github.com/hapyco/dygo/internal/app/registry"
	"github.com/hapyco/dygo/internal/entity/fieldtype"
)

func TestRepositoryEntitiesValidate(t *testing.T) {
	appsRoot := filepath.Join("..", "..", "..")
	apps, err := appregistry.New(appsRoot).Validate()
	if err != nil {
		t.Fatalf("app registry Validate(repository) error = %v, want nil", err)
	}

	entities, err := New(apps, fieldtype.DefaultRegistry()).Validate()
	if err != nil {
		t.Fatalf("entity catalog Validate(repository) error = %v, want nil", err)
	}

	var coreEntities []string
	for _, entity := range entities {
		if entity.AppName != "core" {
			continue
		}
		coreEntities = append(coreEntities, entity.Entity.Name)
	}
	sort.Strings(coreEntities)

	want := []string{
		"activity",
		"app",
		"configuration",
		"constraint",
		"country",
		"currency",
		"entity",
		"field",
		"index",
		"job",
		"job-execution",
		"language",
		"naming-series",
		"patch-run",
		"permission",
		"role",
		"session",
		"user",
		"user-role",
	}
	if strings.Join(coreEntities, ",") != strings.Join(want, ",") {
		t.Fatalf("core entities = %#v, want %#v", coreEntities, want)
	}
}
