package catalog

import (
	"path/filepath"
	"testing"

	appregistry "github.com/dygo-dev/dygo/internal/app/registry"
	"github.com/dygo-dev/dygo/internal/entity/fieldtype"
)

func TestRepositoryEntitiesValidate(t *testing.T) {
	appsRoot := filepath.Join("..", "..", "..")
	apps, err := appregistry.New(appsRoot).Validate()
	if err != nil {
		t.Fatalf("app registry Validate(repository) error = %v, want nil", err)
	}

	if _, err := New(apps, fieldtype.DefaultRegistry()).Validate(); err != nil {
		t.Fatalf("entity catalog Validate(repository) error = %v, want nil", err)
	}
}
