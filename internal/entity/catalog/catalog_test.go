package catalog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dygo-dev/dygo/internal/app/manifest"
	"github.com/dygo-dev/dygo/internal/entity/fieldtype"
)

func TestValidateLoadsEntitiesFromManifestPath(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	app := loadedApp(root, "sales", "sales", manifest.Paths{Entities: "metadata/entities"}, manifest.Module{Name: "crm", Label: "CRM"})
	entityPath := filepath.Join(app.Dir, "metadata", "entities", "lead.yml")
	writeEntity(t, entityPath, "lead", "crm")

	entities, err := New([]manifest.LoadedApp{app}, fieldtype.DefaultRegistry()).Validate()
	if err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
	if len(entities) != 1 {
		t.Fatalf("Validate() len = %d, want 1", len(entities))
	}

	entity := entities[0]
	if entity.AppName != "sales" {
		t.Fatalf("LoadedEntity.AppName = %q, want sales", entity.AppName)
	}
	if entity.AppDir != app.Dir {
		t.Fatalf("LoadedEntity.AppDir = %q, want %q", entity.AppDir, app.Dir)
	}
	if entity.Path != entityPath {
		t.Fatalf("LoadedEntity.Path = %q, want %q", entity.Path, entityPath)
	}
	if entity.Entity.Name != "lead" {
		t.Fatalf("LoadedEntity.Entity.Name = %q, want lead", entity.Entity.Name)
	}
}

func TestValidateAllowsMissingEntitiesDirectory(t *testing.T) {
	t.Parallel()

	app := loadedApp(t.TempDir(), "sales", "sales", manifest.Paths{}, manifest.Module{Name: "crm", Label: "CRM"})

	entities, err := New([]manifest.LoadedApp{app}, fieldtype.DefaultRegistry()).Validate()
	if err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
	if len(entities) != 0 {
		t.Fatalf("Validate() len = %d, want 0", len(entities))
	}
}

func TestValidateReturnsDeterministicOrdering(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	sales := loadedApp(root, "sales", "sales", manifest.Paths{}, manifest.Module{Name: "crm", Label: "CRM"})
	core := loadedApp(root, "core", "core", manifest.Paths{}, manifest.Module{Name: "core", Label: "Core"})
	writeEntity(t, filepath.Join(sales.Dir, "entities", "z-lead.yml"), "lead", "crm")
	writeEntity(t, filepath.Join(sales.Dir, "entities", "a-company.yml"), "company", "crm")
	writeEntity(t, filepath.Join(core.Dir, "entities", "user.yml"), "user", "core")

	entities, err := New([]manifest.LoadedApp{sales, core}, fieldtype.DefaultRegistry()).Validate()
	if err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}

	got := entityKeys(entities)
	want := "core/user,sales/company,sales/lead"
	if strings.Join(got, ",") != want {
		t.Fatalf("Validate() order = %q, want %q", strings.Join(got, ","), want)
	}
}

func TestValidateRejectsInvalidEntityWithAppAndPathContext(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	app := loadedApp(root, "sales", "sales", manifest.Paths{}, manifest.Module{Name: "crm", Label: "CRM"})
	badPath := filepath.Join(app.Dir, "entities", "bad.yml")
	writeFile(t, badPath, `
name: bad
module: crm
fields:
  - name: title
    label: Title
    type: text
`)

	_, err := New([]manifest.LoadedApp{app}, fieldtype.DefaultRegistry()).Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want invalid entity error")
	}
	for _, want := range []string{`app "sales"`, badPath, "label is required"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Validate() error = %q, want substring %q", err.Error(), want)
		}
	}
}

func TestValidateRejectsDuplicateEntityNamesWithinApp(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	app := loadedApp(root, "sales", "sales", manifest.Paths{}, manifest.Module{Name: "crm", Label: "CRM"})
	writeEntity(t, filepath.Join(app.Dir, "entities", "lead.yml"), "lead", "crm")
	writeEntity(t, filepath.Join(app.Dir, "entities", "lead-copy.yml"), "lead", "crm")

	_, err := New([]manifest.LoadedApp{app}, fieldtype.DefaultRegistry()).Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want duplicate entity error")
	}
	if !strings.Contains(err.Error(), `app "sales" has duplicate entity "lead"`) {
		t.Fatalf("Validate() error = %q, want duplicate entity context", err.Error())
	}
}

