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
					Description: "User identity",
					Icon:        "user",
					IsSingle:    true,
					Fields: []schema.Field{
						{Name: "email", Label: "Email", Type: "email", Required: true, Unique: true, Index: true},
						{Name: "enabled", Label: "Enabled", Type: "boolean", Default: yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "true"}},
						{Name: "status", Label: "Status", Type: "select", Check: &schema.Check{Operator: "in", Value: yaml.Node{Kind: yaml.SequenceNode, Content: []*yaml.Node{
							{Kind: yaml.ScalarNode, Tag: "!!str", Value: "Active"},
							{Kind: yaml.ScalarNode, Tag: "!!str", Value: "Disabled"},
						}}}, Options: fieldtype.Options{Values: []string{"Active", "Disabled"}}},
					},
					Indexes: []schema.Index{
						{Name: "by-enabled-status", Fields: []string{"enabled", "status"}},
					},
					Constraints: []schema.Constraint{
						{Type: "unique", Fields: []string{"email", "status"}},
						{Type: "check", Field: "status", Operator: "in", Value: yaml.Node{Kind: yaml.SequenceNode, Content: []*yaml.Node{
							{Kind: yaml.ScalarNode, Tag: "!!str", Value: "Active"},
							{Kind: yaml.ScalarNode, Tag: "!!str", Value: "Disabled"},
						}}},
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
	if len(records.Entities) != 1 || records.Entities[0].Name != "core.user" || records.Entities[0].Key != "user" || records.Entities[0].Slug != "user" || records.Entities[0].Icon != "user" || records.Entities[0].AppName != "core" || !records.Entities[0].IsSingle {
		t.Fatalf("entity records = %+v, want core/user", records.Entities)
	}
	if records.Entities[0].Naming != nil {
		t.Fatalf("single entity naming metadata = %s, want nil", records.Entities[0].Naming)
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
	if !strings.Contains(string(status.Check), `"operator":"in"`) || !strings.Contains(string(status.Check), `"Active"`) {
		t.Fatalf("status check = %s, want field check metadata", status.Check)
	}
	if len(records.Indexes) != 1 || records.Indexes[0].Name != "by-enabled-status" || string(records.Indexes[0].Fields) != `["enabled","status"]` {
		t.Fatalf("index records = %+v, want top-level index", records.Indexes)
	}
	if len(records.Constraints) != 2 {
		t.Fatalf("constraint records count = %d, want 2", len(records.Constraints))
	}
	unique := records.Constraints[0]
	if unique.Name != "user-email-status-key" || unique.Type != "unique" || string(unique.Fields) != `["email","status"]` {
		t.Fatalf("unique constraint record = %+v, want email/status unique", unique)
	}
	check := records.Constraints[1]
	if check.Name != "user-status-in-check" || check.Type != "check" || check.Field != "status" || !strings.Contains(string(check.Value), `"Active"`) {
		t.Fatalf("check constraint record = %+v, want status check", check)
	}
}

func TestBuildMetadataRecordsUsesEntityTableNamingTemplate(t *testing.T) {
	records, err := buildMetadataRecords(metadataCatalog{
		Apps: []manifest.LoadedApp{
			{Manifest: manifest.Manifest{Name: "core", Label: "Core", Version: "0.1.0"}},
		},
		Entities: []catalog.LoadedEntity{
			{
				AppName: "core",
				Path:    "apps/core/entities/entity.yml",
				Entity: schema.Entity{
					Name:   "entity",
					Label:  "Entity",
					Naming: schema.Naming{Strategy: schema.NamingStrategyTemplate, Template: "{app}.{key}"},
					Fields: []schema.Field{
						{Name: "app", Label: "App", Type: "link", Required: true, Options: fieldtype.Options{Entity: "app"}},
						{Name: "key", Label: "Key", Type: "text", Required: true},
					},
				},
			},
			{
				AppName: "core",
				Path:    "apps/core/entities/user.yml",
				Entity: schema.Entity{
					Name:  "user",
					Label: "User",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("buildMetadataRecords() error = %v, want nil", err)
	}
	if len(records.Entities) != 2 {
		t.Fatalf("entity records count = %d, want 2", len(records.Entities))
	}
	if records.Entities[0].Name != "core.entity" || records.Entities[0].Key != "entity" {
		t.Fatalf("entity record = %+v, want core.entity", records.Entities[0])
	}
	if records.Entities[1].Name != "core.user" || records.Entities[1].Key != "user" {
		t.Fatalf("user record = %+v, want core.user", records.Entities[1])
	}
	if !strings.Contains(string(records.Entities[0].Naming), `"strategy":"template"`) || !strings.Contains(string(records.Entities[0].Naming), `"{app}.{key}"`) {
		t.Fatalf("entity naming metadata = %s, want template strategy", records.Entities[0].Naming)
	}
}

func TestEntityNamingJSONOrdersStrategyFirst(t *testing.T) {
	tests := []struct {
		name   string
		naming schema.Naming
		want   string
	}{
		{
			name:   "random",
			naming: schema.Naming{Strategy: schema.NamingStrategyRandom, Length: 16},
			want:   `{"strategy":"random","length":16}`,
		},
		{
			name:   "field",
			naming: schema.Naming{Strategy: schema.NamingStrategyField, Field: "name"},
			want:   `{"strategy":"field","field":"name"}`,
		},
		{
			name:   "series",
			naming: schema.Naming{Strategy: schema.NamingStrategySeries, Pattern: "SINV-{YYYY}-{#####}"},
			want:   `{"strategy":"series","pattern":"SINV-{YYYY}-{#####}"}`,
		},
		{
			name:   "template",
			naming: schema.Naming{Strategy: schema.NamingStrategyTemplate, Template: "{app}.{key}"},
			want:   `{"strategy":"template","template":"{app}.{key}"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := entityNamingJSON(tt.naming)
			if err != nil {
				t.Fatalf("entityNamingJSON() error = %v, want nil", err)
			}
			if string(got) != tt.want {
				t.Fatalf("entityNamingJSON() = %s, want %s", got, tt.want)
			}
		})
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
