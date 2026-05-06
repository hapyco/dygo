package db

import (
	"strings"
	"testing"

	"github.com/dygo-dev/dygo/internal/app/manifest"
	"github.com/dygo-dev/dygo/internal/entity/catalog"
	"github.com/dygo-dev/dygo/internal/entity/fieldtype"
	"github.com/dygo-dev/dygo/internal/entity/schema"
	"gopkg.in/yaml.v3"
)

func TestBuildMetadataRecords(t *testing.T) {
	records, err := buildMetadataRecords(metadataCatalog{
		Apps: []manifest.LoadedApp{
			{Manifest: manifest.Manifest{Name: "core", Label: "Core", Version: "0.1.0"}},
		},
		Entities: []catalog.LoadedEntity{
			{
				AppName: "core",
				Path:    "apps/core/entities/user.yml",
				Entity: schema.Entity{
					Name:        "user",
					Label:       "User",
					PluralName:  "users",
					PluralLabel: "Users",
					Description: "User identity",
					Fields: []schema.Field{
						{Name: "email", Label: "Email", Type: "email", Required: true, Unique: true, Index: true},
						{Name: "enabled", Label: "Enabled", Type: "boolean", Default: yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "true"}},
						{Name: "status", Label: "Status", Type: "select", Options: fieldtype.Options{Values: []string{"Active", "Disabled"}}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("buildMetadataRecords() error = %v, want nil", err)
	}
	if len(records.Apps) != 1 || records.Apps[0].Name != "core" || records.Apps[0].Status != "active" {
		t.Fatalf("app records = %+v, want active core app", records.Apps)
	}
	if len(records.Entities) != 1 || records.Entities[0].Name != "user" || records.Entities[0].AppName != "core" {
		t.Fatalf("entity records = %+v, want core/user", records.Entities)
	}
	if len(records.Fields) != 3 {
		t.Fatalf("field records count = %d, want 3", len(records.Fields))
	}
	email := records.Fields[0]
	if !email.Required || !email.Unique || !email.Index || email.Position != 1 {
		t.Fatalf("email field record = %+v, want required unique indexed position 1", email)
	}
	enabled := records.Fields[1]
	if string(enabled.Default) != "true" {
		t.Fatalf("enabled default = %s, want true", enabled.Default)
	}
	status := records.Fields[2]
	if !strings.Contains(string(status.Options), `"values":["Active","Disabled"]`) {
		t.Fatalf("status options = %s, want select values", status.Options)
	}
}

func TestFieldDefaultJSONRejectsNonScalar(t *testing.T) {
	_, err := fieldDefaultJSON(yaml.Node{Kind: yaml.MappingNode})
	if err == nil {
		t.Fatal("fieldDefaultJSON() error = nil, want non-scalar error")
	}
	if !strings.Contains(err.Error(), "default must be a scalar value") {
		t.Fatalf("fieldDefaultJSON() error = %q, want scalar context", err.Error())
	}
}
