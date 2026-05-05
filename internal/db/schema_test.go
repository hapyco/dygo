package db

import (
	"strings"
	"testing"

	"github.com/dygo-dev/dygo/internal/entity/catalog"
	"github.com/dygo-dev/dygo/internal/entity/fieldtype"
	"github.com/dygo-dev/dygo/internal/entity/schema"
	"gopkg.in/yaml.v3"
)

func TestBuildMetadataSchemaStatements(t *testing.T) {
	entities := []catalog.LoadedEntity{
		{
			AppName: "core",
			Path:    "apps/core/entities/app.yml",
			Entity: schema.Entity{
				Name:       "app",
				PluralName: "apps",
				Fields: []schema.Field{
					{Name: "name", Type: "text", Required: true, Unique: true},
					{Name: "status", Type: "select", Required: true, Default: stringDefault("active"), Options: options("installed", "active")},
				},
			},
		},
		{
			AppName: "core",
			Path:    "apps/core/entities/entity.yml",
			Entity: schema.Entity{
				Name:       "entity",
				PluralName: "entities",
				Fields: []schema.Field{
					{Name: "app", Type: "link", Required: true, Options: entityOption("app")},
					{Name: "plural-name", Type: "text", Required: true, Unique: true},
				},
			},
		},
	}

	statements, err := BuildMetadataSchemaStatements(entities)
	if err != nil {
		t.Fatalf("BuildMetadataSchemaStatements() error = %v, want nil", err)
	}
	joined := strings.Join(statements, "\n")
	assertContains(t, joined, `CREATE TABLE IF NOT EXISTS "apps"`)
	assertContains(t, joined, `ALTER TABLE "apps" ADD COLUMN IF NOT EXISTS "name" text NOT NULL`)
	assertContains(t, joined, `ALTER TABLE "entities" ADD COLUMN IF NOT EXISTS "app_id" bigint NOT NULL`)
	assertContains(t, joined, `CONSTRAINT "entities_app_id_fkey" FOREIGN KEY ("app_id") REFERENCES "apps"("id") ON DELETE CASCADE`)
	assertContains(t, joined, `CONSTRAINT "apps_name_key" UNIQUE ("name")`)
	assertContains(t, joined, `CONSTRAINT "apps_status_check" CHECK ("status" IN ('installed', 'active'))`)
	assertContains(t, joined, `"status" text DEFAULT 'active' NOT NULL`)
}

func TestBuildMetadataSchemaStatementsRejectsChildTable(t *testing.T) {
	_, err := BuildMetadataSchemaStatements([]catalog.LoadedEntity{{
		AppName: "crm",
		Path:    "apps/crm/entities/lead.yml",
		Entity: schema.Entity{
			Name:       "lead",
			PluralName: "leads",
			Fields: []schema.Field{
				{Name: "contacts", Type: "child-table", Options: entityOption("lead-contact")},
			},
		},
	}})
	if err == nil {
		t.Fatal("BuildMetadataSchemaStatements() error = nil, want child-table error")
	}
	if !strings.Contains(err.Error(), "child-table storage is not supported") {
		t.Fatalf("BuildMetadataSchemaStatements() error = %q, want child-table context", err.Error())
	}
}

func TestBuildMetadataSchemaStatementsRejectsDuplicateTableNames(t *testing.T) {
	_, err := BuildMetadataSchemaStatements([]catalog.LoadedEntity{
		{AppName: "one", Path: "apps/one/entities/user.yml", Entity: schema.Entity{Name: "user", PluralName: "users"}},
		{AppName: "two", Path: "apps/two/entities/user.yml", Entity: schema.Entity{Name: "user", PluralName: "users"}},
	})
	if err == nil {
		t.Fatal("BuildMetadataSchemaStatements() error = nil, want duplicate table error")
	}
	if !strings.Contains(err.Error(), "duplicate table name") {
		t.Fatalf("BuildMetadataSchemaStatements() error = %q, want duplicate table context", err.Error())
	}
}

func stringDefault(value string) yaml.Node {
	return yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}

func entityOption(entity string) fieldtype.Options {
	return fieldtype.Options{Entity: entity}
}

func options(values ...string) fieldtype.Options {
	return fieldtype.Options{Values: values}
}

func assertContains(t *testing.T, value string, want string) {
	t.Helper()
	if !strings.Contains(value, want) {
		t.Fatalf("value does not contain %q:\n%s", want, value)
	}
}