func TestValidateAllowsDuplicateEntityNamesAcrossApps(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	sales := loadedApp(root, "sales", "sales", manifest.Paths{}, manifest.Module{Name: "crm", Label: "CRM"})
	support := loadedApp(root, "support", "support", manifest.Paths{}, manifest.Module{Name: "support", Label: "Support"})
	writeEntity(t, filepath.Join(sales.Dir, "entities", "customer.yml"), "customer", "crm")
	writeEntity(t, filepath.Join(support.Dir, "entities", "customer.yml"), "customer", "support")

	entities, err := New([]manifest.LoadedApp{sales, support}, fieldtype.DefaultRegistry()).Validate()
	if err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
	if len(entities) != 2 {
		t.Fatalf("Validate() len = %d, want 2", len(entities))
	}
}

func TestValidateRejectsUndeclaredModule(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	app := loadedApp(root, "sales", "sales", manifest.Paths{}, manifest.Module{Name: "sales", Label: "Sales"})
	entityPath := filepath.Join(app.Dir, "entities", "lead.yml")
	writeEntity(t, entityPath, "lead", "crm")

	_, err := New([]manifest.LoadedApp{app}, fieldtype.DefaultRegistry()).Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want undeclared module error")
	}
	for _, want := range []string{`app "sales"`, `entity "lead"`, entityPath, `undeclared module "crm"`} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Validate() error = %q, want substring %q", err.Error(), want)
		}
	}
}

func TestDiscoverIgnoresNonYAMLFilesAndNestedDirectories(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	app := loadedApp(root, "sales", "sales", manifest.Paths{}, manifest.Module{Name: "crm", Label: "CRM"})
	writeEntity(t, filepath.Join(app.Dir, "entities", "lead.yml"), "lead", "crm")
	writeFile(t, filepath.Join(app.Dir, "entities", "ignored.yaml"), "not: valid: yaml")
	writeFile(t, filepath.Join(app.Dir, "entities", "notes.txt"), "not an entity")
	writeEntity(t, filepath.Join(app.Dir, "entities", "nested", "bad.yml"), "bad", "unknown")

	entities, err := New([]manifest.LoadedApp{app}, fieldtype.DefaultRegistry()).Validate()
	if err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
	if got := entityKeys(entities); strings.Join(got, ",") != "sales/lead" {
		t.Fatalf("Validate() entities = %#v, want [sales/lead]", got)
	}
}

func loadedApp(root string, dirName string, name string, paths manifest.Paths, modules ...manifest.Module) manifest.LoadedApp {
	dir := filepath.Join(root, dirName)
	return manifest.LoadedApp{
		Dir:          dir,
		ManifestPath: filepath.Join(dir, manifest.Filename),
		Manifest: manifest.Manifest{
			Name:    name,
			Label:   labelForName(name),
			Version: "0.1.0",
			Modules: modules,
			Paths:   paths.WithDefaults(),
		},
	}
}

func writeEntity(t *testing.T, path string, name string, module string) {
	t.Helper()

	writeFile(t, path, `
name: `+name+`
label: `+labelForName(name)+`
module: `+module+`
fields:
  - name: title
    label: Title
    type: text
`)
}

func writeFile(t *testing.T, path string, body string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(strings.TrimSpace(body)+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func entityKeys(entities []LoadedEntity) []string {
	keys := make([]string, 0, len(entities))
	for _, entity := range entities {
		keys = append(keys, entity.AppName+"/"+entity.Entity.Name)
	}
	return keys
}

func labelForName(name string) string {
	if name == "" {
		return ""
	}
	return strings.ToUpper(name[:1]) + name[1:]
}
