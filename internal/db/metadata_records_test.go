package db

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/hapyco/dygo/internal/app/manifest"
	"github.com/hapyco/dygo/internal/entity/catalog"
	"github.com/hapyco/dygo/internal/entity/fieldtype"
	"github.com/hapyco/dygo/internal/entity/schema"
	"github.com/hapyco/dygo/internal/jobs"
	"github.com/hapyco/dygo/internal/schedules"
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
				Path:    testEntityPath("core", "user"),
				Entity: schema.Entity{
					Name:        "user",
					Label:       "User",
					Description: "User identity",
					Icon:        "user",
					IsSingle:    true,
					IsSystem:    true,
					Fields: []schema.Field{
						{Name: "email", Label: "Email", Type: "email", Required: true, Unique: true, Index: true},
						{Name: "enabled", Label: "Enabled", Type: "boolean", Default: yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "true"}},
						{Name: "status", Label: "Status", Type: "select", Check: &schema.Check{Operator: "in", Value: yaml.Node{Kind: yaml.SequenceNode, Content: []*yaml.Node{
							{Kind: yaml.ScalarNode, Tag: "!!str", Value: "Active"},
							{Kind: yaml.ScalarNode, Tag: "!!str", Value: "Disabled"},
						}}}, Fetch: &schema.Fetch{From: "profile.status"}, Options: fieldtype.Options{Values: []string{"Active", "Disabled"}}},
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
	if len(records.Entities) != 1 || records.Entities[0].Name != "core.user" || records.Entities[0].Key != "user" || records.Entities[0].Slug == nil || *records.Entities[0].Slug != "user" || records.Entities[0].Icon != "user" || records.Entities[0].AppName != "core" || !records.Entities[0].IsSingle || !records.Entities[0].IsSystem {
		t.Fatalf("entity records = %+v, want core/user", records.Entities)
	}
	if records.Entities[0].Naming != nil {
		t.Fatalf("single entity naming metadata = %s, want nil", records.Entities[0].Naming)
	}
	if len(records.Fields) != 3 {
		t.Fatalf("field records count = %d, want 3", len(records.Fields))
	}
	email := records.Fields[0]
	if email.RecordName != "core.user.email" {
		t.Fatalf("email field record name = %q, want core.user.email", email.RecordName)
	}
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
	if string(status.Fetch) != `{"from":"profile.status"}` {
		t.Fatalf("status fetch = %s, want profile.status", status.Fetch)
	}
	if len(records.Indexes) != 1 || records.Indexes[0].RecordName != "core.user.by-enabled-status" || records.Indexes[0].Name != "by-enabled-status" || string(records.Indexes[0].Fields) != `["enabled","status"]` {
		t.Fatalf("index records = %+v, want top-level index", records.Indexes)
	}
	if len(records.Constraints) != 2 {
		t.Fatalf("constraint records count = %d, want 2", len(records.Constraints))
	}
	unique := records.Constraints[0]
	if unique.RecordName != "core.user.user-email-status-key" || unique.Name != "user-email-status-key" || unique.Type != "unique" || string(unique.Fields) != `["email","status"]` {
		t.Fatalf("unique constraint record = %+v, want email/status unique", unique)
	}
	check := records.Constraints[1]
	if check.Name != "user-status-in-check" || check.Type != "check" || check.Field != "status" || !strings.Contains(string(check.Value), `"Active"`) {
		t.Fatalf("check constraint record = %+v, want status check", check)
	}
}

func TestBuildMetadataRecordsUsesCoreMetadataNamingFormats(t *testing.T) {
	records, err := buildMetadataRecords(metadataCatalog{
		Apps: []manifest.LoadedApp{
			{Manifest: manifest.Manifest{Name: "core", Label: "Core", Version: "0.1.0"}},
			{Manifest: manifest.Manifest{Name: "sales", Label: "Sales", Version: "0.1.0"}},
		},
		Entities: []catalog.LoadedEntity{
			metadataNamingEntity("entity", schema.Naming{Strategy: schema.NamingStrategyFormat, Format: "{app}.{key}"}),
			metadataNamingEntity("field", schema.Naming{Strategy: schema.NamingStrategyFormat, Format: "{entity}.{field-name}"}),
			metadataNamingEntity("index", schema.Naming{Strategy: schema.NamingStrategyFormat, Format: "{entity}.{index-name}"}),
			metadataNamingEntity("constraint", schema.Naming{Strategy: schema.NamingStrategyFormat, Format: "{entity}.{constraint-name}"}),
			{
				AppName: "sales",
				Path:    testEntityPath("sales", "invoice"),
				Entity: schema.Entity{
					Name:  "invoice",
					Label: "Invoice",
					Fields: []schema.Field{
						{Name: "customer", Label: "Customer", Type: "text", Required: true},
					},
					Indexes: []schema.Index{
						{Name: "by-customer", Fields: []string{"customer"}},
					},
					Constraints: []schema.Constraint{
						{Type: "unique", Fields: []string{"customer"}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("buildMetadataRecords() error = %v, want nil", err)
	}
	if records.Entities[len(records.Entities)-1].Name != "sales.invoice" {
		t.Fatalf("invoice entity record = %+v, want sales.invoice", records.Entities[len(records.Entities)-1])
	}
	if got := records.Fields[len(records.Fields)-1].RecordName; got != "sales.invoice.customer" {
		t.Fatalf("invoice field record name = %q, want sales.invoice.customer", got)
	}
	if got := records.Indexes[0].RecordName; got != "sales.invoice.by-customer" {
		t.Fatalf("invoice index record name = %q, want sales.invoice.by-customer", got)
	}
	if got := records.Constraints[0].RecordName; got != "sales.invoice.invoice-customer-key" {
		t.Fatalf("invoice constraint record name = %q, want sales.invoice.invoice-customer-key", got)
	}
}

func TestBuildMetadataRecordsStoresJobMetadata(t *testing.T) {
	records, err := buildMetadataRecords(metadataCatalog{
		Apps: []manifest.LoadedApp{
			{Manifest: manifest.Manifest{Name: "sales", Label: "Sales", Version: "0.1.0"}},
		},
		Entities: []catalog.LoadedEntity{
			metadataNamingEntity("job", schema.Naming{Strategy: schema.NamingStrategyFormat, Format: "{app}.{key}"}),
		},
		Jobs: []jobs.LoadedJob{
			{
				AppName: "sales",
				Job: jobs.Job{
					Name:        "send-welcome-email",
					Label:       "Send Welcome Email",
					Description: "Sends a welcome email.",
					Queue:       "email",
					Timeout:     "30s",
					Retry:       &jobs.Retry{Attempts: 3},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("buildMetadataRecords() error = %v, want nil", err)
	}
	if len(records.Jobs) != 1 {
		t.Fatalf("job records count = %d, want 1", len(records.Jobs))
	}
	job := records.Jobs[0]
	if job.Name != "sales.send-welcome-email" || job.Key != "send-welcome-email" || job.Source != jobs.JobSourceFile || job.Label != "Send Welcome Email" || job.Queue != "email" || job.Timeout != "30s" || !job.Enabled || job.Retired {
		t.Fatalf("job record = %+v, want synced sales job metadata", job)
	}
	for _, want := range []string{`"attempts":3`, `"initial-delay":"10s"`, `"max-delay":"5m"`} {
		if !strings.Contains(string(job.Retry), want) {
			t.Fatalf("job retry = %s, want %s", job.Retry, want)
		}
	}
}

func TestBuildMetadataRecordsStoresScheduleMetadata(t *testing.T) {
	disabled := false
	records, err := buildMetadataRecords(metadataCatalog{
		Apps: []manifest.LoadedApp{
			{Manifest: manifest.Manifest{Name: "sales", Label: "Sales", Version: "0.1.0"}},
		},
		Entities: []catalog.LoadedEntity{
			metadataNamingEntity("schedule", schema.Naming{Strategy: schema.NamingStrategyFormat, Format: "{app}.{key}"}),
		},
		Schedules: []schedules.LoadedSchedule{
			{
				AppName: "sales",
				Schedule: schedules.Schedule{
					Name:        "weekly-report",
					Label:       "Weekly Report",
					Description: "Runs the weekly report.",
					Cron:        "0 9 * * MON",
					Timezone:    "Asia/Karachi",
					Job:         "sales/send-weekly-report",
					Enabled:     &disabled,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("buildMetadataRecords() error = %v, want nil", err)
	}
	if len(records.Schedules) != 1 {
		t.Fatalf("schedule records count = %d, want 1", len(records.Schedules))
	}
	schedule := records.Schedules[0]
	if schedule.Name != "sales.weekly-report" || schedule.Key != "weekly-report" || schedule.Source != schedules.ScheduleSourceFile || schedule.Label != "Weekly Report" || schedule.Description != "Runs the weekly report." || schedule.Cron != "0 9 * * MON" || schedule.Timezone != "Asia/Karachi" || schedule.JobAppName != "sales" || schedule.JobName != "send-weekly-report" || schedule.Enabled || schedule.Retired {
		t.Fatalf("schedule record = %+v, want synced sales schedule metadata", schedule)
	}
	if schedule.NextRunAt.IsZero() {
		t.Fatalf("schedule next run = zero, want calculated next-run-at")
	}
}

func metadataNamingEntity(name string, naming schema.Naming) catalog.LoadedEntity {
	return catalog.LoadedEntity{
		AppName: "core",
		Path:    testEntityPath("core", name),
		Entity: schema.Entity{
			Name:   name,
			Label:  name,
			Naming: naming,
			Fields: []schema.Field{
				{Name: "name", Label: "Name", Type: "text", Required: true},
			},
		},
	}
}

func TestBuildMetadataRecordsUsesEntityTableNamingFormat(t *testing.T) {
	records, err := buildMetadataRecords(metadataCatalog{
		Apps: []manifest.LoadedApp{
			{Manifest: manifest.Manifest{Name: "core", Label: "Core", Version: "0.1.0"}},
		},
		Entities: []catalog.LoadedEntity{
			{
				AppName: "core",
				Path:    testEntityPath("core", "entity"),
				Entity: schema.Entity{
					Name:   "entity",
					Label:  "Entity",
					Naming: schema.Naming{Strategy: schema.NamingStrategyFormat, Format: "{app}.{key}"},
					Fields: []schema.Field{
						{Name: "app", Label: "App", Type: "link", Required: true, Options: fieldtype.Options{Entity: "app"}},
						{Name: "key", Label: "Key", Type: "text", Required: true},
					},
				},
			},
			{
				AppName: "core",
				Path:    testEntityPath("core", "user"),
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
	if !strings.Contains(string(records.Entities[0].Naming), `"strategy":"format"`) || !strings.Contains(string(records.Entities[0].Naming), `"format":"{app}.{key}"`) {
		t.Fatalf("entity naming metadata = %s, want format strategy", records.Entities[0].Naming)
	}
}

func TestBuildMetadataRecordsStoresFrameworkOwnedCollectionMetadata(t *testing.T) {
	records, err := buildMetadataRecords(metadataCatalog{
		Apps: []manifest.LoadedApp{
			{Manifest: manifest.Manifest{Name: "sales", Label: "Sales", Version: "0.1.0"}},
		},
		Entities: []catalog.LoadedEntity{
			{
				AppName: "sales",
				Path:    testCollectionEntityPath("sales", "invoice-item"),
				Entity: schema.Entity{
					Name:         "invoice-item",
					Label:        "Invoice Item",
					IsCollection: true,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("buildMetadataRecords() error = %v, want nil", err)
	}
	if len(records.Entities) != 1 {
		t.Fatalf("entity records count = %d, want 1", len(records.Entities))
	}
	if records.Entities[0].Slug != nil {
		t.Fatalf("collection entity slug = %q, want nil", *records.Entities[0].Slug)
	}
	if records.Entities[0].Naming != nil {
		t.Fatalf("collection entity naming metadata = %s, want nil", string(records.Entities[0].Naming))
	}
}

func TestEntityNamingJSONRoundTrips(t *testing.T) {
	tests := []struct {
		name   string
		naming schema.Naming
	}{
		{
			name:   "random",
			naming: schema.Naming{Strategy: schema.NamingStrategyRandom, Length: 16},
		},
		{
			name:   "manual",
			naming: schema.Naming{Strategy: schema.NamingStrategyManual, Label: "Name"},
		},
		{
			name:   "series",
			naming: schema.Naming{Strategy: schema.NamingStrategySeries, Pattern: "SINV-{YYYY}-{#####}"},
		},
		{
			name:   "format",
			naming: schema.Naming{Strategy: schema.NamingStrategyFormat, Format: "{app}.{key}"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := entityNamingJSON(tt.naming)
			if err != nil {
				t.Fatalf("entityNamingJSON() error = %v, want nil", err)
			}
			var decoded recordNaming
			if err := json.Unmarshal(got, &decoded); err != nil {
				t.Fatalf("json.Unmarshal(entityNamingJSON()) error = %v, want nil", err)
			}
			if decoded.Strategy != tt.naming.Strategy ||
				decoded.Label != tt.naming.Label ||
				decoded.Length != tt.naming.Length ||
				decoded.Pattern != tt.naming.Pattern ||
				decoded.Format != tt.naming.Format {
				t.Fatalf("entityNamingJSON() decoded = %+v, want %+v", decoded, tt.naming)
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
